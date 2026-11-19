package login

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
	bolt "go.etcd.io/bbolt"
)

func TestJobLogsRetainedWindowSurvivesReopen(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "journal")
	j, err := openJournal(dir)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 11, 17, 0, 0, 0, 0, time.UTC)
	for i := range maxJobLogEntries + 5 {
		j.now = func() time.Time { return base.Add(time.Duration(i) * time.Millisecond) }
		phase := "session"
		if i%2 != 0 {
			phase = "authorization"
		}
		record := storedJob{View: Job{ID: "retained", Status: "running", Phase: phase}}
		if err := j.save(record); err != nil {
			t.Fatal(err)
		}
		// Saving an identical diagnostic projection must not manufacture events.
		if err := j.save(record); err != nil {
			t.Fatal(err)
		}
	}
	if err := j.db.Close(); err != nil {
		t.Fatal(err)
	}
	j, err = openJournal(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = j.db.Close() })
	logs, err := j.logs("retained")
	if err != nil {
		t.Fatal(err)
	}
	if logs.Legacy || !logs.Truncated || logs.RetainedCount != maxJobLogEntries || logs.FirstID != 6 || logs.LastID != 205 || logs.NextCursor != 205 || logs.HasMore {
		t.Fatalf("persisted retention metadata is inaccurate: %+v", logs)
	}
	for i, entry := range logs.Entries {
		if entry.ID != uint64(i+6) {
			t.Fatalf("retained sequence changed at %d: %d", i, entry.ID)
		}
	}
}

func TestJobLogsLegacyAndTruncationAreIndependent(t *testing.T) {
	j, err := openJournal(filepath.Join(t.TempDir(), "journal"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = j.db.Close() })
	for _, test := range []struct {
		name      string
		version   uint8
		first     uint64
		legacy    bool
		truncated bool
	}{
		{"legacy-empty", 0, 0, true, false},
		{"legacy-append", 0, 1, true, false},
		{"legacy-truncated", 0, 9, true, true},
		{"complete", 1, 1, false, false},
		{"truncated", 1, 9, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			record := storedJob{View: Job{ID: test.name}, LogVersion: test.version}
			if test.first != 0 {
				record.LogEntries = []LogEntry{{ID: test.first, Status: "queued", Level: "info"}}
			}
			if err := j.db.Update(func(tx *bolt.Tx) error { return putGob(tx, "jobs", test.name, record) }); err != nil {
				t.Fatal(err)
			}
			logs, err := j.logs(test.name)
			if err != nil || logs.Legacy != test.legacy || logs.Truncated != test.truncated || logs.FirstID != test.first || logs.LastID != test.first || logs.RetainedCount != len(record.LogEntries) {
				t.Fatalf("legacy and retention facts conflated: %+v err=%v", logs, err)
			}
		})
	}
}

func TestJobLogRouteCursorPagesKeepRetentionMetadata(t *testing.T) {
	s := offlineService(t, offlineOptions(t))
	record := storedJob{View: Job{ID: "page", Status: "committed"}, LogVersion: 1,
		LogEntries: []LogEntry{{ID: 7, Level: "info"}, {ID: 8, Level: "info"}, {ID: 9, Level: "info"}}}
	if err := s.journal.db.Update(func(tx *bolt.Tx) error { return putGob(tx, "jobs", "page", record) }); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.jobs[record.View.ID] = &session{record: record}
	s.mu.Unlock()
	for _, test := range []struct {
		cursor string
		limit  string
		ids    []uint64
		next   uint64
		more   bool
	}{
		{"0", "2", []uint64{7, 8}, 8, true},
		{"8", "2", []uint64{9}, 9, false},
		{"9", "2", nil, 9, false},
		{"99", "2", nil, 99, false},
		{"0", "3", []uint64{7, 8, 9}, 9, false},
	} {
		t.Run(test.cursor+"-"+test.limit, func(t *testing.T) {
			response := s.Handle(context.Background(), pluginapi.ManagementRequest{Method: http.MethodGet, Path: RoutePrefix + "/logs", Query: url.Values{"id": {"page"}, "cursor": {test.cursor}, "limit": {test.limit}}})
			if response.StatusCode != http.StatusOK {
				t.Fatalf("log page failed: %d %s", response.StatusCode, response.Body)
			}
			var logs JobLogs
			if err := json.Unmarshal(response.Body, &logs); err != nil {
				t.Fatal(err)
			}
			if logs.JobID != "page" || logs.FirstID != 7 || logs.LastID != 9 || logs.RetainedCount != 3 || !logs.Truncated || logs.Legacy || logs.NextCursor != test.next || logs.HasMore != test.more || len(logs.Entries) != len(test.ids) {
				t.Fatalf("cursor page metadata is inaccurate: %+v", logs)
			}
			for i, id := range test.ids {
				if logs.Entries[i].ID != id {
					t.Fatalf("cursor repeated or skipped an event: %+v", logs.Entries)
				}
			}
			if logs.Entries == nil {
				t.Fatal("empty pages must serialize an array")
			}
		})
	}
}

func TestJobLogProjectionKeepsDiagnosticsClosed(t *testing.T) {
	const secret = "private-log-canary-password-token-seed"
	j, err := openJournal(filepath.Join(t.TempDir(), "journal"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = j.db.Close() })
	record := storedJob{View: Job{ID: "safe", Status: secret, Phase: secret, ErrorCode: secret, ErrorStage: secret, ErrorReason: secret, Email: secret, AuthorizationURL: "https://example.test/?code=" + secret, TargetAuthID: secret}}
	if err := j.save(record); err != nil {
		t.Fatal(err)
	}
	logs, err := j.logs("safe")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Encode(logs)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), secret) || strings.Contains(string(raw), "https://") || strings.Contains(string(raw), "authorization_url") || strings.Contains(string(raw), "email") {
		t.Fatal("a private or unrecognized field escaped the diagnostic projection")
	}
	if logs.Entries[0].Status != "unknown" || logs.Entries[0].Phase != "" || logs.Entries[0].ErrorStage != "" || logs.Entries[0].ErrorReason != "" {
		t.Fatal("unrecognized diagnostic labels were preserved")
	}
	if got := safeLogEntry(Job{Status: "validating"}); got.Status != "validating" || got.Level != "info" {
		t.Fatalf("identity validation is not represented as progress: %+v", got)
	}
	if got := safeLogEntry(Job{Status: "awaiting_commit", Phase: "review", ErrorCode: "cpa_plan_excluded"}); got.Level != "warn" || got.ErrorCode != "cpa_plan_excluded" {
		t.Fatalf("upload exclusion is missing from abnormal diagnostics: %+v", got)
	}
	if got := safeLogEntry(Job{Status: "awaiting_commit", Phase: "review"}); got.Level != "info" {
		t.Fatal("ordinary pending confirmation was classified as abnormal")
	}
}
