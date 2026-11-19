package account

import (
	"strings"

	"github.com/MoeForge/moego/base64"
	"github.com/MoeForge/moego/ojson"
)

// ParsePlan normalizes supported subscription labels without turning invalid
// configuration into an exclusion of accounts whose plan is unknown.
func ParsePlan(value string) (string, bool) {
	value = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), "chatgpt_")
	switch value {
	case "business", "self_serve_business":
		value = "team"
	case "pro-lite", "pro_lite":
		value = "prolite"
	case "self_serve_business_prolite":
		value = "business_premium"
	case "education":
		value = "edu"
	}
	switch value {
	case "free", "go", "plus", "pro", "prolite", "business_premium", "team", "enterprise", "edu", "unknown":
		return value, true
	}
	return "unknown", false
}

// LoginPlanTypes returns all known authorization snapshots. These are metadata,
// not a live subscription check. Missing or unrecognized values remain unknown.
func LoginPlanTypes(a *Auth) []string {
	known := []string{}
	seen := map[string]bool{}
	add := func(value string) {
		plan, valid := ParsePlan(value)
		if valid && plan != "unknown" && !seen[plan] {
			seen[plan] = true
			known = append(known, plan)
		}
	}
	if a != nil && a.Provider == "codex" {
		for _, field := range []string{"chatgpt_plan_type", "plan_type", "planType", "subscription_plan", "https://api.openai.com/auth.chatgpt_plan_type"} {
			add(authMetadataString(a, field))
		}
		for _, key := range []string{"id_token", "access_token"} {
			token := authMetadataString(a, key)
			if len(token) > 128<<10 {
				continue
			}
			parts := strings.Split(token, ".")
			if len(parts) != 3 {
				continue
			}
			raw, err := base64.RawURLEncoding.DecodeString(parts[1])
			if err != nil || !ojson.ValidBytes(raw) {
				continue
			}
			claims := ojson.ParseBytes(raw)
			if flat := claims.Get("https://api\\.openai\\.com/auth\\.chatgpt_plan_type"); flat.Type == ojson.String {
				add(flat.String())
			}
			for _, scope := range []ojson.Result{claims.Get("https://api\\.openai\\.com/auth"), claims.Get("auth"), claims} {
				for _, field := range []string{"chatgpt_plan_type", "plan_type", "planType", "subscription_plan"} {
					value := scope.Get(field)
					if value.Type == ojson.String {
						add(value.String())
					}
				}
			}
		}
	}
	if len(known) == 0 {
		return []string{"unknown"}
	}
	return known
}

func LoginPlanType(a *Auth) string { return LoginPlanTypes(a)[0] }
