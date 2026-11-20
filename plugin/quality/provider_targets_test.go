package quality

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

func TestConfiguredRouteIdentity(t *testing.T) {
	for _, tc := range []struct{ id, executor, family string }{
		{"codex:apikey:abcdef012345", "codex", "codex"},
		{"vertex:apikey:abcdef012345-1", "vertex", "vertex"},
		{"gemini-interactions:apikey:abcdef012345", "gemini-interactions", "interactions"},
		{"openai-compatibility:a:b:abcdef012345", "openai-compatible-a:b", "openai-compatibility"},
		{"openai-compatibility:openai-compatible-a:abcdef012345", "openai-compatible-a", "openai-compatibility"},
		{"openai-compatibility:openai-compatibility:abcdef012345", "openai-compatibility", "openai-compatibility"},
		{"openai-compatibility:a:abcdef012345", "openai-compatible-b", ""},
		{"codex:apikey:abcdef012345.json", "codex", ""},
		{"account.json", "codex", ""},
	} {
		if got := providerFamily(tc.id, tc.executor); got != tc.family {
			t.Errorf("%q, %q: got %q, want %q", tc.id, tc.executor, got, tc.family)
		}
	}
}

func TestConfiguredRoutesRequireMatchingPublicInventory(t *testing.T) {
	id, _ := (providerRoute{family: "codex", executor: "codex", key: providerObject{"api-key": "fixture-only-hidden-value"}}).identity()
	compat, _ := (providerRoute{family: "openai-compatibility", executor: "openai-compatible-other", name: "Other"}).identity()
	var config atomic.Value
	config.Store(`{"codex":[{"name":"lab","keys":[{"auth_index":"p","api-key":"fixture-only-hidden-value"}]}],"openai-compatibility":[{"name":"Other","auth_index":"c"}]}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v8/management/config/api-keys" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(config.Load().(string)))
	}))
	defer server.Close()
	m, err := NewManagementClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	m.authorize(http.Header{"Authorization": {"Bearer offline"}})
	h := &NativeHost{Management: m}
	rows := []pluginapi.HostAuthModels{
		{ID: "file.json", AuthIndex: "f", Provider: "codex", Models: []string{"shared"}},
		{ID: id, AuthIndex: "p", Provider: "codex", Models: []string{"shared"}},
		{ID: compat, AuthIndex: "c", Provider: "openai-compatible-other", Models: []string{"shared"}},
	}
	h.Call = func(method string, request, response any) error {
		switch method {
		case "host.auth.list":
			return decode([]byte(`{"files":[{"id":"file.json","name":"file.json","auth_index":"f","provider":"codex","status":"active","source":"file"}]}`), response)
		case "host.auth.models":
			index := request.(pluginapi.HostAuthModelsRequest).AuthIndex
			for _, row := range rows {
				if index == "" || row.AuthIndex == index {
					out := response.(*pluginapi.HostAuthModelsResponse)
					out.Credentials = append(out.Credentials, row)
				}
			}
		case "host.auth.get_runtime":
			return errors.New("no file runtime")
		default:
			t.Fatalf("unexpected callback %s", method)
		}
		return nil
	}
	ctx := context.Background()
	targets, err := h.Targets(ctx)
	if err != nil || len(targets) != 3 || targets[0].IsProvider || !targets[0].EffectiveAvailable ||
		!targets[1].IsProvider || !targets[1].EffectiveAvailable || targets[1].RuntimeStatus != "registered" ||
		targets[1].ProviderName != "lab" || targets[2].ProviderName != "Other" || !targets[2].EffectiveAvailable {
		t.Fatalf("unexpected joined projections: %+v, %v", targets, err)
	}
	packet, err := json.Marshal(targets)
	if err != nil || strings.Contains(string(packet), "fixture-only-hidden-value") {
		t.Fatal("public target exposed a private config field")
	}
	for _, changes := range []string{
		`{}`, `{"claude":[{"keys":[{"auth_index":"p"}]}]}`,
		`{"codex":[{"disabled":true,"keys":[{"auth_index":"p"}]}]}`,
		`{"codex":[{"keys":[{"auth_index":"p"},{"auth_index":"p"}]}]}`,
	} {
		config.Store(changes)
		current, err := h.pinnedTarget(ctx, id, "p")
		if err != nil || !current.IsProvider || current.EffectiveAvailable {
			t.Fatalf("stale or ambiguous route was admitted: %+v, %v", current, err)
		}
	}
}
