package quality

import (
	"net/http"
	"strings"
	"unicode"

	"github.com/MoeForge/moego/ojson"
)

// Only the inner status returned by CPA's API-call endpoint reaches this
// classifier. Provider bodies stay local and are never copied into a sample.
func managedUpstreamCondition(status int, body, model string) string {
	switch status {
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusPaymentRequired:
		return "payment_required"
	case http.StatusBadRequest, http.StatusNotFound, http.StatusTooManyRequests:
	default:
		return ""
	}
	if len(body) > 16<<10 {
		return ""
	}
	message := strings.TrimSpace(body)
	var code, kind string
	if strings.HasPrefix(message, "{") {
		if !ojson.Valid(message) || uniqueJSON([]byte(message)) != nil {
			return ""
		}
		fields := ojson.Parse(message)
		if status == http.StatusTooManyRequests && limitedQuotaFields(fields, 0) {
			return ""
		}
		if status != http.StatusTooManyRequests && conflictingErrorModel(fields, model) {
			return ""
		}
		if nested := fields.Get("error"); nested.Exists() {
			if !nested.IsObject() || status != http.StatusTooManyRequests && conflictingErrorModel(nested, model) {
				return ""
			}
			fields = nested
		}
		for _, key := range []string{"type", "code", "message"} {
			value := fields.Get(key)
			if value.Exists() && value.Raw != "null" && value.Type != ojson.String {
				return ""
			}
		}
		code = strings.ToLower(strings.TrimSpace(fields.Get("code").String()))
		kind = strings.ToLower(strings.TrimSpace(fields.Get("type").String()))
		message = fields.Get("message").String()
		if message == "" && fields.Get("detail").Type == ojson.String {
			message = fields.Get("detail").String()
		}
	}
	message = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(message)), ".")
	if status == http.StatusTooManyRequests {
		if limitedQuotaText(message) {
			return ""
		}
		for _, field := range []string{code, kind} {
			switch field {
			case "insufficient_quota", "quota_exhausted", "quota_exceeded", "usage_limit_reached", "billing_hard_limit_reached":
				return "quota_exhausted"
			}
		}
		switch message {
		case "quota exhausted", "quota exceeded", "insufficient quota", "you have exhausted your quota",
			"you exceeded your current quota, please check your plan and billing details":
			return "quota_exhausted"
		}
		return ""
	}
	for _, wording := range [][2]string{
		{"the model ", " is not supported"},
		{"model ", " is not supported"},
		{"the ", " model is not supported when using codex with a chatgpt account"},
		{"the model ", " does not exist or you do not have access to it"},
	} {
		name, starts := strings.CutPrefix(message, wording[0])
		name, ends := strings.CutSuffix(name, wording[1])
		if starts && ends {
			model = strings.ToLower(strings.TrimSpace(model))
			for _, quote := range []string{"", "'", "\"", "`"} {
				if model != "" && name == quote+model+quote {
					return "model_unsupported"
				}
			}
			return ""
		}
	}
	for _, field := range []string{code, kind} {
		switch field {
		case "model_not_found", "model_not_supported", "unsupported_model", "model_not_found_error", "unknown_model", "model_does_not_exist":
			return "model_unsupported"
		}
	}
	switch message {
	case "model not supported", "unsupported model", "model not found", "unknown model", "the requested model is not supported":
		return "model_unsupported"
	}
	return ""
}

func conflictingErrorModel(fields ojson.Result, model string) bool {
	value := fields.Get("model")
	return value.Exists() && value.Raw != "null" && (value.Type != ojson.String || !strings.EqualFold(strings.TrimSpace(value.String()), model))
}

// A rate limit or a limit on an individual model does not establish that
// the account has exhausted its allowance, even alongside a quota error code.
func limitedQuotaFields(fields ojson.Result, depth int) bool {
	if depth >= 8 {
		return true
	}
	limited := false
	fields.ForEach(func(key, value ojson.Result) bool {
		if value.Raw == "null" || value.String() == "" {
			return true
		}
		switch strings.ToLower(key.String()) {
		case "model", "models", "rate_limit", "rate_limits", "retry_after", "retry_after_ms", "retry_after_seconds", "tpm", "rpm", "tps", "rps":
			limited = true
		case "code", "type", "reason", "message", "detail", "scope", "limit_scope", "quota_scope", "rate_limit_type",
			"param", "unit", "limit", "limit_type", "limit_name", "quota_metric", "metric", "dimension":
			if value.Type == ojson.String {
				limited = limitedQuotaText(value.String())
			}
		}
		if !limited && value.Type == ojson.JSON {
			limited = limitedQuotaFields(value, depth+1)
		}
		return !limited
	})
	return limited
}

func limitedQuotaText(text string) bool {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) })
	for _, word := range words {
		switch word {
		case "model", "models", "tpm", "rpm", "rps", "tps", "rate", "retry", "retryable", "overloaded",
			"minute", "minutes", "second", "seconds", "concurrency", "concurrent", "backoff":
			return true
		}
	}
	normalized := strings.Join(words, " ")
	return normalized == "tokens" || normalized == "requests" ||
		strings.Contains(normalized, "too many requests") || strings.Contains(normalized, "try again")
}
