package quality

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type schedulerTestHost struct {
	*manualOnlyHost
	entered chan string
}

func (h *schedulerTestHost) Execute(ctx context.Context, e Execution) Sample {
	h.entered <- e.AuthID
	<-ctx.Done()
	return Sample{ExecutionStatus: "cancelled", QualityVerdict: "unknown"}
}

func TestSchedulerDoesNotSpendWorkersWaitingOnSharedCredentials(t *testing.T) {
	en, base, _ := manualOnlyFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	en.ctx, en.cancel = ctx, cancel
	h := &schedulerTestHost{manualOnlyHost: base, entered: make(chan string, 4)}
	en.host = h
	t.Cleanup(func() {
		if err := en.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, authID := range []string{"auth-a", "auth-a", "auth-b"} {
		c := lifeConfig()
		c.AuthID = authID
		if _, err := en.Submit(context.Background(), c); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		en.wg.Add(1)
		go en.worker()
	}
	seen := map[string]bool{<-h.entered: true, <-h.entered: true}
	if !seen["auth-a"] || !seen["auth-b"] {
		t.Fatal("unrelated credential starved behind a busy dependency", seen)
	}
	en.mu.Lock()
	queued := 0
	for _, run := range en.runs {
		if run.Status == "queued" {
			queued++
		}
	}
	en.mu.Unlock()
	if queued != 1 {
		t.Fatal("waiting dependency consumed a worker instead of remaining queued")
	}
}

func TestSchedulerClaimsAllDependenciesTogetherAndReclaimsReservations(t *testing.T) {
	en, _, _ := manualOnlyFixture(t)
	one := lifeConfig()
	one.ControlAuthID, one.ControlProvider = "auth-b", "offline"
	first, err := en.Submit(context.Background(), one)
	if err != nil {
		t.Fatal(err)
	}
	two := lifeConfig()
	two.AuthID = "auth-c"
	two.Judge = Judge{Mode: "llm", AuthID: "auth-b", Provider: "offline", Model: "a"}
	second, err := en.Submit(context.Background(), two)
	if err != nil {
		t.Fatal(err)
	}
	en.mu.Lock()
	task := en.nextRunnableLocked()
	if task == nil || task.id != first.ID {
		t.Fatal("first ready task not claimed")
	}
	if blocked := en.nextRunnableLocked(); blocked != nil || en.runs[second.ID].Status != "queued" {
		t.Fatal("shared control/judge dependency was claimed twice")
	}
	if _, held := en.busyAuth["auth-c"]; held {
		t.Fatal("blocked task retained a partial dependency reservation")
	}
	en.mu.Unlock()
	task.cancel()
	en.executeTask(task)
	en.mu.Lock()
	defer en.mu.Unlock()
	if len(en.busyAuth) != 0 {
		t.Fatal("finished task leaked credential reservations")
	}
	next := en.nextRunnableLocked()
	if next == nil || next.id != second.ID {
		t.Fatal("dependency release did not make waiting task eligible")
	}
	next.cancel()
}

func TestJudgeIdentityBoundaryResetsConsecutiveEvidence(t *testing.T) {
	en, h, owner := manualOnlyFixture(t)
	r := en.rules[owner.ID]
	r.Judge = Judge{Mode: "llm", AuthID: "auth-c", Provider: "offline", Model: "a"}
	for range 2 {
		decideSemantic(en, semanticRound(en, r, "incorrect", "incorrect"))
	}
	h.targets[2].IdentityVersion = "new-judge"
	decideSemantic(en, semanticRound(en, r, "incorrect", "incorrect"))
	if h.pauseCalls != 0 || r.FailCount != 1 {
		t.Fatal("old judge evidence crossed the identity boundary", r)
	}
}

func TestFinalDecisionRechecksDependenciesAfterLastModelCall(t *testing.T) {
	for _, change := range []string{"judge_replaced", "subject_disabled", "judge_paused"} {
		t.Run(change, func(t *testing.T) {
			en, h, owner := manualOnlyFixture(t)
			r := en.rules[owner.ID]
			r.Judge = Judge{Mode: "llm", AuthID: "auth-c", Provider: "offline", Model: "a"}
			for range 2 {
				decideSemantic(en, semanticRound(en, r, "incorrect", "incorrect"))
			}
			completed := semanticRound(en, r, "incorrect", "incorrect")
			switch change {
			case "judge_replaced":
				h.targets[2].IdentityVersion = "new-judge"
			case "subject_disabled":
				h.targets[0].ManualDisabled = true
			case "judge_paused":
				h.targets[2].QualityPaused = true
			}
			decideSemantic(en, completed)
			if h.pauseCalls != 0 || r.FailCount != 0 {
				t.Fatal("last sample bypassed final dependency validation", r)
			}
			if change == "judge_paused" && (r.Enabled || r.Status != "awaiting_release") {
				t.Fatal("newly paused judge failed to stop dependent rule", r)
			}
		})
	}
}

func TestQuiescedOrFaultedEngineRejectsAllStateMutations(t *testing.T) {
	for _, state := range []string{"closed", "faulted"} {
		t.Run(state, func(t *testing.T) {
			en, h, owner := manualOnlyFixture(t)
			for range 3 {
				decideSemantic(en, semanticRound(en, owner, "incorrect", "incorrect"))
			}
			r := en.rules[owner.ID]
			action := h.actions[0]
			if state == "closed" {
				en.closed = true
			} else {
				en.fault = errors.New("simulated durable failure")
			}
			for name, fn := range map[string]func() error{
				"pause":  func() error { return en.PauseRule(r.ID, r.Revision, true) },
				"delete": func() error { return en.DeleteRule(context.Background(), r.ID, r.Revision) },
				"release": func() error {
					return en.Release(context.Background(), r.ID, action.ActionID, action.ActionRevision, false)
				},
				"settings": func() error { return en.UpdateConfig(en.cfg) },
				"cancel":   func() error { return en.Cancel("missing") },
				"review":   func() error { return en.Review("missing", 0, "correct", "") },
			} {
				if err := fn(); err == nil || !strings.Contains(err.Error(), "unavailable") {
					t.Errorf("%s mutation accepted after %s: %v", name, state, err)
				}
			}
			if h.releaseCalls != 0 {
				t.Fatal("quiesced instance released a host restriction")
			}
		})
	}
}

func TestRepeatedCloseWaitsForSharedCompletionAndPreservesFailure(t *testing.T) {
	en, _, _ := manualOnlyFixture(t)
	en.closed = true
	en.closeDone = make(chan struct{})
	returned := make(chan error, 2)
	for range 2 {
		go func() { returned <- en.Close() }()
	}
	failure := errors.New("shutdown persistence failed")
	en.mu.Lock()
	en.closeErr = failure
	close(en.closeDone)
	en.mu.Unlock()
	for range 2 {
		if err := <-returned; !errors.Is(err, failure) {
			t.Fatal("repeated close did not wait for and return the first close result", err)
		}
	}
}

func TestCancelledPendingTaskImmediatelyFreesAdmissionCapacity(t *testing.T) {
	en, _, _ := manualOnlyFixture(t)
	en.queue = make(chan string, 1)
	first, err := en.Submit(context.Background(), lifeConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err = en.Cancel(first.ID); err != nil {
		t.Fatal(err)
	}
	second, err := en.Submit(context.Background(), lifeConfig())
	if err != nil {
		t.Fatal("cancelled task retained the only queue slot", err)
	}
	if id := <-en.queue; id != second.ID {
		t.Fatal("queue compaction retained a terminal entry", id)
	}
}

func TestCapturedTickCannotExecuteEditedRuleOrDisabledSchedule(t *testing.T) {
	for _, change := range []string{"edit", "rule_schedule_off", "global_schedule_off"} {
		t.Run(change, func(t *testing.T) {
			en, host, owner := manualOnlyFixture(t)
			en.cfg.SchedulingEnabled = true
			r := en.rules[owner.ID]
			r.SchedulingEnabled = true
			r.NextRun = en.now()
			en.mu.Lock()
			due, err := en.takeDueRulesLocked(en.now())
			en.mu.Unlock()
			if err != nil || len(due) != 1 {
				t.Fatal("expected one persisted due tick", due, err)
			}
			if change == "global_schedule_off" {
				cfg := en.Config()
				cfg.SchedulingEnabled = false
				err = en.UpdateConfig(cfg)
			} else {
				edit := *cloneRule(r)
				if change == "edit" {
					edit.Parameters.MaxTokens = 128
				} else {
					edit.SchedulingEnabled = false
				}
				_, err = en.SaveRule(context.Background(), edit, true)
			}
			if err != nil {
				t.Fatal(err)
			}
			run, err := en.submit(context.Background(), RunConfig{}, due[0].id, due[0].revision)
			if err == nil || run != nil || len(en.queue) != 0 || host.executeCalls != 0 {
				t.Fatal("old tick executed after rule edit or scheduling disable", run, err)
			}
		})
	}
}
