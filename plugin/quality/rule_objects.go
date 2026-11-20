package quality

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
)

const (
	objectOAuth    = "oauth"
	objectProvider = "provider"
	objectUnknown  = "unknown"
)

func objectType(t Target) string {
	if contains(t.Reasons, "unverified_inventory_entry") || t.ObjectType == objectUnknown {
		return objectUnknown
	}
	if t.IsProvider {
		return objectProvider
	}
	// Older SDK inventories expose a file as its provider (e.g. codex) rather
	// than "oauth". The host adapter rejects unverified non-file origins.
	return objectOAuth
}

func ruleTypes(r *Rule) []string {
	if r.ObjectTypes == nil {
		return []string{objectOAuth}
	}
	return r.ObjectTypes
}

func selectRuleObjects(in *Rule, old *Rule) error {
	if in.ObjectTypes == nil {
		in.ObjectTypes = []string{objectOAuth}
		if old != nil {
			in.ObjectTypes = slices.Clone(ruleTypes(old))
		}
	}
	if len(in.ObjectTypes) == 0 || len(in.ObjectTypes) > 2 {
		return fmt.Errorf("invalid_request: 至少选择一类检测对象")
	}
	kinds := slices.Clone(in.ObjectTypes)
	slices.Sort(kinds)
	for i, kind := range kinds {
		if kind != objectOAuth && kind != objectProvider || i != 0 && kind == kinds[i-1] {
			return fmt.Errorf("invalid_request: 检测对象类型无效或重复")
		}
	}
	in.ObjectTypes = kinds
	in.LegacyScope = old != nil && old.LegacyScope && !in.AdoptScope
	in.LegacyProviderObservation = old != nil && old.LegacyProviderObservation && !in.AdoptScope
	in.AdoptScope = false
	if in.TargetGroup != "" || dynamicScope(in.TargetScope) {
		in.TargetAuthIndex = ""
	} else if in.TargetAuthIndex == "" && old != nil && in.AuthID == old.AuthID && in.Provider == old.Provider {
		in.TargetAuthIndex = old.TargetAuthIndex
	}
	if old != nil && in.AuthorizedGroups == nil {
		in.AuthorizedGroups = slices.Clone(old.AuthorizedGroups)
	}
	if len(in.AuthorizedGroups) > 128 {
		return fmt.Errorf("invalid_request: too many authorized provider groups")
	}
	in.AuthorizedGroups = slices.Clone(in.AuthorizedGroups)
	slices.Sort(in.AuthorizedGroups)
	for i, group := range in.AuthorizedGroups {
		if group == "" || len(group) > 512 || i > 0 && in.AuthorizedGroups[i-1] == group {
			return fmt.Errorf("invalid_request: invalid or duplicate provider group")
		}
	}
	if in.TargetGroup != "" && (in.TargetScope != "single" || in.AuthID != "" || in.Provider != "" || len(in.TargetGroup) > 512) {
		return fmt.Errorf("invalid_request: 指定配置组与单路由绑定不能同时设置")
	}
	if (in.TargetGroup != "" || len(in.AuthorizedGroups) != 0) && !contains(kinds, objectProvider) {
		return fmt.Errorf("invalid_request: 配置组选项需要勾选 AI 提供商")
	}
	if in.LegacyProviderObservation && len(in.AuthorizedGroups) > 0 {
		return fmt.Errorf("invalid_request: 明确采用当前规则范围后才能授权整组启停")
	}
	if in.LegacyScope && (!slices.Equal(kinds, old.ObjectTypes) || in.TargetScope != old.TargetScope || in.AuthID != old.AuthID || in.TargetGroup != old.TargetGroup) {
		return fmt.Errorf("invalid_request: 请确认采用当前规则的检测范围")
	}
	return nil
}

// Capture the last effective legacy permission without using a live catalog at
// startup. File and config targets with the same executor remain distinguishable.
func (en *Engine) migrateRuleScopes(previousProviders bool) error {
	change := changes()
	for _, old := range en.rules {
		if old.ObjectTypes != nil {
			continue
		}
		r := cloneRule(old)
		r.ObjectTypes = []string{objectOAuth}
		if previousProviders {
			r.ObjectTypes = append(r.ObjectTypes, objectProvider)
		}
		r.LegacyScope, r.LegacyProviderObservation = true, true
		change.rules[r.ID] = r
	}
	return en.commitLocked(change)
}

func (en *Engine) previousProviderSetting() (bool, error) {
	var cfg Config
	if err := en.store.Get("meta", "settings", &cfg); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return en.cfg.IncludeProviderTargets, nil
		}
		return false, err
	}
	return cfg.IncludeProviderTargets, nil
}

func listedGroup(targets []Target, id string, wholeGroupWrite bool) bool {
	for _, t := range targets {
		if t.IsProvider && t.ProviderGroupID == id && (!wholeGroupWrite || t.ProviderDisableUnit == "group") && !t.identityConflict && !contains(t.Reasons, "provider_route_unverified") {
			return true
		}
	}
	return false
}

func validateRuleGroups(r, old *Rule, targets []Target) error {
	if r.TargetGroup != "" && (old == nil || old.TargetGroup != r.TargetGroup) && !listedGroup(targets, r.TargetGroup, false) {
		return fmt.Errorf("invalid_request: 无法核实指定的提供商配置组")
	}
	for _, id := range r.AuthorizedGroups {
		if (old == nil || !contains(old.AuthorizedGroups, id)) && !listedGroup(targets, id, true) {
			return fmt.Errorf("invalid_request: 无法核实已授权启停的提供商配置组")
		}
	}
	return nil
}

func disabledObject(t Target) bool { return t.ManualDisabled || t.RuntimeStatus == "disabled" }

func (r *Rule) acceptsSubject(t Target) bool {
	return contains(ruleTypes(r), objectType(t))
}

func (r *Rule) holdsGroup(t Target) bool {
	return t.IsProvider && t.ProviderDisableUnit == "group" && contains(r.AuthorizedGroups, t.ProviderGroupID)
}

func selectedGroup(r *Rule, targets []Target) string {
	if r.TargetGroup != "" {
		return r.TargetGroup
	}
	if r.AuthID == "" {
		return ""
	}
	for _, t := range targets {
		if t.AuthID == r.AuthID && t.Provider == r.Provider && (r.TargetAuthIndex == "" || t.AuthIndex == r.TargetAuthIndex) && r.holdsGroup(t) {
			return t.ProviderGroupID
		}
	}
	return ""
}

func selectedIdentity(r *Rule, t Target, group string) bool {
	if dynamicScope(r.TargetScope) {
		return true
	}
	if group != "" {
		return t.ProviderGroupID == group && t.IsProvider
	}
	return r.AuthID != "" && r.AuthID == t.AuthID
}

func providerRuleProblem(rule *Rule, t Target, models []string, subject bool) *scopeRejection {
	if objectType(t) == objectUnknown {
		return rejectScope("blocked: unverified inventory entry", "unverified_inventory_entry", "CPA 未能核实这项目录记录的对象来源。")
	}
	if !t.IsProvider {
		return nil
	}
	if (subject || rule.LegacyScope) && !contains(ruleTypes(rule), objectProvider) {
		return rejectScope("blocked: provider targets are excluded by this rule", "provider_targets_excluded", "本条规则未选择 AI 提供商。")
	}
	if !subject || rule.LegacyScope {
		return providerRouteProblem(t, models)
	}
	if contains(t.Reasons, "provider_runtime_unconfirmed") {
		return rejectScope("blocked: provider runtime unconfirmed", "provider_runtime_unconfirmed", "CPA 配置与运行时尚未确认同一路由的启停状态。")
	}
	if t.AuthID == "" || t.AuthIndex == "" || t.Provider == "" || t.identityConflict || contains(t.Reasons, "provider_route_unverified") {
		return rejectScope("blocked: unverified provider", "provider_unavailable", "CPA 无法唯一确认该提供商路由及密钥索引。")
	}
	return nil
}

func (en *Engine) checkRuleProvidersLocked(r *Rule, c RunConfig, targets []Target) *scopeRejection {
	deps := dependencies(c)
	for _, t := range targets {
		if !contains(deps, t.AuthID) {
			continue
		}
		subject := t.AuthID == c.AuthID
		if subject && objectType(t) != objectUnknown && !r.acceptsSubject(t) {
			code := "object_type_excluded"
			if t.IsProvider {
				code = "provider_targets_excluded"
			}
			return rejectScope("blocked: target origin excluded by this rule", code, "指定对象的类型未在本规则中勾选。")
		}
		if subject && c.AuthID == r.AuthID && r.TargetAuthIndex != "" && t.AuthIndex != r.TargetAuthIndex {
			return rejectScope("blocked: bound route has changed", "identity_conflict", "已绑定对象的路由索引发生变化。")
		}
		models := c.Models
		if c.Judge.Mode == "llm" && t.AuthID == c.Judge.AuthID {
			models = []string{c.Judge.Model}
		}
		if problem := providerRuleProblem(r, t, models, subject); problem != nil {
			return problem
		}
	}
	return nil
}

func (en *Engine) previewRuleScope(ctx context.Context, draft Rule) (ruleDispatch, error) {
	en.mu.Lock()
	defer en.mu.Unlock()
	old := en.rules[draft.ID]
	if draft.ID != "" && (old == nil || draft.Revision != 0 && old.Revision != draft.Revision) {
		return ruleDispatch{}, fmt.Errorf("conflict: rule changed")
	}
	if err := selectRuleObjects(&draft, old); err != nil {
		return ruleDispatch{}, err
	}
	switch draft.Parameters.ReasoningEffort {
	case "default":
		draft.Parameters.ReasoningEffort = ""
	case "":
		if old == nil {
			draft.Parameters.ReasoningEffort = "high"
		} else {
			draft.Parameters.ReasoningEffort = old.Parameters.ReasoningEffort
		}
	}
	if draft.TargetScope == "" && draft.AuthID == "" {
		draft.TargetScope = scopeAll
	}
	if !dynamicScope(draft.TargetScope) && draft.TargetScope != "" && draft.TargetScope != "single" {
		return ruleDispatch{}, fmt.Errorf("invalid_request: unknown target scope")
	}
	result, _, _, err := en.collectScopeLocked(ctx, &draft, en.host.Targets, true)
	return result, err
}
