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
	return r != nil && (r.Action == disableCredential || r.Action == deleteCredential)
}

type CredentialDisposition struct {
	Email        string             `json:"email,omitempty"`
	AuthID       string             `json:"auth_id"`
	RuleID       string             `json:"rule_id"`
	RuleRevision uint64             `json:"rule_revision"`
	ActionID     string             `json:"action_id"`
	RunID        string             `json:"run_id"`
	Operation    string             `json:"operation"`
	State        string             `json:"state"`
	Snapshot     CredentialSnapshot `json:"snapshot"`
	Models       []string           `json:"models"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
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

type HandlingSummary struct {
	Email    string `json:"email,omitempty"`
	AuthID   string `json:"auth_id"`
	ActionID string `json:"action_id"`
	State    string `json:"state"`
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
	if !r.AutoRestore || r.Action != disableCredential {
		return false
	}
	if activeDisposition(d) {
		return d.State == "disabled" && d.RuleID == r.ID
	}
	return r.TargetScope == scopeAllCredentials
}

// The returned projection is local to the probe. CPA stays disabled throughout.
func (en *Engine) admitManagedLocked(ctx context.Context, r *Rule, t Target) (Target, *CredentialSnapshot, error) {
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
	return en.admitSnapshotLocked(r, t, snapshot)
}

func (en *Engine) admitSnapshotLocked(r *Rule, t Target, snapshot CredentialSnapshot) (Target, *CredentialSnapshot, error) {
	d := en.managed[t.AuthID]
	if activeDisposition(d) {
		// An explicit external enable relinquishes our ownership, but never replays
		// an uncertain write or resurrects a removed credential automatically.
		if d.State == "disabled" && !snapshot.Disabled && sameCredential(snapshot, d.Snapshot) {
			next := *d
			next.State = "externally_enabled"
			next.UpdatedAt = en.now()
			c := changes()
			c.disposition = &next
			c.audits = append(c.audits, en.audit("credential_external_enable", d.RuleID, "", d.ActionID, "CPA enablement observed; previous automatic restore ownership relinquished"))
			if err := en.commitLocked(c); err != nil {
				return t, nil, err
			}
		} else {
			if !canRecoverDisabled(r, d) {
				return t, nil, fmt.Errorf("blocked: credential handling requires attention or recovery is disabled")
			}
			if !sameSnapshot(snapshot, d.Snapshot) {
				return t, nil, fmt.Errorf("blocked: suspended credential was modified; automatic restore stopped")
			}
			if len(d.Models) != len(r.Models) {
				return t, nil, fmt.Errorf("blocked: recovery must test the original complete model selection")
			}
			for _, model := range r.Models {
				if !contains(d.Models, model) {
					return t, nil, fmt.Errorf("blocked: recovery must test the original complete model selection")
				}
			}
		}
	}
	if snapshot.Disabled {
		if !canRecoverDisabled(r, d) || t.RuntimeStatus != "disabled" || !t.ManualDisabled || t.RetainedPause || t.QualityPaused {
			return t, nil, fmt.Errorf("blocked: disabled credential is outside recovery scope")
		}
		for _, reason := range t.Reasons {
			if reason != "operator_disabled" && reason != "manual_disabled" {
				return t, nil, fmt.Errorf("blocked: credential has additional runtime restrictions")
			}
		}
		t.ManualDisabled, t.EffectiveAvailable, t.RuntimeStatus = false, true, "active"
		// CPA unregisters models on disable. A recovery run still has to
		// receive a complete, judged answer for every configured model.
		if len(t.Models) == 0 {
			t.Models = append([]string(nil), r.Models...)
		}
	}
	if !eligibleForScope(t) {
		return t, nil, fmt.Errorf("blocked: credential is disabled or unavailable")
	}
	for _, model := range r.Models {
		if !contains(t.Models, model) {
			return t, nil, fmt.Errorf("blocked: handling requires every selected model on the credential")
		}
	}
	return t, &snapshot, nil
}

func (en *Engine) managedTargetsLocked(ctx context.Context, r *Rule, targets []Target) ([]Target, *CredentialSnapshot, error) {
	if !managedRule(r) {
		return targets, nil, nil
	}
	out := append([]Target(nil), targets...)
	for i, t := range out {
		if t.AuthID == r.AuthID {
			adjusted, snapshot, err := en.admitManagedLocked(ctx, r, t)
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
			adjusted, _, err := en.admitSnapshotLocked(r, t, *expected)
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

func (en *Engine) stopManagedDependents(c *changeSet, authID, owner, exceptRun, reason string, stopOwner bool) {
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
	for id, old := range en.runs {
		if id == exceptRun || !usesCredential(old.Config.RunConfig, authID) || (old.Status != "queued" && old.Status != "running") {
			continue
		}
		r := en.cancelledRunLocked(old, "blocked")
		r.BlockReason = reason
		r.ActionStatus = reason
		c.runs[id] = r
	}
}

func (en *Engine) decideManagedLocked(run *Run, r *Rule, c *changeSet) {
	out := c.runs[run.ID]
	r.FailCount = 0
	r.EvidenceSignature = ""
	c.rules[r.ID] = r
	verdict := managedRoundVerdict(run)
	if verdict == "unknown" {
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
	prior := en.managed[current.AuthID]
	restore := false
	if activeDisposition(prior) {
		if prior.State != "disabled" || prior.RuleID != r.ID || !sameSnapshot(current, prior.Snapshot) {
			out.ActionStatus = "credential_action_attention"
			_ = en.commitLocked(c)
			return
		}
		if verdict != "correct" || !r.AutoRestore || r.Action != disableCredential {
			out.ActionStatus = "credential_suspended"
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
	} else if verdict != "incorrect" {
		out.ActionStatus = "observe_only"
		_ = en.commitLocked(c)
		return
	}
	operation := r.Action
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
	if restore && activeDisposition(prior) {
		d.ActionID = prior.ActionID
		d.CreatedAt = prior.CreatedAt
	}
	c.disposition = d
	out.ActionStatus = state
	if !restore {
		en.stopManagedDependents(c, current.AuthID, r.ID, run.ID, "credential_handling", operation == deleteCredential)
	}
	detail := "Complete management probe round; exact credential snapshot checked before requesting CPA action"
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
