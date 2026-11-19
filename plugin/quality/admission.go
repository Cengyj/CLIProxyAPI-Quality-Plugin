package quality

import (
	"context"
	"fmt"
	"strconv"

	bolt "go.etcd.io/bbolt"
)

func (en *Engine) validateManual(c *RunConfig) error {
	if len(c.Models) > maxManualModels {
		return fmt.Errorf("models out of bounds")
	}
	return en.validateConfig(c, false)
}

func (en *Engine) validateConfig(c *RunConfig, scoped bool) error {
	if scoped {
		if c.AuthID != "" || c.Provider != "" || c.ControlAuthID != "" || c.ControlProvider != "" || c.Judge.Mode == "llm" || c.Judge.AuthID != "" {
			return fmt.Errorf("invalid_request: all-enabled observation cannot bind a subject, judge or control credential")
		}
	} else if c.AuthID == "" || c.Provider == "" || len(c.AuthID) > 512 || len(c.Provider) > 128 {
		return fmt.Errorf("auth_id and provider are required")
	}
	if len(c.Models) < 1 || len(c.Models) > en.cfg.MaxModels {
		return fmt.Errorf("models out of bounds")
	}
	seen := map[string]bool{}
	for _, m := range c.Models {
		if m == "" || len(m) > 256 || seen[m] {
			return fmt.Errorf("invalid or duplicate model")
		}
		seen[m] = true
	}
	if c.Samples == 0 {
		c.Samples = 1
	}
	if c.Samples < 1 || c.Samples > 8 || c.Samples*len(c.Models) > en.cfg.MaxSamples {
		return fmt.Errorf("samples must be 1..8 and within total budget")
	}
	if c.Parameters.MaxTokens == 0 {
		c.Parameters.MaxTokens = min(1024, en.cfg.MaxOutputTokens)
	}
	if c.Parameters.MaxTokens < 16 || c.Parameters.MaxTokens > en.cfg.MaxOutputTokens {
		return fmt.Errorf("max_tokens out of bounds")
	}
	if c.Parameters.Temperature != nil && (*c.Parameters.Temperature < 0 || *c.Parameters.Temperature > 2) {
		return fmt.Errorf("temperature out of bounds")
	}
	switch c.Parameters.ReasoningEffort {
	case "", "none", "minimal", "low", "medium", "high", "xhigh", "max", "auto":
	default:
		return fmt.Errorf("invalid reasoning_effort")
	}
	if c.Judge.Mode == "" {
		c.Judge.Mode = "deterministic"
	}
	c.Judge.TemplateVersion = "1"
	switch c.Judge.Mode {
	case "deterministic", "manual":
	case "llm":
		if c.Judge.AuthID == "" || c.Judge.AuthID == c.AuthID || c.Judge.Model == "" || c.Judge.Provider == "" {
			return fmt.Errorf("judge requires an independent credential and model")
		}
	default:
		return fmt.Errorf("invalid judge mode")
	}
	if c.ControlAuthID != "" && (c.ControlAuthID == c.AuthID || c.ControlAuthID == c.Judge.AuthID || c.ControlProvider == "") {
		return fmt.Errorf("control requires a distinct credential and provider")
	}
	if c.ControlAuthID != "" && c.Samples*len(c.Models)*2 > en.cfg.MaxSamples {
		return fmt.Errorf("control and subject exceed total sample limit")
	}
	if len(c.IdempotencyKey) > 128 {
		return fmt.Errorf("idempotency key too long")
	}
	return nil
}
func (en *Engine) Submit(ctx context.Context, c RunConfig) (*Run, error) {
	if c.Judge.Mode == "" {
		c.Judge.Mode = "manual"
	}
	return en.submit(ctx, c, "", 0)
}
func (en *Engine) submit(ctx context.Context, c RunConfig, ruleID string, scheduledRevision uint64) (*Run, error) {
	en.mu.Lock()
	defer en.mu.Unlock()
	return en.submitLocked(ctx, c, ruleID, scheduledRevision, en.host.Targets)
}

func (en *Engine) submitLocked(ctx context.Context, c RunConfig, ruleID string, scheduledRevision uint64, inventory func(context.Context) ([]Target, error)) (*Run, error) {
	if en.closed || en.fault != nil {
		return nil, fmt.Errorf("unavailable: plugin is stopping or persistence failed")
	}
	var rule *Rule
	if ruleID != "" {
		rule = en.rules[ruleID]
		if rule == nil {
			return nil, fmt.Errorf("rule not found")
		}
		if en.unsupportedRule(rule) {
			return nil, unsupportedQuality()
		}
		if dynamicScope(rule.TargetScope) {
			return nil, fmt.Errorf("invalid_request: dynamic scope requires batch admission")
		}
		if scheduledRevision != 0 && (rule.Revision != scheduledRevision || !en.timerAllowed(rule)) {
			return nil, fmt.Errorf("blocked: scheduled rule changed or scheduling disabled")
		}
		if !rule.Enabled || ruleStatus(rule) != "detecting" {
			return nil, fmt.Errorf("blocked: rule paused")
		}
		c = rule.RunConfig
		c.IdempotencyKey = ""
		for _, r := range en.runs {
			if r.Config.RuleID == ruleID && (r.Status == "queued" || r.Status == "running" || en.active[r.ID] != nil) {
				return nil, fmt.Errorf("conflict: rule already running")
			}
		}
	}
	c = cloneConfig(c)
	if rule == nil && c.Judge.Mode == "manual" && c.Parameters.MaxTokens == 0 {
		c.Parameters.MaxTokens = min(2048, en.cfg.MaxOutputTokens)
	}
	var validationErr error
	if rule == nil {
		validationErr = en.validateManual(&c)
	} else {
		validationErr = en.validateConfig(&c, false)
	}
	if validationErr != nil {
		return nil, validationErr
	}
	requestHash := fingerprint(c)
	if c.IdempotencyKey != "" {
		var prior struct{ ID, Hash string }
		if e := en.store.Get("idempotency", c.IdempotencyKey, &prior); e == nil {
			if prior.Hash != requestHash {
				return nil, fmt.Errorf("conflict: idempotency key reused with different input")
			}
			if r := en.runs[prior.ID]; r != nil {
				return cloneRun(r), nil
			}
			return nil, fmt.Errorf("gone: idempotent run was removed")
		}
	}
	if err := en.storageAdmission(); err != nil {
		return nil, err
	}
	if len(en.queue) >= cap(en.queue) {
		en.compactQueueLocked()
	}
	if len(en.queue) >= cap(en.queue) {
		return nil, fmt.Errorf("queue_full: retry later")
	}
	targets, e := inventory(ctx)
	if e != nil {
		return nil, fmt.Errorf("host targets unavailable: %w", e)
	}
	if d := en.managed[c.AuthID]; d != nil && d.RuleID == ruleID {
		found := false
		for _, t := range targets {
			if t.AuthID == c.AuthID {
				found = true
				break
			}
		}
		if !found {
			en.noteRoundEndLocked(ruleID)
		}
	}
	var managed *CredentialSnapshot
	targets, managed, e = en.managedTargetsLocked(ctx, rule, targets)
	if e != nil {
		return nil, e
	}
	if paused := pausedTarget(c, targets); paused != "" {
		if err := en.haltCredentialLocked(paused, "", "quality_paused"); err != nil {
			return nil, err
		}
	}
	if err := en.checkTargetsLocked(c, targets); err != nil {
		return nil, err
	}
	var target *Target
	for i := range targets {
		if targets[i].AuthID == c.AuthID {
			target = &targets[i]
			break
		}
	}
	if target == nil {
		return nil, fmt.Errorf("blocked: credential missing")
	}
	if target.Provider != c.Provider {
		return nil, fmt.Errorf("provider mismatch")
	}
	if target.ManualDisabled {
		return nil, fmt.Errorf("blocked: operator disabled")
	}
	for _, m := range c.Models {
		if !contains(target.Models, m) {
			return nil, fmt.Errorf("model not available for target: %s", m)
		}
	}
	var judge *Target
	if c.Judge.Mode == "llm" {
		for i := range targets {
			if targets[i].AuthID == c.Judge.AuthID {
				judge = &targets[i]
				break
			}
		}
		if judge == nil || judge.Provider != c.Judge.Provider || !judge.EffectiveAvailable || !contains(judge.Models, c.Judge.Model) {
			return nil, fmt.Errorf("blocked: judge target unavailable")
		}
		if target.ComparisonIdentity == "" || judge.ComparisonIdentity == "" || target.ComparisonIdentity == judge.ComparisonIdentity {
			return nil, fmt.Errorf("blocked: independent judge identity cannot be established")
		}
	}
	var control *Target
	if c.ControlAuthID != "" {
		for i := range targets {
			if targets[i].AuthID == c.ControlAuthID {
				control = &targets[i]
				break
			}
		}
		if control == nil || control.Provider != c.ControlProvider || !control.EffectiveAvailable {
			return nil, fmt.Errorf("blocked: control target unavailable")
		}
		if target.ComparisonIdentity == "" || control.ComparisonIdentity == "" || target.ComparisonIdentity == control.ComparisonIdentity {
			return nil, fmt.Errorf("blocked: independent control identity cannot be established")
		}
		for _, m := range c.Models {
			if !contains(control.Models, m) {
				return nil, fmt.Errorf("control model unavailable")
			}
		}
	}
	if c.QuestionID == "random-linear" && c.Seed == 0 {
		seed, err := strconv.ParseInt(ID()[:15], 16, 64)
		if err != nil {
			return nil, err
		}
		c.Seed = seed
	}
	q, e := FreezeQuestion(c)
	if e != nil {
		return nil, e
	}
	now := en.now()
	r := &Run{ID: ID(), Status: "queued", Source: "manual_test", CreatedAt: now, ActionStatus: "not_applicable", Samples: []Sample{}, Config: Frozen{RunConfig: c, Question: q, IdentityVersion: target.IdentityVersion, PayloadConfigVersion: target.PayloadConfigVersion}}
	if rule != nil {
		r.Source = "scheduled_probe"
		r.Config.RuleID = rule.ID
		r.Config.RuleRevision = rule.Revision
		r.Config.Scheduled = scheduledRevision != 0
		r.ActionStatus = "observe_only"
		r.Config.ManagedProbe = managed
	}
	r.CredentialEmail = target.Email
	if judge != nil {
		r.Config.JudgeIdentity = judge.IdentityVersion
	}
	r.Config.ComparisonIdentity = target.ComparisonIdentity
	if control != nil {
		r.Config.ControlIdentity = control.IdentityVersion
		r.Config.BatchID = ID()
	}
	r.Config.Fingerprint = fingerprint(r.Config)
	if e = en.store.Write(func(tx *bolt.Tx) error {
		if e := put(tx, "runs", r.ID, r); e != nil {
			return e
		}
		if c.IdempotencyKey != "" {
			return put(tx, "idempotency", c.IdempotencyKey, struct{ ID, Hash string }{r.ID, requestHash})
		}
		return nil
	}); e != nil {
		return nil, e
	}
	en.runs[r.ID] = r
	en.queue <- r.ID
	en.notifyWorkersLocked()
	return cloneRun(r), nil
}
func contains(a []string, s string) bool {
	for _, v := range a {
		if v == s {
			return true
		}
	}
	return false
}
