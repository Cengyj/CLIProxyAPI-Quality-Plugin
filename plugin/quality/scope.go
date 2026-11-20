package quality

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"sort"
	"sync"
)

const scopeAll = "all_enabled"
const scopeAllCredentials = "all_credentials"
const maxScopePending = 1024

func dynamicScope(scope string) bool {
	return scope == scopeAll || scope == scopeAllCredentials
}

type ruleDispatch struct {
	SkipReasons map[string]int    `json:"skip_reasons,omitempty"`
	SkipDetails []ScopeSkipDetail `json:"skip_details,omitempty"`
	RunID       string            `json:"run_id,omitempty"`
	BatchID     string            `json:"scope_batch_id,omitempty"`
	Count       int               `json:"count"`
	Skipped     int               `json:"skipped"`
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
		if en.terminalDependencyLocked(rule.RunConfig) {
			return result, nil
		}
		var schedRev uint64
		if scheduled {
			schedRev = revision
		}
		r, err := en.submitLocked(ctx, RunConfig{}, id, schedRev, inventory)
		if err == nil {
			result.RunID = r.ID
			if r.SkipDetail != nil {
				result.BatchID, result.Skipped, result.SkipDetails = r.Config.ScopeBatchID, 1, []ScopeSkipDetail{*r.SkipDetail}
			} else {
				result.Count = 1
			}
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
	ctx = en.inventoryContextLocked(ctx)
	plan := cloneRule(rule)
	en.mu.Unlock()
	targets, err := inventory(ctx)
	var profiles []scopeProfile
	if err == nil && managedRule(plan) && en.Capabilities().ManagementSession {
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
	q, err := freezeDispatchQuestion(&base)
	if err != nil {
		return result, err
	}
	batch, now, ch := ID(), en.now(), changes()
	type match struct {
		target   Target
		config   RunConfig
		snapshot *CredentialSnapshot
		missing  string
	}
	var matches []match
	var manualSkips []string
	seen := map[string]Target{}
	for i, t := range targets {
		if err := ctx.Err(); err != nil {
			return ruleDispatch{}, err
		}
		if _, duplicate := seen[t.AuthID]; duplicate || t.AuthID == "" {
			continue
		}
		seen[t.AuthID] = t
		d := en.managed[t.AuthID]
		if errorPolicyDisabled(d).Code != "" {
			continue
		}
		if (t.ManualDisabled || t.RuntimeStatus == "disabled") && !activeDisposition(d) && !canRecoverDisabled(rule, d) && !t.RetainedPause && !t.QualityPaused && en.localPause(RunConfig{AuthID: t.AuthID}) == "" {
			if rule.TargetScope == scopeAllCredentials {
				manualSkips = append(manualSkips, t.AuthID)
				result.skip(t, ScopeSkipDetail{Code: "manual_disabled", Reason: "凭证由管理员停用，当前规则无权复检。"})
			}
			continue
		}
		cfg := base
		cfg.AuthID, cfg.Provider = t.AuthID, t.Provider
		if t.QualityPaused || t.RetainedPause || en.localPause(cfg) != "" {
			result.skip(t, ScopeSkipDetail{Code: "quality_paused", Reason: "凭证仍受质量暂停保护，禁止检测。"})
			continue
		}
		if t.identityConflict {
			result.skip(t, ScopeSkipDetail{Code: "identity_conflict", Reason: "CPA 返回的凭证或模型目录标识互相冲突。"})
			continue
		}
		if activeDisposition(d) && d.State != "disabled" {
			result.skip(t, ScopeSkipDetail{Code: "handling_blocked", Reason: "上次凭证处置尚待核对。"})
			continue
		}
		if !managedRule(rule) && !ruleSubjectEnabled(t) {
			if t.ManualDisabled || t.RuntimeStatus == "disabled" {
				result.skip(t, recoveryDenied(rule, d))
			} else {
				result.skip(t, runtimeSkip(t))
			}
			continue
		}
		var managed *CredentialSnapshot
		if managedRule(rule) {
			if profiles == nil || !en.Capabilities().ManagementSession {
				result.skip(t, sessionSkip())
				continue
			}
			admissionErr := profiles[i].err
			if owned[t.AuthID] != en.managed[t.AuthID] {
				admissionErr = rejectScope("blocked: credential ownership changed during scan", "ownership_changed", "扫描期间凭证的处置归属已变更。")
			}
			if admissionErr == nil {
				t, managed, admissionErr = en.admitSnapshotLocked(rule, t, profiles[i].snapshot, false)
			}
			if admissionErr != nil {
				result.skipManaged(t, admissionErr)
				continue
			}
		}
		if !ruleSubjectEnabled(t) {
			result.skip(t, runtimeSkip(t))
			continue
		}
		var missing string
		if managed == nil || !managed.Disabled {
			missing = firstMissingModel(t, cfg.Models)
		}
		matches = append(matches, match{t, cfg, managed, missing})
	}
	workers := len(matches)
	if workers > 0 {
		en.compactQueueLocked()
	}
	full := workers > 0 && (len(en.queue) >= cap(en.queue) || pending+workers > maxScopePending)
	var ids []string
	for _, m := range matches {
		if full {
			result.skip(m.target, queueSkip())
			continue
		}
		run := newScopedRun(rule, m.target, m.config, q, batch, scheduled, now, m.snapshot)
		if m.missing != "" {
			queueCatalogRun(run, m.missing)
		}
		ch.runs[run.ID] = run
		ids = append(ids, run.ID)
	}
	matched := len(ids)
	skips := result.SkipDetails[:0]
	for i := range result.SkipDetails {
		detail := result.SkipDetails[i]
		run := newScopedRun(rule, seen[detail.AuthID], base, q, batch, scheduled, now, nil)
		markDispatchSkip(run, &detail, now)
		if run.PreflightDetail != nil {
			matched++
			result.Skipped--
		} else {
			skips = append(skips, detail)
		}
		ch.runs[run.ID] = run
	}
	result.SkipDetails = skips
	next := cloneRule(rule)
	next.LastScopeDispatch = cloneScopeDispatch(&ScopeDispatchSummary{At: now, Matched: matched, Skipped: result.Skipped, Details: result.SkipDetails})
	sort.Strings(manualSkips)
	if len(manualSkips) > 0 {
		if result.SkipReasons == nil {
			result.SkipReasons = map[string]int{}
		}
		result.SkipReasons["manual_disabled"] = len(manualSkips)
	}
	next.LastManualSkips = manualSkips
	next.LastScopeBatch = ""
	if len(ch.runs) != 0 {
		next.LastScopeBatch, result.BatchID = batch, batch
	}
	ch.rules[id] = next
	ch.audits = append(ch.audits, en.audit("scope_dispatched", id, "", "", fmt.Sprintf("匹配 %d 个凭证，跳过 %d 个", matched, result.Skipped)))
	if err := ctx.Err(); err != nil {
		return ruleDispatch{}, err
	}
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
	} else {
		en.noteRoundEndLocked(id)
	}
	if result.Skipped > 0 && rule.MaxResults > 0 {
		if err := en.pruneResultCountsLocked(id, ""); err != nil {
			en.persistFailure(err)
		}
	}
	result.Count = matched
	return result, nil
}

func (en *Engine) checkScopeLocked(rule *Rule, scheduled bool) error {
	if err := en.mutableLocked(); err != nil {
		return err
	}
	if (rule.Action != "observe_only" && !managedRule(rule)) || rule.AutomaticPause || en.Capabilities().QualityPause {
		return unsupportedQuality()
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
			profiles[i].err = rejectScope("blocked: credential unavailable for managed probes", "management_unavailable", "宿主不支持凭证管理探测。")
			continue
		}
		d := owned[t.AuthID]
		if errorPolicyDisabled(d).Code != "" {
			continue
		}
		if !ruleSubjectEnabled(t) && !(t.RuntimeStatus == "disabled" && canRecoverDisabled(rule, d)) {
			detail := runtimeSkip(t)
			if t.RuntimeStatus == "disabled" {
				detail = recoveryDenied(rule, d)
			}
			profiles[i].err = &scopeRejection{"blocked: credential is disabled or unavailable", detail}
			continue
		}
		if activeDisposition(d) && d.State != "disabled" {
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
			// A different worker may finish a credential action while this
			// profile is waiting for its shared read slot.
			en.mu.Lock()
			live := en.managed[t.AuthID]
			changed := live != d || errorPolicyDisabled(live).Code != ""
			en.mu.Unlock()
			if changed {
				profiles[i].err = rejectScope("blocked: credential ownership changed during scan", "ownership_changed", "等待读取期间凭证处置归属已变更。")
				return
			}
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
