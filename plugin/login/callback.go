package login

import (
	"errors"
	"net/url"
	"strconv"

	"github.com/MoeForge/moego/ojson"
	codex "github.com/cliproxyapi-plugins/model-quality/login/oauth"
)

func validateCallback(raw, state string) (string, error) {
	if len(raw) > 8192 {
		return "", errors.New("invalid_callback")
	}
	u, err := url.Parse(raw)
	want, _ := url.Parse(codex.RedirectURI)
	if err != nil || u.Scheme != want.Scheme || u.Host != want.Host || u.Path != want.Path || u.RawPath != "" || u.Fragment != "" || u.User != nil || u.Opaque != "" {
		return "", errors.New("invalid_callback")
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", errors.New("invalid_callback")
	}
	for k, v := range q {
		if len(v) != 1 || (k != "state" && k != "code" && k != "scope" && k != "session_state" && k != "error" && k != "error_description") {
			return "", errors.New("invalid_callback")
		}
	}
	if q.Get("state") != state || state == "" {
		return "", errors.New("invalid_callback")
	}
	if q.Get("error") != "" {
		return "", errors.New("interaction_required")
	}
	code := q.Get("code")
	if code == "" || len(code) > 4096 {
		return "", errors.New("invalid_callback")
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
