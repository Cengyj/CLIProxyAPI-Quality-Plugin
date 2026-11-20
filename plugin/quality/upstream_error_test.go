package quality

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MoeForge/moego/json"
)

func TestManagedUpstreamCondition(t *testing.T) {
	const model = "test-model"
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"401", 401, "", "unauthorized"},
		{"402", 402, "<html>payment required</html>", "payment_required"},
		{"quota code", 429, `{"error":{"type":"invalid_request_error","code":"insufficient_quota","message":"billing"}}`, "quota_exhausted"},
		{"codex usage type", 429, `{"error":{"type":"usage_limit_reached","resets_in_seconds":60}}`, "quota_exhausted"},
		{"quota message", 429, `{"error":{"message":"You exceeded your current quota, please check your plan and billing details."}}`, "quota_exhausted"},
		{"plain quota", 429, "Quota exhausted.", "quota_exhausted"},
		{"model code", 400, `{"error":{"code":"model_not_supported"}}`, "model_unsupported"},
		{"model type", 404, `{"error":{"type":"model_not_found"}}`, "model_unsupported"},
		{"flat model", 404, `{"code":"model_not_found","message":"Not Found"}`, "model_unsupported"},
		{"openai model", 404, "{\"error\":{\"message\":\"The model `test-model` does not exist or you do not have access to it.\"}}", "model_unsupported"},
		{"codex exact model", 400, `{"error":{"message":"The 'test-model' model is not supported when using Codex with a ChatGPT account."}}`, "model_unsupported"},
		{"codex top-level detail", 400, `{"detail":"The 'test-model' model is not supported when using Codex with a ChatGPT account."}`, "model_unsupported"},
		{"codex detail other model", 400, `{"detail":"The 'another-model' model is not supported when using Codex with a ChatGPT account."}`, ""},
		{"model code conflicts with field", 404, `{"error":{"code":"model_not_found","model":"other-model"}}`, ""},
		{"model code conflicts with message", 400, `{"error":{"code":"unsupported_model","message":"The model 'other-model' is not supported."}}`, ""},
		{"transient", 429, `{"error":{"code":"rate_limit_exceeded","message":"Rate limit reached for tokens per minute"}}`, ""},
		{"conflicting limit code", 429, `{"error":{"type":"usage_limit_reached","code":"rate_limit_exceeded"}}`, ""},
		{"conflicting limit message", 429, `{"error":{"type":"quota_exceeded","message":"Your requests per minute quota was exceeded."}}`, ""},
		{"model-specific message", 429, `{"error":{"code":"quota_exceeded","message":"Daily quota exceeded for this model."}}`, ""},
		{"model scope", 429, `{"error":{"code":"quota_exceeded","message":"Quota exceeded","metadata":{"scope":"per_model"}}}`, ""},
		{"model field", 429, `{"error":{"code":"quota_exceeded","message":"Quota exceeded","model":"test-model"}}`, ""},
		{"TPM message", 429, `{"error":{"code":"quota_exceeded","message":"TPM exhausted."}}`, ""},
		{"RPM metric", 429, `{"error":{"code":"quota_exceeded","limit":{"metric":"RPM"}}}`, ""},
		{"retry field", 429, `{"error":{"code":"quota_exceeded","message":"Quota exceeded"},"retry_after":2}`, ""},
		{"retry message", 429, `{"error":{"type":"insufficient_quota","message":"Please try again later"}}`, ""},
		{"generic", 429, `{"message":"Too many requests"}`, ""},
		{"incidental quota", 429, `{"message":"Unable to check whether your quota was exhausted","id":"quota_exhausted"}`, ""},
		{"invalid parameter", 400, `{"error":{"type":"invalid_request_error","message":"This model does not support temperature"}}`, ""},
		{"wrong model", 400, `{"error":{"message":"The model other-model is not supported."}}`, ""},
		{"generic 404", 404, `{"error":{"message":"Not Found","id":"model_not_found"}}`, ""},
		{"wrong status", 503, `{"error":{"code":"insufficient_quota"}}`, ""},
		{"duplicate code", 429, `{"error":{"code":"insufficient_quota","code":"rate_limit_exceeded"}}`, ""},
		{"non-string code", 400, `{"error":{"code":["model_not_found"],"message":"Unknown model"}}`, ""},
		{"oversize", 429, `{"error":{"code":"insufficient_quota","message":"` + strings.Repeat("m", 16<<10) + `"}}`, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := managedUpstreamCondition(tc.status, tc.body, model); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestManagedUpstreamConditionRequiresVerifiedProbe(t *testing.T) {
	const sensitive = "response-body-must-remain-private"
	for _, mode := range []string{"verified", "outer_401", "outer_500", "replaced", "cancelled", "oversize"} {
		t.Run(mode, func(t *testing.T) {
			f := managementTestFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			snapshot, err := f.host.CredentialSnapshot(ctx, "one", "one")
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/v8/management/requests/api-call" {
					t.Error("unexpected management request")
				}
				body := `{"error":{"code":"insufficient_quota","message":"` + sensitive + `"}}`
				switch mode {
				case "outer_401":
					w.WriteHeader(401)
				case "outer_500":
					w.WriteHeader(500)
				case "replaced":
					f.mu.Lock()
					f.changedContent = true
					f.mu.Unlock()
				case "oversize":
					body += strings.Repeat(" ", 2048)
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"status_code": 429, "body": body})
			}))
			t.Cleanup(server.Close)
			f.host.Management, err = NewManagementClient(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			f.host.Management.authorize(http.Header{"Authorization": {"Bearer offline-fixture"}})
			if mode == "cancelled" {
				gets := 0
				call := f.host.Call
				f.host.Call = func(method string, in, out any) error {
					err := call(method, in, out)
					if method == "host.auth.get" {
						gets++
						if gets == 2 {
							cancel()
						}
					}
					return err
				}
			}
			s := f.host.Execute(ctx, Execution{AuthID: "one", Provider: "codex", Model: "a", Prompt: "synthetic",
				ResponseLimit: 1024, ManagedProbe: &snapshot})
			if s.ExecutionStatus == "completed" || s.QualityVerdict != "unknown" || s.Comparable || s.Evidence.Verified {
				t.Fatal("upstream failure became answer evidence", s)
			}
			raw, err := json.Marshal(s)
			if err != nil || bytes.Contains(raw, []byte(sensitive)) {
				t.Fatal("raw provider error entered the sample")
			}
			if mode == "verified" {
				if s.UpstreamCondition != "quota_exhausted" || s.ExecutionStatus != "failed" || s.Error != "upstream HTTP 429" ||
					s.Evidence.AuthID != "one" || s.Evidence.RequestedModel != "a" || s.Evidence.PayloadFingerprint == "" || s.Evidence.ResolvedModel != "" {
					t.Fatal("missing scoped management evidence", s)
				}
			} else if s.UpstreamCondition != "" {
				t.Fatal("unverified condition was classified", s)
			}
			if mode == "cancelled" && s.ExecutionStatus != "cancelled" || mode == "replaced" && !s.Evidence.ConditionsChanged {
				t.Fatal("interrupted verification not reflected in sample", s)
			}
		})
	}
}
