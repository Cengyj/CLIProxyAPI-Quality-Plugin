package quality

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

type semanticsHost struct {
	targets                                              []Target
	actions                                              []Action
	pauseCalls, recoverCalls, releaseCalls, executeCalls int
	releaseErr                                           error
}

func (h *semanticsHost) Capabilities() HostCapabilities {
	return HostCapabilities{Mode: "offline_verified_fixture", QualityPause: true, ManualRelease: true, VerifiedEvidence: true, ModelOperationCancel: true, StreamClose: true}
}

func (h *semanticsHost) Targets(context.Context) ([]Target, error) { return h.targets, nil }
func (h *semanticsHost) Actions(context.Context) ([]Action, error) {
	return append([]Action(nil), h.actions...), nil
}
func (h *semanticsHost) Pause(_ context.Context, a Action) (Action, error) {
	h.pauseCalls++
	h.actions = append(h.actions, a)
	return a, nil
}
func (h *semanticsHost) Release(_ context.Context, a Action) error {
	h.releaseCalls++
	if h.releaseErr != nil {
		return h.releaseErr
	}
	for i, p := range h.actions {
		if p.ActionID == a.ActionID {
			h.actions = append(h.actions[:i], h.actions[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("missing action")
}
func (h *semanticsHost) Recover(ctx context.Context, a Action, _ []string) error {
	h.recoverCalls++
	return h.Release(ctx, a)
}
func (h *semanticsHost) Execute(context.Context, Execution) Sample {
	h.executeCalls++
	return Sample{ExecutionStatus: "failed", Error: "offline test has no upstream"}
}
func semanticsEngine(t *testing.T) (*Engine, *semanticsHost, *Rule) {
	t.Helper()
	cfg := DefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.AutomaticPause = true
	store, err := OpenStore(cfg.DataDir, cfg.MaxStorageBytes)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	h := &semanticsHost{targets: []Target{{AuthID: "auth-a", Provider: "offline", Models: []string{"a", "b"}, IdentityVersion: "identity-a", CanPause: true, EffectiveAvailable: true}}}
	en := &Engine{store: store, host: h, cfg: cfg, runs: map[string]*Run{}, rules: map[string]*Rule{}, active: map[string]context.CancelFunc{}, busyAuth: map[string]struct{}{}, queue: make(chan string, 32), ctx: context.Background(), now: func() time.Time { return time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC) }}
	rule, err := en.SaveRule(context.Background(), Rule{RunConfig: RunConfig{AuthID: "auth-a", Provider: "offline", Models: []string{"a", "b"}, Samples: 1, QuestionID: "logic-boxes"}, Enabled: true, Action: "pause_credential", AutomaticPause: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	return en, h, rule
}
func semanticRound(en *Engine, rule *Rule, verdicts ...string) *Run {
	r := &Run{ID: ID(), Status: "completed", Source: "scheduled_probe", CreatedAt: en.now(), Config: Frozen{RunConfig: rule.RunConfig, IdentityVersion: "identity-a", RuleID: rule.ID, RuleRevision: rule.Revision, Question: Question{ID: "logic-boxes", Version: "1", Kind: "logic", Prompt: "frozen problem", Answer: "4"}}}
	for _, target := range mustTargets(en) {
		if target.AuthID == r.Config.Judge.AuthID {
			r.Config.JudgeIdentity = target.IdentityVersion
		}
		if target.AuthID == r.Config.ControlAuthID {
			r.Config.ControlIdentity = target.IdentityVersion
		}
	}
	now := en.now()
	r.CompletedAt = &now
	for i, v := range verdicts {
		m := rule.Models[i%len(rule.Models)]
		r.Samples = append(r.Samples, Sample{Model: m, Index: i / len(rule.Models), ExecutionStatus: "completed", QualityVerdict: v, Comparable: true, Evidence: Evidence{Verified: true, AuthID: rule.AuthID, RequestedModel: m, ResolvedModel: m, Path: "offline-verified", PayloadFingerprint: "stable-conditions"}})
	}
	en.runs[r.ID] = r
	return r
}
func mustTargets(en *Engine) []Target {
	targets, _ := en.host.Targets(en.ctx)
	return targets
}
func decideSemantic(en *Engine, r *Run) { en.mu.Lock(); defer en.mu.Unlock(); en.decideLocked(r) }
func TestConservativeCredentialDecisionNeedsThreeCompleteAllModelFailures(t *testing.T) {
	en, h, r := semanticsEngine(t)
	if r.FailThreshold != 3 {
		t.Fatal("unsafe defaults")
	}
	for range 2 {
		decideSemantic(en, semanticRound(en, r, "incorrect", "incorrect"))
	}
	if h.pauseCalls != 0 {
		t.Fatal("paused before threshold")
	}
	decideSemantic(en, semanticRound(en, r, "incorrect", "correct"))
	if h.pauseCalls != 0 {
		t.Fatal("one healthy model paused whole credential")
	}
	for range 2 {
		decideSemantic(en, semanticRound(en, r, "incorrect", "incorrect"))
	}
	if h.pauseCalls != 0 {
		t.Fatal("mixed round did not break consecutive count")
	}
	decideSemantic(en, semanticRound(en, r, "incorrect", "incorrect"))
	if h.pauseCalls != 1 {
		t.Fatalf("pause count %d", h.pauseCalls)
	}
	for range 4 {
		decideSemantic(en, semanticRound(en, r, "correct", "correct"))
	}
	current := en.Rules()[0]
	if h.recoverCalls != 0 || h.releaseCalls != 0 || len(h.actions) != 1 || current.Enabled || current.Status != "awaiting_release" || current.FailCount != 0 {
		t.Fatal("paused credential must remain held without retests or recovery", current)
	}
}
func TestInfrastructureIncompleteUnknownAndDriftNeverPause(t *testing.T) {
	for _, kind := range []string{"http401", "http429", "network", "cancelled", "unknown", "missing", "rewritten", "duplicate", "stale_rule", "manual"} {
		t.Run(kind, func(t *testing.T) {
			en, h, r := semanticsEngine(t)
			for range 4 {
				run := semanticRound(en, r, "incorrect", "incorrect")
				switch kind {
				case "http401", "http429", "network":
					run.Samples[0].ExecutionStatus = "failed"
					run.Samples[0].Error = kind
				case "cancelled":
					run.Status = "cancelled"
				case "unknown":
					run.Samples[0].QualityVerdict = "unknown"
				case "missing":
					run.Samples = run.Samples[:1]
				case "rewritten":
					run.Samples[0].Comparable = false
					run.Samples[0].Evidence.ConditionsChanged = true
				case "duplicate":
					run.Samples[1] = run.Samples[0]
				case "stale_rule":
					run.Config.RuleRevision++
				case "manual":
					run.Source = "manual_test"
				}
				decideSemantic(en, run)
			}
			if h.pauseCalls != 0 {
				t.Fatal("unsafe automatic pause")
			}
		})
	}
}
func TestIdentityAndConditionsChangesResetConsecutiveEvidence(t *testing.T) {
	for _, kind := range []string{"identity", "model_mapping", "payload"} {
		t.Run(kind, func(t *testing.T) {
			en, h, r := semanticsEngine(t)
			for range 2 {
				decideSemantic(en, semanticRound(en, r, "incorrect", "incorrect"))
			}
			run := semanticRound(en, r, "incorrect", "incorrect")
			switch kind {
			case "identity":
				run.Config.IdentityVersion = "replacement"
			case "model_mapping":
				run.Samples[0].Evidence.ResolvedModel = "other-model"
			case "payload":
				run.Samples[0].Evidence.PayloadFingerprint = "different-conditions"
			}
			decideSemantic(en, run)
			if h.pauseCalls != 0 {
				t.Fatal("old evidence applied to new identity or conditions")
			}
		})
	}
}
func TestManualTestAndSavingRuleNeverPerformActionsOrModelCalls(t *testing.T) {
	en, h, r := semanticsEngine(t)
	if h.executeCalls != 0 || h.pauseCalls != 0 {
		t.Fatal("save triggered model/action")
	}
	for range 5 {
		run := semanticRound(en, r, "incorrect", "incorrect")
		run.Source = "manual_test"
		decideSemantic(en, run)
	}
	if h.pauseCalls != 0 || h.recoverCalls != 0 {
		t.Fatal("manual result caused action")
	}
	cfg := DefaultConfig()
	if cfg.SchedulingEnabled || cfg.AutomaticPause {
		t.Fatal("installation defaults enable automation")
	}
}
func TestHeldPauseIgnoresNewEvidenceAndManualConflictPreservesOwnership(t *testing.T) {
	en, h, r := semanticsEngine(t)
	for range 3 {
		decideSemantic(en, semanticRound(en, r, "incorrect", "incorrect"))
	}
	for _, verdict := range []string{"correct", "unknown", "incorrect", "correct"} {
		decideSemantic(en, semanticRound(en, r, verdict, verdict))
	}
	if h.recoverCalls != 0 || h.releaseCalls != 0 || h.pauseCalls != 1 {
		t.Fatal("held pause accepted further automatic decisions")
	}
	a := h.actions[0]
	h.releaseErr = fmt.Errorf("conflict: action revision changed")
	if err := en.Release(context.Background(), r.ID, a.ActionID, a.ActionRevision, false); err == nil {
		t.Fatal("manual release conflict ignored")
	}
	current := en.Rules()[0]
	if len(h.actions) != 1 || current.ActionID != a.ActionID || current.Enabled || current.Status != "awaiting_release" {
		t.Fatal("manual release conflict lost pause or resumed rule", current)
	}
}
func TestRuleOwnershipSurvivesEditsAndPreventsOrphaning(t *testing.T) {
	en, h, r := semanticsEngine(t)
	for range 3 {
		decideSemantic(en, semanticRound(en, r, "incorrect", "incorrect"))
	}
	current := en.Rules()[0]
	edit := current
	edit.Name = "edited"
	saved, err := en.SaveRule(context.Background(), edit, true)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ActionID != h.actions[0].ActionID || saved.ActionRuleRevision != h.actions[0].RuleRevision {
		t.Fatal("rule edit lost immutable action binding")
	}
	if err = en.DeleteRule(context.Background(), saved.ID, saved.Revision); err == nil {
		t.Fatal("deleted owning rule")
	}
	edit = *saved
	edit.AuthID = "replacement-target"
	if _, err = en.SaveRule(context.Background(), edit, true); err == nil {
		t.Fatal("changed owning target")
	}
	if err = en.PauseRule(saved.ID, saved.Revision, true); err != nil {
		t.Fatal(err)
	}
	if len(h.actions) != 1 {
		t.Fatal("pausing rule released credential")
	}
}
func TestStrictJudgesAndQuestionVersions(t *testing.T) {
	q, err := FreezeQuestion(RunConfig{QuestionID: "logic-boxes"})
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"14", "answer is 4", "4 or 5", "4\n5", "NaN", ""} {
		v, _ := DeterministicJudge(q, input)
		if input == "14" {
			if v != "incorrect" {
				t.Fatal(v)
			}
		} else if v != "unknown" {
			t.Fatalf("%q parsed as %s", input, v)
		}
	}
	if _, err = FreezeQuestion(RunConfig{QuestionID: "logic-boxes", Prompt: q.Prompt + " changed"}); err == nil {
		t.Fatal("edited question retained fixed answer")
	}
	for _, raw := range []string{`{"verdict":"correct","verdict":"incorrect","reason":"x"}`, `{"verdict":"correct","reason":"x","extra":1}`, `{"verdict":"correct","reason":null}`, `{"verdict":"correct","reason":"x"} trailing`, `{"verdict":"maybe","reason":"x"}`, strings.Repeat("x", 4097)} {
		if v, _ := StrictJudge(raw); v != "unknown" {
			t.Fatalf("accepted invalid judge: %q", raw)
		}
	}
}
func TestRandomQuestionsReproducibleAndSolvedLocally(t *testing.T) {
	for seed := int64(0); seed < 32; seed++ {
		a, err := FreezeQuestion(RunConfig{QuestionID: "random-linear", Seed: seed})
		if err != nil {
			t.Fatal(err)
		}
		b, err := FreezeQuestion(RunConfig{QuestionID: "random-linear", Seed: seed})
		if err != nil {
			t.Fatal(err)
		}
		if a != b {
			t.Fatal("seed not reproducible")
		}
		var ab, bc, ac int
		if _, err = fmt.Sscanf(a.Prompt, "Positive integers A, B, C are at most 20. A+B=%d, B+C=%d, A+C=%d.", &ab, &bc, &ac); err != nil {
			t.Fatal(err)
		}
		answer := (ab + bc - ac) / 2
		if a.Answer != fmt.Sprint(answer) {
			t.Fatal("local solver incorrect")
		}
	}
}

func TestRandomRuleRequiresFreshCompleteCompatibleControl(t *testing.T) {
	for _, kind := range []string{"missing", "all_wrong", "different_path", "different_mapping", "different_payload", "stale", "window", "control_replaced", "own_alias", "valid"} {
		t.Run(kind, func(t *testing.T) {
			en, h, r := semanticsEngine(t)
			h.targets[0].ComparisonIdentity = "account-a"
			h.targets = append(h.targets, Target{AuthID: "independent-control", IdentityVersion: "control-v1", ComparisonIdentity: "account-b", EffectiveAvailable: true})
			if kind == "control_replaced" {
				h.targets[1].IdentityVersion = "control-v2"
			}
			if kind == "own_alias" {
				h.targets[1].ComparisonIdentity = "account-a"
			}
			for range 3 {
				run := semanticRound(en, r, "incorrect", "incorrect")
				run.Config.ControlIdentity = "control-v1"
				run.Config.Question.Kind = "random"
				run.Config.BatchID = "one-batch"
				run.Config.ControlAuthID = "independent-control"
				for _, sample := range run.Samples {
					sample.QualityVerdict = "correct"
					sample.Evidence.AuthID = "independent-control"
					run.Controls = append(run.Controls, sample)
				}
				switch kind {
				case "missing":
					run.Controls = nil
				case "all_wrong":
					run.Controls[0].QualityVerdict = "incorrect"
				case "different_path":
					run.Controls[0].Evidence.Path = "different"
				case "different_mapping":
					run.Controls[0].Evidence.ResolvedModel = "different"
				case "different_payload":
					run.Controls[0].Evidence.PayloadFingerprint = "different"
				case "stale":
					old := en.now().Add(-6 * time.Minute)
					run.CompletedAt = &old
				case "window":
					run.CreatedAt = en.now().Add(-6 * time.Minute)
				}
				decideSemantic(en, run)
			}
			if kind == "valid" {
				if h.pauseCalls != 1 {
					t.Fatal("valid control failed to support decision")
				}
			} else if h.pauseCalls != 0 {
				t.Fatal("invalid control caused automatic pause")
			}
		})
	}
}
func TestAutomaticPauseRequiresBothIndependentSwitches(t *testing.T) {
	for _, kind := range []string{"global_off", "rule_off"} {
		t.Run(kind, func(t *testing.T) {
			en, h, r := semanticsEngine(t)
			if kind == "global_off" {
				en.cfg.AutomaticPause = false
			} else {
				en.rules[r.ID].AutomaticPause = false
			}
			for range 5 {
				decideSemantic(en, semanticRound(en, r, "incorrect", "incorrect"))
			}
			if h.pauseCalls != 0 {
				t.Fatal("disabled automatic pause switch ignored")
			}
		})
	}
}
