package login

import (
	"bytes"
	"encoding/gob"
	"errors"

	bolt "go.etcd.io/bbolt"
)

const maxJobLogEntries = 200

// LogEntry contains fixed diagnostic labels and numeric facts, never raw runtime output or credentials.
type LogEntry struct {
	Event             string `json:"event,omitempty"`
	Revision          uint64 `json:"revision,omitempty"`
	RuntimeCode       string `json:"runtime_code,omitempty"`
	RequestKind       string `json:"request_kind,omitempty"`
	Method            string `json:"method,omitempty"`
	HTTPStatus        int    `json:"http_status,omitempty"`
	DurationMS        int64  `json:"duration_ms,omitempty"`
	Failed            bool   `json:"failed,omitempty"`
	SecurityStatus    string `json:"security_status,omitempty"`
	TOTPReset         bool   `json:"totp_reset,omitempty"`
	DevicesLoggedOut  bool   `json:"devices_logged_out,omitempty"`
	CredentialEnabled bool   `json:"credential_enabled,omitempty"`
	ID                uint64 `json:"id"`
	Time              string `json:"time"`
	Level             string `json:"level"`
	Status            string `json:"status"`
	Phase             string `json:"phase"`
	ErrorCode         string `json:"error_code,omitempty"`
	ErrorStage        string `json:"error_stage,omitempty"`
	ErrorReason       string `json:"error_reason,omitempty"`
}

type JobLogs struct {
	JobID   string     `json:"job_id"`
	Entries []LogEntry `json:"entries"`
	Legacy  bool       `json:"legacy"`
	// These describe the persisted retention window before cursor filtering.
	FirstID       uint64 `json:"first_id"`
	LastID        uint64 `json:"last_id"`
	RetainedCount int    `json:"retained_count"`
	Truncated     bool   `json:"truncated"`
	// These describe only the returned page. An empty page preserves its cursor.
	HasMore    bool   `json:"has_more"`
	NextCursor uint64 `json:"next_cursor"`
}

func safeLogEntry(view Job) LogEntry {
	entry := LogEntry{Event: "state", Level: "info", Status: "unknown", TOTPReset: view.TOTPReset, DevicesLoggedOut: view.DevicesLoggedOut, CredentialEnabled: view.CredentialEnabled}
	switch view.Status {
	case "queued", "waiting_browser", "running", "exchanging", "validating", "awaiting_commit", "committing", "committed", "succeeded":
		entry.Status = view.Status
	case "cancelling", "cancelled", "discarded", "expired", "interrupted", "interaction_required":
		entry.Status, entry.Level = view.Status, "warn"
	case "failed", "conflict", "saved_not_adopted", "commit_unknown", "security_uncertain":
		entry.Status, entry.Level = view.Status, "error"
	}
	switch view.Phase {
	case "queued", "authorization", "token_exchange", "review", "saving":
		entry.Phase = view.Phase
	default:
		if runtimeStage(view.Phase) {
			entry.Phase = view.Phase
		}
	}
	if view.ErrorCode != "" {
		entry.ErrorCode = SafeCode(errors.New(view.ErrorCode))
	}
	if entry.Status == "awaiting_commit" && entry.ErrorCode == "cpa_plan_excluded" {
		entry.Level = "warn"
	}
	if logStage(view.ErrorStage) {
		entry.ErrorStage = view.ErrorStage
	}
	if runtimeReason(view.ErrorReason) {
		entry.ErrorReason = view.ErrorReason
	}
	if view.HTTPStatus >= 100 && view.HTTPStatus <= 599 {
		entry.HTTPStatus = view.HTTPStatus
	}
	if runtimeErrorCode(view.RuntimeCode) != "" {
		entry.RuntimeCode = view.RuntimeCode
	}
	if runtimeDiagnostic(view.SecurityStatus, []string{"pending", "running", "complete", "uncertain", "totp_saved", "failed"}) {
		entry.SecurityStatus = view.SecurityStatus
	}
	return entry
}

func logStage(value string) bool {
	return runtimeStage(value) || runtimeDiagnostic(value, []string{"queued", "token_exchange", "review", "saving"})
}

// The runtime reports service_error for HTTP 5xx. A reconciliation state
// can separate that request from the terminal event in older journals.
func serviceHTTPStatus(entries []LogEntry, stage string) int {
	for i := len(entries) - 1; i >= 0; i-- {
		e := entries[i]
		if e.Event == "state" && e.RuntimeCode == "service_error" && e.ErrorStage == stage {
			continue
		}
		if e.Event == "request" && e.Phase == stage && !e.Failed && e.HTTPStatus >= 500 && e.HTTPStatus <= 599 {
			return e.HTTPStatus
		}
		break
	}
	return 0
}

func (j *journal) logs(id string) (JobLogs, error) {
	logs := JobLogs{JobID: id, Entries: []LogEntry{}}
	err := j.db.View(func(tx *bolt.Tx) error {
		raw := tx.Bucket([]byte("jobs")).Get([]byte(id))
		if raw == nil {
			return errors.New("not_found")
		}
		var record storedJob
		if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&record); err != nil {
			return errors.New("storage_unavailable")
		}
		logs.Entries = append(logs.Entries, record.LogEntries...)
		logs.RetainedCount = len(logs.Entries)
		if logs.RetainedCount > 0 {
			logs.FirstID = logs.Entries[0].ID
			logs.LastID = logs.Entries[logs.RetainedCount-1].ID
			logs.NextCursor = logs.LastID
			logs.Truncated = logs.FirstID > 1
		}
		// An old record remains marked incomplete even after new real events arrive.
		logs.Legacy = record.LogVersion == 0
		return nil
	})
	if err != nil {
		if err.Error() == "not_found" {
			return JobLogs{}, err
		}
		return JobLogs{}, errors.New("storage_unavailable")
	}
	return logs, nil
}

func (s *Service) Logs(id string) (JobLogs, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.jobs[id] == nil || s.jobs[id].record.HistoryDeleted {
		return JobLogs{}, errors.New("not_found")
	}
	return s.journal.logs(id)
}
