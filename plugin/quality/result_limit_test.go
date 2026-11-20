package quality

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"
)

type controlAnswerHost struct{ *manualOnlyHost }

func (h *controlAnswerHost) Execute(_ context.Context, e Execution) Sample {
	h.executeCalls++
	return Sample{ExecutionStatus: "completed", Output: fmt.Sprintf("%s/%s/%d", e.AuthID, e.Model, h.executeCalls)}
}

func TestPartialCountPruningKeepsControlsForUnreachedSubjects(t *testing.T) {
	en, base, _ := manualOnlyFixture(t)
	host := &controlAnswerHost{base}
	en.host = host
	cfg := lifeConfig()
	cfg.Models, cfg.Samples = []string{"a", "b"}, 2
	cfg.ControlAuthID, cfg.ControlProvider = "auth-b", "offline"
	rule, err := en.SaveRule(context.Background(), Rule{RunConfig: cfg, Enabled: true, Action: "observe_only", MaxResults: 5}, false)
	if err != nil {
		t.Fatal(err)
	}
	runRound := func() *Run {
		t.Helper()
		r, err := en.RunRule(context.Background(), rule.ID)
		if err != nil {
			t.Fatal(err)
		}
		en.execute(<-en.queue)
		got, err := en.Run(r.ID)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	en.cfg.DailyCalls = 6 // All four controls and both model-a subjects can finish.
	partial := runRound()
	if partial.Error != "budget_exhausted" || len(partial.Controls) != 4 || len(partial.Samples) != 2 {
		t.Fatalf("unexpected partial round: %s, %d controls, %d subjects", partial.Error, len(partial.Controls), len(partial.Samples))
	}
	tomorrow := en.now().Add(24 * time.Hour)
	en.now = func() time.Time { return tomorrow }
	en.cfg.DailyCalls = 8
	complete := runRound()
	if len(complete.Samples) != 4 || complete.Status != "completed" {
		t.Fatal("expected a complete second round", complete.Status, len(complete.Samples))
	}
	var saved Run
	if err := en.store.Get("runs", partial.ID, &saved); err != nil {
		t.Fatal(err)
	}
	t.Logf("two rounds: %d synthetic calls, %d pause/release writes; partial subject records %d -> %d, control records %d -> %d",
		host.executeCalls, host.pauseCalls+host.releaseCalls, len(partial.Samples), len(saved.Samples), len(partial.Controls), len(saved.Controls))
	if len(saved.PrunedSlots) != 1 || saved.PrunedSlots[0] != (SampleSlot{Model: "a", Index: 0}) || len(saved.Samples) != 1 || saved.Samples[0].Output != partial.Samples[1].Output {
		t.Fatal("unexpected retained subject position", saved.PrunedSlots, saved.Samples)
	}
	if len(saved.Controls) != 3 {
		t.Fatalf("control answers for unreached model b were lost, retained %d of 3", len(saved.Controls))
	}
	for i, control := range saved.Controls {
		original := partial.Controls[i+1]
		if control.Model != original.Model || control.Index != original.Index || control.Output != original.Output {
			t.Fatal("control identity or output changed", control)
		}
	}
	if err := en.Review(partial.ID, 0, "incorrect", "stale"); err == nil {
		t.Fatal("trimmed subject still accepted review")
	}
	if err := en.Review(partial.ID, 1, "correct", "retained"); err != nil {
		t.Fatal(err)
	}
	detail, err := en.Run(partial.ID)
	if err != nil || detail.Samples[0].Review != "correct" || detail.Controls[0].Review != "" {
		t.Fatal("retained subject review changed another answer", err)
	}
	if err := en.DeleteManualRun(partial.ID); err == nil {
		t.Fatal("manual deletion removed rule evidence")
	}
	cleared, err := en.clearRuleHistory()
	if err != nil || cleared.Deleted != 2 || cleared.Kept != 0 {
		t.Fatal("explicit clear did not remove the completed history", cleared, err)
	}
	if err := en.Review(partial.ID, 1, "incorrect", "removed"); err == nil {
		t.Fatal("deleted run accepted review")
	}
}

func TestUnresolvedCredentialEvidenceSurvivesHistoryCleanup(t *testing.T) {
	for _, cleanup := range []string{"clear", "count", "age"} {
		t.Run(cleanup, func(t *testing.T) {
			f := managementTestFixture(t)
			f.answers["b"], f.statusFailure = "5", true
			rule := f.rule(t, disableCredential, true, true)
			runs, err := f.round(t, rule)
			if err != nil || len(runs) != 1 || runs[0].ActionStatus != "credential_action_uncertain" {
				t.Fatal("expected unconfirmed status write", runs, err)
			}
			owned := runs[0]
			en := f.engine
			later := cloneRun(owned)
			later.ID, later.ActionStatus = "later-run", "observe_only"
			now := en.now()
			later.CompletedAt = &now
			c := changes()
			c.runs[later.ID] = later
			nextRule := cloneRule(en.rules[rule.ID])
			nextRule.LastRun = later.ID
			if cleanup == "age" {
				nextRule.MaxResults, nextRule.RetentionDays = 0, 1
			} else {
				nextRule.MaxResults = len(later.Samples)
			}
			c.rules[rule.ID] = nextRule
			if err := en.commitLocked(c); err != nil {
				t.Fatal(err)
			}
			switch cleanup {
			case "clear":
				result, err := en.clearRuleHistory()
				if err != nil || result.Deleted != 1 || result.Kept != 1 {
					t.Fatal("cleanup must keep the handling record only", result, err)
				}
			case "count", "age":
				if err := en.Prune(now.AddDate(1, 0, 0)); err != nil {
					t.Fatal(err)
				}
			}
			var saved Run
			if err := en.store.Get("runs", owned.ID, &saved); err != nil || len(saved.Samples) != len(owned.Samples) || saved.Config.Question.Prompt == "" || saved.Samples[0].Output == "" {
				t.Fatalf("missing frozen evidence for %s: %v", cleanup, err)
			}
			history, err := en.History()
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, a := range history {
				if a.RunID == owned.ID && a.Kind == "credential_action_uncertain" {
					found = true
				}
			}
			if !found || len(f.patches) != 1 || en.managed["one"].RunID != owned.ID {
				t.Fatal("outstanding disposition lost its evidence or wrote again")
			}
		})
	}
}

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
	var saved Run
	if err := en.store.Get("runs", "edge", &saved); err != nil || len(saved.PrunedSlots) != 1 || saved.PrunedSlots[0] != (SampleSlot{Model: "a", Index: 0}) {
		t.Fatal("missing persisted slot for the trimmed answer", saved.PrunedSlots, err)
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

func TestPrunedHistoricalDuplicatesKeepOriginalReviewPositions(t *testing.T) {
	en, _, rule := scopeFixture(t)
	now := en.now()
	later := now.Add(time.Hour)
	edge := &Run{ID: "edge", Source: "scheduled_probe", Status: "completed", CompletedAt: &now, PrunedSamples: 1,
		Config:   Frozen{RuleID: rule.ID, RunConfig: RunConfig{AuthID: "A", Models: []string{"a", "b"}, Samples: 2}},
		Samples:  []Sample{{Model: "a", Index: 1, Output: "first"}, {Model: "a", Index: 1, Output: "duplicate"}, {Model: "b", Index: 1, Output: "last"}},
		Controls: []Sample{{Model: "a", Index: 1}, {Model: "b", Index: 1}, {Model: "b", Index: 0}},
	}
	newest := cloneRun(edge)
	newest.ID, newest.CompletedAt, newest.PrunedSamples = "new", &later, 0
	newest.Samples = newest.Samples[:2]
	en.runs[edge.ID], en.runs[newest.ID] = edge, newest
	for trim, limit := range []int{4, 3} {
		en.rules[rule.ID].MaxResults = limit
		if err := en.pruneResultCountsLocked(rule.ID, "A"); err != nil {
			t.Fatal(err)
		}
		var stored Run
		if err := en.store.Get("runs", edge.ID, &stored); err != nil || stored.PrunedSamples != 2+trim || len(stored.PrunedSlots) != 1+trim {
			t.Fatalf("legacy count and known coordinates after trim %d: %+v, %v", trim, stored.PrunedSlots, err)
		}
		if len(stored.Controls) != 3-trim || stored.Controls[0].Model != edge.Controls[trim].Model || stored.Controls[len(stored.Controls)-1].Index != 0 {
			t.Fatal("control for a duplicate or unreached subject was discarded", stored.Controls)
		}
		for _, slot := range stored.PrunedSlots {
			if slot != (SampleSlot{Model: "a", Index: 1}) {
				t.Fatal("invented or misidentified trimmed slot", slot)
			}
		}
		if err := en.Review(edge.ID, 1+trim, "correct", "stale"); err == nil {
			t.Fatal("review for removed record was accepted")
		}
		if err := en.Review(edge.ID, 3, "incorrect", "belongs to last"); err != nil {
			t.Fatal(err)
		}
		for _, sample := range en.runs[edge.ID].Samples {
			if (sample.ReviewNote == "belongs to last") != (sample.Output == "last") {
				t.Fatal("saved review did not match the visible answer", sample)
			}
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
