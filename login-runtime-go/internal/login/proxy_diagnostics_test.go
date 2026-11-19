package login

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cli-proxy-api/model-quality-login-runtime/internal/legacyruntime"
	"github.com/cli-proxy-api/model-quality-login-runtime/internal/transport"
)

func TestNormalizedSOCKSProxyRetainsDNSBehavior(t *testing.T) {
	for _, engine := range []string{"native", "login_fingerprint"} {
		for _, scheme := range []string{"socks5", "socks5h"} {
			t.Run(engine+"/"+scheme, func(t *testing.T) {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = listener.Close() })
				type target struct {
					kind byte
					host string
				}
				observed := make(chan target, 1)
				finished := make(chan error, 1)
				go func() {
					finished <- func() error {
						conn, err := listener.Accept()
						if err != nil {
							return err
						}
						defer func() { _ = conn.Close() }()
						if err := conn.SetDeadline(time.Now().Add(3 * time.Second)); err != nil {
							return err
						}
						greeting := make([]byte, 2)
						if _, err := io.ReadFull(conn, greeting); err != nil {
							return err
						}
						if greeting[0] != 5 {
							return errors.New("unexpected SOCKS greeting")
						}
						if _, err := io.CopyN(io.Discard, conn, int64(greeting[1])); err != nil {
							return err
						}
						if _, err := conn.Write([]byte{5, 0}); err != nil {
							return err
						}
						request := make([]byte, 4)
						if _, err := io.ReadFull(conn, request); err != nil {
							return err
						}
						length := 0
						switch request[3] {
						case 1:
							length = net.IPv4len
						case 4:
							length = net.IPv6len
						case 3:
							var count [1]byte
							if _, err := io.ReadFull(conn, count[:]); err != nil {
								return err
							}
							length = int(count[0])
						default:
							return errors.New("unexpected SOCKS address type")
						}
						address := make([]byte, length+2)
						if _, err := io.ReadFull(conn, address); err != nil {
							return err
						}
						observed <- target{kind: request[3], host: string(address[:length])}
						// Reject without dialing the target. Only this local handshake runs.
						_, err = conn.Write([]byte{5, 5, 0, 1, 0, 0, 0, 0, 0, 0})
						return err
					}()
				}()
				proxyURL, err := normalizeProxy(scheme + "://" + listener.Addr().String())
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
				defer cancel()
				var client Doer
				if engine == "native" {
					native, err := transport.NewNativeClient(proxyURL, nil)
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(native.Close)
					client = native
				} else {
					fingerprint, err := legacyruntime.New(ctx, legacyruntime.Options{ProxyURL: proxyURL, Profile: "chrome146"})
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { _ = fingerprint.Close() })
					client = fingerprint
				}
				_, err = client.Do(ctx, transport.Request{Method: http.MethodGet, URL: "https://localhost:443/", SingleAttempt: true})
				if err == nil {
					t.Fatal("rejecting local proxy unexpectedly completed a request")
				}
				select {
				case err := <-finished:
					if err != nil {
						t.Fatal(err)
					}
				case <-ctx.Done():
					t.Fatal("local SOCKS handshake did not complete")
				}
				destination := <-observed
				if scheme == "socks5h" && (destination.kind != 3 || destination.host != "localhost") {
					t.Fatal("SOCKS5H normalization lost proxy-side DNS")
				}
				if scheme == "socks5" && (destination.kind == 3 || !net.IP(destination.host).IsLoopback()) {
					t.Fatal("SOCKS5 did not retain local DNS")
				}
			})
		}
	}
}

func TestNativeLoginReportsProxyRefusalWithoutReplayOrPrivateData(t *testing.T) {
	for _, status := range []int{http.StatusProxyAuthRequired, http.StatusBadGateway} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var calls atomic.Int32
			proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(status)
				_, _ = io.WriteString(w, "private-proxy-response")
			}))
			defer proxy.Close()
			proxyURL, _ := url.Parse(proxy.URL)
			proxyURL.User = url.UserPassword("private-user", "private-password")
			in := fixtureInput(authOrigin)
			in.ProxyURL = proxyURL.String()
			_, err := run(t.Context(), in, nil, policy{auth: authOrigin, chat: chatOrigin, now: time.Now})
			var failure *Error
			want := "proxy_connection_failed"
			if status == http.StatusProxyAuthRequired {
				want = "proxy_auth_rejected"
			}
			if !errors.As(err, &failure) || failure.Code != "transport_error" || failure.Stage != "session" || failure.Reason != want {
				t.Fatalf("proxy rejection lost its fixed diagnostic: %#v", failure)
			}
			if calls.Load() != 1 || err.Error() != "transport_error" {
				t.Fatal("proxy rejection was replayed or private diagnostics escaped")
			}
		})
	}
}
