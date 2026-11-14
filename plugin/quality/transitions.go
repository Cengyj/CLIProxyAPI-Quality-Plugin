package quality

import (
	"context"
	"fmt"
	"sort"
	"time"

	log "github.com/sirupsen/logrus"
	bolt "go.etcd.io/bbolt"
)

// changeSet publishes a state transition only after all of its records are durable.
type changeSet struct {
	rules  map[string]*Rule
	runs   map[string]*Run
	audits []Audit
	intent *Action
}

func changes() *changeSet { return &changeSet{rules: map[string]*Rule{}, runs: map[string]*Run{}} }
func (en *Engine) commitLocked(c *changeSet) error {
	if len(c.rules) == 0 && len(c.runs) == 0 && len(c.audits) == 0 && c.intent == nil {
		return nil
	}
	err := en.store.Write(func(tx *bolt.Tx) error {
		for id, r := range c.rules {
			if err := put(tx, "rules", id, r); err != nil {
				return err
			}
		}
		for id, r := range c.runs {
			if err := put(tx, "runs", id, r); err != nil {
				return err
			}
		}
		for _, a := range c.audits {
			if err := put(tx, "audit", a.ID, a); err != nil {
				return err
			}
		}
		if c.intent != nil {
			return put(tx, "meta", "intent:"+c.intent.ActionID, c.intent)
		}
		return nil
	})
	if err != nil {
		en.fault = fmt.Errorf("persist quality transition: %w", err)
		for _, cancel := range en.active {
			cancel()
		}
		return en.fault
	}
	for id, r := range c.rules {
		if old := en.rules[id]; old != nil {
			*old = *r
		} else {
			en.rules[id] = r
		}
	}
	for id, r := range c.runs {
		if old := en.runs[id]; old != nil {
			*old = *r
		} else {
			en.runs[id] = r
		}
		if r.CancelRequested {
			if cancel := en.active[id]; cancel != nil {
				cancel()
			}
		}
	}
	if len(c.runs) != 0 {
		en.notifyWorkersLocked()
	}
	return nil
}

func (en *Engine) mutableLocked() error {
	if en.closed || en.fault != nil {
		return fmt.Errorf("unavailable: plugin is stopping or persistence failed")
	}
	return nil
}
func (en *Engine) audit(kind, ruleID, runID, actionID, detail string) Audit {
	return Audit{ID: ID(), Time: en.now(), Kind: kind, RuleID: ruleID, RunID: runID, ActionID: actionID, Detail: detail}
}
func ruleStatus(r *Rule) string {
	if r.ActionID != "" || r.BlockedAuthID != "" {
		return "awaiting_release"
	}
	if r.Enabled {
		return "detecting"
	}
	return "paused"
}
func resetRule(r *Rule) { r.FailCount = 0; r.EvidenceSignature = ""; r.NextRun = time.Time{} }
func dependencies(c RunConfig) []string {
	ids := []string{c.AuthID}
	if c.ControlAuthID != "" {
		ids = append(ids, c.ControlAuthID)
	}
	if c.Judge.Mode == "llm" && c.Judge.AuthID != "" {
		ids = append(ids, c.Judge.AuthID)
	}
	sort.Strings(ids)
	unique := ids[:0]
	for _, id := range ids {
		if id != "" && (len(unique) == 0 || unique[len(unique)-1] != id) {
			unique = append(unique, id)
		}
	}
	return unique
}
func usesCredential(c RunConfig, id string) bool { return contains(dependencies(c), id) }
func pausedTarget(c RunConfig, targets []Target) string {
	for _, id := range dependencies(c) {
		for _, t := range targets {
			if t.AuthID == id && t.QualityPaused {
				return id
			}
		}
	}
	return ""
}
func (en *Engine) localPause(c RunConfig) string {
	for _, r := range en.rules {
		if r.ActionID != "" && usesCredential(c, r.AuthID) {
			return r.AuthID
		}
	}
	return ""
}
func (en *Engine) checkTargetsLocked(c RunConfig, targets []Target) error {
	if en.fault != nil {
		return fmt.Errorf("unavailable: persistent state requires operator attention")
	}
	if id := en.localPause(c); id != "" {
		return fmt.Errorf("blocked: credential quality pause awaits manual release")
	}
	for _, id := range dependencies(c) {
		var found *Target
		for i := range targets {
			if targets[i].AuthID == id {
				found = &targets[i]
				break
			}
		}
		if found == nil {
			return fmt.Errorf("blocked: credential missing")
		}
		if found.QualityPaused {
			return fmt.Errorf("blocked: credential quality pause awaits manual release")
		}
		if found.ManualDisabled || !found.EffectiveAvailable {
			return fmt.Errorf("blocked: credential unavailable")
		}
	}
	return nil
}
func (en *Engine) haltChangesLocked(c *changeSet, authID, exceptRun, reason string) {
	for id, old := range en.rules {
		if !usesCredential(old.RunConfig, authID) {
			continue
		}
		r := c.rules[id]
		if r == nil {
			r = cloneRule(old)
		}
		if !r.Enabled && r.NextRun.IsZero() && r.BlockedAuthID != "" && r.Status == "awaiting_release" && r.FailCount == 0 && r.EvidenceSignature == "" {
			continue
		}
		r.Revision++
		r.Enabled = false
		if r.BlockedAuthID == "" {
			r.BlockedAuthID = authID
		}
		resetRule(r)
		r.Status = "awaiting_release"
		c.rules[id] = r
	}
	for id, old := range en.runs {
		if id == exceptRun || !usesCredential(old.Config.RunConfig, authID) || (old.Status != "queued" && old.Status != "running") {
			continue
		}
		if old.CancelRequested && old.BlockReason == reason {
			continue
		}
		r := cloneRun(old)
		r.CancelRequested = true
		r.BlockReason = reason
		r.ActionStatus = "awaiting_release"
		if r.Status == "queued" {
			r.Status = "blocked"
			now := en.now()
			r.CompletedAt = &now
		}
		c.runs[id] = r
	}
}
func (en *Engine) haltCredentialLocked(authID, exceptRun, reason string) error {
	c := changes()
	en.haltChangesLocked(c, authID, exceptRun, reason)
	return en.commitLocked(c)
}
func (en *Engine) cancelRuleChangesLocked(c *changeSet, id, reason string) {
	for runID, old := range en.runs {
		if old.Config.RuleID != id || (old.Status != "queued" && old.Status != "running") {
			continue
		}
		r := cloneRun(old)
		r.CancelRequested = true
		r.BlockReason = reason
		if r.Status == "queued" {
			r.Status = "blocked"
			now := en.now()
			r.CompletedAt = &now
		}
		c.runs[runID] = r
	}
}

// checkExecution repeats admission after queue waits and before each subject, control, or judge call.
func (en *Engine) checkExecution(ctx context.Context, runID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	en.mu.Lock()
	defer en.mu.Unlock()
	r := en.runs[runID]
	if en.closed || en.fault != nil {
		return fmt.Errorf("unavailable: engine stopped or persistence failed")
	}
	if r == nil || r.Status != "running" || r.CancelRequested {
		return fmt.Errorf("blocked: task is no longer admitted")
	}
	if id := r.Config.RuleID; id != "" {
		rule := en.rules[id]
		if rule == nil || !rule.Enabled || rule.Revision != r.Config.RuleRevision || ruleStatus(rule) != "detecting" {
			return fmt.Errorf("blocked: stale or stopped rule")
		}
	}
	targets, err := en.host.Targets(ctx)
	if err != nil {
		return fmt.Errorf("unavailable: credential status cannot be verified")
	}
	if paused := pausedTarget(r.Config.RunConfig, targets); paused != "" {
		if err = en.haltCredentialLocked(paused, "", "quality_paused"); err != nil {
			return err
		}
	}
	if err = en.checkTargetsLocked(r.Config.RunConfig, targets); err != nil {
		return err
	}
	if err = frozenIdentityError(r.Config, targets); err != nil {
		return err
	}
	return ctx.Err()
}

func frozenIdentityError(f Frozen, targets []Target) error {
	for _, t := range targets {
		if t.AuthID == f.AuthID && t.IdentityVersion != f.IdentityVersion {
			return fmt.Errorf("blocked: subject identity changed")
		}
		if f.Judge.Mode == "llm" && t.AuthID == f.Judge.AuthID && t.IdentityVersion != f.JudgeIdentity {
			return fmt.Errorf("blocked: judge identity changed")
		}
		if t.AuthID == f.ControlAuthID && t.IdentityVersion != f.ControlIdentity {
			return fmt.Errorf("blocked: control identity changed")
		}
	}
	return nil
}
func (en *Engine) persistFailure(err error) {
	if err != nil {
		en.fault = err
		log.WithError(err).Error("model-quality: persistence failed; further execution blocked")
		for _, cancel := range en.active {
			cancel()
		}
	}
}
