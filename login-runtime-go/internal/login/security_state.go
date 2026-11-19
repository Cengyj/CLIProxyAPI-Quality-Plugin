package login

import (
	"context"
	"github.com/cli-proxy-api/model-quality-login-runtime/internal/transport"
	"net/http"
	"strings"
	"time"
)

// Missing/lagging metadata can justify a single enrollment attempt. A business
// refusal, malformed container, multiple factors or recovery factor cannot.
// This never supplies an ID or authorizes removal from an unknown response.
func totpInitialEnrollmentAllowed(info payload) bool {
	if info == nil || totpResponseRejected(info) || !totpOutcomeFieldsValid(info) {
		return false
	}
	if value, exists := info["mfa_enabled_v2"]; exists {
		if _, ok := value.(bool); !ok {
			return false
		}
	}
	value, exists := info["factors"]
	if !exists || value == nil {
		return true
	}
	factors := object(value)
	if factors == nil {
		return false
	}
	value, exists = factors["totp"]
	if !exists || value == nil {
		return true
	}
	list, ok := value.([]any)
	if !ok || len(list) > 1 {
		return false
	}
	if enabled, ok := info["mfa_enabled_v2"].(bool); ok && !enabled && len(list) != 0 {
		return false
	}
	for _, item := range list {
		factor := object(item)
		if factor == nil {
			return false
		}
		if value, exists := factor["is_recovery"]; exists && value != false {
			return false
		}
		if value, exists := factor["factor_type"]; exists && value != "totp" {
			return false
		}
		if value, exists := factor["id"]; exists {
			if _, ok := value.(string); !ok {
				return false
			}
		}
	}
	return true
}

func validFactorID(id string) bool {
	return id != "" && id == strings.TrimSpace(id) && len(id) <= 512 && !strings.ContainsAny(id, "\r\n\x00")
}

// A matching newly enrolled factor may coexist with unrelated authenticators.
// This parser never grants permission to remove any of those factors.
func activationFactorPresent(info payload, expected string) bool {
	if info == nil || info["mfa_enabled_v2"] != true || totpResponseRejected(info) || !totpOutcomeFieldsValid(info) {
		return false
	}
	list, ok := object(info["factors"])["totp"].([]any)
	if !ok {
		return false
	}
	seen, found := map[string]bool{}, false
	for _, item := range list {
		factor := object(item)
		id, ok := factor["id"].(string)
		recovery, typed := factor["is_recovery"].(bool)
		if !ok || !validFactorID(id) || seen[id] || !typed || factor["factor_type"] != "totp" {
			return false
		}
		seen[id] = true
		if id == expected && !recovery {
			found = true
		}
	}
	return found
}

func (f *flow) readState(ctx context.Context, token, account string) (payload, bool, error) {
	r, body, err := f.securityRequest(ctx, token, account, http.MethodGet, mfaInfoEndpoint, nil)
	retry := r != nil && (r.StatusCode == 500 || r.StatusCode == 502 || r.StatusCode == 503 || r.StatusCode == 504)
	if r == nil && err != nil && ctx.Err() == nil {
		retry = transport.IsRetryableReadError(err)
	}
	if err != nil || r == nil || !r.BodyComplete || r.StatusCode < 200 || r.StatusCode >= 300 || isBrowserChallenge(r) || body == nil {
		return nil, retry, failure("security_state_unconfirmed", f.stage, "upstream_rejected")
	}
	return body, false, nil
}

func (f *flow) pauseStateRead(ctx context.Context, attempt int) error {
	if f.p.pause != nil {
		return f.p.pause(ctx, attempt)
	}
	timer := time.NewTimer(500 * time.Millisecond << attempt)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (f *flow) readInitialState(ctx context.Context, token, account string) (payload, error) {
	for attempt := 0; attempt < 4; attempt++ {
		info, retry, err := f.readState(ctx, token, account)
		state, _ := liveFactor(info)
		if err == nil && (state != "unknown" || !totpInitialEnrollmentAllowed(info) || attempt == 3) {
			return info, nil
		}
		if err != nil && (!retry || attempt == 3) {
			return nil, err
		}
		if err := f.pauseStateRead(ctx, attempt); err != nil {
			return nil, err
		}
	}
	return nil, failure("security_state_unconfirmed", f.stage, "upstream_rejected")
}

func (f *flow) reconcileFactor(ctx context.Context, token, account, wanted, expected string) bool {
	for attempt := 0; attempt < 4 && ctx.Err() == nil; attempt++ {
		info, retry, err := f.readState(ctx, token, account)
		if err != nil && !retry {
			return false
		}
		if err == nil {
			state, actual := liveFactor(info)
			if wanted == "disabled" && state == "disabled" || wanted == "enabled" && activationFactorPresent(info, expected) {
				return true
			}
			if state == "enabled" && actual != f.security.OldFactorID && actual != expected {
				return false
			}
			if state == "unknown" && !totpInitialEnrollmentAllowed(info) {
				return false
			}
		}
		if attempt == 3 || f.pauseStateRead(ctx, attempt) != nil {
			return false
		}
	}
	return false
}

func stateAccountMatches(result map[string]any, account string) bool {
	if id, present := result["account_id"]; present && id != account {
		return false
	}
	value, present := result["account"]
	if !present {
		return true
	}
	obj := object(value)
	if obj == nil {
		return false
	}
	for _, key := range []string{"id", "account_id"} {
		if id, present := obj[key]; present && id != account {
			return false
		}
	}
	return true
}
