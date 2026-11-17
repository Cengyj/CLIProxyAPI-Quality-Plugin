package login

import (
	"bytes"
	"encoding/gob"
	"errors"

	bolt "go.etcd.io/bbolt"
)

const maxJobLogEntries = 200

// LogEntry contains fixed diagnostic labels only, never runtime output or credentials.
type LogEntry struct {
	ID          uint64 `json:"id"`
	Time        string `json:"time"`
	Level       string `json:"level"`
	Status      string `json:"status"`
	Phase       string `json:"phase"`
	ErrorCode   string `json:"error_code,omitempty"`
	ErrorStage  string `json:"error_stage,omitempty"`
	ErrorReason string `json:"error_reason,omitempty"`
}

type JobLogs struct {
	JobID   string     `json:"job_id"`
	Entries []LogEntry `json:"entries"`
	Legacy  bool       `json:"legacy"`
}

func safeLogEntry(view Job) LogEntry {
	entry := LogEntry{Level: "info", Status: "unknown"}
	switch view.Status {
	case "queued", "waiting_browser", "running", "exchanging", "awaiting_commit", "committing", "committed", "succeeded":
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
	if runtimeStage(view.ErrorStage) {
		entry.ErrorStage = view.ErrorStage
	}
	if runtimeReason(view.ErrorReason) {
		entry.ErrorReason = view.ErrorReason
	}
	return entry
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
	if s.jobs[id] == nil {
		return JobLogs{}, errors.New("not_found")
	}
	return s.journal.logs(id)
}
