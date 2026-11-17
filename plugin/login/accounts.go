package login

import (
	"bytes"
	"encoding/gob"
	"errors"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	auth "github.com/cliproxyapi-plugins/model-quality/login/account"
	"github.com/google/uuid"
	bolt "go.etcd.io/bbolt"
)

// Account contains management metadata. Its full email is populated only for a
// response; stored records retain the complete plaintext account material.
type Account struct {
	SecurityPending          bool   `json:"security_pending"`
	LastJobAction            string `json:"last_job_action,omitempty"`
	LastJobSecurityStatus    string `json:"last_job_security_status,omitempty"`
	LastJobTOTPReset         bool   `json:"last_job_totp_reset"`
	LastJobDevicesLoggedOut  bool   `json:"last_job_devices_logged_out"`
	ID                       string `json:"id"`
	Revision                 uint64 `json:"revision"`
	Label                    string `json:"label"`
	MaskedEmail              string `json:"masked_email"`
	Email                    string `json:"email,omitempty"`
	ProxyURL                 string `json:"proxy_url"`
	EffectiveProxyURL        string `json:"effective_proxy_url,omitempty"`
	ProxySource              string `json:"proxy_source,omitempty"`
	ProxyError               string `json:"proxy_error,omitempty"`
	CredentialsError         string `json:"credentials_error,omitempty"`
	AuthID                   string `json:"auth_id"`
	IdentityVersion          string `json:"identity_version"`
	HealthStatus             string `json:"health_status"`
	HealthReason             string `json:"health_reason"`
	LastCheckedAt            string `json:"last_checked_at"`
	NextCheckAt              string `json:"next_check_at"`
	LastJobID                string `json:"last_job_id"`
	LastJobStatus            string `json:"last_job_status"`
	LastJobAutoCommit        bool   `json:"last_job_auto_commit"`
	LastJobCredentialEnabled bool   `json:"last_job_credential_enabled"`
	LastJobErrorCode         string `json:"last_job_error_code,omitempty"`
	LastJobErrorStage        string `json:"last_job_error_stage,omitempty"`
	LastJobErrorReason       string `json:"last_job_error_reason,omitempty"`
	LastLoginAt              string `json:"last_login_at"`
	AutoRelogin              bool   `json:"auto_relogin"`
	AttentionRequired        bool   `json:"attention_required"`
	ConsecutiveFailures      int    `json:"consecutive_failures"`
}
type AccountDetail struct {
	SecurityRecovery *SecurityRecovery `json:"security_recovery,omitempty"`
	Account          Account           `json:"account"`
	Credentials      Credentials       `json:"credentials"`
}
type AccountUpdate struct {
	ID          string
	Revision    uint64
	Label       string
	AutoRelogin bool
	Credentials *Credentials
	ProxyURL    *string
}
type AccountSettings struct {
	Revision           uint64 `json:"revision"`
	InspectionEnabled  bool   `json:"inspection_enabled"`
	AutoReloginEnabled bool   `json:"auto_relogin_enabled"`
	IntervalMinutes    int    `json:"interval_minutes"`
	LoginProxyURL      string `json:"login_proxy_url"`
}
type storedAccount struct {
	View                 Account
	EmailHash, Principal string
	SecretRevision       uint64
	Credentials          Credentials
	Evidence             string
	ConsumedEvidence     string
	NextAttempt          time.Time
}

func (s *Service) initAccounts() error {
	s.accounts = map[string]storedAccount{}
	s.accountPermits = map[string]*atomic.Bool{}
	s.accountSettings = AccountSettings{Revision: 1, IntervalMinutes: 15}
	err := s.journal.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("accounts"))
		if err != nil {
			return err
		}
		if raw := tx.Bucket([]byte("meta")).Get([]byte("account-settings")); raw != nil {
			if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&s.accountSettings); err != nil {
				return err
			}
		}
		return b.ForEach(func(k, raw []byte) error {
			var a storedAccount
			if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&a); err != nil {
				return err
			}
			if a.View.ID != string(k) || a.SecretRevision == 0 || !storedCredentialsComplete(a.Credentials) || s.journal.fingerprint("account-email", a.Credentials.Email) != a.EmailHash {
				return errors.New("vault_corrupt")
			}
			s.accounts[a.View.ID] = a
			permit := &atomic.Bool{}
			permit.Store(a.View.AutoRelogin)
			s.accountPermits[a.View.ID] = permit
			return nil
		})
	})
	if err != nil {
		return errors.New("storage_unavailable")
	}
	if s.accountSettings.IntervalMinutes < 5 || s.accountSettings.IntervalMinutes > 1440 {
		return errors.New("storage_unavailable")
	}
	s.accountAutomation.Store(s.accountSettings.InspectionEnabled && s.accountSettings.AutoReloginEnabled)
	s.accountGeneration.Store(s.accountSettings.Revision)
	s.loginProxy.Store(s.accountSettings.LoginProxyURL)
	return nil
}
func (s *Service) accountReady() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.fault {
		return errors.New("storage_unavailable")
	}
	return nil
}
func (s *Service) accountStorageFailure() error {
	s.accountAutomation.Store(false)
	s.mu.Lock()
	s.fault = true
	s.cancel()
	s.mu.Unlock()
	return errors.New("storage_unavailable")
}
func (s *Service) saveAccountLocked(a storedAccount) error {
	a.View.Email, a.View.CredentialsError = "", ""
	a.View.EffectiveProxyURL, a.View.ProxySource, a.View.ProxyError = "", "", ""
	a.View.LastJobErrorCode, a.View.LastJobErrorStage, a.View.LastJobErrorReason = "", "", ""
	if err := s.journal.capacity(true); err != nil {
		return s.accountStorageFailure()
	}
	if err := s.journal.db.Update(func(tx *bolt.Tx) error { return putGob(tx, "accounts", a.View.ID, a) }); err != nil {
		return s.accountStorageFailure()
	}
	s.accounts[a.View.ID] = a
	return nil
}
func (s *Service) setAccountCredentials(a *storedAccount, c Credentials) {
	a.SecretRevision++
	a.Credentials = c
}
func (s *Service) accountCredentials(a storedAccount) (Credentials, error) {
	if a.SecretRevision == 0 || !storedCredentialsComplete(a.Credentials) || s.journal.fingerprint("account-email", a.Credentials.Email) != a.EmailHash {
		return Credentials{}, errors.New("vault_corrupt")
	}
	return a.Credentials, nil
}
func validateAccountCredentials(c Credentials) (Credentials, error) {
	raw, err := Encode(c)
	if err != nil {
		return Credentials{}, err
	}
	defer clear(raw)
	r, err := Object(raw, "email", "password", "totp_secret")
	if err != nil {
		return Credentials{}, err
	}
	return DecodeCredentials(r)
}
func (s *Service) Accounts() ([]Account, error) {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	if err := s.accountReady(); err != nil {
		return nil, err
	}
	if err := s.syncAccountJobsLocked(); err != nil {
		return nil, err
	}
	return s.accountViewsLocked(""), nil
}
func (s *Service) accountViewsLocked(id string) []Account {
	out := []Account{}
	for key, a := range s.accounts {
		if id == "" || id == key {
			out = append(out, s.accountView(a))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Service) accountView(a storedAccount) Account {
	v := a.View
	var proxyErr error
	v.EffectiveProxyURL, v.ProxySource, proxyErr = s.effectiveLoginProxy(v.ProxyURL, v.AuthID)
	v.ProxyError = SafeCode(proxyErr)
	if v.LastJobID != "" {
		if job, err := s.Detail(v.LastJobID); err == nil {
			v.LastJobErrorCode, v.LastJobErrorStage, v.LastJobErrorReason = job.ErrorCode, job.ErrorStage, job.ErrorReason
			v.LastJobAutoCommit, v.LastJobCredentialEnabled = job.AutoCommit, job.CredentialEnabled
			v.LastJobAction, v.LastJobSecurityStatus = job.Action, job.SecurityStatus
			v.LastJobTOTPReset, v.LastJobDevicesLoggedOut = job.TOTPReset, job.DevicesLoggedOut
		}
	}
	c, err := s.accountCredentials(a)
	if err != nil {
		v.Email = ""
		v.CredentialsError = SafeCode(err)
		return v
	}
	v.Email = c.Email
	v.CredentialsError = ""
	return v
}

// Read from the journal rather than taking accountMu while a job view
// holds mu. Account mutations use the opposite lock order.
func (s *Service) storedAccountEmail(id string) string {
	var a storedAccount
	err := s.journal.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket([]byte("accounts")).Get([]byte(id))
		if raw == nil {
			return errors.New("not_found")
		}
		return gob.NewDecoder(bytes.NewReader(raw)).Decode(&a)
	})
	if err != nil || a.View.ID != id {
		return ""
	}
	c, err := s.accountCredentials(a)
	if err != nil {
		return ""
	}
	return c.Email
}

// AccountDetail is called only by the authenticated, non-cached management GET.
// It does not create a login, mutate the account, or journal the returned material.
func (s *Service) AccountDetail(id string) (AccountDetail, error) {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	if err := s.accountReady(); err != nil {
		return AccountDetail{}, err
	}
	if id == "" || len(id) > 512 {
		return AccountDetail{}, errors.New("invalid_request")
	}
	if err := s.syncAccountJobsLocked(); err != nil {
		return AccountDetail{}, err
	}
	a, ok := s.accounts[id]
	if !ok {
		return AccountDetail{}, errors.New("not_found")
	}
	c, err := s.accountCredentials(a)
	if err != nil {
		return AccountDetail{}, err
	}
	v := s.accountView(a)
	return AccountDetail{Account: v, Credentials: c, SecurityRecovery: s.securityRecoveryLocked(a)}, nil
}
func (s *Service) ImportAccounts(entries []Credentials) ([]Account, error) {
	return s.ImportAccountsWithProxy(entries, "")
}

func (s *Service) ImportAccountsWithProxy(entries []Credentials, proxyURL string) ([]Account, error) {
	proxyURL, err := NormalizeLoginProxy(proxyURL)
	if err != nil {
		return nil, err
	}
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	if err := s.accountReady(); err != nil {
		return nil, err
	}
	if len(entries) == 0 || len(entries) > 100 || len(s.accounts)+len(entries) > 1000 {
		return nil, errors.New("invalid_request")
	}
	if err := s.journal.admit(); err != nil {
		return nil, err
	}
	if err := s.vault.ready(); err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, a := range s.accounts {
		seen[a.EmailHash] = true
	}
	pending := []storedAccount{}
	for _, entry := range entries {
		c, err := validateAccountCredentials(entry)
		if err != nil {
			return nil, err
		}
		fingerprint := s.journal.fingerprint("account-email", c.Email)
		if seen[fingerprint] {
			return nil, errors.New("duplicate_account")
		}
		seen[fingerprint] = true
		a := storedAccount{View: Account{ID: uuid.NewString(), Revision: 1, MaskedEmail: Mask(c.Email), ProxyURL: proxyURL, HealthStatus: "not_linked", HealthReason: "login_required"}, EmailHash: fingerprint}
		if s.accountSettings.InspectionEnabled {
			a.View.NextCheckAt = s.opts.Now().Add(time.Duration(s.accountSettings.IntervalMinutes) * time.Minute).Format(time.RFC3339)
		}
		s.setAccountCredentials(&a, c)
		pending = append(pending, a)
	}
	if err := s.journal.db.Update(func(tx *bolt.Tx) error {
		for _, a := range pending {
			if err := putGob(tx, "accounts", a.View.ID, a); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, s.accountStorageFailure()
	}
	out := []Account{}
	for _, a := range pending {
		s.accounts[a.View.ID] = a
		s.accountPermits[a.View.ID] = &atomic.Bool{}
		out = append(out, s.accountView(a))
	}
	return out, nil
}
func (s *Service) accountBusyLocked(a storedAccount) bool {
	if a.View.SecurityPending {
		return true
	}
	if a.View.LastJobID == "" {
		return false
	}
	job, err := s.Detail(a.View.LastJobID)
	return err == nil && (!terminal(job.Status) || job.Status == "commit_unknown" || job.Status == "saved_not_adopted")
}
func (s *Service) UpdateAccount(r AccountUpdate) (Account, error) {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	if err := s.accountReady(); err != nil {
		return Account{}, err
	}
	a, ok := s.accounts[r.ID]
	if !ok {
		return Account{}, errors.New("not_found")
	}
	if a.View.Revision != r.Revision {
		return Account{}, errors.New("conflict")
	}
	if len(r.Label) > 100 || strings.ContainsAny(r.Label, "\r\n\x00") {
		return Account{}, errors.New("invalid_request")
	}
	if a.View.SecurityPending {
		return Account{}, errors.New("security_pending")
	}
	if s.accountBusyLocked(a) && (r.AutoRelogin || r.Credentials != nil || r.ProxyURL != nil) {
		return Account{}, errors.New("conflict")
	}
	if r.ProxyURL != nil {
		proxyURL, err := NormalizeLoginProxy(*r.ProxyURL)
		if err != nil {
			return Account{}, err
		}
		a.View.ProxyURL = proxyURL
	}
	if r.Credentials != nil {
		c, err := validateAccountCredentials(*r.Credentials)
		if err != nil {
			return Account{}, err
		}
		if s.journal.fingerprint("account-email", c.Email) != a.EmailHash {
			return Account{}, errors.New("identity_conflict")
		}
		s.setAccountCredentials(&a, c)
		a.View.AttentionRequired = false
		a.View.ConsecutiveFailures = 0
		a.ConsumedEvidence = ""
		a.NextAttempt = time.Time{}
	}
	if r.AutoRelogin && a.View.AuthID == "" {
		return Account{}, errors.New("account_not_linked")
	}
	if !a.View.AutoRelogin && r.AutoRelogin {
		a.View.AttentionRequired = false
		a.View.ConsecutiveFailures = 0
		a.ConsumedEvidence = ""
		a.NextAttempt = time.Time{}
	}
	a.View.Label = strings.TrimSpace(r.Label)
	a.View.AutoRelogin = r.AutoRelogin
	a.View.Revision++
	if err := s.saveAccountLocked(a); err != nil {
		return Account{}, err
	}
	s.accountPermits[a.View.ID].Store(a.View.AutoRelogin)
	return s.accountView(a), nil
}
func (s *Service) DeleteAccount(id string, revision uint64) error {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	if err := s.accountReady(); err != nil {
		return err
	}
	a, ok := s.accounts[id]
	if !ok {
		return errors.New("not_found")
	}
	if a.View.Revision != revision || s.accountBusyLocked(a) {
		return errors.New("conflict")
	}
	if err := s.journal.db.Update(func(tx *bolt.Tx) error { return tx.Bucket([]byte("accounts")).Delete([]byte(id)) }); err != nil {
		return s.accountStorageFailure()
	}
	s.accountPermits[id].Store(false)
	delete(s.accountPermits, id)
	delete(s.accounts, id)
	return nil
}
func (s *Service) AccountSettings() (AccountSettings, error) {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	if err := s.accountReady(); err != nil {
		return AccountSettings{}, err
	}
	return s.accountSettings, nil
}
func (s *Service) UpdateAccountSettings(next AccountSettings) (AccountSettings, error) {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	if err := s.accountReady(); err != nil {
		return AccountSettings{}, err
	}
	if next.Revision != s.accountSettings.Revision {
		return AccountSettings{}, errors.New("conflict")
	}
	if next.IntervalMinutes < 5 || next.IntervalMinutes > 1440 {
		return AccountSettings{}, errors.New("invalid_request")
	}
	if next.AutoReloginEnabled && !next.InspectionEnabled {
		return AccountSettings{}, errors.New("invalid_request")
	}
	proxyURL, err := NormalizeLoginProxy(next.LoginProxyURL)
	if err != nil {
		return AccountSettings{}, err
	}
	next.LoginProxyURL = proxyURL
	next.Revision++
	now := s.opts.Now()
	updated := map[string]storedAccount{}
	for id, a := range s.accounts {
		a.View.NextCheckAt = ""
		if next.InspectionEnabled {
			a.View.NextCheckAt = now.Add(time.Duration(next.IntervalMinutes) * time.Minute).Format(time.RFC3339)
		}
		a.View.Revision++
		updated[id] = a
	}
	if err := s.journal.db.Update(func(tx *bolt.Tx) error {
		if err := putGob(tx, "meta", "account-settings", next); err != nil {
			return err
		}
		for id, a := range updated {
			if err := putGob(tx, "accounts", id, a); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return AccountSettings{}, s.accountStorageFailure()
	}
	s.accountSettings = next
	s.accounts = updated
	s.accountAutomation.Store(next.InspectionEnabled && next.AutoReloginEnabled)
	s.accountGeneration.Store(next.Revision)
	s.loginProxy.Store(next.LoginProxyURL)
	return next, nil
}
func (s *Service) StartAccount(id string, revision uint64, targetAuthID string) (Job, error) {
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
	if a.View.AuthID != "" && targetAuthID != "" && targetAuthID != a.View.AuthID {
		return Job{}, errors.New("identity_conflict")
	}
	return s.startAccountLocked(a, targetAuthID, false, "")
}
func (s *Service) startAccountLocked(a storedAccount, target string, automatic bool, evidence string) (Job, error) {
	return s.startAccountOperationLocked(a, target, automatic, evidence, "", false, false)
}
func (s *Service) startAccountOperationLocked(a storedAccount, target string, automatic bool, evidence, action string, logoutAll, reauthorize bool) (Job, error) {
	if s.accountBusyLocked(a) {
		return Job{}, errors.New("conflict")
	}
	c, err := s.accountCredentials(a)
	if err != nil {
		return Job{}, err
	}
	if a.View.AuthID != "" {
		target = a.View.AuthID
	}
	r := StartRequest{securityAction: action, logoutAllDevices: logoutAll, reauthorize: reauthorize, ConsumerPlugin: "model-quality", Method: "password_totp", Intent: "import", Credentials: c, ProxyURL: a.View.ProxyURL, accountID: a.View.ID, automatic: automatic, autoEvidence: evidence, autoCommit: true, enableCredential: true}
	if action != "" {
		r.autoCommit, r.enableCredential = reauthorize, reauthorize
	}
	if target != "" {
		snapshot, err := s.opts.Manager.LoginSnapshot(target)
		if err != nil {
			return Job{}, err
		}
		if a.Principal != "" && (snapshot.Principal != a.Principal || snapshot.IdentityVersion != a.View.IdentityVersion) {
			return Job{}, errors.New("identity_changed")
		}
		current, ok := s.opts.Manager.GetByID(target)
		if !ok {
			return Job{}, errors.New("not_found")
		}
		principal, err := auth.LoginPrincipalFromAuth(current)
		if err != nil || principal.Email != c.Email {
			return Job{}, errors.New("identity_conflict")
		}
		r.Intent = "reauthorize"
		r.TargetAuthID = target
		r.ExpectedIdentityVersion = snapshot.IdentityVersion
	}
	if automatic {
		permit := s.accountPermits[a.View.ID]
		generation := s.accountGeneration.Load()
		r.autoGuard = func() error {
			if !s.accountAutomation.Load() || !permit.Load() || s.accountGeneration.Load() != generation {
				return errors.New("automation_disabled")
			}
			return nil
		}
		if err := r.autoGuard(); err != nil {
			return Job{}, err
		}
		if err := s.checkAccountRepair(a, evidence); err != nil {
			return Job{}, err
		}
	}
	if automatic {
		a.ConsumedEvidence = evidence
		a.View.Revision++
		if err := s.saveAccountLocked(a); err != nil {
			return Job{}, err
		}
	}
	r.accountSequence = a.View.Revision
	job, err := s.Start(r)
	if err != nil {
		return Job{}, err
	}
	a.View.LastJobID = job.ID
	a.View.LastJobStatus = job.Status
	a.View.SecurityPending = action != ""
	a.View.LastJobAction = action
	a.View.Revision++
	if !automatic {
		a.View.AttentionRequired = false
		a.View.ConsecutiveFailures = 0
		a.ConsumedEvidence = ""
		a.NextAttempt = time.Time{}
	}
	if automatic {
		a.ConsumedEvidence = evidence
	}
	if err = s.saveAccountLocked(a); err != nil {
		return job, err
	}
	return job, nil
}

func (s *Service) syncAccountJobsLocked() error {
	s.mu.Lock()
	jobs := make(map[string]Job, len(s.jobs))
	evidence := map[string]string{}
	adopted := map[string]storedJob{}
	for id, j := range s.jobs {
		jobs[id] = j.record.View
		evidence[id] = j.record.AccountEvidence
		adopted[id] = j.record
	}
	s.mu.Unlock()
	for id, a := range s.accounts {
		var job Job
		if current, ok := jobs[a.View.LastJobID]; ok {
			job = current
		}
		for _, candidate := range jobs {
			newer := adopted[candidate.ID].AccountSequence > adopted[job.ID].AccountSequence ||
				adopted[candidate.ID].AccountSequence == adopted[job.ID].AccountSequence && adopted[candidate.ID].Created.After(adopted[job.ID].Created)
			if candidate.AccountID == id && (job.ID == "" || newer) {
				job = candidate
			}
		}
		if job.ID == "" || job.ID == a.View.LastJobID && job.Status == a.View.LastJobStatus {
			continue
		}
		previousJob := a.View.LastJobID
		a.View.LastJobID = job.ID
		a.View.LastJobStatus = job.Status
		a.View.LastJobAutoCommit = job.AutoCommit
		a.View.LastJobCredentialEnabled = job.CredentialEnabled
		a.View.LastJobAction, a.View.LastJobSecurityStatus = job.Action, job.SecurityStatus
		a.View.LastJobTOTPReset, a.View.LastJobDevicesLoggedOut = job.TOTPReset, job.DevicesLoggedOut
		if job.Automatic {
			a.Evidence = evidence[job.ID]
			a.ConsumedEvidence = evidence[job.ID]
		}
		if job.Status == "committed" {
			snapshot, err := s.opts.Manager.LoginSnapshot(job.TargetAuthID)
			if err != nil || snapshot.IdentityVersion != adopted[job.ID].AdoptedIdentity || snapshot.Principal != adopted[job.ID].AdoptedPrincipal {
				a.View.AttentionRequired = true
				a.View.HealthReason = "identity_changed"
			} else {
				current, ok := s.opts.Manager.GetByID(job.TargetAuthID)
				principal, principalErr := auth.LoginPrincipalFromAuth(current)
				if !ok || principalErr != nil || s.journal.fingerprint("account-email", principal.Email) != a.EmailHash {
					a.View.AttentionRequired = true
					a.View.HealthReason = "identity_conflict"
				} else {
					a.View.AuthID = job.TargetAuthID
					a.View.IdentityVersion = snapshot.IdentityVersion
					a.Principal = snapshot.Principal
					a.View.LastLoginAt = s.opts.Now().Format(time.RFC3339)
					a.View.HealthStatus = "unknown"
					a.View.HealthReason = "awaiting_inspection"
					a.View.AttentionRequired = false
					a.View.ConsecutiveFailures = 0
					a.NextAttempt = time.Time{}
				}
			}
		} else if job.Status == "succeeded" {
			a.View.AttentionRequired = false
			a.View.HealthStatus, a.View.HealthReason = "unknown", "awaiting_inspection"
		} else if terminal(job.Status) {
			if job.Automatic {
				a.View.ConsecutiveFailures++
				retry := job.Status == "failed" && (job.ErrorCode == "runtime_failed" || job.ErrorCode == "runtime_timeout" || job.ErrorCode == "exchange_failed" || job.ErrorCode == "rate_limited") && a.View.ConsecutiveFailures < 3
				if retry {
					a.ConsumedEvidence = ""
					a.NextAttempt = s.opts.Now().Add(time.Duration(1<<uint(a.View.ConsecutiveFailures-1)) * time.Hour)
				} else {
					a.View.AttentionRequired = true
				}
			} else if job.Status != "cancelled" && job.Status != "discarded" {
				a.View.AttentionRequired = true
			}
			if job.ErrorCode != "" {
				a.View.HealthReason = job.ErrorCode
			}
		} else if previousJob != job.ID {
			a.View.AttentionRequired = false
		}
		a.View.Revision++
		if err := s.saveAccountLocked(a); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) checkAccountRepair(a storedAccount, evidence string) error {
	health, err := s.opts.Manager.LoginHealth(a.View.AuthID)
	if err != nil {
		return err
	}
	if !health.AutoEligible || health.Fingerprint != evidence || health.IdentityVersion != a.View.IdentityVersion || health.Principal != a.Principal {
		return errors.New("policy_changed")
	}
	return nil
}
func (s *Service) InspectAccounts(id string) ([]Account, error) {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	if err := s.accountReady(); err != nil {
		return nil, err
	}
	if id != "" {
		if _, ok := s.accounts[id]; !ok {
			return nil, errors.New("not_found")
		}
	}
	if err := s.syncAccountJobsLocked(); err != nil {
		return nil, err
	}
	for key, a := range s.accounts {
		if id != "" && id != key {
			continue
		}
		if err := s.inspectAccountLocked(a, false); err != nil {
			return nil, err
		}
	}
	return s.accountViewsLocked(id), nil
}
func (s *Service) inspectAccountLocked(a storedAccount, automatic bool) error {
	now := s.opts.Now()
	a.View.LastCheckedAt = now.Format(time.RFC3339)
	a.View.NextCheckAt = ""
	if s.accountSettings.InspectionEnabled {
		a.View.NextCheckAt = now.Add(time.Duration(s.accountSettings.IntervalMinutes) * time.Minute).Format(time.RFC3339)
	}
	eligible := false
	if a.View.AuthID == "" {
		a.View.HealthStatus = "not_linked"
		a.View.HealthReason = "login_required"
	} else {
		health, err := s.opts.Manager.LoginHealth(a.View.AuthID)
		if err != nil {
			a.View.HealthStatus = "unknown"
			a.View.HealthReason = SafeCode(err)
		} else {
			a.View.HealthStatus = health.Status
			a.View.HealthReason = health.Reason
			if health.Exists && (health.IdentityVersion != a.View.IdentityVersion || health.Principal != a.Principal) {
				a.View.HealthStatus = "identity_changed"
				a.View.HealthReason = "identity_changed"
				a.View.AttentionRequired = true
			} else {
				eligible = health.AutoEligible
				if health.Fingerprint != "" && health.Fingerprint != a.Evidence {
					a.Evidence = health.Fingerprint
					a.View.ConsecutiveFailures = 0
					a.NextAttempt = time.Time{}
				}
			}
		}
	}
	a.View.Revision++
	if err := s.saveAccountLocked(a); err != nil {
		return err
	}
	if automatic && s.accountAutomation.Load() && a.View.AutoRelogin && !a.View.AttentionRequired && eligible && a.Evidence != "" && a.ConsumedEvidence != a.Evidence && !now.Before(a.NextAttempt) && !s.accountBusyLocked(a) {
		_, err := s.startAccountLocked(a, "", true, a.Evidence)
		if err != nil {
			a = s.accounts[a.View.ID]
			a.View.HealthReason = SafeCode(err)
			a.NextAttempt = now.Add(time.Hour)
			if err.Error() == "queue_full" || err.Error() == "rate_limited" || err.Error() == "conflict" {
				a.ConsumedEvidence = ""
			}
			a.View.Revision++
			if err.Error() != "queue_full" && err.Error() != "rate_limited" && err.Error() != "conflict" {
				a.View.AttentionRequired = true
			}
			return s.saveAccountLocked(a)
		}
	}
	return nil
}
func (s *Service) maintainAccounts() {
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	if err := s.accountReady(); err != nil {
		return
	}
	if err := s.syncAccountJobsLocked(); err != nil {
		return
	}
	now := s.opts.Now()
	for _, a := range s.accounts {
		if a.View.LastJobID != "" {
			job, err := s.Detail(a.View.LastJobID)
			if err == nil && job.Automatic && !terminal(job.Status) && job.Status != "committing" && (!s.accountAutomation.Load() || !a.View.AutoRelogin) {
				_, _ = s.Cancel(job.ID, true)

			}
		}
		if !s.accountSettings.InspectionEnabled {
			continue
		}
		due, err := time.Parse(time.RFC3339, a.View.NextCheckAt)
		if err != nil || !now.Before(due) {
			_ = s.inspectAccountLocked(a, true)
		}
	}
}
