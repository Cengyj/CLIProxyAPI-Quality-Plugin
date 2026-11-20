package quality

import (
	"context"
	"errors"
	"fmt"
)

type excludedCredentialIDs struct{}

func (en *Engine) inventoryContextLocked(ctx context.Context) context.Context {
	return en.runInventoryContextLocked(ctx, nil)
}

func (en *Engine) runInventoryContextLocked(ctx context.Context, manualSubject *CredentialDisposition) context.Context {
	ids := make(map[string]bool)
	for id, d := range en.managed {
		if d != manualSubject && errorPolicyDisabled(d).Code != "" {
			ids[id] = true
		}
	}
	return context.WithValue(ctx, excludedCredentialIDs{}, ids)
}

// Only an explicitly selected manual subject may be verified against its old
// error suspension. Scheduled work and dependent credentials stay excluded.
func (en *Engine) runDispositionLocked(c RunConfig, manual bool) (*CredentialDisposition, error) {
	var subject *CredentialDisposition
	for _, id := range dependencies(c) {
		d := en.managed[id]
		if !activeDisposition(d) {
			continue
		}
		if cause := errorPolicyCause(d); cause != "" {
			if manual && id == c.AuthID && d.ActionID != "" && d.Snapshot.Disabled && (d.Operation == "" || d.Operation == disableCredential) {
				subject = d
				continue
			}
			return nil, fmt.Errorf("blocked: %s，凭证已按错误策略停用。", cause)
		}
		if manual && d.State != "disabled" {
			return nil, errors.New("blocked: 凭证存在未解除的处置记录，请在质量运维中核对。")
		}
	}
	return subject, nil
}

func (en *Engine) frozenDispositionLocked(f Frozen) (*CredentialDisposition, error) {
	d, err := en.runDispositionLocked(f.RunConfig, f.RuleID == "")
	if err != nil {
		return nil, err
	}
	if d == nil && f.ManualPolicyActionID != "" || d != nil && d.ActionID != f.ManualPolicyActionID {
		return nil, errors.New("blocked: 凭证的处置记录在测试提交后发生了变化，请重新核对。")
	}
	return d, nil
}

func (en *Engine) verifyManualSubjectLocked(ctx context.Context, c RunConfig, targets []Target, d *CredentialDisposition) (string, error) {
	if d == nil {
		return "", nil
	}
	if en.localPause(c) != "" {
		return "", errors.New("blocked: 凭证仍受质量暂停保护，禁止检测。")
	}
	for _, t := range targets {
		if t.AuthID != c.AuthID {
			continue
		}
		if t.ManualDisabled || t.RuntimeStatus == "disabled" {
			return "", fmt.Errorf("blocked: %s，CPA 当前仍停用此凭证。", errorPolicyCause(d))
		}
		if !t.identityVerified || !t.catalogVerified || !ruleSubjectEnabled(t) || !t.EffectiveAvailable || t.RuntimeStatus != "active" && !(t.IsProvider && t.RuntimeStatus == "registered") ||
			t.Provider != c.Provider || t.AuthID != d.Snapshot.AuthID || t.AuthIndex != d.Snapshot.AuthIndex || t.Provider != d.Snapshot.Provider {
			break
		}
		for _, model := range c.Models {
			if !contains(t.Models, model) {
				return "", errors.New("blocked: CPA 当前凭证没有注册所选模型，请刷新模型目录。")
			}
		}
		if t.IsProvider && d.Snapshot.ProviderMechanism != "" {
			h, ok := en.host.(managedHost)
			if !ok {
				break
			}
			now, err := h.CredentialSnapshot(ctx, t.AuthID, t.AuthIndex)
			if err == nil && !now.Disabled && sameCredential(now, d.Snapshot) && now.ProviderGroupID == d.Snapshot.ProviderGroupID {
				return now.Fingerprint, nil
			}
			break
		}
		if h, ok := en.host.(interface {
			verifyEnabledManualFile(context.Context, Target, *CredentialDisposition) (string, error)
		}); ok {
			return h.verifyEnabledManualFile(ctx, t, d)
		}
		break
	}
	return "", errors.New("blocked: 无法核实 CPA 当前已启用同一凭证及其模型目录，请检查凭证状态。")
}

func (en *Engine) terminalDependencyLocked(c RunConfig) bool {
	for _, id := range dependencies(c) {
		if errorPolicyDisabled(en.managed[id]).Code != "" {
			return true
		}
	}
	return false
}

// The confirmed cause describes the original suspension, independently of CPA's current state.
func errorPolicyCause(d *CredentialDisposition) string {
	if d == nil || d.State != "disabled" {
		return ""
	}
	if d.CatalogMissingModel != "" {
		return "CPA 注册目录缺少所选模型"
	}
	switch d.UpstreamCondition {
	case "unauthorized":
		return "上游返回 HTTP 401 未授权"
	case "payment_required":
		return "上游返回 HTTP 402 需要付款"
	case "quota_exhausted":
		return "上游返回 HTTP 429，确认账户额度耗尽"
	case "model_unsupported":
		return "上游明确不支持所选模型（HTTP 400 或 404）"
	default:
		return ""
	}
}

// A confirmed error suspension remains terminal for rules until explicitly acknowledged.
func errorPolicyDisabled(d *CredentialDisposition) ScopeSkipDetail {
	cause := errorPolicyCause(d)
	if cause == "" {
		return ScopeSkipDetail{}
	}
	return ScopeSkipDetail{Code: "error_policy_disabled", Model: d.CatalogMissingModel,
		Reason: cause + "，凭证已按错误策略停用，后续检测已停止。"}
}

// Persist a source missing from an older confirmed disposition before
// retention can remove the linked run. Configuration alone is not evidence.
func (en *Engine) recoverDisabledSources() error {
	// A quality suspension could previously run recovery again after an
	// explicitly classified error. Choose its first retained, matching probe.
	recovery := make(map[string]*Run)
	for _, run := range en.runs {
		d := en.managed[run.Config.AuthID]
		if d == nil || d.State != "disabled" || d.Operation != "" && d.Operation != disableCredential || !d.Snapshot.Disabled ||
			d.CatalogMissingModel != "" || d.UpstreamCondition != "" || run.Source != "scheduled_probe" ||
			run.Status != "completed" || run.ActionStatus != "credential_kept_disabled" || run.Config.RuleID != d.RuleID ||
			d.CreatedAt.IsZero() || run.CreatedAt.Before(d.CreatedAt) || run.Config.ManagedProbe == nil ||
			!sameSnapshot(*run.Config.ManagedProbe, d.Snapshot) || confirmedRecoveryCondition(run) == "" {
			continue
		}
		prev := recovery[d.AuthID]
		if prev == nil || run.CreatedAt.Before(prev.CreatedAt) || run.CreatedAt.Equal(prev.CreatedAt) && run.ID < prev.ID {
			recovery[d.AuthID] = run
		}
	}
	for _, old := range en.managed {
		if old.State != "disabled" || old.Operation != "" && old.Operation != disableCredential || old.UpstreamCondition != "" || old.CatalogMissingModel != "" {
			continue
		}
		run := en.runs[old.RunID]
		next := *old
		if run != nil && run.ID == old.RunID && run.Source == "scheduled_probe" && run.ActionStatus == "credential_suspended" &&
			run.Config.AuthID == old.AuthID && run.Config.RuleID == old.RuleID && run.Config.RuleRevision == old.RuleRevision &&
			(run.Config.ManagedProbe == nil || sameCredential(*run.Config.ManagedProbe, old.Snapshot)) &&
			(run.CredentialAction == "" || run.CredentialAction == disableCredential) {
			switch {
			case (run.Status == "skipped" || run.Status == "completed") && run.Config.CatalogMissingModel != "" && run.Config.UpstreamErrorActions["model_unsupported"] == disableCredential:
				next.CatalogMissingModel = run.Config.CatalogMissingModel
			case run.Status == "completed" && run.UpstreamCondition != "":
				next.UpstreamCondition = run.UpstreamCondition
			case run.Status == "completed" && len(run.Samples) > 0:
				last := &run.Samples[len(run.Samples)-1]
				if upstreamAction(&run.Config, last) == disableCredential {
					next.UpstreamCondition = last.UpstreamCondition
				}
			}
		}
		if errorPolicyDisabled(&next).Code == "" && recovery[old.AuthID] != nil {
			next.UpstreamCondition = confirmedRecoveryCondition(recovery[old.AuthID])
		}
		if errorPolicyDisabled(&next).Code != "" {
			updates := changes()
			updates.disposition = &next
			if err := en.commitLocked(updates); err != nil {
				return err
			}
		}
	}
	return nil
}

func confirmedRecoveryCondition(run *Run) string {
	if run.UpstreamCondition != "" && credentialMutation(run.Config.UpstreamErrorActions[run.UpstreamCondition]) {
		return run.UpstreamCondition
	}
	if len(run.Samples) > 0 {
		last := &run.Samples[len(run.Samples)-1]
		if upstreamAction(&run.Config, last) != "" {
			return last.UpstreamCondition
		}
	}
	return ""
}
