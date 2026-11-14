package quality

import (
	"context"
	"strings"
	"testing"
	"time"

	bolt "go.etcd.io/bbolt"
)

type manualOnlyHost struct {
	*semanticsHost
	states map[string]ActionState
}

type manualOnlyRunningHost struct {
	*manualOnlyHost
	entered chan struct{}
}

func (h *manualOnlyRunningHost) Execute(ctx context.Context, _ Execution) Sample {
	h.executeCalls++
	close(h.entered)
	<-ctx.Done()
	return Sample{ExecutionStatus: "cancelled", QualityVerdict: "unknown"}
}

func (h *manualOnlyHost) ActionStatus(_ context.Context, id string) (ActionState, error) {
	for _, a := range h.actions {
		if a.ActionID == id {
			return ActionState{Status: "active", Action: &a}, nil
		}
	}
	if state, ok := h.states[id]; ok {
		return state, nil
	}
	return ActionState{Status: "absent"}, nil
}

func manualOnlyFixture(t *testing.T) (*Engine, *manualOnlyHost, *Rule) {
	t.Helper()
	en, base, rule := semanticsEngine(t)
	for _, id := range []string{"auth-b", "auth-c"} {
		base.targets = append(base.targets, Target{AuthID: id, Provider: "offline", Models: []string{"a", "b"}, IdentityVersion: "identity-" + id, ComparisonIdentity: "distinct-" + id, PayloadConfigVersion: "payload-v1", CanPause: true, EffectiveAvailable: true})
	}
	base.targets[0].ComparisonIdentity = "distinct-auth-a"
	base.targets[0].PayloadConfigVersion = "payload-v1"
	h := &manualOnlyHost{semanticsHost: base, states: map[string]ActionState{}}
	en.host = h
	return en, h, rule
}

func TestManualOnlyPausedDependenciesRejectedBeforeCallingModels(t *testing.T) {
	for _, role := range []string{"subject", "control", "judge"} {
		t.Run(role, func(t *testing.T) {
			en, h, _ := manualOnlyFixture(t)
			c := lifeConfig()
			paused := 0
			switch role {
			case "control":
				c.ControlAuthID, c.ControlProvider = "auth-b", "offline"
				paused = 1
			case "judge":
				c.Judge = Judge{Mode: "llm", AuthID: "auth-c", Provider: "offline", Model: "a"}
				paused = 2
			}
			h.targets[paused].QualityPaused = true
			if _, err := en.Submit(context.Background(), c); err == nil || !strings.Contains(err.Error(), "blocked") {
				t.Fatal("paused dependency was admitted", err)
			}
			if h.executeCalls != 0 || en.budget.Calls != 0 || len(en.queue) != 0 {
				t.Fatal("blocked submission consumed resources")
			}
		})
	}
}

func TestManualOnlyPauseStopsAllDependenciesAndLegacyReleaseKeepsThemStopped(t *testing.T) {
	en, h, owner := manualOnlyFixture(t)
	dependentConfig := lifeConfig()
	dependentConfig.AuthID = "auth-b"
	dependentConfig.Judge = Judge{Mode: "llm", AuthID: "auth-a", Provider: "offline", Model: "a"}
	dependent, err := en.SaveRule(context.Background(), Rule{RunConfig: dependentConfig, Enabled: true, Action: "observe_only"}, false)
	if err != nil {
		t.Fatal(err)
	}
	queuedRule, err := en.RunRule(context.Background(), dependent.ID)
	if err != nil {
		t.Fatal(err)
	}
	manualConfig := lifeConfig()
	manualConfig.AuthID = "auth-c"
	manualConfig.ControlAuthID, manualConfig.ControlProvider = "auth-a", "offline"
	queuedManual, err := en.Submit(context.Background(), manualConfig)
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		decideSemantic(en, semanticRound(en, owner, "incorrect", "incorrect"))
	}
	if h.pauseCalls != 1 {
		t.Fatal("expected a single durable pause")
	}
	for _, r := range en.Rules() {
		if r.Enabled || r.Status != "awaiting_release" || !r.NextRun.IsZero() || r.FailCount != 0 || r.EvidenceSignature != "" {
			t.Fatal("dependent rule did not stop and reset", r)
		}
	}
	for _, id := range []string{queuedRule.ID, queuedManual.ID} {
		r, _ := en.Run(id)
		if r.Status != "blocked" || !r.CancelRequested || r.BlockReason != "quality_paused" {
			t.Fatal("queued dependent task survived pause", r)
		}
		en.execute(id)
	}
	if h.executeCalls != 0 {
		t.Fatal("blocked queue entry executed")
	}
	a := h.actions[0]
	if err = en.Release(context.Background(), owner.ID, a.ActionID, a.ActionRevision, false); err != nil {
		t.Fatal(err)
	}
	for _, r := range en.Rules() {
		if r.Enabled || r.Status != "paused" || !r.NextRun.IsZero() || r.FailCount != 0 || r.ActionID != "" {
			t.Fatal("legacy pause_rule=false resumed a dependent rule", r)
		}
	}
	if h.releaseCalls != 1 || h.recoverCalls != 0 || h.executeCalls != 0 {
		t.Fatal("manual release caused model calls or verified recovery")
	}
}

func TestManualOnlyReconcileTracksStableRemainingBlocker(t *testing.T) {
	en, h, owner := manualOnlyFixture(t)
	r := en.rules[owner.ID]
	r.ControlAuthID, r.ControlProvider = "auth-b", "offline"
	h.targets[0].QualityPaused, h.targets[1].QualityPaused = true, true
	if err := en.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r.BlockedAuthID != "auth-a" || r.Status != "awaiting_release" {
		t.Fatal("blocker selection is not deterministic", r)
	}
	revision := r.Revision
	for range 4 {
		if err := en.Reconcile(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if r.Revision != revision {
		t.Fatal("unchanged blockers keep rewriting rule revision")
	}
	h.targets[0].QualityPaused = false
	if err := en.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r.BlockedAuthID != "auth-b" || r.Enabled || r.Status != "awaiting_release" {
		t.Fatal("release of one blocker hid the remaining blocker", r)
	}
	if err := en.PauseRule(r.ID, r.Revision, false); err == nil {
		t.Fatal("rule resumed with another paused dependency")
	}
	h.targets[1].QualityPaused = false
	if err := en.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r.BlockedAuthID != "" || r.Enabled || r.Status != "paused" || !r.NextRun.IsZero() {
		t.Fatal("external release resumed rule", r)
	}
	if h.executeCalls != 0 || h.releaseCalls != 0 || h.pauseCalls != 0 {
		t.Fatal("reconciliation performed a model call or new action")
	}
}

func TestManualOnlyReleasePreservesOtherOwnerAndDependencyBlockers(t *testing.T) {
	for _, dependency := range []string{"same credential", "control", "judge"} {
		t.Run(dependency, func(t *testing.T) {
			en, h, owner := manualOnlyFixture(t)
			r := en.rules[owner.ID]
			remainingID, targetIndex := "auth-a", 0
			switch dependency {
			case "control":
				r.ControlAuthID, r.ControlProvider = "auth-b", "offline"
				remainingID, targetIndex = "auth-b", 1
			case "judge":
				r.Judge = Judge{Mode: "llm", AuthID: "auth-c", Provider: "offline", Model: "a"}
				remainingID, targetIndex = "auth-c", 2
			}
			for range 3 {
				decideSemantic(en, semanticRound(en, r, "incorrect", "incorrect"))
			}
			owned := h.actions[0]
			h.targets[targetIndex].QualityPaused = true
			h.actions = append(h.actions, Action{OwnerPlugin: "other-owner", AuthID: remainingID, RuleID: "other-rule", ActionID: "other-action", ActionRevision: 1, RuleRevision: 1})
			if err := en.Release(context.Background(), r.ID, owned.ActionID, owned.ActionRevision, false); err != nil {
				t.Fatal(err)
			}
			if r.ActionID != "" || r.BlockedAuthID != remainingID || r.Status != "awaiting_release" || r.Enabled || !r.NextRun.IsZero() {
				t.Fatal("manual release hid a remaining owner or dependency restriction", r)
			}
			if len(h.actions) != 1 || h.actions[0].ActionID != "other-action" || h.releaseCalls != 1 {
				t.Fatal("manual release removed another owner's restriction")
			}
			if err := en.PauseRule(r.ID, r.Revision, false); err == nil {
				t.Fatal("rule resumed before remaining restriction was released")
			}
			if h.executeCalls != 0 {
				t.Fatal("manual release initiated a model call")
			}
		})
	}
}

func TestManualOnlyPauseCancelsAlreadyRunningDependentEvaluation(t *testing.T) {
	en, base, owner := manualOnlyFixture(t)
	h := &manualOnlyRunningHost{manualOnlyHost: base, entered: make(chan struct{})}
	en.host = h
	c := lifeConfig()
	c.AuthID = "auth-b"
	c.Judge = Judge{Mode: "llm", AuthID: "auth-a", Provider: "offline", Model: "a"}
	run, err := en.Submit(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		en.execute(run.ID)
		close(done)
	}()
	<-h.entered
	for range 3 {
		decideSemantic(en, semanticRound(en, owner, "incorrect", "incorrect"))
	}
	<-done
	result, _ := en.Run(run.ID)
	if result.Status != "blocked" || result.BlockReason != "quality_paused" || !result.CancelRequested || h.executeCalls != 1 || h.pauseCalls != 1 {
		t.Fatal("quality pause failed to cancel active dependent operation before judge", result)
	}
}

func TestManualOnlyReconcileRejectsForeignActionStatusBinding(t *testing.T) {
	en, h, owner := manualOnlyFixture(t)
	for range 3 {
		decideSemantic(en, semanticRound(en, owner, "incorrect", "incorrect"))
	}
	r := en.rules[owner.ID]
	a := h.actions[0]
	h.actions = nil
	foreign := a
	foreign.OwnerPlugin = "another-plugin"
	h.states[a.ActionID] = ActionState{Status: "released", Action: &foreign}
	if err := en.Reconcile(context.Background()); err == nil {
		t.Fatal("foreign release binding was accepted")
	}
	if r.ActionID != a.ActionID || r.Status != "awaiting_release" || r.Enabled {
		t.Fatal("foreign binding cleared ownership", r)
	}
	h.states[a.ActionID] = ActionState{Status: "released", Action: &a}
	if err := en.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if r.ActionID != "" || r.Status != "paused" || r.Enabled {
		t.Fatal("confirmed release failed to leave rule stopped", r)
	}
}

func TestManualOnlyQueuedRevisionAndJudgeIdentityCannotBeReplayed(t *testing.T) {
	t.Run("stale rule", func(t *testing.T) {
		en, h, owner := manualOnlyFixture(t)
		run, err := en.RunRule(context.Background(), owner.ID)
		if err != nil {
			t.Fatal(err)
		}
		r := en.rules[owner.ID]
		if err = en.PauseRule(r.ID, r.Revision, true); err != nil {
			t.Fatal(err)
		}
		if err = en.PauseRule(r.ID, r.Revision, false); err != nil {
			t.Fatal(err)
		}
		en.execute(run.ID)
		result, _ := en.Run(run.ID)
		if result.Status != "blocked" || h.executeCalls != 0 || en.budget.Calls != 0 {
			t.Fatal("stale queue entry revived after explicit resume", result)
		}
	})
	t.Run("judge replacement", func(t *testing.T) {
		en, h, _ := manualOnlyFixture(t)
		c := lifeConfig()
		c.Judge = Judge{Mode: "llm", AuthID: "auth-c", Provider: "offline", Model: "a"}
		run, err := en.Submit(context.Background(), c)
		if err != nil {
			t.Fatal(err)
		}
		h.targets[2].IdentityVersion = "replacement"
		en.execute(run.ID)
		result, _ := en.Run(run.ID)
		if result.Status != "blocked" || h.executeCalls != 0 || en.budget.Calls != 0 {
			t.Fatal("replacement judge accepted frozen evidence conditions", result)
		}
	})
}

func TestManualOnlySchemaMigrationRetainsOwnershipAndInterruptsOldWork(t *testing.T) {
	en, h, owner := manualOnlyFixture(t)
	dependentConfig := lifeConfig()
	dependentConfig.AuthID = "auth-b"
	dependentConfig.ControlAuthID, dependentConfig.ControlProvider = "auth-a", "offline"
	dependent, err := en.SaveRule(context.Background(), Rule{RunConfig: dependentConfig, Enabled: true, Action: "observe_only"}, false)
	if err != nil {
		t.Fatal(err)
	}
	rule := *en.rules[owner.ID]
	rule.ActionID, rule.ActionRevision, rule.ActionRuleRevision = "held-pause", 7, rule.Revision
	rule.FailCount, rule.EvidenceSignature = 2, "obsolete"
	legacyRule := struct {
		Rule
		AutomaticRecovery bool `json:"automatic_recovery"`
		RetestSeconds     int  `json:"retest_seconds"`
		PassThreshold     int  `json:"pass_threshold"`
		PassCount         int  `json:"pass_count"`
	}{rule, true, 60, 1, 8}
	legacyConfig := struct {
		Config
		AutomaticRecovery bool `json:"automatic_recovery"`
	}{en.cfg, true}
	oldRun := Run{ID: "old-running", Status: "running", CreatedAt: en.now(), Samples: []Sample{{Output: "completed sample", ExecutionStatus: "completed"}}}
	if err := en.store.Write(func(tx *bolt.Tx) error {
		if err := put(tx, "rules", rule.ID, legacyRule); err != nil {
			return err
		}
		if err := put(tx, "meta", "settings", legacyConfig); err != nil {
			return err
		}
		if err := put(tx, "runs", oldRun.ID, oldRun); err != nil {
			return err
		}
		return tx.Bucket([]byte("meta")).Put([]byte("schema"), []byte("1"))
	}); err != nil {
		t.Fatal(err)
	}
	if err := en.store.Close(); err != nil {
		t.Fatal(err)
	}
	migrated, err := OpenStore(en.cfg.DataDir, en.cfg.MaxStorageBytes)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := migrated.Close(); err != nil {
			t.Error(err)
		}
	})
	en.store = migrated
	en.rules, en.runs = map[string]*Rule{}, map[string]*Run{}
	if err = en.load(); err != nil {
		t.Fatal(err)
	}
	current := en.rules[rule.ID]
	if current.ActionID != rule.ActionID || current.ActionRevision != 7 || current.ActionRuleRevision != rule.ActionRuleRevision || current.Enabled || current.Status != "awaiting_release" || current.FailCount != 0 || !current.NextRun.IsZero() {
		t.Fatal("migration lost ownership or retained detection", current)
	}
	if d := en.rules[dependent.ID]; d.Enabled || d.Status != "awaiting_release" || d.BlockedAuthID != owner.AuthID {
		t.Fatal("migration failed to stop dependent rule", d)
	}
	loaded := en.runs[oldRun.ID]
	if loaded.Status != "interrupted" || len(loaded.Samples) != 1 || loaded.Samples[0].Output != "completed sample" {
		t.Fatal("migration replayed or discarded prior work", loaded)
	}
	for _, bucket := range []string{"rules", "meta"} {
		if err = migrated.All(bucket, func(raw []byte) error {
			for _, field := range []string{"automatic_recovery", "retest_seconds", "pass_threshold", "pass_count"} {
				if strings.Contains(string(raw), field) {
					t.Errorf("migration retained %s in %s", field, bucket)
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if h.executeCalls != 0 {
		t.Fatal("migration called a model")
	}
}

func TestManualOnlyFailedDurableHaltNeverCallsHostPause(t *testing.T) {
	en, h, owner := manualOnlyFixture(t)
	for range 2 {
		decideSemantic(en, semanticRound(en, owner, "incorrect", "incorrect"))
	}
	prior := *cloneRule(en.rules[owner.ID])
	if err := en.store.Close(); err != nil {
		t.Fatal(err)
	}
	decideSemantic(en, semanticRound(en, owner, "incorrect", "incorrect"))
	current := en.rules[owner.ID]
	if h.pauseCalls != 0 || current.ActionID != "" || current.Revision != prior.Revision || current.Enabled != prior.Enabled || current.FailCount != prior.FailCount || en.fault == nil {
		t.Fatal("failed transaction published fictitious ownership or called host", current)
	}
	if _, err := en.Submit(context.Background(), lifeConfig()); err == nil {
		t.Fatal("failed storage continued accepting execution")
	}
}

func TestManualOnlyPendingActionNeverBecomesScheduledRetest(t *testing.T) {
	en, h, owner := manualOnlyFixture(t)
	en.cfg.SchedulingEnabled = true
	r := en.rules[owner.ID]
	r.SchedulingEnabled = true
	for range 3 {
		decideSemantic(en, semanticRound(en, owner, "incorrect", "incorrect"))
	}
	en.Tick(en.now().Add(31 * 24 * time.Hour))
	if len(en.queue) != 0 || h.executeCalls != 0 || h.pauseCalls != 1 || h.recoverCalls != 0 || h.releaseCalls != 0 {
		t.Fatal("elapsed time initiated retest or recovery")
	}
}
