package quality

import (
	"context"
	"errors"
	"slices"
)

// New routes in a retained group cannot escape an unresolved group action by
// receiving a new ID. Existing members also carry their own exact snapshot.
func groupGuard(records map[string]*CredentialDisposition, t Target) *CredentialDisposition {
	if t.ProviderGroupID == "" {
		return nil
	}
	for _, d := range records {
		if activeDisposition(d) && d.Snapshot.ProviderMechanism == "group_disabled" && d.Snapshot.ProviderGroupID == t.ProviderGroupID {
			return d
		}
	}
	return nil
}

func providerWriteAllowed(r *Rule, s CredentialSnapshot) bool {
	if r == nil || r.LegacyProviderObservation || !contains(r.ObjectTypes, "provider") {
		return false
	}
	switch s.ProviderMechanism {
	case "excluded_models":
		return true
	case "group_disabled":
		return s.ProviderGroupID != "" && contains(r.AuthorizedGroups, s.ProviderGroupID)
	}
	return false
}

// For ordinary answer decisions the entire group must have received the same
// plan in this dispatch. A skipped or missing route prevents both operations.
func (en *Engine) providerRoundVerdict(run *Run, snap CredentialSnapshot) string {
	if snap.ProviderMechanism != "group_disabled" || len(snap.ProviderMembers) == 1 {
		return managedRoundVerdict(run)
	}
	if run.Config.ScopeBatchID == "" || len(snap.ProviderMembers) == 0 {
		return "unknown"
	}
	found := make(map[string]bool, len(snap.ProviderMembers))
	verdict := "correct"
	for _, child := range en.runs {
		cfg := &child.Config
		if cfg.RuleID != run.Config.RuleID || cfg.RuleRevision != run.Config.RuleRevision || cfg.ScopeBatchID != run.Config.ScopeBatchID || !contains(snap.ProviderMembers, cfg.AuthID) {
			continue
		}
		s := cfg.ManagedProbe
		if found[cfg.AuthID] || s == nil || !sameProviderGroupPlan(*s, snap) || cfg.Samples != run.Config.Samples ||
			!slices.Equal(cfg.Models, run.Config.Models) || cfg.Question != run.Config.Question || child.CancelRequested || child.BlockReason != "" {
			return "unknown"
		}
		found[cfg.AuthID] = true
		switch managedRoundVerdict(child) {
		case "incorrect":
			verdict = "incorrect"
		case "correct":
		default:
			return "unknown"
		}
	}
	if len(found) != len(snap.ProviderMembers) {
		return "unknown"
	}
	return verdict
}

func sameProviderGroupPlan(a, b CredentialSnapshot) bool {
	return a.ProviderGroupID != "" && a.ProviderGroupID == b.ProviderGroupID && a.ProviderMechanism == b.ProviderMechanism &&
		a.Fingerprint != "" && a.Fingerprint == b.Fingerprint && a.Disabled == b.Disabled &&
		slices.Equal(a.ProviderMembers, b.ProviderMembers)
}

// Even an early upstream error requires authority over every affected route.
// Read back exact membership and identities immediately before journaling.
func (en *Engine) providerWriteSnapshots(ctx context.Context, h managedHost, expected CredentialSnapshot, r *Rule) ([]CredentialSnapshot, bool) {
	targets, err := en.host.Targets(ctx)
	if err != nil {
		return nil, false
	}
	ids := []string{expected.AuthID}
	if expected.ProviderMechanism == "group_disabled" {
		ids = expected.ProviderMembers
	}
	if len(ids) == 0 || !contains(ids, expected.AuthID) {
		return nil, false
	}
	current := make([]CredentialSnapshot, 0, len(ids))
	seen := make(map[string]bool, len(ids))
	for _, t := range targets {
		if !contains(ids, t.AuthID) {
			continue
		}
		if seen[t.AuthID] || !t.IsProvider || t.AuthIndex == "" || t.ProviderGroupID != expected.ProviderGroupID ||
			t.identityConflict || t.QualityPaused || t.RetainedPause || en.localPause(RunConfig{AuthID: t.AuthID}) != "" {
			return nil, false
		}
		seen[t.AuthID] = true
		s, err := h.CredentialSnapshot(ctx, t.AuthID, t.AuthIndex)
		if err != nil || s.AuthID != t.AuthID || s.AuthIndex != t.AuthIndex || s.Disabled != t.ManualDisabled || !sameProviderGroupPlan(s, expected) {
			return nil, false
		}
		if s.AuthID == expected.AuthID && !sameSnapshot(s, expected) {
			return nil, false
		}
		if d := en.managed[t.AuthID]; activeDisposition(d) {
			if d.RuleID != r.ID || d.State != "disabled" || errorPolicyDisabled(d).Code != "" || !sameSnapshot(s, d.Snapshot) {
				return nil, false
			}
		} else if groupGuard(en.managed, t) != nil {
			return nil, false
		}
		current = append(current, s)
	}
	return current, len(current) == len(ids)
}

func (en *Engine) decideProviderLocked(run *Run, r *Rule, c *changeSet, verdict, operation, condition string) {
	out := c.runs[run.ID]
	snapshot := *run.Config.ManagedProbe
	finish := func(status string) {
		out.ActionStatus = status
		_ = en.commitLocked(c)
	}
	if !providerWriteAllowed(r, snapshot) || operation == deleteCredential {
		finish("observe_only")
		return
	}
	if condition == "" {
		verdict = en.providerRoundVerdict(run, snapshot)
	}
	prior := en.managed[snapshot.AuthID]
	restore := snapshot.Disabled && condition == "" && verdict == "correct" && canRecoverDisabled(r, prior)
	errorOnOwned := snapshot.Disabled && condition != "" && activeDisposition(prior) && prior.State == "disabled" && prior.RuleID == r.ID
	if !restore && !errorOnOwned && (snapshot.Disabled || operation != disableCredential || condition == "" && verdict != "incorrect") {
		status := "observe_only"
		if snapshot.Disabled {
			status = "credential_kept_disabled"
		} else if verdict == "unknown" {
			status = "insufficient_evidence"
		}
		finish(status)
		return
	}
	h, ok := en.host.(managedHost)
	if !ok || !en.Capabilities().ManagementSession {
		finish("management_session_required")
		return
	}
	members, ok := en.providerWriteSnapshots(en.ctx, h, snapshot, r)
	if !ok {
		finish("credential_changed")
		return
	}
	if run.Config.CatalogMissingModel != "" && !en.confirmCatalogActionLocked(run, r) {
		finish("catalog_action_unconfirmed")
		return
	}
	if errorOnOwned {
		for _, s := range members {
			d := en.managed[s.AuthID]
			if !activeDisposition(d) || d.ActionID != prior.ActionID {
				finish("credential_action_attention")
				return
			}
		}
		for _, s := range members {
			next := *en.managed[s.AuthID]
			next.UpstreamCondition, next.CatalogMissingModel, next.UpdatedAt = condition, run.Config.CatalogMissingModel, en.now()
			next.TriggerAuthID, next.TriggerRunID = run.Config.AuthID, run.ID
			c.dispositions = append(c.dispositions, &next)
		}
		c.audits = append(c.audits, en.audit("error_policy_disabled", r.ID, run.ID, prior.ActionID, "Confirmed upstream condition during provider recovery; disabled state retained"))
		finish("credential_kept_disabled")
		return
	}

	state := "disable_pending"
	if restore {
		state, operation = "restore_pending", "restore_credential"
	}
	now, actionID := en.now(), ID()
	previous := make(map[string]*CredentialDisposition, len(members))
	for _, s := range members {
		previous[s.AuthID] = en.managed[s.AuthID]
		d := &CredentialDisposition{AuthID: s.AuthID, RuleID: r.ID, RuleRevision: r.Revision, ActionID: actionID,
			RunID: run.ID, TriggerAuthID: run.Config.AuthID, TriggerRunID: run.ID, State: state, Operation: operation, Snapshot: s, Models: slices.Clone(r.Models),
			UpstreamCondition: condition, CatalogMissingModel: run.Config.CatalogMissingModel, CreatedAt: now, UpdatedAt: now}
		c.dispositions = append(c.dispositions, d)
		if !restore {
			en.cancelManagedRuns(c, s.AuthID, run.ID)
		}
	}
	out.ActionStatus, out.CredentialAction = state, operation
	c.audits = append(c.audits, en.audit(state, r.ID, run.ID, actionID, "Verified provider configuration, members and rule authority before conditional write"))
	if err := en.commitLocked(c); err != nil {
		return
	}
	if restore && activeDisposition(prior) && sameSnapshot(snapshot, prior.Snapshot) {
		snapshot.ProviderExclusionSource = prior.Snapshot.ProviderExclusionSource
	}
	en.mu.Unlock()
	after, err := h.SetCredentialDisabled(en.ctx, snapshot, !restore)
	verified := map[string]CredentialSnapshot{after.AuthID: after}
	if err == nil {
		for _, member := range members {
			s := after
			if member.AuthID != after.AuthID {
				s, err = h.CredentialSnapshot(en.ctx, member.AuthID, member.AuthIndex)
			}
			if err != nil || !sameCredential(s, member) || s.Disabled == restore || !sameProviderGroupPlan(s, after) {
				err = errors.New("CPA provider state was not confirmed for all affected routes")
				break
			}
			verified[s.AuthID] = s
		}
	}
	en.mu.Lock()
	update := changes()
	out = cloneRun(en.runs[run.ID])
	update.runs[out.ID] = out
	state, out.ActionStatus = "disabled", "credential_suspended"
	if restore {
		state, out.ActionStatus = "restored", "credential_restored"
	}
	if err != nil {
		state, out.ActionStatus = "attention", "credential_action_uncertain"
		if errors.Is(err, errProviderPrecondition) {
			state, out.ActionStatus = "precondition_failed", "provider_config_conflict"
		}
	}
	for _, intent := range c.dispositions {
		next := *intent
		next.State, next.UpdatedAt = state, en.now()
		if err == nil {
			next.Snapshot = verified[next.AuthID]
		} else if state == "precondition_failed" && previous[next.AuthID] != nil {
			next = *previous[next.AuthID]
		}
		update.dispositions = append(update.dispositions, &next)
	}
	detail := "Verified provider configuration and resulting CPA runtime membership"
	if err != nil {
		detail = scrubError(err)
	}
	update.audits = append(update.audits, en.audit(out.ActionStatus, r.ID, run.ID, actionID, detail))
	_ = en.commitLocked(update)
}

func (en *Engine) acknowledgeProviderGroupLocked(rule *Rule, disposition *CredentialDisposition, targets []Target) error {
	members := disposition.Snapshot.ProviderMembers
	for _, t := range targets {
		if t.ProviderGroupID == disposition.Snapshot.ProviderGroupID && (!t.identityVerified || t.ManualDisabled || t.QualityPaused || t.RetainedPause || !ruleSubjectEnabled(t)) {
			return errors.New("blocked: first verify the whole provider group in CPA")
		}
	}
	update := changes()
	for _, id := range members {
		d := en.managed[id]
		_, busy := en.busyAuth[id]
		if d == nil || busy || d.ActionID != disposition.ActionID || d.RuleID != rule.ID || !activeDisposition(d) {
			return errors.New("conflict: provider group ownership changed")
		}
		next := *d
		next.State, next.UpdatedAt = "acknowledged", en.now()
		update.dispositions = append(update.dispositions, &next)
	}
	next := cloneRule(rule)
	next.Enabled, next.SchedulingEnabled = false, false
	next.Revision++
	resetRule(next)
	next.Status = ruleStatus(next)
	update.rules[next.ID] = next
	en.cancelRuleChangesLocked(update, next.ID, "credential_handling_acknowledged")
	update.audits = append(update.audits, en.audit("credential_handling_acknowledged", next.ID, "", disposition.ActionID, "Operator released local handling for the complete provider group; rule stopped"))
	return en.commitLocked(update)
}
