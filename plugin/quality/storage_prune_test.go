package quality

import (
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

func TestAgePrunePreservesHistoricalPauseEvidenceOnReload(t *testing.T) {
	en, h := lifeFixture(t)
	old := en.now().AddDate(0, 0, -60)
	rules := []*Rule{
		{ID: "owner", ActionID: "pause", LastRun: "owner-last", RetentionDays: 7},
		{ID: "dependent", BlockedAuthID: "auth-a", RetentionDays: 7},
		{ID: "ordinary", RetentionDays: 7},
	}
	c := changes()
	for _, rule := range rules {
		c.rules[rule.ID] = rule
	}
	for _, item := range []struct{ id, rule string }{
		{"owner-first", "owner"}, {"owner-evidence", "owner"}, {"owner-last", "owner"},
		{"dependent-first", "dependent"}, {"ordinary-first", "ordinary"},
	} {
		c.runs[item.id] = &Run{ID: item.id, Source: "scheduled_probe", Status: "completed", CreatedAt: old, CompletedAt: &old, Config: Frozen{RuleID: item.rule}, Samples: []Sample{{Output: "retained answer"}}}
		c.audits = append(c.audits, Audit{ID: item.id, RunID: item.id, RuleID: item.rule, Kind: "result", Time: old})
	}
	c.intent = &Action{OwnerPlugin: "model-quality", RuleID: "owner", AuthID: "auth-a", ActionID: "pause", EvidenceRunID: "owner-evidence"}
	c.audits = append(c.audits,
		Audit{ID: "pause-adopted", ActionID: "pause", RuleID: "owner", Kind: "reconciled_pause", Time: old},
		Audit{ID: "old-released-action", ActionID: "different", RuleID: "owner", Kind: "manual_release", Time: old},
		Audit{ID: "standalone", Time: old},
	)
	if err := en.commitLocked(c); err != nil {
		t.Fatal(err)
	}
	if err := en.Prune(en.now()); err != nil {
		t.Fatal(err)
	}
	for phase := range 2 {
		if phase == 1 {
			if err := en.store.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := OpenStore(en.cfg.DataDir, en.cfg.MaxStorageBytes)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := reopened.Close(); err != nil {
					t.Error(err)
				}
			})
			en.store = reopened
			en.runs, en.rules = map[string]*Run{}, map[string]*Rule{}
			if err := en.load(); err != nil {
				t.Fatal(err)
			}
		}
		for _, id := range []string{"owner-first", "owner-evidence", "owner-last", "dependent-first"} {
			if r := en.runs[id]; r == nil || len(r.Samples) != 1 || r.Samples[0].Output != "retained answer" {
				t.Errorf("phase %d lost historical run %s", phase, id)
			}
			if err := en.store.Get("audit", id, &Audit{}); err != nil {
				t.Errorf("phase %d lost audit %s: %v", phase, id, err)
			}
		}
		if err := en.store.Get("audit", "pause-adopted", &Audit{}); err != nil {
			t.Errorf("phase %d lost the active action audit: %v", phase, err)
		}
		for _, id := range []string{"old-released-action", "standalone", "ordinary-first"} {
			if err := en.store.Get("audit", id, &Audit{}); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("phase %d retained unrelated audit %s: %v", phase, id, err)
			}
		}
		if en.runs["ordinary-first"] != nil || en.rules["owner"].ActionID != "pause" || en.rules["dependent"].BlockedAuthID != "auth-a" || h.pauses != 0 || len(h.calls) != 0 {
			t.Errorf("phase %d: unrelated run, ownership or host state changed", phase)
		}
	}
}

func TestAgePrunePreservesSurvivingUncertainAudit(t *testing.T) {
	en, _ := lifeFixture(t)
	old := en.now().AddDate(0, 0, -60)
	c := changes()
	c.disposition = &CredentialDisposition{AuthID: "auth-a", RuleID: "old-rule", RunID: "no-longer-present", ActionID: "attempt", State: "attention"}
	c.audits = []Audit{{ID: "remaining-evidence", RuleID: "old-rule", RunID: "no-longer-present", ActionID: "attempt", Kind: "credential_action_uncertain", Time: old}}
	if err := en.commitLocked(c); err != nil {
		t.Fatal(err)
	}
	if err := en.Prune(en.now()); err != nil {
		t.Fatal(err)
	}
	if err := en.store.Get("audit", "remaining-evidence", &Audit{}); err != nil {
		t.Fatalf("unresolved credential write lost its surviving audit: %v", err)
	}
}

func TestAgePruneHonorsStorageFault(t *testing.T) {
	en, _ := lifeFixture(t)
	old := en.now().AddDate(0, 0, -60)
	run := &Run{ID: "history", CompletedAt: &old}
	if err := en.store.Put("runs", run.ID, run); err != nil {
		t.Fatal(err)
	}
	en.runs[run.ID] = run
	en.fault = errors.New("uncertain persisted state")
	if err := en.Prune(en.now()); err == nil {
		t.Error("cleanup wrote after persistence fault")
	}
	if en.runs[run.ID] == nil || en.store.Get("runs", run.ID, &Run{}) != nil {
		t.Error("lost history during persistence fault")
	}
}

func TestAgePruneRollbackAndMixedPageBoundaries(t *testing.T) {
	en, _ := lifeFixture(t)
	old := en.now().AddDate(0, 0, -60)
	wantKept := 0
	if err := en.store.Write(func(tx *bolt.Tx) error {
		for i := range 400 {
			id := fmt.Sprintf("page-%04d", i)
			at := old
			if i%7 == 5 || i%7 == 6 {
				at = en.now()
				wantKept++
			}
			r := &Run{ID: id, Status: "completed", CreatedAt: at, CompletedAt: &at}
			if err := put(tx, "runs", id, r); err != nil {
				return err
			}
			if err := put(tx, "usage", id, UsageEvent{ID: id, Time: at}); err != nil {
				return err
			}
			if err := put(tx, "audit", id, Audit{ID: id, RunID: id, Time: at}); err != nil {
				return err
			}
			en.runs[id] = r
		}
		return tx.Bucket([]byte("audit")).Put([]byte("zz-malformed"), []byte("{"))
	}); err != nil {
		t.Fatal(err)
	}
	if err := en.Prune(en.now()); err == nil || len(en.runs) != 400 || en.store.Get("runs", "page-0000", &Run{}) != nil || en.store.Get("usage", "page-0000", &UsageEvent{}) != nil {
		t.Fatal("failed retention published a partial deletion", err)
	}
	if err := en.store.Write(func(tx *bolt.Tx) error { return tx.Bucket([]byte("audit")).Delete([]byte("zz-malformed")) }); err != nil {
		t.Fatal(err)
	}
	if err := en.Prune(en.now()); err != nil {
		t.Fatal(err)
	}
	if len(en.runs) != wantKept {
		t.Fatalf("unexpected retained count %d, want %d", len(en.runs), wantKept)
	}
	for i := range 400 {
		id := fmt.Sprintf("page-%04d", i)
		want := i%7 == 5 || i%7 == 6
		for _, bucket := range []string{"runs", "usage", "audit"} {
			var record struct{ Time time.Time }
			err := en.store.Get(bucket, id, &record)
			if err != nil && !errors.Is(err, os.ErrNotExist) || (err == nil) != want {
				t.Fatalf("%s/%s wanted kept=%t, got %v", bucket, id, want, err)
			}
		}
	}
}

func TestBudgetReservePrunesAdjacentDaysAndKeepsUnfinishedCalls(t *testing.T) {
	en, _ := lifeFixture(t)
	en.cfg.RetentionDays = 1
	currentDay := en.now().UTC().Format(time.DateOnly)
	pendingDay := en.now().AddDate(0, 0, -5).UTC().Format(time.DateOnly)
	en.budgetPending = map[string]int{pendingDay: 1}
	var settled []string
	if err := en.store.Write(func(tx *bolt.Tx) error {
		for i := 12; i >= 1; i-- {
			day := en.now().AddDate(0, 0, -i).UTC().Format(time.DateOnly)
			if day != pendingDay {
				settled = append(settled, day)
			}
			if err := put(tx, "meta", "budget:"+day, Budget{Day: day, Calls: 1, ReservedTokens: 16}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := en.reserve(32); err != nil {
		t.Fatal(err)
	}
	for _, day := range settled {
		if err := en.store.Get("meta", "budget:"+day, &Budget{}); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("settled day %s was not deleted: %v", day, err)
		}
	}
	if err := en.store.Get("meta", "budget:"+pendingDay, &Budget{}); err != nil {
		t.Fatal("lost the unfinished day", err)
	}
	if err := en.account(pendingDay, &Usage{Total: 9}); err != nil {
		t.Fatal(err)
	}
	if err := en.store.Get("meta", "budget:"+pendingDay, &Budget{}); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("completed old day was not released", err)
	}
	var active Budget
	if err := en.store.Get("meta", "budget:"+currentDay, &active); err != nil || active.Calls != 1 || active.ReservedTokens != 32 || active.KnownTokens != 0 || en.budget != active {
		t.Fatalf("current day changed on settlement: %+v, %v", active, err)
	}
}

func TestBudgetReserveWriteFailureRollsBackAndBlocksCleanup(t *testing.T) {
	en, _ := lifeFixture(t)
	en.cfg.RetentionDays = 1
	old := en.now().AddDate(0, 0, -7).UTC().Format(time.DateOnly)
	day := en.now().UTC().Format(time.DateOnly)
	if err := en.store.Write(func(tx *bolt.Tx) error {
		if err := put(tx, "meta", "budget:"+old, Budget{Day: old, Calls: 1}); err != nil {
			return err
		}
		// The active record write fails after writing the new per-day row.
		_, err := tx.Bucket([]byte("meta")).CreateBucket([]byte("budget"))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := en.reserve(16); err == nil || en.fault == nil {
		t.Fatal("expected a failed durable reservation")
	}
	if en.budget.Calls != 0 || len(en.budgetPending) != 0 || !errors.Is(en.store.Get("meta", "budget:"+day, &Budget{}), os.ErrNotExist) || en.store.Get("meta", "budget:"+old, &Budget{}) != nil {
		t.Fatal("failed reservation changed the ledger or in-flight calls")
	}
	if err := en.Prune(en.now()); err == nil {
		t.Fatal("retention ignored the persistence fault")
	}
}
