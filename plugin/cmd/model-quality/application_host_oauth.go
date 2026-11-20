//go:build credential_ops_host_oauth

package main

import (
	"context"
	"net/http"
	"path/filepath"

	"github.com/cliproxyapi-plugins/model-quality/quality"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

const accountCapabilitiesPath = "/v0/management/plugins/model-quality/login/capabilities"

type hostOAuthApplication struct {
	*quality.Engine
}

func accountManagementRoutes() []quality.ManagementRoute {
	return []quality.ManagementRoute{{Method: http.MethodGet, Path: accountCapabilitiesPath}}
}

func newApplication(cfg quality.Config, native *quality.NativeHost) (runtimeEngine, error) {
	directory, err := filepath.Abs(cfg.DataDir)
	if err != nil {
		return nil, err
	}
	cfg.DataDir = directory
	engine, err := quality.New(cfg, native)
	if err != nil {
		return nil, err
	}
	return &hostOAuthApplication{Engine: engine}, nil
}

func (app *hostOAuthApplication) Handle(ctx context.Context, req pluginapi.ManagementRequest) pluginapi.ManagementResponse {
	if req.Method == http.MethodGet && req.Path == accountCapabilitiesPath {
		return pluginapi.ManagementResponse{
			StatusCode: http.StatusOK,
			Headers:    http.Header{"Content-Type": {"application/json"}, "Cache-Control": {"no-store"}},
			Body:       []byte(`{"login_mode":"host_oauth"}`),
		}
	}
	return app.Engine.Handle(ctx, req)
}
