package login

import (
	"testing"
	"time"
)

func TestFailedAccountJobPreservesCredentialHealth(t *testing.T) {
	for _, tc := range []struct{ status, reason string }{
		{"not_linked", "login_required"},
		{"unknown", "no_auth_failure_observed"},
		{"needs_reauth", "unauthorized"},
		{"disabled", "manual_disabled"},
		{"missing", "credential_deleted"},
		{"quality_paused", "quality_paused"},
		{"unknown", "security_uncertain"},
	} {
		for _, automatic := range []bool{false, true} {
			t.Run(tc.reason+map[bool]string{false: "/manual", true: "/automatic"}[automatic], func(t *testing.T) {
				s := offlineService(t, offlineOptions(t))
				a := importOfflineAccount(t, s)
				s.accountMu.Lock()
				stored := s.accounts[a.ID]
				stored.View.HealthStatus, stored.View.HealthReason = tc.status, tc.reason
				stored.View.LastCheckedAt = s.opts.Now().Add(-time.Hour).Format(time.RFC3339)
				stored.View.SecurityPending = tc.reason == "security_uncertain"
				if err := s.saveAccountLocked(stored); err != nil {
					t.Fatal(err)
				}
				s.accountMu.Unlock()
				record := historyJob("failed-callback", a.ID, 1, s.opts.Now())
				record.View.Status, record.View.Automatic = "failed", automatic
				record.View.ErrorCode, record.View.ErrorStage = "invalid_callback", "callback"
				record.View.ErrorReason = "callback_state_mismatch"
				s.mu.Lock()
				s.jobs[record.View.ID] = &session{record: record}
				s.mu.Unlock()
				accounts, err := s.Accounts()
				if err != nil || len(accounts) != 1 {
					t.Fatalf("account read failed: %v", err)
				}
				got := accounts[0]
				if got.HealthStatus != tc.status || got.HealthReason != tc.reason || got.LastCheckedAt != stored.View.LastCheckedAt {
					t.Fatalf("login failure replaced the credential inspection: %s / %s", got.HealthStatus, got.HealthReason)
				}
				if got.LastJobErrorCode != "invalid_callback" || got.LastJobErrorStage != "callback" || got.LastJobErrorReason != "callback_state_mismatch" || !got.AttentionRequired {
					t.Fatal("separating health lost the operation diagnosis or attention flag")
				}
				if got.SecurityPending != stored.View.SecurityPending || automatic && got.ConsecutiveFailures != 1 {
					t.Fatal("separating health changed quarantine or automatic failure accounting")
				}
				s.accountMu.Lock()
				saved := s.accounts[a.ID]
				s.accountMu.Unlock()
				if saved.View.HealthReason != tc.reason || saved.Credentials != stored.Credentials {
					t.Fatal("synchronization rewrote credential health or account material")
				}
			})
		}
	}
}
