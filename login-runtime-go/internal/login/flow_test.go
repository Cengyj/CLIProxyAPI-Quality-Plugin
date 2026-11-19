package login

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	json "github.com/MoeForge/moego/json"
	"github.com/cli-proxy-api/model-quality-login-runtime/internal/jshost"
)

func TestMain(m *testing.M) {
	if handled, code := jshost.WorkerMain(os.Args[1:], os.Stdin, os.Stdout); handled {
		os.Exit(code)
	}
	os.Exit(m.Run())
}

// The fixture uses the actual native HTTP transport and the actual Sobek worker.
// Only the remote server and official SDK source are replaced with local data.
type loginFixture struct {
	t            *testing.T
	server       *httptest.Server
	in           Input
	p            policy
	mode         string
	mu           sync.Mutex
	paths        []string
	identities   []string
	device       string
	issued       bool
	oauthStarted bool
	workspace    *workspaceFixture
	workspaceWeb bool
}

type workspaceFixture struct {
	items                                              []any
	want                                               string
	pathOnly, omitContinuation, directCallback, repeat bool
}

func newLoginFixture(t *testing.T, mode string) *loginFixture {
	t.Helper()
	f := &loginFixture{t: t, mode: mode}
	f.server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.server.Close)
	f.in = fixtureInput(f.server.URL)
	f.in.RedirectURI = f.server.URL + "/auth/callback"
	authorization, _ := url.Parse(f.in.AuthorizationURL)
	query := authorization.Query()
	query.Set("redirect_uri", f.in.RedirectURI)
	authorization.RawQuery = query.Encode()
	f.in.AuthorizationURL = authorization.String()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	f.p = policy{auth: f.server.URL, chat: f.server.URL, sdk: f.server.URL + "/loader.js", worker: executable, now: func() time.Time { return time.Unix(59, 0) }}
	return f
}

func fixtureInput(origin string) Input {
	in := Input{Version: 1, JobID: "offline-fixture", Attempt: 1, RedirectURI: "http://localhost:1455/auth/callback", State: "host-owned-state", Credentials: Credentials{Email: "fixture@example.com", Password: "offline-password", TOTPSecret: "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"}, TimeoutSeconds: 20}
	q := url.Values{"client_id": {clientID}, "redirect_uri": {in.RedirectURI}, "state": {in.State}, "response_type": {"code"}, "scope": {"openid profile email offline_access"}, "code_challenge_method": {"S256"}, "code_challenge": {strings.Repeat("a", 43)}}
	in.AuthorizationURL = origin + "/oauth/authorize?" + q.Encode()
	return in
}

func (f *loginFixture) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.paths = append(f.paths, r.Method+" "+r.URL.Path)
	f.identities = append(f.identities, r.Header.Get("User-Agent"))
	out := func(v any) {
		w.Header().Set("Content-Type", "application/json")
		b, err := encodePayload(v)
		if err != nil {
			f.t.Error(err)
		}
		_, _ = w.Write(b)
	}
	expect := func(want payload) {
		var got payload
		b, err := io.ReadAll(r.Body)
		if err != nil || json.Unmarshal(b, &got) != nil || !reflect.DeepEqual(got, want) {
			f.t.Errorf("unexpected fixture request body at %s", r.URL.Path)
		}
	}
	checkSentinel := func(ref string) {
		var token payload
		if json.Unmarshal([]byte(r.Header.Get("Openai-Sentinel-Token")), &token) != nil || token["flow"] != "password_verify" || token["id"] != f.device || token["c"] != "fixture-challenge" {
			f.t.Errorf("missing dynamic SDK token at %s", r.URL.Path)
		}
		if r.Header.Get("Referer") != f.server.URL+ref || r.Header.Get("Origin") != f.server.URL {
			f.t.Errorf("incorrect auth request origin/referer at %s", r.URL.Path)
		}
		if cookie, err := r.Cookie("sdk-session"); err != nil || cookie.Value != "shared" {
			f.t.Errorf("SDK and protocol request did not share their cookie jar at %s", r.URL.Path)
		}
	}
	customWorkspace := f.workspace != nil && f.workspaceWeb != f.oauthStarted
	workspaceResponse := func() payload {
		items := []any{payload{"id": "workspace-fixture"}}
		next := f.server.URL + "/workspace"
		page := "workspace"
		if customWorkspace {
			items = f.workspace.items
			if f.workspace.pathOnly {
				page = "external_url"
			}
			if f.workspace.omitContinuation {
				next = ""
			}
			if f.workspace.directCallback {
				next = f.in.RedirectURI + "?code=fixture-code&state=" + f.in.State
				if !f.oauthStarted {
					next = f.server.URL + "/api/auth/callback/openai?code=web-code"
				}
			}
		}
		return payload{"page": payload{"type": page}, "continue_url": next, "oai-client-auth-session": payload{"workspaces": items}}
	}
	switch r.URL.Path {
	case "/":
		http.SetCookie(w, &http.Cookie{Name: "web-session", Value: "shared", Path: "/", HttpOnly: true})
		_, _ = io.WriteString(w, "<!doctype html><html>fixture root</html>")
	case "/api/auth/providers":
		out(payload{"openai": payload{"id": "openai"}})
	case "/api/auth/csrf":
		out(payload{"csrfToken": "fixture-csrf"})
	case "/api/auth/signin/openai":
		_ = r.ParseForm()
		if r.PostForm.Get("csrfToken") != "fixture-csrf" || r.PostForm.Get("callbackUrl") != f.server.URL+"/" || r.URL.Query().Get("login_hint") != f.in.Credentials.Email {
			f.t.Error("signin did not retain the supplied account and CSRF context")
		}
		f.device = r.URL.Query().Get("ext-oai-did")
		if f.device == "" {
			f.t.Error("signin device missing")
		}
		out(payload{"url": f.server.URL + "/authorize"})
	case "/authorize":
		http.Redirect(w, r, f.server.URL+"/log-in/password", http.StatusFound)
	case "/log-in/password":
		_, _ = io.WriteString(w, "<!doctype html><html>fixture password page</html>")
	case "/loader.js":
		_, _ = fmt.Fprintf(w, `var s=document.createElement("script");s.src=%q;document.head.appendChild(s);`, f.server.URL+"/sentinel/fixture/sdk.js")
	case "/sentinel/fixture/sdk.js":
		_, _ = fmt.Fprintf(w, `window.SentinelSDK={token:async function(flow){const r=await fetch(%q,{method:"POST",body:flow,headers:{"X-Fixture-Page":window.location.href}});const c=await r.text();return JSON.stringify({p:"fixture-proof",t:null,c:c,id:document.cookie.split("oai-did=")[1].split(";")[0],flow:flow});},sessionObserverToken:async function(){return "";}};`, f.server.URL+"/backend-api/sentinel/req")
	case "/backend-api/sentinel/req":
		b, _ := io.ReadAll(r.Body)
		if r.Method != http.MethodPost || string(b) != "password_verify" {
			f.t.Error("Sobek did not issue the fixture SDK request through native HTTP")
		}
		if r.Header.Get("X-Fixture-Page") != f.server.URL+"/log-in/password" {
			f.t.Error("SDK page context diverged from the reference flow mapping")
		}
		if cookie, err := r.Cookie("web-session"); err != nil || cookie.Value != "shared" {
			f.t.Error("SDK lost the web session cookie")
		}
		if cookie, err := r.Cookie("oai-did"); err != nil || cookie.Value != f.device {
			f.t.Error("SDK lost the device cookie")
		}
		http.SetCookie(w, &http.Cookie{Name: "sdk-session", Value: "shared", Path: "/", HttpOnly: true})
		_, _ = io.WriteString(w, "fixture-challenge")
	case "/api/accounts/password/verify":
		expect(payload{"password": f.in.Credentials.Password})
		checkSentinel("/log-in/password")
		if f.mode == "password-disconnect" {
			f.disconnect(w)
			return
		}
		if f.mode == "password-rejected" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			out(payload{"error": payload{"code": "invalid_password", "message": "private upstream text"}})
			return
		}
		if f.mode == "email-required" {
			out(payload{"page": payload{"type": "email_verification"}, "continue_url": f.server.URL + "/email-verification"})
			return
		}
		if f.mode == "untrusted-continuation" {
			out(payload{"continue_url": "https://untrusted.invalid/take-password"})
			return
		}
		out(payload{"page": payload{"type": "mfa_challenge"}, "continue_url": f.server.URL + "/mfa-challenge/factor-fixture", "oai-client-auth-session": payload{"mfa_challenge_factors": []any{payload{"factor_type": "totp", "id": "factor-fixture"}}}})
	case "/api/accounts/mfa/issue_challenge":
		expect(payload{"type": "totp", "id": "factor-fixture", "force_fresh_challenge": false})
		if r.Header.Get("Referer") != f.server.URL+"/log-in/password" {
			f.t.Error("MFA challenge issue changed reference request order")
		}
		f.issued = true
		out(payload{})
	case "/api/accounts/mfa/verify":
		// RFC 6238's time=59 vector, truncated to the service's six digits.
		expect(payload{"type": "totp", "id": "factor-fixture", "code": "287082"})
		checkSentinel("/mfa-challenge/factor-fixture")
		if !f.issued {
			f.t.Error("MFA verification preceded issue_challenge")
		}
		if f.mode == "totp-disconnect" {
			f.disconnect(w)
			return
		}
		if f.mode == "totp-rejected" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			out(payload{"error": payload{"code": "invalid_code", "message": "private upstream text"}})
			return
		}
		if customWorkspace {
			out(workspaceResponse())
			return
		}
		out(payload{"continue_url": f.server.URL + "/api/auth/callback/openai?code=web-code"})
	case "/api/auth/callback/openai":
		http.Redirect(w, r, f.server.URL+"/", http.StatusFound)
	case "/api/auth/session":
		email := f.in.Credentials.Email
		if f.mode == "different-account" {
			email = "another@example.com"
		}
		out(payload{"user": payload{"email": email}})
	case "/oauth/authorize":
		f.oauthStarted = true
		if f.server.URL+r.URL.String() != f.in.AuthorizationURL {
			f.t.Error("native flow changed host-owned OAuth parameters")
		}
		_, _ = io.WriteString(w, `<input name="session_id" value="us_fixture_account_session">`)
	case "/api/accounts/session/select":
		expect(payload{"session_id": "us_fixture_account_session"})
		out(workspaceResponse())
	case "/api/accounts/workspace/select":
		want, referer := "workspace-fixture", f.server.URL+"/workspace"
		if customWorkspace {
			want = f.workspace.want
			if f.workspace.omitContinuation && f.oauthStarted {
				referer = f.server.URL + "/sign-in-with-chatgpt/codex/consent"
			}
		}
		expect(payload{"workspace_id": want})
		if r.Header.Get("Origin") != f.server.URL || r.Header.Get("Referer") != referer {
			f.t.Error("workspace selection lost its origin or continuation context")
		}
		if customWorkspace && f.workspace.repeat {
			out(workspaceResponse())
			return
		}
		if !f.oauthStarted {
			out(payload{"continue_url": f.server.URL + "/api/auth/callback/openai?code=web-code"})
			return
		}
		state := f.in.State
		if f.mode == "wrong-state" {
			state = "untrusted-state"
		}
		out(payload{"continue_url": f.in.RedirectURI + "?code=fixture-code&state=" + state})
	case "/auth/callback":
		f.t.Error("native runtime must return the callback without requesting its one-time code")
		w.WriteHeader(http.StatusBadRequest)
	default:
		f.t.Errorf("unexpected native flow request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *loginFixture) disconnect(w http.ResponseWriter) {
	conn, _, err := w.(http.Hijacker).Hijack()
	if err != nil {
		f.t.Error(err)
		return
	}
	_ = conn.Close()
}

func (f *loginFixture) count(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, actual := range f.paths {
		if actual == path {
			n++
		}
	}
	return n
}

func TestNativeFlowUsesSobekAndReturnsHostBoundCallback(t *testing.T) {
	f := newLoginFixture(t, "success")
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	var stages []string
	callback, err := run(context.Background(), f.in, func(stage string) { stages = append(stages, stage) }, f.p)
	if err != nil {
		var fixed *Error
		_ = errors.As(err, &fixed)
		t.Fatalf("native flow failed: %#v", fixed)
	}
	if callback != f.in.RedirectURI+"?code=fixture-code&state="+f.in.State {
		t.Fatal("callback was not bound to the host authorization")
	}
	wantStages := []string{"runtime_start", "session", "password", "totp", "session", "oauth", "workspace", "callback"}
	if !reflect.DeepEqual(stages, wantStages) {
		t.Fatalf("stages = %v", stages)
	}
	for _, path := range []string{"POST /api/accounts/password/verify", "POST /api/accounts/mfa/issue_challenge", "POST /api/accounts/mfa/verify", "GET /api/auth/callback/openai", "GET /oauth/authorize", "POST /api/accounts/session/select", "POST /api/accounts/workspace/select"} {
		if f.count(path) != 1 {
			t.Errorf("expected exactly one %s", path)
		}
	}
	if f.count("POST /backend-api/sentinel/req") != 2 {
		t.Error("both password and MFA must execute the SDK in Sobek")
	}
	if f.count("GET /auth/callback") != 0 {
		t.Fatal("callback was requested before returning to the host")
	}
	f.mu.Lock()
	for _, identity := range f.identities {
		if identity == "" || identity != f.identities[0] {
			t.Error("web, protocol and SDK did not share one browser identity")
			break
		}
	}
	f.mu.Unlock()
	entries, err := os.ReadDir(tmp)
	if err != nil || len(entries) != 0 {
		t.Fatal("native script worker input survived operation")
	}
}

func TestNativeFlowAutomaticallySelectsReferenceWorkspace(t *testing.T) {
	organization := []any{payload{"id": "personal-fixture", "kind": "personal"}, payload{"id": "workspace-fixture", "kind": "organization"}, payload{"id": "second-organization", "kind": "organization"}}
	cases := []struct {
		name    string
		fixture workspaceFixture
		pin     string
		blocked bool
	}{
		{"organization_before_personal", workspaceFixture{items: organization, want: "workspace-fixture"}, "", false},
		{"first_valid_without_organization", workspaceFixture{items: []any{"invalid", payload{"id": ""}, payload{"id": 123}, payload{"id": "workspace-fixture", "kind": "personal"}, payload{"id": "second-personal", "kind": "personal"}}, want: "workspace-fixture", pathOnly: true}, "", false},
		{"bound_personal_overrides_organization", workspaceFixture{items: []any{payload{"id": "another-organization", "kind": "organization"}, payload{"id": "workspace-fixture", "kind": "personal"}}, want: "workspace-fixture"}, "workspace-fixture", false},
		{"missing_bound_workspace", workspaceFixture{items: organization}, "missing-workspace", true},
		{"no_valid_workspace_ids", workspaceFixture{items: []any{payload{"id": ""}, payload{"id": 123}, "invalid"}}, "", true},
		{"missing_continuation_uses_stage_referer", workspaceFixture{items: organization, want: "workspace-fixture", omitContinuation: true}, "", false},
		{"callback_precedes_stale_workspace_list", workspaceFixture{items: []any{payload{"id": "stale-workspace", "kind": "organization"}}, directCallback: true}, "", false},
	}
	for _, stage := range []string{"web", "oauth"} {
		for _, tc := range cases {
			t.Run(stage+"/"+tc.name, func(t *testing.T) {
				f := newLoginFixture(t, "success")
				f.workspace, f.workspaceWeb = &tc.fixture, stage == "web"
				f.in.WorkspaceID = tc.pin
				callback, err := run(context.Background(), f.in, nil, f.p)
				if tc.blocked {
					var fixed *Error
					if callback != "" || !errors.As(err, &fixed) || fixed.Code != "interaction_required" || fixed.Stage != "workspace" || fixed.Reason != "workspace_selection" {
						t.Fatalf("unavailable workspace did not stop safely: %#v", fixed)
					}
					if f.count("POST /api/accounts/workspace/select") != 0 || stage == "web" && f.count("GET /oauth/authorize") != 0 {
						t.Fatal("unavailable bound workspace fell back to another account")
					}
				} else {
					if err != nil || callback != f.in.RedirectURI+"?code=fixture-code&state="+f.in.State {
						t.Fatalf("automatic workspace flow failed: %v", err)
					}
					selections := 1
					if stage == "web" {
						selections++
					}
					if tc.fixture.directCallback {
						selections--
					}
					if f.count("POST /api/accounts/workspace/select") != selections {
						t.Fatalf("expected %d workspace submissions", selections)
					}
				}
				if f.count("POST /api/accounts/password/verify") != 1 || f.count("POST /api/accounts/mfa/verify") != 1 || f.count("GET /auth/callback") != 0 {
					t.Fatal("workspace continuation replayed credentials or consumed the host callback")
				}
			})
		}
	}
}

func TestNativeFlowBoundsRepeatedWorkspaceResponses(t *testing.T) {
	f := newLoginFixture(t, "success")
	f.workspace = &workspaceFixture{items: []any{payload{"id": "workspace-fixture", "kind": "organization"}}, want: "workspace-fixture", repeat: true}
	callback, err := run(context.Background(), f.in, nil, f.p)
	var fixed *Error
	if callback != "" || !errors.As(err, &fixed) || fixed.Code != "interaction_required" || fixed.Reason != "workspace_selection" {
		t.Fatalf("nonadvancing workspace did not stop: %#v", fixed)
	}
	if selections := f.count("POST /api/accounts/workspace/select"); selections < 1 || selections > 4 {
		t.Fatalf("workspace submissions were not bounded: %d", selections)
	}
	if f.count("POST /api/accounts/password/verify") != 1 || f.count("POST /api/accounts/mfa/verify") != 1 {
		t.Fatal("workspace failure restarted accepted credentials")
	}
}

func TestNativeFlowStopsWithoutReplayOrMismatchedAccount(t *testing.T) {
	cases := []struct{ mode, code, stage, reason string }{
		{"password-rejected", "password_rejected", "password", "upstream_rejected"},
		{"totp-rejected", "totp_rejected", "totp", "upstream_rejected"},
		{"different-account", "interaction_required", "session", "session_unconfirmed"},
		{"wrong-state", "callback_state_mismatch", "workspace", ""},
		{"email-required", "interaction_required", "email", "email_verification"},
		{"untrusted-continuation", "interaction_required", "password", "unsupported_navigation"},
		{"password-disconnect", "transport_error", "password", ""},
		{"totp-disconnect", "transport_error", "totp", ""},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			f := newLoginFixture(t, tc.mode)
			callback, err := run(context.Background(), f.in, nil, f.p)
			var fixed *Error
			if callback != "" || !errors.As(err, &fixed) || fixed.Code != tc.code || fixed.Stage != tc.stage || fixed.Reason != tc.reason {
				t.Fatalf("incorrect fixed outcome: %#v", fixed)
			}
			if err.Error() != tc.code {
				t.Fatal("outward error exposed private upstream text")
			}
			if f.count("POST /api/accounts/password/verify") > 1 || f.count("POST /api/accounts/mfa/verify") > 1 {
				t.Fatal("rejected credential was automatically resubmitted")
			}
			if tc.mode == "different-account" && f.count("GET /oauth/authorize") != 0 {
				t.Fatal("OAuth proceeded after mismatched web account")
			}
		})
	}
}

func TestNativeFlowCancellationDoesNotStartNetwork(t *testing.T) {
	f := newLoginFixture(t, "success")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := run(ctx, f.in, nil, f.p)
	var fixed *Error
	if !errors.As(err, &fixed) || fixed.Code != "cancelled" {
		t.Fatalf("cancelled input returned %#v", fixed)
	}
	if f.count("GET /") != 0 {
		t.Fatal("cancelled input reached the upstream")
	}
}

func TestNativeFlowUsesOneConfiguredProxyForWebAndSDK(t *testing.T) {
	f := newLoginFixture(t, "success")
	var proxyCalls atomic.Int32
	var active sync.WaitGroup
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyCalls.Add(1)
		if r.Header.Get("Proxy-Authorization") == "" {
			t.Error("configured proxy credentials were not supplied")
		}
		if r.Method == http.MethodConnect {
			if r.Host != strings.TrimPrefix(f.server.URL, "http://") {
				t.Error("proxy tunnel used an untrusted destination")
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			upstream, err := net.DialTimeout("tcp", r.Host, time.Second)
			if err != nil {
				t.Error(err)
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			client, buffer, err := w.(http.Hijacker).Hijack()
			if err != nil {
				_ = upstream.Close()
				t.Error(err)
				return
			}
			_, _ = buffer.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n")
			_ = buffer.Flush()
			active.Add(1)
			go func() {
				defer active.Done()
				defer upstream.Close()
				defer client.Close()
				done := make(chan struct{})
				go func() { _, _ = io.Copy(upstream, client); _ = upstream.Close(); close(done) }()
				_, _ = io.Copy(client, upstream)
				_ = client.Close()
				<-done
			}()
			return
		}
		if r.URL.Scheme+"://"+r.URL.Host != f.server.URL {
			t.Error("proxy request used an untrusted destination")
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		r.RequestURI = ""
		r.Header.Del("Proxy-Authorization")
		response, err := http.DefaultTransport.RoundTrip(r)
		if err != nil {
			t.Error(err)
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer response.Body.Close()
		for k, values := range response.Header {
			w.Header()[k] = values
		}
		w.WriteHeader(response.StatusCode)
		_, _ = io.Copy(w, response.Body)
	}))
	defer func() { proxy.Close(); active.Wait() }()
	p, _ := url.Parse(proxy.URL)
	p.User = url.UserPassword("fixture-user", "fixture-password")
	f.in.ProxyURL = p.String()
	callback, err := run(context.Background(), f.in, nil, f.p)
	if err != nil || callback == "" {
		t.Fatalf("native flow through configured proxy failed: %v", err)
	}
	if proxyCalls.Load() < 1 || f.count("POST /backend-api/sentinel/req") != 2 || f.count("POST /api/accounts/mfa/verify") != 1 {
		t.Fatal("configured proxy did not carry the complete native login")
	}
}
