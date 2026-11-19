package login

import (
	"net/url"
	"strconv"

	"github.com/MoeForge/moego/ojson"
	codex "github.com/cliproxyapi-plugins/model-quality/login/oauth"
)

func validateCallback(raw, state string) (string, error) {
	return validateCallbackFor(raw, state, codex.RedirectURI)
}

func callbackFailure(code, reason string) error {
	return &runtimeDiagnosticError{code: code, stage: "callback", reason: reason}
}

// Accept optional provider metadata as the reference client does. Only the code
// is passed to token exchange; unknown parameters never affect binding or storage.
func validateCallbackFor(raw, state, redirect string) (string, error) {
	if len(raw) > 8192 {
		return "", callbackFailure("invalid_callback", "callback_url_invalid")
	}
	u, err := url.Parse(raw)
	want, targetErr := url.Parse(redirect)
	if err != nil || targetErr != nil || want.Scheme == "" || want.Host == "" || u.Scheme != want.Scheme || u.Host != want.Host || u.Path != want.Path || u.RawPath != "" || u.Fragment != "" || u.User != nil || u.Opaque != "" {
		return "", callbackFailure("invalid_callback", "callback_url_invalid")
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", callbackFailure("invalid_callback", "callback_parameters_invalid")
	}
	for _, name := range []string{"state", "code", "scope", "session_state", "error", "error_description"} {
		if len(q[name]) > 1 {
			return "", callbackFailure("invalid_callback", "callback_parameters_invalid")
		}
	}
	if len(q["state"]) != 1 || q.Get("state") != state || state == "" {
		return "", callbackFailure("invalid_callback", "callback_state_mismatch")
	}
	if values, present := q["error"]; present {
		if values[0] == "" || len(q["code"]) != 0 {
			return "", callbackFailure("invalid_callback", "callback_parameters_invalid")
		}
		return "", callbackFailure("interaction_required", "callback_authorization_denied")
	}
	code := q.Get("code")
	if code == "" || len(code) > 4096 {
		return "", callbackFailure("invalid_callback", "callback_code_invalid")
	}
	return code, nil
}
func ojsonDocumentWithoutAuthorization(raw []byte) []byte {
	d := ojson.NewDoc(raw)
	var scrub func(ojson.Result, string)
	scrub = func(value ojson.Result, prefix string) {
		if value.IsArray() {
			for i, child := range value.Array() {
				scrub(child, prefix+strconv.Itoa(i)+".")
			}
			return
		}
		if !value.IsObject() {
			return
		}
		value.ForEach(func(key, child ojson.Result) bool {
			name := key.String()
			switch name {
			case "authorization_url", "email", "password", "totp_secret":
				_ = d.Delete(prefix + name)
			default:
				scrub(child, prefix+name+".")
			}
			return true
		})
	}
	scrub(ojson.ParseBytes(raw), "")
	return d.Bytes()
}
