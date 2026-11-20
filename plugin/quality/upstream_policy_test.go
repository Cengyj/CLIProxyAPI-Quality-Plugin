package quality

import (
	"context"
	"testing"
)

func TestErrorOnlyRuleIsIndependentOfQualityVerdict(t *testing.T) {
	f := managementTestFixture(t)
	r := f.rule(t, "observe_only", true, false)
	r.UpstreamErrorActions = map[string]string{"unauthorized": disableCredential, "payment_required": deleteCredential}
	r.Samples = 2
	r, err := f.engine.SaveRule(context.Background(), *r, true)
	if err != nil {
		t.Fatal(err)
	}
	f.answers["a"] = "5"
	runs, err := f.round(t, r)
	if err != nil || len(runs) != 1 || runs[0].ActionStatus != "observe_only" || len(runs[0].Samples) != 4 || len(f.patches) != 0 || f.deletes != 0 {
		t.Fatalf("wrong answer applied an upstream condition: %v, %v", runs, err)
	}
	f.upstreamStatus = 401
	runs, err = f.round(t, r)
	if err != nil || len(runs) != 1 || len(runs[0].Samples) != 1 || runs[0].UpstreamCondition != "unauthorized" || runs[0].ActionStatus != "credential_suspended" || len(f.patches) != 1 || f.deletes != 0 {
		t.Fatalf("expected one classified probe and one disable: %v, %v", runs, err)
	}
	f.add("two")
	f.upstreamStatus = 402
	runs, err = f.round(t, r)
	if err != nil || len(runs) != 1 || runs[0].Config.AuthID != "two" || runs[0].ActionStatus != "credential_removed" || len(f.patches) != 1 || f.deletes != 1 {
		t.Fatalf("unacknowledged prior suspension blocked a new credential: %v, %v", runs, err)
	}
	if !f.engine.rules[r.ID].Enabled || f.engine.rules[r.ID].Revision != r.Revision || !f.entries["one"].Disabled {
		t.Fatal("a confirmed error action changed the rule or previous credential")
	}
}

func TestUpstreamActionsPreserveOmittedSettingsAndRejectInvalidInput(t *testing.T) {
	f := managementTestFixture(t)
	r := f.rule(t, "observe_only", true, false)
	if managedRule(r) {
		t.Fatal("default observation selected management probes")
	}
	update := func(settings map[string]string) error {
		next := *r
		next.UpstreamErrorActions = settings
		saved, err := f.engine.SaveRule(context.Background(), next, true)
		if err == nil {
			r = saved
		}
		return err
	}
	for _, invalid := range []map[string]string{{"http_400": disableCredential}, {"unauthorized": "suspend"}, {"quota_exhausted": ""}} {
		if err := update(invalid); err == nil {
			t.Fatal("invalid actions accepted", invalid)
		}
	}
	if err := update(map[string]string{"quota_exhausted": deleteCredential}); err != nil {
		t.Fatal(err)
	}
	if err := update(nil); err != nil || r.UpstreamErrorActions["quota_exhausted"] != deleteCredential {
		t.Fatal("legacy editor reset actions", err)
	}
	if err := update(map[string]string{"quota_exhausted": "observe_only"}); err != nil || managedRule(r) {
		t.Fatal("explicit clear retained a destructive action", err)
	}
}

func TestScheduledUpstreamActionContinuesWithoutAcknowledgement(t *testing.T) {
	f := managementTestFixture(t)
	r := f.rule(t, "observe_only", true, false)
	r.Cron = "*/5 * * * *"
	r.UpstreamErrorActions = map[string]string{"unauthorized": disableCredential}
	r, err := f.engine.saveRule(context.Background(), *r, true, "start")
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range []string{"one", "two"} {
		f.upstreamStatus = 401
		if i == 1 {
			f.add(id)
			// Changing a future error action must not require marking the
			// previous confirmed suspension or grant authority to delete it.
			r.UpstreamErrorActions["unauthorized"] = deleteCredential
			r, err = f.engine.saveRule(context.Background(), *r, true, "keep")
			if err != nil {
				t.Fatal(err)
			}
		}
		f.engine.Tick(f.engine.rules[r.ID].NextRun)
		task := f.engine.nextRunnableLocked()
		if task == nil || !task.frozen.Scheduled || task.frozen.AuthID != id {
			t.Fatalf("missing scheduled probe for %s", id)
		}
		f.engine.executeTask(task)
		if !f.engine.timerAllowed(f.engine.rules[r.ID]) || len(f.engine.queue) != 0 {
			t.Fatal("scheduled rule stopped or retried a handled credential")
		}
	}
	if len(f.patches) != 1 || f.deletes != 1 || !f.entries["one"].Disabled || f.probes != 2 {
		t.Fatalf("unexpected mutations, disables=%v deletes=%d probes=%d", f.patches, f.deletes, f.probes)
	}
}

func TestUpstreamDecisionRechecksCancelledOrStaleRun(t *testing.T) {
	for _, scenario := range []string{"cancelled", "revision", "snapshot", "nonmatching policy", "changed probe", "native path"} {
		t.Run(scenario, func(t *testing.T) {
			f := managementTestFixture(t)
			r := f.rule(t, "observe_only", false, false)
			r.UpstreamErrorActions = map[string]string{"unauthorized": deleteCredential}
			r, err := f.engine.SaveRule(context.Background(), *r, true)
			if err != nil {
				t.Fatal(err)
			}
			snap, err := f.host.CredentialSnapshot(context.Background(), "one", "one")
			if err != nil {
				t.Fatal(err)
			}
			cfg := Frozen{RuleID: r.ID, RuleRevision: r.Revision, ManagedProbe: &snap, RunConfig: r.RunConfig, UpstreamErrorActions: r.UpstreamErrorActions}
			row := &Run{ID: "fixture", Source: "scheduled_probe", Status: "completed", Config: cfg, Samples: []Sample{{Model: "a", ExecutionStatus: "failed", QualityVerdict: "unknown", UpstreamCondition: "unauthorized", Evidence: Evidence{Path: "cpa_management_probe", AuthID: "one", RequestedModel: "a", PayloadFingerprint: "fixture"}}}}
			switch scenario {
			case "cancelled":
				row.CancelRequested = true
			case "revision":
				row.Config.RuleRevision--
			case "snapshot":
				f.changedContent = true
			case "nonmatching policy":
				row.Config.UpstreamErrorActions = map[string]string{"unauthorized": disableCredential}
			case "changed probe":
				row.Samples[0].Evidence.ConditionsChanged = true
			case "native path":
				row.Samples[0].Evidence.Path = "native"
			}
			f.engine.mu.Lock()
			f.engine.decideLocked(row)
			f.engine.mu.Unlock()
			if f.deletes != 0 || len(f.patches) != 0 {
				t.Fatalf("unconfirmed response changed CPA: %v, %v", f.deletes, f.patches)
			}
		})
	}
}
