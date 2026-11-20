package quality

import (
	"context"
	"errors"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

// The native model callback publishes the same exact credential through its
// directory scan and through the pinned lookup used before a write.
func catalogFixture(t *testing.T, action string, dynamic bool) (*managementFixture, *Rule, *string) {
	t.Helper()
	f := managementTestFixture(t)
	mode := "missing"
	base := f.host.Call
	f.host.Call = func(method string, in, out any) error {
		if err := base(method, in, out); err != nil || method != "host.auth.models" {
			return err
		}
		entries := out.(*pluginapi.HostAuthModelsResponse)
		for i := range entries.Credentials {
			switch mode {
			case "missing":
				entries.Credentials[i].Models = []string{"a"}
			case "empty":
				entries.Credentials[i].Models = []string{}
			case "wrong_identity":
				entries.Credentials[i].ID = "someone-else"
			case "unavailable":
				return errors.New("offline catalog unavailable")
			}
		}
		if mode == "duplicate" && len(entries.Credentials) > 0 {
			entries.Credentials = append(entries.Credentials, entries.Credentials[0])
		}
		return nil
	}
	input := Rule{Name: "catalog fixture", RunConfig: lifeConfig(), Action: "observe_only", Enabled: true,
		UpstreamErrorActions: map[string]string{"model_unsupported": action}}
	input.Models = []string{"a", "b"}
	input.AuthID, input.Provider = "one", "codex"
	if dynamic {
		input.TargetScope, input.AuthID, input.Provider = scopeAll, "", ""
	}
	r, err := f.engine.SaveRule(context.Background(), input, false)
	if err != nil {
		t.Fatal(err)
	}
	return f, r, &mode
}

func TestCatalogModelMissingUsesOneConfirmedWrite(t *testing.T) {
	for _, dynamic := range []bool{true, false} {
		for _, action := range []string{disableCredential, deleteCredential} {
			t.Run(map[bool]string{true: "dynamic/", false: "single/"}[dynamic]+action, func(t *testing.T) {
				ctx := context.Background()
				f, rule, _ := catalogFixture(t, action, dynamic)
				receipt, err := f.engine.dispatchRule(ctx, rule.ID, rule.Revision, false)
				if err != nil || receipt.Count != 1 || receipt.Skipped != 0 || len(receipt.SkipDetails) != 0 ||
					len(f.patches) != 0 || f.deletes != 0 || f.probes != 0 {
					t.Fatal("dispatch changed the credential or failed to admit the catalog check", receipt, err)
				}
				task := f.engine.nextRunnableLocked()
				if task == nil || receipt.RunID != "" && task.id != receipt.RunID {
					t.Fatal("catalog evidence and write must share a run", receipt)
				}
				f.engine.executeTask(task)
				run, err := f.engine.Run(task.id)
				if err != nil || run.Status != "completed" || len(run.Samples) != 0 || run.UpstreamCondition != "" ||
					run.Config.CatalogMissingModel != "b" || run.PreflightDetail == nil || run.PreflightDetail.Model != "b" || run.SkipDetail != nil || run.CredentialAction != action ||
					f.probes != 0 || f.nativeCalls != 0 {
					t.Fatal("catalog action produced model evidence or lost its source", run, err)
				}
				if action == disableCredential && (run.ActionStatus != "credential_suspended" || len(f.patches) != 1 || !f.entries["one"].Disabled || f.deletes != 0) ||
					action == deleteCredential && (run.ActionStatus != "credential_removed" || f.deletes != 1 || len(f.entries) != 0 || len(f.patches) != 0) {
					t.Fatal(run.ActionStatus, f.patches, f.deletes, len(f.entries))
				}
				handling := f.engine.queryRules(nil)["items"].([]ruleView)[0].Handling
				if len(handling) != 1 || handling[0].CatalogMissingModel != "b" || handling[0].UpstreamCondition != "" {
					t.Fatal("rule card lost the catalog source", handling)
				}
				if _, err := f.engine.dispatchRule(ctx, rule.ID, rule.Revision, false); err != nil {
					t.Fatal(err)
				}
				if task := f.engine.nextRunnableLocked(); task != nil {
					t.Fatal("terminal catalog action was dispatched twice")
				}
			})
		}
	}
}

func TestCatalogActionRechecksBeforeWrite(t *testing.T) {
	for _, cause := range []string{"registered", "wrong_identity", "unavailable", "duplicate", "replaced", "disabled", "pause", "uncertain", "session", "cancelled", "model_policy", "rule_stopped"} {
		t.Run(cause, func(t *testing.T) {
			ctx := context.Background()
			f, r, mode := catalogFixture(t, deleteCredential, true)
			receipt, err := f.engine.dispatchRule(ctx, r.ID, r.Revision, false)
			if err != nil || receipt.Count != 1 || receipt.Skipped != 0 {
				t.Fatal(receipt, err)
			}
			task := f.engine.nextRunnableLocked()
			if task == nil {
				t.Fatal("missing catalog worker")
			}
			switch cause {
			case "replaced":
				f.changedContent = true
			case "disabled":
				a := f.entries["one"]
				a.Status, a.Disabled = "disabled", true
				f.entries["one"] = a
			case "pause":
				f.engine.rules["retained"] = &Rule{ID: "retained", RunConfig: RunConfig{AuthID: "one"}, ActionID: "held"}
			case "uncertain":
				f.engine.managed["one"] = &CredentialDisposition{AuthID: "one", State: "attention", RuleID: r.ID}
			case "session":
				f.host.Management.key = ""
			case "cancelled":
				if err := f.engine.Cancel(task.id); err != nil {
					t.Fatal(err)
				}
			case "model_policy":
				current := cloneRule(f.engine.rules[r.ID])
				current.UpstreamErrorActions = map[string]string{"model_unsupported": disableCredential}
				if _, err := f.engine.SaveRule(ctx, *current, true); err != nil {
					t.Fatal(err)
				}
			case "rule_stopped":
				if err := f.engine.PauseRule(r.ID, r.Revision, true); err != nil {
					t.Fatal(err)
				}
			default:
				*mode = cause
			}
			f.engine.executeTask(task)
			run, err := f.engine.Run(task.id)
			if err != nil || run.CredentialAction != "" || len(run.Samples) != 0 || f.probes != 0 || f.nativeCalls != 0 ||
				f.deletes != 0 || len(f.patches) != 0 || len(f.entries) != 1 {
				t.Fatal("unverified catalog action wrote or called a model", cause, run, err)
			}
		})
	}
}

func TestCatalogActionUncertainWriteSurvivesRestart(t *testing.T) {
	ctx := context.Background()
	f, rule, _ := catalogFixture(t, disableCredential, true)
	f.statusFailure = true // CPA applies the write but returns an unconfirmed result.
	receipt, err := f.engine.dispatchRule(ctx, rule.ID, rule.Revision, false)
	if err != nil || receipt.Count != 1 || receipt.Skipped != 0 {
		t.Fatal(receipt, err)
	}
	task := f.engine.nextRunnableLocked()
	if task == nil {
		t.Fatal("missing worker")
	}
	f.engine.executeTask(task)
	if r := f.engine.runs[task.id]; r.ActionStatus != "credential_action_uncertain" || len(f.patches) != 1 || !f.entries["one"].Disabled {
		t.Fatal(r.ActionStatus, f.patches)
	}
	f.statusFailure = false
	cfg := f.engine.cfg
	if err := f.engine.Close(); err != nil {
		t.Fatal(err)
	}
	f.engine, err = New(cfg, f.host)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.engine.Close() })
	recovered, err := f.engine.Run(task.id)
	if err != nil || recovered.ActionStatus != "credential_action_uncertain" || recovered.Config.CatalogMissingModel != "b" ||
		len(recovered.Samples) != 0 || recovered.PreflightDetail == nil || recovered.PreflightDetail.Code != "missing_model" || recovered.SkipDetail != nil {
		t.Fatal(recovered, err)
	}
	for range 2 {
		next, err := f.engine.dispatchRule(ctx, rule.ID, rule.Revision, false)
		if err != nil || next.Count != 1 || next.Skipped != 0 {
			t.Fatal(next, err)
		}
	}
	if len(f.patches) != 1 || f.deletes != 0 || f.probes != 0 || f.nativeCalls != 0 || f.engine.managed["one"].State != "attention" {
		t.Fatal("restart or redispatch replayed the uncertain catalog action")
	}
}
