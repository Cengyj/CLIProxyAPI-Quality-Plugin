#!/bin/sh
# Manage the persistent, loopback-only CPA installation for this workspace.
set -eu
TASK_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
TASK_DEPLOY="$TASK_ROOT/artifacts/local-cpa"
TASK_DOMAIN="gui/$(id -u)"
TASK_SERVICE="$TASK_DOMAIN/local.cpa.model-quality"
TASK_PLIST="$TASK_DEPLOY/local.cpa.model-quality.plist"

case "${1:-status}" in
  start)
    test -f "$TASK_PLIST"
    test -f "$TASK_DEPLOY/static/management.html"
    if ! /bin/launchctl print "$TASK_SERVICE" >/dev/null 2>&1; then
      /bin/launchctl bootstrap "$TASK_DOMAIN" "$TASK_PLIST"
    fi
    /bin/launchctl kickstart "$TASK_SERVICE"
    /usr/bin/python3 - "$TASK_DEPLOY" <<'PY'
import json
import sys
import threading
import time
from pathlib import Path
from urllib.error import URLError
from urllib.request import ProxyHandler, Request, build_opener

directory = Path(sys.argv[1])
access = json.loads((directory / 'access-info.json').read_text())
client = build_opener(ProxyHandler({}))
waiter = threading.Event()
deadline = time.monotonic() + 45
while time.monotonic() < deadline:
    try:
        request = Request(
            'http://127.0.0.1:8317/v8/management/plugins/model-quality/targets',
            headers={'Authorization': 'Bearer ' + access['management_key']},
        )
        with client.open(request, timeout=2) as response:
            json.load(response)
        print('CPA and model-quality are ready: http://127.0.0.1:8317/management.html')
        break
    except (OSError, URLError, ValueError):
        waiter.wait(0.25)
else:
    raise SystemExit('CPA did not become ready; inspect artifacts/local-cpa/logs and run status.')
PY
    ;;
  stop)
    if /bin/launchctl print "$TASK_SERVICE" >/dev/null 2>&1; then
      /bin/launchctl bootout "$TASK_SERVICE"
    fi
    printf '%s\n' 'CPA stopped; configuration, credentials and plugin state are preserved.'
    ;;
  restart)
    "$0" stop
    "$0" start
    ;;
  status)
    if /bin/launchctl print "$TASK_SERVICE" >/dev/null 2>&1; then
      /bin/launchctl print "$TASK_SERVICE" | /usr/bin/sed -n '/state = /p; /pid = /p; /last exit code = /p'
      /usr/bin/curl --noproxy '*' --fail --silent --output /dev/null --max-time 5 http://127.0.0.1:8317/management.html
      printf '%s\n' 'CPA management page is reachable: http://127.0.0.1:8317/management.html'
    else
      printf '%s\n' 'CPA service is not loaded.'
      exit 1
    fi
    ;;
  *)
    printf '%s\n' 'Usage: scripts/local_cpa.sh start|stop|restart|status' >&2
    exit 2
    ;;
esac
