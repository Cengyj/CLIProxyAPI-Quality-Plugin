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
	var c struct {
		Version           int  `json:"version"`
		ManualReleaseOnly bool `json:"manual_release_only"`
		ManagementV8      bool `json:"management_v8"`
		ModelOperations   bool `json:"model_operations"`
		QualityPause      bool `json:"quality_pause"`
		QualityRetest     bool `json:"quality_retest"`
		AutomaticRecovery bool `json:"automatic_recovery"`
	}
	if e := h.call(context.Background(), "host.quality.capabilities", struct{}{}, &c); e != nil {
		return fmt.Errorf("model-quality requires the bundled quality host patch: %w", e)
	}
	if c.Version < 2 || !c.ManualReleaseOnly || !c.ManagementV8 || !c.ModelOperations || !c.QualityPause || c.QualityRetest || c.AutomaticRecovery {
		return fmt.Errorf("incompatible host quality capability")
	}
	return nil
}
func (h *NativeHost) Targets(ctx context.Context) ([]Target, error) {
	var r struct {
		Targets []Target `json:"targets"`
	}
	e := h.call(ctx, "host.quality.targets", struct{}{}, &r)
	return r.Targets, e
}
func (h *NativeHost) Actions(ctx context.Context) ([]Action, error) {
	var r struct {
		Actions []Action `json:"actions"`
	}
	e := h.call(ctx, "host.quality.actions", struct{}{}, &r)
	return r.Actions, e
}
func (h *NativeHost) Pause(ctx context.Context, a Action) (Action, error) {
	var r struct {
		Action Action `json:"action"`
	}
	e := h.call(ctx, "host.quality.pause", a, &r)
	return r.Action, e
}
func (h *NativeHost) Release(ctx context.Context, a Action) error {
	var r any
	id, _ := ctx.Value(managementCallbackKey{}).(string)
	if id == "" {
		return fmt.Errorf("blocked: manual release requires active authenticated management request")
	}
	request := struct {
		Action
		HostCallbackID string `json:"host_callback_id"`
	}{a, id}
	return h.call(ctx, "host.quality.release", request, &r)
}
func (h *NativeHost) Execute(ctx context.Context, e Execution) (s Sample) {
	start := time.Now()
	s = Sample{Model: e.Model, ExecutionStatus: "failed", QualityVerdict: "unknown"}
	defer func() {
		s.TotalMS = time.Since(start).Milliseconds()
		if ctx.Err() != nil {
			s.ExecutionStatus = "cancelled"
			s.Error = "cancelled"
		}
	}()

	var op struct {
		ID string `json:"operation_id"`
	}
	if err := h.call(ctx, "host.model.operation_open", struct{}{}, &op); err != nil {
		s.Error = scrubError(fmt.Errorf("open model operation: %w", err))
		return s
	}
	if op.ID == "" {
		s.Error = "host returned empty operation"
		return s
	}
	var once sync.Once
	cancel := func() {
		once.Do(func() {
			var ignored any
			if err := h.call(context.Background(), "host.model.cancel", map[string]string{"operation_id": op.ID}, &ignored); err != nil {
				log.WithField("operation", "cancel").Warn("model-quality: host operation cleanup failed")
			}
		})
	}
	stop := context.AfterFunc(ctx, cancel)
	defer func() { stop(); cancel() }()
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
	req := pluginapi.HostModelExecutionRequest{EntryProtocol: "openai", ExitProtocol: "openai", Model: e.Model, Stream: e.Parameters.Stream, Body: doc.Bytes(), AuthID: e.AuthID, ForcedProvider: e.Provider, OperationID: op.ID, TrafficSource: e.Source}

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
		s.Evidence = Evidence(opened.Evidence)
		if opened.StreamID != "" {
			defer func() {
				var ignored any
				if err := h.call(context.Background(), "host.model.stream_close", pluginapi.HostModelStreamCloseRequest{StreamID: opened.StreamID}, &ignored); err != nil {
					log.WithField("operation", "stream_close").Warn("model-quality: host operation cleanup failed")
				}
			}()
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
				if chunk.Evidence != nil {
					s.Evidence = Evidence(*chunk.Evidence)
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
					if collector.rawEvents && chunk.UpstreamComplete {
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
		s.Evidence = Evidence(response.Evidence)
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
