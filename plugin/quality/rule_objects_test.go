package quality

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestPreviewAndDispatchMatchObjectTypes(t *testing.T) {
	ctx := context.Background()
	file := scopedTarget("file", "a", "b")
	a := policyProvider()
	a.ProviderGroupID, a.ProviderDisableUnit = "group-a", "group"
	b := a
	b.AuthID, b.AuthIndex = "second", "ix-second"
	en, host, base := scopeFixture(t, file, a, b)
	for _, tc := range []struct {
		kinds            []string
		oauth, providers int
	}{{[]string{objectOAuth}, 1, 0}, {[]string{objectProvider}, 0, 2}, {[]string{objectOAuth, objectProvider}, 1, 2}} {
		r := *base
		r.ID, r.Revision = "", 0
		r.ObjectTypes = tc.kinds
		calls := host.executeCalls
		preview, err := en.previewRuleScope(ctx, r)
		if err != nil || !preview.Verified || preview.Candidates[objectOAuth] != tc.oauth || preview.Candidates[objectProvider] != tc.providers || preview.Count != tc.oauth+tc.providers {
			t.Fatal(preview, err)
		}
		saved, err := en.saveRule(ctx, r, false, "start")
		if err != nil || host.executeCalls != calls {
			t.Fatal(saved, err)
		}
		for _, scheduled := range []bool{false, true} {
			receipt, err := en.dispatchRule(ctx, saved.ID, saved.Revision, scheduled)
			if err != nil || receipt.Count != preview.Count || receipt.Candidates[objectOAuth] != tc.oauth || receipt.Candidates[objectProvider] != tc.providers {
				t.Fatal(receipt, err)
			}
			for task := en.nextRunnableLocked(); task != nil; task = en.nextRunnableLocked() {
				en.executeTask(task)
			}
		}
		if host.executeCalls-calls != 2*2*(tc.oauth+tc.providers) {
			t.Fatal("unexpected observed number of model calls", host.executeCalls-calls)
		}
	}
	r := *base
	r.ObjectTypes = []string{}
	if _, err := en.previewRuleScope(ctx, r); err == nil {
		t.Fatal("empty object selection accepted")
	}
	r.ObjectTypes = []string{objectProvider}
	r.TargetScope, r.AuthID, r.TargetAuthIndex, r.Provider = "single", a.AuthID, a.AuthIndex, a.Provider
	for _, want := range []int{1, 2} {
		if want == 2 {
			r.AuthorizedGroups = []string{a.ProviderGroupID}
		}
		preview, err := en.previewRuleScope(ctx, r)
		if err != nil || preview.Count != want {
			t.Fatal(preview, err)
		}
	}
	r.AuthID, r.Provider, r.TargetAuthIndex, r.TargetGroup = "", "", "", a.ProviderGroupID
	r.AuthorizedGroups = nil
	if preview, err := en.previewRuleScope(ctx, r); err != nil || preview.Count != 2 {
		t.Fatal(preview, err)
	}
	host.targets = host.targets[:2]
	if preview, err := en.previewRuleScope(ctx, r); err != nil || preview.Count != 1 {
		t.Fatal("did not re-match current group membership", preview, err)
	}
	a.ProviderDisableUnit, b.ProviderDisableUnit = "route", "route"
	host.targets = []Target{file, a, b}
	r.ID, r.Revision = "", 0
	saved, err := en.SaveRule(ctx, r, false)
	if err != nil {
		t.Fatal("selecting all routes in a verified group should not require a group-wide status switch", err)
	}
	if p, err := en.previewRuleScope(ctx, *saved); err != nil || p.Count != 2 {
		t.Fatal(p, err)
	}
	saved.AuthorizedGroups = []string{a.ProviderGroupID}
	if _, err := en.SaveRule(ctx, *saved, true); err == nil {
		t.Fatal("group-wide writes allowed for a per-route family")
	}
}

func TestLegacyScopesFreezeActualPermissionsOnFirstLoad(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "off", true: "on"}[enabled], func(t *testing.T) {
			ctx := context.Background()
			provider := policyProvider()
			file := scopedTarget("file", "a", "b")
			disabled := scopedTarget("manual", "a", "b")
			disabled.ManualDisabled, disabled.RuntimeStatus = true, "disabled"
			en, host, rule := scopeFixture(t, file, provider, disabled)
			legacy := *rule
			legacy.ObjectTypes = nil
			legacy.TargetScope, legacy.SchedulingEnabled, legacy.Cron = scopeAllCredentials, true, "*/20 * * * *"
			legacy.NextRun = time.Now().UTC().Add(time.Hour)
			fixed := legacy
			fixed.ID, fixed.TargetScope, fixed.AuthID, fixed.Provider = "old-fixed", "single", provider.AuthID, provider.Provider
			c := en.cfg
			c.IncludeProviderTargets = enabled
			for _, entry := range []struct {
				bucket, key string
				value       any
			}{
				{"meta", "settings", c}, {"meta", "config_input", fingerprint(en.cfg)}, {"meta", ruleTimersKey, map[string]bool{legacy.ID: true, fixed.ID: true}}, {"rules", legacy.ID, legacy}, {"rules", fixed.ID, fixed},
			} {
				if err := en.store.Put(entry.bucket, entry.key, entry.value); err != nil {
					t.Fatal(err)
				}
			}
			for range 2 {
				en.rules = map[string]*Rule{}
				if err := en.load(); err != nil {
					t.Fatal(err)
				}
				r := en.rules[legacy.ID]
				if !r.LegacyScope || !r.LegacyProviderObservation || len(r.AuthorizedGroups) != 0 || r.Revision != legacy.Revision || !r.NextRun.Equal(legacy.NextRun) || !en.timerAllowed(r) || contains(r.ObjectTypes, objectProvider) != enabled {
					t.Fatal(r)
				}
				preview, err := en.previewRuleScope(ctx, *r)
				want := 1
				if enabled {
					want = 2
				}
				if err != nil || preview.Count != want || preview.Skipped != 1 || preview.SkipDetails[0].AuthID != disabled.AuthID || host.executeCalls != 0 {
					t.Fatal(preview, err)
				}
				restricted := en.rules[fixed.ID]
				if !enabled {
					if _, err = en.dispatchRule(ctx, fixed.ID, restricted.Revision, true); err == nil || len(en.runs) != 0 {
						t.Fatal("old excluded timer executed after upgrade", err)
					}
				}
			}
			r := cloneRule(en.rules[legacy.ID])
			r.Name = "rename only"
			saved, err := en.SaveRule(ctx, *r, true)
			if err != nil || !saved.LegacyScope || !saved.LegacyProviderObservation {
				t.Fatal(saved, err)
			}
			saved.AdoptScope = true
			saved.ObjectTypes = []string{objectOAuth, objectProvider}
			saved, err = en.SaveRule(ctx, *saved, true)
			if err != nil || saved.LegacyScope || saved.LegacyProviderObservation || !en.rules[fixed.ID].LegacyScope || !slices.Equal(saved.ObjectTypes, []string{objectOAuth, objectProvider}) {
				t.Fatal(saved, err)
			}
		})
	}
}

type unreadableRuleDirectory struct{ Host }

type partialProviderDirectory struct{ Host }

func (h partialProviderDirectory) Targets(ctx context.Context) ([]Target, error) {
	if status, ok := ctx.Value(providerInventoryStatusKey{}).(*providerInventoryStatus); ok {
		status.Verified = false
	}
	return h.Host.Targets(ctx)
}

func TestMissingProviderDirectoryPreservesFileCandidates(t *testing.T) {
	ctx := context.Background()
	file, provider := scopedTarget("file", "a", "b"), policyProvider()
	en, h, rule := scopeFixture(t, file, provider)
	en.host = partialProviderDirectory{en.host}
	for _, tc := range []struct {
		kinds       []string
		verified    bool
		files, skip int
	}{
		{[]string{objectOAuth}, true, 1, 0}, {[]string{objectProvider}, false, 0, 1}, {[]string{objectOAuth, objectProvider}, false, 1, 1},
	} {
		r := *rule
		r.ObjectTypes = tc.kinds
		preview, err := en.previewRuleScope(ctx, r)
		if err != nil || preview.Verified != tc.verified || preview.Count != tc.files || preview.Skipped != tc.skip {
			t.Fatal(preview, err)
		}
	}
	r := *rule
	r.TargetScope, r.AuthID, r.Provider = "single", file.AuthID, file.Provider
	r.ObjectTypes = []string{objectOAuth, objectProvider}
	p, err := en.previewRuleScope(ctx, r)
	if err != nil || !p.Verified || p.Count != 1 {
		t.Fatal("fixed file incorrectly depended on provider configuration", p, err)
	}
	r = *rule
	r.ObjectTypes = []string{objectOAuth, objectProvider}
	armed, err := en.saveRule(ctx, r, true, "start")
	if err != nil {
		t.Fatal(err)
	}
	en.Tick(armed.NextRun)
	last := en.rules[armed.ID].LastScopeDispatch
	if last.Matched != 1 || last.Skipped != 1 || last.Reason == "" {
		t.Fatal("timer lost directory coverage", last)
	}
	for task := en.nextRunnableLocked(); task != nil; task = en.nextRunnableLocked() {
		en.executeTask(task)
	}
	if h.executeCalls != 2 {
		t.Fatal("unsafe provider executed, or independent file skipped", h.executeCalls)
	}
}

func (unreadableRuleDirectory) Targets(context.Context) ([]Target, error) {
	return nil, errors.New("synthetic directory unavailable")
}

func TestUnverifiedPreviewStillPermitsSavingDynamicRules(t *testing.T) {
	ctx := context.Background()
	en, h, rule := scopeFixture(t, scopedTarget("one", "a", "b"))
	en.host = unreadableRuleDirectory{en.host}
	preview, err := en.previewRuleScope(ctx, *rule)
	if err != nil || preview.Verified || !strings.Contains(preview.Reason, "待核实") {
		t.Fatal(preview, err)
	}
	r := *rule
	r.ID, r.Revision = "", 0
	r.ObjectTypes = []string{objectProvider}
	if _, err := en.SaveRule(ctx, r, false); err != nil || h.executeCalls != 0 {
		t.Fatal(err)
	}
}

func TestRuleDefaultReasoningIsOmittedOnDispatch(t *testing.T) {
	ctx := context.Background()
	en, _, base := scopeFixture(t, scopedTarget("file", "a", "b"))
	if base.Parameters.ReasoningEffort != "high" {
		t.Fatal(base.Parameters)
	}
	base.Parameters.ReasoningEffort = "default"
	saved, err := en.SaveRule(ctx, *base, true)
	if err != nil || saved.Parameters.ReasoningEffort != "" {
		t.Fatal(saved, err)
	}
	receipt, err := en.dispatchRule(ctx, saved.ID, saved.Revision, false)
	if err != nil || receipt.Count != 1 {
		t.Fatal(receipt, err)
	}
	task := en.nextRunnableLocked()
	if task == nil || task.frozen.Parameters.ReasoningEffort != "" {
		t.Fatal("default reasoning sent as a wire override", task)
	}
	task.cancel()
	en.executeTask(task)
}
