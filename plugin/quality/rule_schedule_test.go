package quality

import (
	"context"
	"testing"
	"time"

	"github.com/MoeForge/moego/json"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

func TestRuleCronUsesServerWallClock(t *testing.T) {
	for _, tc := range []struct{ zone, expr, from, want string }{
		{"Asia/Shanghai", defaultRuleCron, "2026-11-19T00:17:42Z", "2026-11-19T00:30:00Z"},
		{"Asia/Shanghai", defaultRuleCron, "2026-11-19T00:30:00Z", "2026-11-19T01:00:00Z"},
		{"Asia/Kathmandu", defaultRuleCron, "2026-11-19T00:05:09Z", "2026-11-19T00:15:00Z"},
		{"Asia/Shanghai", "0 9 * * *", "2026-11-19T00:05:00Z", "2026-11-19T01:00:00Z"},
		{"America/New_York", "0 9 * * *", "2026-03-07T14:00:00Z", "2026-03-08T13:00:00Z"},
	} {
		loc, err := time.LoadLocation(tc.zone)
		if err != nil {
			t.Fatal(err)
		}
		from, err := time.Parse(time.RFC3339, tc.from)
		if err != nil {
			t.Fatal(err)
		}
		got, err := ruleNextInstant(&Rule{Cron: tc.expr}, from, loc)
		if err != nil || got.Location() != time.UTC || got.Format(time.RFC3339) != tc.want {
			t.Fatalf("%s %s after %s: got %s, %v; want %s", tc.zone, tc.expr, tc.from, got, err, tc.want)
		}
	}
}

func TestRuleCronDefaultsValidationAndPermissions(t *testing.T) {
	en, h, _ := scopeFixture(t, scopedTarget("one", "a"))
	write := func(method string, body map[string]any, status int) ruleView {
		t.Helper()
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		resp := en.Handle(context.Background(), pluginapi.ManagementRequest{Method: method, Path: "/rules", Body: raw})
		if resp.StatusCode != status {
			t.Fatalf("%s: got %d %s, wanted %d", method, resp.StatusCode, resp.Body, status)
		}
		var out ruleView
		if err = json.Unmarshal(resp.Body, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	b := map[string]any{"enabled": true, "target_scope": scopeAll, "models": []string{"a"}, "question_id": "logic-boxes"}
	manual := write("POST", b, 200)
	if manual.Cron != defaultRuleCron || manual.Timezone != "" || manual.IntervalSeconds != 0 || manual.SchedulingEnabled || !manual.NextRun.IsZero() || manual.NextRunLocal != "" || en.cfg.SchedulingEnabled {
		t.Fatal("unexpected default plan or permissions", manual)
	}
	for _, bad := range []any{"", "  ", nil, 5, "30 2 *", "0 0 * * * *", "@daily", "TZ=UTC 0 9 * * *", "65 9 * * *", "*/0 * * * *", "0 9 31 2 *"} {
		b["cron"] = bad
		write("POST", b, 400)
	}
	b["cron"] = "0  9\t* * *"
	b["timezone"] = "Asia/Shanghai"
	write("POST", b, 400)
	delete(b, "timezone")
	b["interval_seconds"] = 60
	write("POST", b, 400)
	delete(b, "interval_seconds")
	b["id"], b["revision"], b["scheduling_enabled"] = manual.ID, manual.Revision, true
	custom := write("PUT", b, 200)
	if custom.Cron != "0 9 * * *" || custom.NextRun.IsZero() || custom.NextRunLocal == "" {
		t.Fatal("missing normalized schedule", custom)
	}
	delete(b, "cron")
	b["revision"], b["name"] = custom.Revision, "changed"
	b["next_run"], b["next_run_local"] = "1970-01-01T00:00:00Z", "ignored"
	en.now = func() time.Time { return custom.NextRun.Add(24 * time.Hour) }
	edit := write("PUT", b, 200)
	if edit.Cron != custom.Cron || !edit.NextRun.Equal(custom.NextRun) || edit.NextRunLocal != custom.NextRunLocal {
		t.Fatal("unrelated edit re-anchored the schedule", edit)
	}
	en.Tick(en.now())
	if len(en.queue) != 0 || h.executeCalls != 0 {
		t.Fatal("global scheduling permission bypassed")
	}
	b["revision"], b["scheduling_enabled"] = edit.Revision, false
	stopped := write("PUT", b, 200)
	if stopped.Cron != custom.Cron || !stopped.NextRun.IsZero() || stopped.NextRunLocal != "" {
		t.Fatal("stopping lost the schedule or retained a due time", stopped)
	}
	if err := en.setScheduling(false, true); err != nil {
		t.Fatal(err)
	}
	en.Tick(en.now())
	if len(en.queue) != 0 {
		t.Fatal("rule-level opt-out bypassed")
	}
	b["revision"], b["scheduling_enabled"] = stopped.Revision, true
	restarted := write("PUT", b, 200)
	en.Tick(restarted.NextRun.Add(-time.Second))
	if len(en.queue) != 0 || !restarted.NextRun.After(en.now()) {
		t.Fatal("early execution or stale due time")
	}
	en.Tick(restarted.NextRun)
	en.Tick(restarted.NextRun)
	if len(en.queue) != 1 || !en.rules[manual.ID].NextRun.After(restarted.NextRun) {
		t.Fatal("expected exactly one admission at the boundary")
	}
}

func TestLegacyRuleSchedulePreservedUntilConverted(t *testing.T) {
	for _, expr := range []string{"", "0 9 * * *"} {
		en, _, r := scopeFixture(t)
		// Seed a real pre-upgrade representation, including the inactive interval on a Cron rule.
		r.IntervalSeconds, r.Timezone, r.Cron, r.SchedulingEnabled = 750, "Asia/Kathmandu", expr, true
		r.NextRun = nextRun(*r, en.now())
		en.rules[r.ID] = cloneRule(r)
		anchor := r.NextRun
		if anchor.IsZero() || (expr == "" && anchor.Sub(en.now()) != 750*time.Second) || (expr != "" && viewRule(*r).NextRunLocal != "2026-11-14T09:00:00+05:45") {
			t.Fatal("legacy schedule semantics changed", r)
		}
		now := en.now().Add(2 * time.Minute)
		en.now = func() time.Time { return now }
		in := *r
		in.Name, in.IntervalSeconds, in.Timezone, in.Cron = "renamed", 0, "", ""
		saved, err := en.SaveRule(context.Background(), in, true)
		if err != nil || !sameRuleSchedule(saved, r) || !saved.NextRun.Equal(anchor) {
			t.Fatal("legacy plan changed on omission", saved, err)
		}
		in = *saved
		if saved, err = en.SaveRule(context.Background(), in, true); err != nil || !saved.NextRun.Equal(anchor) {
			t.Fatal("legacy round-trip", saved, err)
		}
		in = *saved
		in.IntervalSeconds++
		if _, err = en.SaveRule(context.Background(), in, true); err == nil {
			t.Fatal("accepted a legacy plan mutation")
		}
		in.IntervalSeconds, in.Timezone = 0, ""
		if expr == "" {
			in.Cron = defaultRuleCron
		}
		converted, err := en.SaveRule(context.Background(), in, true)
		if err != nil || converted.IntervalSeconds != 0 || converted.Timezone != "" || converted.Cron != in.Cron || !converted.NextRun.Equal(nextRun(in, now)) {
			t.Fatal("explicit conversion failed", converted, err)
		}
		var stored Rule
		if err = en.store.Get("rules", in.ID, &stored); err != nil || !sameRuleSchedule(&stored, converted) || !stored.NextRun.Equal(converted.NextRun) {
			t.Fatal("converted plan not persisted", err)
		}
	}
}

func TestInvalidStoredCronDoesNotFallBackOrDispatch(t *testing.T) {
	en, _, r := scopeFixture(t, scopedTarget("one", "a"))
	r.Cron, r.IntervalSeconds, r.SchedulingEnabled, r.NextRun = "75 * * * *", 3600, true, en.now()
	en.rules[r.ID] = r
	en.cfg.SchedulingEnabled = true
	en.Tick(en.now())
	if len(en.queue) != 0 || !en.rules[r.ID].NextRun.IsZero() {
		t.Fatal("invalid schedule admitted or rearmed")
	}
}
