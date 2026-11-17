// Adapted from toSub2-go-port account-security workflows; see SOURCE_MANIFEST.json.
package login

import (
	"context"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/MoeForge/moego/base64"
	json "github.com/MoeForge/moego/json"
	"github.com/cli-proxy-api/model-quality-login-runtime/internal/authmaterial"
	"github.com/cli-proxy-api/model-quality-login-runtime/internal/transport"
)

const disableEndpoint = "/backend-api/accounts/mfa/user/disable_in_house"
const enrollEndpoint = "/backend-api/accounts/mfa/enroll"
const activateEndpoint = "/backend-api/accounts/mfa/user/activate_enrollment"
const logoutEndpoint = "/backend-api/accounts/logout_all"
const mfaInfoEndpoint = "/backend-api/accounts/mfa_info"

func (f *flow) securityRequestAllowed(u *url.URL) bool {
	if f.security == nil || origin(u) != f.p.chat {
		return false
	}
	switch u.Path {
	case disableEndpoint, enrollEndpoint, activateEndpoint:
		return f.in.Operation == "reset_totp"
	case logoutEndpoint:
		return f.in.Operation == "logout_all" || f.in.Operation == "reset_totp" && f.in.LogoutAfterReset
	}
	return false
}

func (f *flow) securityAuthorization() (string, string, error) {
	invalid := func() (string, string, error) {
		return "", "", failure("security_identity_mismatch", "security_check", "session_unconfirmed")
	}
	session := f.session
	if !strings.EqualFold(strings.TrimSpace(text(object(session["user"])["email"])), strings.TrimSpace(f.in.Credentials.Email)) {
		return invalid()
	}
	account := object(session["account"])
	id := ""
	for _, v := range []any{account["id"], account["account_id"], session["account_id"]} {
		if v == nil {
			continue
		}
		s, ok := v.(string)
		if !ok || strings.TrimSpace(s) == "" || s != strings.TrimSpace(s) || id != "" && s != id {
			return invalid()
		}
		id = s
	}
	token := text(session["accessToken"])
	if value, present := session["accessToken"]; present && value != nil {
		if _, valid := value.(string); !valid {
			return invalid()
		}
	}
	if strings.TrimSpace(token) == "" {
		token = f.securityPageToken
	}
	if f.in.WorkspaceID != "" && f.in.WorkspaceID != id {
		return invalid()
	}
	if id == "" || token == "" || len(id) > 512 || len(token) > 32<<10 || strings.ContainsAny(token, "\r\n") {
		return invalid()
	}
	// Token account claims corroborate the live session; the user ID is never
	// substituted for a workspace/account ID in account-scoped security headers.
	parts := strings.Split(token, ".")
	if len(parts) == 3 {
		raw, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err == nil {
			var claims payload
			if authmaterial.ValidateUnambiguousJSON(raw) == nil && json.Unmarshal(raw, &claims) == nil {
				tokenID := text(object(claims["https://api.openai.com/auth"])["chatgpt_account_id"])
				if tokenID != "" && tokenID != id {
					return invalid()
				}
			}
			clear(raw)
		}
	}
	return token, id, nil
}

func (f *flow) runSecurity(ctx context.Context) error {
	f.emit("security_check")
	token, account, err := f.securityAuthorization()
	if err != nil {
		return err
	}
	j := f.security
	j.AccountID = account
	if err = j.save("security_check"); err != nil {
		return err
	}
	if f.in.Operation == "reset_totp" {
		if err = f.resetTOTP(ctx, token, account); err != nil {
			return err
		}
	}
	if f.in.Operation == "logout_all" || f.in.LogoutAfterReset {
		if err = f.logoutAll(ctx, token, account); err != nil {
			return err
		}
	}
	j.Status = "succeeded"
	if err = j.save("security_complete"); err != nil {
		return err
	}
	f.emit("security_complete")
	return nil
}

func (f *flow) securityHeaders(token, account string) http.Header {
	h := make(http.Header)
	h.Set("Authorization", "Bearer "+token)
	h.Set("Accept", "*/*")
	h.Set("User-Agent", f.client.UserAgent)
	h.Set("Accept-Language", f.client.AcceptLanguage)
	h.Set("Sec-Fetch-Site", "same-origin")
	h.Set("Sec-Fetch-Mode", "cors")
	h.Set("Sec-Fetch-Dest", "empty")
	h.Set("Priority", "u=1, i")
	h.Set("Oai-Did", f.device)
	locale := strings.Split(strings.Split(f.client.AcceptLanguage, ",")[0], ";")[0]
	if locale == "" {
		locale = "en-US"
	}
	h.Set("Oai-Language", locale)
	h.Set("Originator", "web")
	h.Set("Chatgpt-Account-Id", account)
	return h
}

// Every mutation is dispatched once, without challenge replay or HTTP retry.
// A complete response is evidence even if closing its transport reports an error.
func (f *flow) securityRequest(ctx context.Context, token, account, method, path string, body any) (*transport.Response, payload, error) {
	headers := f.securityHeaders(token, account)
	if method == http.MethodGet {
		headers.Set("Cache-Control", "no-cache, no-store")
		headers.Set("Pragma", "no-cache")
	}
	opt := RequestOptions{Headers: headers, Referer: f.p.chat + "/", Origin: f.p.chat, DisableDefaultHeaders: true, DisableChallengeReplay: true, SingleAttempt: method != http.MethodGet, MaxResponseBytes: 256 << 10, CaptureResponse: true, JSON: body}
	response, err := f.client.Request(ctx, method, f.p.chat+path, opt)
	var decoded payload
	if response != nil {
		if parsed, valid := decodeTOTPResponse(response.Body); valid {
			decoded = payload(parsed)
		}
		if response.BodyComplete {
			err = nil
		}
	}
	return response, decoded, err
}

func receiptOutcome(response *transport.Response, err error, step string, body payload, factor, session string) totpMutationOutcome {
	if response == nil || err != nil || isBrowserChallenge(response) {
		return totpMutationUnknown
	}
	if response.StatusCode >= 400 && response.StatusCode < 500 && response.StatusCode != 408 {
		return totpMutationRejected
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return totpMutationUnknown
	}
	return classifyTOTPMutation(step, body, factor, session)
}

func liveFactor(info payload) (state, id string) {
	if info == nil || !totpOutcomeFieldsValid(info) || totpResponseRejected(info) {
		return "unknown", ""
	}
	enabled, hasEnabled := info["mfa_enabled_v2"]
	factors, hasFactors := info["factors"]
	container := object(factors)
	if hasFactors && factors != nil && container == nil {
		return "unknown", ""
	}
	list, hasList := container["totp"].([]any)
	if hasEnabled && enabled == false && (container["totp"] == nil || hasList && len(list) == 0) || !hasEnabled && hasList && len(list) == 0 {
		return "disabled", ""
	}
	if enabled != true || !hasList || len(list) != 1 {
		return "unknown", ""
	}
	factor := object(list[0])
	id = text(factor["id"])
	if strings.TrimSpace(id) == "" || id != strings.TrimSpace(id) || len(id) > 512 || factor["factor_type"] != "totp" || factor["is_recovery"] != false {
		return "unknown", ""
	}
	return "enabled", id
}
func (f *flow) readFactor(ctx context.Context, token, account string) (string, string, error) {
	r, body, err := f.securityRequest(ctx, token, account, http.MethodGet, mfaInfoEndpoint, nil)
	if err != nil || r == nil || r.StatusCode < 200 || r.StatusCode >= 300 || isBrowserChallenge(r) {
		return "unknown", "", failure("security_state_unconfirmed", f.stage, "upstream_rejected")
	}
	state, id := liveFactor(body)
	return state, id, nil
}
func (f *flow) reconcileFactor(ctx context.Context, token, account, wanted, id string) bool {
	// Read-only reconciliation is limited to this operation's in-memory session.
	for i := 0; i < 3; i++ {
		if ctx.Err() != nil {
			return false
		}
		state, actual, err := f.readFactor(ctx, token, account)
		if err == nil && state == wanted && (wanted == "disabled" || actual == id) {
			return true
		}
	}
	return false
}
func securityOutcomeError(phase, state string) error {
	code := "security_outcome_unknown"
	if state == "rejected" {
		code = "security_rejected"
	}
	return failure(code, phase, "upstream_rejected")
}

func (f *flow) resetTOTP(ctx context.Context, token, account string) error {
	j := f.security
	state, old, err := f.readFactor(ctx, token, account)
	if err != nil {
		return err
	}
	if state != "enabled" {
		return failure("security_state_unconfirmed", "security_check", "totp_factor_missing")
	}
	j.OldFactorID = old
	j.DisableStatus = "pending"
	f.emit("totp_disable")
	if err = j.save("totp_disable"); err != nil {
		return err
	}
	r, body, requestErr := f.securityRequest(ctx, token, account, http.MethodPost, disableEndpoint, payload{"factor_id": old})
	switch receiptOutcome(r, requestErr, "disable", body, old, "") {
	case totpMutationSucceeded:
		j.DisableStatus = "succeeded"
	case totpMutationRejected:
		j.DisableStatus = "rejected"
	default:
		j.DisableStatus = "unknown"
	}
	if err = j.save("totp_disable"); err != nil {
		return err
	}
	if j.DisableStatus == "unknown" && f.reconcileFactor(ctx, token, account, "disabled", "") {
		j.DisableStatus = "succeeded"
		if err = j.save("totp_disable"); err != nil {
			return err
		}
	}
	if j.DisableStatus != "succeeded" {
		return securityOutcomeError("totp_disable", j.DisableStatus)
	}
	j.EnrollStatus = "pending"
	f.emit("totp_enroll")
	if err = j.save("totp_enroll"); err != nil {
		return err
	}
	r, body, requestErr = f.securityRequest(ctx, token, account, http.MethodPost, enrollEndpoint, payload{"factor_type": "totp", "source": "settings"})
	// Preserve recognized enrollment material even when its receipt is incomplete.
	secret, session := text(body["secret"]), text(body["session_id"])
	factor := object(body["factor"])
	id := text(factor["id"])
	if len(secret) <= 2048 {
		j.NewTOTPSecret = secret
	}
	if len(session) <= 2048 {
		j.EnrollmentSessionID = session
	}
	if len(id) <= 512 {
		j.NewFactorID = id
	}
	_, seedErr := normalizeSecret(secret)
	valid := seedErr == nil && strings.TrimSpace(session) != "" && session == strings.TrimSpace(session) && len(session) <= 2048 && strings.TrimSpace(id) != "" && id == strings.TrimSpace(id) && len(id) <= 512 && id != old && factor["factor_type"] == "totp" && factor["is_recovery"] == false && totpOutcomeFieldsValid(body) && !totpResponseRejected(body) && totpResponseBindingMatches(body, id, session)
	j.EnrollStatus = "unknown"
	if r != nil && requestErr == nil && !isBrowserChallenge(r) {
		if r.StatusCode >= 400 && r.StatusCode < 500 && r.StatusCode != 408 || r.StatusCode >= 200 && r.StatusCode < 300 && totpResponseRejected(body) {
			j.EnrollStatus = "rejected"
		}
		if r.StatusCode >= 200 && r.StatusCode < 300 && valid {
			j.EnrollStatus = "succeeded"
		}
	}
	if err = j.save("totp_enroll"); err != nil {
		return err
	}
	if j.EnrollStatus != "succeeded" {
		return securityOutcomeError("totp_enroll", j.EnrollStatus)
	}
	j.ActivationStatus = "pending"
	f.emit("totp_activate")
	// This publication, fsync and readback is the pending-seed durability barrier.
	if err = j.save("totp_activate"); err != nil {
		return err
	}
	code, err := generateTOTP(secret, f.p.now())
	if err != nil {
		return err
	}
	r, body, requestErr = f.securityRequest(ctx, token, account, http.MethodPost, activateEndpoint, payload{"factor_type": "totp", "session_id": session, "code": code, "source": "settings"})
	switch receiptOutcome(r, requestErr, "activation", body, id, session) {
	case totpMutationSucceeded:
		j.ActivationStatus = "succeeded"
		j.ActivationConfirmation = "business_receipt"
	case totpMutationRejected:
		j.ActivationStatus = "rejected"
	default:
		j.ActivationStatus = "unknown"
	}
	if err = j.save("totp_activate"); err != nil {
		return err
	}
	if j.ActivationStatus == "unknown" && f.reconcileFactor(ctx, token, account, "enabled", id) {
		j.ActivationStatus = "succeeded"
		j.ActivationConfirmation = "state_read"
	}
	if j.ActivationStatus != "succeeded" {
		return securityOutcomeError("totp_activate", j.ActivationStatus)
	}
	// Confirmed evidence is the authoritative credential result. The parent can
	// promote it on exit or restart even if subsequent logout or OAuth fails.
	if err = j.save("totp_saved"); err != nil {
		return err
	}
	f.emit("totp_saved")
	return nil
}

func (f *flow) logoutAll(ctx context.Context, token, account string) error {
	j := f.security
	j.LogoutStatus = "pending"
	f.emit("logout_all")
	if err := j.save("logout_all"); err != nil {
		return err
	}
	r, _, requestErr := f.securityRequest(ctx, token, account, http.MethodPost, logoutEndpoint, nil)
	j.LogoutStatus = "unknown"
	if r != nil && requestErr == nil {
		if r.StatusCode >= 400 && r.StatusCode < 500 && r.StatusCode != 408 && !isBrowserChallenge(r) {
			j.LogoutStatus = "rejected"
		}
		if r.StatusCode >= 200 && r.StatusCode < 300 && logoutResponseValid(r) {
			j.LogoutStatus = "succeeded"
		}
	}
	if err := j.save("logout_all"); err != nil {
		return err
	}
	if j.LogoutStatus != "succeeded" {
		return securityOutcomeError("logout_all", j.LogoutStatus)
	}
	return nil
}

func logoutResponseValid(r *transport.Response) bool {
	if r == nil || isBrowserChallenge(r) {
		return false
	}
	body := strings.TrimSpace(r.Text())
	lower := strings.ToLower(body)
	if strings.Contains(strings.ToLower(r.Header.Get("Content-Type")), "text/html") || strings.HasPrefix(lower, "<!doctype") || strings.HasPrefix(lower, "<html") || strings.HasPrefix(lower, "<script") {
		return false
	}
	if !strings.HasPrefix(body, "{") && !strings.HasPrefix(body, "[") {
		return true
	}
	if authmaterial.ValidateUnambiguousJSON(r.Body) != nil {
		return false
	}
	var decoded any
	if json.Unmarshal(r.Body, &decoded) != nil {
		return false
	}
	receipt := object(decoded)
	if receipt == nil {
		return true
	}
	if receipt["success"] == false {
		return false
	}
	switch v := receipt["error"].(type) {
	case nil:
		return true
	case bool:
		return !v
	case string:
		return v == ""
	case float64:
		return v == 0
	default:
		return false
	}
}

// The reference enters the settings activation page before its final live
// identity read. Page token fallback is accepted only after that identity agrees.
func (f *flow) prepareSecurity(ctx context.Context) error {
	if f.in.Operation != "reset_totp" {
		return nil
	}
	f.emit("security_check")
	page, err := f.client.Follow(ctx, f.p.chat+"/?action=enable&factor=totp", RequestOptions{Referer: f.p.chat + "/", DisableChallengeReplay: true, MaxResponseBytes: 2 << 20})
	if err != nil {
		return err
	}
	if page == nil || page.Response == nil {
		return failure("security_state_unconfirmed", "security_check", "session_unconfirmed")
	}
	final, err := f.trusted(page.FinalURL, false)
	if err != nil || origin(final) != f.p.chat || final.Path != "/" {
		return failure("security_state_unconfirmed", "security_check", "session_unconfirmed")
	}
	f.securityPageToken = securityPageAccessToken(page.Response.Text())
	session, err := f.json(ctx, http.MethodGet, f.p.chat+"/api/auth/session", nil, RequestOptions{Referer: f.p.chat + "/", DisableChallengeReplay: true})
	if err != nil {
		return err
	}
	f.session = session
	return nil
}

var securityAccessTokenPattern = regexp.MustCompile(`["']accessToken["']\s*:\s*["']((?:\\.|[^"'\\])+)`)

func securityPageAccessToken(body string) string {
	for _, raw := range []string{body, html.UnescapeString(body)} {
		if match := securityAccessTokenPattern.FindStringSubmatch(raw); len(match) > 1 {
			var token string
			if json.Unmarshal([]byte(`"`+match[1]+`"`), &token) == nil && len(token) >= 20 && len(token) <= 32<<10 {
				return token
			}
		}
	}
	return ""
}
