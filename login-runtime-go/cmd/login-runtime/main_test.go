package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/url"
	"runtime"
	"strings"
	"testing"
	"time"

	json "github.com/MoeForge/moego/json"
	"github.com/cli-proxy-api/model-quality-login-runtime/internal/login"
)

const privateFixture = "PRIVATE_PASSWORD_COOKIE_TOTP_762394"

func validInput(t *testing.T) string {
	t.Helper()
	redirect := "http://127.0.0.1:1455/auth/callback"
	query := url.Values{
		"client_id":             {"app_EMoamEEZ73f0CkXaXp7hrann"},
		"redirect_uri":          {redirect},
		"state":                 {"fixture-state"},
		"response_type":         {"code"},
		"code_challenge":        {strings.Repeat("A", 43)},
		"code_challenge_method": {"S256"},
		"scope":                 {"openid profile email offline_access"},
	}
	input := login.Input{
		Version: 1, JobID: "fixture-job", Attempt: 3,
		AuthorizationURL: "https://auth.openai.com/oauth/authorize?" + query.Encode(),
		RedirectURI:      redirect, State: "fixture-state", TimeoutSeconds: 30,
		Credentials: login.Credentials{Email: "fixture@example.test", Password: privateFixture, TOTPSecret: "JBSWY3DPEHPK3PXP"},
	}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func outputEvents(t *testing.T, output string) []map[string]any {
	t.Helper()
	if strings.Contains(output, privateFixture) {
		t.Fatal("IPC output exposed private error or credential text")
	}
	var events []map[string]any
	decoder := json.NewDecoder(strings.NewReader(output))
	for {
		var event map[string]any
		if err := decoder.Decode(&event); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatalf("invalid NDJSON event: %v", err)
		}
		events = append(events, event)
	}
	return events
}

func TestInvalidInputNeverRunsLogin(t *testing.T) {
	valid := validInput(t)
	for _, test := range []struct {
		name string
		raw  string
	}{
		{"duplicate-root", strings.TrimSuffix(valid, "}") + `,"version":1}`},
		{"duplicate-credential", strings.Replace(valid, `"email":`, `"email":"duplicate@example.test","email":`, 1)},
		{"unknown-root", strings.TrimSuffix(valid, "}") + `,"ignored":"` + privateFixture + `"}`},
		{"unknown-credential", strings.Replace(valid, `"email":`, `"ignored":"`+privateFixture+`","email":`, 1)},
		{"malformed", `{"version":`},
		{"trailing-object", valid + "\n{}"},
		{"array", "[" + valid + "]"},
		{"null", "null"},
		{"oversized", valid + strings.Repeat(" ", inputLimit)},
		{"fractional-attempt", strings.Replace(valid, `"attempt":3`, `"attempt":3.5`, 1)},
		{"wrong-credential-type", strings.Replace(valid, `"password":"`+privateFixture+`"`, `"password":true`, 1)},
		{"wrong-version", strings.Replace(valid, `"version":1`, `"version":2`, 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			called := false
			status := run(t.Context(), nil, strings.NewReader(test.raw), &output, func(context.Context, login.Input, func(string)) (string, error) {
				called = true
				return "", nil
			})
			if called || status != 1 {
				t.Fatalf("invalid IPC input ran login=%t, exit=%d", called, status)
			}
			events := outputEvents(t, output.String())
			if len(events) != 1 || events[0]["version"] != float64(1) || events[0]["error_code"] != "invalid_input" {
				t.Fatal("invalid input did not produce one closed diagnostic")
			}
		})
	}
}

func TestProgressAndFinalKeepIdentityAndDeduplicate(t *testing.T) {
	var output bytes.Buffer
	callback := "http://127.0.0.1:1455/auth/callback?code=fixture-code&state=fixture-state"
	status := run(t.Context(), nil, strings.NewReader(validInput(t)), &output, func(ctx context.Context, input login.Input, progress func(string)) (string, error) {
		if input.JobID != "fixture-job" || input.Attempt != 3 || input.Credentials.Password != privateFixture {
			t.Fatal("valid IPC input changed before reaching the login engine")
		}
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 30*time.Second || time.Until(deadline) < 29*time.Second {
			t.Fatal("login engine did not receive the configured deadline")
		}
		for _, phase := range []string{"runtime_start", "runtime_start", privateFixture, "session", "session", "authorization", "totp", "callback"} {
			progress(phase)
		}
		return callback, nil
	})
	if status != 0 {
		t.Fatalf("successful login exit=%d", status)
	}
	events := outputEvents(t, output.String())
	wantPhases := []string{"runtime_start", "session", "authorization", "totp", "callback"}
	if len(events) != len(wantPhases)+1 {
		t.Fatalf("expected deduplicated progress and one final event, got %d", len(events))
	}
	for index, event := range events {
		if event["version"] != float64(1) || event["job_id"] != "fixture-job" || event["attempt"] != float64(3) {
			t.Fatal("IPC event lost its job and attempt identity")
		}
		if index < len(wantPhases) && (event["type"] != "progress" || event["phase"] != wantPhases[index] || len(event) != 5) {
			t.Fatal("progress event carried an unexpected phase or extra fields")
		}
	}
	final := events[len(events)-1]
	if final["callback_url"] != callback || len(final) != 4 {
		t.Fatal("final callback event changed or contained unexpected fields")
	}
}

func TestErrorsAndPanicsUseClosedIdentityBoundDiagnostics(t *testing.T) {
	for _, test := range []struct {
		name, code, stage, reason string
		err                       error
		panicValue                any
	}{
		{name: "private-transport-error", code: "transport_error", err: errors.New(privateFixture)},
		{name: "unknown-typed-error", code: "transport_error", err: &login.Error{Code: privateFixture, Stage: privateFixture, Reason: privateFixture}},
		{name: "known-typed-error", code: "totp_rejected", stage: "totp", reason: "upstream_rejected", err: &login.Error{Code: "totp_rejected", Stage: "totp", Reason: "upstream_rejected"}},
		{name: "cancellation", code: "cancelled", err: context.Canceled},
		{name: "deadline", code: "timeout", err: context.DeadlineExceeded},
		{name: "panic", code: "runtime_unavailable", panicValue: privateFixture},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			status := run(t.Context(), nil, strings.NewReader(validInput(t)), &output, func(context.Context, login.Input, func(string)) (string, error) {
				if test.panicValue != nil {
					panic(test.panicValue)
				}
				return "", test.err
			})
			if status != 1 {
				t.Fatalf("failed login exit=%d", status)
			}
			events := outputEvents(t, output.String())
			if len(events) != 1 {
				t.Fatalf("expected one final failure, got %d", len(events))
			}
			event := events[0]
			if event["version"] != float64(1) || event["job_id"] != "fixture-job" || event["attempt"] != float64(3) || event["error_code"] != test.code {
				t.Fatal("final failure lost its closed code or input identity")
			}
			wantFields := 4
			if test.stage != "" {
				wantFields++
				if event["error_stage"] != test.stage {
					t.Fatal("known error stage was not preserved")
				}
			}
			if test.reason != "" {
				wantFields++
				if event["error_reason"] != test.reason {
					t.Fatal("known error reason was not preserved")
				}
			}
			if len(event) != wantFields {
				t.Fatal("failure event included unrecognized diagnostic fields")
			}
		})
	}
}

func TestCancelledContextReachesRunnerAndProducesCancelledFinal(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var output bytes.Buffer
	status := run(ctx, nil, strings.NewReader(validInput(t)), &output, func(ctx context.Context, _ login.Input, _ func(string)) (string, error) {
		return "", ctx.Err()
	})
	events := outputEvents(t, output.String())
	if status != 1 || len(events) != 1 || events[0]["error_code"] != "cancelled" {
		t.Fatal("parent cancellation was not retained at the IPC boundary")
	}
}

func TestSelfTestExecutesSobekWithoutExternalRuntimes(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	var output bytes.Buffer
	status := run(t.Context(), []string{"--self-test"}, strings.NewReader(""), &output, func(context.Context, login.Input, func(string)) (string, error) {
		t.Fatal("self-test attempted account login")
		return "", nil
	})
	events := outputEvents(t, output.String())
	if status != 0 || len(events) != 1 {
		t.Fatal("Sobek self-test failed without external runtime executables")
	}
	event := events[0]
	if event["runtime_version"] != version || event["engine"] != "go_sobek_protocol" || event["browser_required"] != false || event["node_required"] != false || event["platform"] != runtime.GOOS || event["arch"] != runtime.GOARCH {
		t.Fatal("self-test did not attest the native Go and Sobek runtime")
	}
	methods, ok := event["methods"].([]any)
	if !ok || len(methods) != 1 || methods[0] != "password_totp" {
		t.Fatal("self-test advertised unsupported login methods")
	}
}

func TestSecurityIPCFieldsAndCompletion(t *testing.T) {
	for _, reauthorize := range []bool{false, true} {
		input, err := decodeInput([]byte(validInput(t)))
		if err != nil {
			t.Fatal(err)
		}
		input.Operation = "reset_totp"
		input.LogoutAfterReset = true
		input.Reauthorize = reauthorize
		input.SecurityJournalPath = "/private/tmp/offline-security.json"
		input.TimeoutSeconds = 0
		raw, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		var output bytes.Buffer
		status := run(t.Context(), nil, bytes.NewReader(raw), &output, func(ctx context.Context, in login.Input, progress func(string)) (string, error) {
			if in.Operation != input.Operation || !in.LogoutAfterReset || in.Reauthorize != reauthorize || in.SecurityJournalPath != input.SecurityJournalPath {
				t.Fatal("security IPC discarded operation intent")
			}
			deadline, ok := ctx.Deadline()
			remaining := time.Until(deadline)
			if !ok || remaining < 350*time.Second || remaining > 361*time.Second {
				t.Fatal("security operation did not receive its acquisition budget")
			}
			for _, phase := range []string{"security_check", "totp_disable", "totp_enroll", "totp_activate", "totp_saved", "logout_all", "reauthorization", "security_complete"} {
				progress(phase)
			}
			if reauthorize {
				return "http://127.0.0.1:1455/auth/callback?code=fixture&state=fixture-state", nil
			}
			return "", nil
		})
		if status != 0 {
			t.Fatal("security operation failed")
		}
		events := outputEvents(t, output.String())
		last := events[len(events)-1]
		if last["security_complete"] != true {
			t.Fatal("security completion marker missing")
		}
		_, hasCallback := last["callback_url"]
		if hasCallback != reauthorize {
			t.Fatal("unexpected security callback")
		}
		if len(events) != 9 {
			t.Fatal("security progress phase was dropped")
		}
	}
}

func TestInvalidSecurityIPCStopsBeforeRunner(t *testing.T) {
	input, err := decodeInput([]byte(validInput(t)))
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"unknown-operation", "relative-path", "missing-path", "logout-reset-flag", "login-security-flag"} {
		t.Run(mode, func(t *testing.T) {
			in := input
			in.Operation = "reset_totp"
			in.SecurityJournalPath = "/private/tmp/offline-security.json"
			switch mode {
			case "unknown-operation":
				in.Operation = "reset_password"
			case "relative-path":
				in.SecurityJournalPath = "security.json"
			case "missing-path":
				in.SecurityJournalPath = ""
			case "logout-reset-flag":
				in.Operation = "logout_all"
				in.LogoutAfterReset = true
			case "login-security-flag":
				in.Operation = ""
			}
			raw, _ := json.Marshal(in)
			var output bytes.Buffer
			called := false
			status := run(t.Context(), nil, bytes.NewReader(raw), &output, func(context.Context, login.Input, func(string)) (string, error) { called = true; return "", nil })
			if called || status != 1 {
				t.Fatal("invalid operation reached runner")
			}
		})
	}
}
