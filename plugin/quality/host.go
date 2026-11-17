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

func (h *NativeHost) Capabilities() HostCapabilities { return officialCapabilities() }

func (h *NativeHost) Targets(ctx context.Context) ([]Target, error) {
	var listed struct {
		Files []pluginapi.HostAuthFileEntry `json:"files"`
	}
	if err := h.call(ctx, "host.auth.list", struct{}{}, &listed); err != nil {
		return nil, err
	}
	var catalog pluginapi.HostAuthModelsResponse
	if err := h.call(ctx, "host.auth.models", pluginapi.HostAuthModelsRequest{}, &catalog); err != nil {
		return nil, err
	}
	models := make(map[string][]string, len(catalog.Credentials))
	for _, item := range catalog.Credentials {
		models[item.ID] = append([]string{}, item.Models...)
	}
	out := make([]Target, 0, len(listed.Files))
	seen := make(map[string]bool)
	for _, item := range listed.Files {
		if item.ID == "" || item.AuthIndex == "" || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		var current pluginapi.HostAuthGetRuntimeResponse
		err := h.call(ctx, "host.auth.get_runtime", pluginapi.HostAuthGetRequest{AuthIndex: item.AuthIndex}, &current)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		verified := err == nil && current.Auth.ID == item.ID && current.Auth.AuthIndex == item.AuthIndex
		if verified {
			item = current.Auth
		}
		provider := item.Provider
		if provider == "" {
			provider = item.Type
		}
		reasons := []string{}
		disabled := item.Disabled || item.Status == "disabled"
		if disabled {
			reasons = append(reasons, "operator_disabled")
		}
		if item.Unavailable {
			reasons = append(reasons, "native_unavailable")
		}
		if !verified {
			reasons = append(reasons, "runtime_status_unavailable")
		}
		out = append(out, Target{
			AuthID: item.ID, AuthIndex: item.AuthIndex, Provider: provider, Type: item.Type,
			Models: append([]string{}, models[item.ID]...), Reasoning: map[string]any{},
			ManualDisabled: disabled, EffectiveAvailable: verified && !disabled && !item.Unavailable,
			Reasons: reasons, UnsupportedReason: "official_host_quality_pause_unavailable",
		})
	}
	return out, nil
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
