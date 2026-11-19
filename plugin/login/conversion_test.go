package login

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/MoeForge/moego/base64"
	"github.com/MoeForge/moego/ojson"
	"github.com/cliproxyapi-plugins/model-quality/login/account"
	"github.com/cliproxyapi-plugins/model-quality/login/conversion"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

func conversionFixture(t *testing.T) string {
	t.Helper()
	token := offlineBundle(t, time.Now(), "offline-account").TokenData.AccessToken
	d := ojson.NewDocString(`{"type":"sub2api-data","version":1,"accounts":[{"platform":"openai","type":"oauth","name":"Offline conversion","credentials":{"refresh_token":"CONVERSION_PRIVATE_REFRESH","plan_type":"plus"}}]}`)
	if err := d.Set("accounts.0.credentials.access_token", token); err != nil {
		t.Fatal(err)
	}
	return d.String()
}

func TestConversionRefusesExistingFilesAndChangedPolicy(t *testing.T) {
	for _, mode := range []string{"filename", "identity", "prepare", "policy-changed", "runtime-missing", "runtime-disabled"} {
		t.Run(mode, func(t *testing.T) {
			h := newFakeHost()
			opts := offlineOptions(t)
			opts.Call = h.call
			var s *Service
			opts.Prepare = func(context.Context, string, string) error {
				if mode == "prepare" {
					return errors.New("policy_changed")
				}
				if mode == "policy-changed" {
					s.uploadExcludedPlans.Store([]string{"plus"})
				}
				return nil
			}
			s = offlineService(t, opts)
			source := conversionFixture(t)
			converted, err := conversion.Convert([]byte(source), time.Now())
			if err != nil {
				t.Fatal(err)
			}
			item := converted.Items[0]
			want, saves := "duplicate_identity", 0
			switch mode {
			case "filename", "identity":
				name := item.Filename
				if mode == "identity" {
					name = "existing.json"
				}
				h.files[name] = item.Credential
				h.entries[name] = account.Entry{ID: name, Index: name, Name: name, Provider: "codex", Status: "active"}
			case "prepare":
				want = "policy_changed"
			case "policy-changed":
				want = "cpa_plan_excluded"
			case "runtime-missing":
				h.adoptError = true
				want, saves = "saved_not_adopted", 1
			case "runtime-disabled":
				h.runtimeDisabled = true
				want, saves = "saved_not_adopted", 1
			}
			r := conversionRequest(t, s, "upload", "conversion-guard", source)
			if ojson.GetBytes(r.Body, "items.0.code").String() != want || h.saves != saves {
				t.Fatalf("guard failed %s %d", r.Body, h.saves)
			}
			if saves == 1 {
				h.adoptError, h.runtimeDisabled = false, false
				r = conversionRequest(t, s, "upload", "conversion-guard-retry", source)
				if h.saves != 1 || ojson.GetBytes(r.Body, "items.0.status").String() != "uncertain" {
					t.Fatal("unadopted save replayed")
				}
			}
		})
	}
}

func TestConversionBlocksPendingIdentityWithoutEmail(t *testing.T) {
	for _, mode := range []string{"account", "job"} {
		t.Run(mode, func(t *testing.T) {
			h := newFakeHost()
			opts := offlineOptions(t)
			opts.Call = h.call
			s := offlineService(t, opts)
			if mode == "account" {
				a := importOfflineAccount(t, s)
				s.accountMu.Lock()
				stored := s.accounts[a.ID]
				stored.Principal = (account.LoginPrincipal{AccountID: "offline-account", UserID: "offline-user"}).Key()
				stored.View.SecurityPending = true
				err := s.saveAccountLocked(stored)
				s.accountMu.Unlock()
				if err != nil {
					t.Fatal(err)
				}
			} else {
				s.mu.Lock()
				s.jobs["pending"] = &session{record: storedJob{SecurityAccountID: "offline-account", View: Job{ID: "pending", Action: "reset_totp", Status: "security_uncertain"}}}
				s.mu.Unlock()
			}
			d := ojson.NewDocString(conversionFixture(t))
			claims := `{"https://api.openai.com/auth":{"chatgpt_account_id":"offline-account","chatgpt_user_id":"offline-user"}}`
			_ = d.Set("accounts.0.credentials.access_token", "e30."+base64.RawURLEncoding.EncodeToString([]byte(claims))+".offline")
			r := conversionRequest(t, s, "upload", "conversion-no-email", d.String())
			if h.saves != 0 || ojson.GetBytes(r.Body, "items.0.code").String() != "security_pending" {
				t.Fatalf("security identity bypass: %s", r.Body)
			}
		})
	}
}

func conversionRequest(t *testing.T, s *Service, path, key, source string) pluginapi.ManagementResponse {
	t.Helper()
	body := map[string]any{"source": source}
	if path == "upload" {
		body["indexes"] = []int{0}
	}
	raw, err := Encode(body)
	if err != nil {
		t.Fatal(err)
	}
	return s.Handle(context.Background(), pluginapi.ManagementRequest{Method: http.MethodPost, Path: RoutePrefix + "/conversion/" + path, Body: raw, Headers: http.Header{"Idempotency-Key": {key}}})
}

func TestConversionPreviewAndUploadAreOfflineAndIdempotent(t *testing.T) {
	h := newFakeHost()
	opts := offlineOptions(t)
	opts.Call = h.call
	opts.Runtime = offlineRuntime{run: func(context.Context, RuntimeRequest) (string, error) {
		t.Error("conversion started login runtime")
		return "", nil
	}}
	s := offlineService(t, opts)
	source := conversionFixture(t)
	preview := conversionRequest(t, s, "preview", "", source)
	if preview.StatusCode != 200 || ojson.GetBytes(preview.Body, "ready").Int() != 1 || h.saves != 0 {
		t.Fatalf("preview failed: %d", preview.StatusCode)
	}
	if ojson.GetBytes(preview.Body, "items.0.credential.id_token").Exists() {
		t.Fatal("fabricated ID token")
	}
	for _, key := range []string{"conversion-one", "conversion-one", "conversion-new-key"} {
		r := conversionRequest(t, s, "upload", key, source)
		if r.StatusCode != 200 || ojson.GetBytes(r.Body, "items.0.status").String() != "uploaded" {
			t.Fatalf("upload failed: %d %s", r.StatusCode, r.Body)
		}
	}
	if h.saves != 1 {
		t.Fatal("conversion repeated credential write")
	}
	if accounts, err := s.Accounts(); err != nil || len(accounts) != 0 {
		t.Fatal("token import created password/2FA account")
	}
	if all, err := s.opts.Manager.ListChecked(); err != nil || len(all) != 1 {
		t.Fatalf("access-token-only credential broke subsequent account scans: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(opts.Dir + "/credential-login.db")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("CONVERSION_PRIVATE_REFRESH")) {
		t.Fatal("preview or upload journal retained credential tokens")
	}
	s = offlineService(t, opts)
	r := conversionRequest(t, s, "upload", "conversion-reopened", source)
	if r.StatusCode != 200 || h.saves != 1 || ojson.GetBytes(r.Body, "items.0.status").String() != "uploaded" {
		t.Fatal("restart repeated the import")
	}
	changed := ojson.NewDocString(source)
	_ = changed.Set("accounts.0.credentials.refresh_token", "NEW_OFFLINE_REFRESH")
	r = conversionRequest(t, s, "upload", "conversion-new-token", changed.String())
	if ojson.GetBytes(r.Body, "items.0.code").String() != "duplicate_identity" || h.saves != 1 {
		t.Fatal("changed credential was reported as uploaded or overwrote existing file")
	}
}

func TestConversionUnknownSaveNeverReplaysAfterRestart(t *testing.T) {
	h := newFakeHost()
	h.saveError = true
	opts := offlineOptions(t)
	opts.Call = h.call
	s := offlineService(t, opts)
	source := conversionFixture(t)
	r := conversionRequest(t, s, "upload", "conversion-lost-ack", source)
	if ojson.GetBytes(r.Body, "items.0.status").String() != "uncertain" || h.saves != 1 {
		t.Fatal("lost save acknowledgment was not quarantined")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	h.saveError = false
	s = offlineService(t, opts)
	r = conversionRequest(t, s, "upload", "conversion-retry-new", source)
	if ojson.GetBytes(r.Body, "items.0.status").String() != "uncertain" || h.saves != 1 {
		t.Fatal("unknown import was replayed after restart")
	}
}

func TestConversionHonorsPolicyAndPendingSecurity(t *testing.T) {
	for _, mode := range []string{"plan", "security", "invalid-selection", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			h := newFakeHost()
			opts := offlineOptions(t)
			opts.Call = h.call
			s := offlineService(t, opts)
			source := conversionFixture(t)
			want := ""
			switch mode {
			case "plan":
				settings, _ := s.AccountSettings()
				settings.CPAUploadExcludedPlans = []string{"plus"}
				if _, err := s.UpdateAccountSettings(settings); err != nil {
					t.Fatal(err)
				}
				want = "cpa_plan_excluded"
			case "security":
				a := importOfflineAccount(t, s)
				s.accountMu.Lock()
				stored := s.accounts[a.ID]
				stored.View.SecurityPending = true
				err := s.saveAccountLocked(stored)
				s.accountMu.Unlock()
				if err != nil {
					t.Fatal(err)
				}
				want = "security_pending"
			case "invalid-selection":
				source = `{"accounts":[{"platform":"anthropic","type":"oauth","credentials":{"access_token":"offline"}}]}`
			case "disabled":
				d := ojson.NewDocString(source)
				_ = d.Set("accounts.0.disabled", true)
				source = d.String()
			}
			r := conversionRequest(t, s, "upload", "conversion-policy-case", source)
			if mode == "invalid-selection" {
				if r.StatusCode != 400 || h.saves != 0 {
					t.Fatal("invalid selection wrote credentials")
				}
				return
			}
			if mode == "disabled" {
				if h.saves != 1 || ojson.GetBytes(r.Body, "items.0.enabled").Bool() {
					t.Fatal("conversion enabled a disabled credential")
				}
				return
			}
			if ojson.GetBytes(r.Body, "items.0.code").String() != want || h.saves != 0 {
				t.Fatalf("policy guard not honored: %s", r.Body)
			}
		})
	}
}
