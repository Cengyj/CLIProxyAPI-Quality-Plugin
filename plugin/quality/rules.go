package quality

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	bolt "go.etcd.io/bbolt"
)

func (en *Engine) Rules() []Rule {
	en.mu.Lock()
	defer en.mu.Unlock()
	items := []Rule{}
	for _, r := range en.rules {
		next := cloneRule(r)
		if en.unsupportedRule(next) {
			next.Enabled = false
			next.Status = "unsupported"
			next.NextRun = time.Time{}
		}
		items = append(items, *next)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}
func (en *Engine) SaveRule(ctx context.Context, in Rule, update bool) (*Rule, error) {
	en.mu.Lock()
	defer en.mu.Unlock()
	if en.closed || en.fault != nil {
		return nil, fmt.Errorf("unavailable")
	}
	if en.unsupportedRule(&in) {
		return nil, unsupportedQuality()
	}
	if e := en.validate(&in.RunConfig); e != nil {
		return nil, e
	}
	if _, e := FreezeQuestion(in.RunConfig); e != nil {
		return nil, e
	}
	if in.Action == "" {
		in.Action = "observe_only"
	}
	if in.Action != "observe_only" && in.Action != "pause_credential" {
		return nil, fmt.Errorf("unsupported action")
	}
	if len(in.Name) > 128 {
		return nil, fmt.Errorf("name too long")
	}
	if in.FailThreshold == 0 {
		in.FailThreshold = 3
	}
	if in.FailThreshold < 1 || in.FailThreshold > 100 {
		return nil, fmt.Errorf("threshold must be 1..100")
	}
	if in.Timezone == "" {
		in.Timezone = "Asia/Shanghai"
	}
	if _, e := time.LoadLocation(in.Timezone); e != nil {
		return nil, fmt.Errorf("invalid timezone")
	}
	if in.IntervalSeconds == 0 {
		in.IntervalSeconds = 3600
	}
	if in.IntervalSeconds < 60 || in.IntervalSeconds > 31*86400 {
		return nil, fmt.Errorf("interval must be 60..2678400 seconds")
	}
	if in.Cron != "" {
		if len(strings.Fields(in.Cron)) != 5 {
			return nil, fmt.Errorf("cron requires five fields")
		}
		if _, e := cron.ParseStandard(in.Cron); e != nil {
			return nil, fmt.Errorf("invalid cron")
		}
	}
	if in.RetentionDays == 0 {
		in.RetentionDays = en.cfg.RetentionDays
	}
	if in.RetentionDays < 1 || in.RetentionDays > 365 {
		return nil, fmt.Errorf("retention must be 1..365")
	}
	if err := en.storageAdmission(); err != nil {
		return nil, err
	}
	old := en.rules[in.ID]
	if update {
		if old == nil {
			return nil, fmt.Errorf("not_found: rule")
		}
		if !en.Capabilities().QualityPause && (old.ActionID != "" || old.BlockedAuthID != "") {
			return nil, unsupportedQuality()
		}
		if in.Revision != old.Revision {
			return nil, fmt.Errorf("conflict: stale rule revision")
		}
		if old.ActionID != "" && in.AuthID != old.AuthID {
			return nil, fmt.Errorf("conflict: release owned pause before changing target")
		}
		in.Revision++
		in.BlockedAuthID = old.BlockedAuthID
		in.IntentRevision = old.IntentRevision
		in.ActionID = old.ActionID
		in.ActionRevision = old.ActionRevision
		in.ActionRuleRevision = old.ActionRuleRevision
		in.IdentityVersion = old.IdentityVersion
	} else {
		if in.ID != "" {
			return nil, fmt.Errorf("new rule ID must be empty")
		}
		in.ID = ID()
		in.BlockedAuthID = ""
		in.IntentRevision = 0
		in.Revision = 1
		in.ActionID = ""
		in.ActionRevision = 0
		in.ActionRuleRevision = 0
		in.IdentityVersion = ""
	}
	in.EvidenceSignature = ""
	in.FailCount = 0
	in.LastRun = ""
	in.IdempotencyKey = ""
	in.NextRun = nextRun(in, en.now())
	for _, r := range en.rules {
		if r.ID != in.ID && r.AuthID == in.AuthID && r.Action == "pause_credential" && in.Action == "pause_credential" {
			return nil, fmt.Errorf("conflict: one credential permits one pause rule")
		}
	}
	targets, err := en.host.Targets(ctx)
	if err != nil {
		return nil, err
	}
	if in.Action == "pause_credential" {
		supported := false
		for _, t := range targets {
			if t.AuthID == in.AuthID && t.CanPause {
				supported = true
			}
		}
		if !supported {
			return nil, fmt.Errorf("unsupported: target cannot be paused precisely")
		}
	}
	if in.Enabled {
		if in.ActionID != "" {
			return nil, fmt.Errorf("blocked: owned pause requires manual release")
		}
		if err = en.checkTargetsLocked(in.RunConfig, targets); err != nil {
			return nil, err
		}
		in.BlockedAuthID = ""
	} else if blocked := pausedTarget(in.RunConfig, targets); blocked != "" {
		in.BlockedAuthID = blocked
	}
	in.Status = ruleStatus(&in)
	in.NextRun = nextRun(in, en.now())
	c := changes()
	c.rules[in.ID] = cloneRule(&in)
	kind := "rule_created"
	if update {
		kind = "rule_updated"
		en.cancelRuleChangesLocked(c, in.ID, "rule_changed")
	}
	c.audits = append(c.audits, en.audit(kind, in.ID, "", "", fmt.Sprintf("revision=%d", in.Revision)))
	if err = en.commitLocked(c); err != nil {
		return nil, err
	}
	return cloneRule(&in), nil
}

func (en *Engine) PauseRule(id string, rev uint64, paused bool) error {
	en.mu.Lock()
	defer en.mu.Unlock()
	if err := en.mutableLocked(); err != nil {
		return err
	}
	r := en.rules[id]
	if r == nil {
		return fmt.Errorf("not_found: rule")
	}
	if r.Revision != rev {
		return fmt.Errorf("conflict: stale rule revision")
	}
	next := cloneRule(r)
	next.Enabled = !paused
	next.Revision++
	resetRule(next)
	if !paused {
		if en.unsupportedRule(r) {
			return unsupportedQuality()
		}
		if r.ActionID != "" {
			return fmt.Errorf("blocked: owned pause requires manual release")
		}
		targets, err := en.host.Targets(en.ctx)
		if err != nil {
			return err
		}
		if err = en.checkTargetsLocked(r.RunConfig, targets); err != nil {
			return err
		}
		next.BlockedAuthID = ""
	}
	next.Status = ruleStatus(next)
	next.NextRun = nextRun(*next, en.now())
	c := changes()
	c.rules[id] = next
	en.cancelRuleChangesLocked(c, id, "rule_changed")
	c.audits = append(c.audits, en.audit("rule_schedule_state", id, "", r.ActionID, fmt.Sprintf("paused=%t revision=%d", paused, next.Revision)))
	return en.commitLocked(c)
}
func (en *Engine) DeleteRule(ctx context.Context, id string, rev uint64) error {
	en.mu.Lock()
	defer en.mu.Unlock()
	if err := en.mutableLocked(); err != nil {
		return err
	}
	r := en.rules[id]
	if r == nil {
		return fmt.Errorf("not_found: rule")
	}
	if r.Revision != rev {
		return fmt.Errorf("conflict: stale rule revision")
	}
	if en.Capabilities().QualityPause {
		actions, e := en.host.Actions(ctx)
		if e != nil {
			return e
		}
		for _, a := range actions {
			if a.RuleID == id {
				return fmt.Errorf("conflict: release owned pause before deleting rule")
			}
		}
	} else if r.ActionID != "" || r.BlockedAuthID != "" {
		return unsupportedQuality()
	}
	if r.ActionID != "" {
		return fmt.Errorf("conflict: reconcile pending action before deleting rule")
	}
	for _, run := range en.runs {
		if run.Config.RuleID == id && (run.Status == "queued" || run.Status == "running") {
			return fmt.Errorf("conflict: cancel active rule run first")
		}
	}
	audit := en.audit("rule_deleted", id, "", "", "rule removed without owned pause")
	if err := en.store.Write(func(tx *bolt.Tx) error {
		if err := tx.Bucket([]byte("rules")).Delete([]byte(id)); err != nil {
			return err
		}
		return put(tx, "audit", audit.ID, audit)
	}); err != nil {
		en.persistFailure(err)
		return err
	}
	delete(en.rules, id)
	return nil
}
func (en *Engine) RunRule(ctx context.Context, id string) (*Run, error) {
	return en.submit(ctx, RunConfig{}, id, 0)
}

// RoundVerdict requires every planned sample and explicit verdicts for every model.
func RoundVerdict(r *Run) string {
	if r.Status != "completed" || len(r.Samples) != len(r.Config.Models)*r.Config.Samples {
		return "unknown"
	}
	failedModels := 0
	allPass := true
	for _, m := range r.Config.Models {
		seen := map[int]bool{}
		failed := false
		for _, s := range r.Samples {
			if s.Model != m {
				continue
			}
			if seen[s.Index] || s.Index < 0 || s.Index >= r.Config.Samples || s.ExecutionStatus != "completed" || !s.Comparable || (s.QualityVerdict != "correct" && s.QualityVerdict != "incorrect") {
				return "unknown"
			}
			seen[s.Index] = true
			if s.QualityVerdict == "incorrect" {
				failed = true
			}
		}
		if len(seen) != r.Config.Samples {
			return "unknown"
		}
		if failed {
			failedModels++
			allPass = false
		}
	}
	if failedModels == len(r.Config.Models) {
		return "incorrect"
	}
	if allPass {
		return "correct"
	}
	return "mixed"
}
func (en *Engine) decideLocked(run *Run) {
	if run.Source != "scheduled_probe" || run.Config.RuleID == "" || en.closed || en.fault != nil {
		return
	}
	r := cloneRule(en.rules[run.Config.RuleID])
	out := cloneRun(run)
	c := changes()
	c.runs[run.ID] = out
	if r == nil || !r.Enabled || r.Revision != run.Config.RuleRevision || ruleStatus(r) != "detecting" {
		out.ActionStatus = "stale_rule"
		if r != nil && ruleStatus(r) == "awaiting_release" {
			out.ActionStatus = "awaiting_release"
		}
		_ = en.commitLocked(c)
		return
	}
	r.LastRun = run.ID
	if !en.Capabilities().VerifiedEvidence {
		r.FailCount = 0
		r.EvidenceSignature = ""
		out.ActionStatus = "observe_only"
		c.rules[r.ID] = r
		_ = en.commitLocked(c)
		return
	}
	verdict := RoundVerdict(run)
	var targets []Target
	if verdict != "unknown" {
		var err error
		targets, err = en.host.Targets(en.ctx)
		if err == nil {
			if paused := pausedTarget(run.Config.RunConfig, targets); paused != "" {
				c.rules[r.ID] = r
				en.haltChangesLocked(c, paused, run.ID, "quality_paused")
				out.ActionStatus = "awaiting_release"
				_ = en.commitLocked(c)
				return
			}
			err = en.checkTargetsLocked(run.Config.RunConfig, targets)
		}
		if err == nil {
			err = frozenIdentityError(run.Config, targets)
		}
		if err != nil {
			verdict = "unknown"
			out.ActionStatus = "evidence_unavailable"
		}
	}
	signature := roundSignature(run)
	if r.EvidenceSignature != signature {
		r.FailCount = 0
		r.EvidenceSignature = signature
	}
	if run.Config.Question.Kind == "random" && !en.hasControlLocked(run, targets) {
		verdict = "unknown"
		out.ActionStatus = "missing_valid_control"
	}
	if verdict == "incorrect" {
		r.FailCount++
	} else {
		r.FailCount = 0
	}
	c.rules[r.ID] = r
	if r.Action != "pause_credential" || verdict != "incorrect" || r.FailCount < r.FailThreshold || !r.AutomaticPause || !en.cfg.AutomaticPause {
		_ = en.commitLocked(c)
		return
	}
	a := Action{AuthID: r.AuthID, OwnerPlugin: "model-quality", RuleID: r.ID, RuleRevision: r.Revision, ActionID: ID(), ActionRevision: 1, EvidenceRunID: run.ID, IdentityVersion: run.Config.IdentityVersion, Reason: "all selected models explicitly failed complete comparable rounds; manual release required", CreatedAt: en.now()}
	r.ActionID = a.ActionID
	r.ActionRevision = a.ActionRevision
	r.ActionRuleRevision = a.RuleRevision
	r.IdentityVersion = a.IdentityVersion
	en.haltChangesLocked(c, r.AuthID, run.ID, "quality_paused")
	r = c.rules[r.ID]
	r.IntentRevision = r.Revision
	out.ActionStatus = "pending_reconciliation"
	c.intent = &a
	c.audits = append(c.audits, en.audit("pause_requested", r.ID, run.ID, a.ActionID, "all dependent rules and tasks stopped before host action"))
	if err := en.commitLocked(c); err != nil {
		return
	}
	result, err := en.host.Pause(en.ctx, a)
	update := changes()
	out = cloneRun(en.runs[run.ID])
	update.runs[run.ID] = out
	if err != nil {
		out.ActionStatus = "pending_reconciliation"
		update.audits = append(update.audits, en.audit("pause_uncertain", r.ID, run.ID, a.ActionID, scrubError(err)))
	} else {
		held := cloneRule(en.rules[r.ID])
		held.ActionRevision = result.ActionRevision
		update.rules[r.ID] = held
		out.ActionStatus = "paused"
		update.audits = append(update.audits, en.audit("credential_paused", r.ID, run.ID, a.ActionID, "credential paused; all evaluation stopped; manual release required"))
	}
	_ = en.commitLocked(update)
}
func (en *Engine) hasControlLocked(r *Run, targets []Target) bool {
	if r.Config.BatchID == "" || r.Config.ControlAuthID == "" || r.CompletedAt == nil || en.now().Sub(*r.CompletedAt) > 5*time.Minute || r.CompletedAt.Sub(r.CreatedAt) > 5*time.Minute {
		return false
	}
	var subject, peer *Target
	for i := range targets {
		if targets[i].AuthID == r.Config.AuthID {
			subject = &targets[i]
		}
		if targets[i].AuthID == r.Config.ControlAuthID {
			peer = &targets[i]
		}
	}
	if subject == nil || peer == nil || subject.IdentityVersion != r.Config.IdentityVersion || peer.IdentityVersion != r.Config.ControlIdentity || subject.ComparisonIdentity == "" || peer.ComparisonIdentity == "" || subject.ComparisonIdentity == peer.ComparisonIdentity {
		return false
	}
	control := *r
	control.Samples = r.Controls
	if RoundVerdict(&control) != "correct" || len(r.Controls) != len(r.Samples) {
		return false
	}
	for i, s := range r.Samples {
		o := r.Controls[i]
		if s.Model != o.Model || s.Index != o.Index || s.Evidence.Path != o.Evidence.Path || s.Evidence.ResolvedModel != o.Evidence.ResolvedModel || s.Evidence.PayloadFingerprint != o.Evidence.PayloadFingerprint {
			return false
		}
	}
	return true
}

func (en *Engine) auditLocked(kind, ruleID, runID, actionID, detail string) {
	a := Audit{ID: ID(), Time: en.now(), Kind: kind, RuleID: ruleID, RunID: runID, ActionID: actionID, Detail: detail}
	en.persistFailure(en.store.Put("audit", a.ID, a))
}
func (en *Engine) Actions(ctx context.Context) ([]Action, error) {
	if en.Capabilities().QualityPause {
		return en.host.Actions(ctx)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	en.mu.Lock()
	defer en.mu.Unlock()
	retained := []Action{}
	for _, rule := range en.rules {
		if rule.ActionID == "" {
			continue
		}
		action := Action{AuthID: rule.AuthID, OwnerPlugin: "model-quality", RuleID: rule.ID, RuleRevision: rule.ActionRuleRevision, ActionID: rule.ActionID, ActionRevision: rule.ActionRevision, IdentityVersion: rule.IdentityVersion}
		var intent Action
		if err := en.store.Get("meta", "intent:"+rule.ActionID, &intent); err == nil {
			action = intent
		}
		action.Status = "retained_unverified"
		retained = append(retained, action)
	}
	sort.Slice(retained, func(i, j int) bool { return retained[i].ActionID < retained[j].ActionID })
	return retained, nil
}
func (en *Engine) Preview(ctx context.Context, ruleID, actionID string, revision uint64) (map[string]any, error) {
	if !en.Capabilities().ManualRelease {
		return nil, unsupportedQuality()
	}
	actions, e := en.host.Actions(ctx)
	if e != nil {
		return nil, e
	}
	for _, a := range actions {
		if a.ActionID == actionID && a.RuleID == ruleID {
			if a.ActionRevision != revision {
				return nil, fmt.Errorf("conflict: stale action revision")
			}
			targets, e := en.host.Targets(ctx)
			if e != nil {
				return nil, e
			}
			return map[string]any{"action": a, "targets": targets, "scope": "entire credential; all models", "manual_release_only": true, "rules_remain_paused": true, "effect": "release only this owned quality pause; all dependent rules remain paused until explicitly resumed; other restrictions remain"}, nil
		}
	}
	return nil, fmt.Errorf("not_found: owned action")
}
func (en *Engine) Release(ctx context.Context, ruleID, actionID string, revision uint64, _ bool) error {
	en.mu.Lock()
	defer en.mu.Unlock()
	if err := en.mutableLocked(); err != nil {
		return err
	}
	if !en.Capabilities().ManualRelease {
		return unsupportedQuality()
	}
	r := en.rules[ruleID]
	if r == nil {
		return fmt.Errorf("not_found: rule")
	}
	actions, err := en.host.Actions(ctx)
	if err != nil {
		return err
	}
	var action *Action
	for i := range actions {
		if actions[i].ActionID == actionID && actions[i].RuleID == ruleID && actions[i].AuthID == r.AuthID && actions[i].OwnerPlugin == "model-quality" {
			action = &actions[i]
			break
		}
	}
	if action == nil || action.ActionRevision != revision || !matchesActionBinding(r, action) {
		return fmt.Errorf("conflict: action missing or revision changed")
	}
	c := changes()
	en.haltChangesLocked(c, action.AuthID, "", "manual_release")
	for _, rule := range c.rules {
		rule.Revision++
		resetRule(rule)
	}
	c.audits = append(c.audits, en.audit("manual_release_requested", ruleID, "", actionID, "all dependent rules and tasks stopped before release"))
	if err = en.commitLocked(c); err != nil {
		return err
	}
	if err = en.host.Release(ctx, *action); err != nil {
		en.auditLocked("manual_release_blocked", ruleID, "", actionID, scrubError(err))
		return err
	}
	remaining := map[string]bool{}
	targets, targetsErr := en.host.Targets(ctx)
	active, actionsErr := en.host.Actions(ctx)
	for _, t := range targets {
		if t.QualityPaused {
			remaining[t.AuthID] = true
		}
	}
	for _, a := range active {
		remaining[a.AuthID] = true
	}
	for id, rule := range en.rules {
		if id != ruleID && rule.ActionID != "" {
			remaining[rule.AuthID] = true
		}
	}
	next := cloneRule(en.rules[ruleID])
	next.ActionID = ""
	next.ActionRevision = 0
	next.ActionRuleRevision = 0
	next.IntentRevision = 0
	update := changes()
	update.rules[ruleID] = next
	for id, rule := range en.rules {
		if usesCredential(rule.RunConfig, action.AuthID) {
			n := cloneRule(rule)
			if id == ruleID {
				n = next
			}
			if targetsErr == nil && actionsErr == nil {
				n.BlockedAuthID = ""
				for _, authID := range dependencies(n.RunConfig) {
					if remaining[authID] {
						n.BlockedAuthID = authID
						break
					}
				}
			}
			n.Enabled = false
			resetRule(n)
			n.Status = ruleStatus(n)
			update.rules[id] = n
		}
	}
	update.audits = append(update.audits, en.audit("manual_release", ruleID, "", actionID, "owned restriction released; dependent rules remain paused; explicit resume required"))
	if err = en.commitLocked(update); err != nil {
		return err
	}
	if targetsErr != nil || actionsErr != nil {
		return fmt.Errorf("unavailable: owned pause released; remaining restrictions await reconciliation")
	}
	return nil
}

// roundSignature prevents consecutive evidence from crossing identity or effective-condition boundaries.
func roundSignature(r *Run) string {
	conditions := make([]any, 0, len(r.Samples))
	question := r.Config.Question.Prompt
	random := r.Config.Question.Kind == "random"
	if random {
		question = r.Config.Question.Family
	}
	for _, s := range r.Samples {
		payload := s.Evidence.PayloadFingerprint
		if random {
			payload = "verified-within-controlled-round"
		}
		conditions = append(conditions, []any{s.Model, s.Index, s.Evidence.ResolvedModel, payload, s.Evidence.Path})
	}
	return fingerprint([]any{r.Config.IdentityVersion, r.Config.JudgeIdentity, r.Config.ControlIdentity, r.Config.PayloadConfigVersion, r.Config.Question.Version, question, r.Config.Parameters, r.Config.Judge, conditions})
}
