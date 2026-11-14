#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
case $(uname -s) in Darwin) EXT=dylib;; Linux|FreeBSD) EXT=so;; *) echo 'Platform not verified; build explicitly with the matching Go C toolchain.' >&2; exit 1;; esac
mkdir -p "$ROOT/artifacts"
(cd "$ROOT/plugin" && gofmt -w . && go test ./... && go build -buildmode=c-shared -o "$ROOT/artifacts/model-quality.$EXT" ./cmd/model-quality)
(cd "$ROOT/workspaces/CLIProxyAPI" && gofmt -w . && go build -o test-output ./cmd/server && rm test-output && go build -o "$ROOT/artifacts/cli-proxy-api-quality" ./cmd/server)
"$ROOT/scripts/build_ui.sh"
