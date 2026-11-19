package login

import "context"

type securityCommitKey struct{}
type factorVerifiedKey struct{}

// The parent owns durable credentials. These synchronous callbacks acknowledge
// local persistence; they never initialize a transport or resume remote work.
func WithSecurityCommit(ctx context.Context, commit func(context.Context) error) context.Context {
	return context.WithValue(ctx, securityCommitKey{}, commit)
}
func securityCommitFromContext(ctx context.Context) func(context.Context) error {
	commit, _ := ctx.Value(securityCommitKey{}).(func(context.Context) error)
	return commit
}
func WithFactorVerified(ctx context.Context, verified func(context.Context, string) error) context.Context {
	return context.WithValue(ctx, factorVerifiedKey{}, verified)
}
func reportFactorVerified(ctx context.Context, id string) error {
	if verified, ok := ctx.Value(factorVerifiedKey{}).(func(context.Context, string) error); ok {
		if err := verified(ctx, id); err != nil {
			return failure("security_storage_failed", "totp", "journal_unavailable")
		}
	}
	return nil
}

func selectTOTPLoginFactor(p payload, bound string) (string, error) {
	session := object(p["oai-client-auth-session"])
	seen := map[string]bool{}
	for _, key := range []string{"mfa_challenge_factors", "mfa_factors"} {
		for _, item := range values(session[key]) {
			factor := object(item)
			id := text(factor["id"])
			if factor["factor_type"] == "totp" && factor["is_recovery"] != true && validFactorID(id) {
				seen[id] = true
			}
		}
	}
	if bound != "" && seen[bound] {
		return bound, nil
	}
	if bound == "" && len(seen) == 1 {
		for id := range seen {
			return id, nil
		}
	}
	return "", failure("interaction_required", "totp", "totp_factor_missing")
}
