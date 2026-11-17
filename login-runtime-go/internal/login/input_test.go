package login

import (
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestHostAuthorizationCannotBeReplacedOrRelaxed(t *testing.T) {
	valid := fixtureInput(authOrigin)
	if err := ValidateInput(valid); err != nil {
		t.Fatal("valid host authorization rejected")
	}
	query := func(in *Input, key, value string) {
		u, _ := url.Parse(in.AuthorizationURL)
		q := u.Query()
		q.Set(key, value)
		u.RawQuery = q.Encode()
		in.AuthorizationURL = u.String()
	}
	cases := map[string]func(*Input){
		"foreign auth origin": func(in *Input) {
			in.AuthorizationURL = strings.Replace(in.AuthorizationURL, authOrigin, "https://foreign.invalid", 1)
		},
		"credentials in auth URL": func(in *Input) {
			in.AuthorizationURL = strings.Replace(in.AuthorizationURL, "https://", "https://fixture@", 1)
		},
		"different client":      func(in *Input) { query(in, "client_id", "another-client") },
		"state mismatch":        func(in *Input) { query(in, "state", "another-state") },
		"duplicate state":       func(in *Input) { in.AuthorizationURL += "&state=host-owned-state" },
		"plain PKCE":            func(in *Input) { query(in, "code_challenge_method", "plain") },
		"missing refresh scope": func(in *Input) { query(in, "scope", "openid profile email") },
		"foreign redirect": func(in *Input) {
			in.RedirectURI = "http://foreign.invalid:1455/auth/callback"
			query(in, "redirect_uri", in.RedirectURI)
		},
		"callback with query": func(in *Input) { in.RedirectURI += "?state=another"; query(in, "redirect_uri", in.RedirectURI) },
		"callback path": func(in *Input) {
			in.RedirectURI = "http://localhost:1455/another-path"
			query(in, "redirect_uri", in.RedirectURI)
		},
		"missing secret": func(in *Input) { in.Credentials.TOTPSecret = "" },
		"unsupported hash": func(in *Input) {
			in.Credentials.TOTPSecret = "otpauth://totp/Fixture?secret=JBSWY3DPEHPK3PXP&algorithm=SHA256"
		},
		"duplicate secret": func(in *Input) {
			in.Credentials.TOTPSecret = "otpauth://totp/Fixture?secret=JBSWY3DPEHPK3PXP&secret=GEZDGNBVGY3TQOJQ"
		},
		"bad proxy scheme": func(in *Input) { in.ProxyURL = "file:///private/config" },
		"proxy query":      func(in *Input) { in.ProxyURL = "http://localhost:8080/?password=private" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := valid
			mutate(&in)
			var fixed *Error
			if !errors.As(ValidateInput(in), &fixed) {
				t.Fatal("invalid input did not fail with a fixed error")
			}
		})
	}
}

func TestTOTPMatchesRFCVectorsWithoutChangingServiceParameters(t *testing.T) {
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	for _, vector := range []struct {
		at   int64
		want string
	}{{59, "287082"}, {1111111109, "081804"}, {1111111111, "050471"}, {1234567890, "005924"}, {2000000000, "279037"}, {20000000000, "353130"}} {
		for _, encoded := range []string{secret, "otpauth://totp/Fixture?secret=" + secret + "&digits=6&period=30&algorithm=SHA1"} {
			got, err := generateTOTP(encoded, time.Unix(vector.at, 0))
			if err != nil || got != vector.want {
				t.Fatalf("RFC vector failed at %d", vector.at)
			}
		}
	}
}

func TestOAuthCallbackRequiresUniqueHostStateAndCode(t *testing.T) {
	in := fixtureInput(authOrigin)
	f := &flow{in: in, oauth: true, stage: "oauth"}
	for _, raw := range []string{
		in.RedirectURI + "?code=fixture&state=wrong-state",
		in.RedirectURI + "?code=fixture&state=" + in.State + "&state=" + in.State,
		in.RedirectURI + "?code=fixture&code=another&state=" + in.State,
		in.RedirectURI + "?error=access_denied&state=" + in.State,
		in.RedirectURI + "?state=" + in.State,
		"http://localhost:1455/another?code=fixture&state=" + in.State,
		"http://foreign.invalid:1455/auth/callback?code=fixture&state=" + in.State,
	} {
		callback, err := f.callback(raw)
		if callback != "" || err == nil {
			t.Fatal("invalid callback was accepted")
		}
	}
}

func TestAccountSessionSelectionRejectsAmbiguousHTML(t *testing.T) {
	if extractSession(`<input name="session_id" value="us_fixture_primary">`) != "us_fixture_primary" {
		t.Fatal("single structured account session was not parsed")
	}
	for _, raw := range []string{
		`<input name="session_id" value="us_fixture_primary"><input name="session_id" value="us_fixture_other">`,
		`<div>us_fixture_unbound_text</div>`,
		`{"session_id":"us_short"}`,
	} {
		if extractSession(raw) != "" {
			t.Fatal("ambiguous or unbound account session was selected")
		}
	}
}
