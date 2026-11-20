package quality

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
)

type scopeMatch struct {
	target         Target
	config         RunConfig
	snapshot       *CredentialSnapshot
	missing        string
	recovery       bool
	judge, control *Target
}

var errScopeDirectoryUnavailable = errors.New("unavailable: cannot read CPA targets")

func needsScopeSnapshot(rule *Rule, t Target) bool {
	if disabledObject(t) {
		return true
	}
	if t.IsProvider {
		if rule.LegacyProviderObservation || !managedRule(rule) {
			return false
		}
		// Neither a selected channel nor the directory scope authorizes group writes.
		if t.ProviderDisableUnit == "group" && !rule.holdsGroup(t) {
			return false
		}
		if rule.Action == disableCredential {
			return true
		}
		for _, action := range rule.UpstreamErrorActions {
			if action == disableCredential {
				return true
			}
		}
		return false
	}
	return managedRule(rule)
}

func recoveryMember(rule *Rule, t Target, d *CredentialDisposition) bool {
	return disabledObject(t) && activeDisposition(d) && d.State == "disabled" && d.RuleID == rule.ID &&
		errorPolicyDisabled(d).Code == "" && rule.AutoRestore && rule.Action == disableCredential
}

func scopeMember(rule *Rule, t Target, d *CredentialDisposition, group string) bool {
	if !selectedIdentity(rule, t, group) || errorPolicyDisabled(d).Code != "" {
		return false
	}
	if !rule.acceptsSubject(t) && objectType(t) != objectUnknown && dynamicScope(rule.TargetScope) {
		return false
	}
	if rule.TargetScope == scopeAll && disabledObject(t) {
		return recoveryMember(rule, t, d)
	}
	return true
}

func (en *Engine) collectScopeLocked(ctx context.Context, draft *Rule, inventory func(context.Context) ([]Target, error), preview bool) (ruleDispatch, []scopeMatch, map[string]Target, error) {
	r := cloneRule(draft)
	original := en.rules[r.ID]
	owned := maps.Clone(en.managed)
	ctx = en.inventoryContextLocked(ctx)
	coverage := &providerInventoryStatus{Verified: true}
	ctx = context.WithValue(ctx, providerInventoryStatusKey{}, coverage)
	if en.snapshotSlots == nil {
		en.snapshotSlots = make(chan struct{}, max(1, en.cfg.Workers))
	}
	en.mu.Unlock()
	targets, err := inventory(ctx)
	if err == nil && !coverage.Verified {
		// The SDK still knows file credentials when the separate provider
		// config directory is down. Its runtime provider rows are partial.
		targets = slices.Clone(targets)
		for i := range targets {
			if targets[i].IsProvider {
				t := &targets[i]
				t.Reasons = append(slices.Clone(t.Reasons), "provider_route_unverified")
				t.ProviderProbeReason = "CPA 提供商配置目录未读到，无法核实路由。"
			}
		}
	}
	var profiles []scopeProfile
	if err == nil {
		profiles = en.readScopeProfiles(ctx, r, targets, owned)
	}
	en.mu.Lock()
	result := ruleDispatch{Candidates: map[string]int{objectOAuth: 0, objectProvider: 0}, Verified: err == nil}
	if ctx.Err() != nil {
		return result, nil, nil, ctx.Err()
	}
	if original != nil && (en.rules[r.ID] != original || original.Revision != r.Revision) {
		return result, nil, nil, fmt.Errorf("conflict: stale rule revision")
	}
	if stateErr := en.mutableLocked(); stateErr != nil {
		return result, nil, nil, stateErr
	}
	if err != nil {
		result.Reason = "CPA 对象目录尚未读到，范围待核实。"
		if preview {
			return result, nil, nil, nil
		}
		return result, nil, nil, fmt.Errorf("%w: %w", errScopeDirectoryUnavailable, err)
	}
	if !preview && !r.LegacyScope && !dynamicScope(r.TargetScope) && en.Capabilities().QualityPause {
		return result, nil, nil, unsupportedQuality()
	}
	seen := make(map[string]Target, len(targets))
	for _, t := range targets {
		if t.AuthID != "" {
			seen[t.AuthID] = t
		}
	}
	if !coverage.Verified && scopeNeedsProviderDirectory(r, seen) {
		result.Verified = false
		result.Reason = "CPA 提供商配置目录未读到，AI 提供商范围待核实。"
	}
	group := selectedGroup(r, targets)
	var matches []scopeMatch
	var encounters int
	if !dynamicScope(r.TargetScope) && en.terminalDependencyLocked(r.RunConfig) {
		result.Reason = "指定对象或其依赖已按上游错误策略暂停，退出后续运维检测。"
		return result, nil, seen, nil
	}
	if r.LegacyScope && !dynamicScope(r.TargetScope) && !contains(r.ObjectTypes, objectProvider) {
		for _, id := range dependencies(r.RunConfig) {
			if seen[id].IsProvider {
				result.Reason = "旧规则中的供应商绑定未获原工作台授权。"
				result.skip(seen[id], ScopeSkipDetail{Code: "provider_targets_excluded", Reason: result.Reason, NotExecuted: true})
				if !preview {
					return result, nil, seen, fmt.Errorf("blocked: legacy provider binding was not enabled")
				}
				return result, nil, seen, nil
			}
		}
	}
	for i, t := range targets {
		if ctx.Err() != nil {
			return result, nil, nil, ctx.Err()
		}
		d := en.managed[t.AuthID]
		held := groupGuard(en.managed, t)
		if held != nil && errorPolicyDisabled(held).Code != "" {
			continue
		}
		if !scopeMember(r, t, d, group) {
			continue
		}
		encounters++
		kind := objectType(t)
		result.Candidates[kind]++
		if held != nil && !activeDisposition(d) {
			result.skip(t, ScopeSkipDetail{Code: "group_members_changed", Reason: "本组已有处置归属，当前路由的身份与停用快照无法对应。"})
			continue
		}
		cfg := r.RunConfig
		cfg.AuthID, cfg.Provider = t.AuthID, t.Provider
		m := scopeMatch{target: t, config: cfg, recovery: r.TargetScope == scopeAll && recoveryMember(r, t, d)}
		detail := en.scopeAdmissionLocked(r, &m, t, seen, profiles[i], owned[t.AuthID], preview)
		if detail != nil {
			if disabledObject(t) {
				detail.NotExecuted = true
			}
			result.skip(t, *detail)
			continue
		}
		if m.snapshot == nil || !m.snapshot.Disabled {
			m.missing = firstMissingModel(t, cfg.Models)
		}
		matches = append(matches, m)
		if m.recovery {
			result.RecoveryCount++
		}
	}
	if encounters == 0 && dynamicScope(r.TargetScope) && result.Verified {
		result.Reason = "当前目录中没有可纳入本轮的对象。下次执行时会重新匹配。"
	}
	if encounters == 0 && !dynamicScope(r.TargetScope) {
		missing := Target{AuthID: r.AuthID, AuthIndex: r.TargetAuthIndex, Provider: r.Provider, ProviderGroupID: r.TargetGroup}
		detail := missingCredentialSkip()
		if !result.Verified {
			detail = ScopeSkipDetail{Code: "provider_directory_unavailable", NotExecuted: true, Reason: result.Reason}
		}
		if r.TargetGroup != "" {
			missing.AuthID = r.TargetGroup
			if result.Verified {
				detail = ScopeSkipDetail{Code: "group_missing", Reason: "指定的提供商配置组不在当前可核实目录中。"}
			}
		}
		for _, t := range seen {
			if usesCredential(r.RunConfig, t.AuthID) && (t.QualityPaused || t.RetainedPause) {
				detail = ScopeSkipDetail{Code: "quality_paused", Reason: "规则依赖的对象仍受独立质量暂停保护。"}
			}
		}
		// A terminal group or subject is retired, with no repeated records.
		excluded := false
		for _, t := range targets {
			if selectedIdentity(r, t, group) && (errorPolicyDisabled(en.managed[t.AuthID]).Code != "" || errorPolicyDisabled(groupGuard(en.managed, t)).Code != "") {
				excluded = true
			}
		}
		for _, d := range en.managed {
			if r.TargetGroup != "" && d.Snapshot.ProviderGroupID == r.TargetGroup && errorPolicyDisabled(d).Code != "" {
				excluded = true
			}
		}
		if excluded {
			result.Reason = "指定的提供商组已按上游错误策略暂停，退出后续运维检测。"
		} else {
			seen[missing.AuthID] = missing
			result.skip(missing, detail)
		}
	}
	result.Count = len(matches)
	if preview {
		result.classifyPreflight()
	}
	return result, matches, seen, nil
}

func scopeNeedsProviderDirectory(rule *Rule, targets map[string]Target) bool {
	if rule.TargetGroup != "" {
		return true
	}
	if dynamicScope(rule.TargetScope) {
		return contains(ruleTypes(rule), objectProvider)
	}
	for _, id := range dependencies(rule.RunConfig) {
		if t, ok := targets[id]; ok {
			if t.IsProvider {
				return true
			}
			continue
		}
		if id == rule.AuthID && contains(ruleTypes(rule), objectProvider) && (len(ruleTypes(rule)) == 1 || providerFamily(id, rule.Provider) != "") {
			return true
		}
	}
	return false
}

func (receipt *ruleDispatch) classifyPreflight() {
	skips := receipt.SkipDetails[:0]
	for _, detail := range receipt.SkipDetails {
		if !detail.NotExecuted && preflightStatus(detail.Code) != "" {
			receipt.PreflightDetails = append(receipt.PreflightDetails, detail)
			receipt.Count++
			receipt.Skipped--
		} else {
			skips = append(skips, detail)
		}
	}
	receipt.SkipDetails = skips
}

func (en *Engine) scopeAdmissionLocked(r *Rule, m *scopeMatch, t Target, targets map[string]Target, p scopeProfile, old *CredentialDisposition, preview bool) *ScopeSkipDetail {
	d := en.managed[t.AuthID]
	problem := func(code, reason string) *ScopeSkipDetail { return &ScopeSkipDetail{Code: code, Reason: reason} }
	if t.QualityPaused || t.RetainedPause || en.localPause(m.config) != "" {
		return problem("quality_paused", "对象仍受独立质量暂停保护，禁止检测。")
	}
	if t.identityConflict {
		return problem("identity_conflict", "CPA 返回的对象或模型目录标识互相冲突。")
	}
	if !r.acceptsSubject(t) {
		if objectType(t) == objectUnknown {
			return problem("unverified_inventory_entry", "CPA 未能核实这项目录记录的对象来源。")
		}
		return problem("object_type_excluded", "指定对象的类型未在本规则中勾选。")
	}
	if r.AuthID == t.AuthID && r.Provider != t.Provider {
		return problem("provider_mismatch", "当前对象提供方与规则保存的绑定不一致。")
	}
	if r.AuthID == t.AuthID && r.TargetAuthIndex != "" && r.TargetAuthIndex != t.AuthIndex {
		return problem("identity_conflict", "指定对象的提供方或路由索引已经变化。")
	}
	if e := providerRuleProblem(r, t, r.Models, true); e != nil {
		return &e.detail
	}
	if activeDisposition(d) && (d.State != "disabled" || d.RuleID != r.ID && disabledObject(t)) {
		if d.State == "disabled" && d.RuleID != r.ID {
			return problem("other_rule_owner", "该对象由其他规则持有停用。")
		}
		return problem("handling_blocked", "对象的上次处置尚待核对。")
	}
	if d != old {
		return problem("ownership_changed", "目录核对期间处置归属已变化。")
	}
	if disabledObject(t) && !canRecoverDisabled(r, d) && (r.LegacyScope || r.TargetScope != scopeAllCredentials || activeDisposition(d)) {
		if !activeDisposition(d) {
			return problem("manual_disabled", "对象由管理员停用，本规则范围不允许探测该对象。")
		}
		detail := recoveryDenied(r, d)
		return &detail
	}
	if needsScopeSnapshot(r, t) {
		// A verified missing model on an enabled object is a catalog check,
		// independent of support for sending the requested upstream payload.
		missing := firstMissingModel(t, r.Models)
		if t.IsProvider && disabledObject(t) && missing != "" {
			return &ScopeSkipDetail{Code: "provider_probe_unavailable", Model: missing, Reason: "停用路由的配置映射中没有所选模型，无法定向探测。"}
		}
		if reason := providerPlanProbeReason(t, r.Models, r.Parameters); reason != "" && (disabledObject(t) || missing == "") {
			return problem("provider_probe_unavailable", reason)
		}
		if p.err != nil {
			var e *scopeRejection
			if errors.As(p.err, &e) {
				return cloneValue(&e.detail)
			}
			return problem("snapshot_unavailable", "无法从 CPA 核对本次定向探测需要的快照。")
		}
		projected, snapshot, err := en.admitSnapshotLocked(r, t, p.snapshot, preview)
		if err != nil {
			var e *scopeRejection
			if errors.As(err, &e) {
				return cloneValue(&e.detail)
			}
			return problem("snapshot_unavailable", "对象快照无法核实。")
		}
		t, m.snapshot = projected, snapshot
	}
	if !ruleSubjectEnabled(t) {
		detail := runtimeSkip(t)
		return &detail
	}
	for _, id := range dependencies(m.config) {
		if id == t.AuthID {
			continue
		}
		dep, ok := targets[id]
		if !ok {
			return problem("dependency_unavailable", "规则指定的判题或对照对象不在当前目录中。")
		}
		if dep.QualityPaused || dep.RetainedPause {
			return problem("quality_paused", "规则所需凭证仍受质量暂停保护，禁止检测。")
		}
		if activeDisposition(en.managed[id]) || groupGuard(en.managed, dep) != nil || !dep.EffectiveAvailable || dep.ManualDisabled {
			return problem("dependency_unavailable", "规则指定的判题或对照对象不可用。")
		}
		models := m.config.Models
		desired := m.config.ControlProvider
		if id == m.config.Judge.AuthID {
			models = []string{m.config.Judge.Model}
			desired = m.config.Judge.Provider
		}
		if dep.Provider != desired {
			return problem("dependency_unavailable", "判题或对照对象的提供方已变化。")
		}
		if e := providerRuleProblem(r, dep, models, false); e != nil {
			return &e.detail
		}
		if dep.ComparisonIdentity == "" || dep.ComparisonIdentity == t.ComparisonIdentity || t.ComparisonIdentity == "" {
			return problem("dependency_identity", "无法确认判题或对照对象独立于被测对象。")
		}
		for _, model := range models {
			if !contains(dep.Models, model) {
				return problem("dependency_unavailable", "判题或对照对象未注册所选模型。")
			}
		}
		if id == m.config.Judge.AuthID {
			m.judge = &dep
		} else {
			m.control = &dep
		}
	}
	return nil
}
