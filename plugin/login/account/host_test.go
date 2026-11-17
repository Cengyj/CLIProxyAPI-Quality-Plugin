package account

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MoeForge/moego/base64"
)

type rawHostFixture struct {
	raw   json.RawMessage
	entry Entry
	saves int
}

func newRawHostFixture() *rawHostFixture {
	claims := `{"sub":"fixture-user","email":"fixture@example.test","https://api.openai.com/auth":{"chatgpt_account_id":"fixture-account","chatgpt_user_id":"fixture-user"}}`
	token := "e30." + base64.RawURLEncoding.EncodeToString([]byte(claims)) + ".fixture"
	raw, _ := json.Marshal(map[string]any{"type": "codex", "id_token": token, "access_token": token, "refresh_token": "fixture-old", "policy": json.RawMessage(`{"integer":9007199254740993123456789,"fraction":0.12345678901234567890123456789}`)})
	return &rawHostFixture{raw: raw, entry: Entry{ID: "fixture.json", Index: "fixture-index", Name: "fixture.json", Provider: "codex", Status: "active"}}
}

func (h *rawHostFixture) call(method string, in, out any) error {
	var result any
	switch method {
	case "host.auth.list":
		result = map[string]any{"files": []Entry{h.entry}}
	case "host.auth.get":
		result = map[string]any{"name": h.entry.Name, "json": h.raw}
	case "host.auth.get_runtime":
		result = map[string]any{"auth": h.entry}
	case "host.auth.save":
		raw, err := json.Marshal(in)
		if err != nil {
			return err
		}
		var request struct {
			Name string          `json:"name"`
			JSON json.RawMessage `json:"json"`
		}
		if err = json.Unmarshal(raw, &request); err != nil {
			return err
		}
		h.saves++
		h.raw = append([]byte(nil), request.JSON...)
		var saved struct {
			Disabled bool `json:"disabled"`
		}
		if err = json.Unmarshal(request.JSON, &saved); err != nil {
			return err
		}
		h.entry.Disabled = saved.Disabled
		h.entry.Status = "active"
		if saved.Disabled {
			h.entry.Status = "disabled"
		}
		result = map[string]any{"name": request.Name}
	default:
		return errors.New("unexpected callback")
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func newFixtureManager(t *testing.T, h *rawHostFixture) *Manager {
	t.Helper()
	m, err := New(filepath.Join(t.TempDir(), "login"), h.call)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Close() })
	return m
}

func TestCallbackCommitPreservesExactPolicyNumbers(t *testing.T) {
	h := newRawHostFixture()
	m := newFixtureManager(t, h)
	before, err := m.LoginSnapshot(h.entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	candidate, ok := m.GetByID(h.entry.ID)
	if !ok {
		t.Fatal("fixture read failed")
	}
	candidate.Metadata["refresh_token"] = "fixture-new"
	result, err := m.CommitLoginCandidate(context.Background(), LoginCommitRequest{CommitID: "fixture-commit", Candidate: candidate, Expected: before})
	if err != nil || result.State != "committed" || h.saves != 1 {
		t.Fatalf("commit did not complete once: %+v %v", result, err)
	}
	for _, number := range []string{"9007199254740993123456789", "0.12345678901234567890123456789"} {
		if !strings.Contains(string(h.raw), number) {
			t.Fatal("credential policy number lost precision")
		}
	}
}

func TestCallbackCommitPreservesStatusOnlyDisabled(t *testing.T) {
	h := newRawHostFixture()
	h.entry.Status = "disabled"
	h.entry.Disabled = false
	m := newFixtureManager(t, h)
	before, err := m.LoginSnapshot(h.entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	candidate, ok := m.GetByID(h.entry.ID)
	if !ok || !candidate.Disabled {
		t.Fatal("official disabled status was not treated as a restriction")
	}
	health, err := m.LoginHealth(h.entry.ID)
	if err != nil || health.Status != "disabled" || health.AutoEligible {
		t.Fatal("status-only disabled credential became eligible for automatic login")
	}
	candidate.Metadata["refresh_token"] = "fixture-new"
	result, err := m.CommitLoginCandidate(context.Background(), LoginCommitRequest{CommitID: "fixture-disabled-commit", Candidate: candidate, Expected: before})
	if err != nil || result.State != "committed" || h.saves != 1 {
		t.Fatalf("commit did not complete once: %+v %v", result, err)
	}
	var saved struct {
		Disabled bool `json:"disabled"`
	}
	if err := json.Unmarshal(h.raw, &saved); err != nil || !saved.Disabled {
		t.Fatal("reauthorization cleared the official disabled status")
	}
}

func TestDisabledStatusChangeInvalidatesLoginSnapshot(t *testing.T) {
	h := newRawHostFixture()
	m := newFixtureManager(t, h)
	before, err := m.LoginSnapshot(h.entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	candidate, ok := m.GetByID(h.entry.ID)
	if !ok {
		t.Fatal("fixture read failed")
	}
	h.entry.Status = "disabled"
	result, err := m.CommitLoginCandidate(context.Background(), LoginCommitRequest{CommitID: "fixture-stale-commit", Candidate: candidate, Expected: before})
	if err == nil || err.Error() != "policy_changed" || result.State != "aborted" || h.saves != 0 {
		t.Fatal("status-only restriction changed without invalidating the pending candidate")
	}
}

func TestHealthRequiresExactHostFailureEvidence(t *testing.T) {
	cases := []struct {
		name, message, status string
		disabled, cooldown    bool
		want                  bool
	}{
		{name: "official refresh failure", message: "unauthorized (refresh token invalid)", status: "error", want: true},
		{name: "machine code", message: "invalid_grant", status: "error", want: true},
		{name: "nested machine code", message: `{"error":{"code":"requires_sign_in"}}`, status: "error", want: true},
		{name: "generic expired", message: "token expired", status: "error"},
		{name: "message contains code", message: "quota unavailable: unauthorized", status: "error"},
		{name: "retry in progress", message: "invalid grant (retrying)", status: "error"},
		{name: "active", message: "invalid_grant", status: "active"},
		{name: "disabled", message: "invalid_grant", status: "error", disabled: true},
		{name: "cooldown", message: "invalid_grant", status: "error", cooldown: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := newRawHostFixture()
			h.entry.StatusMessage, h.entry.Status, h.entry.Disabled = tc.message, tc.status, tc.disabled
			if tc.cooldown {
				h.entry.NextRetryAfter = time.Now().Add(time.Hour)
			}
			m := newFixtureManager(t, h)
			health, err := m.LoginHealth(h.entry.ID)
			if err != nil || health.AutoEligible != tc.want {
				t.Fatalf("unexpected health: %+v %v", health, err)
			}
		})
	}
}

func TestCanonicalPolicyRejectsDuplicateFields(t *testing.T) {
	for _, raw := range []string{`{"weight":1,"weight":2}`, `{"policy":{"weight":1,"weight":2}}`} {
		if _, err := canonicalJSON([]byte(raw)); err == nil {
			t.Fatal("ambiguous policy accepted")
		}
	}
}

func TestCommitRechecksGuardAndPolicyImmediatelyBeforeSave(t *testing.T) {
	for _, change := range []string{"permission", "policy"} {
		t.Run(change, func(t *testing.T) {
			h := newRawHostFixture()
			m := newFixtureManager(t, h)
			before, err := m.LoginSnapshot(h.entry.ID)
			if err != nil {
				t.Fatal(err)
			}
			candidate, ok := m.GetByID(h.entry.ID)
			if !ok {
				t.Fatal("fixture read failed")
			}
			checks := 0
			result, err := m.CommitLoginCandidate(context.Background(), LoginCommitRequest{CommitID: "last-guard", Candidate: candidate, Expected: before, EnableCredential: true, Guard: func() error {
				checks++
				if checks == 3 {
					if change == "permission" {
						return errors.New("automation_disabled")
					}
					h.entry.Status = "disabled"
				}
				return nil
			}})
			if err == nil || result.State != "aborted" || result.CredentialEnabled || h.saves != 0 {
				t.Fatal("late change did not abort before writing")
			}
			receipt, readErr := m.LoginCommitStatus("last-guard")
			if readErr != nil || receipt.State != "aborted" {
				t.Fatal("pre-write abortion did not replace the unresolved receipt")
			}
		})
	}
}

func TestCommitContextCancelledDuringFinalSnapshotDoesNotSave(t *testing.T) {
	h := newRawHostFixture()
	m := newFixtureManager(t, h)
	before, err := m.LoginSnapshot(h.entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	candidate, ok := m.GetByID(h.entry.ID)
	if !ok {
		t.Fatal("fixture read failed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reads := 0
	m.call = func(method string, in, out any) error {
		err := h.call(method, in, out)
		if method == "host.auth.get" {
			reads++
			if reads == 3 {
				cancel()
			}
		}
		return err
	}
	result, err := m.CommitLoginCandidate(ctx, LoginCommitRequest{CommitID: "cancelled-final-read", Candidate: candidate, Expected: before, EnableCredential: true})
	if err == nil || err.Error() != "cancelled" || result.State != "aborted" || h.saves != 0 {
		t.Fatal("cancelled commit started a write after its final read")
	}
}
