package login

import (
	"errors"
	"strings"

	auth "github.com/cliproxyapi-plugins/model-quality/login/account"
)

func normalizeExcludedPlans(values []string) ([]string, error) {
	if len(values) > 32 {
		return nil, errors.New("invalid_request")
	}
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		plan, valid := auth.ParsePlan(value)
		if !valid || plan == "unknown" && strings.ToLower(strings.TrimSpace(value)) != "unknown" {
			return nil, errors.New("invalid_request")
		}
		if !seen[plan] {
			seen[plan] = true
			out = append(out, plan)
		}
	}
	return out, nil
}

func cloneAccountSettings(settings AccountSettings) AccountSettings {
	settings.CPAUploadExcludedPlans = append([]string{}, settings.CPAUploadExcludedPlans...)
	return settings
}

// Account plans are authorization snapshots. Reading an existing CPA binding
// uses public local callbacks only and never initiates a subscription request.
func (s *Service) accountPlanType(a storedAccount) string {
	plan, valid := auth.ParsePlan(a.View.PlanType)
	if a.View.AuthID != "" {
		current, ok := s.opts.Manager.GetByID(a.View.AuthID)
		principal, err := auth.LoginPrincipalFromAuth(current)
		if !ok || err != nil || principal.Key() != a.Principal || s.journal.fingerprint("account-email", principal.Email) != a.EmailHash {
			return "unknown"
		}
		if currentPlan := auth.LoginPlanType(current); currentPlan != "unknown" {
			return currentPlan
		}
	}
	if valid {
		return plan
	}
	return "unknown"
}

// The immutable atomic settings snapshot avoids accountMu/mu lock inversion.
// Known local, candidate and current bound plans each veto an excluded upload;
// unknown is considered only when none of those sources contains a known plan.
func (s *Service) checkUploadPlans(candidate *auth.Auth, expected auth.LoginSnapshot, prior []string) error {
	configured, _ := s.uploadExcludedPlans.Load().([]string)
	if len(configured) == 0 {
		return nil
	}
	plans := append(append([]string{}, prior...), auth.LoginPlanTypes(candidate)...)
	if expected.AuthID != "" {
		current, ok := s.opts.Manager.GetByID(expected.AuthID)
		principal, err := auth.LoginPrincipalFromAuth(current)
		if !ok || err != nil || principal.Key() != expected.Principal {
			return errors.New("policy_changed")
		}
		plans = append(plans, auth.LoginPlanTypes(current)...)
	}
	known := map[string]bool{}
	for _, value := range plans {
		if plan, valid := auth.ParsePlan(value); valid && plan != "unknown" {
			known[plan] = true
		}
	}
	if len(known) == 0 {
		known["unknown"] = true
	}
	for _, excluded := range configured {
		if known[excluded] {
			return errors.New("cpa_plan_excluded")
		}
	}
	return nil
}
