#!/bin/sh
set -eu
TASK_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TASK_GO=${GO_BIN:-go}
mkdir -p "$TASK_ROOT/artifacts/official-plugin/verification"
(cd "$TASK_ROOT/plugin" && "$TASK_GO" test ./...) > "$TASK_ROOT/artifacts/official-plugin/verification/plugin-tests.log" 2>&1
(cd "$TASK_ROOT/plugin" && "$TASK_GO" test -race ./...) > "$TASK_ROOT/artifacts/official-plugin/verification/plugin-race.log" 2>&1
(cd "$TASK_ROOT/login-runtime-go" && "$TASK_GO" test ./...) > "$TASK_ROOT/artifacts/official-plugin/verification/runtime-tests.log" 2>&1
(cd "$TASK_ROOT/plugin" && "$TASK_GO" test -tags loginbundle_embedded ./...) > "$TASK_ROOT/artifacts/official-plugin/verification/embedded-bundle.log" 2>&1
python3 "$TASK_ROOT/scripts/verify_baseline.py" --source "$TASK_ROOT/workspaces/CLIProxyAPI-official"
(cd "$TASK_ROOT/workspaces/CLIProxyAPI-official" && "$TASK_GO" build -o "$TASK_ROOT/artifacts/official-plugin/cli-proxy-api-official" ./cmd/server)
TASK_ARCHIVE=$(python3 - "$TASK_ROOT/artifacts/official-plugin/build-manifest.json" <<'PY'
import json, sys
print(json.load(open(sys.argv[1]))['archive'])
PY
)
python3 "$TASK_ROOT/scripts/verify_official_plugin.py" --archive "$TASK_ARCHIVE"
