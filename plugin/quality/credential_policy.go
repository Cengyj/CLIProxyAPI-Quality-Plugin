package quality

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/MoeForge/moego/json"
	bolt "go.etcd.io/bbolt"
)

const dispositionPrefix = "credential-disposition:"
const disableCredential = "disable_credential"
const deleteCredential = "delete_credential"

// The native independent-pause action is intentionally kept separate from CPA's
// disabled boolean. Historical pause ownership never becomes a new API action.
func managedRule(r *Rule) bool {
	if r == nil {
		return false
	}
	for _, action := range r.UpstreamErrorActions {
		if credentialMutation(action) {
			return true
		}
	}
	return credentialMutation(r.Action)
}

func credentialMutation(action string) bool {
	return action == disableCredential || action == deleteCredential
}

// Omitted settings from older editors preserve the saved policy. An explicit
// empty object or observe-only entries clear it.
func prepareUpstreamActions(in, old *Rule) error {
	settings := in.UpstreamErrorActions
	if settings == nil && old != nil {
		settings = old.UpstreamErrorActions
	}
	in.UpstreamErrorActions = nil
	for condition, action := range settings {
		switch condition {
		case "unauthorized", "payment_required", "quota_exhausted", "model_unsupported":
		default:
			return fmt.Errorf("invalid_request: unknown upstream error condition")
		}
		if action == "observe_only" {
			continue
		}
		if !credentialMutation(action) {
			return fmt.Errorf("invalid_request: invalid upstream error action")
		}
		if in.UpstreamErrorActions == nil {
			in.UpstreamErrorActions = make(map[string]string)
		}
		in.UpstreamErrorActions[condition] = action
	}
	return nil
}

func upstreamAction(f *Frozen, s *Sample) string {
	if f.ManagedProbe == nil || s.UpstreamCondition == "" || s.ExecutionStatus != "failed" || s.QualityVerdict != "unknown" {
		return ""
	}
	e := s.Evidence
	if e.Path != "cpa_management_probe" || e.ConditionsChanged || e.AuthID != f.AuthID || e.RequestedModel != s.Model || e.PayloadFingerprint == "" ||
		s.Index < 0 || s.Index >= f.Samples || !contains(f.Models, s.Model) {
		return ""
	}
	if action := f.UpstreamErrorActions[s.UpstreamCondition]; credentialMutation(action) {
		return action
	}
	return ""
}

type CredentialDisposition struct {
	CatalogMissingModel string             `json:"catalog_missing_model,omitempty"`
	UpstreamCondition   string             `json:"upstream_condition,omitempty"`
	Email               string             `json:"email,omitempty"`
	AuthID              string             `json:"auth_id"`
	RuleID              string             `json:"rule_id"`
	RuleRevision        uint64             `json:"rule_revision"`
	ActionID            string             `json:"action_id"`
	RunID               string             `json:"run_id"`
	Operation           string             `json:"operation"`
	State               string             `json:"state"`
	Snapshot            CredentialSnapshot `json:"snapshot"`
	Models              []string           `json:"models"`
	CreatedAt           time.Time          `json:"created_at"`
	UpdatedAt           time.Time          `json:"updated_at"`
}

func (en *Engine) loadDispositions() error {
	en.managed = map[string]*CredentialDisposition{}
	return en.store.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte("meta")).Cursor()
		for key, raw := c.Seek([]byte(dispositionPrefix)); key != nil && strings.HasPrefix(string(key), dispositionPrefix); key, raw = c.Next() {
			var d CredentialDisposition
			if err := json.Unmarshal(raw, &d); err != nil {
				return err
			}
			if d.AuthID == "" || string(key) != dispositionPrefix+d.AuthID {
				return fmt.Errorf("invalid credential disposition identity")
			}
			en.managed[d.AuthID] = &d
		}
		return nil
	})
}

func activeDisposition(d *CredentialDisposition) bool {
	return d != nil && d.State != "restored" && d.State != "externally_enabled" && d.State != "acknowledged"
}

func pendingCredentialAction(state string) string {
	switch state {
	case "disable_pending":
		return disableCredential
	case "delete_pending":
		return deleteCredential
	case "restore_pending":
		return "restore_credential"
	}
	return ""
}

// Startup has exclusive access. A pending intent cannot establish whether CPA
// received the write. Preserve its evidence without consulting or changing CPA.
func (en *Engine) interruptCredentialWrites() error {
	for _, d := range en.managed {
		operation := pendingCredentialAction(d.State)
		if operation == "" {
			continue
		}
		c := changes()
		next := *d
		next.State, next.UpdatedAt = "attention", en.now()
		c.disposition = &next
		if r := en.runs[d.RunID]; r != nil && r.ActionStatus == d.State && r.Config.RuleID == d.RuleID && r.Config.AuthID == d.AuthID {
			updated := cloneRun(r)
			updated.CredentialAction, updated.ActionStatus = operation, "credential_action_uncertain"
			c.runs[r.ID] = updated
		}
		c.audits = append(c.audits, en.audit("credential_action_uncertain", d.RuleID, d.RunID, d.ActionID, "Plugin restarted before confirmation of "+operation))
		if err := en.commitLocked(c); err != nil {
			return err
		}
	}
	// Old history may outlive a handling marker that was explicitly acknowledged.
	for id, r := range en.runs {
		if op := pendingCredentialAction(r.ActionStatus); op != "" {
			c := changes()
			updated := cloneRun(r)
			updated.CredentialAction, updated.ActionStatus = op, "credential_action_uncertain"
			c.runs[id] = updated
			if err := en.commitLocked(c); err != nil {
				return err
			}
		}
	}
	return nil
}

type HandlingSummary struct {
	CatalogMissingModel string `json:"catalog_missing_model,omitempty"`
	UpstreamCondition   string `json:"upstream_condition,omitempty"`
	Email               string `json:"email,omitempty"`
	AuthID              string `json:"auth_id"`
	ActionID            string `json:"action_id"`
	State               string `json:"state"`
}

// Acknowledgment is an explicit local release of bookkeeping after an operator
// has enabled/replaced/removed the credential in CPA. It never changes CPA state.
func (en *Engine) acknowledgeHandling(ctx context.Context, ruleID, authID, actionID string) error {
	en.mu.Lock()
	defer en.mu.Unlock()
	if err := en.mutableLocked(); err != nil {
		return err
	}
	d := en.managed[authID]
	r := en.rules[ruleID]
	if d == nil || r == nil || d.RuleID != ruleID || d.ActionID != actionID || !activeDisposition(d) {
		return fmt.Errorf("conflict: credential handling record changed")
	}
	if _, busy := en.busyAuth[authID]; busy {
		return fmt.Errorf("conflict: wait for the active credential task to finish")
	}
	targets, err := en.host.Targets(ctx)
	if err != nil {
		return err
	}
	for _, t := range targets {
		if t.AuthID == authID && (t.ManualDisabled || t.QualityPaused || t.RuntimeStatus != "active") {
			return fmt.Errorf("blocked: first resolve this credential's disabled state in CPA")
		}
	}
	next := *d
	next.State = "acknowledged"
	next.UpdatedAt = en.now()
	c := changes()
	c.disposition = &next
	stopped := cloneRule(r)
	stopped.Enabled = false
	stopped.SchedulingEnabled = false
	stopped.Revision++
	resetRule(stopped)
	stopped.Status = ruleStatus(stopped)
	c.rules[r.ID] = stopped
	en.cancelRuleChangesLocked(c, r.ID, "credential_handling_acknowledged")
	c.audits = append(c.audits, en.audit("credential_handling_acknowledged", r.ID, "", d.ActionID, "Operator acknowledged external resolution; rule stopped; no CPA credential write"))
	return en.commitLocked(c)
}

func (en *Engine) validateManagedRule(in, old *Rule) error {
	if len(in.UpstreamErrorActions) > 0 && (in.Action == "pause_credential" || in.AutomaticPause) {
		return fmt.Errorf("invalid_request: upstream error actions cannot be combined with independent quality pause")
	}
	if in.AutoRestore && in.Action != disableCredential {
		return fmt.Errorf("invalid_request: automatic restore requires credential suspension")
	}
	if managedRule(in) {
		if !en.Capabilities().CredentialManagement {
			return fmt.Errorf("unsupported: CPA management adapter is unavailable")
		}
		if in.Judge.Mode != "" && in.Judge.Mode != "deterministic" || in.ControlAuthID != "" {
			return fmt.Errorf("invalid_request: credential handling requires standard-answer judging without a control credential")
		}
		if _, ok := en.host.(managedHost); !ok {
			return fmt.Errorf("unsupported: CPA credential actions unavailable")
		}
	}
	if old != nil {
		for _, d := range en.managed {
			if d.RuleID != old.ID || !activeDisposition(d) || d.State == "deleted" {
				continue
			}
			scopeChanged := in.TargetScope != old.TargetScope && !(dynamicScope(in.TargetScope) && dynamicScope(old.TargetScope))
			if in.Action != old.Action || fingerprint(in.RunConfig) != fingerprint(old.RunConfig) || scopeChanged {
				return fmt.Errorf("conflict: enable or resolve the suspended credential in CPA before changing the test or handling policy")
			}
		}
	}
	return nil
}

func canRecoverDisabled(r *Rule, d *CredentialDisposition) bool {
	return recoveryDenied(r, d).Code == ""
}

func recoveryDenied(r *Rule, d *CredentialDisposition) ScopeSkipDetail {
	if detail := errorPolicyDisabled(d); detail.Code != "" {
		return detail
	}
	if !r.AutoRestore {
		return ScopeSkipDetail{Code: "recovery_disabled", Reason: "规则未开启整轮通过后的自动恢复，不能复检停用凭证。"}
	}
	if r.Action != disableCredential {
		return ScopeSkipDetail{Code: "recovery_not_permitted", Reason: "当前处置策略没有停用凭证复检权限。"}
	}
	if activeDisposition(d) {
		if d.State != "disabled" {
			return ScopeSkipDetail{Code: "handling_blocked", Reason: "这份处置记录没有已确认的停用状态，不能复检。"}
		}
		if d.RuleID != r.ID {
			return ScopeSkipDetail{Code: "other_rule_owner", Reason: "停用记录属于另一条规则。"}
		}
	} else if r.TargetScope != scopeAllCredentials {
		return ScopeSkipDetail{Code: "recovery_not_permitted", Reason: "当前范围没有复检人工停用凭证的权限。"}
	}
	return ScopeSkipDetail{}
}

// The returned projection is local to the probe. CPA stays disabled throughout.
func (en *Engine) admitManagedLocked(ctx context.Context, r *Rule, t Target, validateOnly bool) (Target, *CredentialSnapshot, error) {
	h, ok := en.host.(managedHost)
	if !ok || !en.Capabilities().ManagementSession {
		return t, nil, fmt.Errorf("management_session_required: reconnect the authenticated plugin panel")
	}
	if en.localPause(RunConfig{AuthID: t.AuthID}) != "" || t.QualityPaused || t.RetainedPause {
		return t, nil, fmt.Errorf("blocked: retained quality pause")
	}
	snapshot, err := h.CredentialSnapshot(ctx, t.AuthID, t.AuthIndex)
	if err != nil {
		return t, nil, err
	}
	return en.admitSnapshotLocked(r, t, snapshot, validateOnly)
}

func (en *Engine) admitSnapshotLocked(r *Rule, t Target, snapshot CredentialSnapshot, validateOnly bool) (Target, *CredentialSnapshot, error) {
	d := en.managed[t.AuthID]
	if detail := errorPolicyDisabled(d); detail.Code != "" {
		return t, nil, &scopeRejection{"blocked: credential already disabled by error policy", detail}
	}
	if activeDisposition(d) {
		if d.State == "disabled" && !snapshot.Disabled {
			if err := en.observeEnabledTargetLocked(t, snapshot, d, validateOnly); err != nil {
				return t, nil, err
			}
		} else {
			if detail := recoveryDenied(r, d); detail.Code != "" {
				return t, &snapshot, &scopeRejection{"blocked: credential handling requires attention or recovery is disabled", detail}
			}
			return projectOwnedTarget(r, t, snapshot, d)
		}
	}
	if snapshot.Disabled {
		if detail := recoveryDenied(r, d); detail.Code != "" {
			return t, nil, &scopeRejection{"blocked: disabled credential is outside recovery scope", detail}
		}
	}
	return projectManagedTarget(r, t, snapshot)
}

// Retire only a confirmed suspension when CPA identifies its current live
// target independently. This is local reconciliation, with no credential write.
func (en *Engine) observeEnabledTargetLocked(t Target, s CredentialSnapshot, d *CredentialDisposition, validateOnly bool) error {
	if t.QualityPaused || t.RetainedPause || en.localPause(RunConfig{AuthID: t.AuthID}) != "" {
		return rejectScope("blocked: retained quality pause", "quality_paused", "凭证仍受质量暂停保护，禁止检测。")
	}
	if !ruleSubjectEnabled(t) {
		if t.ManualDisabled || t.RuntimeStatus == "disabled" {
			return &scopeRejection{"blocked: credential runtime and snapshot disagree", snapshotRuntimeConflict(t, false)}
		}
		return &scopeRejection{"blocked: credential unavailable", runtimeSkip(t)}
	}
	if s.AuthID != t.AuthID || s.AuthIndex != t.AuthIndex || s.Provider != t.Provider || s.Name == "" || s.Fingerprint == "" || s.UpdatedAt.IsZero() {
		return rejectScope("blocked: current credential identity cannot be verified", "identity_conflict", "CPA 当前文件快照与本轮目录凭证标识不一致，无法核实当前凭证。")
	}
	changed := !sameCredential(s, d.Snapshot)
	if changed && !t.catalogVerified && !t.identityVerified {
		return rejectScope("blocked: current credential catalog identity cannot be verified", "identity_conflict", "CPA 未能从当前目录唯一核实已启用的凭证，原停用记录已保留。")
	}
	if validateOnly {
		return nil
	}
	next := *d
	next.State, next.UpdatedAt = "externally_enabled", en.now()
	kind, detail := "credential_external_enable", "CPA enablement observed. Previous automatic restore ownership relinquished."
	if changed {
		kind, detail = "credential_external_change", "CPA reports an enabled credential with a changed file or identity. The previous suspension was retired locally without a credential write."
	}
	c := changes()
	c.disposition = &next
	c.audits = append(c.audits, en.audit(kind, d.RuleID, d.RunID, d.ActionID, detail))
	return en.commitLocked(c)
}

func projectOwnedTarget(r *Rule, t Target, snapshot CredentialSnapshot, d *CredentialDisposition) (Target, *CredentialSnapshot, error) {
	if !sameSnapshot(snapshot, d.Snapshot) {
		return t, nil, rejectScope("blocked: suspended credential was modified; automatic restore stopped", "snapshot_changed", snapshotDifference(snapshot, d.Snapshot))
	}
	if len(d.Models) != len(r.Models) {
		return t, nil, rejectScope("blocked: recovery must test the original complete model selection", "recovery_models_changed", "当前所选模型数量与停用记录中的数量不同。")
	}
	for _, model := range r.Models {
		if !contains(d.Models, model) {
			return t, nil, &scopeRejection{"blocked: recovery must test the original complete model selection", ScopeSkipDetail{Code: "recovery_models_changed", Model: model, Reason: "当前所选模型不在停用时的模型集合中。"}}
		}
	}
	return projectManagedTarget(r, t, snapshot)
}

func projectManagedTarget(r *Rule, t Target, snapshot CredentialSnapshot) (Target, *CredentialSnapshot, error) {
	if snapshot.Disabled {
		if t.RuntimeStatus != "disabled" || !t.ManualDisabled {
			return t, nil, &scopeRejection{"blocked: disabled credential is outside recovery scope", snapshotRuntimeConflict(t, true)}
		}
		if t.RetainedPause || t.QualityPaused {
			return t, nil, rejectScope("blocked: disabled credential is outside recovery scope", "quality_paused", "凭证仍受质量暂停保护，禁止检测。")
		}
		for _, reason := range t.Reasons {
			if reason != "operator_disabled" && reason != "manual_disabled" {
				return t, nil, &scopeRejection{"blocked: credential has additional runtime restrictions", runtimeSkip(t)}
			}
		}
		t.ManualDisabled, t.EffectiveAvailable, t.RuntimeStatus = false, true, "active"
		// CPA unregisters models on disable. A recovery run still has to
		// receive a complete, judged answer for every configured model.
		if len(t.Models) == 0 {
			t.Models = append([]string(nil), r.Models...)
		}
	}
	if !ruleSubjectEnabled(t) {
		detail := runtimeSkip(t)
		if !snapshot.Disabled && (t.RuntimeStatus == "disabled" || t.ManualDisabled) {
			detail = snapshotRuntimeConflict(t, false)
		}
		return t, nil, &scopeRejection{"blocked: credential is disabled or unavailable", detail}
	}
	return t, &snapshot, nil
}

func (en *Engine) managedTargetsLocked(ctx context.Context, r *Rule, targets []Target, validateOnly bool) ([]Target, *CredentialSnapshot, error) {
	if !managedRule(r) {
		return targets, nil, nil
	}
	out := append([]Target(nil), targets...)
	for i, t := range out {
		if t.AuthID == r.AuthID {
			adjusted, snapshot, err := en.admitManagedLocked(ctx, r, t, validateOnly)
			out[i] = adjusted
			return out, snapshot, err
		}
	}
	return out, nil, fmt.Errorf("blocked: credential is missing")
}

func (en *Engine) checkManagedRunLocked(run *Run, targets []Target) ([]Target, error) {
	if run.Config.ManagedProbe == nil {
		return targets, nil
	}
	r := en.rules[run.Config.RuleID]
	if !managedRule(r) {
		return targets, fmt.Errorf("blocked: credential handling rule changed")
	}
	out := append([]Target(nil), targets...)
	for i, t := range out {
		if t.AuthID == run.Config.AuthID {
			expected := run.Config.ManagedProbe
			if t.AuthIndex != expected.AuthIndex || t.QualityPaused || t.RetainedPause || !expected.Disabled && activeDisposition(en.managed[t.AuthID]) {
				return nil, fmt.Errorf("blocked: credential handling changed after dispatch")
			}
			adjusted, _, err := en.admitSnapshotLocked(r, t, *expected, false)
			if err != nil {
				return nil, err
			}
			out[i] = adjusted
			return out, nil
		}
	}
	return nil, fmt.Errorf("blocked: credential is missing")
}

func managedRoundVerdict(run *Run) string {
	if run.Config.ManagedProbe == nil || run.Config.Judge.Mode != "deterministic" {
		return "unknown"
	}
	verdict := roundVerdict(run, func(s *Sample) bool {
		e := &s.Evidence
		return e.Path == "cpa_management_probe" && !e.ConditionsChanged && e.AuthID == run.Config.AuthID && e.RequestedModel == s.Model && e.PayloadFingerprint != "" && e.ResolvedModel != ""
	})
	if verdict == "mixed" {
		return "incorrect"
	}
	return verdict
}

func (en *Engine) stopManagedRules(c *changeSet, authID, owner string, stopOwner bool) {
	for id, old := range en.rules {
		if !usesCredential(old.RunConfig, authID) || id == owner && !stopOwner {
			continue
		}
		r := cloneRule(old)
		r.Enabled = false
		r.SchedulingEnabled = false
		r.Revision++
		resetRule(r)
		r.Status = ruleStatus(r)
		c.rules[id] = r
	}
}

func (en *Engine) cancelManagedRuns(c *changeSet, authID, exceptRun string) {
	for id, old := range en.runs {
		if id == exceptRun || !usesCredential(old.Config.RunConfig, authID) || (old.Status != "queued" && old.Status != "running") {
			continue
		}
		r := en.cancelledRunLocked(old, "blocked")
		r.BlockReason = "credential_handling"
		r.ActionStatus = "credential_handling"
		c.runs[id] = r
	}
}

func (en *Engine) decideManagedLocked(run *Run, r *Rule, c *changeSet) {
	out := c.runs[run.ID]
	r.FailCount = 0
	r.EvidenceSignature = ""
	c.rules[r.ID] = r
	verdict := managedRoundVerdict(run)
	operation := r.Action
	condition := ""
	if errorPolicyDisabled(en.managed[run.Config.AuthID]).Code != "" {
		out.ActionStatus = "credential_kept_disabled"
		_ = en.commitLocked(c)
		return
	}
	catalogAction := run.Config.CatalogMissingModel != ""
	if catalogAction {
		if !credentialMutation(r.UpstreamErrorActions["model_unsupported"]) {
			out.ActionStatus = "observe_only"
			_ = en.commitLocked(c)
			return
		}
		if run.Config.ManagedProbe == nil {
			out.ActionStatus = "catalog_action_unconfirmed"
			_ = en.commitLocked(c)
			return
		}
		operation, condition = r.UpstreamErrorActions["model_unsupported"], "model_unsupported"
	}
	if run.Status == "completed" && !run.CancelRequested && run.BlockReason == "" && len(run.Samples) > 0 {
		last := &run.Samples[len(run.Samples)-1]
		if action := upstreamAction(&run.Config, last); action != "" && action == r.UpstreamErrorActions[last.UpstreamCondition] {
			operation, condition = action, last.UpstreamCondition
			out.UpstreamCondition = condition
		}
	}
	if verdict == "unknown" && condition == "" {
		out.ActionStatus = "insufficient_evidence"
		_ = en.commitLocked(c)
		return
	}
	h, ok := en.host.(managedHost)
	if !ok || !en.Capabilities().ManagementSession {
		out.ActionStatus = "management_session_required"
		_ = en.commitLocked(c)
		return
	}
	current, err := h.CredentialSnapshot(en.ctx, run.Config.AuthID, run.Config.ManagedProbe.AuthIndex)
	if err != nil || !sameSnapshot(current, *run.Config.ManagedProbe) {
		out.ActionStatus = "credential_changed"
		_ = en.commitLocked(c)
		return
	}
	if catalogAction && !en.confirmCatalogActionLocked(run, r) {
		out.ActionStatus = "catalog_action_unconfirmed"
		_ = en.commitLocked(c)
		return
	}
	prior := en.managed[current.AuthID]
	restore := false
	if activeDisposition(prior) {
		if prior.State != "disabled" || prior.RuleID != r.ID || !sameSnapshot(current, prior.Snapshot) {
			out.ActionStatus = "credential_action_attention"
			_ = en.commitLocked(c)
			return
		}
		if current.Disabled && condition != "" && !catalogAction {
			if canRecoverDisabled(r, prior) {
				next := *prior
				next.UpstreamCondition = condition
				next.UpdatedAt = en.now()
				c.disposition = &next
				c.audits = append(c.audits, en.audit("error_policy_disabled", r.ID, run.ID, prior.ActionID, "Confirmed upstream condition during an owned recovery probe; credential kept disabled; no CPA write"))
			}
			out.ActionStatus = "credential_kept_disabled"
			_ = en.commitLocked(c)
			return
		}
		if verdict != "correct" || !canRecoverDisabled(r, prior) {
			out.ActionStatus = "credential_kept_disabled"
			_ = en.commitLocked(c)
			return
		}
		restore = true
	} else if current.Disabled {
		if !canRecoverDisabled(r, prior) || verdict != "correct" {
			out.ActionStatus = "credential_kept_disabled"
			_ = en.commitLocked(c)
			return
		}
		restore = true
	} else if condition == "" && (verdict != "incorrect" || !credentialMutation(operation)) {
		out.ActionStatus = "observe_only"
		_ = en.commitLocked(c)
		return
	}
	state := "disable_pending"
	if operation == deleteCredential {
		state = "delete_pending"
	}
	if restore {
		state = "restore_pending"
		operation = "restore_credential"
	}
	d := &CredentialDisposition{AuthID: current.AuthID, RuleID: r.ID, RuleRevision: r.Revision, ActionID: ID(), RunID: run.ID, Operation: operation, State: state, Snapshot: current, Models: append([]string(nil), run.Config.Models...), CreatedAt: en.now(), UpdatedAt: en.now()}
	d.Email = run.CredentialEmail
	if catalogAction {
		d.CatalogMissingModel = run.Config.CatalogMissingModel
	} else {
		d.UpstreamCondition = condition
	}
	if restore && activeDisposition(prior) {
		d.ActionID = prior.ActionID
		d.CreatedAt = prior.CreatedAt
	}
	c.disposition = d
	out.CredentialAction = operation
	out.ActionStatus = state
	if !restore {
		if condition == "" {
			en.stopManagedRules(c, current.AuthID, r.ID, operation == deleteCredential)
		}
		en.cancelManagedRuns(c, current.AuthID, run.ID)
	}
	detail := "Complete management probe round; exact credential snapshot checked before requesting CPA action"
	if condition != "" {
		detail = "Confirmed upstream condition " + condition + "; exact credential snapshot checked before requesting CPA action"
	}
	if catalogAction {
		detail = "CPA credential catalog confirmed the selected model was not registered; no upstream request; exact credential snapshot checked before requesting CPA action"
	}
	if restore && !activeDisposition(prior) {
		detail = "Manually disabled credential passed every sample in the all-credentials rule; requesting enablement"
	}
	c.audits = append(c.audits, en.audit(state, r.ID, run.ID, d.ActionID, detail))
	// The durable intent is written before the API call; pending or uncertain
	// actions are never replayed by restart, timers, GETs, or retries.
	if err = en.commitLocked(c); err != nil {
		return
	}
	// The worker still owns this credential and the pending intent blocks
	// conflicting operations. An unrelated worker need not wait for CPA's write.
	en.mu.Unlock()
	next, err := applyCredentialIntent(en.ctx, h, *d)
	en.mu.Lock()
	result := changes()
	out = cloneRun(en.runs[run.ID])
	result.runs[run.ID] = out
	if err != nil {
		next.State = "attention"
		next.Snapshot = current
		out.ActionStatus = "credential_action_uncertain"
	} else {
		switch next.State {
		case "disabled":
			out.ActionStatus = "credential_suspended"
		case "deleted":
			out.ActionStatus = "credential_removed"
		case "restored":
			out.ActionStatus = "credential_restored"
		}
	}
	next.UpdatedAt = en.now()
	result.disposition = &next
	detail = "CPA action confirmed; ownership and credential snapshot recorded"
	if err != nil {
		detail = "CPA action could not be confirmed; no automatic retry: " + scrubError(err)
	}
	result.audits = append(result.audits, en.audit(out.ActionStatus, r.ID, run.ID, next.ActionID, detail))
	_ = en.commitLocked(result)
}

func applyCredentialIntent(ctx context.Context, h managedHost, d CredentialDisposition) (CredentialDisposition, error) {
	switch d.Operation {
	case deleteCredential:
		err := h.DeleteCredential(ctx, d.Snapshot)
		d.State = "deleted"
		return d, err
	case disableCredential, "restore_credential":
	default:
		return d, fmt.Errorf("invalid credential operation")
	}
	disable := d.Operation == disableCredential
	after, err := h.SetCredentialDisabled(ctx, d.Snapshot, disable)
	d.Snapshot = after
	d.State = "restored"
	if disable {
		d.State = "disabled"
	}
	return d, err
}
