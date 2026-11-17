package login

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	json "github.com/MoeForge/moego/json"
	"github.com/cli-proxy-api/model-quality-login-runtime/internal/transport"
)

const securityNewSeed = "JBSWY3DPEHPK3PXP"

type securityFake struct {
	t     *testing.T
	f     *flow
	mode  string
	calls []string
	reads int
}

func securityInput(t *testing.T, operation string) Input {
	t.Helper()
	in := fixtureInput(authOrigin)
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	in.Operation = operation
	in.SecurityJournalPath = filepath.Join(dir, "security.json")
	return in
}
func securityFlow(t *testing.T, mode, operation string) (*flow, *securityFake) {
	t.Helper()
	in := securityInput(t, operation)
	journal, err := newSecurityJournal(in)
	if err != nil {
		t.Fatal(err)
	}
	f := &flow{in: in, p: policy{auth: authOrigin, chat: chatOrigin, now: func() time.Time { return time.Unix(59, 0) }}, security: journal, device: "fixture-device", session: payload{"user": payload{"email": in.Credentials.Email}, "account": payload{"id": "account-fixture"}, "accessToken": "offline-token"}}
	mock := &securityFake{t: t, f: f, mode: mode}
	f.client = NewClient(mock, nil, "chrome146")
	return f, mock
}
func readSecurityTestJournal(t *testing.T, path string) SecurityJournal {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var j SecurityJournal
	if err = json.Unmarshal(raw, &j); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "offline-token") || strings.Contains(string(raw), "Cookie") {
		t.Fatal("journal persisted web session material")
	}
	return j
}
func (s *securityFake) Do(ctx context.Context, r transport.Request) (*transport.Response, error) {
	s.calls = append(s.calls, r.Method+" "+strings.TrimPrefix(r.URL, chatOrigin))
	path := strings.TrimPrefix(r.URL, chatOrigin)
	j := readSecurityTestJournal(s.t, s.f.security.path)
	if r.Headers.Get("Authorization") != "Bearer offline-token" || r.Headers.Get("Chatgpt-Account-Id") != "account-fixture" {
		s.t.Error("security request lost verified identity")
	}
	if r.Method == http.MethodPost && !r.SingleAttempt {
		s.t.Error("mutation was replayable")
	}
	response := func(body any) *transport.Response {
		raw, err := encodePayload(body)
		if err != nil {
			s.t.Fatal(err)
		}
		return &transport.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: raw, BodyComplete: true}
	}
	factor := func(id string) payload {
		return payload{"mfa_enabled_v2": true, "factors": payload{"totp": []any{payload{"id": id, "factor_type": "totp", "is_recovery": false}}}}
	}
	switch path {
	case mfaInfoEndpoint:
		s.reads++
		if s.reads > 1 {
			switch s.mode {
			case "disable-unknown-resolved":
				return response(payload{"mfa_enabled_v2": false}), nil
			case "activation-unknown-resolved":
				return response(factor("new-factor")), nil
			case "activation-wrong-factor":
				return response(factor("another-factor")), nil
			}
		}
		if s.mode == "blank-factor" {
			return response(factor(" ")), nil
		}
		if s.mode == "multiple-factors" {
			return response(payload{"mfa_enabled_v2": true, "factors": payload{"totp": []any{payload{"id": "old-factor", "factor_type": "totp", "is_recovery": false}, payload{"id": "other-factor", "factor_type": "totp", "is_recovery": false}}}}), nil
		}
		return response(factor("old-factor")), nil
	case disableEndpoint:
		if j.DisableStatus != "pending" || j.OldFactorID != "old-factor" {
			s.t.Error("disable preceded durable identity/intent")
		}
		if s.mode == "disable-unknown" || s.mode == "disable-unknown-resolved" {
			return nil, errors.New("offline connection lost")
		}
		if s.mode == "disable-challenge" {
			r := response(payload{})
			r.StatusCode = 403
			r.Header.Set("Cf-Mitigated", "challenge")
			r.Body = []byte("<html>challenge</html>")
			return r, nil
		}
		if s.mode == "disable-http-rejected" {
			r := response(payload{"detail": "User must re-authenticate to enroll/disable a factor"})
			r.StatusCode = 401
			return r, nil
		}
		if s.mode == "disable-rejected" {
			return response(payload{"success": false}), nil
		}
		return response(payload{"message": "Factor disabled", "passkey_credential_manager_sync": nil}), nil
	case enrollEndpoint:
		if j.DisableStatus != "succeeded" || j.EnrollStatus != "pending" {
			s.t.Error("enroll preceded confirmed removal")
		}
		if s.mode == "enroll-disconnect" {
			return nil, errors.New("offline disconnect")
		}
		id := "new-factor"
		if s.mode == "enroll-same-factor" {
			id = "old-factor"
		}
		sid := "enrollment-session"
		if s.mode == "blank-enrollment" {
			sid = " "
		}
		body := payload{"secret": securityNewSeed, "session_id": sid, "factor": payload{"id": id, "factor_type": "totp", "is_recovery": false}}
		r := response(body)
		if s.mode == "enroll-truncated" {
			r.BodyComplete = false
			return r, errors.New("offline incomplete body")
		}
		return r, nil
	case activateEndpoint:
		if j.ActivationStatus != "pending" || j.EnrollStatus != "succeeded" || j.NewTOTPSecret != securityNewSeed || j.NewFactorID != "new-factor" || j.EnrollmentSessionID != "enrollment-session" {
			s.t.Error("activation preceded durable new seed")
		}
		code, _ := generateTOTP(securityNewSeed, time.Unix(59, 0))
		var body payload
		if json.Unmarshal(r.Body, &body) != nil || body["code"] != code || body["source"] != "settings" {
			s.t.Error("incorrect activation payload")
		}
		if s.mode == "activation-unknown-resolved" || s.mode == "activation-wrong-factor" {
			return nil, errors.New("offline disconnect")
		}
		if s.mode == "activation-rejected" {
			return response(payload{"success": false}), nil
		}
		return response(payload{"activated": true, "factor_id": "new-factor", "session_id": "enrollment-session"}), nil
	case logoutEndpoint:
		if j.LogoutStatus != "pending" {
			s.t.Error("logout preceded durable intent")
		}
		if j.Operation == "reset_totp" && (j.ActivationStatus != "succeeded" || j.NewTOTPSecret != securityNewSeed) {
			s.t.Error("logout preceded confirmed durable seed")
		}
		if s.mode == "logout-disconnect" {
			return nil, errors.New("offline disconnect")
		}
		if s.mode == "logout-500" {
			r := response(payload{})
			r.StatusCode = 500
			return r, nil
		}
		return &transport.Response{StatusCode: 204, Header: make(http.Header), BodyComplete: true}, nil
	}
	s.t.Errorf("unexpected security path %s", path)
	return nil, errors.New("unexpected path")
}

func TestSecurityResetPersistsBeforeMutation(t *testing.T) {
	f, mock := securityFlow(t, "success", "reset_totp")
	f.in.LogoutAfterReset = true
	if err := f.runSecurity(t.Context()); err != nil {
		t.Fatal(err)
	}
	want := []string{"GET " + mfaInfoEndpoint, "POST " + disableEndpoint, "POST " + enrollEndpoint, "POST " + activateEndpoint, "POST " + logoutEndpoint}
	if !reflect.DeepEqual(mock.calls, want) {
		t.Fatalf("unexpected mutation order %#v", mock.calls)
	}
	j := readSecurityTestJournal(t, f.security.path)
	if j.Status != "succeeded" || j.Phase != "security_complete" || j.ActivationStatus != "succeeded" || j.ActivationConfirmation != "business_receipt" || j.LogoutStatus != "succeeded" {
		t.Fatalf("completion evidence missing: %+v", j)
	}
	if _, err := newSecurityJournal(f.in); err == nil {
		t.Fatal("existing journal allowed operation replay")
	}
}
func TestSecurityResetUncertainNeverReplays(t *testing.T) {
	cases := []struct {
		mode, step, state string
		reads, posts      int
		success           bool
	}{
		{"disable-unknown", "totp_disable", "unknown", 4, 1, false}, {"disable-unknown-resolved", "security_complete", "succeeded", 2, 3, true},
		{"disable-rejected", "totp_disable", "failed", 1, 1, false}, {"disable-http-rejected", "totp_disable", "failed", 1, 1, false}, {"disable-challenge", "totp_disable", "unknown", 4, 1, false}, {"enroll-disconnect", "totp_enroll", "unknown", 1, 2, false},
		{"enroll-truncated", "totp_enroll", "unknown", 1, 2, false}, {"enroll-same-factor", "totp_enroll", "unknown", 1, 2, false},
		{"activation-unknown-resolved", "security_complete", "succeeded", 2, 3, true}, {"activation-wrong-factor", "totp_activate", "unknown", 4, 3, false},
		{"activation-rejected", "totp_activate", "failed", 1, 3, false}, {"multiple-factors", "security_check", "failed", 1, 0, false}, {"blank-factor", "security_check", "failed", 1, 0, false}, {"blank-enrollment", "totp_enroll", "unknown", 1, 2, false},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			f, mock := securityFlow(t, tc.mode, "reset_totp")
			err := f.runSecurity(t.Context())
			if (err == nil) != tc.success {
				t.Fatalf("unexpected error %v", err)
			}
			if err != nil {
				if saveErr := f.security.finishFailure(); saveErr != nil {
					t.Fatal(saveErr)
				}
			}
			posts := 0
			seen := map[string]bool{}
			for _, call := range mock.calls {
				if strings.HasPrefix(call, "POST ") {
					posts++
					if seen[call] {
						t.Fatal("mutation replayed")
					}
					seen[call] = true
				}
			}
			j := readSecurityTestJournal(t, f.security.path)
			if mock.reads != tc.reads || posts != tc.posts || j.Status != tc.state || j.Phase != tc.step {
				t.Fatalf("reads=%d posts=%d status=%s phase=%s", mock.reads, posts, j.Status, j.Phase)
			}
			if tc.mode == "enroll-truncated" && j.NewTOTPSecret != securityNewSeed {
				t.Fatal("received pending seed was discarded")
			}
		})
	}
}
func TestSecurityStandaloneLogoutAndFailures(t *testing.T) {
	for _, mode := range []string{"success", "logout-disconnect", "logout-500"} {
		t.Run(mode, func(t *testing.T) {
			f, mock := securityFlow(t, mode, "logout_all")
			err := f.runSecurity(t.Context())
			if (err == nil) != (mode == "success") {
				t.Fatalf("unexpected error %v", err)
			}
			if len(mock.calls) != 1 || mock.calls[0] != "POST "+logoutEndpoint {
				t.Fatal(mock.calls)
			}
			if f.security.NewTOTPSecret != "" {
				t.Fatal("logout altered TOTP")
			}
		})
	}
}
func TestSecurityIdentityBlocksEveryMutation(t *testing.T) {
	for _, mode := range []string{"email", "missing-account", "conflicting-account", "missing-token", "workspace"} {
		t.Run(mode, func(t *testing.T) {
			f, mock := securityFlow(t, "success", "logout_all")
			switch mode {
			case "email":
				f.session["user"] = payload{"email": "other@example.test"}
			case "missing-account":
				delete(f.session, "account")
			case "conflicting-account":
				f.session["account_id"] = "other"
			case "workspace":
				f.in.WorkspaceID = "another-account"
			case "missing-token":
				delete(f.session, "accessToken")
			}
			if f.runSecurity(t.Context()) == nil || len(mock.calls) != 0 {
				t.Fatal("unverified identity authorized mutation")
			}
		})
	}
}
func TestSecurityJournalFailureBlocksNextMutation(t *testing.T) {
	f, mock := securityFlow(t, "success", "reset_totp")
	if err := os.WriteFile(f.security.path, []byte(`{"changed":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if f.runSecurity(t.Context()) == nil || len(mock.calls) != 0 {
		t.Fatal("modified journal authorized mutation")
	}
}
func TestSecurityLogoutReceiptCompatibility(t *testing.T) {
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{"", true}, {"null", true}, {"[]", true}, {`{"success":true}`, true}, {`{"success":false}`, false}, {`{"error":{}}`, false}, {`{"error":0}`, true}, {`{"success":true,"success":false}`, false}, {`<html>login</html>`, false}, {`{"`, false},
	} {
		r := &transport.Response{StatusCode: 200, Header: make(http.Header), Body: []byte(tc.body), BodyComplete: true}
		if logoutResponseValid(r) != tc.valid {
			t.Errorf("unexpected result for %q", tc.body)
		}
	}
}

func TestNativeSecurityFlowAndFreshReauthorization(t *testing.T) {
	for _, operation := range []string{"reset_totp", "reset_totp_reauthorize", "logout_all"} {
		t.Run(operation, func(t *testing.T) {
			fixture := newLoginFixture(t, "")
			op := strings.TrimSuffix(operation, "_reauthorize")
			in := securityInput(t, op)
			fixture.in.Operation = op
			fixture.in.SecurityJournalPath = in.SecurityJournalPath
			fixture.in.Reauthorize = operation != "reset_totp"
			mockFlow := &flow{in: fixture.in, security: &securityJournal{path: in.SecurityJournalPath}}
			mock := &securityFake{t: t, f: mockFlow, mode: "success"}
			original := fixture.server.Config.Handler
			sessions, signins := 0, 0
			fixture.server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/auth/signin/openai" {
					signins++
				}
				if r.URL.Path == "/api/accounts/mfa/verify" && operation == "reset_totp_reauthorize" && signins > 1 {
					raw, _ := io.ReadAll(r.Body)
					var body payload
					code, _ := generateTOTP(securityNewSeed, fixture.p.now())
					if json.Unmarshal(raw, &body) != nil || body["code"] != code {
						t.Error("reauthorization did not use confirmed new seed")
					}
					response, _ := encodePayload(payload{"continue_url": fixture.server.URL + "/api/auth/callback/openai?code=web-code"})
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(response)
					return
				}
				if r.URL.Path == "/api/auth/session" {
					sessions++
					raw, err := encodePayload(payload{"user": payload{"email": fixture.in.Credentials.Email}, "account": payload{"id": "account-fixture"}, "accessToken": "offline-token"})
					if err != nil {
						t.Error(err)
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(raw)
					return
				}
				if strings.HasPrefix(r.URL.Path, "/backend-api/accounts/") {
					raw, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
					}
					response, err := mock.Do(r.Context(), transport.Request{Method: r.Method, URL: chatOrigin + r.URL.Path, Headers: r.Header, Body: raw, SingleAttempt: r.Method == http.MethodPost})
					if err != nil {
						t.Error(err)
						w.WriteHeader(500)
						return
					}
					for key, values := range response.Header {
						w.Header()[key] = values
					}
					w.WriteHeader(response.StatusCode)
					_, _ = w.Write(response.Body)
					return
				}
				original.ServeHTTP(w, r)
			})
			callback, err := run(t.Context(), fixture.in, nil, fixture.p)
			if err != nil {
				t.Fatal(err)
			}
			wantLogins := 1
			if fixture.in.Reauthorize {
				wantLogins = 2
			}
			wantSessions := wantLogins
			if op == "reset_totp" {
				wantSessions++
			}
			if sessions != wantSessions || signins != wantLogins {
				t.Fatalf("fresh login sessions=%d starts=%d want=%d", sessions, signins, wantLogins)
			}
			if (callback != "") != fixture.in.Reauthorize {
				t.Fatal("unexpected OAuth continuation")
			}
			j := readSecurityTestJournal(t, in.SecurityJournalPath)
			if j.Status != "succeeded" || j.Phase != "security_complete" {
				t.Fatal("missing durable completion")
			}
		})
	}
}

func TestNativeSecurityLoginFailurePreservesUnsentEvidence(t *testing.T) {
	fixture := newLoginFixture(t, "password-rejected")
	in := securityInput(t, "reset_totp")
	fixture.in.Operation = in.Operation
	fixture.in.SecurityJournalPath = in.SecurityJournalPath
	if _, err := run(t.Context(), fixture.in, nil, fixture.p); err == nil {
		t.Fatal("login denial ignored")
	}
	j := readSecurityTestJournal(t, in.SecurityJournalPath)
	if j.Status != "failed" || j.DisableStatus != "not_requested" || j.EnrollStatus != "not_requested" {
		t.Fatal("unsent failure became ambiguous")
	}
}

func TestSecurityJournalRejectsUnsafeStorage(t *testing.T) {
	for _, mode := range []string{"public-directory", "symlink-file", "symlink-directory"} {
		t.Run(mode, func(t *testing.T) {
			in := securityInput(t, "reset_totp")
			switch mode {
			case "public-directory":
				if err := os.Chmod(filepath.Dir(in.SecurityJournalPath), 0755); err != nil {
					t.Fatal(err)
				}
			case "symlink-file":
				target := filepath.Join(filepath.Dir(in.SecurityJournalPath), "target.json")
				if err := os.WriteFile(target, []byte(`{}`), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, in.SecurityJournalPath); err != nil {
					t.Fatal(err)
				}
			case "symlink-directory":
				dir := filepath.Dir(in.SecurityJournalPath)
				link := filepath.Join(t.TempDir(), "linked")
				if err := os.Symlink(dir, link); err != nil {
					t.Fatal(err)
				}
				in.SecurityJournalPath = filepath.Join(link, "security.json")
			}
			if _, err := newSecurityJournal(in); err == nil {
				t.Fatal("unsafe journal storage accepted")
			}
		})
	}
}
func TestSecurityPageTokenRequiresVerifiedIdentity(t *testing.T) {
	f, mock := securityFlow(t, "success", "logout_all")
	delete(f.session, "accessToken")
	f.securityPageToken = securityPageAccessToken(`<script>{"accessToken":"offline-page-token-material"}</script>`)
	token, account, err := f.securityAuthorization()
	if err != nil || token != "offline-page-token-material" || account != "account-fixture" {
		t.Fatal("page fallback rejected confirmed identity")
	}
	f.session["user"] = payload{"email": "wrong@example.test"}
	if _, _, err = f.securityAuthorization(); err == nil || len(mock.calls) != 0 {
		t.Fatal("page token bypassed identity")
	}
}
