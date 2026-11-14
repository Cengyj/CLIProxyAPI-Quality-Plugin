package quality

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
	bolt "go.etcd.io/bbolt"
)

func TestRunPaginationIsStableFilteredAndDetached(t *testing.T) {
	en, _ := lifeFixture(t)
	at := en.now()
	for _, id := range []string{"c", "a", "b"} {
		en.runs[id] = &Run{ID: id, Source: "manual_test", CreatedAt: at, Config: Frozen{RunConfig: lifeConfig()}, Samples: []Sample{{Output: strings.Repeat(id, 128)}}}
	}
	en.runs["probe"] = &Run{ID: "probe", Source: "scheduled_probe", CreatedAt: at.Add(time.Second)}
	result, err := en.QueryRuns(url.Values{"source": {"manual_test"}, "offset": {"1"}, "limit": {"1"}})
	if err != nil {
		t.Fatal(err)
	}
	entries := result["items"].([]Run)
	if result["total"] != 3 || len(entries) != 1 || entries[0].ID != "b" {
		t.Fatal(result)
	}
	entries[0].Config.Models[0] = "mutated"
	entries[0].Samples[0].Output = "mutated"
	if en.runs["b"].Config.Models[0] == "mutated" || en.runs["b"].Samples[0].Output == "mutated" {
		t.Fatal("query leaked mutable state")
	}
	beyond, err := en.QueryRuns(url.Values{"offset": {"100"}, "limit": {"100"}})
	if err != nil || len(beyond["items"].([]Run)) != 0 || beyond["offset"] != 4 {
		t.Fatal(beyond, err)
	}
	if _, err = en.QueryRuns(url.Values{"source": {"production"}}); err == nil {
		t.Fatal("invalid run source accepted")
	}
}

func TestHistoryAndMetricsCorruptionReturnUnavailable(t *testing.T) {
	for _, endpoint := range []string{"history", "metrics"} {
		t.Run(endpoint, func(t *testing.T) {
			en, _ := lifeFixture(t)
			bucket := "audit"
			if endpoint == "metrics" {
				bucket = "usage"
			}
			if err := en.store.Write(func(tx *bolt.Tx) error { return tx.Bucket([]byte(bucket)).Put([]byte("broken"), []byte(`{`)) }); err != nil {
				t.Fatal(err)
			}
			response := en.Handle(context.Background(), pluginapi.ManagementRequest{Method: "GET", Path: "/v8/management/plugins/model-quality/" + endpoint})
			if response.StatusCode != http.StatusServiceUnavailable || !strings.Contains(string(response.Body), `"code":"unavailable"`) {
				t.Fatal(response.StatusCode, string(response.Body))
			}
		})
	}
}

func TestMetricsUnknownProvenanceAndQualityDenominators(t *testing.T) {
	en, _ := lifeFixture(t)
	en.runs["sample"] = &Run{Config: Frozen{RunConfig: RunConfig{Models: []string{"a"}, Samples: 4}}, Samples: []Sample{
		{ExecutionStatus: "completed", QualityVerdict: "correct", Usage: &Usage{Total: 10}},
		{ExecutionStatus: "completed", QualityVerdict: "incorrect", Usage: &Usage{Total: 20}},
		{ExecutionStatus: "failed", QualityVerdict: "unknown"},
	}}
	for id, source := range map[string]string{"legacy": "", "untrusted": "arbitrary", "business": "production"} {
		if err := en.store.Put("usage", id, UsageEvent{ID: id, Source: source}); err != nil {
			t.Fatal(err)
		}
	}
	stats, err := en.Metrics()
	if err != nil {
		t.Fatal(err)
	}
	if stats["quality_pass_rate"] != float64(.5) || stats["coverage"] != float64(.5) || stats["unknown_usage"] != 1 {
		t.Fatal(stats)
	}
	sources := stats["traffic_sources"].(map[string]map[string]int64)
	if sources["unknown"]["attempts"] != 2 || sources["production"]["attempts"] != 1 || sources["production"]["unknown_usage"] != 1 || len(sources) != 2 {
		t.Fatal(sources)
	}
}

func TestErrorClassificationDoesNotInspectIncidentalMessageText(t *testing.T) {
	for _, tc := range []struct {
		err    error
		code   string
		status int
	}{
		{fmt.Errorf("model not available for target: conflict-model"), "invalid_request", 400},
		{fmt.Errorf("invalid unavailable model name"), "invalid_request", 400},
		{fmt.Errorf("blocked: paused credential"), "blocked", 409},
		{fmt.Errorf("host_call_failed: %w", &pluginabi.Error{Code: "host_call_failed", Message: "action_conflict", HTTPStatus: 500}), "conflict", 409},
		{fmt.Errorf("host_call_failed: %w", &pluginabi.Error{Code: "host_call_failed", Message: "internal unavailable data", HTTPStatus: 500}), "unavailable", 503},
		{fmt.Errorf("storage_limit: retention required"), "storage_limit", 507},
		{fmt.Errorf("read state: %w", fmt.Errorf("unavailable: database closed")), "unavailable", 503},
	} {
		code, status := classifyError(tc.err)
		if code != tc.code || status != tc.status {
			t.Fatal(tc.err, code, status)
		}
	}
}

func TestPresetMutationGuardAndInputOwnership(t *testing.T) {
	en, _ := lifeFixture(t)
	config := lifeConfig()
	preset, err := en.SavePreset("saved", config)
	if err != nil {
		t.Fatal(err)
	}
	config.Models[0] = "changed"
	if preset.Config.Models[0] == "changed" {
		t.Fatal("preset shares caller memory")
	}
	en.mu.Lock()
	en.closed = true
	en.mu.Unlock()
	if _, err = en.SavePreset("after close", lifeConfig()); err == nil {
		t.Fatal("closed engine saved preset")
	}
	if err = en.DeletePreset(preset.ID); err == nil {
		t.Fatal("closed engine removed preset")
	}
}

func TestSnapshotsDetachNestedRunAndRuleState(t *testing.T) {
	temperature, ttft := 0.2, int64(12)
	now := time.Now().UTC()
	original := &Run{Config: Frozen{RunConfig: RunConfig{Models: []string{"m"}, Parameters: Parameters{Temperature: &temperature}}}, CompletedAt: &now, Samples: []Sample{{TTFTMS: &ttft, Usage: &Usage{Total: 4}, JudgeUsage: &Usage{Total: 2}}}, Controls: []Sample{{Usage: &Usage{Total: 3}}}}
	copy := cloneRun(original)
	*copy.Config.Parameters.Temperature = 1
	copy.Config.Models[0] = "changed"
	*copy.CompletedAt = time.Time{}
	*copy.Samples[0].TTFTMS = 99
	copy.Samples[0].Usage.Total = 999
	copy.Samples[0].JudgeUsage.Total = 999
	copy.Controls[0].Usage.Total = 999
	if temperature != 0.2 || ttft != 12 || now.IsZero() || original.Config.Models[0] != "m" || original.Samples[0].Usage.Total != 4 || original.Samples[0].JudgeUsage.Total != 2 || original.Controls[0].Usage.Total != 3 {
		t.Fatal("snapshot leaked a nested mutable field")
	}
	rule := &Rule{RunConfig: original.Config.RunConfig}
	cloned := cloneRule(rule)
	*cloned.Parameters.Temperature = 0.8
	cloned.Models[0] = "rule change"
	if temperature != 0.2 || rule.Models[0] != "m" {
		t.Fatal("rule snapshot leaked config")
	}
}

func TestRuleDeletionAndAuditShareOneTransaction(t *testing.T) {
	en, _, owner := manualOnlyFixture(t)
	if err := en.DeleteRule(context.Background(), owner.ID, owner.Revision); err != nil {
		t.Fatal(err)
	}
	entries, err := en.History()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range entries {
		if entry.RuleID == owner.ID && entry.Kind == "rule_deleted" {
			found = true
		}
	}
	if !found || en.rules[owner.ID] != nil {
		t.Fatal("deletion missing its audit")
	}
	second, err := en.SaveRule(context.Background(), Rule{RunConfig: lifeConfig(), Enabled: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = en.store.Close(); err != nil {
		t.Fatal(err)
	}
	if err = en.DeleteRule(context.Background(), second.ID, second.Revision); err == nil {
		t.Fatal("failed deletion claimed success")
	}
	if en.rules[second.ID] == nil || en.fault == nil {
		t.Fatal("failed deletion published state or failed to stop writes")
	}
}
