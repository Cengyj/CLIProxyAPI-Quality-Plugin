package login

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

func TestAccountAdmissionFailureDoesNotPersistOrStartJob(t *testing.T) {
	for _, action := range []string{"", "reset_totp", "logout_all"} {
		t.Run("action="+action, func(t *testing.T) {
			opts := offlineOptions(t)
			var calls atomic.Int32
			opts.Runtime = offlineRuntime{run: func(context.Context, RuntimeRequest) (string, error) {
				calls.Add(1)
				return "", errors.New("unexpected runtime start")
			}}
			s := offlineService(t, opts)
			a := importOfflineAccount(t, s)
			// Force the account write to fail inside an otherwise writable DB.
			if err := s.journal.db.Update(func(tx *bolt.Tx) error {
				b := tx.Bucket([]byte("accounts"))
				if err := b.Delete([]byte(a.ID)); err != nil {
					return err
				}
				_, err := b.CreateBucket([]byte(a.ID))
				return err
			}); err != nil {
				t.Fatal(err)
			}
			var err error
			if action == "" {
				_, err = s.StartAccount(a.ID, a.Revision, "")
			} else {
				_, err = s.StartAccountSecurity(a.ID, a.Revision, action, false, false)
			}
			if err == nil {
				t.Fatal("account write failure was ignored")
			}
			records, err := s.journal.load()
			if err != nil || len(records) != 0 {
				t.Errorf("failed admission left a durable job: count=%d error=%v", len(records), err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
			if calls.Load() != 0 {
				t.Fatal("runtime started without a durable account binding")
			}
		})
	}
}

func TestSecurityCommitsPreserveJobLogHistory(t *testing.T) {
	opts := offlineOptions(t)
	var s *Service
	opts.Runtime = offlineRuntime{run: func(_ context.Context, in RuntimeRequest) (string, error) {
		before, err := s.Logs(in.JobID)
		if err != nil || before.Legacy || len(before.Entries) < 2 {
			return "", errors.New("missing initial job log")
		}
		if err := in.factorVerified("old-factor"); err != nil {
			return "", err
		}
		after, err := s.Logs(in.JobID)
		if err != nil || after.Legacy || len(after.Entries) < len(before.Entries) || !reflect.DeepEqual(after.Entries[:len(before.Entries)], before.Entries) {
			t.Error("factor binding erased existing job logs")
		}
		v := alignedSecurityFixture(in)
		if err := writeSecurityFixture(in, v); err != nil {
			return "", err
		}
		if err := in.commitSecurity(); err != nil {
			return "", err
		}
		v["local_commit"], v["status"], v["phase"] = "committed", "succeeded", "security_complete"
		return "", writeSecurityFixture(in, v)
	}}
	s = offlineService(t, opts)
	a := importOfflineAccount(t, s)
	job, err := s.StartAccountSecurity(a.ID, a.Revision, "reset_totp", false, false)
	if err != nil {
		t.Fatal(err)
	}
	waitLogin(t, s, job.ID, "succeeded")
	logs, err := s.Logs(job.ID)
	if err != nil || logs.Legacy || len(logs.Entries) < 3 || logs.Entries[0].Status != "queued" {
		t.Fatalf("reset completion erased log history: entries=%d legacy=%v error=%v", len(logs.Entries), logs.Legacy, err)
	}
	for i, entry := range logs.Entries {
		if entry.ID != uint64(i+1) {
			t.Fatal("log cursor sequence changed after security commit")
		}
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = offlineService(t, opts)
	reopened, err := s.Logs(job.ID)
	if err != nil || !reflect.DeepEqual(logs, reopened) {
		t.Fatal("security log history changed after restart")
	}
}

func TestSecurityCapacityFailureKeepsUnresolvedState(t *testing.T) {
	s := offlineService(t, offlineOptions(t))
	a := importOfflineAccount(t, s)
	now := time.Now().UTC()
	record := storedJob{Created: now, Updated: now, SecurityStarted: true, View: Job{ID: "offline-capacity", AccountID: a.ID, Action: "reset_totp", Status: "security_uncertain", Revision: 1}}
	in := RuntimeRequest{Version: 1, JobID: record.View.ID, Attempt: 1, Operation: "reset_totp", Credentials: offlineStart().Credentials}
	in.SecurityJournalPath, _ = s.securityJournalPath(in.JobID, true)
	v := alignedSecurityFixture(in)
	v["local_commit"], v["status"] = "committed", "succeeded"
	if err := writeSecurityFixture(in, v); err != nil {
		t.Fatal(err)
	}
	if err := s.journal.save(record); err != nil {
		t.Fatal(err)
	}
	s.accountMu.Lock()
	stored := s.accounts[a.ID]
	stored.View.LastJobID, stored.View.SecurityPending = record.View.ID, true
	err := s.saveAccountLocked(stored)
	s.accountMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	j := &session{record: record}
	s.mu.Lock()
	s.jobs[record.View.ID] = j
	s.mu.Unlock()
	info, err := os.Stat(s.journal.db.Path())
	if err != nil {
		t.Fatal(err)
	}
	// Extend only the synthetic DB's unused tail to exercise the capacity gate.
	if err := os.Truncate(s.journal.db.Path(), journalMaxBytes); err != nil {
		t.Fatal(err)
	}
	err = s.reconcileSecurity(j)
	if restoreErr := os.Truncate(s.journal.db.Path(), info.Size()); restoreErr != nil {
		t.Fatal(restoreErr)
	}
	if err == nil {
		t.Fatal("capacity failure was ignored")
	}
	s.mu.Lock()
	unchanged := reflect.DeepEqual(j.record, record)
	blocked := securityBlocks(j.record)
	s.mu.Unlock()
	if !unchanged || !blocked {
		t.Fatal("failed persistence published resolved security state in memory")
	}
	if err := s.journal.db.View(func(tx *bolt.Tx) error {
		var saved storedJob
		if err := gob.NewDecoder(bytes.NewReader(tx.Bucket([]byte("jobs")).Get([]byte(record.View.ID)))).Decode(&saved); err != nil {
			return err
		}
		if saved.SecurityResolved || saved.View.TOTPReset {
			return errors.New("failed reconciliation altered durable security evidence")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCredentialBindingUpdatesAllMatchingVaultCopiesAtomically(t *testing.T) {
	for _, mode := range []string{"binding", "reset", "corrupt"} {
		t.Run(mode, func(t *testing.T) {
			s := offlineService(t, offlineOptions(t))
			a := importOfflineAccount(t, s)
			original := s.accounts[a.ID]
			record := storedJob{View: Job{ID: "offline-vault", Status: "running", Phase: "mfa_totp", AccountID: a.ID}}
			if err := s.journal.save(record); err != nil {
				t.Fatal(err)
			}
			initialLogs, err := s.journal.logs(record.View.ID)
			if err != nil {
				t.Fatal(err)
			}
			entries := map[string]vaultEntry{}
			for i := range 150 {
				c := original.Credentials
				c.Password = strings.Repeat("offline", 600)
				if i%3 == 1 {
					c.TOTPSecret = securityFixtureSeed
				} else if i%3 == 2 {
					c.Email = "unrelated@example.test"
				}
				entries[fmt.Sprintf("entry-%03d", i)] = vaultEntry{Credentials: c, Revision: 1}
			}
			if err := s.journal.db.Update(func(tx *bolt.Tx) error {
				for key, entry := range entries {
					if err := putGob(tx, "vault", key, entry); err != nil {
						return err
					}
				}
				if mode == "corrupt" {
					return tx.Bucket([]byte("vault")).Put([]byte("zz-invalid"), []byte("invalid-gob"))
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			next := original
			next.Credentials.TOTPFactorID = "verified-factor"
			if mode != "binding" {
				next.Credentials.TOTPSecret = securityFixtureSeed
			}
			s.accountMu.Lock()
			s.mu.Lock()
			err = s.persistCredentialBinding(next, record, original.Credentials.TOTPSecret, mode != "binding")
			s.mu.Unlock()
			s.accountMu.Unlock()
			if (err != nil) != (mode == "corrupt") {
				t.Fatalf("unexpected binding result: %v", err)
			}
			if err := s.journal.db.View(func(tx *bolt.Tx) error {
				var saved storedAccount
				if err := gob.NewDecoder(bytes.NewReader(tx.Bucket([]byte("accounts")).Get([]byte(a.ID)))).Decode(&saved); err != nil {
					return err
				}
				want := next.Credentials
				if mode == "corrupt" {
					want = original.Credentials
				}
				if saved.Credentials != want {
					return errors.New("account write was not atomic with vault copies")
				}
				for key, entry := range entries {
					var actual vaultEntry
					if err := gob.NewDecoder(bytes.NewReader(tx.Bucket([]byte("vault")).Get([]byte(key)))).Decode(&actual); err != nil {
						return err
					}
					if mode != "corrupt" && entry.Credentials.Email == original.Credentials.Email && (mode == "reset" || entry.Credentials.TOTPSecret == original.Credentials.TOTPSecret) {
						entry.Credentials.TOTPSecret, entry.Credentials.TOTPFactorID = next.Credentials.TOTPSecret, next.Credentials.TOTPFactorID
						entry.Revision++
					}
					if actual.Credentials != entry.Credentials || actual.Revision != entry.Revision {
						return fmt.Errorf("vault copy mismatch: %s", key)
					}
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			logs, err := s.journal.logs(record.View.ID)
			if err != nil || !reflect.DeepEqual(initialLogs, logs) {
				t.Fatal("credential transaction altered existing log history")
			}
		})
	}
}

func TestSecurityLocalCompletionUsesMaintenanceReserve(t *testing.T) {
	s := offlineService(t, offlineOptions(t))
	if err := os.Truncate(s.journal.db.Path(), journalMaxBytes-journalReserveBytes+1); err != nil {
		t.Fatal(err)
	}
	if err := s.journal.admit(); err == nil {
		t.Fatal("ordinary admissions consumed maintenance reserve")
	}
	called := false
	_, err := s.Idempotent("offline-complete-reserve", "POST/accounts/security/complete", []byte(`{}`), func() (any, error) {
		called = true
		return map[string]bool{"completed": true}, nil
	})
	if err != nil || !called {
		t.Fatalf("offline completion cannot use maintenance reserve: %v", err)
	}
}
