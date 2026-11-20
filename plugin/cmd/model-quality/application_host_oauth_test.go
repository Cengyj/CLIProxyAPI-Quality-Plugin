//go:build credential_ops_host_oauth

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

func TestHostOAuthAccountCapabilities(t *testing.T) {
	routes := accountManagementRoutes()
	if len(routes) != 1 || routes[0].Method != http.MethodGet || routes[0].Path != accountCapabilitiesPath {
		t.Fatalf("unexpected account API for host OAuth: %v", routes)
	}
	result := (&hostOAuthApplication{}).Handle(context.Background(), pluginapi.ManagementRequest{Method: http.MethodGet, Path: accountCapabilitiesPath})
	var caps struct {
		Mode string `json:"login_mode"`
	}
	if result.StatusCode != http.StatusOK || json.Unmarshal(result.Body, &caps) != nil || caps.Mode != "host_oauth" {
		t.Fatalf("unexpected host OAuth capabilities: %v", result)
	}
}
