package conversion

import (
	"strings"
	"testing"
	"time"

	"github.com/MoeForge/moego/base64"
	"github.com/MoeForge/moego/ojson"
)

func fixture(t *testing.T) *ojson.Doc {
	t.Helper()
	claims := `{"exp":1999999999,"email":"offline@example.test","https://api.openai.com/auth":{"chatgpt_account_id":"account-1","chatgpt_user_id":"user-1","chatgpt_plan_type":"plus"}}`
	d := ojson.NewDocString(`{"name":"../test","platform":"openai","type":"oauth","credentials":{"refresh_token":"CONVERSION_REFRESH_CANARY","expires_at":"2034-01-01T00:00:00Z"},"expires_at":1,"proxy_key":"ignored","extra":{"private":"ignored"}}`)
	if err := d.Set("credentials.access_token", "e30."+base64.RawURLEncoding.EncodeToString([]byte(claims))+".fixture"); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestExportShapesMapCredentialsWithoutInventingTokens(t *testing.T) {
	row := fixture(t).String()
	for _, source := range []string{row, "[" + row + "]", `{"type":"sub2api-data","version":1,"accounts":[` + row + `],"proxies":[]}`, `{"data":{"type":"sub2api-bundle","version":1,"accounts":[` + row + `]}}`} {
		result, err := Convert([]byte(source), time.Unix(1900000000, 0))
		if err != nil || result.Ready != 1 {
			t.Fatalf("conversion failed: %v %+v", err, result)
		}
		i := result.Items[0]
		if i.Credential["type"] != "codex" || i.Credential["account_id"] != "account-1" || i.Email != "offline@example.test" || i.Credential["expired"] != "2033-05-18T03:33:19Z" {
			t.Fatal("credential mapping or token expiry precedence changed")
		}
		for _, field := range []string{"id_token", "last_refresh", "proxy_key", "extra", "expires_at", "credentials"} {
			if _, exists := i.Credential[field]; exists {
				t.Fatalf("invented or leaked field: %s", field)
			}
		}
		if strings.ContainsAny(i.Filename, "/\\") || !strings.HasSuffix(i.Filename, ".json") {
			t.Fatal("unsafe output filename")
		}
	}
}

func TestConversionRejectsInvalidAndConflictingEntries(t *testing.T) {
	for _, test := range []struct {
		path  string
		value any
		code  string
	}{
		{"platform", "anthropic", "conversion_unsupported_provider"}, {"type", "apikey", "conversion_unsupported_type"},
		{"credentials.access_token", "", "conversion_missing_access_token"}, {"credentials.access_token", 42, "invalid_credentials"},
		{"credentials.account_id", "other-account", "identity_conflict"}, {"credentials.email", "other@example.test", "identity_conflict"},
		{"credentials.id_token", "not-a-jwt", "conversion_invalid_token"}, {"credentials.expires_at", true, "conversion_invalid_expiry"},
		{"credentials.access_token", "opaque-token", "conversion_invalid_token"}, {"credentials.refresh_token", "token with space", "conversion_invalid_token"},
		{"credentials.disabled", "false", "invalid_credentials"},
	} {
		t.Run(test.path+test.code, func(t *testing.T) {
			d := fixture(t)
			if err := d.Set(test.path, test.value); err != nil {
				t.Fatal(err)
			}
			r, err := Convert(d.Bytes(), time.Unix(1900000000, 0))
			if err != nil || r.Ready != 0 || r.Items[0].Error != test.code || r.Items[0].Credential != nil {
				t.Fatalf("expected %s: %v %+v", test.code, err, r)
			}
		})
	}
}

func TestConversionRequiresTokenIdentityAndChecksPresentIDToken(t *testing.T) {
	for _, test := range []struct {
		claims string
		code   string
	}{
		{`{"https://api.openai.com/auth":{"chatgpt_account_id":"account-1"}}`, "identity_unverifiable"},
		{`{"https://api.openai.com/auth":{"chatgpt_user_id":"user-1"}}`, "identity_unverifiable"},
		{`{"https://api.openai.com/auth":{"chatgpt_account_id":"account-1","chatgpt_user_id":12}}`, "identity_conflict"},
	} {
		d := fixture(t)
		_ = d.Set("credentials.account_id", "account-1")
		_ = d.Set("credentials.user_id", "user-1")
		_ = d.Set("credentials.access_token", "e30."+base64.RawURLEncoding.EncodeToString([]byte(test.claims))+".fixture")
		r, err := Convert(d.Bytes(), time.Now())
		if err != nil || r.Ready != 0 || r.Items[0].Error != test.code {
			t.Fatalf("unverifiable identity accepted: %v %+v", err, r)
		}
	}
	d := fixture(t)
	_ = d.Set("credentials.id_token", "e30."+base64.RawURLEncoding.EncodeToString([]byte(`{"https://api.openai.com/auth":{"chatgpt_account_id":"other-account"}}`))+".fixture")
	r, err := Convert(d.Bytes(), time.Now())
	if err != nil || r.Items[0].Error != "identity_conflict" {
		t.Fatal("conflicting ID token accepted")
	}
}

func TestConversionBoundsDuplicatesAndExpiry(t *testing.T) {
	for _, source := range []string{`{"accounts":[],"accounts":[]}`, `{"version":2,"accounts":[{}]}`, `[]`, strings.Repeat("[", 34) + "0" + strings.Repeat("]", 34), strings.Repeat(" ", MaxBytes+1)} {
		if _, err := Convert([]byte(source), time.Now()); err == nil {
			t.Fatal("invalid source accepted")
		}
	}
	d := fixture(t)
	row := d.String()
	r, err := Convert([]byte("["+row+","+row+"]"), time.Unix(1900000000, 0))
	if err != nil || r.Ready != 1 || r.Items[1].Error != "duplicate_identity" {
		t.Fatal("duplicate identity accepted")
	}
	if err := d.Set("credentials.expires_at", 1000); err != nil {
		t.Fatal(err)
	}
	if err := d.Set("disabled", true); err != nil {
		t.Fatal(err)
	}
	r, err = Convert(d.Bytes(), time.Now())
	if err != nil || r.Ready != 1 || r.Items[0].Credential["disabled"] != true || !strings.Contains(strings.Join(r.Items[0].Warnings, ","), "access_token_expired") {
		t.Fatal("expiry/disabled information lost")
	}
	if err := d.Delete("credentials.refresh_token"); err != nil {
		t.Fatal(err)
	}
	r, err = Convert(d.Bytes(), time.Now())
	if err != nil || r.Items[0].Error != "conversion_expired_without_refresh" {
		t.Fatal("expired nonrefreshable token accepted")
	}
}
