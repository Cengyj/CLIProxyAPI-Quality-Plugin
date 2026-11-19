//go:build darwin || linux

package login

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MoeForge/moego/ojson"
	codex "github.com/cliproxyapi-plugins/model-quality/login/oauth"
)

const requestFixture = `{"version":1,"type":"diagnostic","job_id":"fixture","attempt":1,"event":"request","phase":"password","request_kind":"password_verify","method":"POST","http_status":403,"duration_ms":1234,"failed":false}`

func TestRequestDiagnosticsClosedProjectionAndBounds(t *testing.T) {
	entry, err := parseRuntimeDiagnostic([]byte(requestFixture))
	if err != nil || entry.Event != "request" || entry.Level != "warn" || entry.HTTPStatus != 403 || entry.DurationMS != 1234 || entry.Method != "POST" {
		t.Fatalf("safe request facts lost: %+v %v", entry, err)
	}
	for _, test := range []struct {
		name  string
		value any
	}{
		{"request_kind", "private-url-password"}, {"phase", "private"}, {"method", "private"},
		{"http_status", 600}, {"http_status", 99}, {"http_status", -1}, {"http_status", 403.5},
		{"duration_ms", -1}, {"duration_ms", 900001}, {"duration_ms", 1.5},
		{"failed", "false"}, {"error_reason", "private-error"}, {"headers", "private-token"},
	} {
		d := ojson.NewDocString(requestFixture)
		if err := d.Set(test.name, test.value); err != nil {
			t.Fatal(err)
		}
		if _, err := parseRuntimeDiagnostic(d.Bytes()); err == nil {
			t.Fatalf("accepted private or malformed field %s", test.name)
		}
	}
	d := ojson.NewDocString(requestFixture)
	_ = d.Set("failed", true)
	_ = d.Set("http_status", 200)
	entry, err = parseRuntimeDiagnostic(d.Bytes())
	if err != nil || !entry.Failed || entry.Level != "error" {
		t.Fatal("a failed body read became successful because HTTP headers arrived")
	}
	limit := []byte(`{"version":1,"type":"diagnostic","job_id":"fixture","attempt":1,"event":"request_log_limit","phase":"session"}`)
	entry, err = parseRuntimeDiagnostic(limit)
	if err != nil || entry.Level != "warn" || entry.Event != "request_log_limit" {
		t.Fatal("request log limit was not disclosed")
	}
}

func TestRequestLogsPersistWithoutLosingFinalStateOrConfirmedEffects(t *testing.T) {
	j, err := openJournal(filepath.Join(t.TempDir(), "journal"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = j.db.Close() })
	record := storedJob{View: Job{ID: "request-job", Status: "running", Phase: "session", Revision: 1}}
	if err := j.save(record); err != nil {
		t.Fatal(err)
	}
	entry, err := parseRuntimeDiagnostic([]byte(requestFixture))
	if err != nil {
		t.Fatal(err)
	}
	for range maxJobLogEntries + 3 {
		record.View.Revision++
		entry.Status = "running"
		if err := j.save(record, entry); err != nil {
			t.Fatal(err)
		}
	}
	record.View.TOTPReset = true
	record.View.SecurityStatus = "totp_saved"
	record.View.Revision++
	if err := j.save(record); err != nil {
		t.Fatal(err)
	}
	record.View.Status, record.View.Phase = "failed", ""
	record.View.ErrorCode, record.View.ErrorStage, record.View.ErrorReason, record.View.RuntimeCode = "runtime_failed", "logout_all", "upstream_rejected", "service_error"
	record.View.Revision++
	if err := j.save(record); err != nil {
		t.Fatal(err)
	}
	logs, err := j.logs(record.View.ID)
	if err != nil {
		t.Fatal(err)
	}
	last := logs.Entries[len(logs.Entries)-1]
	effect := logs.Entries[len(logs.Entries)-2]
	if len(logs.Entries) != maxJobLogEntries || !logs.Truncated || last.Status != "failed" || last.RuntimeCode != "service_error" || last.ErrorStage != "logout_all" || !last.TOTPReset || effect.SecurityStatus != "totp_saved" || !effect.TOTPReset || last.Revision != record.View.Revision {
		t.Fatalf("retention lost terminal diagnosis or confirmed mutation: %+v %+v", last, effect)
	}
}

func TestGenericFailureKeepsLastKnownStage(t *testing.T) {
	s := offlineService(t, offlineOptions(t))
	for _, phase := range []string{"password", "token_exchange", "saving"} {
		j := &session{record: storedJob{View: Job{ID: phase, Status: "running", Phase: phase}}}
		s.finishError(j, errors.New("runtime_timeout"))
		if j.record.View.ErrorStage != phase {
			t.Fatalf("lost timeout stage %s: %+v", phase, j.record.View)
		}
	}
}

func TestRuntimeSourceCodeSurvivesCoarsePublicError(t *testing.T) {
	raw := []byte(`{"version":1,"job_id":"fixture","attempt":1,"error_code":"service_error","error_stage":"password"}`)
	_, err := runtimeResponse(raw, runtimeInput(), errors.New("worker exited"))
	var diagnostic *runtimeDiagnosticError
	if !errors.As(err, &diagnostic) || diagnostic.code != "runtime_failed" || diagnostic.runtimeCode != "service_error" {
		t.Fatal("coarse public error discarded the runtime source code")
	}
	entry := safeLogEntry(Job{Status: "failed", RuntimeCode: diagnostic.runtimeCode, ErrorCode: diagnostic.code})
	if entry.RuntimeCode != "service_error" {
		t.Fatal("runtime code was not retained")
	}
	entry = safeLogEntry(Job{Status: "failed", RuntimeCode: "private-token", SecurityStatus: "private-secret"})
	raw, _ = Encode(entry)
	if strings.Contains(string(raw), "private-") {
		t.Fatal("private runtime metadata escaped projection")
	}
}

func TestNativeRequestEventsValidateIdentityAndPersistOnlySafeFields(t *testing.T) {
	r := nativeFixture(t, "cat >/dev/null\nprintf '%s\\n' '"+requestFixture+"' '{\"version\":1,\"job_id\":\"fixture\",\"attempt\":1,\"error_code\":\"service_error\",\"error_stage\":\"password\"}'\nexit 1\n")
	var events []LogEntry
	_, err := r.RunWithDiagnostics(context.Background(), runtimeInput(), nil, func(event LogEntry) error { events = append(events, event); return nil })
	if err == nil || err.Error() != "runtime_failed" || len(events) != 1 || events[0].HTTPStatus != 403 {
		t.Fatalf("native diagnostics did not reach consumer: %+v %v", events, err)
	}
	for _, raw := range []string{strings.Replace(requestFixture, `"job_id":"fixture"`, `"job_id":"wrong"`, 1), strings.Replace(requestFixture, `"attempt":1`, `"attempt":2`, 1)} {
		stream := &runtimeProgressOutput{request: runtimeInput(), failed: make(chan error, 1), diagnostic: func(LogEntry) error { t.Fatal("wrong job's log accepted"); return nil }}
		stream.line([]byte(raw))
		if stream.err == nil {
			t.Fatal("request diagnostic identity mismatch accepted")
		}
	}
}

type diagnosticFixtureRuntime struct{ offlineRuntime }

func (diagnosticFixtureRuntime) RunWithDiagnostics(ctx context.Context, input RuntimeRequest, progress func(string) error, diagnostic func(LogEntry) error) (string, error) {
	if err := progress("password"); err != nil {
		return "", err
	}
	event, err := parseRuntimeDiagnostic([]byte(requestFixture))
	if err != nil {
		return "", err
	}
	if err := diagnostic(event); err != nil {
		return "", err
	}
	return "", &runtimeDiagnosticError{code: "credentials_rejected", stage: "password", reason: "upstream_rejected", runtimeCode: "password_rejected"}
}

func TestServiceDetailedRuntimeWritesCorrelatedRequestsAndTerminalCause(t *testing.T) {
	opts := offlineOptions(t)
	opts.Runtime = diagnosticFixtureRuntime{}
	s := offlineService(t, opts)
	job, err := s.Start(offlineStart())
	if err != nil {
		t.Fatal(err)
	}
	job = waitLogin(t, s, job.ID, "failed")
	logs, err := s.Logs(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	var request LogEntry
	for _, event := range logs.Entries {
		if event.Event == "request" {
			request = event
		}
	}
	last := logs.Entries[len(logs.Entries)-1]
	if request.HTTPStatus != 403 || request.Status != "running" || request.Revision == 0 || last.RuntimeCode != "password_rejected" || last.ErrorStage != "password" || last.Status != "failed" || job.RuntimeCode != "password_rejected" {
		t.Fatalf("service lost request/terminal diagnostic correlation: %+v %+v", request, last)
	}
	raw, _ := Encode(logs)
	for _, secret := range []string{offlineStart().Credentials.Email, offlineStart().Credentials.Password, offlineStart().Credentials.TOTPSecret} {
		if strings.Contains(string(raw), secret) {
			t.Fatal("service request log leaked credentials")
		}
	}
}

func TestOAuthFailureRetainsStatusAndStageInDurableLogs(t *testing.T) {
	opts := offlineOptions(t)
	opts.Exchange = func(context.Context, string, *codex.PKCECodes, string) (*codex.CodexAuthBundle, error) {
		return nil, &codex.ExchangeError{HTTPStatus: 429, Reason: "upstream_rejected"}
	}
	s := offlineService(t, opts)
	job, err := s.Start(offlineStart())
	if err != nil {
		t.Fatal(err)
	}
	job = waitLogin(t, s, job.ID, "failed")
	logs, err := s.Logs(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	last := logs.Entries[len(logs.Entries)-1]
	if job.HTTPStatus != 429 || last.HTTPStatus != 429 || last.ErrorStage != "token_exchange" || last.ErrorReason != "upstream_rejected" || last.ErrorCode != "exchange_failed" {
		t.Fatalf("token-exchange HTTP diagnosis was lost: %+v", last)
	}
}

func TestServiceHTTPStatusRequiresLastMatchingRequest(t *testing.T) {
	request := LogEntry{Event: "request", Phase: "session", RequestKind: "oauth_authorize", HTTPStatus: 504}
	state := LogEntry{Event: "state", RuntimeCode: "service_error", ErrorStage: "session"}
	for _, tc := range []struct {
		name string
		tail []LogEntry
		want int
	}{
		{"current", nil, 504},
		{"historical reconciliation and failure", []LogEntry{state, state}, 504},
		{"later success", []LogEntry{{Event: "request", Phase: "session", HTTPStatus: 200}}, 0},
		{"other stage", []LogEntry{{Event: "request", Phase: "oauth", HTTPStatus: 503}}, 0},
		{"transport error", []LogEntry{{Event: "request", Phase: "session", HTTPStatus: 504, Failed: true}}, 0},
		{"log limit", []LogEntry{{Event: "request_log_limit", Phase: "session"}}, 0},
		{"new phase", []LogEntry{{Event: "state", Phase: "session"}}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := serviceHTTPStatus(append([]LogEntry{request}, tc.tail...), "session"); got != tc.want {
				t.Errorf("got HTTP %d, want %d", got, tc.want)
			}
		})
	}
}

type gatewayFailureRuntime struct{ offlineRuntime }

func (gatewayFailureRuntime) RunWithDiagnostics(_ context.Context, _ RuntimeRequest, progress func(string) error, diagnostic func(LogEntry) error) (string, error) {
	if err := progress("session"); err != nil {
		return "", err
	}
	if err := diagnostic(LogEntry{Event: "request", Phase: "session", RequestKind: "oauth_authorize", Level: "warn", Method: "GET", HTTPStatus: 504}); err != nil {
		return "", err
	}
	return "", &runtimeDiagnosticError{code: "runtime_failed", stage: "session", runtimeCode: "service_error"}
}

func TestGatewayFailurePersistsHTTPStatus(t *testing.T) {
	opts := offlineOptions(t)
	opts.Runtime = gatewayFailureRuntime{}
	s := offlineService(t, opts)
	job, err := s.Start(offlineStart())
	if err != nil {
		t.Fatal(err)
	}
	job = waitLogin(t, s, job.ID, "failed")
	logs, err := s.Logs(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if job.HTTPStatus != 504 || logs.Entries[len(logs.Entries)-1].HTTPStatus != 504 {
		t.Fatal("gateway status missing from job or terminal event")
	}
}
