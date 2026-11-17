package quality

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

func (en *Engine) Handle(ctx context.Context, req pluginapi.ManagementRequest) pluginapi.ManagementResponse {
	path := strings.TrimPrefix(req.Path, "/v0/management/plugins/model-quality")
	var result any
	var err error
	status := 200
	switch req.Method + " " + path {
	case "GET /targets":
		var items []Target
		items, err = en.host.Targets(ctx)
		result = page(items, req)
		result.(map[string]any)["capabilities"] = en.Capabilities()
	case "GET /questions":
		items := Questions()
		for i := range items {
			items[i].Answer = ""
		}
		result = page(items, req)
	case "GET /runs":
		result, err = en.QueryRuns(req.Query)
	case "GET /runs/detail":
		result, err = en.Run(req.Query.Get("id"))
	case "POST /runs":
		var c RunConfig
		if err = DecodeStrict(req.Body, &c); err == nil {
			var run *Run
			run, err = en.Submit(ctx, c)
			if run != nil {
				result = map[string]string{"run_id": run.ID}
				status = 202
			}
		}
	case "POST /runs/cancel":
		var body struct {
			ID string `json:"id"`
		}
		if err = DecodeStrict(req.Body, &body); err == nil {
			err = en.Cancel(body.ID)
		}
	case "POST /runs/review":
		var body struct {
			ID          string `json:"id"`
			SampleIndex int    `json:"sample_index"`
			Verdict     string `json:"verdict"`
			Note        string `json:"note"`
		}
		if err = DecodeStrict(req.Body, &body); err == nil {
			err = en.Review(body.ID, body.SampleIndex, body.Verdict, body.Note)
		}
	case "GET /rules":
		result = page(en.Rules(), req)
	case "POST /rules", "PUT /rules":
		var body Rule
		if err = DecodeStrict(req.Body, &body); err == nil {
			result, err = en.SaveRule(ctx, body, req.Method == "PUT")
		}
	case "POST /rules/run":
		var body struct {
			ID string `json:"id"`
		}
		if err = DecodeStrict(req.Body, &body); err == nil {
			var run *Run
			run, err = en.RunRule(ctx, body.ID)
			if run != nil {
				result = map[string]string{"run_id": run.ID}
				status = 202
			}
		}
	case "POST /rules/pause":
		var body struct {
			ID       string `json:"id"`
			Revision uint64 `json:"revision"`
			Paused   bool   `json:"paused"`
		}
		if err = DecodeStrict(req.Body, &body); err == nil {
			err = en.PauseRule(body.ID, body.Revision, body.Paused)
		}
	case "DELETE /rules":
		var body struct {
			ID       string `json:"id"`
			Revision uint64 `json:"revision"`
		}
		if err = DecodeStrict(req.Body, &body); err == nil {
			err = en.DeleteRule(ctx, body.ID, body.Revision)
		}
	case "GET /actions":
		var items []Action
		items, err = en.Actions(ctx)
		result = page(items, req)
	case "POST /actions/preview", "POST /actions/release":
		var body struct {
			RuleID         string `json:"rule_id"`
			ActionID       string `json:"action_id"`
			ActionRevision uint64 `json:"action_revision"`
			PauseRule      bool   `json:"pause_rule"`
		}
		if err = DecodeStrict(req.Body, &body); err == nil {
			if path == "/actions/preview" {
				result, err = en.Preview(ctx, body.RuleID, body.ActionID, body.ActionRevision)
			} else {
				err = en.Release(ctx, body.RuleID, body.ActionID, body.ActionRevision, body.PauseRule)
			}
		}
	case "GET /history":
		var entries []Audit
		entries, err = en.History()
		result = page(entries, req)
	case "GET /metrics":
		result, err = en.Metrics()
	case "GET /presets":
		var items []Preset
		items, err = en.Presets()
		result = page(items, req)
	case "POST /presets":
		var body struct {
			Name   string    `json:"name"`
			Config RunConfig `json:"config"`
		}
		if err = DecodeStrict(req.Body, &body); err == nil {
			result, err = en.SavePreset(body.Name, body.Config)
		}
	case "DELETE /presets":
		var body struct {
			ID string `json:"id"`
		}
		if err = DecodeStrict(req.Body, &body); err == nil {
			err = en.DeletePreset(body.ID)
		}
	case "GET /settings":
		result = en.settingsResponse()
	case "PUT /settings":
		var c Config
		if err = DecodeStrict(req.Body, &c); err == nil {
			err = en.UpdateConfig(c)
			result = en.settingsResponse()
		}
	default:
		err = fmt.Errorf("not_found: route")
	}
	if err != nil {
		code, mappedStatus := classifyError(err)
		status = mappedStatus
		msg := scrubError(err)
		result = map[string]any{"error": map[string]string{"code": code, "message": msg}}
	} else if result == nil {
		result = map[string]bool{"ok": true}
	}
	body, e := json.Marshal(result)
	if e != nil {
		status = 500
		body = []byte(`{"error":{"code":"serialization_error"}}`)
	}
	return pluginapi.ManagementResponse{StatusCode: status, Headers: http.Header{"Content-Type": {"application/json; charset=utf-8"}, "Cache-Control": {"no-store"}, "X-Content-Type-Options": {"nosniff"}}, Body: body}
}

func (en *Engine) settingsResponse() any {
	return struct {
		Config
		Capabilities HostCapabilities `json:"capabilities"`
	}{en.Config(), en.Capabilities()}
}
