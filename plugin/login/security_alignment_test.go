package login

import (
	"bytes"
	"context"
	"errors"
	"os"
	"sync/atomic"
	"testing"
)

func alignedSecurityFixture(in RuntimeRequest) map[string]any {
	v := securityFixtureJournal(in)
	v["version"], v["initial_state"], v["initial_enroll_status"] = 3, "enabled", "not_requested"
	v["disable_confirmation"], v["local_commit"] = "business_receipt", "pending"
	v["logout_status"], v["phase"], v["status"] = "not_requested", "totp_commit", "running"
	v["first_enroll_status"] = "succeeded"
	v["confirmation_digest"] = (securityJournal{JobID: in.JobID, AccountID: "offline-account", Email: in.Credentials.Email, NewSecret: securityFixtureSeed, EnrollmentSession: "new-enrollment", NewFactor: "new-factor", Confirmation: "business_receipt"}).confirmationDigest()
	return v
}
func TestResetParentCommitsBeforeAcknowledgingLogout(t *testing.T) {
	opts := offlineOptions(t)
	var s *Service
	var acknowledged atomic.Bool
	opts.Runtime = offlineRuntime{run: func(_ context.Context, in RuntimeRequest) (string, error) {
		v := alignedSecurityFixture(in)
		if err := writeSecurityFixture(in, v); err != nil {
			return "", err
		}
		if in.commitSecurity == nil {
			return "", errors.New("missing durable barrier")
		}
		if err := in.commitSecurity(); err != nil {
			return "", err
		}
		acknowledged.Store(true)
		s.accountMu.Lock()
		for _, a := range s.accounts {
			if a.Credentials.TOTPSecret != securityFixtureSeed || a.Credentials.TOTPFactorID != "new-factor" || !a.View.SecurityPending {
				t.Error("ack preceded durable seed/binding or released account")
			}
		}
		s.accountMu.Unlock()
		v["local_commit"], v["logout_status"], v["status"], v["phase"] = "committed", "succeeded", "succeeded", "security_complete"
		return "", writeSecurityFixture(in, v)
	}}
	s = offlineService(t, opts)
	a := importOfflineAccount(t, s)
	job, err := s.StartAccountSecurity(a.ID, a.Revision, "reset_totp", true, false)
	if err != nil {
		t.Fatal(err)
	}
	job = waitLogin(t, s, job.ID, "succeeded")
	detail, err := s.AccountDetail(a.ID)
	if err != nil || !acknowledged.Load() || !job.TOTPReset || !job.DevicesLoggedOut || detail.Account.SecurityPending || detail.Credentials.TOTPFactorID != "new-factor" {
		t.Fatalf("commit sequence incomplete: %v", err)
	}
}

func TestFailedLocalBarrierRequiresExplicitOfflineCompletion(t *testing.T) {
	opts := offlineOptions(t)
	var runs atomic.Int32
	opts.Runtime = offlineRuntime{run: func(_ context.Context, in RuntimeRequest) (string, error) {
		runs.Add(1)
		v := alignedSecurityFixture(in)
		// Simulate interruption after the activation receipt and before parent ack.
		if err := writeSecurityFixture(in, v); err != nil {
			return "", err
		}
		return "", errors.New("storage_unavailable")
	}}
	s := offlineService(t, opts)
	a := importOfflineAccount(t, s)
	job, err := s.StartAccountSecurity(a.ID, a.Revision, "reset_totp", true, true)
	if err != nil {
		t.Fatal(err)
	}
	waitLogin(t, s, job.ID, "security_uncertain")
	path, _ := s.securityJournalPath(job.ID, false)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s = offlineService(t, opts)
	detail, err := s.AccountDetail(a.ID)
	if err != nil || !detail.Account.SecurityPending || detail.SecurityRecovery == nil || !detail.SecurityRecovery.CanComplete || detail.Credentials.TOTPSecret == securityFixtureSeed {
		t.Fatal("restart silently completed the local barrier")
	}
	if _, err = s.CompleteAccountSecurity(a.ID, detail.Account.Revision, job.ID); err != nil {
		t.Fatal(err)
	}
	detail, err = s.AccountDetail(a.ID)
	after, _ := os.ReadFile(path)
	if err != nil || detail.Account.SecurityPending || detail.Credentials.TOTPSecret != securityFixtureSeed || detail.Credentials.TOTPFactorID != "new-factor" || runs.Load() != 1 || !bytes.Equal(before, after) {
		t.Fatal("offline completion lost binding, rewrote historical evidence or started runtime")
	}
}

func TestEnrollmentWithoutOldFactorPromotesOnlyBoundEvidence(t *testing.T) {
	for _, mode := range []string{"direct", "conflict-disabled", "conflict-unproven", "missing-read-evidence", "mixed-secret", "missing-digest"} {
		t.Run(mode, func(t *testing.T) {
			opts := offlineOptions(t)
			opts.Runtime = offlineRuntime{run: func(_ context.Context, in RuntimeRequest) (string, error) {
				v := alignedSecurityFixture(in)
				v["old_factor_id"], v["disable_status"], v["disable_confirmation"] = "", "not_requested", ""
				v["initial_state"], v["initial_enroll_status"], v["first_enroll_status"] = "disabled", "succeeded", "not_requested"
				if mode == "conflict-disabled" || mode == "conflict-unproven" {
					v["initial_enroll_status"], v["first_enroll_status"] = "conflict", "succeeded"
				}
				if mode == "conflict-disabled" {
					v["post_conflict_state"] = "disabled"
				}
				if mode == "missing-read-evidence" {
					v["activation_confirmation"] = "state_read"
				}
				if mode == "mixed-secret" {
					v["new_totp_secret"] = "JBSWY3DPEHPK3PXP"
				}
				if mode == "missing-digest" {
					delete(v, "confirmation_digest")
				}
				if err := writeSecurityFixture(in, v); err != nil {
					return "", err
				}
				if err := in.commitSecurity(); err != nil {
					return "", err
				}
				v["local_commit"], v["status"], v["phase"] = "committed", "succeeded", "security_complete"
				return "", writeSecurityFixture(in, v)
			}}
			s := offlineService(t, opts)
			a := importOfflineAccount(t, s)
			job, err := s.StartAccountSecurity(a.ID, a.Revision, "reset_totp", false, false)
			if err != nil {
				t.Fatal(err)
			}
			success := mode == "direct" || mode == "conflict-disabled"
			wanted := "security_uncertain"
			if success {
				wanted = "succeeded"
			}
			waitLogin(t, s, job.ID, wanted)
			detail, err := s.AccountDetail(a.ID)
			if err != nil || (detail.Credentials.TOTPSecret == securityFixtureSeed) != success {
				t.Fatal("invalid evidence promotion")
			}
		})
	}
}

func TestVerifiedLoginFactorIsSavedWithMatchingSeed(t *testing.T) {
	opts := offlineOptions(t)
	opts.Runtime = offlineRuntime{run: func(_ context.Context, in RuntimeRequest) (string, error) {
		if err := in.factorVerified("verified-factor"); err != nil {
			return "", err
		}
		return "", errors.New("runtime_failed")
	}}
	s := offlineService(t, opts)
	a := importOfflineAccount(t, s)
	job, err := s.StartAccountSecurity(a.ID, a.Revision, "logout_all", false, false)
	if err != nil {
		t.Fatal(err)
	}
	waitLogin(t, s, job.ID, "security_uncertain")
	detail, err := s.AccountDetail(a.ID)
	if err != nil || detail.Credentials.TOTPFactorID != "verified-factor" {
		t.Fatal("verified binding was lost after later failure")
	}
}
