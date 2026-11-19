package login

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/cli-proxy-api/model-quality-login-runtime/internal/transport"
)

// RequestDiagnostic is deliberately unable to carry identities, URLs, headers,
// response bodies or raw error messages. Durations cover one protocol request.
type RequestDiagnostic struct {
	Phase       string `json:"phase"`
	RequestKind string `json:"request_kind"`
	Method      string `json:"method"`
	HTTPStatus  int    `json:"http_status,omitempty"`
	DurationMS  int64  `json:"duration_ms"`
	ErrorReason string `json:"error_reason,omitempty"`
	Failed      bool   `json:"failed"`
}

type diagnosticObserverKey struct{}

// WithRequestDiagnostics observes requests without authorizing any retry or
// changing the flow. The callback receives only the closed safe projection.
func WithRequestDiagnostics(ctx context.Context, observer func(RequestDiagnostic)) context.Context {
	return context.WithValue(ctx, diagnosticObserverKey{}, observer)
}

func requestKind(path string) string {
	switch path {
	case "/api/auth/providers":
		return "auth_providers"
	case "/api/auth/csrf":
		return "auth_csrf"
	case "/api/auth/signin/openai":
		return "auth_signin"
	case "/api/auth/session":
		return "auth_session"
	case "/api/auth/callback/openai":
		return "auth_callback"
	case "/oauth/authorize":
		return "oauth_authorize"
	case "/api/accounts/authorize/continue":
		return "authorize_continue"
	case "/api/accounts/password/verify":
		return "password_verify"
	case "/api/accounts/mfa/issue_challenge":
		return "totp_challenge"
	case "/api/accounts/mfa/verify":
		return "totp_verify"
	case "/api/accounts/session/select":
		return "session_select"
	case "/api/accounts/workspace/select":
		return "workspace_select"
	case "/api/accounts/organization/select":
		return "organization_select"
	case "/api/accounts/consent", "/api/accounts/consent/accept":
		return "consent_accept"
	case mfaInfoEndpoint:
		return "mfa_info"
	case disableEndpoint:
		return "totp_disable"
	case enrollEndpoint:
		return "totp_enroll"
	case activateEndpoint:
		return "totp_activate"
	case logoutEndpoint:
		return "logout_all"
	default:
		return "navigation"
	}
}

func (f *flow) observeRequest(ctx context.Context, request transport.Request, u *url.URL, started time.Time, status int, err error) {
	observer, _ := ctx.Value(diagnosticObserverKey{}).(func(RequestDiagnostic))
	if observer == nil {
		return
	}
	method := request.Method
	if method == "" {
		method = http.MethodGet
	}
	if method != http.MethodGet && method != http.MethodHead && method != http.MethodPost {
		return
	}
	duration := time.Since(started).Milliseconds()
	if duration < 0 {
		duration = 0
	}
	if duration > 900000 {
		duration = 900000
	}
	if status < 100 || status > 599 {
		status = 0
	}
	observer(RequestDiagnostic{Phase: f.stage, RequestKind: requestKind(u.Path), Method: method, HTTPStatus: status, DurationMS: duration, ErrorReason: transport.DiagnosticReason(err), Failed: err != nil})
}
