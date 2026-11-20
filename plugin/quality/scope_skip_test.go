package quality

import (
	"context"
	"errors"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestScopeDispatchPersistsEveryCountedSkip(t *testing.T) {
	ctx := context.Background()
	f := managementTestFixture(t)
	f.add("partial")
	f.add("kimi")
	for id, entry := range f.entries {
		entry.Email = id + "@example.test"
		switch id {
		case "one":
			entry.Disabled, entry.Status = true, "disabled"
		case "kimi":
			entry.Provider = "kimi"
		}
		f.entries[id] = entry
	}
	r := f.rule(t, disableCredential, true, false)
	r.TargetScope = scopeAllCredentials
	r, err := f.engine.SaveRule(ctx, *r, true)
	if err != nil {
		t.Fatal(err)
	}
	targets, err := f.host.Targets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i := range targets {
		if targets[i].AuthID == "partial" {
			targets[i].Models = []string{"a"}
		}
	}
	receipt, err := f.engine.dispatchWithInventory(ctx, r.ID, r.Revision, false, func(context.Context) ([]Target, error) { return targets, nil })
	if err != nil || receipt.Count != 2 || receipt.Skipped != 1 || len(receipt.SkipDetails) != 1 || receipt.BatchID == "" {
		t.Fatal(receipt, err)
	}
	if receipt.SkipDetails[0].AuthID != "one" || receipt.SkipDetails[0].Code != "manual_disabled" {
		t.Fatal(receipt)
	}
	for task := f.engine.nextRunnableLocked(); task != nil; task = f.engine.nextRunnableLocked() {
		f.engine.executeTask(task)
	}
	codes := map[string]string{"one": "manual_disabled", "partial": "missing_model", "kimi": "unsupported_provider"}
	for _, run := range f.engine.runs {
		detail := run.PreflightDetail
		if detail == nil {
			detail = run.SkipDetail
		}
		if detail == nil || detail.Code != codes[run.Config.AuthID] || detail.RunID != run.ID || detail.Email != run.Config.AuthID+"@example.test" || detail.Reason == "" {
			t.Fatal("missing check evidence", run)
		}
		want := map[string]string{"one": "skipped", "partial": "completed", "kimi": "failed"}[run.Config.AuthID]
		if run.Status != want || len(run.Samples) != 0 || run.CompletedAt == nil || run.Config.Question.Prompt == "" || !reflect.DeepEqual(run.Config.Models, r.Models) || run.Config.ScopeBatchID != receipt.BatchID {
			t.Fatal("invalid frozen result", run)
		}
		copy, err := f.engine.Run(run.ID)
		if err != nil {
			t.Fatal(err)
		}
		if copy.PreflightDetail != nil {
			copy.PreflightDetail.Reason = "changed read copy"
		} else {
			copy.SkipDetail.Reason = "changed read copy"
		}
		if detail.Reason == "changed read copy" {
			t.Fatal("read aliased evidence")
		}
		delete(codes, run.Config.AuthID)
	}
	if len(codes) != 0 || f.probes != 0 || f.nativeCalls != 0 || len(f.patches) != 0 || f.deletes != 0 {
		t.Fatal(codes, "preflight called or wrote")
	}
	page, err := f.engine.QueryRuns(url.Values{"view": {"summary"}, "rule_id": {r.ID}, "scope_batch_id": {receipt.BatchID}})
	if err != nil || page["total"] != 3 {
		t.Fatal(page, err)
	}
	for _, item := range page["items"].([]map[string]any) {
		detail, _ := item["preflight_detail"].(*ScopeSkipDetail)
		if detail == nil {
			detail, _ = item["skip_detail"].(*ScopeSkipDetail)
		}
		if detail == nil || detail.RunID != item["id"] {
			t.Fatal(item)
		}
		detail.Reason = "changed list copy"
	}
	metrics, err := f.engine.metrics(true)
	if err != nil || metrics["planned"] != 0 || metrics["unknown"] != 0 || metrics["covered"] != 0 || metrics["quality_pass_rate"] != nil {
		t.Fatal("checks counted as answers", metrics, err)
	}
	original := cloneScopeDispatch(f.engine.rules[r.ID].LastScopeDispatch)
	if original == nil || original.Matched != 2 || original.Skipped != 1 || len(original.Details) != 1 {
		t.Fatal(original)
	}
	display := f.engine.Rules()[0]
	display.LastScopeDispatch.Details[0].Reason = "mutated display"
	display.LastScopeDispatch.Matched = 91
	updated, err := f.engine.SaveRule(ctx, display, true)
	if err != nil || !reflect.DeepEqual(updated.LastScopeDispatch, original) {
		t.Fatal(updated, err)
	}
	f.engine.rules, f.engine.runs = map[string]*Rule{}, map[string]*Run{}
	if err := f.engine.load(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(f.engine.rules[r.ID].LastScopeDispatch, original) || len(f.engine.runs) != 3 {
		t.Fatal("lost history on reload")
	}
	for _, run := range f.engine.runs {
		detail := run.PreflightDetail
		if detail == nil {
			detail = run.SkipDetail
		}
		if detail.Reason == "changed list copy" {
			t.Fatal("summary aliased persisted detail")
		}
	}
	cleared, err := f.engine.clearRuleHistory()
	if err != nil || cleared.Deleted != 3 || cleared.Kept != 0 {
		t.Fatal(cleared, err)
	}
	var stored Rule
	if err := f.engine.store.Get("rules", r.ID, &stored); err != nil || stored.LastScopeBatch != "" || stored.LastScopeDispatch.Skipped != 1 {
		t.Fatal(stored, err)
	}
	for _, d := range stored.LastScopeDispatch.Details {
		if d.RunID != "" || d.Reason == "" {
			t.Fatal(d)
		}
	}
}

func TestScheduledZeroMatchPersistsSkipDetails(t *testing.T) {
	f := managementTestFixture(t)
	en := f.engine
	r := f.rule(t, disableCredential, true, true)
	r, err := en.setRuleTimer(context.Background(), r.ID, r.Revision, true)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := f.host.CredentialSnapshot(context.Background(), "one", "one")
	if err != nil {
		t.Fatal(err)
	}
	entry := f.entries["one"]
	entry.Disabled, entry.Status = true, "disabled"
	f.entries["one"] = entry
	snap.Disabled, snap.UpdatedAt = true, snap.UpdatedAt.Add(-time.Second)
	en.managed["one"] = &CredentialDisposition{AuthID: "one", RuleID: r.ID, State: "disabled", Models: r.Models, Snapshot: snap}
	en.Tick(r.NextRun)
	var saved Rule
	if err := en.store.Get("rules", r.ID, &saved); err != nil {
		t.Fatal(err)
	}
	summary := saved.LastScopeDispatch
	if summary == nil || summary.Matched != 0 || summary.Skipped != 1 || len(summary.Details) != 1 ||
		summary.Details[0].Code != "snapshot_changed" || !strings.Contains(summary.Details[0].Reason, "运行更新时间") ||
		!saved.NextRun.After(r.NextRun) || len(en.runs) != 1 || f.probes != 0 || len(f.patches) != 0 {
		t.Fatal("unexpected scheduled scan", summary, saved.NextRun, r.NextRun)
	}
	run, err := en.Run(summary.Details[0].RunID)
	if err != nil || !run.Config.Scheduled || run.CompletedAt == nil || run.Status != "skipped" || run.Config.ScopeBatchID != saved.LastScopeBatch {
		t.Fatal("missing durable scheduled skip", run, err)
	}
}

func TestScopeExplainsDirectorySnapshotDisagreement(t *testing.T) {
	en := &Engine{}
	r := &Rule{Action: disableCredential, AutoRestore: true, TargetScope: scopeAllCredentials}
	for _, disabled := range []bool{true, false} {
		target := scopedTarget("discordant", "a")
		target.ManualDisabled = !disabled
		if !disabled {
			target.RuntimeStatus = "disabled"
		}
		_, _, err := en.admitSnapshotLocked(r, target, CredentialSnapshot{Disabled: disabled}, false)
		var rejected *scopeRejection
		if !errors.As(err, &rejected) || rejected.detail.Code != "snapshot_runtime_conflict" ||
			!strings.Contains(rejected.detail.Reason, "凭证快照") || !strings.Contains(rejected.detail.Reason, target.RuntimeStatus) {
			t.Fatal(target, err)
		}
	}
}

func TestAllEnabledScopeSkipCountsAndLastSuccessfulScan(t *testing.T) {
	manual := scopedTarget("manual", "a")
	manual.ManualDisabled, manual.RuntimeStatus = true, "disabled"
	held := scopedTarget("held", "a")
	held.RetainedPause = true
	cooldown := scopedTarget("cooldown", "a", "b")
	cooldown.EffectiveAvailable, cooldown.Reasons = false, []string{"native_cooldown"}
	en, h, r := scopeFixture(t, manual, held, cooldown, scopedTarget("unregistered", "other"))
	ctx := context.Background()
	receipt, err := en.dispatchRule(ctx, r.ID, r.Revision, false)
	if err != nil || receipt.Count != 3 || receipt.Skipped != 0 || len(receipt.SkipDetails) != 0 {
		t.Fatal(receipt, err)
	}
	for task := en.nextRunnableLocked(); task != nil; task = en.nextRunnableLocked() {
		en.executeTask(task)
	}
	seen := map[string]bool{}
	for _, run := range en.runs {
		seen[run.Config.AuthID] = true
		switch run.Config.AuthID {
		case "held":
			if run.Status != "blocked" || run.PreflightDetail.Code != "quality_paused" {
				t.Fatal(run)
			}
		case "unregistered":
			if run.Status != "completed" || run.PreflightDetail.Code != "missing_model" || len(run.Samples) != 0 {
				t.Fatal(run)
			}
		case "cooldown":
			if run.Status != "completed" || len(run.Samples) != 2 {
				t.Fatal(run)
			}
		}
	}
	if seen["manual"] || len(seen) != 3 || h.executeCalls != 2 {
		t.Fatal(seen, h.executeCalls)
	}
	old := cloneScopeDispatch(en.rules[r.ID].LastScopeDispatch)
	if _, err := en.dispatchRule(ctx, r.ID, r.Revision+1, false); err == nil || !reflect.DeepEqual(en.rules[r.ID].LastScopeDispatch, old) {
		t.Fatal("failed dispatch replaced summary")
	}
	h.targets = nil
	en.now = func() time.Time { return old.At.Add(time.Hour) }
	if empty, err := en.dispatchRule(ctx, r.ID, r.Revision, false); err != nil || empty.Count != 0 || empty.Skipped != 0 {
		t.Fatal(empty, err)
	}
	var stored Rule
	if err := en.store.Get("rules", r.ID, &stored); err != nil || stored.LastScopeDispatch.At != en.now() || stored.LastScopeDispatch.Matched != 0 {
		t.Fatal(stored, err)
	}
}

func TestScopeSkipNeverIncludesProviderErrorText(t *testing.T) {
	const private = "synthetic-private-material"
	var receipt ruleDispatch
	receipt.skipManaged(Target{AuthID: "fake"}, errors.New(private))
	for legacy := range receipt.SkipReasons {
		if strings.Contains(legacy, private) {
			t.Fatal("provider error was exported in the legacy count")
		}
	}
	if len(receipt.SkipDetails) != 1 || receipt.SkipDetails[0].Code != "snapshot_unavailable" || strings.Contains(receipt.SkipDetails[0].Reason, private) {
		t.Fatal(receipt)
	}
}

func TestScheduledKnownCredentialsWithoutManagementSession(t *testing.T) {
	for _, scope := range []string{"single", scopeAll} {
		t.Run(scope, func(t *testing.T) {
			ctx := context.Background()
			f := managementTestFixture(t)
			f.add("two")
			r := f.rule(t, disableCredential, scope == scopeAll, false)
			r, err := f.engine.setRuleTimer(ctx, r.ID, r.Revision, true)
			if err != nil {
				t.Fatal(err)
			}
			f.host.Management.key = ""
			unknown := func(context.Context) ([]Target, error) { return nil, errors.New("synthetic inventory failure") }
			if _, err := f.engine.dispatchWithInventory(ctx, r.ID, r.Revision, true, unknown); err == nil || len(f.engine.runs) != 0 {
				t.Fatal("failed directory read invented credential history", err)
			}
			f.engine.Tick(r.NextRun)
			want := 1
			if scope == scopeAll {
				want = 2
			}
			if len(f.engine.runs) != want || f.engine.rules[r.ID].LastScopeDispatch.Matched != want || f.engine.rules[r.ID].LastScopeDispatch.Skipped != 0 || !f.engine.rules[r.ID].NextRun.After(r.NextRun) {
				t.Fatal("missing scheduled round", f.engine.rules[r.ID].LastScopeDispatch)
			}
			f.engine.rules, f.engine.runs = map[string]*Rule{}, map[string]*Run{}
			if err := f.engine.load(); err != nil {
				t.Fatal(err)
			}
			saved := f.engine.rules[r.ID]
			for _, run := range f.engine.runs {
				detail := run.PreflightDetail
				if detail == nil || detail.Code != "management_session_required" || run.Status != "blocked" || !run.Config.Scheduled || run.Config.ScopeBatchID != saved.LastScopeBatch || run.Config.ManagedProbe != nil || len(run.Samples) != 0 {
					t.Fatal("missing session result after restart", run)
				}
			}
			if len(f.engine.queue) != 0 || f.probes != 0 || f.nativeCalls != 0 || len(f.patches) != 0 || f.deletes != 0 || f.host.Management.ready() {
				t.Fatal("sessionless trigger used or restored management authority")
			}
		})
	}
}

func TestFixedRuleSkipHistoryFollowsBoundCredential(t *testing.T) {
	ctx := context.Background()
	initial := scopedTarget("auth-a", "a")
	initial.Email = "fixed@example.test"
	en, host, _ := scopeFixture(t, initial)
	config := lifeConfig()
	config.QuestionID = "random-linear"
	rule, err := en.SaveRule(ctx, Rule{RunConfig: config, Enabled: true, RetentionMode: "count", MaxResults: 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	previousID := ""
	check := func(code, status string) *Run {
		t.Helper()
		now := en.now().Add(time.Minute)
		en.now = func() time.Time { return now }
		receipt, err := en.dispatchRule(ctx, rule.ID, rule.Revision, false)
		if err != nil || receipt.RunID == "" {
			t.Fatal(receipt, err)
		}
		run := en.runs[receipt.RunID]
		if run.Status == "queued" {
			en.executeTask(en.nextRunnableLocked())
			run = en.runs[receipt.RunID]
		}
		detail := run.PreflightDetail
		if detail == nil {
			detail = run.SkipDetail
		}
		if run.Status != status || detail == nil || detail.Code != code || run.Config.AuthID != initial.AuthID || run.Config.Seed == 0 || run.Config.Question.Prompt == "" || len(run.Samples) != 0 {
			t.Fatal("invalid frozen fixed result", run)
		}
		if previousID != "" && en.runs[previousID] != nil {
			t.Fatal("old result exceeded retention")
		}
		previousID = run.ID
		return run
	}
	host.targets = nil
	if run := check("credential_missing", "blocked"); run.CredentialEmail != "" {
		t.Fatal(run)
	}
	host.targets = []Target{initial}
	host.targets[0].Provider = "other"
	if run := check("provider_mismatch", "blocked"); run.CredentialEmail != initial.Email {
		t.Fatal(run)
	}
	host.targets[0] = initial
	host.targets[0].Models = []string{"different"}
	check("missing_model", "completed")
	host.targets[0] = initial
	en.queue = make(chan string, 1)
	en.queue <- "occupied"
	en.runs["occupied"] = &Run{ID: "occupied", Status: "queued"}
	check("queue_full", "skipped")
	<-en.queue
	delete(en.runs, "occupied")
	host.targets[0].catalogVerified = false
	host.targets[0].Models = nil
	receipt, err := en.dispatchRule(ctx, rule.ID, rule.Revision, false)
	if err != nil || receipt.Count != 1 || receipt.Skipped != 0 {
		t.Fatal(receipt, err)
	}
	en.executeTask(en.nextRunnableLocked())
	run := en.runs[receipt.RunID]
	if host.executeCalls != 1 || run.Status != "completed" || run.PreflightDetail != nil || len(run.Samples) != 1 {
		t.Fatal("unknown directory was not tested", run, host.executeCalls)
	}
}

func TestObservationDistinguishesMissingModelFromUnreadableCatalog(t *testing.T) {
	absent := scopedTarget("absent", "other")
	unknown := scopedTarget("unknown", "other")
	unknown.catalogVerified = false
	empty := scopedTarget("empty")
	en, host, r := scopeFixture(t, absent, unknown, empty)
	receipt, err := en.dispatchRule(context.Background(), r.ID, r.Revision, false)
	if err != nil || receipt.Count != 3 || receipt.Skipped != 0 {
		t.Fatal(receipt, err)
	}
	for task := en.nextRunnableLocked(); task != nil; task = en.nextRunnableLocked() {
		en.executeTask(task)
	}
	for _, run := range en.runs {
		if run.Status != "completed" || run.SkipDetail != nil {
			t.Fatal(run)
		}
		if run.Config.AuthID == "unknown" || run.Config.AuthID == "empty" {
			if run.PreflightDetail != nil || len(run.Samples) != 2 {
				t.Fatal("unknown directory silently skipped", run)
			}
		} else if run.PreflightDetail == nil || run.PreflightDetail.Code != "missing_model" || len(run.Samples) != 0 {
			t.Fatal("known missing directory did not produce a check", run)
		}
	}
	if host.executeCalls != 4 || host.pauseCalls != 0 {
		t.Fatal(host.executeCalls, host.pauseCalls)
	}
}

func TestMissingSubjectStillProtectsPausedDependency(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []string{"manual", "rule"} {
		t.Run(mode, func(t *testing.T) {
			subject, control := scopedTarget("auth-a", "a"), scopedTarget("control", "a")
			subject.ComparisonIdentity, control.ComparisonIdentity = "subject-identity", "control-identity"
			en, host, _ := scopeFixture(t, subject, control)
			config := lifeConfig()
			config.ControlAuthID, config.ControlProvider = control.AuthID, control.Provider
			rule, err := en.SaveRule(ctx, Rule{RunConfig: config, Enabled: true}, false)
			if err != nil {
				t.Fatal(err)
			}
			control.QualityPaused = true
			host.targets = []Target{control}
			if mode == "manual" {
				if _, err := en.Submit(ctx, config); err == nil || len(en.runs) != 0 {
					t.Fatal("manual submission with a paused dependency was admitted", err)
				}
			} else {
				receipt, err := en.dispatchRule(ctx, rule.ID, rule.Revision, false)
				if err != nil || receipt.Count != 1 || receipt.Skipped != 0 || len(receipt.SkipDetails) != 0 ||
					en.runs[receipt.RunID].PreflightDetail.Code != "quality_paused" || en.runs[receipt.RunID].Config.RuleRevision != rule.Revision {
					t.Fatal("missing evidence for original rule revision", receipt, err)
				}
			}
			stored := en.rules[rule.ID]
			if stored.BlockedAuthID != control.AuthID || stored.Enabled || stored.Revision <= rule.Revision || host.executeCalls != 0 || len(en.queue) != 0 {
				t.Fatal("dependency pause did not stop the rule", stored)
			}
		})
	}
}
