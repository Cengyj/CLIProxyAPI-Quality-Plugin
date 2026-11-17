package login

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MoeForge/moego/ojson"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

const securityFixtureSeed = "KRSXG5DSNFXGOIDB"

func securityFixtureJournal(in RuntimeRequest) map[string]any {
	v := map[string]any{
		"version": 1, "job_id": in.JobID, "attempt": in.Attempt,
		"operation": in.Operation, "email": in.Credentials.Email,
		"phase": "security_complete", "status": "succeeded", "account_id": "offline-account",
		"disable_status": "not_requested", "enroll_status": "not_requested",
		"activation_status": "not_requested", "logout_status": "not_requested",
	}
	if in.Operation == "reset_totp" {
		v["old_factor_id"], v["new_factor_id"] = "old-factor", "new-factor"
		v["new_totp_secret"], v["enrollment_session_id"] = securityFixtureSeed, "new-enrollment"
		v["disable_status"], v["enroll_status"], v["activation_status"] = "succeeded", "succeeded", "succeeded"
		v["activation_confirmation"] = "business_receipt"
	}
	if in.Operation == "logout_all" || in.LogoutAfterReset {
		v["logout_status"] = "succeeded"
	}
	return v
}

func writeSecurityFixture(in RuntimeRequest, v map[string]any) error {
	if in.SecurityJournalPath == "" || !filepath.IsAbs(in.SecurityJournalPath) {
		return errors.New("missing private security journal path")
	}
	if err := os.MkdirAll(filepath.Dir(in.SecurityJournalPath), 0700); err != nil {
		return err
	}
	raw, err := Encode(v)
	if err != nil {
		return err
	}
	return os.WriteFile(in.SecurityJournalPath, raw, 0600)
}

func securityRequest(t *testing.T, s *Service, idempotency, path string, body any) pluginapi.ManagementResponse {
	t.Helper()
	raw, err := Encode(body)
	if err != nil {
		t.Fatal(err)
	}
	return s.Handle(context.Background(), pluginapi.ManagementRequest{
		Method: http.MethodPost, Path: RoutePrefix + path,
		Headers: http.Header{"Idempotency-Key": {idempotency}}, Body: raw,
	})
}

func TestSecurityResetPersistsNewSeedWithoutOAuthOrCPAWrite(t *testing.T) {
	h := newFakeHost()
	opts := offlineOptions(t)
	opts.Call = h.call
	var calls atomic.Int32
	opts.Runtime = offlineRuntime{run: func(_ context.Context, in RuntimeRequest) (string, error) {
		calls.Add(1)
		if in.Operation != "reset_totp" || in.Reauthorize || in.LogoutAfterReset {
			return "", errors.New("unexpected security operation")
		}
		return "", writeSecurityFixture(in, securityFixtureJournal(in))
	}}
	s := offlineService(t, opts)
	a := importOfflineAccount(t, s)
	job, err := s.StartAccountSecurity(a.ID, a.Revision, "reset_totp", false, false)
	if err != nil {
		t.Fatal(err)
	}
	job = waitLogin(t, s, job.ID, "succeeded")
	if !job.TOTPReset || job.DevicesLoggedOut || job.Reauthorize || job.CredentialEnabled {
		t.Fatal("reset-only result claimed an unrelated operation")
	}
	detail, err := s.AccountDetail(a.ID)
	if err != nil || detail.Credentials.TOTPSecret != securityFixtureSeed || detail.Account.SecurityPending {
		t.Fatal("confirmed reset did not publish the new account credential")
	}
	if calls.Load() != 1 || h.saves != 0 {
		t.Fatal("reset-only operation replayed login or saved a CPA credential")
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s = offlineService(t, opts)
	detail, err = s.AccountDetail(a.ID)
	if err != nil || detail.Credentials.TOTPSecret != securityFixtureSeed || calls.Load() != 1 {
		t.Fatal("restart lost the new seed or repeated a remote operation")
	}
}

func TestSecurityResetRetainsConfirmedSeedWhenLogoutIsUnknown(t *testing.T) {
	opts := offlineOptions(t)
	var calls atomic.Int32
	opts.Runtime = offlineRuntime{run: func(_ context.Context, in RuntimeRequest) (string, error) {
		calls.Add(1)
		v := securityFixtureJournal(in)
		v["status"], v["phase"], v["logout_status"] = "unknown", "logout_all", "unknown"
		if err := writeSecurityFixture(in, v); err != nil {
			return "", err
		}
		return "", errors.New("runtime_failed")
	}}
	s := offlineService(t, opts)
	a := importOfflineAccount(t, s)
	job, err := s.StartAccountSecurity(a.ID, a.Revision, "reset_totp", true, true)
	if err != nil {
		t.Fatal(err)
	}
	job = waitLogin(t, s, job.ID, "security_uncertain")
	detail, err := s.AccountDetail(a.ID)
	if err != nil || !job.TOTPReset || job.DevicesLoggedOut || detail.Credentials.TOTPSecret != securityFixtureSeed || !detail.Account.SecurityPending {
		t.Fatal("uncertain logout rolled back the confirmed seed or claimed success")
	}
	if _, err = s.StartAccountSecurity(a.ID, detail.Account.Revision, "logout_all", false, false); err == nil || calls.Load() != 1 {
		t.Fatal("uncertain logout was allowed to replay")
	}
}

func TestSecurityResetReauthorizesAndUploadsOnce(t *testing.T) {
	h := newFakeHost()
	opts := offlineOptions(t)
	opts.Call = h.call
	var calls atomic.Int32
	opts.Runtime = offlineRuntime{run: func(_ context.Context, in RuntimeRequest) (string, error) {
		calls.Add(1)
		if !in.Reauthorize || in.Operation != "reset_totp" {
			return "", errors.New("missing explicit reauthorization")
		}
		if err := writeSecurityFixture(in, securityFixtureJournal(in)); err != nil {
			return "", err
		}
		return offlineCallback(in.State), nil
	}}
	s := offlineService(t, opts)
	a := importOfflineAccount(t, s)
	job, err := s.StartAccountSecurity(a.ID, a.Revision, "reset_totp", true, true)
	if err != nil {
		t.Fatal(err)
	}
	job = waitLogin(t, s, job.ID, "committed")
	if !job.TOTPReset || !job.DevicesLoggedOut || !job.CredentialEnabled || calls.Load() != 1 {
		t.Fatal("security result did not survive automatic credential adoption")
	}
	detail, err := s.AccountDetail(a.ID)
	if err != nil || detail.Credentials.TOTPSecret != securityFixtureSeed || detail.Account.AuthID != job.TargetAuthID || detail.Account.SecurityPending {
		t.Fatal("automatic upload did not retain the new seed and account binding")
	}
	if _, err := s.CompleteAccountSecurity(a.ID, detail.Account.Revision, job.ID); err == nil {
		t.Fatal("local completion overwrote an already committed security job")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.saves != 1 || h.entries[job.TargetAuthID].Disabled {
		t.Fatal("security reauthorization did not save and enable exactly once")
	}
}

func TestSecurityCompleteOnlyPersistsConfirmedMaterialWithoutNetwork(t *testing.T) {
	opts := offlineOptions(t)
	var calls atomic.Int32
	request := make(chan RuntimeRequest, 1)
	opts.Runtime = offlineRuntime{run: func(_ context.Context, in RuntimeRequest) (string, error) {
		calls.Add(1)
		request <- in
		v := securityFixtureJournal(in)
		v["status"], v["phase"], v["activation_status"] = "unknown", "totp_activate", "pending"
		delete(v, "activation_confirmation")
		if err := writeSecurityFixture(in, v); err != nil {
			return "", err
		}
		return "", errors.New("runtime_failed")
	}}
	s := offlineService(t, opts)
	a := importOfflineAccount(t, s)
	job, err := s.StartAccountSecurity(a.ID, a.Revision, "reset_totp", false, false)
	if err != nil {
		t.Fatal(err)
	}
	waitLogin(t, s, job.ID, "security_uncertain")
	detail, err := s.AccountDetail(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	in := <-request
	if err := writeSecurityFixture(in, securityFixtureJournal(in)); err != nil {
		t.Fatal(err)
	}
	response := securityRequest(t, s, "security-complete-only-local", "/accounts/security/complete", map[string]any{"id": a.ID, "revision": detail.Account.Revision, "job_id": job.ID})
	if response.StatusCode != 200 {
		t.Fatalf("confirmed local completion returned %d: %s", response.StatusCode, response.Body)
	}
	detail, err = s.AccountDetail(a.ID)
	if err != nil || detail.Credentials.TOTPSecret != securityFixtureSeed || detail.Account.SecurityPending || calls.Load() != 1 {
		t.Fatal("local completion failed to publish the confirmed seed or dispatched network work")
	}
}

func TestSecurityUnknownActivationBlocksOtherEntryPointsAcrossRestart(t *testing.T) {
	opts := offlineOptions(t)
	var calls atomic.Int32
	opts.Runtime = offlineRuntime{run: func(_ context.Context, in RuntimeRequest) (string, error) {
		calls.Add(1)
		v := securityFixtureJournal(in)
		v["status"], v["phase"], v["activation_status"] = "unknown", "totp_activate", "pending"
		delete(v, "activation_confirmation")
		if err := writeSecurityFixture(in, v); err != nil {
			return "", err
		}
		return "", errors.New("runtime_failed")
	}}
	s := offlineService(t, opts)
	a := importOfflineAccount(t, s)
	job, err := s.StartAccountSecurity(a.ID, a.Revision, "reset_totp", false, false)
	if err != nil {
		t.Fatal(err)
	}
	waitLogin(t, s, job.ID, "security_uncertain")
	assertBlocked := func(s *Service) {
		t.Helper()
		detail, err := s.AccountDetail(a.ID)
		if err != nil || !detail.Account.SecurityPending || detail.SecurityRecovery == nil || detail.SecurityRecovery.NewTOTPSecret != securityFixtureSeed || detail.SecurityRecovery.ActivationConfirmed {
			t.Fatal("uncertain activation did not preserve pending recovery material")
		}
		if _, err = s.StartAccount(a.ID, detail.Account.Revision, ""); err == nil {
			t.Fatal("account login used an uncertain TOTP seed")
		}
		if _, err = s.Start(offlineStart()); err == nil {
			t.Fatal("generic login bypassed the account safety barrier")
		}
		credentials := offlineStart().Credentials
		if _, err = s.UpdateAccount(AccountUpdate{ID: a.ID, Revision: detail.Account.Revision, Credentials: &credentials}); err == nil {
			t.Fatal("account edit discarded unresolved security state")
		}
		if err = s.DeleteAccount(a.ID, detail.Account.Revision); err == nil {
			t.Fatal("account deletion discarded unresolved security state")
		}
		if _, err = s.CompleteAccountSecurity(a.ID, detail.Account.Revision, job.ID); err == nil {
			t.Fatal("local completion promoted an unconfirmed activation")
		}
	}
	assertBlocked(s)
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s = offlineService(t, opts)
	assertBlocked(s)
	if calls.Load() != 1 {
		t.Fatal("restart or a blocked action started a remote request")
	}
}

func TestSecurityAPIDeduplicatesOneIntentAndRejectsInvalidActions(t *testing.T) {
	opts := offlineOptions(t)
	var calls atomic.Int32
	opts.Runtime = offlineRuntime{run: func(_ context.Context, in RuntimeRequest) (string, error) {
		calls.Add(1)
		return "", writeSecurityFixture(in, securityFixtureJournal(in))
	}}
	s := offlineService(t, opts)
	a := importOfflineAccount(t, s)
	body := map[string]any{"id": a.ID, "revision": a.Revision, "action": "logout_all", "logout_all_devices": false, "reauthorize": false}
	first := securityRequest(t, s, "security-one-intent", "/accounts/security", body)
	second := securityRequest(t, s, "security-one-intent", "/accounts/security", body)
	id := ojson.GetBytes(first.Body, "id").String()
	if first.StatusCode != 200 || second.StatusCode != 200 || id == "" || ojson.GetBytes(second.Body, "id").String() != id {
		t.Fatal("security API did not preserve its idempotent job")
	}
	job := waitLogin(t, s, id, "succeeded")
	if !job.DevicesLoggedOut || job.TOTPReset || calls.Load() != 1 {
		t.Fatal("standalone logout reset TOTP or dispatched more than once")
	}
	for i, extra := range []map[string]any{{"action": "delete_account"}, {"action": "reset_totp", "reauthorize": "yes"}, {"action": "reset_totp", "logout_all_devices": "yes"}, {"action": "reset_totp", "security_journal_path": "/tmp/untrusted"}} {
		bad := map[string]any{"id": a.ID, "revision": a.Revision, "reauthorize": false, "logout_all_devices": false}
		for k, v := range extra {
			bad[k] = v
		}
		response := securityRequest(t, s, "security-invalid-"+string(rune('a'+i)), "/accounts/security", bad)
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("invalid security input returned %d", response.StatusCode)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("invalid requests reached the login engine")
	}
}

func TestSecurityAccountMutualExclusion(t *testing.T) {
	opts := offlineOptions(t)
	entered := make(chan struct{})
	opts.Runtime = offlineRuntime{run: func(ctx context.Context, in RuntimeRequest) (string, error) {
		close(entered)
		<-ctx.Done()
		return "", ctx.Err()
	}}
	s := offlineService(t, opts)
	a := importOfflineAccount(t, s)
	job, err := s.StartAccountSecurity(a.ID, a.Revision, "reset_totp", false, false)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("security worker was not admitted")
	}
	detail, err := s.AccountDetail(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.StartAccountSecurity(a.ID, detail.Account.Revision, "logout_all", false, false); err == nil {
		t.Fatal("concurrent account mutations were both admitted")
	}
	if _, err = s.StartAccount(a.ID, detail.Account.Revision, ""); err == nil {
		t.Fatal("ordinary login raced a security mutation")
	}
	if _, err = s.Cancel(job.ID, false); err != nil {
		t.Fatal(err)
	}
	waitLogin(t, s, job.ID, "security_uncertain")
	detail, err = s.AccountDetail(a.ID)
	if err != nil || !detail.Account.SecurityPending {
		t.Fatal("cancellation without a durable outcome released the account")
	}
}

func TestSecurityIncompleteOrMismatchedEvidenceCannotReplaceSeed(t *testing.T) {
	for _, field := range []string{"account_id", "enrollment_session_id", "job_id", "email", "new_factor_id", "activation_confirmation"} {
		t.Run(field, func(t *testing.T) {
			opts := offlineOptions(t)
			opts.Runtime = offlineRuntime{run: func(_ context.Context, in RuntimeRequest) (string, error) {
				v := securityFixtureJournal(in)
				switch field {
				case "job_id":
					v[field] = "different-operation"
				case "email":
					v[field] = "someone-else@example.test"
				case "new_factor_id":
					v[field] = v["old_factor_id"]
				default:
					delete(v, field)
				}
				return "", writeSecurityFixture(in, v)
			}}
			s := offlineService(t, opts)
			a := importOfflineAccount(t, s)
			job, err := s.StartAccountSecurity(a.ID, a.Revision, "reset_totp", false, false)
			if err != nil {
				t.Fatal(err)
			}
			job = waitLogin(t, s, job.ID, "security_uncertain")
			detail, err := s.AccountDetail(a.ID)
			if err != nil || job.TOTPReset || detail.Credentials.TOTPSecret == securityFixtureSeed || !detail.Account.SecurityPending {
				t.Fatal("incomplete or mismatched activation evidence replaced the account seed")
			}
		})
	}
}

func TestSecurityResolvedFinalizerCannotRestoreAnOlderSeed(t *testing.T) {
	const nextSeed = "MFRGGZDFMZTWQ2LK"
	opts := offlineOptions(t)
	var calls atomic.Int32
	opts.Runtime = offlineRuntime{run: func(_ context.Context, in RuntimeRequest) (string, error) {
		v := securityFixtureJournal(in)
		if calls.Add(1) == 2 {
			v["new_totp_secret"] = nextSeed
		}
		return "", writeSecurityFixture(in, v)
	}}
	s := offlineService(t, opts)
	a := importOfflineAccount(t, s)
	first, err := s.StartAccountSecurity(a.ID, a.Revision, "reset_totp", false, false)
	if err != nil {
		t.Fatal(err)
	}
	waitLogin(t, s, first.ID, "succeeded")
	detail, err := s.AccountDetail(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.StartAccountSecurity(a.ID, detail.Account.Revision, "reset_totp", false, false)
	if err != nil {
		t.Fatal(err)
	}
	waitLogin(t, s, second.ID, "succeeded")
	s.mu.Lock()
	old := s.jobs[first.ID]
	s.mu.Unlock()
	// A delayed finalizer from an earlier completed task must be harmless.
	s.finishSecurityRun(old, true)
	detail, err = s.AccountDetail(a.ID)
	if err != nil || detail.Credentials.TOTPSecret != nextSeed || detail.Account.LastJobID != second.ID {
		t.Fatal("an older task overwrote the latest confirmed TOTP seed")
	}
}

func TestSecurityLogoutRequiresAccountIdentity(t *testing.T) {
	for _, mode := range []string{"missing_identity", "bound_identity_mismatch"} {
		t.Run(mode, func(t *testing.T) {
			opts := offlineOptions(t)
			opts.Runtime = offlineRuntime{run: func(_ context.Context, in RuntimeRequest) (string, error) {
				if in.Operation == "" {
					return offlineCallback(in.State), nil
				}
				v := securityFixtureJournal(in)
				if mode == "missing_identity" {
					delete(v, "account_id")
				} else {
					v["account_id"] = "different-workspace"
				}
				return "", writeSecurityFixture(in, v)
			}}
			s := offlineService(t, opts)
			a := importOfflineAccount(t, s)
			if mode == "bound_identity_mismatch" {
				startOfflineAccount(t, s, a, "", "committed")
				detail, err := s.AccountDetail(a.ID)
				if err != nil {
					t.Fatal(err)
				}
				a = detail.Account
			}
			job, err := s.StartAccountSecurity(a.ID, a.Revision, "logout_all", false, false)
			if err != nil {
				t.Fatal(err)
			}
			job = waitLogin(t, s, job.ID, "security_uncertain")
			if job.DevicesLoggedOut {
				t.Fatal("logout succeeded without matching account identity")
			}
		})
	}
}

func startSecurityBrowserFixture(t *testing.T, s *Service) Job {
	t.Helper()
	j, err := s.Start(StartRequest{ConsumerPlugin: "model-quality", Method: "browser", Intent: "import"})
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	state := s.jobs[j.ID].state
	s.mu.Unlock()
	if _, err := s.Callback(j.ID, offlineCallback(state)); err != nil {
		t.Fatal(err)
	}
	return j
}

func TestSecurityResetRejectsExistingManualBrowserCandidate(t *testing.T) {
	h := newFakeHost()
	opts := offlineOptions(t)
	opts.Call = h.call
	var runtimeCalls atomic.Int32
	opts.Runtime = offlineRuntime{run: func(_ context.Context, in RuntimeRequest) (string, error) {
		runtimeCalls.Add(1)
		return "", writeSecurityFixture(in, securityFixtureJournal(in))
	}}
	s := offlineService(t, opts)
	a := importOfflineAccount(t, s)
	manual := startSecurityBrowserFixture(t, s)
	manual = waitLogin(t, s, manual.ID, "awaiting_commit")
	candidate, err := s.Candidate(manual.CandidateID)
	if err != nil || candidate.Email != offlineStart().Credentials.Email {
		t.Fatal("manual browser fixture did not identify the selected account")
	}
	if _, err := s.StartAccountSecurity(a.ID, a.Revision, "reset_totp", false, false); err == nil || err.Error() != "conflict" {
		t.Fatal("reset raced an already identified manual browser candidate")
	}
	if runtimeCalls.Load() != 0 || h.saves != 0 {
		t.Fatal("rejected reset dispatched remote work or saved a credential")
	}
	current, err := s.Detail(manual.ID)
	if err != nil || current.Status != "awaiting_commit" || current.CandidateID != manual.CandidateID {
		t.Fatal("rejected reset discarded the prior manual candidate")
	}
}

func TestSecurityUnknownBlocksManualBrowserCallbackCandidate(t *testing.T) {
	h := newFakeHost()
	opts := offlineOptions(t)
	opts.Call = h.call
	var runtimeCalls atomic.Int32
	opts.Runtime = offlineRuntime{run: func(_ context.Context, in RuntimeRequest) (string, error) {
		runtimeCalls.Add(1)
		v := securityFixtureJournal(in)
		v["status"], v["phase"], v["activation_status"] = "unknown", "totp_activate", "pending"
		delete(v, "activation_confirmation")
		if err := writeSecurityFixture(in, v); err != nil {
			return "", err
		}
		return "", errors.New("runtime_failed")
	}}
	s := offlineService(t, opts)
	a := importOfflineAccount(t, s)
	security, err := s.StartAccountSecurity(a.ID, a.Revision, "reset_totp", false, false)
	if err != nil {
		t.Fatal(err)
	}
	waitLogin(t, s, security.ID, "security_uncertain")
	manual := startSecurityBrowserFixture(t, s)
	manual = waitLogin(t, s, manual.ID, "conflict")
	if manual.ErrorCode != "security_pending" || manual.CandidateID != "" {
		t.Fatal("manual callback bypassed unresolved account security")
	}
	if _, err := s.Candidate(manual.ID + ":candidate"); err == nil {
		t.Fatal("blocked callback exposed a candidate")
	}
	if _, err := s.Commit(context.Background(), manual.ID, manual.ID+":candidate", manual.Revision); err == nil {
		t.Fatal("blocked callback remained commit-capable")
	}
	if runtimeCalls.Load() != 1 || h.saves != 0 {
		t.Fatal("blocked callback saved a credential or repeated security work")
	}
}

func TestSecurityPreMutationFailurePreservesLoginDiagnosis(t *testing.T) {
	opts := offlineOptions(t)
	opts.Runtime = offlineRuntime{run: func(_ context.Context, in RuntimeRequest) (string, error) {
		v := map[string]any{"version": 1, "job_id": in.JobID, "attempt": in.Attempt, "operation": in.Operation,
			"email": in.Credentials.Email, "phase": "security_check", "status": "failed", "account_id": "",
			"disable_status": "not_requested", "enroll_status": "not_requested", "activation_status": "not_requested", "logout_status": "not_requested"}
		if err := writeSecurityFixture(in, v); err != nil {
			return "", err
		}
		return "", &runtimeDiagnosticError{code: "totp_rejected", stage: "totp", reason: "upstream_rejected"}
	}}
	s := offlineService(t, opts)
	a := importOfflineAccount(t, s)
	job, err := s.StartAccountSecurity(a.ID, a.Revision, "reset_totp", false, false)
	if err != nil {
		t.Fatal(err)
	}
	job = waitLogin(t, s, job.ID, "failed")
	if job.ErrorCode != "totp_rejected" || job.ErrorStage != "totp" || job.TOTPReset {
		t.Fatal("pre-mutation authentication failure lost its diagnosis")
	}
	detail, err := s.AccountDetail(a.ID)
	if err != nil || detail.Account.SecurityPending {
		t.Fatal("proven pre-mutation failure quarantined unchanged credentials")
	}
}
