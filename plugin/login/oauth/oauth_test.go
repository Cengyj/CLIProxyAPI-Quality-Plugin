package oauth

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MoeForge/moego/base64"
)

func TestPKCEAndSingleUseExchange(t *testing.T) {
	p, e := GeneratePKCECodes()
	if e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256([]byte(p.CodeVerifier))
	if p.CodeChallenge != base64.RawURLEncoding.EncodeToString(sum[:]) || len(p.CodeVerifier) != 128 {
		t.Fatal("PKCE construction changed")
	}
	var calls atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_ = r.ParseForm()
		if r.Method != "POST" || r.PostForm.Get("code") != "fixture-code" || r.PostForm.Get("code_verifier") != p.CodeVerifier || r.PostForm.Get("redirect_uri") != RedirectURI {
			t.Error("code exchange lost binding")
		}
		fmt.Fprint(w, `{"access_token":"fixture-access","refresh_token":"fixture-refresh","id_token":"fixture-id","expires_in":3600}`)
	}))
	defer s.Close()
	b, e := exchange(context.Background(), s.Client(), s.URL, "fixture-code", p, time.Unix(100, 0))
	if e != nil || b.TokenData.AccessToken != "fixture-access" || calls.Load() != 1 {
		t.Fatal("single exchange failed")
	}
}
func TestExchangeFailuresNeverExposeRemoteBodyOrRetry(t *testing.T) {
	for _, mode := range []string{"denied", "duplicate", "fractional_expiry", "oversize", "redirect", "disconnect"} {
		t.Run(mode, func(t *testing.T) {
			var calls atomic.Int32
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				switch mode {
				case "denied":
					w.WriteHeader(401)
					fmt.Fprint(w, "private-remote-body")
				case "duplicate":
					fmt.Fprint(w, `{"access_token":"a","access_token":"b","refresh_token":"r","id_token":"i","expires_in":1}`)
				case "fractional_expiry":
					fmt.Fprint(w, `{"access_token":"a","refresh_token":"r","id_token":"i","expires_in":1.5}`)
				case "oversize":
					fmt.Fprint(w, strings.Repeat("x", (1<<20)+1))
				case "redirect":
					w.Header().Set("Location", "/private")
					w.WriteHeader(302)
				case "disconnect":
					c, _, e := w.(http.Hijacker).Hijack()
					if e != nil {
						t.Error(e)
						return
					}
					c.Close()
				}
			}))
			defer s.Close()
			client := s.Client()
			client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
			p, _ := GeneratePKCECodes()
			result, e := exchange(context.Background(), client, s.URL, "fixture", p, time.Now())
			if result != nil || e == nil || e.Error() != "exchange_failed" || calls.Load() != 1 {
				t.Fatal("exchange failure was replayed or leaked diagnostics")
			}
		})
	}
}
