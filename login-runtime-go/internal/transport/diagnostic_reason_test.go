package transport

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestDiagnosticReasonPreservesOnlyOwnedClassifications(t *testing.T) {
	for _, test := range []struct {
		name, want string
		failure    TransportError
	}{
		{"proxy_auth", "proxy_auth_rejected", TransportError{kind: failureProxyAuth, phase: phaseProxyTunnel, status: 407}},
		{"proxy_dns", "proxy_dns_failed", TransportError{kind: failureDNS, phase: phaseProxyConnect}},
		{"proxy_refused", "proxy_connection_refused", TransportError{kind: failureRefused, phase: phaseProxyConnect}},
		{"proxy_timeout", "proxy_connection_timeout", TransportError{kind: failureTimeout, phase: phaseProxyConnect}},
		{"proxy_tls", "proxy_tls_failed", TransportError{kind: failureCertificate, phase: phaseProxyTLS}},
		{"loopback_refused", "proxy_loopback_unreachable", TransportError{kind: failureRefused, phase: phaseProxyConnect, loopbackProxy: true}},
		{"loopback_timeout", "proxy_loopback_unreachable", TransportError{kind: failureTimeout, phase: phaseProxyConnect, loopbackProxy: true}},
		{"tunnel_refusal", "proxy_connection_failed", TransportError{kind: failureProxyStatus, phase: phaseProxyTunnel, status: 502}},
		{"target_dns", "network_dns_failed", TransportError{kind: failureDNS}},
		{"target_refused", "network_connection_refused", TransportError{kind: failureRefused}},
		{"target_timeout", "network_timeout", TransportError{kind: failureTimeout}},
		{"target_certificate", "network_tls_failed", TransportError{kind: failureCertificate}},
		{"target_tls", "network_tls_failed", TransportError{kind: failureTLS}},
		{"unknown_transport", "", TransportError{kind: failureRequest}},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, err := range []error{test.failure, &test.failure, fmt.Errorf("private-wrapper: %w", &test.failure), errors.Join(&test.failure, &test.failure)} {
				if got := DiagnosticReason(err); got != test.want {
					t.Fatalf("unexpected diagnostic label: %q, want %q", got, test.want)
				}
			}
		})
	}
	known := &TransportError{kind: failureProxyAuth, phase: phaseProxyTunnel, status: 407}
	for _, err := range []error{
		nil,
		errors.New("proxy authentication rejected (HTTP 407) private-remote-text"),
		fmt.Errorf("proxy error: %w", errors.New("proxy_connection_failed")),
		errors.Join(known, errors.New("private-unrelated-error")),
		errors.Join(known, &TransportError{kind: failureDNS}),
		errors.Join(known, context.Canceled),
	} {
		if DiagnosticReason(err) != "" {
			t.Fatal("unknown, ambiguous or cancelled error borrowed a transport diagnosis")
		}
	}
}
