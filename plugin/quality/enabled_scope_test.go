package quality

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

func TestEnabledScoredCredentialUsesActualProbeDespiteCPARuntimeHints(t *testing.T) {
	for _, scope := range []string{"single", scopeAll, scopeAllCredentials} {
		for _, mode := range []string{"actual 401", "misleading status", "unknown catalog"} {
			t.Run(scope+"/"+mode, func(t *testing.T) {
				f := managementTestFixture(t)
				entry := f.entries["one"]
				entry.Status, entry.Unavailable, entry.NextRetryAfter = "error", true, time.Now().Add(time.Hour)
				entry.StatusMessage = "old upstream HTTP 401"
				f.entries["one"] = entry
				if mode == "actual 401" {
					f.upstreamStatus = 401
				}
				if mode == "unknown catalog" {
					base := f.host.Call
					f.host.Call = func(method string, in, out any) error {
						if method == "host.auth.models" {
							return errors.New("catalog unavailable")
						}
						return base(method, in, out)
					}
				}
				r := f.rule(t, disableCredential, scope != "single", true)
				r.TargetScope, r.Samples = scope, 2
				r.UpstreamErrorActions = map[string]string{"unauthorized": disableCredential}
				r, err := f.engine.SaveRule(context.Background(), *r, true)
				if err != nil {
					t.Fatal(err)
				}
				runs, err := f.round(t, r)
				if err != nil || len(runs) != 1 || runs[0].PreflightDetail != nil || runs[0].SkipDetail != nil || runs[0].Status != "completed" {
					t.Fatal(runs, err)
				}
				if mode == "actual 401" {
					if runs[0].UpstreamCondition != "unauthorized" || runs[0].ActionStatus != "credential_suspended" || len(f.patches) != 1 || f.probes != 1 {
						t.Fatal("verified response did not trigger exactly one action", runs[0], f.patches, f.probes)
					}
					assertErrorPolicySkip(t, f, r, "")
				} else if runs[0].ActionStatus != "observe_only" || runs[0].UpstreamCondition != "" || len(runs[0].Samples) != 4 || f.probes != 4 || len(f.patches) != 0 {
					t.Fatal("stale runtime messages or missing catalog produced an action", runs[0], f.probes, f.patches)
				}
			})
		}
	}
}

func TestProfileWaitingForSlotCannotReadTerminalCredential(t *testing.T) {
	f := managementTestFixture(t)
	r := f.rule(t, disableCredential, true, true)
	targets, err := f.host.Targets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	f.engine.snapshotSlots = make(chan struct{}, 1)
	f.engine.snapshotSlots <- struct{}{}
	var reads atomic.Int64
	original := f.host.Call
	f.host.Call = func(method string, in, out any) error {
		if method == "host.auth.get" || method == "host.auth.get_runtime" ||
			method == "host.auth.models" && in.(pluginapi.HostAuthModelsRequest).AuthIndex != "" {
			reads.Add(1)
		}
		return original(method, in, out)
	}
	done := make(chan []scopeProfile, 1)
	go func() {
		done <- f.engine.readScopeProfiles(context.Background(), r, targets, map[string]*CredentialDisposition{})
	}()
	f.engine.mu.Lock()
	d := &CredentialDisposition{AuthID: "one", RuleID: r.ID, State: "disabled", UpstreamCondition: "payment_required"}
	c := changes()
	c.disposition = d
	err = f.engine.commitLocked(c)
	f.engine.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	<-f.engine.snapshotSlots
	result := <-done
	if reads.Load() != 0 || len(result) != 1 || result[0].err == nil {
		t.Fatal("waiting profile fetched a retired credential", result, reads.Load())
	}
}
