package login

import (
	"bytes"
	"encoding/gob"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/MoeForge/moego/ojson"
	bolt "go.etcd.io/bbolt"
)

// SecurityRecovery is returned only by the authenticated account detail route.
// An unconfirmed seed is evidence, never a replacement login credential.
type SecurityRecovery struct {
	JobID               string `json:"job_id"`
	NewTOTPSecret       string `json:"new_totp_secret,omitempty"`
	ActivationConfirmed bool   `json:"activation_confirmed"`
	CanComplete         bool   `json:"can_complete"`
	Phase               string `json:"phase"`
}

type securityJournal struct {
	AccountID, Status, Phase, Operation                string
	NewSecret, OldFactor, NewFactor, EnrollmentSession string
	Disable, Enroll, Activation, Confirmation, Logout  string
}

func validSecurityAction(action string) bool { return action == "reset_totp" || action == "logout_all" }
func securityBlocks(record storedJob) bool {
	return record.View.Action != "" && !record.SecurityResolved
}

func (s *Service) securityAvailable() bool {
	if runtime, ok := s.opts.Runtime.(*Runtime); ok {
		metadata, err := runtime.Metadata()
		return err == nil && metadata["runtime_version"] == "2.1.0"
	}
	// Alternate runtimes are injected by offline tests only.
	return s.opts.Runtime != nil
}

func (s *Service) securityJournalPath(id string, create bool) (string, error) {
	if id == "" || filepath.Base(id) != id || strings.ContainsAny(id, "/\\:\x00") {
		return "", errors.New("security_journal_invalid")
	}
	dir := filepath.Join(s.opts.Dir, "security")
	if create {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return "", errors.New("storage_unavailable")
		}
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return "", errors.New("security_journal_invalid")
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		return "", errors.New("security_journal_invalid")
	}
	return filepath.Join(dir, id+".json"), nil
}

func (s *Service) readSecurityJournal(record storedJob, email string) (securityJournal, error) {
	var out securityJournal
	path, err := s.securityJournalPath(record.View.ID, false)
	if err != nil {
		return out, err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > 32<<10 {
		return out, errors.New("security_journal_invalid")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return out, errors.New("security_journal_invalid")
	}
	defer clear(raw)
	r, err := Object(raw, "version", "job_id", "attempt", "operation", "email", "phase", "status", "account_id", "old_factor_id", "new_factor_id", "new_totp_secret", "enrollment_session_id", "disable_status", "enroll_status", "activation_status", "activation_confirmation", "logout_status")
	if err != nil || r.Get("version").Raw != "1" || r.Get("attempt").Raw != "1" || r.Get("job_id").String() != record.View.ID || r.Get("operation").String() != record.View.Action || r.Get("email").String() != email {
		return out, errors.New("security_journal_invalid")
	}
	valid := true
	r.ForEach(func(k, v ojson.Result) bool {
		if k.String() != "version" && k.String() != "attempt" && (v.Type != ojson.String || len(v.String()) > 4096) {
			valid = false
		}
		return valid
	})
	if !valid {
		return out, errors.New("security_journal_invalid")
	}
	out = securityJournal{AccountID: r.Get("account_id").String(), Status: r.Get("status").String(), Phase: r.Get("phase").String(), Operation: r.Get("operation").String(), NewSecret: r.Get("new_totp_secret").String(), OldFactor: r.Get("old_factor_id").String(), NewFactor: r.Get("new_factor_id").String(), EnrollmentSession: r.Get("enrollment_session_id").String(), Disable: r.Get("disable_status").String(), Enroll: r.Get("enroll_status").String(), Activation: r.Get("activation_status").String(), Confirmation: r.Get("activation_confirmation").String(), Logout: r.Get("logout_status").String()}
	if !runtimeDiagnostic(out.Status, []string{"running", "succeeded", "failed", "unknown"}) || !runtimeDiagnostic(out.Phase, []string{"security_login", "security_check", "totp_disable", "totp_enroll", "totp_activate", "totp_saved", "logout_all", "reauthorization", "security_complete"}) {
		return securityJournal{}, errors.New("security_journal_invalid")
	}
	for _, value := range []string{out.Disable, out.Enroll, out.Activation, out.Logout} {
		if !runtimeDiagnostic(value, []string{"not_requested", "pending", "succeeded", "rejected", "unknown"}) {
			return securityJournal{}, errors.New("security_journal_invalid")
		}
	}
	for _, state := range []string{out.Disable, out.Enroll, out.Activation, out.Logout} {
		if state != "not_requested" && !validSecurityBinding(out.AccountID) {
			return securityJournal{}, errors.New("security_journal_invalid")
		}
	}
	if out.NewSecret != "" && !validTOTPSecret(out.NewSecret) {
		return securityJournal{}, errors.New("security_journal_invalid")
	}
	if out.Operation == "logout_all" && (out.Disable != "not_requested" || out.Enroll != "not_requested" || out.Activation != "not_requested" || out.NewSecret != "") {
		return securityJournal{}, errors.New("security_journal_invalid")
	}
	if record.SecurityAccountID != "" && out.AccountID != "" && record.SecurityAccountID != out.AccountID {
		return securityJournal{}, errors.New("security_journal_invalid")
	}
	if out.Activation == "succeeded" && !out.activated() {
		return securityJournal{}, errors.New("security_journal_invalid")
	}
	if !record.View.LogoutAllDevices && record.View.Action != "logout_all" && out.Logout != "not_requested" {
		return securityJournal{}, errors.New("security_journal_invalid")
	}
	return out, nil
}

func validSecurityBinding(value string) bool {
	return value != "" && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\r\n\x00")
}
func (j securityJournal) activated() bool {
	return validSecurityBinding(j.AccountID) && validSecurityBinding(j.EnrollmentSession) && j.Operation == "reset_totp" && j.Disable == "succeeded" && j.Enroll == "succeeded" && j.Activation == "succeeded" && validSecurityBinding(j.OldFactor) && validSecurityBinding(j.NewFactor) && j.NewFactor != j.OldFactor && validTOTPSecret(j.NewSecret) && (j.Confirmation == "business_receipt" || j.Confirmation == "state_read")
}
func (j securityJournal) uncertain() bool {
	if j.Status == "unknown" {
		return true
	}
	for _, value := range []string{j.Disable, j.Enroll, j.Activation, j.Logout} {
		if value == "pending" || value == "unknown" {
			return true
		}
	}
	return (j.Disable == "succeeded" || j.Enroll == "succeeded") && !j.activated()
}
func (j securityJournal) complete(job Job) bool {
	return (!job.LogoutAllDevices && job.Action != "logout_all" || j.Logout == "succeeded") && (job.Action != "reset_totp" || j.activated())
}

func (s *Service) StartAccountSecurity(id string, revision uint64, action string, logoutAll, reauthorize bool) (Job, error) {
	if !validSecurityAction(action) || action == "logout_all" && logoutAll {
		return Job{}, errors.New("invalid_request")
	}
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	if err := s.accountReady(); err != nil {
		return Job{}, err
	}
	if err := s.syncAccountJobsLocked(); err != nil {
		return Job{}, err
	}
	a, ok := s.accounts[id]
	if !ok {
		return Job{}, errors.New("not_found")
	}
	if a.View.Revision != revision {
		return Job{}, errors.New("conflict")
	}
	if a.View.SecurityPending {
		return Job{}, errors.New("security_pending")
	}
	return s.startAccountOperationLocked(a, "", false, "", action, logoutAll, reauthorize)
}

// reconcileSecurity runs only after the helper has exited. It never resumes a
// remote operation. Confirmed seeds and existing vault copies are saved together.
func (s *Service) reconcileSecurity(j *session) error {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reconcileSecurityLocked(j)
}
func (s *Service) reconcileSecurityLocked(j *session) error {
	if j.record.View.Action == "" || j.record.SecurityResolved {
		return nil
	}
	a, ok := s.accounts[j.record.View.AccountID]
	if !ok {
		return errors.New("security_journal_invalid")
	}
	var evidence securityJournal
	var readErr error
	if j.record.SecurityStarted {
		evidence, readErr = s.readSecurityJournal(j.record, a.Credentials.Email)
	}
	uncertain := readErr != nil || j.record.SecurityStarted && evidence.uncertain()
	activated := readErr == nil && evidence.activated()
	if activated {
		if a.Credentials.TOTPSecret != evidence.NewSecret {
			credentials := a.Credentials
			credentials.TOTPSecret = evidence.NewSecret
			s.setAccountCredentials(&a, credentials)
		}
		j.record.View.TOTPReset = true
	}
	j.record.View.DevicesLoggedOut = readErr == nil && evidence.Logout == "succeeded"
	j.record.SecurityResolved = !uncertain
	a.View.SecurityPending = uncertain
	j.record.View.SecurityStatus = "complete"
	if uncertain {
		j.record.View.SecurityStatus = "uncertain"
		a.View.AttentionRequired = true
		a.View.HealthReason = "security_uncertain"
	} else if activated && !evidence.complete(j.record.View) {
		j.record.View.SecurityStatus = "totp_saved"
	} else if !j.record.SecurityStarted || !evidence.complete(j.record.View) {
		j.record.View.SecurityStatus = "failed"
	}
	a.View.LastJobAction, a.View.LastJobSecurityStatus = j.record.View.Action, j.record.View.SecurityStatus
	a.View.LastJobTOTPReset, a.View.LastJobDevicesLoggedOut = j.record.View.TOTPReset, j.record.View.DevicesLoggedOut
	a.View.Revision++
	if err := s.journal.capacity(true); err != nil {
		return err
	}
	s.vault.mu.Lock()
	err := s.journal.db.Update(func(tx *bolt.Tx) error {
		if err := putGob(tx, "accounts", a.View.ID, a); err != nil {
			return err
		}
		if !activated {
			return nil
		}
		bucket := tx.Bucket([]byte("vault"))
		return bucket.ForEach(func(key, raw []byte) error {
			var entry vaultEntry
			if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&entry); err != nil {
				return err
			}
			if entry.Credentials.Email == a.Credentials.Email && entry.Credentials.TOTPSecret != evidence.NewSecret {
				entry.Credentials.TOTPSecret = evidence.NewSecret
				entry.Revision++
				return putGob(tx, "vault", string(key), entry)
			}
			return nil
		})
	})
	s.vault.mu.Unlock()
	if err != nil {
		s.fault = true
		s.cancel()
		return errors.New("storage_unavailable")
	}
	s.accounts[a.View.ID] = a
	if uncertain {
		if err := s.transitionLocked(j, "security_uncertain", "", "security_uncertain"); err != nil {
			return err
		}
		return errors.New("security_uncertain")
	}
	if err := s.persistLocked(j); err != nil {
		return err
	}
	if j.record.SecurityStarted && !evidence.complete(j.record.View) {
		return errors.New("security_failed")
	}
	return nil
}

func (s *Service) finishSecurityRun(j *session, reconcile bool) {
	if reconcile {
		_ = s.reconcileSecurity(j)
	}
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	_ = s.syncAccountJobsLocked()
}

func (s *Service) recoverSecurity() error {
	// Startup has no active helper; any interrupted mutation remains isolated.
	for _, j := range s.jobs {
		if j.record.View.Action == "" || j.record.SecurityResolved {
			continue
		}
		if err := s.reconcileSecurity(j); err != nil && err.Error() != "security_uncertain" && err.Error() != "security_failed" {
			return err
		}
	}
	return nil
}

func (s *Service) securityRecoveryLocked(a storedAccount) *SecurityRecovery {
	if !a.View.SecurityPending {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	j := s.jobs[a.View.LastJobID]
	if j == nil || j.record.View.Action == "" {
		return nil
	}
	evidence, err := s.readSecurityJournal(j.record, a.Credentials.Email)
	if err != nil {
		return &SecurityRecovery{JobID: j.record.View.ID}
	}
	return &SecurityRecovery{JobID: j.record.View.ID, NewTOTPSecret: evidence.NewSecret, ActivationConfirmed: evidence.activated(), CanComplete: terminal(j.record.View.Status) && evidence.activated() && !evidence.uncertain(), Phase: evidence.Phase}
}

// CompleteAccountSecurity only finishes a confirmed local save. It cannot
// activate a factor, sign out devices, or authorize/upload another credential.
func (s *Service) CompleteAccountSecurity(id string, revision uint64, jobID string) (Job, error) {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	if err := s.accountReady(); err != nil {
		return Job{}, err
	}
	a, ok := s.accounts[id]
	if !ok {
		return Job{}, errors.New("not_found")
	}
	if a.View.Revision != revision || a.View.LastJobID != jobID {
		return Job{}, errors.New("conflict")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	j := s.jobs[jobID]
	if j == nil || j.record.View.AccountID != id || j.record.View.Action != "reset_totp" || j.record.SecurityResolved || !terminal(j.record.View.Status) {
		return Job{}, errors.New("conflict")
	}
	evidence, err := s.readSecurityJournal(j.record, a.Credentials.Email)
	if err != nil || !evidence.activated() || evidence.uncertain() {
		return Job{}, errors.New("security_uncertain")
	}
	if err = s.reconcileSecurityLocked(j); err != nil && err.Error() != "security_failed" {
		return Job{}, err
	}
	if err = s.transitionLocked(j, "succeeded", "totp_saved", ""); err != nil {
		return Job{}, err
	}
	return s.viewLocked(j), nil
}
