package quality

import (
	"errors"
	"math"
	"os"
	"testing"
	"time"
)

func TestBudgetLateUsageRemainsOnAdmissionDay(t *testing.T) {
	en, _ := lifeFixture(t)
	en.cfg.DailyTokens = 1000
	dayOne, err := en.reserve(64)
	if err != nil {
		t.Fatal(err)
	}
	en.now = func() time.Time { return time.Date(2026, 11, 14, 0, 0, 1, 0, time.UTC) }
	dayTwo, err := en.reserve(64)
	if err != nil {
		t.Fatal(err)
	}
	if err = en.account(dayOne, &Usage{Total: 2000}); err != nil {
		t.Fatal(err)
	}
	if err = en.account(dayTwo, &Usage{Total: 80}); err != nil {
		t.Fatal(err)
	}
	if en.budget.Day != dayTwo || en.budget.Calls != 1 || en.budget.KnownTokens != 80 || en.budget.ReservedTokens != 64 {
		t.Fatal("prior-day completion polluted the current-day budget", en.budget)
	}
	var prior Budget
	if err = en.store.Get("meta", "budget:"+dayOne, &prior); err != nil || prior.KnownTokens != 2000 || prior.Calls != 1 {
		t.Fatal("prior-day usage was discarded", prior, err)
	}
	if _, err = en.reserve(64); err != nil {
		t.Fatal("prior-day usage incorrectly exhausted current-day budget", err)
	}
}

func TestBudgetLedgerRetentionProtectsUnfinishedCalls(t *testing.T) {
	en, _ := lifeFixture(t)
	en.cfg.RetentionDays = 1
	oldDay, err := en.reserve(64)
	if err != nil {
		t.Fatal(err)
	}
	en.now = func() time.Time { return time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC) }
	if _, err = en.reserve(64); err != nil {
		t.Fatal(err)
	}
	var retained Budget
	if err = en.store.Get("meta", "budget:"+oldDay, &retained); err != nil {
		t.Fatal("in-flight prior day was pruned", err)
	}
	if err = en.account(oldDay, nil); err != nil {
		t.Fatal(err)
	}
	if err = en.store.Get("meta", "budget:"+oldDay, &retained); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("expired settled day was retained indefinitely", err)
	}
}

func TestLegacyBudgetAndClockRollbackCannotResetDailyLimit(t *testing.T) {
	en, _ := lifeFixture(t)
	en.cfg.DailyCalls = 1
	oldDay := en.now().Format("2006-01-02")
	en.budget = Budget{Day: oldDay, Calls: 1, KnownTokens: 123, ReservedTokens: 64}
	if err := en.store.Put("meta", "budget", en.budget); err != nil {
		t.Fatal(err)
	}
	en.now = func() time.Time { return time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC) }
	if _, err := en.reserve(64); err != nil {
		t.Fatal(err)
	}
	var previous Budget
	if err := en.store.Get("meta", "budget:"+oldDay, &previous); err != nil || previous.KnownTokens != 123 {
		t.Fatal("legacy active budget was not preserved on first rollover", previous, err)
	}
	en.now = func() time.Time { return time.Date(2026, 11, 13, 13, 0, 0, 0, time.UTC) }
	if _, err := en.reserve(64); err == nil {
		t.Fatal("clock rollback reset an already exhausted day")
	}
}

func TestBudgetUsesUTCAndSaturatesUntrustedTokenTotals(t *testing.T) {
	en, _ := lifeFixture(t)
	en.now = func() time.Time {
		return time.Date(2026, 11, 14, 1, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))
	}
	day, err := en.reserve(64)
	if err != nil || day != "2026-11-13" {
		t.Fatal("budget date was not UTC", day, err)
	}
	if err = en.account(day, &Usage{Total: math.MaxInt64}); err != nil {
		t.Fatal(err)
	}
	if err = en.account(day, &Usage{Total: 1}); err != nil {
		t.Fatal(err)
	}
	if en.budget.KnownTokens != math.MaxInt64 {
		t.Fatal("token accounting overflowed", en.budget)
	}
	if _, err = en.reserve(64); err == nil {
		t.Fatal("overflowed token total bypassed budget")
	}
}
