package login

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/cli-proxy-api/model-quality-login-runtime/internal/transport"
)

type resetLedgerEntry struct {
	path   string
	body   payload
	status int
	lost   bool
}
type resetLedger struct {
	t       *testing.T
	entries []resetLedgerEntry
	calls   []string
}

func (l *resetLedger) Do(_ context.Context, r transport.Request) (*transport.Response, error) {
	path := strings.TrimPrefix(r.URL, chatOrigin)
	l.calls = append(l.calls, path)
	if len(l.entries) == 0 {
		l.t.Fatalf("unexpected request %s", path)
	}
	next := l.entries[0]
	l.entries = l.entries[1:]
	if next.path != path {
		l.t.Fatalf("got %s want %s", path, next.path)
	}
	if next.lost {
		return nil, errors.New("synthetic response loss")
	}
	raw, err := encodePayload(next.body)
	if err != nil {
		l.t.Fatal(err)
	}
	status := next.status
	if status == 0 {
		status = 200
	}
	return &transport.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: raw, BodyComplete: true}, nil
}
func alignmentState(ids ...string) payload {
	list := []any{}
	for _, id := range ids {
		list = append(list, payload{"id": id, "factor_type": "totp", "is_recovery": false})
	}
	return payload{"mfa_enabled_v2": len(ids) > 0, "factors": payload{"totp": list}}
}
func TestReferenceResetRequestLedger(t *testing.T) {
	get := func(body payload) resetLedgerEntry { return resetLedgerEntry{path: mfaInfoEndpoint, body: body} }
	disable := resetLedgerEntry{path: disableEndpoint, body: payload{"message": "Factor disabled", "passkey_credential_manager_sync": nil}}
	enroll := resetLedgerEntry{path: enrollEndpoint, body: payload{"secret": securityNewSeed, "session_id": "new-session", "factor": payload{"id": "new-factor", "factor_type": "totp", "is_recovery": false}}}
	activate := resetLedgerEntry{path: activateEndpoint, body: payload{"success": true, "factor_id": "new-factor", "session_id": "new-session"}}
	conflict := resetLedgerEntry{path: enrollEndpoint, status: 409, body: payload{"error": payload{"code": "totp_already_enrolled"}}}
	lostActivation := resetLedgerEntry{path: activateEndpoint, lost: true}
	tests := []struct {
		name, binding                  string
		entries                        []resetLedgerEntry
		success, logout, commitFailure bool
	}{
		{name: "empty", entries: []resetLedgerEntry{get(alignmentState()), enroll, activate}, success: true},
		{name: "old", binding: "old-factor", entries: []resetLedgerEntry{get(alignmentState("old-factor")), disable, enroll, activate}, success: true},
		{name: "missing-metadata", entries: []resetLedgerEntry{get(payload{}), get(payload{}), get(payload{}), get(payload{}), enroll, activate}, success: true},
		{name: "conflict-bound", binding: "old-factor", entries: []resetLedgerEntry{get(alignmentState()), conflict, get(alignmentState("old-factor")), disable, enroll, activate}, success: true},
		{name: "conflict-disabled", entries: []resetLedgerEntry{get(alignmentState()), conflict, get(alignmentState()), enroll, activate}, success: true},
		{name: "conflict-unbound", entries: []resetLedgerEntry{get(alignmentState()), conflict, get(alignmentState("unrelated"))}},
		{name: "conflict-new-id", binding: "old-factor", entries: []resetLedgerEntry{get(alignmentState()), conflict, get(alignmentState("unrelated"))}},
		{name: "second-conflict", binding: "old-factor", entries: []resetLedgerEntry{get(alignmentState()), conflict, get(alignmentState("old-factor")), disable, conflict}},
		{name: "after-removal-conflict-disabled", binding: "old-factor", entries: []resetLedgerEntry{get(alignmentState("old-factor")), disable, conflict, get(alignmentState()), enroll, activate}, success: true},
		{name: "after-removal-conflict-active", binding: "old-factor", entries: []resetLedgerEntry{get(alignmentState("old-factor")), disable, conflict, get(alignmentState("other-factor"))}},
		{name: "after-removal-second-conflict", binding: "old-factor", entries: []resetLedgerEntry{get(alignmentState("old-factor")), disable, conflict, get(alignmentState()), conflict}},
		{name: "activation-wrong-account", entries: []resetLedgerEntry{get(alignmentState()), enroll, {path: activateEndpoint, body: payload{"success": true, "account_id": "different-account"}}}},
		{name: "activation-crossed-signals", entries: []resetLedgerEntry{get(alignmentState()), enroll, {path: activateEndpoint, body: payload{"success": true, "message": "Factor disabled"}}, get(alignmentState("unrelated"))}},
		{name: "ambiguous-409", entries: []resetLedgerEntry{get(alignmentState()), {path: enrollEndpoint, status: 409, body: payload{"message": "possibly exists"}}}},
		{name: "lost-enrollment", entries: []resetLedgerEntry{get(alignmentState()), {path: enrollEndpoint, lost: true}}},
		{name: "concurrent-factor-activation", entries: []resetLedgerEntry{get(alignmentState()), enroll, lostActivation, get(alignmentState("other-factor", "new-factor"))}, success: true},
		{name: "duplicate-factor-activation", entries: []resetLedgerEntry{get(alignmentState()), enroll, lostActivation, get(alignmentState("new-factor", "new-factor"))}},
		{name: "multiple-old-factors", binding: "old-factor", entries: []resetLedgerEntry{get(alignmentState("old-factor", "other-factor"))}},
		{name: "wrong-old-factor", binding: "old-factor", entries: []resetLedgerEntry{get(alignmentState("other-factor"))}},
		{name: "logout-after-commit", entries: []resetLedgerEntry{get(alignmentState()), enroll, activate, {path: logoutEndpoint, body: payload{}}}, success: true, logout: true},
		{name: "failed-local-commit", entries: []resetLedgerEntry{get(alignmentState()), enroll, activate}, logout: true, commitFailure: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, _ := securityFlow(t, "", "reset_totp")
			f.in.Credentials.TOTPFactorID = tc.binding
			f.in.LogoutAfterReset = tc.logout
			ledger := &resetLedger{t: t, entries: append([]resetLedgerEntry{}, tc.entries...)}
			f.client = NewClient(ledger, nil, "chrome146")
			commits := 0
			f.p.commit = func(context.Context) error {
				commits++
				journal := readSecurityTestJournal(t, f.security.path)
				if journal.ActivationStatus != "succeeded" || journal.NewFactorID != "new-factor" || journal.NewTOTPSecret != securityNewSeed || journal.LogoutStatus != "not_requested" {
					t.Fatal("commit lacked durable activation or followed logout")
				}
				ledger.calls = append(ledger.calls, "LOCAL_COMMIT")
				if tc.commitFailure {
					return errors.New("synthetic readback failure")
				}
				return nil
			}
			err := f.runSecurity(t.Context())
			if (err == nil) != tc.success {
				t.Fatalf("success=%v err=%v", tc.success, err)
			}
			if len(ledger.entries) != 0 {
				t.Fatalf("missing requests: %v", ledger.entries)
			}
			if tc.success && commits != 1 {
				t.Fatal("local commit not performed exactly once")
			}
			if tc.logout && tc.success && !reflect.DeepEqual(ledger.calls[len(ledger.calls)-2:], []string{"LOCAL_COMMIT", logoutEndpoint}) {
				t.Fatal("logout preceded local commit")
			}
			j := readSecurityTestJournal(t, f.security.path)
			if tc.name == "concurrent-factor-activation" && (j.ActivationConfirmation != "state_read" || j.ActivationVerifiedAt == "") {
				t.Fatal("missing actual read evidence")
			}
			if _, err := newSecurityJournal(f.in); err == nil {
				t.Fatal("restart could replay operation")
			}
		})
	}
}

func TestReferenceFactorSelection(t *testing.T) {
	p := payload{"oai-client-auth-session": payload{"mfa_challenge_factors": []any{payload{"id": "first", "factor_type": "totp"}, payload{"id": "bound", "factor_type": "totp"}, payload{"id": "recovery", "factor_type": "totp", "is_recovery": true}}}}
	if id, err := selectTOTPLoginFactor(p, "bound"); err != nil || id != "bound" {
		t.Fatal("did not select bound factor")
	}
	for _, bound := range []string{"", "missing", "recovery"} {
		if _, err := selectTOTPLoginFactor(p, bound); err == nil {
			t.Fatal("ambiguous or unbound selection accepted")
		}
	}
}

func TestResetRequiresParentCommitBeforeLogin(t *testing.T) {
	fixture := newLoginFixture(t, "")
	in := securityInput(t, "reset_totp")
	fixture.in.Operation = in.Operation
	fixture.in.SecurityJournalPath = in.SecurityJournalPath
	if _, err := run(t.Context(), fixture.in, nil, fixture.p); err == nil {
		t.Fatal("missing parent persistence callback accepted")
	}
}

func TestReferenceReceiptContradictions(t *testing.T) {
	for _, body := range []payload{
		{"success": true, "activated": true, "message": "Factor disabled"},
		{"success": true, "request_id": nil},
		{"success": true, "secret": "unexpected"},
		{"success": true, "otpauth_uri": nil},
		{"success": true, "new_totp_factor_id": "new-factor"},
		{"success": true, "enrollment_factor": payload{}},
	} {
		for _, step := range []string{"disable", "activation"} {
			if classifyTOTPMutation(step, body, "new-factor", "new-session") != totpMutationUnknown {
				t.Fatalf("accepted contradictory %s receipt", step)
			}
		}
	}
}
