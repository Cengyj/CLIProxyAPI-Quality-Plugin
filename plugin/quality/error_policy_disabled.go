package quality

import "context"

type excludedCredentialIDs struct{}

func (en *Engine) inventoryContextLocked(ctx context.Context) context.Context {
	ids := make(map[string]bool)
	for id, d := range en.managed {
		if errorPolicyDisabled(d).Code != "" {
			ids[id] = true
		}
	}
	return context.WithValue(ctx, excludedCredentialIDs{}, ids)
}

func (en *Engine) terminalDependencyLocked(c RunConfig) bool {
	for _, id := range dependencies(c) {
		if errorPolicyDisabled(en.managed[id]).Code != "" {
			return true
		}
	}
	return false
}

// A confirmed error suspension remains terminal until explicitly acknowledged.
func errorPolicyDisabled(d *CredentialDisposition) ScopeSkipDetail {
	if d == nil || d.State != "disabled" {
		return ScopeSkipDetail{}
	}
	var cause string
	if d.CatalogMissingModel != "" {
		cause = "CPA 注册目录缺少所选模型"
	} else {
		switch d.UpstreamCondition {
		case "unauthorized":
			cause = "上游返回 HTTP 401 未授权"
		case "payment_required":
			cause = "上游返回 HTTP 402 需要付款"
		case "quota_exhausted":
			cause = "上游返回 HTTP 429，确认账户额度耗尽"
		case "model_unsupported":
			cause = "上游明确不支持所选模型（HTTP 400 或 404）"
		default:
			return ScopeSkipDetail{}
		}
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
