package quality

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestRuleResultPolicyAndLegacyEdits(t *testing.T) {
	en, _, r := scopeFixture(t)
	ctx := context.Background()
	if r.MaxResults != 100 || r.RetentionDays != 0 || r.Parameters.ReasoningEffort != "high" {
		t.Fatal("new rule defaults", r)
	}
	for _, count := range []int{-1, 1, 201} {
		in := *r
		in.MaxResults = count
		if _, err := en.SaveRule(ctx, in, true); err == nil {
			t.Fatal("invalid result cap", count)
		}
	}
	for _, effort := range []string{"none", "auto", "minimal"} {
		in := *r
		in.Parameters.ReasoningEffort = effort
		if _, err := en.SaveRule(ctx, in, true); err == nil {
			t.Fatal("accepted a retired effort on a current rule", effort)
		}
	}
	temp := 0.25
	old := en.rules[r.ID]
	old.Parameters.Temperature, old.Parameters.ReasoningEffort = &temp, "minimal"
	old.RetentionDays, old.MaxResults = 42, 0
	in := *old
	in.Parameters = Parameters{}
	in.RetentionDays, in.TargetScope = 0, ""
	saved, err := en.SaveRule(ctx, in, true)
	if err != nil || saved.MaxResults != 0 || saved.RetentionDays != 42 || saved.Parameters.Temperature == nil || *saved.Parameters.Temperature != temp || saved.Parameters.ReasoningEffort != "minimal" || saved.TargetScope != scopeAll {
		t.Fatal("unrelated edit changed legacy fields or demanded a target", saved, err)
	}
	in = *saved
	in.MaxResults, in.Parameters.ReasoningEffort = 17, "max"
	saved, err = en.SaveRule(ctx, in, true)
	if err != nil || saved.RetentionDays != 0 || saved.MaxResults != 17 || saved.Parameters.ReasoningEffort != "max" {
		t.Fatal("could not change legacy policy", saved, err)
	}
	for _, raw := range []string{`{"max_results":0}`, `{"max_results":null}`, `{"max_results":201}`, `{"max_results":1.5}`} {
		if _, err := decodeRuleWrite([]byte(raw), false); err == nil {
			t.Fatal("accepted invalid JSON limit", raw)
		}
	}
}

func TestResultLimitRetainsNewestSamplesPerRuleAndCredential(t *testing.T) {
	en, _, rule := scopeFixture(t)
	storeRun := func(id, ruleID, auth string, hour, count int, status string) *Run {
		t.Helper()
		tm := en.now().Add(time.Duration(hour) * time.Hour)
		r := &Run{ID: id, CreatedAt: tm, CompletedAt: &tm, Status: status, Source: "scheduled_probe", Config: Frozen{RuleID: ruleID, RunConfig: RunConfig{AuthID: auth, Models: []string{"a"}, Samples: count}}}
		if status == "running" {
			r.CompletedAt = nil
		}
		for i := range count {
			r.Samples = append(r.Samples, Sample{Index: i, Model: "a"})
		}
		r.Controls = cloneSamples(r.Samples)
		en.runs[id] = r
		if err := en.store.Put("runs", id, r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	en.rules[rule.ID].MaxResults = 3
	otherRule := cloneRule(rule)
	otherRule.ID = "another-rule"
	en.rules[otherRule.ID] = otherRule
	heldRule := cloneRule(rule)
	heldRule.ID, heldRule.ActionID, heldRule.MaxResults = "held-rule", "retained", 1
	en.rules[heldRule.ID] = heldRule
	storeRun("oldest", rule.ID, "A", 0, 2, "completed")
	storeRun("edge", rule.ID, "A", 1, 2, "completed")
	storeRun("newest", rule.ID, "A", 2, 2, "completed")
	storeRun("active", rule.ID, "A", 3, 1, "running")
	storeRun("other-account", rule.ID, "B", 0, 3, "completed")
	storeRun("other-rule", otherRule.ID, "A", 0, 3, "completed")
	storeRun("protected", heldRule.ID, "A", 0, 3, "completed")
	manual := storeRun("manual", "", "A", 0, 4, "completed")
	manual.Source = "manual_test"
	if err := en.store.Put("runs", manual.ID, manual); err != nil {
		t.Fatal(err)
	}
	if err := en.pruneResultCountsLocked(rule.ID, "A"); err != nil {
		t.Fatal(err)
	}
	if en.runs["manual"] == nil {
		t.Fatal("count prune included manual test")
	}
	if err := en.Prune(en.now().AddDate(0, 4, 0)); err != nil {
		t.Fatal(err)
	}
	if en.runs["oldest"] != nil || en.runs["manual"] != nil || !errors.Is(en.store.Get("runs", "oldest", &Run{}), os.ErrNotExist) {
		t.Fatal("trimmed history not consistent")
	}
	for id, count := range map[string]int{"edge": 1, "newest": 2, "other-account": 3, "other-rule": 3, "protected": 3, "active": 1} {
		var stored Run
		if r := en.runs[id]; r == nil || len(r.Samples) != count || en.store.Get("runs", id, &stored) != nil || len(stored.Samples) != count {
			t.Fatalf("%s: unexpected result count", id)
		}
	}
	edge := en.runs["edge"]
	if edge.PrunedSamples != 1 || len(edge.Controls) != 1 || edge.Samples[0].Index != 1 || edge.Controls[0].Index != 1 {
		t.Fatal("lost original ordinals", edge)
	}
	if e := en.Review("edge", 0, "correct", ""); e == nil {
		t.Fatal("stale review hit a different sample")
	}
	if e := en.Review("edge", 1, "correct", ""); e != nil || en.runs["edge"].Samples[0].Review != "correct" {
		t.Fatal("retained sample not reviewable", e)
	}
}

func TestAllEnabledDiscoversNewCredentialOnNextTick(t *testing.T) {
	en, h, r := scopeFixture(t)
	r.SchedulingEnabled = true
	r, err := en.SaveRule(context.Background(), *r, true)
	if err != nil {
		t.Fatal(err)
	}
	en.cfg.SchedulingEnabled = true
	for i, want := range []int{0, 1, 2} {
		if i == 1 {
			h.targets = []Target{scopedTarget("first", "a"), scopedTarget("later", "b")}
			h.targets[1].ManualDisabled = true
		}
		if i == 2 {
			h.targets[1].ManualDisabled = false
		}
		en.Tick(en.rules[r.ID].NextRun)
		batch := en.rules[r.ID].LastScopeBatch
		members := map[string]bool{}
		for _, child := range en.runs {
			if batch != "" && child.Config.ScopeBatchID == batch {
				members[child.Config.AuthID] = true
			}
		}
		if len(members) != want || i == 2 && !members["later"] {
			t.Fatal("stale enrollment", i, members)
		}
		for task := en.nextRunnableLocked(); task != nil; task = en.nextRunnableLocked() {
			en.executeTask(task)
		}
	}
}

func TestManualRetentionAndRuleDeletion(t *testing.T) {
	en, h, r := scopeFixture(t)
	ctx := context.Background()
	r.MaxResults, r.RetentionMode = 0, "manual"
	r, err := en.SaveRule(ctx, *r, true)
	if err != nil {
		t.Fatal(err)
	}
	in := *r
	in.RetentionMode = ""
	r, err = en.SaveRule(ctx, in, true)
	if err != nil || r.RetentionMode != "manual" {
		t.Fatal(r, err)
	}
	h.targets = []Target{scopedTarget("one", "a")}
	for range 2 {
		result, err := en.dispatchRule(ctx, r.ID, r.Revision, false)
		if err != nil || result.Count != 1 {
			t.Fatal(result, err)
		}
		if err := en.DeleteRule(ctx, r.ID, r.Revision); err == nil {
			t.Fatal("deleted active rule")
		}
		en.executeTask(en.nextRunnableLocked())
	}
	other := cloneRule(r)
	other.ID = ""
	other, err = en.SaveRule(ctx, *other, false)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := en.dispatchRule(ctx, other.ID, other.Revision, false)
	if err != nil || receipt.Count != 1 {
		t.Fatal(receipt, err)
	}
	task := en.nextRunnableLocked()
	en.executeTask(task)
	if err := en.Prune(en.now().AddDate(2, 0, 0)); err != nil || len(en.runs) != 3 {
		t.Fatal("manual retention expired", err)
	}
	ids := []string{}
	for id, run := range en.runs {
		if run.Config.RuleID == r.ID {
			ids = append(ids, id)
		}
		if err := en.store.Get("runs", id, &Run{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := en.DeleteRule(ctx, r.ID, r.Revision); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if err := en.store.Get("runs", id, &Run{}); !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	if len(en.runs) != 1 || en.runs[task.id] == nil {
		t.Fatal("wrong deletion scope")
	}
	other.RetentionMode, other.MaxResults = "count", 10
	other, err = en.SaveRule(ctx, *other, true)
	if err != nil || other.MaxResults != 10 {
		t.Fatal(other, err)
	}
}
