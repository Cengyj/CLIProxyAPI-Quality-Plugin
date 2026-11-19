package quality

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

func TestRuleQuestionPreviewIsFixedVersionedAndReadOnly(t *testing.T) {
	en, h, _ := scopeFixture(t)
	read := func(path, id string) pluginapi.ManagementResponse {
		return en.Handle(context.Background(), pluginapi.ManagementRequest{Method: "GET", Path: path, Query: url.Values{"id": {id}}})
	}
	var list struct{ Items []Question }
	if resp := read("/questions", ""); resp.StatusCode != 200 || json.Unmarshal(resp.Body, &list) != nil || len(list.Items) == 0 {
		t.Fatal("question directory unavailable")
	}
	for _, q := range list.Items {
		if q.Answer != "" {
			t.Fatal("answer disclosed in the general catalog", q.ID)
		}
		resp := read("/rules/question-preview", q.ID)
		if q.Kind != "logic" {
			if resp.StatusCode != 404 {
				t.Fatal("fabricated reference for", q.ID)
			}
			continue
		}
		var reference Question
		frozen, err := FreezeQuestion(RunConfig{QuestionID: q.ID})
		if err != nil || resp.StatusCode != 200 || json.Unmarshal(resp.Body, &reference) != nil || reference != frozen || reference.Answer == "" || reference.Version != q.Version || reference.Prompt != q.Prompt {
			t.Fatal("reference differs from the actual frozen question", q.ID)
		}
	}
	if read("/rules/question-preview", "absent").StatusCode != 404 || len(en.runs) != 0 || h.executeCalls != 0 {
		t.Fatal("preview created work or accepted an unknown reference")
	}
}

// Assert the requested value at the SDK boundary, independently of a provider's later translation.
func TestRuleEffortSDKPayload(t *testing.T) {
	for _, level := range []string{"low", "medium", "high", "xhigh", "max"} {
		for _, stream := range []bool{false, true} {
			en, _, rule := scopeFixture(t, scopedTarget("one", "a", "b"))
			draft := Rule{Name: "payload", TargetScope: scopeAll, RunConfig: rule.RunConfig, Enabled: true}
			draft.Parameters.ReasoningEffort, draft.Parameters.Stream = level, stream
			saved, err := en.SaveRule(context.Background(), draft, false)
			if err != nil {
				t.Fatal(err)
			}
			d, err := en.dispatchRule(context.Background(), saved.ID, saved.Revision, false)
			if err != nil || d.Count != 1 {
				t.Fatal(d, err)
			}
			for _, r := range en.runs {
				f := r.Config
				if f.Parameters.ReasoningEffort != level || len(f.Models) != 2 {
					t.Fatal("lost frozen rule settings", f)
				}
				for _, model := range f.Models {
					calls := 0
					host := NativeHost{Call: func(method string, in, _ any) error {
						calls++
						req := in.(pluginapi.HostModelExecutionRequest)
						var payload struct {
							Model  string `json:"model"`
							Effort string `json:"reasoning_effort"`
						}
						if json.Unmarshal(req.Body, &payload) != nil || payload.Effort != level || payload.Model != model || req.Stream != stream || req.AuthID != "one" || req.ForcedProvider != "offline" || (method == "host.model.execute_stream") != stream {
							t.Fatal("incorrect SDK request", method, payload)
						}
						return context.Canceled
					}}
					s := host.Execute(context.Background(), Execution{AuthID: f.AuthID, Provider: f.Provider, Model: model, Prompt: f.Question.Prompt, Parameters: f.Parameters})
					if calls != 1 || s.Evidence.Verified || s.Comparable {
						t.Fatal("a request is not proof of its effective settings")
					}
				}
			}
		}
	}
}
