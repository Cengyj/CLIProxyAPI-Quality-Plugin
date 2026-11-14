#!/bin/sh
set -eu
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
SOURCE=${1:?Usage: apply_host_patch.sh /path/to/clean/8.0.67/source-copy}
TASK_SOURCE=$(CDPATH= cd -- "$SOURCE" && pwd -P)
TASK_CEILING=$(CDPATH= cd -- "$TASK_SOURCE/.." && pwd -P)
python3 "$ROOT/scripts/verify_baseline.py" --source "$TASK_SOURCE"
# A plain source copy must not inherit an enclosing workspace's Git repository.
GIT_CEILING_DIRECTORIES="$TASK_CEILING" git -C "$TASK_SOURCE" apply --check "$ROOT/host-patches/model-quality.patch"
GIT_CEILING_DIRECTORIES="$TASK_CEILING" git -C "$TASK_SOURCE" apply "$ROOT/host-patches/model-quality.patch"
echo 'Patch applied to verified source copy. Build and test before deployment.'
