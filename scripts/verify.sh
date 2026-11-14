#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
case $(uname -s) in Darwin) EXT=dylib;; *) EXT=so;; esac
mkdir -p "$ROOT/artifacts/verification"
(cd "$ROOT/plugin" && go test ./...) > "$ROOT/artifacts/verification/plugin-tests.log" 2>&1
(cd "$ROOT/plugin" && go test -race ./...) > "$ROOT/artifacts/verification/plugin-race.log" 2>&1
(cd "$ROOT/workspaces/CLIProxyAPI" && go test ./...) > "$ROOT/artifacts/verification/host-go-test.log" 2>&1
(cd "$ROOT/workspaces/CLIProxyAPI" && go test -race ./sdk/cliproxy/auth ./internal/pluginhost ./sdk/cliproxy/usage) > "$ROOT/artifacts/verification/host-race.log" 2>&1
(cd "$ROOT/workspaces/CLIProxyAPI" && MODEL_QUALITY_LIBRARY="$ROOT/artifacts/model-quality.$EXT" go test -count=1 -run TestModelQualityNativeLifecycle -v ./internal/pluginhost) > "$ROOT/artifacts/verification/native-e2e.log" 2>&1
python3 "$ROOT/scripts/verify_baseline.py" --reference
