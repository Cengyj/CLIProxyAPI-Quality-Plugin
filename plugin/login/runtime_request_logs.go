package login

import "github.com/MoeForge/moego/ojson"

func runtimeRequestKind(value string) bool {
	return runtimeDiagnostic(value, []string{"auth_providers", "auth_csrf", "auth_signin", "auth_session", "auth_callback", "oauth_authorize", "authorize_continue", "password_verify", "totp_challenge", "totp_verify", "session_select", "workspace_select", "organization_select", "consent_accept", "mfa_info", "totp_disable", "totp_enroll", "totp_activate", "logout_all", "navigation"})
}

// Reject unknown labels and malformed numbers before any event reaches storage.
// Neither runtime output nor an arbitrary upstream string enters this type.
func parseRuntimeDiagnostic(raw []byte) (LogEntry, error) {
	entry := LogEntry{Level: "info"}
	invalid := func() (LogEntry, error) { return LogEntry{}, runtimeError("runtime_protocol_error") }
	r, err := Object(raw, "version", "type", "job_id", "attempt", "event", "phase", "request_kind", "method", "http_status", "duration_ms", "error_reason", "failed")
	if err != nil || r.Get("type").String() != "diagnostic" || r.Get("event").Type != ojson.String || r.Get("phase").Type != ojson.String || !runtimeStage(r.Get("phase").String()) {
		return invalid()
	}
	entry.Event, entry.Phase = r.Get("event").String(), r.Get("phase").String()
	if entry.Event == "request_log_limit" {
		if _, err := Object(raw, "version", "type", "job_id", "attempt", "event", "phase"); err != nil {
			return invalid()
		}
		entry.Level = "warn"
		return entry, nil
	}
	if entry.Event != "request" || r.Get("request_kind").Type != ojson.String || !runtimeRequestKind(r.Get("request_kind").String()) || r.Get("method").Type != ojson.String || !runtimeDiagnostic(r.Get("method").String(), []string{"GET", "HEAD", "POST"}) {
		return invalid()
	}
	integer := func(name string, max int64) (int64, bool) {
		v := r.Get(name)
		return v.Int(), v.Type == ojson.Number && v.Int() >= 0 && v.Int() <= max && v.Float() == float64(v.Int())
	}
	status, validStatus := integer("http_status", 599)
	duration, validDuration := integer("duration_ms", 900000)
	failed := r.Get("failed")
	if !validStatus || status > 0 && status < 100 || !validDuration || failed.Type != ojson.True && failed.Type != ojson.False {
		return invalid()
	}
	if reason := r.Get("error_reason"); reason.Exists() {
		if reason.Type != ojson.String || !runtimeReason(reason.String()) {
			return invalid()
		}
		entry.ErrorReason = reason.String()
	}
	entry.RequestKind, entry.Method = r.Get("request_kind").String(), r.Get("method").String()
	entry.HTTPStatus, entry.DurationMS, entry.Failed = int(status), duration, failed.Bool()
	if status >= 400 {
		entry.Level = "warn"
	}
	if entry.Failed {
		entry.Level = "error"
	}
	return entry, nil
}

func (s *Service) runtimeDiagnostic(j *session, entry LogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.fault || s.jobs[j.record.View.ID] != j || j.record.View.Status != "running" || !j.owner.Valid() {
		return runtimeError("cancelled")
	}
	entry.Status = j.record.View.Status
	j.record.View.Revision++
	return s.persistLocked(j, entry)
}
