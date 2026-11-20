package quality

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/MoeForge/moego/ojson"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
	log "github.com/sirupsen/logrus"
)

type Callback func(method string, request, response any) error
type NativeHost struct {
	Management       *ManagementClient
	Call             Callback
	MaxResponseBytes int
}

func (h *NativeHost) call(ctx context.Context, method string, request, response any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if h == nil || h.Call == nil {
		return fmt.Errorf("unavailable: native host callback is not configured")
	}
	return h.Call(method, request, response)
}

func (h *NativeHost) Check() error {
	var response struct {
		Files []pluginapi.HostAuthFileEntry `json:"files"`
	}
	return h.call(context.Background(), "host.auth.list", struct{}{}, &response)
}

func (h *NativeHost) Capabilities() HostCapabilities {
	c := officialCapabilities()
	c.CredentialManagement = h.Management != nil
	c.ManagementSession = h.Management != nil && h.Management.ready()
	return c
}

func (h *NativeHost) Targets(ctx context.Context) ([]Target, error) {
	var listed struct {
		Files []pluginapi.HostAuthFileEntry `json:"files"`
	}
	if err := h.call(ctx, "host.auth.list", struct{}{}, &listed); err != nil {
		return nil, err
	}
	var catalog pluginapi.HostAuthModelsResponse
	if err := h.call(ctx, "host.auth.models", pluginapi.HostAuthModelsRequest{}, &catalog); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		catalog.Credentials = nil
	}
	models := make(map[string]pluginapi.HostAuthModels, len(catalog.Credentials))
	catalogIDs := make(map[string]int, len(catalog.Credentials))
	for _, item := range catalog.Credentials {
		catalogIDs[item.ID]++
		if _, duplicate := models[item.AuthIndex]; duplicate {
			models[item.AuthIndex] = pluginapi.HostAuthModels{}
		} else {
			models[item.AuthIndex] = item
		}
	}
	// Some config-backed keys appear only in the model catalog. Report them as unknown
	// if the official runtime callback cannot verify them, rather than hiding or testing them.
	listedIDs := make(map[string]bool, len(listed.Files))
	for _, item := range listed.Files {
		listedIDs[item.ID] = item.AuthIndex != "" || listedIDs[item.ID]
	}
	for _, item := range catalog.Credentials {
		if !listedIDs[item.ID] && item.ID != "" && item.AuthIndex != "" {
			listed.Files = append(listed.Files, pluginapi.HostAuthFileEntry{ID: item.ID, AuthIndex: item.AuthIndex, Provider: item.Provider})
			listedIDs[item.ID] = true
		}
	}
	out := make([]Target, 0, len(listed.Files))
	seen := make(map[string]bool)
	excluded, _ := ctx.Value(excludedCredentialIDs{}).(map[string]bool)
	inventoryIDs := make(map[string]int, len(listed.Files))
	inventoryIndices := make(map[string]int, len(listed.Files))
	for _, item := range listed.Files {
		inventoryIDs[item.ID]++
		inventoryIndices[item.AuthIndex]++
	}
	for _, item := range listed.Files {
		if item.ID == "" || item.AuthIndex == "" || seen[item.ID] || excluded[item.ID] {
			continue
		}
		seen[item.ID] = true
		entry, hasEntry := models[item.AuthIndex]
		conflict := inventoryIDs[item.ID] != 1 || inventoryIndices[item.AuthIndex] != 1 || catalogIDs[item.ID] > 1 ||
			hasEntry && (entry.ID != item.ID || entry.AuthIndex != item.AuthIndex) || !hasEntry && catalogIDs[item.ID] > 0
		// CPA's file inventory already contains the same runtime projection as
		// get_runtime. Disk-only fallback entries have no runtime ID or index.
		if item.Status != "" && item.Source == "file" && item.Provider != "" {
			t := runtimeTarget(item, models[item.AuthIndex], true)
			t.identityConflict = t.identityConflict || conflict
			t.catalogVerified = t.catalogVerified && !t.identityConflict
			out = append(out, t)
			continue
		}
		var current pluginapi.HostAuthGetRuntimeResponse
		err := h.call(ctx, "host.auth.get_runtime", pluginapi.HostAuthGetRequest{AuthIndex: item.AuthIndex}, &current)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		verified := err == nil && current.Auth.ID == item.ID && current.Auth.AuthIndex == item.AuthIndex
		if verified {
			item = current.Auth
		}
		t := runtimeTarget(item, models[item.AuthIndex], verified)
		t.identityConflict = t.identityConflict || conflict || err == nil && !verified
		t.catalogVerified = t.catalogVerified && !t.identityConflict
		out = append(out, t)
	}
	return out, nil
}

// A scoped child rechecks only its frozen credential, never a substitute or the whole directory.
func (h *NativeHost) pinnedTarget(ctx context.Context, id, index string) (Target, error) {
	var catalog pluginapi.HostAuthModelsResponse
	if err := h.call(ctx, "host.auth.models", pluginapi.HostAuthModelsRequest{AuthIndex: index}, &catalog); err != nil {
		if ctx.Err() != nil {
			return Target{}, ctx.Err()
		}
		catalog.Credentials = nil
	}
	var current pluginapi.HostAuthGetRuntimeResponse
	if err := h.call(ctx, "host.auth.get_runtime", pluginapi.HostAuthGetRequest{AuthIndex: index}, &current); err != nil {
		return Target{}, err
	}
	var entry pluginapi.HostAuthModels
	if len(catalog.Credentials) == 1 {
		entry = catalog.Credentials[0]
	}
	verified := current.Auth.ID == id && current.Auth.AuthIndex == index
	t := runtimeTarget(current.Auth, entry, verified)
	t.identityConflict = t.identityConflict || !verified || len(catalog.Credentials) > 1
	return t, nil
}

func runtimeTarget(item pluginapi.HostAuthFileEntry, catalog pluginapi.HostAuthModels, verified bool) Target {
	provider := item.Provider
	if provider == "" {
		provider = item.Type
	}
	t := Target{identityVerified: verified, AuthID: item.ID, AuthIndex: item.AuthIndex, Email: item.Email, Provider: provider, Type: item.Type, RuntimeStatus: item.Status,
		ManualDisabled: item.Disabled || item.Status == "disabled", Models: []string{}, Reasoning: map[string]any{}, Reasons: []string{}, UnsupportedReason: "official_host_quality_pause_unavailable"}
	if t.ManualDisabled {
		t.Reasons = append(t.Reasons, "operator_disabled")
	}
	if item.Unavailable {
		t.Reasons = append(t.Reasons, "native_unavailable")
	}
	if item.NextRetryAfter.After(time.Now()) {
		t.Reasons = append(t.Reasons, "native_cooldown")
	}
	if !verified || item.Status == "" {
		t.RuntimeStatus = "unknown"
		t.Reasons = append(t.Reasons, "runtime_status_unavailable")
	}
	t.EffectiveAvailable = len(t.Reasons) == 0
	t.identityConflict = catalog.AuthIndex != "" && (item.AuthIndex != catalog.AuthIndex || item.ID != catalog.ID || catalog.Provider != provider)
	if verified && provider != "" && item.ID == catalog.ID && item.AuthIndex == catalog.AuthIndex && provider == catalog.Provider {
		t.catalogVerified = catalog.Models != nil
		t.Models = append(t.Models, catalog.Models...)
	}
	return t
}
func (h *NativeHost) Actions(ctx context.Context) ([]Action, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, unsupportedQuality()
}
func (h *NativeHost) Pause(ctx context.Context, _ Action) (Action, error) {
	if err := ctx.Err(); err != nil {
		return Action{}, err
	}
	return Action{}, unsupportedQuality()
}
func (h *NativeHost) Release(ctx context.Context, _ Action) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return unsupportedQuality()
}
func (h *NativeHost) Execute(ctx context.Context, e Execution) (s Sample) {
	if e.ManagedProbe != nil {
		return h.executeManaged(ctx, e)
	}
	start := time.Now()
	s = Sample{Model: e.Model, ExecutionStatus: "failed", QualityVerdict: "unknown", Evidence: Evidence{Reason: "official_host_evidence_unavailable"}}
	defer func() {
		s.TotalMS = time.Since(start).Milliseconds()
		if ctx.Err() != nil {
			s.ExecutionStatus = "cancelled"
			s.Error = "cancelled"
		}
	}()

	if err := ctx.Err(); err != nil {
		return s
	}

	doc := ojson.NewDocString(`{}`)
	messages := []map[string]string{}
	if e.System != "" {
		messages = append(messages, map[string]string{"role": "system", "content": e.System})
	}
	messages = append(messages, map[string]string{"role": "user", "content": e.Prompt})
	fields := map[string]any{"model": e.Model, "stream": e.Parameters.Stream, "messages": messages, "max_tokens": e.Parameters.MaxTokens}
	if e.Parameters.Temperature != nil {
		fields["temperature"] = *e.Parameters.Temperature
	}
	if e.Parameters.ReasoningEffort != "" {
		fields["reasoning_effort"] = e.Parameters.ReasoningEffort
	}
	for k, v := range fields {
		if err := doc.Set(k, v); err != nil {
			s.Error = "request construction failed"
			return s
		}
	}
	req := pluginapi.HostModelExecutionRequest{EntryProtocol: "openai", ExitProtocol: "openai", Model: e.Model, Stream: e.Parameters.Stream, Body: doc.Bytes(), AuthID: e.AuthID, ForcedProvider: e.Provider}

	limit := e.ResponseLimit
	if limit == 0 {
		limit = h.MaxResponseBytes
	}
	if limit < 1024 {
		limit = 262144
	}
	collector := newCollector(limit)
	var err error
	if req.Stream {
		var opened pluginapi.HostModelStreamResponse
		err = h.call(ctx, "host.model.execute_stream", req, &opened)
		if opened.StreamID != "" {
			var once sync.Once
			closeStream := func() {
				once.Do(func() {
					var ignored any
					if closeErr := h.call(context.Background(), "host.model.stream_close", pluginapi.HostModelStreamCloseRequest{StreamID: opened.StreamID}, &ignored); closeErr != nil {
						log.Warn("model-quality: host stream cleanup failed")
					}
				})
			}
			stop := context.AfterFunc(ctx, closeStream)
			defer func() { stop(); closeStream() }()
		}
		if err == nil && (opened.StatusCode < 200 || opened.StatusCode >= 300) {
			err = fmt.Errorf("upstream HTTP %d", opened.StatusCode)
		}
		if err == nil && opened.StreamID == "" {
			err = fmt.Errorf("host returned empty stream")
		}
		if err == nil {
			for {
				if ctx.Err() != nil {
					err = ctx.Err()
					break
				}
				var chunk pluginapi.HostModelStreamReadResponse
				if err = h.call(ctx, "host.model.stream_read", pluginapi.HostModelStreamReadRequest{StreamID: opened.StreamID}, &chunk); err != nil {
					break
				}
				if len(chunk.Payload) > limit {
					err = fmt.Errorf("fragment exceeds response limit")
					break
				}
				if chunk.Error != "" {
					err = fmt.Errorf("upstream stream error")
					break
				}
				if len(chunk.Payload) > 0 {
					if err = collector.FeedHostChunk(chunk.Payload); err != nil {
						break
					}
					if s.TTFTMS == nil && collector.text.Len() > 0 {
						ms := time.Since(start).Milliseconds()
						s.TTFTMS = &ms
					}
				}
				if chunk.Done {
					// Official callbacks report stream EOF but do not prove upstream final conditions.
					if collector.rawEvents && collector.finish != "" {
						collector.done = true
					}
					err = collector.End()
					break
				}
			}
		}
	} else {
		var response pluginapi.HostModelExecutionResponse
		err = h.call(ctx, "host.model.execute", req, &response)
		if err == nil {
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				err = fmt.Errorf("upstream HTTP %d", response.StatusCode)
			} else if len(response.Body) > limit {
				err = fmt.Errorf("response limit exceeded")
			} else {
				err = collector.Completion(response.Body)
			}
		}
	}
	s.Output = collector.text.String()
	s.Usage = collector.usage
	s.ResponseModel = collector.model
	if err != nil {
		var withStatus interface{ StatusCode() int }
		if errors.As(err, &withStatus) && withStatus.StatusCode() >= 400 && withStatus.StatusCode() <= 599 {
			s.Error = fmt.Sprintf("upstream HTTP %d", withStatus.StatusCode())
		} else {
			s.Error = scrubError(err)
		}
		return s
	}
	if collector.refusal {
		s.ExecutionStatus = "refused"
		s.Reason = "refusal is not quality evidence"
		return s
	}
	if collector.finish != "stop" {
		s.ExecutionStatus = "incomplete"
		s.Reason = "missing or truncated completion"
		return s
	}
	if strings.TrimSpace(s.Output) == "" {
		s.ExecutionStatus = "empty"
		return s
	}
	s.ExecutionStatus = "completed"
	return s
}

func decode(b []byte, v any) error { return json.Unmarshal(b, v) }

type managementCallbackKey struct{}

// WithManagementCallbackID carries a callback only for the lifetime of its management RPC.
func WithManagementCallbackID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, managementCallbackKey{}, id)
}
