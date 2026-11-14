package main

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginapi"
)

func TestHostErrorRetainsStructuredStatus(t *testing.T) {
	err := decodeHostResponse([]byte(`{"ok":false,"error":{"code":"host_call_failed","message":"denied","http_status":401}}`), 0, nil)
	var detail *pluginabi.Error
	if !errors.As(err, &detail) || detail.StatusCode() != 401 || detail.ErrorCode() != "host_call_failed" {
		t.Fatal("structured host error lost", err)
	}
}

func TestPublicResourcesStayStaticAndMatchExactRegisteredPaths(t *testing.T) {
	for _, resource := range staticResources {
		raw, err := json.Marshal(pluginapi.ManagementRequest{Method: "GET", Path: "/v0/resource/plugins/model-quality/" + resource.name})
		if err != nil {
			t.Fatal(err)
		}
		result, err := handle(pluginabi.MethodManagementHandle, raw)
		if err != nil {
			t.Fatal(err)
		}
		response := result.(pluginapi.ManagementResponse)
		if response.StatusCode != 200 || len(response.Body) == 0 || response.Headers.Get("Content-Type") != resource.contentType || response.Headers.Get("Content-Security-Policy") == "" {
			t.Fatal("static resource contract", resource.name, response.StatusCode)
		}
	}
	for _, req := range []pluginapi.ManagementRequest{
		{Method: "POST", Path: "/v0/resource/plugins/model-quality/index.html"},
		{Method: "GET", Path: "/v0/resource/plugins/model-quality/nested/app.js"},
		{Method: "GET", Path: "/v0/resource/plugins/model-quality/../app.js"},
	} {
		raw, _ := json.Marshal(req)
		result, err := handle(pluginabi.MethodManagementHandle, raw)
		if err != nil || result.(pluginapi.ManagementResponse).StatusCode != 404 {
			t.Fatal("accepted unregistered public resource", req, err)
		}
	}
}

func TestHostEnvelopeRequiresConsistentSuccessAndResult(t *testing.T) {
	for _, tc := range []struct {
		raw    string
		status int
	}{
		{`{"ok":true,"result":null}`, 0},
		{`{"ok":true}`, 0},
		{`{"ok":true,"result":{},"error":{"code":"error","message":"failed"}}`, 0},
		{`{"ok":true,"result":{}}`, 1},
		{`{"ok":false,"result":{}}`, 0},
		{`{"ok":true,"result":"unexpected string"}`, 0},
		{`null`, 0},
		{`malformed`, 0},
	} {
		var result struct{ Value string }
		if err := decodeHostResponse([]byte(tc.raw), tc.status, &result); err == nil {
			t.Fatal("accepted invalid host response", tc.raw, tc.status)
		}
	}
	var result struct{ Value string }
	if err := decodeHostResponse([]byte(`{"ok":true,"result":{"Value":"ok"}}`), 0, &result); err != nil || result.Value != "ok" {
		t.Fatal(result, err)
	}
}
