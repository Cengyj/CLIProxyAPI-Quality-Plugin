package quality

import (
	"fmt"
	"strings"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

func TestAgeRetentionAgreesAcrossPruneAndReload(t *testing.T) {
	en, _ := lifeFixture(t)
	before := en.now().AddDate(0, 0, -en.cfg.RetentionDays)
	old := before.Add(-time.Second)
	if err := en.store.Write(func(tx *bolt.Tx) error {
		for i := range 66 {
			id := fmt.Sprintf("neighbor-%02d", i)
			at := old
			if i == 64 {
				at = before
			} else if i == 65 {
				at = time.Time{}
			}
			r := &Run{ID: id, Source: "manual_test", Status: "completed", CreatedAt: at, CompletedAt: &at}
			if err := put(tx, "runs", id, r); err != nil {
				return err
			}
			en.runs[id] = r
			if err := put(tx, "usage", id, UsageEvent{ID: id, Time: at}); err != nil {
				return err
			}
			if err := put(tx, "audit", id, Audit{ID: id, Time: at}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := en.Prune(en.now()); err != nil {
		t.Fatal(err)
	}
	if len(en.runs) != 2 || en.runs["neighbor-64"] == nil || en.runs["neighbor-65"] == nil {
		t.Errorf("live view retained %d runs, want the cutoff and unknown timestamps", len(en.runs))
	}
	en.runs = map[string]*Run{}
	if err := en.load(); err != nil {
		t.Fatal(err)
	}
	if len(en.runs) != 2 || en.runs["neighbor-64"] == nil || en.runs["neighbor-65"] == nil {
		t.Errorf("reloaded %d runs, want the cutoff and unknown timestamps", len(en.runs))
	}
	for _, bucket := range []string{"usage", "audit"} {
		kept := map[string]bool{}
		err := en.store.All(bucket, func(raw []byte) error {
			var record struct{ ID string }
			if err := decode(raw, &record); err != nil {
				return err
			}
			if strings.HasPrefix(record.ID, "neighbor-") {
				kept[record.ID] = true
			}
			return nil
		})
		if err != nil || len(kept) != 2 || !kept["neighbor-64"] || !kept["neighbor-65"] {
			t.Errorf("%s retained %v: %v", bucket, kept, err)
		}
	}
}
