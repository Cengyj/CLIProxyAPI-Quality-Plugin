package quality

import (
	"context"
	"testing"
	"time"
)

func TestSingleTimerIsScopedAndNeverRunsOnSave(t *testing.T) {
	en, h, sibling := scopeFixture(t, scopedTarget("one", "a", "b"))
	ctx := context.Background()
	sibling.SchedulingEnabled = true
	sibling, err := en.SaveRule(ctx, *sibling, true)
	if err != nil {
		t.Fatal(err)
	}
	oldDue := sibling.NextRun
	// The legacy global gate is closed; the sibling may even be overdue.
	en.now = func() time.Time { return oldDue.Add(time.Hour) }
	r := Rule{Name: "chosen", TargetScope: scopeAll, RunConfig: sibling.RunConfig}
	rule, err := en.saveRule(ctx, r, false, "start")
	if err != nil || !rule.timerActive || !rule.NextRun.After(en.now()) || en.cfg.SchedulingEnabled || h.executeCalls != 0 || len(en.runs) != 0 {
		t.Fatal(rule, err)
	}
	if err := en.UpdateConfig(en.Config()); err != nil || !en.timerAllowed(rule) {
		t.Fatal("saving unchanged settings revoked this rule's timer", err)
	}
	en.Tick(rule.NextRun)
	if len(en.runs) != 1 || en.rules[sibling.ID].NextRun != oldDue || en.rules[sibling.ID].Revision != sibling.Revision {
		t.Fatal("starting one rule changed its sibling")
	}
	for _, run := range en.runs {
		if run.Config.RuleID != rule.ID || !run.Config.Scheduled {
			t.Fatal(run.Config.RuleID)
		}
	}
	if err := en.setScheduling(false, false); err != nil {
		t.Fatal(err)
	}
	if en.timerAllowed(rule) || en.timerAllowed(sibling) {
		t.Fatal("instance stop left an individual timer active")
	}
	if d, err := en.dispatchRule(ctx, rule.ID, rule.Revision, true); err == nil || d.Count != 0 {
		t.Fatal("accepted a previously admitted schedule")
	}
	rule, err = en.saveRule(ctx, *rule, true, "keep")
	if err != nil || rule.timerActive {
		t.Fatal("editing a stopped rule rearmed it", err)
	}
	sibling, err = en.saveRule(ctx, *sibling, true, "start")
	if err != nil || !sibling.NextRun.After(en.now()) {
		t.Fatal("starting an overdue legacy plan replayed it", err)
	}
	sibling, err = en.saveRule(ctx, *sibling, true, "stop")
	if err != nil || sibling.timerActive || !sibling.NextRun.IsZero() || !sibling.Enabled {
		t.Fatal("turning off a timer should allow manual observation", err)
	}
}

func TestSingleTimerRejectsInvalidOrStaleStart(t *testing.T) {
	en, _, r := scopeFixture(t)
	ctx := context.Background()
	for _, mod := range []func(*Rule){func(r *Rule) { r.Cron = "65 * * * *" }, func(r *Rule) { r.Revision++ }} {
		in := *r
		mod(&in)
		if _, err := en.saveRule(ctx, in, true, "start"); err == nil || len(en.timers) != 0 {
			t.Fatal("failed edit armed a timer")
		}
	}
	if err := en.PauseRule(r.ID, r.Revision, true); err != nil {
		t.Fatal(err)
	}
	saved, err := en.saveRule(ctx, *cloneRule(en.rules[r.ID]), true, "keep")
	if err != nil || saved.Enabled || saved.timerActive {
		t.Fatal("unrelated edit resumed a stopped rule", err)
	}
	en.rules[r.ID].BlockedAuthID = "retained"
	if _, err := en.saveRule(ctx, *saved, true, "start"); err == nil || len(en.timers) != 0 {
		t.Fatal("start ignored a retained hold")
	}
}

func TestRuleListTimerPersistsAndRejectsStaleClicks(t *testing.T) {
	en, host, rule := scopeFixture(t, scopedTarget("one", "a", "b"))
	ctx := context.Background()
	on, err := en.setRuleTimer(ctx, rule.ID, rule.Revision, true)
	if err != nil || !on.timerActive || !on.NextRun.After(en.now()) || en.cfg.SchedulingEnabled {
		t.Fatal(on, err)
	}
	if _, err := en.setRuleTimer(ctx, rule.ID, rule.Revision, false); err == nil {
		t.Fatal("stale click stopped a newer rule")
	}
	off, err := en.setRuleTimer(ctx, on.ID, on.Revision, false)
	if err != nil || off.timerActive || !off.Enabled || !off.NextRun.IsZero() || off.Revision != on.Revision+1 {
		t.Fatal(off, err)
	}
	var stored Rule
	if err := en.store.Get("rules", off.ID, &stored); err != nil || stored.SchedulingEnabled || !stored.Enabled || stored.Revision != off.Revision {
		t.Fatal("stopped state was not persisted", err)
	}
	en.rules[off.ID].BlockedAuthID = "retained"
	if _, err := en.setRuleTimer(ctx, off.ID, off.Revision, true); err == nil || en.timers[off.ID] {
		t.Fatal("list switch bypassed a retained pause")
	}
	if host.executeCalls != 0 || len(en.runs) != 0 {
		t.Fatal("changing a timer dispatched a test")
	}
}
