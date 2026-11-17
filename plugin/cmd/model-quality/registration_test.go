package main

import (
	"strings"
	"testing"

	"github.com/cliproxyapi-plugins/model-quality/quality"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginabi"
)

func TestOfficialManagementRoutesUseAuthenticatedPluginNamespace(t *testing.T) {
	reply, err := handle(pluginabi.MethodManagementRegister, nil)
	if err != nil {
		t.Fatal(err)
	}
	registration := reply.(map[string]any)
	if _, exists := registration["routes_v8"]; exists {
		t.Fatal("requires patched host route registration")
	}
	seen := map[string]bool{}
	for _, route := range registration["routes"].([]quality.ManagementRoute) {
		if !strings.HasPrefix(route.Path, "/v0/management/plugins/model-quality/") {
			t.Fatalf("route outside plugin namespace: %s", route.Path)
		}
		key := route.Method + " " + route.Path
		if seen[key] {
			t.Fatalf("duplicate route: %s", key)
		}
		seen[key] = true
	}
	for _, required := range []string{"GET /targets", "POST /runs", "GET /login/capabilities", "POST /login/accounts/import", "POST /login/commit"} {
		method, path, _ := strings.Cut(required, " ")
		if !seen[method+" /v0/management/plugins/model-quality"+path] {
			t.Fatal("missing route:", required)
		}
	}
}
