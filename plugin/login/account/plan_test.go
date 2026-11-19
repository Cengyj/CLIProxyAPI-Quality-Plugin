package account

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/MoeForge/moego/base64"
)

func TestPlanLabelsPreserveUnknownAndReferenceAliases(t *testing.T) {
	for input, want := range map[string]string{" CHATGPT_PLUS ": "plus", "business": "team", "self_serve_business": "team", "pro-lite": "prolite", "pro_lite": "prolite", "self_serve_business_prolite": "business_premium", "education": "edu", "unknown": "unknown"} {
		if got, valid := ParsePlan(input); !valid || got != want {
			t.Errorf("%q normalized to %q/%v, want %q", input, got, valid, want)
		}
	}
	for _, value := range []string{"", "other", "plus\n<script>", "UNKNOWN_PLAN"} {
		if got, valid := ParsePlan(value); valid || got != "unknown" {
			t.Fatalf("invalid exclusion %q was accepted", value)
		}
	}
	token := func(claims string) string {
		return "e30." + base64.RawURLEncoding.EncodeToString([]byte(claims)) + ".fixture"
	}
	a := &Auth{Provider: "codex", Metadata: map[string]any{"plan_type": "unknown", "id_token": token(`{"https://api.openai.com/auth":{"chatgpt_plan_type":"chatgpt_plus"}}`), "access_token": token(`{"auth":{"plan_type":"free"}}`)}}
	if got := LoginPlanTypes(a); !reflect.DeepEqual(got, []string{"plus", "free"}) {
		t.Fatalf("known token plans were lost or collapsed: %v", got)
	}
	if got := LoginPlanType(&Auth{Provider: "codex", Metadata: map[string]any{"plan_type": "not-recognized"}}); got != "unknown" {
		t.Fatal("unrecognized plan became a free subscription")
	}
	for _, metadata := range []map[string]any{
		{"https://api.openai.com/auth.chatgpt_plan_type": "chatgpt_pro"},
		{"id_token": token(`{"https://api.openai.com/auth.chatgpt_plan_type":"chatgpt_pro"}`)},
	} {
		if got := LoginPlanType(&Auth{Provider: "codex", Metadata: metadata}); got != "pro" {
			t.Fatal("flat reference plan field was not recognized")
		}
	}
}

func TestFinalPlanExclusionReceiptAllowsOnlyUnwrittenRetry(t *testing.T) {
	h := newRawHostFixture()
	m := newFixtureManager(t, h)
	before, err := m.LoginSnapshot(h.entry.ID)
	if err != nil {
		t.Fatal(err)
	}
	candidate, _ := m.GetByID(h.entry.ID)
	checks := 0
	request := LoginCommitRequest{CommitID: "plan-exclusion-final-gate", Candidate: candidate, Expected: before, Guard: func() error {
		checks++
		if checks == 4 {
			return errors.New("cpa_plan_excluded")
		}
		return nil
	}}
	result, err := m.CommitLoginCandidate(context.Background(), request)
	if err == nil || err.Error() != "cpa_plan_excluded" || result.State != "upload_excluded" || h.saves != 0 {
		t.Fatalf("last exclusion gate did not safely abort: %+v %v saves=%d", result, err, h.saves)
	}
	receipt, err := m.LoginCommitStatus(request.CommitID)
	if err != nil || receipt.State != "upload_excluded" {
		t.Fatal("known pre-write exclusion did not have a durable receipt")
	}
	request.Guard = nil
	result, err = m.CommitLoginCandidate(context.Background(), request)
	if err != nil || result.State != "committed" || h.saves != 1 {
		t.Fatalf("explicit retained-candidate retry failed: %+v %v", result, err)
	}
	if _, err = m.CommitLoginCandidate(context.Background(), request); err == nil || h.saves != 1 {
		t.Fatal("successful retry was replayed")
	}
}
