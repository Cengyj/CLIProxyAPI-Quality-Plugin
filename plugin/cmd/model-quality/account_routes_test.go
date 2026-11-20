//go:build !credential_ops_host_oauth

package main

import "testing"

func TestBundledAccountRoutes(t *testing.T) {
	seen := make(map[string]bool)
	for _, route := range accountManagementRoutes() {
		seen[route.Method+" "+route.Path] = true
	}
	for _, path := range []string{"/login/accounts/import", "/login/commit"} {
		if !seen["POST /v0/management/plugins/model-quality"+path] {
			t.Errorf("missing account route %s", path)
		}
	}
}
