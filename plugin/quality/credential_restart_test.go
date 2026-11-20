package quality

import (
	"context"
	"testing"
)

func TestRestartReconcilesPendingCredentialWritesLocally(t *testing.T) {
	for _, phase := range []string{"disable_pending", "delete_pending", "restore_pending"} {
		t.Run(phase, func(t *testing.T) {
			f := managementTestFixture(t)
			en := f.engine
			policy := disableCredential
			if phase == "delete_pending" {
				policy = deleteCredential
			}
			r := f.rule(t, policy, true, policy == disableCredential)
			snapshot, err := f.host.CredentialSnapshot(context.Background(), "one", "one")
			if err != nil {
				t.Fatal(err)
			}
			at := en.now()
			run := &Run{ID: "interrupted-write", Source: "scheduled_probe", CreatedAt: at, CompletedAt: &at, Status: "completed", ActionStatus: phase,
				Config: Frozen{RuleID: r.ID, RunConfig: RunConfig{AuthID: "one"}}, Samples: []Sample{{Output: "frozen answer"}}}
			d := &CredentialDisposition{AuthID: "one", RuleID: r.ID, RunID: run.ID, ActionID: "original-intent", State: phase, Snapshot: snapshot, CreatedAt: at}
			c := changes()
			c.runs[run.ID], c.disposition = run, d
			if err := en.commitLocked(c); err != nil {
				t.Fatal(err)
			}
			for restart := 1; restart <= 2; restart++ {
				if err := en.Close(); err != nil {
					t.Fatal(err)
				}
				en, err = New(en.cfg, f.host)
				if err != nil {
					t.Fatal(err)
				}
				reopened := en
				t.Cleanup(func() { _ = reopened.Close() })
				got, err := en.Run(run.ID)
				if err != nil {
					t.Fatal(err)
				}
				if got.ActionStatus != "credential_action_uncertain" || got.CredentialAction != pendingCredentialAction(phase) || len(got.Samples) != 1 || got.Samples[0].Output != "frozen answer" {
					t.Fatalf("restart %d: expected retained evidence and an uncertain result, got %+v", restart, got)
				}
				marker := en.managed["one"]
				if marker.State != "attention" || marker.ActionID != d.ActionID || marker.Snapshot != snapshot {
					t.Fatal("lost original ownership or snapshot", marker)
				}
				audits, err := en.History()
				n := 0
				for _, a := range audits {
					if a.RunID == run.ID && a.ActionID == d.ActionID && a.Kind == "credential_action_uncertain" {
						n++
					}
				}
				result, clearErr := en.clearRuleHistory()
				if err != nil || clearErr != nil || n != 1 || result.Kept != 1 || len(f.patches)+f.deletes+f.probes != 0 {
					t.Fatalf("restart %d: audit=%d cleanup=%+v extra CPA calls=%d errors=%v %v", restart, n, result, len(f.patches)+f.deletes+f.probes, err, clearErr)
				}
			}
		})
	}
}
