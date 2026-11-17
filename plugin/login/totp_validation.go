package login

import (
	"net/url"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// validTOTPSecret applies the login worker's format rules before credentials
// are stored. Validation never changes the original secret or password.
func validTOTPSecret(value string) bool {
	seed := value
	if strings.HasPrefix(strings.ToLower(value), "otpauth:") {
		uri, err := url.Parse(value)
		if err != nil || !strings.EqualFold(uri.Scheme, "otpauth") || uri.Hostname() != "totp" || uri.Fragment != "" || uri.User != nil {
			return false
		}
		parameters := map[string]string{}
		for _, part := range strings.Split(uri.RawQuery, "&") {
			key, raw, _ := strings.Cut(part, "=")
			key, err = url.QueryUnescape(key)
			if err != nil {
				continue
			}
			switch key {
			case "secret", "algorithm", "digits", "period":
				if _, exists := parameters[key]; exists {
					return false
				}
				decoded, decodeErr := url.QueryUnescape(raw)
				if decodeErr != nil {
					return false
				}
				parameters[key] = decoded
			}
		}
		if algorithm := parameters["algorithm"]; algorithm != "" && strings.ToUpper(algorithm) != "SHA1" {
			return false
		}
		if digits := parameters["digits"]; digits != "" && digits != "6" {
			return false
		}
		if period := parameters["period"]; period != "" && period != "30" {
			return false
		}
		seed = parameters["secret"]
	}
	seed = cases.Upper(language.Und).String(seed)
	seed = strings.Map(func(r rune) rune {
		if r == '-' || totpWorkerWhitespace(r) {
			return -1
		}
		return r
	}, seed)
	seed = strings.TrimRight(seed, "=")
	if len(seed) < 16 || len(seed) > 128 {
		return false
	}
	switch len(seed) % 8 {
	case 0, 2, 4, 5, 7:
	default:
		return false
	}
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	for _, r := range seed {
		if !strings.ContainsRune(alphabet, r) {
			return false
		}
	}
	// Canonical Base32 permits only zero bits in the unused final fragment.
	remaining := uint(len(seed) * 5 % 8)
	last := strings.IndexByte(alphabet, seed[len(seed)-1])
	return last&((1<<remaining)-1) == 0
}

// JavaScript's whitespace set differs from unicode.IsSpace for U+0085/U+FEFF.
func totpWorkerWhitespace(r rune) bool {
	return r >= '\t' && r <= '\r' || r >= '\u2000' && r <= '\u200a' ||
		r == ' ' || r == '\u00a0' || r == '\u1680' || r == '\u2028' || r == '\u2029' ||
		r == '\u202f' || r == '\u205f' || r == '\u3000' || r == '\ufeff'
}
