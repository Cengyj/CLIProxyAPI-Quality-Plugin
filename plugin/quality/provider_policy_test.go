package quality

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/MoeForge/moego/json"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

func policyProvider() Target {
	t := scopedTarget("provider-one", "a", "b")
	t.IsProvider, t.Type, t.RuntimeStatus = true, "config_api_key", "registered"
	return t
}

// This host keeps file probes on the existing fake CPA and records native model requests separately.
type providerPolicyHost struct {
	Host
	managedHost
	native   *NativeHost
	provider Target
	requests []Execution
}

func (h *providerPolicyHost) Capabilities() HostCapabilities { return h.native.Capabilities() }
func (h *providerPolicyHost) Targets(ctx context.Context) ([]Target, error) {
	targets, err := h.Host.Targets(ctx)
	return append(targets, h.provider), err
}
func (h *providerPolicyHost) CredentialSnapshot(ctx context.Context, id, index string) (CredentialSnapshot, error) {
	if id == h.provider.AuthID {
		panic("provider was sent to the file snapshot API")
	}
	return h.managedHost.CredentialSnapshot(ctx, id, index)
}
func (h *providerPolicyHost) Execute(ctx context.Context, e Execution) Sample {
	if e.AuthID != h.provider.AuthID {
		return h.Host.Execute(ctx, e)
	}
	h.requests = append(h.requests, e)
	return Sample{ExecutionStatus: "completed", Output: "999"}
}

func TestConfiguredProviderMixedRuleObservesWithoutHandling(t *testing.T) {
	f := managementTestFixture(t)
	h := &providerPolicyHost{Host: f.host, managedHost: f.host, native: f.host, provider: policyProvider()}
	f.engine.host = h
	r := f.rule(t, deleteCredential, true, false)
	r.UpstreamErrorActions = map[string]string{"model_unsupported": deleteCredential}
	r, err := f.engine.SaveRule(context.Background(), *r, true)
	if err != nil {
		t.Fatal(err)
	}
	runs, err := f.round(t, r)
	if err != nil || len(runs) != 1 || runs[0].Config.AuthID != "one" || len(h.requests) != 0 {
		t.Fatal("default membership", runs, err)
	}
	r.ObjectTypes = []string{objectOAuth, objectProvider}
	r, err = f.engine.SaveRule(context.Background(), *r, true)
	if err != nil {
		t.Fatal(err)
	}
	runs, err = f.round(t, r)
	if err != nil || len(runs) != 2 {
		t.Fatal(runs, err)
	}
	probed := false
	for _, run := range runs {
		if run.Config.AuthID != h.provider.AuthID {
			continue
		}
		probed = true
		if !run.Config.ProviderObservation || run.Config.ManagedProbe != nil || len(run.Config.UpstreamErrorActions) != 0 || run.ActionStatus != "observe_only" || len(run.Samples) != 2 || run.Samples[0].QualityVerdict != "incorrect" {
			t.Fatal(run)
		}
	}
	if !probed || len(h.requests) != 2 || f.deletes != 0 || len(f.patches) != 0 {
		t.Fatal(h.requests, f.deletes, f.patches)
	}
	for _, req := range h.requests {
		if req.ManagedProbe != nil || req.Provider != h.provider.Provider || req.Source != "scheduled_probe" {
			t.Fatal(req)
		}
	}
	h.provider.Models = []string{"a"}
	receipt, err := f.engine.dispatchRule(context.Background(), r.ID, r.Revision, false)
	if err != nil || receipt.Count != 2 || receipt.Skipped != 0 {
		t.Fatal(receipt, err)
	}
	for task := f.engine.nextRunnableLocked(); task != nil; task = f.engine.nextRunnableLocked() {
		f.engine.executeTask(task)
	}
	if len(h.requests) != 2 || f.deletes != 0 {
		t.Fatal("provider missing-model check invoked an action")
	}
}

func TestConfiguredProviderFixedRuleDependenciesAndManualSelection(t *testing.T) {
	ctx := context.Background()
	provider, account := policyProvider(), scopedTarget("file", "a", "b")
	provider.ComparisonIdentity, account.ComparisonIdentity = "provider-account", "file-account"
	en, h, _ := scopeFixture(t, provider, account)
	fixed := Rule{Name: "subject", Enabled: true, RunConfig: RunConfig{AuthID: provider.AuthID, Provider: provider.Provider, Models: []string{"a"}, QuestionID: "logic-boxes"}}
	if _, err := en.SaveRule(ctx, fixed, false); err == nil {
		t.Fatal("created excluded provider rule")
	}
	run, err := en.Submit(ctx, fixed.RunConfig)
	if err != nil || run.Source != "manual_test" {
		t.Fatal(run, err)
	}
	en.executeTask(en.nextRunnableLocked())
	if h.executeCalls != 1 {
		t.Fatal("manual selection was excluded")
	}
	fixed.ObjectTypes = []string{objectProvider}
	r, err := en.SaveRule(ctx, fixed, false)
	if err != nil {
		t.Fatal(err)
	}
	withJudge := fixed
	withJudge.Name = "judge"
	withJudge.ObjectTypes = []string{objectOAuth}
	withJudge.AuthID = account.AuthID
	withJudge.Judge = Judge{Mode: "llm", AuthID: provider.AuthID, Provider: provider.Provider, Model: "a"}
	judgeRule, err := en.SaveRule(ctx, withJudge, false)
	if err != nil {
		t.Fatal(err)
	}
	withControl := withJudge
	withControl.Judge = Judge{Mode: "deterministic"}
	withControl.ControlAuthID = provider.AuthID
	withControl.ControlProvider = provider.Provider
	controlRule, err := en.SaveRule(ctx, withControl, false)
	if err != nil {
		t.Fatal(err)
	}
	eligible := []*Rule{r, judgeRule, controlRule}
	var pending []string
	for _, rule := range eligible {
		record, err := en.dispatchRule(ctx, rule.ID, rule.Revision, false)
		if err != nil || record.Count != 1 {
			t.Fatal(record, err)
		}
		pending = append(pending, record.RunID)
	}
	edit := *r
	edit.TargetScope, edit.AuthID, edit.Provider, edit.TargetAuthIndex = scopeAll, "", "", ""
	edit.ObjectTypes = []string{objectOAuth}
	if _, err := en.SaveRule(ctx, edit, true); err != nil {
		t.Fatal(err)
	}
	if en.runs[pending[0]].Status != "blocked" || en.runs[pending[0]].BlockReason != "rule_changed" {
		t.Fatal("edited rule kept its old queued target")
	}
	for _, id := range pending[1:] {
		if en.runs[id].Status != "queued" {
			t.Fatal("editing one rule stopped another rule", id)
		}
	}
}

func TestManualSupplierKeepsSelectedRoute(t *testing.T) {
	provider := policyProvider()
	provider.ProviderName = "Test supplier"
	for _, tc := range []struct {
		name   string
		change func(*Target)
	}{
		{"different key", func(t *Target) { t.AuthIndex = "replacement" }},
		{"different executor", func(t *Target) { t.Provider = "replacement" }},
		{"file with same id", func(t *Target) { t.IsProvider = false }},
		{"model removed", func(t *Target) { t.Models = []string{"b"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			en, host, _ := scopeFixture(t, provider)
			run, err := en.Submit(context.Background(), RunConfig{AuthID: provider.AuthID, Provider: provider.Provider, Models: []string{"a"}, QuestionID: "logic-candy"})
			if err != nil {
				t.Fatal(err)
			}
			tc.change(&host.targets[0])
			en.executeTask(en.nextRunnableLocked())
			stored, err := en.Run(run.ID)
			if err != nil || stored.Status != "blocked" || host.executeCalls != 0 {
				t.Fatalf("changed route was not blocked: %+v, %v", stored, err)
			}
			summary := runListSummary(stored)["config"].(map[string]any)
			if summary["provider_name"] != provider.ProviderName || summary["subject_auth_index"] != provider.AuthIndex {
				t.Fatalf("original selection missing from history: %+v", summary)
			}
		})
	}
}

func TestProviderTargetsSettingsCompatibility(t *testing.T) {
	en, _, _ := scopeFixture(t, policyProvider())
	req := func(method, route, body string) pluginapi.ManagementResponse {
		return en.Handle(context.Background(), pluginapi.ManagementRequest{Method: method, Path: "/v0/management/plugins/model-quality/" + route, Body: []byte(body)})
	}
	response := req(http.MethodGet, "settings", "")
	if response.StatusCode != 200 || !strings.Contains(string(response.Body), `"include_provider_targets":false`) {
		t.Fatal(string(response.Body))
	}
	for _, body := range []string{`{}`, `{"enabled":null}`, `{"enabled":"yes"}`, `{"enabled":true,"other":true}`} {
		if got := req("PUT", "settings/provider-targets", body); got.StatusCode == 200 {
			t.Fatal(body)
		}
	}
	response = req(http.MethodPut, "settings/provider-targets", `{"enabled":true}`)
	if response.StatusCode != 404 || en.cfg.IncludeProviderTargets {
		t.Fatal(string(response.Body))
	}
	c := en.Config()
	c.IncludeProviderTargets = false
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "include_provider_targets") {
		t.Fatal("changed fingerprint of existing host inputs")
	}
	if got := req("PUT", "settings", string(raw)); got.StatusCode != 200 || en.cfg.IncludeProviderTargets {
		t.Fatal("settings changed per-rule scope", string(got.Body))
	}
	var saved Config
	if err := en.store.Get("meta", "settings", &saved); err != nil || saved.IncludeProviderTargets {
		t.Fatal(saved, err)
	}
}

func TestProviderTimerRechecksCurrentRouteAndPermission(t *testing.T) {
	ctx := context.Background()
	provider := policyProvider()
	en, host, rule := scopeFixture(t, provider)
	rule.ObjectTypes = []string{objectProvider}
	rule, err := en.SaveRule(ctx, *rule, true)
	if err != nil {
		t.Fatal(err)
	}
	armed, err := en.setRuleTimer(ctx, rule.ID, rule.Revision, true)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := en.dispatchRule(ctx, armed.ID, armed.Revision, true)
	if err != nil || receipt.Count != 1 {
		t.Fatal(receipt, err)
	}
	task := en.nextRunnableLocked()
	if task == nil || !task.frozen.Scheduled || !task.frozen.ProviderObservation {
		t.Fatal(task)
	}
	if err := en.checkExecution(task.ctx, task.id, task.frozen); err != nil {
		t.Fatal(err)
	}
	host.targets[0].Models = []string{"a"}
	if err := en.checkExecution(task.ctx, task.id, task.frozen); err == nil {
		t.Fatal("lost model was readmitted")
	}
	host.targets[0] = provider
	host.targets[0].Reasons = []string{"provider_route_unverified"}
	if err := en.checkExecution(task.ctx, task.id, task.frozen); err == nil {
		t.Fatal("unverified provider was readmitted")
	}
	host.targets[0] = provider
	armed.ObjectTypes = []string{objectOAuth}
	armed, err = en.SaveRule(ctx, *armed, true)
	if err != nil {
		t.Fatal(err)
	}
	if task.ctx.Err() == nil {
		t.Fatal("active provider was not interrupted")
	}
	en.executeTask(task)
	if host.executeCalls != 0 || en.runs[task.id].BlockReason != "rule_changed" || en.runs[task.id].Status == "completed" {
		t.Fatal(en.runs[task.id])
	}
	receipt, err = en.dispatchRule(ctx, armed.ID, armed.Revision, true)
	if err != nil || receipt.Count != 0 || receipt.Skipped != 0 || len(en.runs) != 1 {
		t.Fatal("opted-out provider produced timer records", receipt, err)
	}
}
