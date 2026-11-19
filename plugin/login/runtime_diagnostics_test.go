package login

import (
	"errors"
	"testing"
)

func TestRuntimeNetworkDiagnosticsRemainClosedAcrossIPC(t *testing.T) {
	for _, reason := range []string{
		"proxy_auth_rejected", "proxy_loopback_unreachable", "proxy_tls_failed",
		"proxy_dns_failed", "network_dns_failed", "proxy_connection_refused",
		"network_connection_refused", "proxy_connection_timeout", "network_timeout", "network_tls_failed",
	} {
		t.Run(reason, func(t *testing.T) {
			raw, err := Encode(map[string]any{"version": 1, "job_id": "fixture", "attempt": 1, "error_code": "transport_error", "error_stage": "session", "error_reason": reason})
			if err != nil {
				t.Fatal(err)
			}
			callback, err := runtimeResponse(raw, RuntimeRequest{Version: 1, JobID: "fixture", Attempt: 1}, errors.New("worker exited"))
			var diagnostic *runtimeDiagnosticError
			if callback != "" || !errors.As(err, &diagnostic) || diagnostic.code != "runtime_failed" || diagnostic.stage != "session" || diagnostic.reason != reason {
				t.Fatal("fixed network diagnostic did not survive runtime protocol validation")
			}
			entry := safeLogEntry(Job{Status: "failed", ErrorCode: diagnostic.code, ErrorStage: diagnostic.stage, ErrorReason: diagnostic.reason})
			if entry.ErrorReason != reason || entry.ErrorStage != "session" {
				t.Fatal("validated network diagnostic did not survive durable log projection")
			}
		})
	}
	raw, err := Encode(map[string]any{"version": 1, "job_id": "fixture", "attempt": 1, "error_code": "transport_error", "error_stage": "session", "error_reason": "proxy-secret@private.invalid"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = runtimeResponse(raw, RuntimeRequest{Version: 1, JobID: "fixture", Attempt: 1}, errors.New("worker exited"))
	if err == nil || err.Error() != "runtime_protocol_error" {
		t.Fatal("unrecognized runtime error reason was accepted")
	}
}
