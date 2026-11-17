package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

type fixtureQualityState struct {
	Actions      map[string]struct{ AuthID, ActionID string }
	Retired      map[string]struct{ AuthID, ActionID string }
	LoginCommits map[string]struct{ State, AuthID string }
}

func addQualityFixture(t *testing.T, f *fixture, mode string) {
	t.Helper()
	f.options.QualityState = filepath.Join(filepath.Dir(f.options.Source), "quality-state.db")
	f.options.QualityDB = filepath.Join(filepath.Dir(f.options.Destination), "quality.db")
	value := fixtureQualityState{
		Actions:      map[string]struct{ AuthID, ActionID string }{},
		Retired:      map[string]struct{ AuthID, ActionID string }{"old-release": {"fixture", "released"}},
		LoginCommits: map[string]struct{ State, AuthID string }{"old-success": {"committed", "fixture"}, "old-abort": {"aborted", "fixture"}},
	}
	switch mode {
	case "pause":
		value.Actions["new-pause"] = struct{ AuthID, ActionID string }{"fixture", "active"}
	case "committing", "commit_unknown", "saved_not_adopted", "unknown_commit":
		value.LoginCommits["pending"] = struct{ State, AuthID string }{mode, "fixture"}
	}
	db, err := bolt.Open(f.options.QualityState, 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		b, e := tx.CreateBucket([]byte("quality-v1"))
		if e != nil {
			return e
		}
		return b.Put([]byte("state"), gobBytes(t, value))
	})
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	db, err = bolt.Open(f.options.QualityDB, 0600, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		rules, e := tx.CreateBucket([]byte("rules"))
		if e != nil {
			return e
		}
		runs, e := tx.CreateBucket([]byte("runs"))
		if e != nil {
			return e
		}
		rule := `{"id":"fixture-rule","action_id":"","blocked_auth_id":""}`
		if mode == "action" {
			rule = `{"id":"fixture-rule","action_id":"new-action"}`
		}
		if mode == "blocked" {
			rule = `{"id":"fixture-rule","blocked_auth_id":"old-restriction"}`
		}
		if mode == "bad_rule" {
			rule = `{"action_id":"","action_id":"hidden"}`
		}
		if err := rules.Put([]byte("fixture-rule"), []byte(rule)); err != nil {
			return err
		}
		run := `{"status":"completed"}`
		switch mode {
		case "queued":
			run = `{"status":"queued"}`
		case "running":
			run = `{"status":"running"}`
		case "unknown_run":
			run = `{"status":"unknown"}`
		}
		return runs.Put([]byte("fixture-run"), []byte(run))
	})
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpgradeQualityGuardPassesOnlyTerminalUnrestrictedState(t *testing.T) {
	f := makeFixture(t)
	addQualityFixture(t, &f, "")
	beforeHost, _ := fileHash(f.options.QualityState)
	beforePlugin, _ := fileHash(f.options.QualityDB)
	r, err := upgrade(f.options)
	if err != nil {
		t.Fatal(err)
	}
	if !r.QualityChecked {
		t.Fatal("successful quality verification not reported")
	}
	afterHost, _ := fileHash(f.options.QualityState)
	afterPlugin, _ := fileHash(f.options.QualityDB)
	if beforeHost != afterHost || beforePlugin != afterPlugin {
		t.Fatal("quality verification changed authority data")
	}
}

func TestUpgradeQualityGuardRejectsRestrictionsAndPendingState(t *testing.T) {
	for _, mode := range []string{"pause", "committing", "commit_unknown", "saved_not_adopted", "unknown_commit", "action", "blocked", "bad_rule", "queued", "running", "unknown_run"} {
		t.Run(mode, func(t *testing.T) {
			f := makeFixture(t)
			addQualityFixture(t, &f, mode)
			before, _ := fileHash(f.options.Source)
			beforeHost, _ := fileHash(f.options.QualityState)
			beforePlugin, _ := fileHash(f.options.QualityDB)
			_, err := upgrade(f.options)
			if err == nil {
				t.Fatal("quality restriction did not reject conversion")
			}
			if _, err = os.Lstat(f.options.Destination); !os.IsNotExist(err) {
				t.Fatal("quality restriction published a target")
			}
			after, _ := fileHash(f.options.Source)
			afterHost, _ := fileHash(f.options.QualityState)
			afterPlugin, _ := fileHash(f.options.QualityDB)
			if before != after || beforeHost != afterHost || beforePlugin != afterPlugin {
				t.Fatal("rejection mutated source state")
			}
		})
	}
}

func TestQualityFlagsMustBePairedAndLocksStayHeld(t *testing.T) {
	f := makeFixture(t)
	addQualityFixture(t, &f, "")
	unpaired := f.options
	unpaired.QualityDB = ""
	if _, err := upgrade(unpaired); err == nil {
		t.Fatal("unpaired quality flag accepted")
	}
	closeQuality, err := checkQuality(f.options.QualityState, f.options.QualityDB)
	if err != nil {
		t.Fatal(err)
	}
	defer closeQuality()
	for _, path := range []string{f.options.QualityState, f.options.QualityDB} {
		db, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 20 * time.Millisecond})
		if err == nil {
			db.Close()
			t.Fatal("quality authority writer was admitted during guarded publication")
		}
	}
}
