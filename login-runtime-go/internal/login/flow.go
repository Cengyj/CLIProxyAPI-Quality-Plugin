package login

import (
	"context"
	"crypto/x509"
	"errors"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/cli-proxy-api/model-quality-login-runtime/internal/legacyruntime"
	"github.com/cli-proxy-api/model-quality-login-runtime/internal/transport"
)

type policy struct {
	auth, chat, sdk, worker string
	roots                   *x509.CertPool
	now                     func() time.Time
}

// Run uses only the native Go transport and Sobek workers. The CPA host retains
// the verifier, token exchange and credential commit; no browser fallback exists.
func Run(ctx context.Context, in Input, progress func(string)) (string, error) {
	return run(ctx, in, progress, policy{auth: authOrigin, chat: chatOrigin, now: time.Now})
}

type flow struct {
	in                Input
	p                 policy
	client            *Client
	engine            *legacyruntime.Client
	stage             string
	progress          func(string)
	device            string
	oauth             bool
	session           payload
	security          *securityJournal
	securityPageToken string
}

func run(ctx context.Context, in Input, progress func(string), p policy) (callback string, err error) {
	if err = validateInput(in, p.auth); err != nil {
		return "", err
	}
	seconds := in.TimeoutSeconds
	if seconds == 0 {
		seconds = 180
		if in.Operation != "" {
			seconds = 360
		}
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
	defer cancel()
	f := &flow{in: in, p: p, progress: progress}
	f.emit("runtime_start")
	if in.Operation != "" {
		f.security, err = newSecurityJournal(in)
		if err != nil {
			return "", err
		}
	}
	defer func() {
		if f.security != nil && err != nil {
			if saveErr := f.security.finishFailure(); saveErr != nil {
				err = saveErr
			}
		}
	}()
	defer func() {
		if err != nil {
			err = f.safeError(ctx, err)
		}
	}()
	proxy, err := normalizeProxy(in.ProxyURL)
	if err != nil {
		return "", err
	}
	opts := legacyruntime.Options{ProxyURL: proxy, Profile: "chrome146", SDKURL: p.sdk, WorkerPath: p.worker, Timeout: 30 * time.Second, SolverTimeout: 120 * time.Second, ConnectionRetries: 2, RetryDelay: 200 * time.Millisecond}
	engine, err := legacyruntime.NewWithRootCAs(ctx, opts, p.roots)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", failure("transport_error", "runtime_start", "")
	}
	f.engine = engine
	defer func() {
		if closeErr := engine.Close(); err == nil && closeErr != nil {
			err = failure("transport_error", f.stage, "")
		}
	}()
	jar := transport.NewCookieJar()
	engine.SetJar(jar)
	f.client = NewClient(&guardedTransport{flow: f}, jar, engine.Profile())
	f.client.UserAgent = engine.UserAgent()
	f.client.ConnectionRetries = 2
	f.client.Challenge = func(ctx context.Context, raw string) error {
		previous := f.stage
		f.emit("browser_verification")
		if err := engine.Challenge(ctx, raw); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return failure("timeout", "browser_verification", "browser_verification_timeout")
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return failure("interaction_required", "browser_verification", "browser_verification_required")
		}
		f.emit(previous)
		return nil
	}
	if err = f.loginWeb(ctx); err != nil {
		return "", err
	}
	if f.security != nil {
		if err = f.prepareSecurity(ctx); err != nil {
			return "", err
		}
		if err = f.runSecurity(ctx); err != nil {
			return "", err
		}
		if !in.Reauthorize {
			return "", nil
		}
		if err = f.security.save("reauthorization"); err != nil {
			return "", err
		}
		f.emit("reauthorization")
		f.in.Credentials.TOTPSecret = f.security.currentSecret(in.Credentials.TOTPSecret)
		if err = f.client.ResetSession(ctx); err != nil {
			return "", err
		}
		if err = f.loginWeb(ctx); err != nil {
			return "", err
		}
	}
	callback, err = f.oauthFlow(ctx)
	if err == nil && f.security != nil {
		err = f.security.save("security_complete")
	}
	return callback, err
}

func (f *flow) emit(stage string) {
	if f.stage != stage {
		f.stage = stage
		if f.progress != nil {
			f.progress(stage)
		}
	}
}

var forbiddenPath = regexp.MustCompile(`(?i)/(?:oauth/token|password/(?:add|reset)|reset-password|create[._-]account|mfa/(?:enroll|activate|disable)|add-phone|phone-otp)(?:/|$)`)
var postPaths = map[string]bool{"/api/accounts/authorize/continue": true, "/api/accounts/password/verify": true, "/api/accounts/mfa/issue_challenge": true, "/api/accounts/mfa/verify": true, "/api/accounts/session/select": true, "/api/accounts/workspace/select": true, "/api/accounts/organization/select": true, "/api/accounts/consent": true, "/api/accounts/consent/accept": true}

type guardedTransport struct{ flow *flow }

// A fresh authorization clears both web cookies and native challenge/connection
// state while retaining the explicitly selected account proxy and device.
func (g *guardedTransport) ResetSession(ctx context.Context) error {
	return g.flow.engine.ResetSession(ctx)
}

func (g *guardedTransport) Do(ctx context.Context, r transport.Request) (*transport.Response, error) {
	f := g.flow
	u, err := f.trusted(r.URL, false)
	if err != nil {
		return nil, err
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != "" {
		if r.Method != http.MethodPost || !(f.securityRequestAllowed(u) || origin(u) == f.p.auth && postPaths[u.Path] || origin(u) == f.p.chat && u.Path == "/api/auth/signin/openai") {
			return nil, failure("interaction_required", f.stage, "unsupported_request")
		}
	}
	return f.engine.Do(ctx, r)
}

func origin(u *url.URL) string { return u.Scheme + "://" + u.Host }
func (f *flow) trusted(raw string, callback bool) (*url.URL, error) {
	base, _ := url.Parse(f.p.auth + "/")
	u, e := url.Parse(raw)
	if e != nil || raw != strings.TrimSpace(raw) || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return nil, failure("interaction_required", f.stage, "unsupported_navigation")
	}
	u = base.ResolveReference(u)
	if callback && f.oauth && f.isCallback(u.String()) {
		return u, nil
	}
	if (origin(u) != f.p.auth && origin(u) != f.p.chat) || forbiddenPath.MatchString(u.Path) && !f.securityRequestAllowed(u) {
		return nil, failure("interaction_required", f.stage, "unsupported_navigation")
	}
	return u, nil
}

func (f *flow) isCallback(raw string) bool {
	u, e := url.Parse(raw)
	r, re := url.Parse(f.in.RedirectURI)
	return e == nil && re == nil && u.User == nil && u.Fragment == "" && origin(u) == origin(r) && u.Path == r.Path
}
func (f *flow) callback(raw string) (string, error) {
	if !f.oauth || !f.isCallback(raw) || len(raw) > 8192 {
		return "", failure("callback_invalid", f.stage, "")
	}
	u, _ := url.Parse(raw)
	q, e := url.ParseQuery(u.RawQuery)
	if e != nil {
		return "", failure("callback_invalid", f.stage, "")
	}
	if len(q["state"]) != 1 || q.Get("state") != f.in.State {
		return "", failure("callback_state_mismatch", f.stage, "")
	}
	if _, ok := q["error"]; ok {
		return "", failure("authorization_denied", f.stage, "")
	}
	if len(q["code"]) != 1 || q.Get("code") == "" || len(q.Get("code")) > 4096 {
		return "", failure("callback_invalid", f.stage, "")
	}
	f.emit("callback")
	return raw, nil
}

func (f *flow) follow(ctx context.Context, raw, ref string) (*transport.FollowResult, error) {
	u, e := f.trusted(raw, true)
	if e != nil {
		return nil, e
	}
	if f.isCallback(u.String()) {
		return &transport.FollowResult{FinalURL: u.String()}, nil
	}
	r, e := f.client.Follow(ctx, u.String(), RequestOptions{Referer: ref, StopAt: func(raw string) bool { return f.oauth && f.isCallback(raw) }, MaxResponseBytes: 2 << 20})
	if e != nil {
		return nil, e
	}
	if _, e = f.trusted(r.FinalURL, true); e != nil {
		return nil, e
	}
	return r, nil
}

type payload map[string]any

func object(v any) payload {
	switch x := v.(type) {
	case payload:
		return x
	case map[string]any:
		return payload(x)
	}
	return nil
}
func text(v any) string         { x, _ := v.(string); return x }
func values(v any) []any        { x, _ := v.([]any); return x }
func pageType(p payload) string { return text(object(p["page"])["type"]) }
func continuation(p payload) string {
	if s := text(p["continue_url"]); s != "" {
		return s
	}
	return text(object(object(p["page"])["payload"])["url"])
}
func pathOf(raw string) string {
	u, e := url.Parse(raw)
	if e != nil {
		return ""
	}
	return u.Path
}

func (f *flow) json(ctx context.Context, method, raw string, body any, opt RequestOptions) (payload, error) {
	if _, e := f.trusted(raw, false); e != nil {
		return nil, e
	}
	opt.MaxResponseBytes = 256 << 10
	var p payload
	_, e := f.client.JSON(ctx, method, raw, body, opt, &p)
	if e != nil {
		return nil, e
	}
	if p == nil {
		return nil, failure("interaction_required", f.stage, "authentication_not_advanced")
	}
	return p, nil
}
func (f *flow) auth(ctx context.Context, path string, body any, ref, tokenFlow string) (payload, error) {
	if ref == "" {
		ref = f.p.auth + "/"
	}
	var h http.Header
	if tokenFlow != "" {
		var err error
		// The reference runner binds the SDK page to its flow name. In
		// particular, MFA uses password_verify's password page context while
		// its HTTP request retains the current MFA challenge as Referer.
		page := "/log-in"
		if tokenFlow == "password_verify" {
			page = "/log-in/password"
		}
		h, err = f.engine.Sentinel(ctx, f.p.auth, f.device, tokenFlow, f.p.auth+page)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, failure("interaction_required", f.stage, "sentinel_unavailable")
		}
	}
	return f.json(ctx, http.MethodPost, f.p.auth+path, body, RequestOptions{Headers: h, Origin: f.p.auth, Referer: ref})
}

func (f *flow) next(p payload) error {
	typeName, next := pageType(p), pathOf(continuation(p))
	if typeName == "email_verification" || next == "/email-verification" {
		return failure("interaction_required", "email", "email_verification")
	}
	if typeName == "add_phone" || next == "/add-phone" || next == "/phone-verification" {
		return failure("interaction_required", f.stage, "phone_verification")
	}
	if typeName == "about_you" || next == "/about-you" {
		return failure("interaction_required", f.stage, "unsupported_page")
	}
	return nil
}
func (f *flow) advanced(p payload, previous, ref string) error {
	if p == nil || p["error"] != nil && p["error"] != false || pageType(p) == previous || pageType(p) == "login_password" {
		return failure("interaction_required", f.stage, "authentication_not_advanced")
	}
	if v, ok := p["success"]; ok && v != true {
		return failure("interaction_required", f.stage, "authentication_not_advanced")
	}
	if err := f.next(p); err != nil {
		return err
	}
	if next := continuation(p); next != "" {
		u, e := f.trusted(next, true)
		if e != nil {
			return e
		}
		if u.Path == pathOf(ref) || previous == "mfa_challenge" && strings.HasPrefix(u.Path, "/mfa-challenge") {
			return failure("interaction_required", f.stage, "authentication_not_advanced")
		}
		return nil
	}
	if pageType(p) == "mfa_challenge" || pageType(p) == "workspace" {
		return nil
	}
	return failure("interaction_required", f.stage, "authentication_not_advanced")
}

func (f *flow) loginWeb(ctx context.Context) error {
	f.emit("session")
	if _, e := f.follow(ctx, f.p.chat+"/", f.p.chat+"/"); e != nil {
		return e
	}
	if _, e := f.json(ctx, http.MethodGet, f.p.chat+"/api/auth/providers", nil, RequestOptions{Referer: f.p.chat + "/"}); e != nil {
		return e
	}
	csrf, e := f.json(ctx, http.MethodGet, f.p.chat+"/api/auth/csrf", nil, RequestOptions{Referer: f.p.chat + "/"})
	if e != nil {
		return e
	}
	if text(csrf["csrfToken"]) == "" {
		return failure("interaction_required", f.stage, "session_unconfirmed")
	}
	f.device = f.client.Jar.Value("oai-did", f.p.chat+"/")
	if f.device == "" {
		f.device = randomUUID()
	}
	u, _ := url.Parse(f.p.chat)
	f.client.Jar.Set(transport.Cookie{Name: "oai-did", Value: f.device, Domain: u.Hostname(), Path: "/", HostOnly: true, Secure: u.Scheme == "https"})
	q := url.Values{"prompt": {"login"}, "ext-oai-did": {f.device}, "auth_session_logging_id": {randomUUID()}, "screen_hint": {"login_or_signup"}, "login_hint": {f.in.Credentials.Email}}
	signin, e := f.json(ctx, http.MethodPost, f.p.chat+"/api/auth/signin/openai?"+q.Encode(), nil, RequestOptions{Origin: f.p.chat, Referer: f.p.chat + "/", Form: url.Values{"callbackUrl": {f.p.chat + "/"}, "csrfToken": {text(csrf["csrfToken"])}, "json": {"true"}}})
	if e != nil {
		return e
	}
	raw := text(signin["url"])
	if raw == "" {
		return failure("interaction_required", f.stage, "session_unconfirmed")
	}
	u, e = f.trusted(raw, false)
	if e != nil {
		return e
	}
	if origin(u) != f.p.auth {
		return failure("interaction_required", f.stage, "session_unconfirmed")
	}
	page, e := f.follow(ctx, raw, f.p.chat+"/")
	if e != nil {
		return e
	}
	webURL, _ := url.Parse(page.FinalURL)
	if webURL == nil || origin(webURL) != f.p.chat || webURL.Path != "/" {
		path := pathOf(page.FinalURL)
		if path == "/email-verification" {
			return failure("interaction_required", "email", "email_verification")
		}
		if path == "/log-in" || path == "/log-in-or-create-account" {
			f.emit("email")
			p, e := f.auth(ctx, "/api/accounts/authorize/continue", payload{"username": payload{"kind": "email", "value": f.in.Credentials.Email}}, page.FinalURL, "authorize_continue")
			if e != nil {
				return e
			}
			if e = f.next(p); e != nil {
				return e
			}
			if continuation(p) == "" {
				return failure("interaction_required", f.stage, "authentication_not_advanced")
			}
			page, e = f.follow(ctx, continuation(p), page.FinalURL)
			if e != nil {
				return e
			}
			path = pathOf(page.FinalURL)
		}
		if path != "/log-in/password" {
			return failure("interaction_required", f.stage, "unsupported_page")
		}
		f.emit("password")
		current, e := f.auth(ctx, "/api/accounts/password/verify", payload{"password": f.in.Credentials.Password}, page.FinalURL, "password_verify")
		if e != nil {
			return e
		}
		if e = f.advanced(current, "login_password", page.FinalURL); e != nil {
			return e
		}
		if pageType(current) == "mfa_challenge" || strings.HasPrefix(pathOf(continuation(current)), "/mfa-challenge/") {
			f.emit("totp")
			session := object(current["oai-client-auth-session"])
			factor := ""
			for _, key := range []string{"mfa_challenge_factors", "mfa_factors"} {
				for _, v := range values(session[key]) {
					x := object(v)
					if text(x["factor_type"]) == "totp" && text(x["id"]) != "" {
						factor = text(x["id"])
						break
					}
				}
				if factor != "" {
					break
				}
			}
			if factor == "" {
				return failure("interaction_required", f.stage, "totp_factor_missing")
			}
			if _, e = f.auth(ctx, "/api/accounts/mfa/issue_challenge", payload{"type": "totp", "id": factor, "force_fresh_challenge": false}, page.FinalURL, ""); e != nil {
				return e
			}
			ref := continuation(current)
			if ref == "" {
				ref = f.p.auth + "/mfa-challenge/" + url.PathEscape(factor)
			}
			if _, e = f.trusted(ref, false); e != nil {
				return e
			}
			code, e := generateTOTP(f.in.Credentials.TOTPSecret, f.p.now())
			if e != nil {
				return e
			}
			current, e = f.auth(ctx, "/api/accounts/mfa/verify", payload{"type": "totp", "id": factor, "code": code}, ref, "password_verify")
			if e != nil {
				return e
			}
			if e = f.advanced(current, "mfa_challenge", ref); e != nil {
				return e
			}
		}
		page, e = f.continuePayload(ctx, current)
		if e != nil {
			return e
		}
	}
	u, e = url.Parse(page.FinalURL)
	if e != nil || origin(u) != f.p.chat || u.Path != "/" {
		return failure("interaction_required", "session", "session_unconfirmed")
	}
	f.emit("session")
	session, e := f.json(ctx, http.MethodGet, f.p.chat+"/api/auth/session", nil, RequestOptions{Referer: f.p.chat + "/"})
	if e != nil {
		return e
	}
	f.session = session
	if !strings.EqualFold(text(object(session["user"])["email"]), f.in.Credentials.Email) {
		return failure("interaction_required", f.stage, "session_unconfirmed")
	}
	return nil
}

func (f *flow) continuePayload(ctx context.Context, p payload) (*transport.FollowResult, error) {
	for step := 0; step < 4; step++ {
		if e := f.next(p); e != nil {
			return nil, e
		}
		next := continuation(p)
		if next != "" {
			u, e := f.trusted(next, true)
			if e != nil {
				return nil, e
			}
			if f.isCallback(next) || origin(u) == f.p.chat && u.Path == "/api/auth/callback/openai" {
				return f.follow(ctx, next, "")
			}
		}
		ws := values(object(p["oai-client-auth-session"])["workspaces"])
		if pageType(p) == "workspace" || len(ws) > 0 {
			selected := ""
			valid := []string{}
			for _, v := range ws {
				w := object(v)
				if id := text(w["id"]); id != "" {
					valid = append(valid, id)
					if id == f.in.WorkspaceID {
						selected = id
					}
				}
			}
			if f.in.WorkspaceID == "" && len(valid) == 1 {
				selected = valid[0]
			}
			if selected == "" {
				return nil, failure("interaction_required", "workspace", "workspace_selection")
			}
			f.emit("workspace")
			var e error
			p, e = f.auth(ctx, "/api/accounts/workspace/select", payload{"workspace_id": selected}, next, "")
			if e != nil {
				return nil, e
			}
			continue
		}
		if next == "" {
			return nil, failure("interaction_required", f.stage, "authentication_not_advanced")
		}
		return f.follow(ctx, next, "")
	}
	return nil, failure("interaction_required", "workspace", "workspace_selection")
}

var sessionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`"session_id"\s*:\s*"(us_[A-Za-z0-9_-]{10,})"`),
	regexp.MustCompile(`session_id\\",\\"(us_[A-Za-z0-9_-]{10,})`),
	regexp.MustCompile(`name="session_id"[^>]+value="(us_[A-Za-z0-9_-]{10,})"`),
	regexp.MustCompile(`value="(us_[A-Za-z0-9_-]{10,})"[^>]+name="session_id"`),
}

func extractSession(raw string) string {
	ids := map[string]bool{}
	raw = html.UnescapeString(raw)
	for _, re := range sessionPatterns {
		for _, match := range re.FindAllStringSubmatch(raw, -1) {
			ids[match[1]] = true
		}
	}
	if len(ids) != 1 {
		return ""
	}
	for id := range ids {
		return id
	}
	return ""
}
func (f *flow) oauthFlow(ctx context.Context) (string, error) {
	f.emit("oauth")
	f.oauth = true
	page, e := f.follow(ctx, f.in.AuthorizationURL, "")
	if e != nil {
		return "", e
	}
	if f.isCallback(page.FinalURL) {
		return f.callback(page.FinalURL)
	}
	if page.Response == nil {
		return "", failure("interaction_required", f.stage, "account_selection")
	}
	session := extractSession(page.Response.Text())
	if session == "" {
		return "", failure("interaction_required", f.stage, "account_selection")
	}
	current, e := f.auth(ctx, "/api/accounts/session/select", payload{"session_id": session}, f.p.auth+"/choose-an-account", "")
	if e != nil {
		return "", e
	}
	page, e = f.continuePayload(ctx, current)
	if e != nil {
		return "", e
	}
	return f.callback(page.FinalURL)
}

func (f *flow) safeError(ctx context.Context, err error) error {
	var known *Error
	if errors.As(err, &known) {
		return known
	}
	if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
		return failure("cancelled", f.stage, "")
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		reason := ""
		if f.stage == "browser_verification" {
			reason = "browser_verification_timeout"
		}
		return failure("timeout", f.stage, reason)
	}
	var h *HTTPError
	if errors.As(err, &h) {
		if h.StatusCode == 429 {
			return failure("rate_limited", f.stage, "")
		}
		if h.StatusCode >= 500 {
			return failure("service_error", f.stage, "")
		}
		if h.StatusCode == 400 || h.StatusCode == 401 || h.StatusCode == 403 || h.StatusCode == 422 {
			if f.stage == "password" && (h.Code == "invalid_password" || h.Code == "incorrect_password" || h.Code == "wrong_password") {
				return failure("password_rejected", f.stage, "upstream_rejected")
			}
			if f.stage == "totp" && (h.Code == "invalid_totp" || h.Code == "invalid_otp" || h.Code == "invalid_code" || h.Code == "incorrect_code" || h.Code == "wrong_code") {
				return failure("totp_rejected", f.stage, "upstream_rejected")
			}
		}
		return failure("interaction_required", f.stage, "upstream_rejected")
	}
	if errors.Is(err, ErrBrowserChallenge) {
		return failure("interaction_required", "browser_verification", "browser_verification_required")
	}
	return failure("transport_error", f.stage, "")
}
