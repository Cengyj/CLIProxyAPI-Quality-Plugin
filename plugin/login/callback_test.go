package login

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	codex "github.com/cliproxyapi-plugins/model-quality/login/oauth"
)

func TestCallbackProviderMetadataAcceptedAcrossBoundaries(t *testing.T) {
	for _, extra := range []string{
		"&provider_field=retained",
		"&iss=https%3A%2F%2Fauth.openai.com",
		"&scope=openid+email+profile&session_state=fixture-session",
		"&provider_field=one&provider_field=two",
	} {
		raw := offlineCallback("fixture-state") + extra
		request := RuntimeRequest{Version: 1, JobID: "fixture", Attempt: 1, State: "fixture-state", RedirectURI: codex.RedirectURI}
		result, err := Encode(map[string]any{"version": 1, "job_id": request.JobID, "attempt": request.Attempt, "callback_url": raw})
		if err != nil {
			t.Fatal(err)
		}
		callback, err := runtimeResponse(result, request, nil)
		if err != nil || callback != raw {
			t.Fatal("runtime boundary rejected optional provider metadata")
		}
		code, err := validateCallback(callback, request.State)
		if err != nil || code != "OFFLINE_CODE_SECRET" {
			t.Fatal("plugin boundary rejected the runtime-accepted callback")
		}
	}
}

func TestCallbackRejectsInvalidBindingAndAmbiguousParameters(t *testing.T) {
	good := offlineCallback("fixture-state")
	for name, raw := range map[string]string{
		"wrong_state":            offlineCallback("other-state"),
		"duplicate_state":        good + "&state=fixture-state",
		"duplicate_code":         good + "&code=other-code",
		"encoded_duplicate_code": good + "&%63ode=other-code",
		"duplicate_scope":        good + "&scope=one&scope=two",
		"duplicate_error":        good + "&error=&error=access_denied",
		"duplicate_description":  good + "&error_description=one&error_description=two",
		"empty_error":            good + "&error=",
		"error_with_code":        good + "&error=access_denied",
		"malformed_extra":        good + "&provider_field=%zz",
		"semicolon":              good + ";provider_field=ignored",
		"fragment":               good + "#state=other",
		"userinfo":               strings.Replace(good, "//localhost", "//user:private-password@localhost", 1),
		"wrong_host":             strings.Replace(good, "localhost", "example.invalid", 1),
		"wrong_port":             strings.Replace(good, ":1455", ":1456", 1),
		"wrong_scheme":           strings.Replace(good, "http:", "https:", 1),
		"encoded_path":           strings.Replace(good, "/auth/callback", "/auth/%63allback", 1),
		"wrong_path":             strings.Replace(good, "/auth/callback", "/auth/other", 1),
		"missing_state":          codex.RedirectURI + "?code=private-code",
		"missing_code":           codex.RedirectURI + "?state=fixture-state",
		"empty_code":             codex.RedirectURI + "?state=fixture-state&code=",
		"oversized_code":         codex.RedirectURI + "?state=fixture-state&code=" + strings.Repeat("x", 4097),
		"oversized_url":          good + "&provider_field=" + strings.Repeat("x", 8192),
	} {
		t.Run(name, func(t *testing.T) {
			if code, err := validateCallback(raw, "fixture-state"); code != "" || err == nil {
				t.Fatal("invalid callback was accepted")
			} else if strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "SECRET") {
				t.Fatal("callback error exposed private input")
			}
			response, err := Encode(map[string]any{"version": 1, "job_id": "fixture", "attempt": 1, "callback_url": raw})
			if err != nil {
				t.Fatal(err)
			}
			if callback, err := runtimeResponse(response, RuntimeRequest{Version: 1, JobID: "fixture", Attempt: 1, State: "fixture-state", RedirectURI: codex.RedirectURI}, nil); callback != "" || err == nil {
				t.Fatal("runtime and service callback validation diverged")
			}
		})
	}
	if _, err := validateCallback(good, ""); err == nil {
		t.Fatal("empty expected state was accepted")
	}
}

func TestAccountInvalidCallbackRetainsSafeDiagnosisWithoutExchange(t *testing.T) {
	for _, tc := range []struct{ suffix, reason string }{
		{"&state=other", "callback_parameters_invalid"},
		{"&provider_field=%zz", "callback_parameters_invalid"},
	} {
		t.Run(tc.reason+tc.suffix, func(t *testing.T) {
			opts := offlineOptions(t)
			host := newFakeHost()
			opts.Call = host.call
			var exchanged atomic.Bool
			opts.Exchange = func(context.Context, string, *codex.PKCECodes, string) (*codex.CodexAuthBundle, error) {
				exchanged.Store(true)
				return nil, errors.New("unexpected exchange")
			}
			opts.Runtime = offlineRuntime{run: func(_ context.Context, request RuntimeRequest) (string, error) {
				return offlineCallback(request.State) + tc.suffix, nil
			}}
			s := offlineService(t, opts)
			a := importOfflineAccount(t, s)
			job := startOfflineAccount(t, s, a, "", "failed")
			if job.ErrorCode != "invalid_callback" || job.ErrorStage != "callback" || job.ErrorReason != tc.reason || exchanged.Load() || host.saves != 0 {
				t.Fatal("invalid callback lost its diagnosis or reached token exchange")
			}
			logs, err := s.Logs(job.ID)
			if err != nil || len(logs.Entries) == 0 {
				t.Fatal("callback failure log unavailable")
			}
			last := logs.Entries[len(logs.Entries)-1]
			if last.ErrorCode != "invalid_callback" || last.ErrorReason != tc.reason || last.ErrorStage != "callback" {
				t.Fatal("callback diagnosis did not survive durable log projection")
			}
		})
	}
}

func TestAccountCallbackProviderMetadataCommitsExactlyOnce(t *testing.T) {
	opts := offlineOptions(t)
	host := newFakeHost()
	opts.Call = host.call
	var exchanges atomic.Int32
	exchange := opts.Exchange
	opts.Exchange = func(ctx context.Context, code string, pkce *codex.PKCECodes, proxy string) (*codex.CodexAuthBundle, error) {
		exchanges.Add(1)
		if code != "OFFLINE_CODE_SECRET" {
			t.Error("provider metadata changed the authorization code")
		}
		return exchange(ctx, code, pkce, proxy)
	}
	var callback string
	opts.Runtime = offlineRuntime{run: func(_ context.Context, request RuntimeRequest) (string, error) {
		callback = offlineCallback(request.State) + "&iss=" + url.QueryEscape("https://auth.openai.com") + "&provider_field=retained"
		raw, err := Encode(map[string]any{"version": 1, "job_id": request.JobID, "attempt": request.Attempt, "callback_url": callback})
		if err != nil {
			return "", err
		}
		return runtimeResponse(raw, request, nil)
	}}
	s := offlineService(t, opts)
	a := importOfflineAccount(t, s)
	job := startOfflineAccount(t, s, a, "", "committed")
	if !job.CredentialEnabled || exchanges.Load() != 1 || host.saves != 1 {
		t.Fatal("one-click login did not save and enable exactly once")
	}
	if _, err := s.Callback(job.ID, callback); err == nil || err.Error() != "already_consumed" {
		t.Fatal("completed callback was replayed")
	}
	if exchanges.Load() != 1 || host.saves != 1 {
		t.Fatal("callback replay repeated token exchange or saving")
	}
	logs, err := s.Logs(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Encode(logs)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"OFFLINE_CODE_SECRET", "OFFLINE_REFRESH_SECRET", "provider_field", "code_verifier"} {
		if strings.Contains(string(raw), secret) {
			t.Fatal("callback metadata or credentials escaped into logs")
		}
	}
}

func TestCallbackAuthorizationDenied(t *testing.T) {
	_, err := validateCallback(codex.RedirectURI+"?state=fixture-state&error=access_denied&error_description=private-description&provider_field=retained", "fixture-state")
	if err == nil || err.Error() != "interaction_required" {
		t.Fatal("authorization denial was not handled without exposing its description")
	}
	var diagnostic *runtimeDiagnosticError
	if errors.As(err, &diagnostic) && diagnostic.stage != "callback" {
		t.Fatal("callback diagnostic used the wrong stage")
	}
}
