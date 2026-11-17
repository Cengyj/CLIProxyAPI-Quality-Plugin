package login

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MoeForge/moego/base64"
	"github.com/MoeForge/moego/ojson"
	auth "github.com/cliproxyapi-plugins/model-quality/login/account"
	codex "github.com/cliproxyapi-plugins/model-quality/login/oauth"
	"github.com/google/uuid"
)

type Owner struct {
	ID      string
	Valid   func() bool
	Prepare func(context.Context, string, string, string) error
}
type Options struct {
	Call            func(method string, request, response any) error
	RuntimeManifest string
	Prepare         func(context.Context, string, string) error
	DefaultProxyURL string
	Dir             string
	Config          *codex.Config
	Manager         *auth.Manager
	Owner           func() (Owner, error)
	Runtime         interface {
		Capabilities() error
		Run(context.Context, RuntimeRequest) (string, error)
	}
	// Exchange is injected only by in-process offline tests, never management input.
	Exchange func(context.Context, string, *codex.PKCECodes, string) (*codex.CodexAuthBundle, error)
	Now      func() time.Time
}
type session struct {
	displayEmail      string
	autoGuard         func() error
	record            storedJob
	owner             Owner
	ctx               context.Context
	cancel            context.CancelFunc
	pkce              *codex.PKCECodes
	state, url, proxy string
	workspace         string
	expected          auth.LoginSnapshot
	credentials       Credentials
	remember          bool
	candidate         *auth.Auth
	consumed          bool
}
type Service struct {
	accountMu         sync.Mutex
	accounts          map[string]storedAccount
	accountPermits    map[string]*atomic.Bool
	accountSettings   AccountSettings
	accountAutomation atomic.Bool
	accountGeneration atomic.Uint64
	loginProxy        atomic.Value
	mu                sync.Mutex
	journal           *journal
	vault             *vault
	opts              Options
	jobs              map[string]*session
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	slot              chan struct{}
	closed            bool
	fault             bool
	lastPrune         time.Time
	ownsManager       bool
}

func New(opts Options) (*Service, error) {
	if opts.Config == nil {
		opts.Config = &codex.Config{ProxyURL: opts.DefaultProxyURL}
	}
	owned := opts.Manager == nil
	if owned {
		var err error
		opts.Manager, err = auth.New(opts.Dir, opts.Call)
		if err != nil {
			return nil, err
		}
	}
	initialized := false
	defer func() {
		if owned && !initialized {
			_ = opts.Manager.Close()
		}
	}()
	if opts.Owner == nil {
		opts.Owner = func() (Owner, error) {
			return Owner{ID: "plugin", Valid: opts.Manager.Valid, Prepare: func(ctx context.Context, id, identity, _ string) error {
				if opts.Prepare != nil {
					return opts.Prepare(ctx, id, identity)
				}
				return ctx.Err()
			}}, nil
		}
	}
	if opts.Now == nil {
		opts.Now = func() time.Time { return time.Now().UTC() }
	}
	j, err := openJournal(opts.Dir)
	if err != nil {
		return nil, err
	}
	j.now = opts.Now
	if _, err = j.prune(opts.Now()); err != nil {
		_ = j.db.Close()
		return nil, errors.New("storage_unavailable")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{journal: j, opts: opts, jobs: map[string]*session{}, ctx: ctx, cancel: cancel, slot: make(chan struct{}, 1), lastPrune: opts.Now(), ownsManager: owned}
	s.vault = &vault{journal: j, manager: opts.Manager}
	if opts.Runtime == nil {
		s.opts.Runtime = NewRuntime(opts.RuntimeManifest)
	}
	if opts.Exchange == nil {
		s.opts.Exchange = func(ctx context.Context, code string, p *codex.PKCECodes, proxy string) (*codex.CodexAuthBundle, error) {
			return codex.NewCodexAuthWithProxyURL(opts.Config, proxy).ExchangeLoginCode(ctx, code, p)
		}
	}
	old, err := j.load()
	if err != nil {
		_ = j.db.Close()
		cancel()
		return nil, errors.New("storage_unavailable")
	}
	for _, v := range old {
		if !terminal(v.View.Status) || v.View.Status == "saved_not_adopted" || v.View.Status == "commit_unknown" {
			if receipt, receiptErr := opts.Manager.LoginCommitStatus(v.View.ID); receiptErr == nil {
				v.View.Status = reconcileStatus(receipt.State)
				v.View.TargetAuthID = receipt.AuthID
				v.View.CredentialEnabled = receipt.State == "committed" && receipt.CredentialEnabled
				v.AdoptedIdentity = receipt.IdentityVersion
				v.AdoptedPrincipal = receipt.Principal
				v.View.ErrorCode = ""
				if v.View.Status == "commit_unknown" || v.View.Status == "saved_not_adopted" {
					v.View.ErrorCode = v.View.Status
				}
			} else {
				if v.View.Status == "committing" || v.View.Status == "saved_not_adopted" || v.View.Status == "commit_unknown" {
					v.View.Status = "commit_unknown"
					v.View.ErrorCode = "commit_unknown"
				} else {
					v.View.Status = "interrupted"
					v.View.ErrorCode = "interrupted"
				}
			}
			v.View.Revision++
			v.View.Phase = ""
			v.View.CandidateID = ""
			v.Updated = opts.Now()
			if err = j.save(v); err != nil {
				_ = j.db.Close()
				cancel()
				return nil, errors.New("storage_unavailable")
			}
		}
		s.jobs[v.View.ID] = &session{record: v}
	}
	if err = s.initAccounts(); err != nil {
		_ = j.db.Close()
		cancel()
		return nil, err
	}
	if err = s.recoverSecurity(); err != nil {
		_ = j.db.Close()
		cancel()
		return nil, err
	}
	s.wg.Add(1)
	go s.monitor()
	initialized = true
	return s, nil
}
func reconcileStatus(status string) string {
	switch status {
	case "committed", "saved_not_adopted", "commit_unknown":
		return status
	case "aborted":
		return "failed"
	default:
		return "commit_unknown"
	}
}
func terminal(v string) bool {
	switch v {
	case "succeeded", "security_uncertain", "committed", "failed", "cancelled", "discarded", "expired", "interrupted", "interaction_required", "conflict", "saved_not_adopted", "commit_unknown":
		return true
	}
	return false
}
func (s *Service) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.cancel()
	for _, j := range s.jobs {
		if j.cancel != nil && j.record.View.Status != "committing" {
			j.cancel()
			j.candidate = nil
			j.pkce = nil
			j.state, j.url = "", ""
			j.credentials = Credentials{}
		}
	}
	s.mu.Unlock()
	s.wg.Wait()
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	clear(s.accounts)
	err := s.journal.db.Close()
	if s.ownsManager {
		err = errors.Join(err, s.opts.Manager.Close())
	}
	return err
}
func (s *Service) persistLocked(j *session) error {
	j.record.Updated = s.opts.Now()
	if err := s.journal.save(j.record); err != nil {
		s.fault = true
		s.cancel()
		return errors.New("storage_unavailable")
	}
	return nil
}
func (s *Service) transitionLocked(j *session, status, phase, code string) error {
	if j.record.View.Status == status && j.record.View.Phase == phase && j.record.View.ErrorCode == code {
		return nil
	}
	j.record.View.Status = status
	j.record.View.Phase = phase
	j.record.View.ErrorCode = code
	if code == "" {
		j.record.View.ErrorStage, j.record.View.ErrorReason = "", ""
	}
	j.record.View.Revision++
	if terminal(status) {
		j.candidate = nil
		j.pkce = nil
		j.state = ""
		j.url = ""
		j.credentials = Credentials{}
		j.record.View.CandidateID = ""
	}
	return s.persistLocked(j)
}
func (s *Service) viewLocked(j *session) Job {
	v := j.record.View
	v.Email = j.displayEmail
	if v.Email == "" {
		v.Email = j.credentials.Email
	}
	if j.candidate != nil {
		if p, err := auth.LoginPrincipalFromAuth(j.candidate); err == nil {
			v.Email = p.Email
		}
	}
	if v.Email == "" && v.AccountID != "" {
		v.Email = s.storedAccountEmail(v.AccountID)
	}
	if v.Email == "" && v.TargetAuthID != "" {
		principal := j.record.AdoptedPrincipal
		if principal == "" {
			principal = j.expected.Principal
		}
		if a, ok := s.opts.Manager.GetByID(v.TargetAuthID); ok {
			if p, err := auth.LoginPrincipalFromAuth(a); err == nil && principal != "" && p.Key() == principal {
				v.Email = p.Email
			}
		}
	}
	if v.Status == "waiting_browser" && j.owner.Valid != nil && j.owner.Valid() {
		v.AuthorizationURL = j.url
	}
	return v
}
func (s *Service) Detail(id string) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j := s.jobs[id]
	if j == nil {
		return Job{}, errors.New("not_found")
	}
	return s.viewLocked(j), nil
}
func (s *Service) Jobs() []Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []Job{}
	for _, j := range s.jobs {
		v := s.viewLocked(j)
		v.AuthorizationURL = ""
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	if len(out) > 200 {
		out = out[:200]
	}
	return out
}
func (s *Service) Capabilities() any {
	owner, err := s.opts.Owner()
	capabilityErr := s.opts.Manager.LoginCapability()
	s.mu.Lock()
	ready := !s.closed && !s.fault
	s.mu.Unlock()
	available := ready && err == nil && owner.Valid != nil && owner.Valid() && capabilityErr == nil
	reason := ""
	if !available {
		reason = "plugin_unavailable"
		if capabilityErr != nil {
			reason = SafeCode(capabilityErr)
		}
		if !ready {
			reason = "storage_unavailable"
		}
	}
	runtimeErr := s.opts.Runtime.Capabilities()
	result := map[string]any{"security_actions": map[string]bool{"reset_totp": available && runtimeErr == nil && s.securityAvailable(), "logout_all": available && runtimeErr == nil && s.securityAvailable()}, "available": available, "version": 1, "automatic_commit": true, "commit_mode": "account_auto_commit", "manual_oauth_commit": true, "reason": reason, "methods": map[string]any{"browser": map[string]any{"available": available}, "password_totp": map[string]any{"available": available && runtimeErr == nil, "reason": SafeCode(runtimeErr)}}, "vault": s.vault.status("")}
	if metadata, ok := s.opts.Runtime.(interface {
		Metadata() (map[string]string, error)
	}); ok && runtimeErr == nil {
		if value, metadataErr := metadata.Metadata(); metadataErr == nil {
			result["runtime"] = value
		}
	}
	return result
}
func (s *Service) Targets() []map[string]any {
	out := []map[string]any{}
	for _, a := range s.opts.Manager.List() {
		if a.Provider != "codex" {
			continue
		}
		snapshot, err := s.opts.Manager.LoginSnapshot(a.ID)
		p, _ := auth.LoginPrincipalFromAuth(a)
		target, _ := s.opts.Manager.QualityTarget(a.ID)
		out = append(out, map[string]any{"auth_id": a.ID, "auth_index": a.Index, "email": p.Email, "account_id": p.AccountID, "user_id": p.UserID, "masked_email": Mask(p.Email), "identity_version": snapshot.IdentityVersion, "can_update": err == nil, "reason": SafeCode(err), "quality_paused": target.QualityPaused, "manual_disabled": target.OperatorDisabled})
	}
	return out
}
func (s *Service) Start(r StartRequest) (Job, error) {
	if err := s.opts.Manager.LoginCapability(); err != nil {
		return Job{}, err
	}
	owner, err := s.opts.Owner()
	if err != nil || owner.Valid == nil || !owner.Valid() {
		return Job{}, errors.New("plugin_unavailable")
	}
	if r.securityAction != "" && (r.accountID == "" || !validSecurityAction(r.securityAction) || !s.securityAvailable()) {
		return Job{}, errors.New("security_unavailable")
	}
	if r.Method == "password_totp" {
		if err = s.opts.Runtime.Capabilities(); err != nil {
			return Job{}, err
		}
	}
	var expected auth.LoginSnapshot
	workspace := ""
	proxy, _, err := s.effectiveLoginProxy(r.ProxyURL, r.TargetAuthID)
	if err != nil {
		return Job{}, err
	}
	if r.Intent == "reauthorize" {
		expected, err = s.opts.Manager.LoginSnapshot(r.TargetAuthID)
		if err != nil {
			return Job{}, err
		}
		if expected.IdentityVersion != r.ExpectedIdentityVersion {
			return Job{}, errors.New("identity_changed")
		}
		if a, ok := s.opts.Manager.GetByID(r.TargetAuthID); ok {
			principal, principalErr := auth.LoginPrincipalFromAuth(a)
			if principalErr != nil {
				return Job{}, principalErr
			}
			workspace = principal.AccountID
		}
	}
	if r.UseSavedCredentials {
		r.Credentials, err = s.vault.read(r.TargetAuthID)
		if err != nil {
			return Job{}, err
		}
	}
	if r.RememberCredentials {
		if err = s.vault.ready(); err != nil {
			return Job{}, err
		}
	}
	pkce, err := codex.GeneratePKCECodes()
	if err != nil {
		return Job{}, errors.New("login_failed")
	}
	state := uuid.NewString() + uuid.NewString()
	authURL, err := codex.NewCodexAuth(s.opts.Config).GenerateAuthURL(state, pkce)
	if err != nil {
		return Job{}, errors.New("login_failed")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.fault {
		return Job{}, errors.New("storage_unavailable")
	}
	now := s.opts.Now()
	removed, err := s.journal.prune(now)
	if err != nil {
		return Job{}, errors.New("storage_unavailable")
	}
	for _, id := range removed {
		delete(s.jobs, id)
	}
	if err = s.journal.admit(); err != nil {
		return Job{}, err
	}
	key := s.journal.fingerprint(r.Credentials.Email)
	if r.TargetAuthID != "" {
		key = s.journal.fingerprint(expected.Principal)
		if a, found := s.opts.Manager.GetByID(r.TargetAuthID); found {
			if p, principalErr := auth.LoginPrincipalFromAuth(a); principalErr == nil && p.Email != "" {
				key = s.journal.fingerprint(p.Email)
			}
		}
	}
	active, attempts := 0, 0
	for _, j := range s.jobs {
		if securityBlocks(j.record) && (j.record.RateKey == key || r.TargetAuthID != "" && r.TargetAuthID == j.record.View.TargetAuthID) {
			return Job{}, errors.New("security_pending")
		}
		if !terminal(j.record.View.Status) {
			if r.securityAction != "" && j.candidate != nil {
				if p, err := auth.LoginPrincipalFromAuth(j.candidate); err == nil && s.journal.fingerprint(p.Email) == key {
					return Job{}, errors.New("conflict")
				}
			}
			active++
			if j.record.RateKey == key {
				return Job{}, errors.New("conflict")
			}
		}
		if j.record.RateKey == key && now.Sub(j.record.Created) < time.Hour {
			attempts++
		}
	}
	if active >= 16 {
		return Job{}, errors.New("queue_full")
	}
	if attempts >= 3 {
		return Job{}, errors.New("rate_limited")
	}
	id := uuid.NewString()
	ctx, cancel := context.WithDeadline(s.ctx, now.Add(15*time.Minute))
	j := &session{displayEmail: r.Credentials.Email, autoGuard: r.autoGuard, record: storedJob{SecurityAccountID: workspace, AccountEvidence: r.autoEvidence, AccountSequence: r.accountSequence, View: Job{Action: r.securityAction, LogoutAllDevices: r.logoutAllDevices, Reauthorize: r.reauthorize, AccountID: r.accountID, Automatic: r.automatic, AutoCommit: r.autoCommit, EnableCredential: r.enableCredential, ID: id, Revision: 1, Method: r.Method, Intent: r.Intent, Status: "queued", Phase: "queued", MaskedEmail: Mask(r.Credentials.Email), TargetAuthID: r.TargetAuthID, CreatedAt: now.Format(time.RFC3339Nano), ExpiresAt: now.Add(15 * time.Minute).Format(time.RFC3339)}, Created: now, Updated: now, RateKey: key}, owner: owner, ctx: ctx, cancel: cancel, pkce: pkce, state: state, url: authURL, proxy: proxy, workspace: workspace, expected: expected, credentials: r.Credentials, remember: r.RememberCredentials}
	if r.securityAction != "" {
		j.record.View.SecurityStatus = "pending"
		if _, err = s.securityJournalPath(id, true); err != nil {
			cancel()
			return Job{}, err
		}
	}
	if err = s.persistLocked(j); err != nil {
		cancel()
		return Job{}, err
	}
	s.jobs[id] = j
	if r.Method == "browser" {
		err = s.transitionLocked(j, "waiting_browser", "authorization", "")
	} else {
		s.wg.Add(1)
		go s.run(j)
	}
	return s.viewLocked(j), err
}
func (s *Service) checkAutomaticExecution(j *session) error {
	if j.autoGuard != nil {
		if err := j.autoGuard(); err != nil {
			return err
		}
	}
	if j.record.View.Automatic {
		// Queue admission may precede a pause, cooldown, or replacement grant.
		// Recheck frozen evidence before submitting credentials or an OAuth code.
		health, err := s.opts.Manager.LoginHealth(j.expected.AuthID)
		if err != nil || !health.AutoEligible || health.Fingerprint != j.record.AccountEvidence ||
			health.IdentityVersion != j.expected.IdentityVersion || health.Principal != j.expected.Principal {
			return errors.New("policy_changed")
		}
	}
	return nil
}
func (s *Service) run(j *session) {
	defer s.wg.Done()
	securityReconciled := false
	if j.record.View.Action != "" {
		defer func() { s.finishSecurityRun(j, !securityReconciled) }()
	}
	select {
	case s.slot <- struct{}{}:
		defer func() { <-s.slot }()
	case <-j.ctx.Done():
		s.finishError(j, j.ctx.Err())
		return
	}
	if err := s.checkAutomaticExecution(j); err != nil {
		s.finishError(j, err)
		return
	}
	s.mu.Lock()
	if terminal(j.record.View.Status) || j.ctx.Err() != nil {
		s.mu.Unlock()
		s.finishError(j, errors.New("cancelled"))
		return
	}
	if err := s.transitionLocked(j, "running", "authorization", ""); err != nil {
		s.mu.Unlock()
		return
	}
	req := RuntimeRequest{Operation: j.record.View.Action, LogoutAfterReset: j.record.View.LogoutAllDevices, Reauthorize: j.record.View.Reauthorize, Version: 1, JobID: j.record.View.ID, Attempt: 1, AuthorizationURL: j.url, RedirectURI: codex.RedirectURI, State: j.state, Credentials: j.credentials, ProxyURL: j.proxy, WorkspaceID: j.workspace}
	if req.Operation != "" {
		req.SecurityJournalPath, _ = s.securityJournalPath(j.record.View.ID, false)
		j.record.SecurityStarted = true
		j.record.View.SecurityStatus = "running"
		if err := s.persistLocked(j); err != nil {
			s.mu.Unlock()
			return
		}
	}
	if req.ProxyURL == "direct" {
		req.ProxyURL = ""
	}
	if !j.remember {
		j.credentials = Credentials{}
	}
	s.mu.Unlock()
	var callback string
	var err error
	if progressive, ok := s.opts.Runtime.(interface {
		RunWithProgress(context.Context, RuntimeRequest, func(string) error) (string, error)
	}); ok {
		callback, err = progressive.RunWithProgress(j.ctx, req, func(phase string) error { return s.runtimeProgress(j, phase) })
	} else {
		callback, err = s.opts.Runtime.Run(j.ctx, req)
	}
	req.Credentials = Credentials{}
	if req.Operation != "" {
		securityReconciled = true
		if reconcileErr := s.reconcileSecurity(j); reconcileErr != nil && (err == nil || reconcileErr.Error() != "security_failed") {
			s.finishError(j, reconcileErr)
			return
		}
	}
	if err != nil {
		s.finishError(j, err)
		return
	}
	if req.Operation != "" && !req.Reauthorize {
		s.mu.Lock()
		if !terminal(j.record.View.Status) && j.ctx.Err() == nil {
			_ = s.transitionLocked(j, "succeeded", "security_complete", "")
		}
		s.mu.Unlock()
		return
	}
	if _, err = s.Callback(j.record.View.ID, callback); err != nil {
		s.finishError(j, err)
	}
}

func (s *Service) runtimeProgress(j *session, phase string) error {
	if !runtimeStage(phase) {
		return runtimeError("runtime_protocol_error")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := j.ctx.Err(); err != nil {
		return err
	}
	if s.closed || s.fault || s.jobs[j.record.View.ID] != j || j.record.View.Status != "running" || !j.owner.Valid() {
		return runtimeError("cancelled")
	}
	return s.transitionLocked(j, "running", phase, "")
}

func (s *Service) finishError(j *session, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if terminal(j.record.View.Status) || j.record.View.Status == "committing" {
		return
	}
	code := SafeCode(err)
	status := "failed"
	if errors.Is(err, context.Canceled) || j.record.View.Status == "cancelling" {
		status = "cancelled"
		code = "cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		status = "expired"
		code = "expired"
	}
	if code == "interaction_required" {
		status = code
	}
	j.record.View.ErrorStage, j.record.View.ErrorReason = "", ""
	var diagnostic *runtimeDiagnosticError
	if errors.As(err, &diagnostic) && status != "cancelled" && status != "expired" {
		j.record.View.ErrorStage, j.record.View.ErrorReason = diagnostic.stage, diagnostic.reason
	}
	_ = s.transitionLocked(j, status, "", code)
	if j.cancel != nil {
		j.cancel()
	}
}
func (s *Service) Callback(id, callback string) (Job, error) {
	s.mu.Lock()
	j := s.jobs[id]
	if j == nil {
		s.mu.Unlock()
		return Job{}, errors.New("not_found")
	}
	if j.consumed || (j.record.View.Status != "waiting_browser" && j.record.View.Status != "running") {
		s.mu.Unlock()
		return Job{}, errors.New("already_consumed")
	}
	if j.ctx.Err() != nil || !j.owner.Valid() {
		s.mu.Unlock()
		return Job{}, errors.New("cancelled")
	}
	code, err := validateCallback(callback, j.state)
	if err != nil {
		s.mu.Unlock()
		return Job{}, err
	}
	j.consumed = true
	pkce := j.pkce
	j.pkce = nil
	if err = s.transitionLocked(j, "exchanging", "token_exchange", ""); err != nil {
		s.mu.Unlock()
		return Job{}, err
	}
	s.wg.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.wg.Done()
		select {
		case s.slot <- struct{}{}:
			defer func() { <-s.slot }()
		case <-j.ctx.Done():
			s.finishError(j, j.ctx.Err())
			return
		}
		if err := j.ctx.Err(); err != nil {
			s.finishError(j, err)
			return
		}
		if err := s.checkAutomaticExecution(j); err != nil {
			s.finishError(j, err)
			return
		}
		ctx, cancel := context.WithTimeout(j.ctx, 60*time.Second)
		defer cancel()
		bundle, err := s.opts.Exchange(ctx, code, pkce, j.proxy)
		if err != nil {
			s.finishError(j, err)
			return
		}
		candidate, err := validateBundle(bundle, s.opts.Now())
		if err != nil {
			s.finishError(j, err)
			return
		}
		p, _ := auth.LoginPrincipalFromAuth(candidate)
		s.mu.Lock()
		defer s.mu.Unlock()
		if j.ctx.Err() != nil || !j.owner.Valid() || j.record.View.Status != "exchanging" {
			if !terminal(j.record.View.Status) && j.record.View.Status != "committing" {
				status := "cancelled"
				if errors.Is(j.ctx.Err(), context.DeadlineExceeded) {
					status = "expired"
				}
				_ = s.transitionLocked(j, status, "", status)
			}
			return
		}
		for _, other := range s.jobs {
			if other != j && securityBlocks(other.record) && other.record.RateKey == s.journal.fingerprint(p.Email) {
				_ = s.transitionLocked(j, "conflict", "", "security_pending")
				return
			}
		}
		if j.record.View.AccountID != "" && s.journal.fingerprint(p.Email) != j.record.RateKey {
			_ = s.transitionLocked(j, "conflict", "", "identity_conflict")
			return
		}
		if j.expected.AuthID != "" && p.Key() != j.expected.Principal {
			_ = s.transitionLocked(j, "conflict", "", "identity_conflict")
			return
		}
		existing, listErr := s.opts.Manager.ListChecked()
		if listErr != nil {
			_ = s.transitionLocked(j, "failed", "", SafeCode(listErr))
			return
		}
		for _, a := range existing {
			other, e := auth.LoginPrincipalFromAuth(a)
			if e == nil && other.Key() == p.Key() && a.ID != j.expected.AuthID {
				_ = s.transitionLocked(j, "conflict", "", "duplicate_identity")
				return
			}
		}
		if j.expected.AuthID != "" {
			candidate.ID = j.expected.AuthID
		} else {
			candidate.ID = uuid.NewString() + ".json"
		}
		candidate.FileName = candidate.ID
		candidate.ProxyURL = j.proxy
		j.candidate = candidate
		j.displayEmail = p.Email
		j.record.View.MaskedEmail = Mask(p.Email)
		j.record.View.ExpiresAt = s.opts.Now().Add(5 * time.Minute).Format(time.RFC3339)
		if j.record.View.AutoCommit {
			_ = s.beginCommitLocked(j)
		} else {
			j.record.View.CandidateID = id + ":candidate"
			_ = s.transitionLocked(j, "awaiting_commit", "review", "")
		}
	}()
	return s.Detail(id)
}
func (s *Service) Candidate(id string) (Candidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	jobID := strings.TrimSuffix(id, ":candidate")
	j := s.jobs[jobID]
	if j == nil || j.record.View.CandidateID != id || j.candidate == nil || j.record.View.Status != "awaiting_commit" {
		return Candidate{}, errors.New("not_found")
	}
	p, err := auth.LoginPrincipalFromAuth(j.candidate)
	if err != nil {
		return Candidate{}, err
	}
	restrictions := []string{"not_inference_verified", "rules_stopped"}
	if j.expected.AuthID == "" {
		restrictions = append(restrictions, "new_credential_disabled")
	} else {
		t, e := s.opts.Manager.QualityTarget(j.expected.AuthID)
		if e != nil {
			return Candidate{}, e
		}
		restrictions = append(restrictions, t.UnavailableReasons...)
	}
	return Candidate{ID: id, JobID: jobID, Revision: j.record.View.Revision, Email: p.Email, AccountID: p.AccountID, UserID: p.UserID, MaskedEmail: Mask(p.Email), AccountIDMasked: Mask(p.AccountID), TargetAuthID: j.candidate.ID, Restrictions: restrictions, Intent: j.record.View.Intent}, nil
}
func (s *Service) Cancel(id string, discard bool) (Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	j := s.jobs[id]
	if j == nil {
		return Job{}, errors.New("not_found")
	}
	if terminal(j.record.View.Status) || j.record.View.Status == "committing" {
		return s.viewLocked(j), nil
	}
	if j.cancel != nil {
		j.cancel()
	}
	status := "cancelled"
	if discard {
		status = "discarded"
	}
	if j.record.View.Status == "running" || j.record.View.Status == "exchanging" || j.record.View.Status == "cancelling" {
		status = "cancelling"
	}
	err := s.transitionLocked(j, status, "", "")
	return s.viewLocked(j), err
}
func (s *Service) Commit(ctx context.Context, id, candidateID string, revision uint64) (Job, error) {
	s.mu.Lock()
	j := s.jobs[id]
	if j == nil {
		s.mu.Unlock()
		return Job{}, errors.New("not_found")
	}
	if j.record.View.Status == "committed" || j.record.View.Status == "committing" {
		v := s.viewLocked(j)
		s.mu.Unlock()
		return v, nil
	}
	if j.record.View.Status != "awaiting_commit" || j.record.View.Revision != revision || j.record.View.CandidateID != candidateID || j.candidate == nil {
		s.mu.Unlock()
		return Job{}, errors.New("conflict")
	}
	if j.ctx.Err() != nil || !j.owner.Valid() {
		s.mu.Unlock()
		return Job{}, errors.New("cancelled")
	}
	expiry, errExpiry := time.Parse(time.RFC3339, j.record.View.ExpiresAt)
	if errExpiry != nil || !s.opts.Now().Before(expiry) {
		_ = s.transitionLocked(j, "expired", "", "expired")
		j.cancel()
		s.mu.Unlock()
		return Job{}, errors.New("expired")
	}
	err := s.beginCommitLocked(j)
	s.mu.Unlock()
	if err != nil {
		return Job{}, err
	}
	return s.Detail(id)
}

// beginCommitLocked records submission before starting the only credential write.
// Account login is authorized when the operator starts it; explicit OAuth callers
// still enter through Commit with their reviewed candidate and revision.
func (s *Service) beginCommitLocked(j *session) error {
	if j.candidate == nil {
		return errors.New("conflict")
	}
	principal, err := auth.LoginPrincipalFromAuth(j.candidate)
	if err != nil {
		return err
	}
	for _, other := range s.jobs {
		if other != j && securityBlocks(other.record) && other.record.RateKey == s.journal.fingerprint(principal.Email) {
			return errors.New("security_pending")
		}
	}
	candidate := j.candidate.Clone()
	expected := j.expected
	owner := j.owner
	id := j.record.View.ID
	enableCredential := j.record.View.EnableCredential
	accountID := j.record.View.AccountID
	if err := s.transitionLocked(j, "committing", "saving", ""); err != nil {
		return err
	}
	s.wg.Add(1)
	// The commit is detached from an HTTP disconnect after its gate is durable.
	go func() {
		defer s.wg.Done()
		commitCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		guard := func() error {
			if err := commitCtx.Err(); err != nil {
				return errors.New("cancelled")
			}
			if err := j.ctx.Err(); err != nil {
				return errors.New("cancelled")
			}
			if !owner.Valid() {
				return errors.New("plugin_unavailable")
			}
			return s.checkAutomaticExecution(j)
		}
		result, err := s.opts.Manager.CommitLoginCandidate(commitCtx, auth.LoginCommitRequest{CommitID: id, Candidate: candidate, Expected: expected, EnableCredential: enableCredential, Guard: guard, Prepare: func(ctx context.Context, authID, identity string) error {
			return owner.Prepare(ctx, authID, identity, id)
		}})
		status := result.State
		if status == "" {
			status = "failed"
		}
		if status != "committed" && status != "saved_not_adopted" && status != "commit_unknown" {
			status = "failed"
		}
		s.mu.Lock()
		if status == "committed" && j.remember {
			if e := s.vault.save(result.AuthID, 0, j.credentials, true); e != nil {
				j.record.View.VaultError = SafeCode(e)
			}
		}
		j.record.AdoptedIdentity = result.IdentityVersion
		j.record.AdoptedPrincipal = result.Principal
		j.record.View.TargetAuthID = result.AuthID
		j.record.View.CredentialEnabled = status == "committed" && result.CredentialEnabled
		_ = s.transitionLocked(j, status, "", SafeCode(err))
		if j.cancel != nil {
			j.cancel()
		}
		s.mu.Unlock()
		if accountID != "" {
			s.accountMu.Lock()
			_ = s.syncAccountJobsLocked()
			s.accountMu.Unlock()
		}
	}()
	return nil
}
func (s *Service) monitor() {
	defer s.wg.Done()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.maintain()
			s.maintainAccounts()
		}
	}
}
func (s *Service) maintain() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.fault {
		return
	}
	now := s.opts.Now()
	for _, j := range s.jobs {
		if terminal(j.record.View.Status) || j.record.View.Status == "committing" || j.record.View.Status == "cancelling" || j.cancel == nil {
			continue
		}
		expiry, _ := time.Parse(time.RFC3339, j.record.View.ExpiresAt)
		if !now.Before(expiry) || j.ctx.Err() != nil || !j.owner.Valid() {
			j.cancel()
			if j.record.View.Status != "running" && j.record.View.Status != "exchanging" {
				_ = s.transitionLocked(j, "expired", "", "expired")
			} else {
				_ = s.transitionLocked(j, "cancelling", "", "")
			}
		}
	}
	if !now.Before(s.lastPrune.Add(time.Hour)) {
		removed, err := s.journal.prune(now)
		if err != nil {
			s.fault = true
			s.cancel()
			return
		}
		for _, id := range removed {
			delete(s.jobs, id)
		}
		s.lastPrune = now
	}
}
func (s *Service) Idempotent(key, path string, body []byte, fn func() (any, error)) ([]byte, error) {
	raw, err := s.journal.idempotent(key, path, body, fn)
	if err != nil {
		return nil, err
	}
	// The journal excludes display identities. Reconstruct them for both first
	// responses and replay without persisting plaintext account material in idempotency records.
	d := ojson.NewDoc(raw)
	value := ojson.ParseBytes(raw)
	if strings.Contains(path, "/accounts/") {
		if items := value.Get("items"); items.IsArray() {
			for i, account := range items.Array() {
				_ = d.Set(fmt.Sprintf("items.%d.email", i), s.storedAccountEmail(account.Get("id").String()))
			}
		} else if value.Get("health_status").Exists() {
			_ = d.Set("email", s.storedAccountEmail(value.Get("id").String()))
		}
	}
	if value.Get("method").Exists() {
		if job, err := s.Detail(value.Get("id").String()); err == nil {
			_ = d.Set("email", job.Email)
		}
	}
	if value.Get("entry").Exists() {
		if c, err := s.vault.read(ojson.ParseBytes(body).Get("auth_id").String()); err == nil {
			_ = d.Set("entry.email", c.Email)
		}
	}
	return d.Bytes(), nil
}
func validateBundle(bundle *codex.CodexAuthBundle, now time.Time) (*auth.Auth, error) {
	if bundle == nil || bundle.TokenData.AccessToken == "" || bundle.TokenData.RefreshToken == "" || bundle.TokenData.IDToken == "" {
		return nil, errors.New("identity_unverifiable")
	}
	t := bundle.TokenData
	parts := strings.Split(t.IDToken, ".")
	if len(parts) != 3 {
		return nil, errors.New("identity_unverifiable")
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || !ojson.ValidBytes(raw) {
		return nil, errors.New("identity_unverifiable")
	}
	claims := ojson.ParseBytes(raw)
	aud := claims.Get("aud")
	audOK := aud.String() == codex.ClientID
	for _, v := range aud.Array() {
		if v.String() == codex.ClientID {
			audOK = true
		}
	}
	if !audOK || claims.Get("iss").String() != "https://auth.openai.com" || claims.Get("exp").Int() <= now.Unix() {
		return nil, errors.New("identity_unverifiable")
	}
	a := &auth.Auth{Provider: "codex", Metadata: map[string]any{"type": "codex", "id_token": t.IDToken, "access_token": t.AccessToken, "refresh_token": t.RefreshToken, "expired": t.Expire, "last_refresh": bundle.LastRefresh}}
	p, err := auth.LoginPrincipalFromAuth(a)
	if err != nil {
		return nil, err
	}
	a.Metadata["account_id"] = p.AccountID
	a.Metadata["email"] = p.Email
	return a, nil
}
