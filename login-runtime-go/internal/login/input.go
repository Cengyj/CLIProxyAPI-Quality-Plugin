// Package login adapts the reference Go authentication engine to CPA-owned OAuth.
package login

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/MoeForge/moego/ojson"
)

const clientID = "app_EMoamEEZ73f0CkXaXp7hrann"
const authOrigin = "https://auth.openai.com"
const chatOrigin = "https://chatgpt.com"

type Credentials struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	TOTPSecret string `json:"totp_secret"`
}

type Input struct {
	Operation           string      `json:"operation,omitempty"`
	LogoutAfterReset    bool        `json:"logout_after_reset,omitempty"`
	Reauthorize         bool        `json:"reauthorize,omitempty"`
	SecurityJournalPath string      `json:"security_journal_path,omitempty"`
	Version             int         `json:"version"`
	JobID               string      `json:"job_id"`
	Attempt             int         `json:"attempt"`
	AuthorizationURL    string      `json:"authorization_url"`
	RedirectURI         string      `json:"redirect_uri"`
	State               string      `json:"state"`
	Credentials         Credentials `json:"credentials"`
	ProxyURL            string      `json:"proxy_url,omitempty"`
	TimeoutSeconds      int         `json:"timeout_seconds,omitempty"`
	WorkspaceID         string      `json:"workspace_id,omitempty"`
}

type Error struct{ Code, Stage, Reason string }

func (e *Error) Error() string                 { return e.Code }
func failure(code, stage, reason string) error { return &Error{code, stage, reason} }

var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
var challengePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

func ValidateInput(in Input) error { return validateInput(in, authOrigin) }

func validateInput(in Input, origin string) error {
	invalid := func() error { return failure("invalid_input", "", "") }
	if in.Operation != "" && in.Operation != "reset_totp" && in.Operation != "logout_all" {
		return invalid()
	}
	if in.Operation == "" && (in.LogoutAfterReset || in.Reauthorize || in.SecurityJournalPath != "") {
		return invalid()
	}
	if in.Operation != "" && (!filepath.IsAbs(in.SecurityJournalPath) || filepath.Clean(in.SecurityJournalPath) != in.SecurityJournalPath || len(in.SecurityJournalPath) > 4096 || strings.ContainsAny(in.SecurityJournalPath, "\x00\r\n") || in.Operation != "reset_totp" && in.LogoutAfterReset) {
		return invalid()
	}
	if in.Version != 1 || in.JobID == "" || len(in.JobID) > 256 || in.Attempt < 1 || in.Attempt > 100 || len(in.State) > 256 || len(in.AuthorizationURL) > 8192 || len(in.RedirectURI) > 1024 || len(in.WorkspaceID) > 256 {
		return invalid()
	}
	if !emailPattern.MatchString(in.Credentials.Email) || len(in.Credentials.Email) > 320 || in.Credentials.Password == "" || len(in.Credentials.Password) > 4096 {
		return invalid()
	}
	if in.TimeoutSeconds != 0 && (in.TimeoutSeconds < 10 || in.TimeoutSeconds > 900) {
		return invalid()
	}
	if in.Operation == "" || in.Reauthorize {
		if in.State == "" {
			return invalid()
		}
		u, err := url.Parse(in.AuthorizationURL)
		if err != nil || u.User != nil || u.Fragment != "" || u.Scheme+"://"+u.Host != origin || u.Path != "/oauth/authorize" {
			return invalid()
		}
		r, err := url.Parse(in.RedirectURI)
		if err != nil || r.User != nil || r.Fragment != "" || r.RawQuery != "" || r.Scheme != "http" || r.Port() == "" || r.Path != "/auth/callback" || (r.Hostname() != "localhost" && r.Hostname() != "127.0.0.1" && r.Hostname() != "::1") {
			return invalid()
		}
		q, err := url.ParseQuery(u.RawQuery)
		if err != nil {
			return invalid()
		}
		for _, key := range []string{"client_id", "redirect_uri", "state", "response_type", "code_challenge", "code_challenge_method", "scope"} {
			if len(q[key]) != 1 {
				return invalid()
			}
		}
		if q.Get("client_id") != clientID || q.Get("redirect_uri") != in.RedirectURI || q.Get("state") != in.State || q.Get("response_type") != "code" || q.Get("code_challenge_method") != "S256" || !challengePattern.MatchString(q.Get("code_challenge")) {
			return invalid()
		}
		scopes := map[string]bool{}
		for _, s := range strings.Fields(q.Get("scope")) {
			scopes[s] = true
		}
		for _, s := range []string{"openid", "profile", "email", "offline_access"} {
			if !scopes[s] {
				return invalid()
			}
		}
	}
	if _, err := normalizeSecret(in.Credentials.TOTPSecret); err != nil {
		return err
	}
	if _, err := normalizeProxy(in.ProxyURL); err != nil {
		return err
	}
	return nil
}

func normalizeProxy(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "direct") || strings.EqualFold(raw, "off") || strings.EqualFold(raw, "none") {
		return "", nil
	}
	if len(raw) > 4096 || strings.ContainsAny(raw, "\x00\r\n\t ?#") {
		return "", failure("invalid_input", "runtime_start", "")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" || u.Fragment != "" || u.RawQuery != "" || (u.Path != "" && u.Path != "/") {
		return "", failure("invalid_input", "runtime_start", "")
	}
	switch u.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return "", failure("invalid_input", "runtime_start", "")
	}
	if u.User != nil && u.User.Username() == "" {
		return "", failure("invalid_input", "runtime_start", "")
	}
	if u.Scheme == "socks5h" {
		u.Scheme = "socks5"
	}
	return u.String(), nil
}

func normalizeSecret(raw string) ([]byte, error) {
	invalid := func() ([]byte, error) { return nil, failure("invalid_totp_secret", "", "") }
	if len(raw) > 2048 {
		return invalid()
	}
	if strings.HasPrefix(strings.ToLower(raw), "otpauth:") {
		u, e := url.Parse(raw)
		if e != nil || u.Scheme != "otpauth" || u.Host != "totp" || u.User != nil || u.Fragment != "" {
			return invalid()
		}
		q, e := url.ParseQuery(u.RawQuery)
		if e != nil {
			return invalid()
		}
		for _, k := range []string{"secret", "algorithm", "digits", "period"} {
			if len(q[k]) > 1 {
				return invalid()
			}
		}
		if a := q.Get("algorithm"); a != "" && !strings.EqualFold(a, "SHA1") {
			return nil, failure("unsupported_totp_parameters", "", "")
		}
		if d := q.Get("digits"); d != "" && d != "6" {
			return nil, failure("unsupported_totp_parameters", "", "")
		}
		if p := q.Get("period"); p != "" && p != "30" {
			return nil, failure("unsupported_totp_parameters", "", "")
		}
		raw = q.Get("secret")
	}
	s := strings.ToUpper(strings.Join(strings.Fields(raw), ""))
	s = strings.ReplaceAll(s, "-", "")
	s = strings.TrimRight(s, "=")
	if len(s) < 16 || len(s) > 128 {
		return invalid()
	}
	switch len(s) % 8 {
	case 0, 2, 4, 5, 7:
	default:
		return invalid()
	}
	key := make([]byte, 0, len(s)*5/8)
	var pending uint16
	bits := 0
	for _, r := range s {
		n := strings.IndexRune("ABCDEFGHIJKLMNOPQRSTUVWXYZ234567", r)
		if n < 0 {
			return invalid()
		}
		pending = pending<<5 | uint16(n)
		bits += 5
		if bits >= 8 {
			bits -= 8
			key = append(key, byte(pending>>bits))
		}
	}
	if pending&uint16((1<<bits)-1) != 0 {
		return invalid()
	}
	return key, nil
}

func generateTOTP(raw string, at time.Time) (string, error) {
	key, err := normalizeSecret(raw)
	if err != nil {
		return "", err
	}
	defer clear(key)
	if at.Unix() < 0 {
		return "", failure("invalid_totp_secret", "totp", "")
	}
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(at.Unix()/30))
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(counter[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 15
	return fmt.Sprintf("%06d", (binary.BigEndian.Uint32(sum[offset:offset+4])&0x7fffffff)%1000000), nil
}

func encodePayload(v any) ([]byte, error) {
	d := ojson.NewDocString(`{}`)
	if e := d.Set("value", v); e != nil {
		return nil, e
	}
	return []byte(ojson.ParseBytes(d.Bytes()).Get("value").Raw), nil
}

type HTTPError struct {
	StatusCode int
	Code       string
}

func (e *HTTPError) Error() string { return "upstream_rejected" }
func newHTTPError(status int, body []byte) error {
	code := ""
	if ojson.ValidBytes(body) {
		code = ojson.ParseBytes(body).Get("error.code").String()
	}
	return &HTTPError{status, code}
}
