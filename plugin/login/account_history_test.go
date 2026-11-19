package login

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/MoeForge/moego/ojson"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

func historyJob(id, accountID string, sequence uint64, created time.Time) storedJob {
	return storedJob{
		AccountSequence: sequence, Created: created, Updated: created,
		View: Job{ID: id, AccountID: accountID, Revision: 1, Method: "password_totp", Intent: "import", Status: "cancelled", CreatedAt: created.Format(time.RFC3339Nano)},
	}
}

func historyIDs(items []Job) []string {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	return ids
}

func TestAccountJobHistoryFiltersBeforeGlobalLimit(t *testing.T) {
	s := offlineService(t, offlineOptions(t))
	a := importOfflineAccount(t, s)
	base := s.opts.Now().Add(-time.Hour).Truncate(time.Second)
	s.mu.Lock()
	for i := range 205 {
		record := historyJob(fmt.Sprintf("foreign-%03d", i), "another-account", uint64(i), base.Add(time.Duration(i+10)*time.Second))
		s.jobs[record.View.ID] = &session{record: record, displayEmail: a.Email}
	}
	for i := range 3 {
		record := historyJob(fmt.Sprintf("owned-%d", i), a.ID, uint64(i+1), base.Add(time.Duration(i)*time.Second))
		record.View.AuthorizationURL = "https://auth.openai.com/private-authorization"
		s.jobs[record.View.ID] = &session{record: record, displayEmail: a.Email}
	}
	legacy := historyJob("legacy-same-email", "", 1000, base.Add(time.Hour))
	s.jobs[legacy.View.ID] = &session{record: legacy, displayEmail: a.Email}
	prefix := historyJob("matching-id-prefix", a.ID+"-other", 1000, base.Add(time.Hour))
	s.jobs[prefix.View.ID] = &session{record: prefix, displayEmail: a.Email}
	s.mu.Unlock()

	global, more, err := s.JobsPage("", 200)
	if err != nil || len(global) != 200 || !more {
		t.Fatalf("global history bound changed: count=%d more=%t err=%v", len(global), more, err)
	}
	for _, job := range global {
		if job.AccountID == a.ID {
			t.Fatal("fixture did not place the account outside the global result limit")
		}
	}
	items, more, err := s.JobsPage(a.ID, 2)
	if err != nil || !more || !reflect.DeepEqual(historyIDs(items), []string{"owned-2", "owned-1"}) {
		t.Fatalf("account history was globally truncated or not ordered: %v, more=%t err=%v", historyIDs(items), more, err)
	}
	for _, job := range items {
		if job.AccountID != a.ID || job.AuthorizationURL != "" {
			t.Fatal("account history crossed ownership or exposed an authorization URL")
		}
	}
	items, more, err = s.JobsPage(a.ID, 200)
	if err != nil || more || !reflect.DeepEqual(historyIDs(items), []string{"owned-2", "owned-1", "owned-0"}) {
		t.Fatalf("account history inferred ownership from email or ID prefix: %v more=%t err=%v", historyIDs(items), more, err)
	}
}

func TestAccountJobHistoryRouteValidatesScopeAndLimit(t *testing.T) {
	s := offlineService(t, offlineOptions(t))
	a := importOfflineAccount(t, s)
	base := s.opts.Now().Add(-time.Minute).Truncate(time.Second)
	s.mu.Lock()
	for i := range 2 {
		record := historyJob(fmt.Sprintf("route-%d", i), a.ID, uint64(i+1), base.Add(time.Duration(i)*time.Millisecond))
		s.jobs[record.View.ID] = &session{record: record, displayEmail: a.Email}
	}
	s.mu.Unlock()
	request := func(path string, query url.Values) pluginapi.ManagementResponse {
		return s.Handle(context.Background(), pluginapi.ManagementRequest{Method: http.MethodGet, Path: RoutePrefix + path, Query: query})
	}
	response := request("/jobs", url.Values{"account_id": {a.ID}, "limit": {"1"}})
	value := ojson.ParseBytes(response.Body)
	if response.StatusCode != http.StatusOK || !value.Get("has_more").Bool() || value.Get("items.0.id").String() != "route-1" || len(value.Get("items").Array()) != 1 {
		t.Fatalf("filtered route did not return its ordered bounded page: %d %s", response.StatusCode, response.Body)
	}
	for _, query := range []url.Values{
		{"account_id": {""}}, {"account_id": {strings.Repeat("a", 513)}}, {"account_id": {a.ID, a.ID}},
		{"account_id": {a.ID}, "limit": {"0"}}, {"account_id": {a.ID}, "limit": {"201"}},
		{"account_id": {a.ID}, "limit": {"1", "2"}}, {"account_id": {a.ID}, "limit": {"1.5"}},
		{"account_id": {a.ID}, "cursor": {"0"}}, {"account_id": {a.ID}, "email": {a.Email}},
	} {
		response = request("/jobs", query)
		if response.StatusCode != http.StatusBadRequest || ojson.GetBytes(response.Body, "code").String() != "invalid_request" {
			t.Fatalf("malformed history query accepted: %v: %d %s", query, response.StatusCode, response.Body)
		}
	}
	response = request("/jobs", url.Values{"account_id": {a.ID + "-missing"}})
	if response.StatusCode != http.StatusNotFound || ojson.GetBytes(response.Body, "code").String() != "not_found" {
		t.Fatalf("missing account did not fail explicitly: %d %s", response.StatusCode, response.Body)
	}
	for _, path := range []string{"/accounts", "/detail", "/logs"} {
		if response = request(path, url.Values{"account_id": {a.ID}}); response.StatusCode != http.StatusBadRequest {
			t.Fatalf("account history scope escaped jobs route: %s", path)
		}
	}
}

func TestAccountJobHistoryOrdersEqualTimesDeterministically(t *testing.T) {
	s := offlineService(t, offlineOptions(t))
	a := importOfflineAccount(t, s)
	base := s.opts.Now().Add(-time.Minute).Truncate(time.Second)
	s.mu.Lock()
	for _, id := range []string{"b", "a", "c"} {
		record := historyJob(id, a.ID, 1, base)
		s.jobs[id] = &session{record: record}
	}
	s.mu.Unlock()
	for range 5 {
		items, more, err := s.JobsPage(a.ID, 3)
		if err != nil || more || !reflect.DeepEqual(historyIDs(items), []string{"c", "b", "a"}) {
			t.Fatalf("history changed order across reads: %v more=%t err=%v", historyIDs(items), more, err)
		}
	}
}

func TestAccountSyncIndexPreservesSequenceAndCurrentJob(t *testing.T) {
	base := time.Date(2026, time.November, 17, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name          string
		current       string
		currentStatus string
		records       []storedJob
		want          string
		changed       bool
	}{
		{"sequence_before_time", "current", "cancelled", []storedJob{historyJob("current", "subject", 5, base), historyJob("higher-sequence", "subject", 6, base.Add(-time.Hour)), historyJob("newer-clock", "subject", 4, base.Add(time.Hour))}, "higher-sequence", true},
		{"time_breaks_sequence_tie", "current", "cancelled", []storedJob{historyJob("current", "subject", 5, base), historyJob("later", "subject", 5, base.Add(time.Second))}, "later", true},
		{"equal_rank_keeps_current", "current", "cancelled", []storedJob{historyJob("current", "subject", 5, base), historyJob("tied", "subject", 5, base)}, "current", false},
		{"foreign_jobs_never_adopted", "", "", []storedJob{historyJob("owned", "subject", 1, base), historyJob("foreign", "other", 999, base.Add(time.Hour))}, "owned", true},
		{"same_job_status_advances", "current", "running", []storedJob{historyJob("current", "subject", 5, base)}, "current", true},
		{"legacy_zero_sequences", "", "", []storedJob{historyJob("earlier", "subject", 0, base), historyJob("later", "subject", 0, base.Add(time.Second))}, "later", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := offlineOptions(t)
			opts.Now = func() time.Time { return base.Add(2 * time.Hour) }
			s := offlineService(t, opts)
			a := importOfflineAccount(t, s)
			s.accountMu.Lock()
			defer s.accountMu.Unlock()
			stored := s.accounts[a.ID]
			stored.View.LastJobID, stored.View.LastJobStatus = tc.current, tc.currentStatus
			if err := s.saveAccountLocked(stored); err != nil {
				t.Fatal(err)
			}
			s.mu.Lock()
			for _, record := range tc.records {
				if record.View.AccountID == "subject" {
					record.View.AccountID = a.ID
				}
				s.jobs[record.View.ID] = &session{record: record}
			}
			s.mu.Unlock()
			if err := s.syncAccountJobsLocked(); err != nil {
				t.Fatal(err)
			}
			got := s.accounts[a.ID].View
			wantRevision := a.Revision
			if tc.changed {
				wantRevision++
			}
			if got.LastJobID != tc.want || got.LastJobStatus != "cancelled" || got.Revision != wantRevision {
				t.Fatalf("account synchronization changed ordering or skipped status: got=%+v wantJob=%s wantRevision=%d", got, tc.want, wantRevision)
			}
			if err := s.syncAccountJobsLocked(); err != nil || s.accounts[a.ID].View.Revision != wantRevision {
				t.Fatalf("unchanged indexed synchronization rewrote the account: %v", err)
			}
		})
	}
}

func TestAccountLastJobMetadataIsDerivedWithoutStoredMigration(t *testing.T) {
	s := offlineService(t, offlineOptions(t))
	a := importOfflineAccount(t, s)
	record := historyJob("older-current-operation", a.ID, 1, s.opts.Now().Add(-time.Hour))
	record.View.Automatic, record.View.Intent, record.View.Phase = true, "reauthorize", "oauth"
	s.accountMu.Lock()
	defer s.accountMu.Unlock()
	s.mu.Lock()
	s.jobs[record.View.ID] = &session{record: record}
	s.mu.Unlock()
	stored := s.accounts[a.ID]
	stored.View.LastJobID, stored.View.LastJobStatus = record.View.ID, record.View.Status
	if err := s.saveAccountLocked(stored); err != nil {
		t.Fatal(err)
	}
	view := s.accountView(s.accounts[a.ID])
	if view.LastJobCreatedAt != record.View.CreatedAt || !view.LastJobAutomatic || view.LastJobIntent != "reauthorize" || view.LastJobPhase != "oauth" {
		t.Fatalf("account summary lost metadata outside global history: %+v", view)
	}
	stored.View = view
	if err := s.saveAccountLocked(stored); err != nil {
		t.Fatal(err)
	}
	saved := s.accounts[a.ID].View
	if saved.LastJobCreatedAt != "" || saved.LastJobAutomatic || saved.LastJobIntent != "" || saved.LastJobPhase != "" {
		t.Fatal("response-only metadata was stored in the account record")
	}
	s.mu.Lock()
	delete(s.jobs, record.View.ID)
	s.mu.Unlock()
	view = s.accountView(s.accounts[a.ID])
	if view.LastJobCreatedAt != "" || view.LastJobAutomatic || view.LastJobIntent != "" || view.LastJobPhase != "" {
		t.Fatal("missing historical job invented metadata")
	}
}
