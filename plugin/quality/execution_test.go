package quality

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

func TestStrictJudgingAndFrozenQuestions(t *testing.T) {
	q, e := FreezeQuestion(RunConfig{QuestionID: "logic-boxes"})
	if e != nil || q.Answer != "4" {
		t.Fatal(q, e)
	}
	for _, test := range []struct{ answer, verdict string }{{"4", "correct"}, {" 4\n", "correct"}, {"21", "incorrect"}, {"The answer is 4", "unknown"}, {"4 or 5", "unknown"}, {"14", "incorrect"}, {"4.0", "unknown"}, {"04", "unknown"}, {"", "unknown"}} {
		v, _ := DeterministicJudge(q, test.answer)
		if v != test.verdict {
			t.Errorf("%q got %s", test.answer, v)
		}
	}
	if _, e = FreezeQuestion(RunConfig{QuestionID: "logic-boxes", Prompt: "changed"}); e == nil {
		t.Fatal("edited built-in inherited oracle")
	}
	for seed := int64(-20); seed < 100; seed++ {
		a, e := FreezeQuestion(RunConfig{QuestionID: "random-linear", Seed: seed})
		b, _ := FreezeQuestion(RunConfig{QuestionID: "random-linear", Seed: seed})
		if e != nil || a != b || a.Answer == "" {
			t.Fatal("random reproducibility", seed, e)
		}
	}
	for _, test := range []struct{ input, want string }{{`{"verdict":"correct","reason":"ok"}`, "correct"}, {`{"verdict":"incorrect","reason":"bad"}`, "incorrect"}, {`{"verdict":"correct","verdict":"incorrect","reason":"bad"}`, "unknown"}, {`{"verdict":"correct","reason":"ok","extra":true}`, "unknown"}, {`{"verdict":"correct","reason":"ok"} prose`, "unknown"}, {`{"verdict":"CORRECT","reason":"ok"}`, "unknown"}, {`{"verdict":"correct","reason":null}`, "unknown"}, {`[]`, "unknown"}, {"```json\n{}\n```", "unknown"}} {
		v, _ := StrictJudge(test.input)
		if v != test.want {
			t.Errorf("%s got %s", test.input, v)
		}
	}
}
func TestSSEAssemblyRequiresTerminalCompletion(t *testing.T) {
	stream := ": heartbeat\n\ndata: {\"model\":\"mapped\",\"choices\":[{\"delta\":{\"content\":\"4\"},\"finish_reason\":null}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":8,\"completion_tokens\":1,\"total_tokens\":9}}\n\ndata: [DONE]\n\n"
	for width := 1; width < 50; width++ {
		c := newCollector(4096)
		for start := 0; start < len(stream); start += width {
			if e := c.Feed([]byte(stream[start:min(start+width, len(stream))])); e != nil {
				t.Fatal(e)
			}
		}
		if e := c.End(); e != nil || c.text.String() != "4" || c.usage.Total != 9 {
			t.Fatal(c, e)
		}
	}
	for _, raw := range []string{"data: {\"choices\":[{\"delta\":{\"content\":\"4\"}}]}\n\n", "data: [DONE]\n\n", "data: {\"error\":{\"message\":\"bad\"}}\n\n", stream + "data: {}\n\n"} {
		c := newCollector(4096)
		if e := c.Feed([]byte(raw)); e == nil && c.End() == nil {
			t.Fatalf("accepted incomplete/error stream %s", raw)
		}
	}
	c := newCollector(64)
	if c.Feed([]byte(strings.Repeat("x", 65))) == nil {
		t.Fatal("unbounded stream accepted")
	}
}
func TestNativeHostCancellationBeforeFirstByteRemainsPendingUntilOfficialCallbackReturns(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			started, released := make(chan struct{}), make(chan struct{})
			h := &NativeHost{Call: func(method string, in, out any) error {
				if method != "host.model.execute" && method != "host.model.execute_stream" {
					t.Errorf("unexpected callback %s", method)
				}
				req := in.(pluginapi.HostModelExecutionRequest)
				if req.AuthID != "fixed" || req.ForcedProvider != "provider" {
					return fmt.Errorf("lost request binding")
				}
				close(started)
				<-released
				return context.Canceled
			}}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan Sample, 1)
			go func() {
				done <- h.Execute(ctx, Execution{AuthID: "fixed", Provider: "provider", Model: "alias", Parameters: Parameters{MaxTokens: 64, Stream: stream}})
			}()
			<-started
			cancel()
			select {
			case <-done:
				t.Fatal("claimed to cancel a callback without official cancellation support")
			default:
			}
			close(released)
			s := <-done
			if s.ExecutionStatus != "cancelled" || s.Evidence.Verified || h.Capabilities().ModelOperationCancel {
				t.Fatal(s)
			}
		})
	}
}

func TestNonstreamCompletionClassification(t *testing.T) {
	for _, tc := range []struct{ body, status string }{{`{"model":"upstream","choices":[{"message":{"content":"4"},"finish_reason":"stop"}]}`, "completed"}, {`{"choices":[{"message":{"content":"4"},"finish_reason":"length"}]}`, "incomplete"}, {`{"choices":[{"message":{"content":""},"finish_reason":"stop"}]}`, "empty"}, {`{"choices":[{"message":{"refusal":"cannot comply"},"finish_reason":"stop"}]}`, "refused"}, {`{"error":{"message":"rate limit"}}`, "failed"}} {
		h := &NativeHost{Call: func(method string, in, out any) error {
			switch method {
			case "host.model.operation_open":
				return decode([]byte(`{"operation_id":"op"}`), out)
			case "host.model.cancel":
				return nil
			case "host.model.execute":
				b, _ := json.Marshal(pluginapi.HostModelExecutionResponse{StatusCode: 200, Body: []byte(tc.body)})
				return decode(b, out)
			}
			return fmt.Errorf("unexpected method")
		}}
		s := h.Execute(context.Background(), Execution{AuthID: "x", Provider: "x", Model: "model", Prompt: "test", Parameters: Parameters{MaxTokens: 64}})
		if s.ExecutionStatus != tc.status || s.QualityVerdict != "unknown" {
			t.Fatalf("%s: %+v", tc.body, s)
		}
	}
}

func TestManualMaxReasoningPreservedThroughExecution(t *testing.T) {
	engine, host := lifeFixture(t)
	var payloads []struct {
		Model           string `json:"model"`
		ReasoningEffort string `json:"reasoning_effort"`
	}
	native := &NativeHost{Call: func(method string, in, out any) error {
		switch method {
		case "host.model.operation_open":
			return decode([]byte(`{"operation_id":"offline-op"}`), out)
		case "host.model.cancel":
			return nil
		case "host.model.execute":
			req := in.(pluginapi.HostModelExecutionRequest)
			if req.AuthID != "auth-a" || req.ForcedProvider != "offline" {
				return fmt.Errorf("lost manual request binding")
			}
			var payload struct {
				Model           string `json:"model"`
				ReasoningEffort string `json:"reasoning_effort"`
			}
			if err := json.Unmarshal(req.Body, &payload); err != nil {
				return err
			}
			payloads = append(payloads, payload)
			body, err := json.Marshal(pluginapi.HostModelExecutionResponse{StatusCode: 200, Body: []byte(`{"choices":[{"message":{"content":"4"},"finish_reason":"stop"}]}`)})
			if err != nil {
				return err
			}
			return decode(body, out)
		}
		return fmt.Errorf("unexpected callback %s", method)
	}}
	host.execute = native.Execute
	config := lifeConfig()
	config.Judge.Mode = "manual"
	config.Samples = 2
	config.Parameters.ReasoningEffort = "max"
	run, err := engine.Submit(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	config.Parameters.ReasoningEffort = "low"
	engine.execute(<-engine.queue)
	result, err := engine.Run(run.ID)
	if err != nil || result.Status != "completed" || result.Config.Parameters.ReasoningEffort != "max" || len(payloads) != 2 {
		t.Fatalf("max reasoning was not admitted, frozen and executed: result=%+v payloads=%+v err=%v", result, payloads, err)
	}
	for _, payload := range payloads {
		if payload.Model != "a" || payload.ReasoningEffort != "max" {
			t.Fatalf("host payload lost explicit max reasoning: %+v", payload)
		}
	}
}

func TestDuplicateAPIBodyIsRejected(t *testing.T) {
	var v RunConfig
	if DecodeStrict([]byte(`{"auth_id":"a","auth_id":"b"}`), &v) == nil {
		t.Fatal("duplicate auth accepted")
	}
	if DecodeStrict([]byte(`{"parameters":{"max_tokens":1,"max_tokens":2}}`), &v) == nil {
		t.Fatal("nested duplicate accepted")
	}
}

func TestOfficialStreamCompletionNeverBecomesVerifiedEvidence(t *testing.T) {
	for _, finished := range []bool{false, true} {
		t.Run(fmt.Sprint(finished), func(t *testing.T) {
			read := 0
			h := &NativeHost{Call: func(method string, in, out any) error {
				switch method {
				case "host.model.stream_close":
					return nil
				case "host.model.execute_stream":
					return decode([]byte(`{"status_code":200,"stream_id":"s"}`), out)
				case "host.model.stream_read":
					response := out.(*pluginapi.HostModelStreamReadResponse)
					switch read {
					case 0:
						response.Payload = []byte(`{"choices":[{"delta":{"content":"4"},"finish_reason":null}]}`)
					case 1:
						if finished {
							response.Payload = []byte(`{"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":9,"completion_tokens":1,"total_tokens":10}}`)
						} else {
							response.Done = true
						}
					default:
						response.Done = true
					}
					read++
					return nil
				}
				return fmt.Errorf("unexpected callback %s", method)
			}}
			sample := h.Execute(context.Background(), Execution{AuthID: "a", Provider: "p", Model: "m", Parameters: Parameters{MaxTokens: 64, Stream: true}})
			if sample.Evidence.Verified || sample.Comparable || sample.Evidence.Reason != "official_host_evidence_unavailable" {
				t.Fatal("invented verified evidence", sample)
			}
			if finished && (sample.ExecutionStatus != "completed" || sample.Output != "4" || sample.Usage == nil || sample.Usage.Total != 10) {
				t.Fatal(sample)
			}
			if !finished && sample.ExecutionStatus == "completed" {
				t.Fatal("accepted a stream without completion", sample)
			}
		})
	}
}
