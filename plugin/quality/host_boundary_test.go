package quality

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

func TestNativeHostAlreadyCancelledMakesNoCallbacks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h := &NativeHost{Call: func(string, any, any) error { t.Fatal("cancelled call reached host"); return nil }}
	if s := h.Execute(ctx, Execution{}); s.ExecutionStatus != "cancelled" || s.QualityVerdict != "unknown" {
		t.Fatal(s)
	}
	if _, err := h.Targets(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := h.Actions(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := h.Pause(ctx, Action{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := h.Release(WithManagementCallbackID(ctx, "scope"), Action{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := h.ActionStatus(ctx, "action"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestNativeHostCancellationWinsCompletedResponse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	h := &NativeHost{Call: func(method string, in, out any) error {
		switch method {
		case "host.model.operation_open":
			return decode([]byte(`{"operation_id":"op"}`), out)
		case "host.model.cancel":
			return nil
		case "host.model.execute":
			cancel()
			response := out.(*pluginapi.HostModelExecutionResponse)
			response.StatusCode = 200
			response.Body = []byte(`{"choices":[{"message":{"content":"4"},"finish_reason":"stop"}]}`)
			return nil
		}
		return fmt.Errorf("unexpected callback %s", method)
	}}
	if s := h.Execute(ctx, Execution{Parameters: Parameters{MaxTokens: 16}}); s.ExecutionStatus != "cancelled" || s.QualityVerdict != "unknown" {
		t.Fatal(s)
	}
}

func TestNativeHostClosesRejectedStreamAndNeverReadsInvalidID(t *testing.T) {
	for _, tc := range []struct {
		name, id string
		status   int
		err      error
	}{
		{"http_error", "stream", 429, nil},
		{"callback_error", "stream", 200, errors.New("callback failed")},
		{"empty_id", "", 200, nil},
		{"redirect", "stream", 302, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var closed atomic.Int32
			h := &NativeHost{Call: func(method string, in, out any) error {
				switch method {
				case "host.model.execute_stream":
					response := out.(*pluginapi.HostModelStreamResponse)
					response.StreamID, response.StatusCode = tc.id, tc.status
					return tc.err
				case "host.model.stream_close":
					if in.(pluginapi.HostModelStreamCloseRequest).StreamID != tc.id {
						t.Error("closed a different stream")
					}
					closed.Add(1)
					return nil
				default:
					t.Errorf("invalid stream reached %s", method)
					return errors.New("unexpected read")
				}
			}}
			s := h.Execute(context.Background(), Execution{Parameters: Parameters{MaxTokens: 16, Stream: true}})
			wantClose := int32(0)
			if tc.id != "" {
				wantClose = 1
			}
			if s.ExecutionStatus != "failed" || closed.Load() != wantClose {
				t.Fatal(s, closed.Load())
			}
		})
	}
}

func TestNativeHostChecksOnlyOfficialCallbackAvailability(t *testing.T) {
	h := &NativeHost{Call: func(method string, _, out any) error {
		if method != "host.auth.list" {
			t.Fatalf("used a private host extension: %s", method)
		}
		return decode([]byte(`{"files":[]}`), out)
	}}
	if err := h.Check(); err != nil {
		t.Fatal(err)
	}
	caps := h.Capabilities()
	if caps.QualityPause || caps.ManualRelease || caps.VerifiedEvidence || caps.ModelOperationCancel || !caps.StreamClose {
		t.Fatal(caps)
	}
	if (&NativeHost{}).Check() == nil {
		t.Fatal("accepted absent callback")
	}
}

func TestNativeHostCancellationDuringStreamClosesOnce(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reading := make(chan struct{})
	cancelled := make(chan struct{})
	var closeCalls atomic.Int32
	reads := 0
	h := &NativeHost{Call: func(method string, in, out any) error {
		switch method {
		case "host.model.operation_open":
			return decode([]byte(`{"operation_id":"op"}`), out)
		case "host.model.execute_stream":
			return decode([]byte(`{"status_code":200,"stream_id":"stream"}`), out)
		case "host.model.stream_read":
			reads++
			if reads == 1 {
				out.(*pluginapi.HostModelStreamReadResponse).Payload = []byte(`{"choices":[{"delta":{"content":"4"},"finish_reason":null}]}`)
				return nil
			}
			close(reading)
			<-cancelled
			return context.Canceled
		case "host.model.stream_close":
			if closeCalls.Add(1) == 1 {
				close(cancelled)
			}
			return nil
		}
		return fmt.Errorf("unexpected callback %s", method)
	}}
	done := make(chan Sample, 1)
	go func() { done <- h.Execute(ctx, Execution{Parameters: Parameters{MaxTokens: 16, Stream: true}}) }()
	<-reading
	cancel()
	s := <-done
	if s.ExecutionStatus != "cancelled" || s.Output != "4" || closeCalls.Load() != 1 {
		t.Fatal(s, closeCalls.Load())
	}
}

func TestNativeHostRecordsHTTPErrorWithoutHostDetails(t *testing.T) {
	for _, status := range []int{401, 403, 429, 500} {
		h := &NativeHost{Call: func(method string, _, out any) error {
			switch method {
			case "host.model.operation_open":
				return decode([]byte(`{"operation_id":"op"}`), out)
			case "host.model.cancel":
				return nil
			default:
				return &pluginabi.Error{Code: "host_call_failed", Message: "private upstream response", HTTPStatus: status}
			}
		}}
		s := h.Execute(context.Background(), Execution{Parameters: Parameters{MaxTokens: 16}})
		if s.ExecutionStatus != "failed" || s.QualityVerdict != "unknown" || s.Error != fmt.Sprintf("upstream HTTP %d", status) {
			t.Fatal(s)
		}
	}
}

func TestCollectorRejectsAnswerAfterFinishAndInvalidUsage(t *testing.T) {
	c := newCollector(4096)
	if err := c.FeedHostChunk([]byte(`{"choices":[{"delta":{"content":"4"},"finish_reason":"stop"}]}`)); err != nil {
		t.Fatal(err)
	}
	if err := c.FeedHostChunk([]byte(`{"choices":[{"delta":{"content":"1"}}]}`)); err == nil {
		t.Fatal("accepted answer content after stop")
	}
	for _, usage := range []string{
		`{"total_tokens":5}`,
		`{"prompt_tokens":3,"completion_tokens":2,"total_tokens":-1}`,
		`{"prompt_tokens":3,"completion_tokens":2,"total_tokens":4}`,
		`{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5.1}`,
		`{"prompt_tokens":"3","completion_tokens":2,"total_tokens":5}`,
		`{"prompt_tokens":3,"completion_tokens":-2,"total_tokens":5}`,
	} {
		c := newCollector(4096)
		raw := `{"choices":[{"message":{"content":"4"},"finish_reason":"stop"}],"usage":` + usage + `}`
		if err := c.Completion([]byte(raw)); err != nil || c.usage != nil {
			t.Fatal("invalid usage must remain unknown", usage, c.usage, err)
		}
	}
}
