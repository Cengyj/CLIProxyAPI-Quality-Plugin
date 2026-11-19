package quality

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"sort"
	"strconv"
	"sync"
)

const scopeAll = "all_enabled"
const scopeAllCredentials = "all_credentials"
const maxScopePending = 1024

func dynamicScope(scope string) bool {
	return scope == scopeAll || scope == scopeAllCredentials
}

type ruleDispatch struct {
	SkipReasons map[string]int `json:"skip_reasons,omitempty"`
	RunID       string         `json:"run_id,omitempty"`
	BatchID     string         `json:"scope_batch_id,omitempty"`
	Count       int            `json:"count"`
	Skipped     int            `json:"skipped"`
}

func eligibleForScope(t Target) bool {
	return t.AuthID != "" && t.AuthIndex != "" && t.Provider != "" && t.RuntimeStatus == "active" && t.EffectiveAvailable && !t.ManualDisabled && !t.QualityPaused && !t.RetainedPause
}

// A batch is one fair queue entry, not an unbounded burst of queue reservations.
// Its durable child runs are pinned to concrete credentials and never replayed on restart.
func (en *Engine) dispatchRule(ctx context.Context, id string, revision uint64, scheduled bool) (ruleDispatch, error) {
	return en.dispatchWithInventory(ctx, id, revision, scheduled, en.host.Targets)
}

func (en *Engine) dispatchWithInventory(ctx context.Context, id string, revision uint64, scheduled bool, inventory func(context.Context) ([]Target, error)) (ruleDispatch, error) {
	en.mu.Lock()
	defer en.mu.Unlock()
	var result ruleDispatch
	if err := en.mutableLocked(); err != nil {
		return result, err
	}
	rule := en.rules[id]
	if rule == nil {
		return result, fmt.Errorf("not_found: rule")
	}
	if (revision != 0 || scheduled || dynamicScope(rule.TargetScope)) && revision != rule.Revision {
		return result, fmt.Errorf("conflict: stale rule revision")
	}
	if !dynamicScope(rule.TargetScope) {
		var schedRev uint64
		if scheduled {
			schedRev = revision
		}
		r, err := en.submitLocked(ctx, RunConfig{}, id, schedRev, inventory)
		if err == nil {
			result.RunID = r.ID
			result.Count = 1
		}
		return result, err
	}
	if err := en.checkScopeLocked(rule, scheduled); err != nil {
		return result, err
	}
	if en.scanning[id] {
		return result, fmt.Errorf("conflict: rule already scanning")
	}
	if en.scanning == nil {
		en.scanning = make(map[string]bool)
	}
	en.scanning[id] = true
	defer delete(en.scanning, id)
	if en.snapshotSlots == nil {
		en.snapshotSlots = make(chan struct{}, en.cfg.Workers)
	}
	owned := maps.Clone(en.managed)
	plan := cloneRule(rule)
	en.mu.Unlock()
	targets, err := inventory(ctx)
	var profiles []scopeProfile
	if err == nil && managedRule(plan) {
		profiles = en.readScopeProfiles(ctx, plan, targets, owned)
	}
	en.mu.Lock()
	if err != nil {
		return result, fmt.Errorf("unavailable: cannot read CPA targets: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	rule = en.rules[id]
	if rule == nil || rule.Revision != revision {
		return result, fmt.Errorf("conflict: stale rule revision")
	}
	if err := en.checkScopeLocked(rule, scheduled); err != nil {
		return result, err
	}
	base := cloneConfig(rule.RunConfig)
	if err := en.validateConfig(&base, true); err != nil {
		return result, err
	}
	if err := en.storageAdmission(); err != nil {
		return result, err
	}
	pending := 0
	for _, r := range en.runs {
		if r.Status != "queued" && r.Status != "running" {
			continue
		}
		if r.Config.ScopeBatchID != "" {
			pending++
		}
	}
	if base.QuestionID == "random-linear" && base.Seed == 0 {
		base.Seed, err = strconv.ParseInt(ID()[:15], 16, 64)
		if err != nil {
			return result, err
		}
	}
	q, err := FreezeQuestion(base)
	if err != nil {
		return result, err
	}
	batch, now, ch := ID(), en.now(), changes()
	ids := []string{}
	var manualSkips []string
	seen := map[string]bool{}
	for i, t := range targets {
		if err := ctx.Err(); err != nil {
			return ruleDispatch{}, err
		}
		if seen[t.AuthID] {
			continue
		}
		seen[t.AuthID] = true
		d := en.managed[t.AuthID]
		if (t.ManualDisabled || t.RuntimeStatus == "disabled") && !activeDisposition(d) && !canRecoverDisabled(rule, d) && !t.RetainedPause && !t.QualityPaused && en.localPause(RunConfig{AuthID: t.AuthID}) == "" {
			if rule.TargetScope == scopeAllCredentials {
				manualSkips = append(manualSkips, t.AuthID)
				result.Skipped++
			}
			continue
		}
		cfg := cloneConfig(base)
		cfg.AuthID, cfg.Provider = t.AuthID, t.Provider
		if t.QualityPaused || t.RetainedPause || en.localPause(cfg) != "" {
			result.Skipped++
			continue
		}
		var managed *CredentialSnapshot
		if managedRule(rule) {
			admissionErr := profiles[i].err
			if owned[t.AuthID] != en.managed[t.AuthID] {
				admissionErr = fmt.Errorf("blocked: credential ownership changed during scan")
			}
			if admissionErr == nil {
				t, managed, admissionErr = en.admitSnapshotLocked(rule, t, profiles[i].snapshot)
			}
			if admissionErr != nil {
				result.Skipped++
				if result.SkipReasons == nil {
					result.SkipReasons = map[string]int{}
				}
				result.SkipReasons[scrubError(admissionErr)]++
				continue
			}
		}
		if !eligibleForScope(t) {
			result.Skipped++
			continue
		}
		cfg.Models = nil
		for _, model := range base.Models {
			if contains(t.Models, model) {
				cfg.Models = append(cfg.Models, model)
			}
		}
		if len(cfg.Models) == 0 {
			result.Skipped++
			continue
		}
		if pending+len(ids) >= maxScopePending {
			return ruleDispatch{}, fmt.Errorf("queue_full: all-enabled pending limit is %d; nothing was queued", maxScopePending)
		}
		f := Frozen{RunConfig: cfg, Question: q, RuleID: id, RuleRevision: rule.Revision, ScopeBatchID: batch, SubjectIndex: t.AuthIndex, Scheduled: scheduled, IdentityVersion: t.IdentityVersion, ComparisonIdentity: t.ComparisonIdentity, PayloadConfigVersion: t.PayloadConfigVersion}
		f.ManagedProbe = managed
		f.Fingerprint = fingerprint(f)
		r := &Run{ID: ID(), CredentialEmail: t.Email, Status: "queued", Source: "scheduled_probe", CreatedAt: now, Config: f, Samples: []Sample{}, ActionStatus: "observe_only"}
		ch.runs[r.ID] = r
		ids = append(ids, r.ID)
	}
	if len(ids) > 0 || rule.TargetScope == scopeAllCredentials {
		next := cloneRule(rule)
		sort.Strings(manualSkips)
		if len(manualSkips) > 0 {
			if result.SkipReasons == nil {
				result.SkipReasons = map[string]int{}
			}
			result.SkipReasons["manual_disabled"] = len(manualSkips)
		}
		next.LastManualSkips = manualSkips
		next.LastScopeBatch = ""
		if len(ids) > 0 {
			next.LastScopeBatch = batch
		}
		ch.rules[id] = next
	}
	if len(ids) > 0 {
		en.compactQueueLocked()
		if len(en.queue) >= cap(en.queue) {
			return ruleDispatch{}, fmt.Errorf("queue_full: retry later; nothing was queued")
		}
	}
	ch.audits = append(ch.audits, en.audit("scope_dispatched", id, "", "", fmt.Sprintf("匹配 %d 个凭证，跳过 %d 个", len(ids), result.Skipped)))
	if err = en.commitLocked(ch); err != nil {
		return ruleDispatch{}, err
	}
	if len(ids) > 0 {
		if en.scopeBatches == nil {
			en.scopeBatches = make(map[string][]string)
		}
		en.scopeBatches[batch] = ids
		if en.roundRemaining == nil {
			en.roundRemaining = make(map[string]int)
		}
		en.roundRemaining[batch] = len(ids)
		en.queue <- batch
		en.notifyWorkersLocked()
		result.BatchID = batch
	} else {
		en.noteRoundEndLocked(id)
	}
	result.Count = len(ids)
	return result, nil
}

func (en *Engine) checkScopeLocked(rule *Rule, scheduled bool) error {
	if err := en.mutableLocked(); err != nil {
		return err
	}
	if (rule.Action != "observe_only" && !managedRule(rule)) || rule.AutomaticPause || en.Capabilities().QualityPause {
		return unsupportedQuality()
	}
	if managedRule(rule) && !en.Capabilities().ManagementSession {
		return fmt.Errorf("management_session_required: reconnect the authenticated plugin panel")
	}
	if !rule.Enabled || ruleStatus(rule) != "detecting" || scheduled && !en.timerAllowed(rule) {
		return fmt.Errorf("blocked: rule or scheduling is stopped")
	}
	for _, run := range en.runs {
		if run.Config.RuleID == rule.ID && (run.Status == "queued" || run.Status == "running" || en.active[run.ID] != nil) {
			return fmt.Errorf("conflict: rule already running")
		}
	}
	return nil
}

type scopeProfile struct {
	snapshot CredentialSnapshot
	err      error
}

// Read independent files outside the engine lock. All scans share the configured
// worker limit; each slot writes only its own indexed result.
func (en *Engine) readScopeProfiles(ctx context.Context, rule *Rule, targets []Target, owned map[string]*CredentialDisposition) []scopeProfile {
	profiles := make([]scopeProfile, len(targets))
	host, supported := en.host.(managedHost)
	var wg sync.WaitGroup
	for i, t := range targets {
		if !supported || t.QualityPaused || t.RetainedPause {
			profiles[i].err = fmt.Errorf("blocked: credential unavailable for managed probes")
			continue
		}
		d := owned[t.AuthID]
		if !eligibleForScope(t) && !(t.RuntimeStatus == "disabled" && canRecoverDisabled(rule, d)) {
			profiles[i].err = fmt.Errorf("blocked: credential is disabled or unavailable")
			continue
		}
		select {
		case en.snapshotSlots <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return profiles
		}
		wg.Go(func() {
			defer func() { <-en.snapshotSlots }()
			profiles[i].snapshot, profiles[i].err = host.CredentialSnapshot(ctx, t.AuthID, t.AuthIndex)
		})
	}
	wg.Wait()
	return profiles
}

// Take at most one runnable member per queue rotation; unavailable siblings do not own workers.
func (en *Engine) rotateBatchLocked(batch string, ids []string, claim bool) *executionTask {
	live := ids[:0]
	var task *executionTask
	for i, id := range ids {
		if r := en.runs[id]; r == nil || r.Status != "queued" {
			continue
		}
		if claim && task == nil {
			task = en.claimRunLocked(id)
			if task != nil {
				// Usually the first child is free. Avoid scanning all remaining siblings per claim.
				if len(live) == 0 {
					live = ids[i+1:]
				} else {
					live = append(live, ids[i+1:]...)
				}
				break
			}
		}
		live = append(live, id)
	}
	if len(live) == 0 {
		delete(en.scopeBatches, batch)
	} else {
		en.scopeBatches[batch] = live
		en.queue <- batch
	}
	return task
}

func (en *Engine) visibleTargets(ctx context.Context) ([]Target, error) {
	targets, err := en.host.Targets(ctx)
	if err != nil {
		return nil, err
	}
	en.mu.Lock()
	defer en.mu.Unlock()
	out := append([]Target(nil), targets...)
	for i := range out {
		out[i].RetainedPause = en.localPause(RunConfig{AuthID: out[i].AuthID}) != ""
		if d := en.managed[out[i].AuthID]; activeDisposition(d) {
			out[i].CredentialHandling = d.State
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AuthID < out[j].AuthID })
	return out, nil
}

func (en *Engine) setScheduling(expected, enabled bool) error {
	en.mu.Lock()
	defer en.mu.Unlock()
	if err := en.mutableLocked(); err != nil {
		return err
	}
	if en.cfg.SchedulingEnabled != expected {
		return fmt.Errorf("conflict: scheduling permission changed")
	}
	saved := en.cfg
	if err := en.store.Get("meta", "settings", &saved); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	saved.SchedulingEnabled = enabled
	if err := en.writeScheduleSettings(saved, !enabled); err != nil {
		return err
	}
	en.cfg.SchedulingEnabled = enabled
	return nil
}
