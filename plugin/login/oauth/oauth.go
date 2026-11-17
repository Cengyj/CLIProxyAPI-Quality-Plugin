// Package oauth contains the bounded Codex authorization-code exchange adapted
// from CLIProxyAPI 8.0.67 internal/auth/codex/{pkce,openai_auth}.go.
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/MoeForge/moego/base64"
	"github.com/MoeForge/moego/ojson"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/proxyutil"
)

const ClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
const RedirectURI = "http://localhost:1455/auth/callback"
const AuthURL = "https://auth.openai.com/oauth/authorize"
const TokenURL = "https://auth.openai.com/oauth/token"

type Config struct{ ProxyURL string }
type PKCECodes struct{ CodeVerifier, CodeChallenge string }
type CodexTokenData struct{ IDToken, AccessToken, RefreshToken, AccountID, Email, PlanType, Expire string }
type CodexAuthBundle struct {
	TokenData   CodexTokenData
	LastRefresh string
}
type CodexAuth struct{ proxy string }

func GeneratePKCECodes() (*PKCECodes, error) {
	var b [96]byte
	if _, e := rand.Read(b[:]); e != nil {
		return nil, errors.New("exchange_failed")
	}
	v := base64.RawURLEncoding.EncodeToString(b[:])
	h := sha256.Sum256([]byte(v))
	return &PKCECodes{v, base64.RawURLEncoding.EncodeToString(h[:])}, nil
}
func NewCodexAuth(c *Config) *CodexAuth { return NewCodexAuthWithProxyURL(c, "") }
func NewCodexAuthWithProxyURL(c *Config, p string) *CodexAuth {
	if p == "" && c != nil {
		p = c.ProxyURL
	}
	return &CodexAuth{proxy: p}
}
func (*CodexAuth) GenerateAuthURL(state string, p *PKCECodes) (string, error) {
	if p == nil || p.CodeVerifier == "" || state == "" {
		return "", errors.New("exchange_failed")
	}
	q := url.Values{"client_id": {ClientID}, "response_type": {"code"}, "redirect_uri": {RedirectURI}, "scope": {"openid email profile offline_access"}, "state": {state}, "code_challenge": {p.CodeChallenge}, "code_challenge_method": {"S256"}, "prompt": {"login"}, "id_token_add_organizations": {"true"}, "codex_cli_simplified_flow": {"true"}}
	return AuthURL + "?" + q.Encode(), nil
}
func (o *CodexAuth) ExchangeLoginCode(ctx context.Context, code string, p *PKCECodes) (*CodexAuthBundle, error) {
	if p == nil || code == "" || p.CodeVerifier == "" {
		return nil, errors.New("exchange_failed")
	}
	tr, _, err := proxyutil.BuildHTTPTransport(o.proxy)
	if err != nil {
		return nil, errors.New("invalid_proxy")
	}
	if tr == nil {
		tr = &http.Transport{Proxy: nil}
	}
	tr.DisableKeepAlives = true
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 45 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return exchange(ctx, client, TokenURL, code, p, time.Now())
}

// The code is single use. This function never retries or follows a redirect,
// and no transport/server diagnostics or token body leave this boundary.
func exchange(ctx context.Context, client *http.Client, endpoint, code string, p *PKCECodes, now time.Time) (*CodexAuthBundle, error) {
	form := url.Values{"grant_type": {"authorization_code"}, "client_id": {ClientID}, "code": {code}, "redirect_uri": {RedirectURI}, "code_verifier": {p.CodeVerifier}}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if e != nil {
		return nil, errors.New("exchange_failed")
	}
	req.GetBody = nil
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	res, e := client.Do(req)
	if e != nil {
		return nil, errors.New("exchange_failed")
	}
	defer res.Body.Close()
	b, e := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	defer clear(b)
	if e != nil || len(b) > 1<<20 || res.StatusCode != 200 || !ojson.ValidBytes(b) {
		return nil, errors.New("exchange_failed")
	}
	r := ojson.ParseBytes(b)
	if !r.IsObject() {
		return nil, errors.New("exchange_failed")
	}
	seen := map[string]bool{}
	valid := true
	r.ForEach(func(k, v ojson.Result) bool {
		if seen[k.String()] {
			valid = false
		}
		seen[k.String()] = true
		return valid
	})
	for _, k := range []string{"access_token", "refresh_token", "id_token"} {
		v := r.Get(k)
		if v.Type != ojson.String || v.String() == "" {
			valid = false
		}
	}
	exp := r.Get("expires_in")
	seconds, parseErr := strconv.ParseInt(exp.Raw, 10, 64)
	if exp.Type != ojson.Number || parseErr != nil || seconds <= 0 || seconds > 365*86400 {
		valid = false
	}
	if !valid {
		return nil, errors.New("exchange_failed")
	}
	return &CodexAuthBundle{TokenData: CodexTokenData{IDToken: r.Get("id_token").String(), AccessToken: r.Get("access_token").String(), RefreshToken: r.Get("refresh_token").String(), Expire: now.Add(time.Duration(seconds) * time.Second).UTC().Format(time.RFC3339)}, LastRefresh: now.UTC().Format(time.RFC3339)}, nil
}
