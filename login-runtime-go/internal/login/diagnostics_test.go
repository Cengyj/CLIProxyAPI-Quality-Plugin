package login

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/cli-proxy-api/model-quality-login-runtime/internal/transport"
)

func TestActualLoginRequestDiagnosticsAreUsefulAndPrivate(t *testing.T) {
	for _, mode := range []string{"success", "password-rejected", "totp-disconnect"} {
		t.Run(mode, func(t *testing.T) {
			fixture := newLoginFixture(t, mode)
			var events []RequestDiagnostic
			ctx := WithRequestDiagnostics(context.Background(), func(event RequestDiagnostic) { events = append(events, event) })
			_, err := run(ctx, fixture.in, nil, fixture.p)
			if (err == nil) != (mode == "success") {
				t.Fatalf("unexpected fixture outcome %s: %v", mode, err)
			}
			if len(events) < 4 {
				t.Fatalf("actual protocol requests were not recorded: %d", len(events))
			}
			var password, failed bool
			for _, event := range events {
				if event.Phase == "" || event.Method == "" || event.DurationMS < 0 || event.RequestKind == "" {
					t.Fatalf("missing diagnostic facts %+v", event)
				}
				if event.RequestKind == "password_verify" {
					password = true
					if mode == "password-rejected" && event.HTTPStatus != http.StatusUnauthorized {
						t.Fatalf("failure lost HTTP status: %+v", event)
					}
				}
				failed = failed || event.Failed
			}
			if !password || mode == "totp-disconnect" && !failed {
				t.Fatal("failed request diagnostic was lost")
			}
			raw, e := encodePayload(events)
			if e != nil {
				t.Fatal(e)
			}
			for _, secret := range []string{fixture.in.Credentials.Email, fixture.in.Credentials.Password, fixture.in.Credentials.TOTPSecret, fixture.in.State, fixture.server.URL, "cookie", "authorization_url", "headers"} {
				if strings.Contains(string(raw), secret) {
					t.Fatalf("request diagnostics leaked %q", secret)
				}
			}
		})
	}
}

func TestUnknownEndpointCannotBecomeDiagnosticIdentity(t *testing.T) {
	const private = "private-password-token"
	var event RequestDiagnostic
	ctx := WithRequestDiagnostics(context.Background(), func(value RequestDiagnostic) { event = value })
	f := &flow{stage: "session"}
	u, _ := url.Parse("https://example.invalid/" + private + "?code=" + private)
	f.observeRequest(ctx, transport.Request{Method: http.MethodGet, URL: u.String()}, u, time.Now(), 200, nil)
	raw, err := encodePayload(event)
	if err != nil {
		t.Fatal(err)
	}
	if event.RequestKind != "navigation" || strings.Contains(string(raw), private) || strings.Contains(string(raw), "example.invalid") {
		t.Fatal("dynamic URL became persistent diagnostic data")
	}
}
