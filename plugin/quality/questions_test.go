package quality

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func TestCandyReferenceHasExactVersionAndProvableAnswer(t *testing.T) {
	q, err := FreezeQuestion(RunConfig{QuestionID: "logic-candy"})
	if err != nil {
		t.Fatal(err)
	}
	if q.Version != "sub2api-ccf997c7-v1" || fmt.Sprintf("%x", sha256.Sum256([]byte(q.Prompt))) != "df1a06950b3883d44cb2f1046164281bd7e6ba09c6792c3042e6658dfbb30eb5" {
		t.Fatal("the candy reference must change version when its prompt changes")
	}
	// Enumerate every possible draw without the required cross-shape fruit pair.
	// Shape can be felt, so the contestant may choose both shape quotas in advance.
	var failing [25][18]bool
	for roundApple := 0; roundApple <= 7; roundApple++ {
		for roundPeach := 0; roundPeach <= 9; roundPeach++ {
			for starApple := 0; starApple <= 7; starApple++ {
				for starPeach := 0; starPeach <= 6; starPeach++ {
					if (roundApple > 0 && starPeach > 0) || (roundPeach > 0 && starApple > 0) {
						continue
					}
					for roundWatermelon := 0; roundWatermelon <= 8; roundWatermelon++ {
						for starWatermelon := 0; starWatermelon <= 4; starWatermelon++ {
							failing[roundApple+roundPeach+roundWatermelon][starApple+starPeach+starWatermelon] = true
						}
					}
				}
			}
		}
	}
	minimum := 42
	for round := 0; round <= 24; round++ {
		for star := 0; star <= 17; star++ {
			if !failing[round][star] {
				minimum = min(minimum, round+star)
			}
		}
	}
	if strconv.Itoa(minimum) != q.Answer || minimum != 21 || failing[9][12] {
		t.Fatalf("frozen answer %q disagrees with bounded solution %d", q.Answer, minimum)
	}
	for _, tc := range []struct{ output, verdict string }{
		{"21", "correct"}, {" 21\n", "correct"}, {"121", "incorrect"}, {"20", "incorrect"},
		{"21 or 22", "unknown"}, {"The answer is 21.", "unknown"}, {"21个", "unknown"},
	} {
		if verdict, _ := DeterministicJudge(q, tc.output); verdict != tc.verdict {
			t.Errorf("%q: got %s, want %s", tc.output, verdict, tc.verdict)
		}
	}
	if _, err = FreezeQuestion(RunConfig{QuestionID: q.ID, Prompt: q.Prompt + " Changed."}); err == nil {
		t.Fatal("an edited candy question inherited the built-in answer")
	}
	custom, err := FreezeQuestion(RunConfig{QuestionID: "custom", Prompt: q.Prompt + " Changed."})
	if err != nil || custom.Answer != "" {
		t.Fatal("custom question inherited the candy answer", err)
	}
}

func TestQuestionTestDefaultsToManualWithoutChangingRules(t *testing.T) {
	en, host := lifeFixture(t)
	host.execute = func(_ context.Context, e Execution) Sample {
		return Sample{ExecutionStatus: "completed", Output: "21", Evidence: Evidence{Verified: true, RequestedModel: e.Model, ResolvedModel: e.Model, AuthID: e.AuthID, Path: "offline", PayloadFingerprint: "stable", PayloadConfigVersion: "payload-v1"}}
	}
	cfg := lifeConfig()
	cfg.QuestionID = "logic-candy"
	cfg.Judge = Judge{}
	cfg.Samples = 2
	run, err := en.Submit(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if run.Config.Judge.Mode != "manual" || len(host.calls) != 0 {
		t.Fatal("submission did not freeze a manual question test")
	}
	en.execute(<-en.queue)
	result, err := en.Run(run.ID)
	if err != nil || result.Status != "completed" || len(result.Samples) != 2 {
		t.Fatal("question test did not complete", result, err)
	}
	for i, sample := range result.Samples {
		if sample.QualityVerdict != "unknown" || host.calls[i].Prompt != run.Config.Question.Prompt || strings.Contains(host.calls[i].Prompt, "21") {
			t.Fatal("manual test graded, mutated the question, or leaked its answer")
		}
	}
	if host.pauses != 0 || result.ActionStatus != "not_applicable" {
		t.Fatal("manual question test caused a credential action")
	}
	for _, rule := range en.Rules() {
		if rule.Judge.Mode != "deterministic" {
			t.Fatal("manual default changed existing rule semantics")
		}
	}
}
