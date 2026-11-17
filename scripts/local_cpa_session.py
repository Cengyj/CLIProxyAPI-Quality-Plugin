#!/usr/bin/env python3
"""Manage this workspace's local CPA when LaunchAgent access is pending."""
from pathlib import Path
import json
import os
import plistlib
import signal
import subprocess
import sys
import threading
import time
import urllib.request
import urllib.error

SITE = Path(__file__).resolve().parents[1] / 'artifacts/local-cpa'
PID_FILE = SITE / 'session-process.json'
BINARY = str(SITE / 'bin/cli-proxy-api')


def process(strict=True):
    if not PID_FILE.exists():
        return None
    record = json.loads(PID_FILE.read_text())
    pid = record.get('pid', 0)
    if not isinstance(pid, int) or pid <= 1:
        raise RuntimeError('Invalid CPA process record')
    result = subprocess.run(['ps', '-p', str(pid), '-o', 'stat=', '-o', 'command='], capture_output=True, text=True)
    if result.returncode != 0:
        return None
    state, command = result.stdout.strip().split(None, 1)
    if state.startswith('Z'):
        return None
    if not command.startswith(BINARY + ' --config '):
        if not strict:
            return None
        raise RuntimeError('CPA PID no longer identifies this installation; refusing to signal it')
    return pid


def ready():
    access = json.loads((SITE / 'access-info.json').read_text())
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    for prefix in ['/v0/management/plugins/model-quality', '/v8/management/plugins/model-quality']:
        request = urllib.request.Request('http://127.0.0.1:8317' + prefix + '/targets',
            headers={'Authorization': 'Bearer ' + access['management_key']})
        try:
            with client.open(request, timeout=2) as response:
                json.load(response)
            return
        except urllib.error.HTTPError as error:
            if error.code != 404 or prefix.startswith('/v8/'):
                raise


def main():
    command = sys.argv[1] if len(sys.argv) > 1 else 'status'
    pid = process()
    if command == 'stop':
        if pid:
            os.kill(pid, signal.SIGTERM)
            deadline = time.monotonic() + 20
            waiter = threading.Event()
            while process(strict=False) and time.monotonic() < deadline:
                waiter.wait(.2)
            if process(strict=False):
                raise RuntimeError('CPA did not exit gracefully; refusing to replace running binaries')
        PID_FILE.unlink(missing_ok=True)
        print('CPA stopped; configuration, credentials and plugin state are preserved.')
    elif command == 'start':
        if not pid:
            plist = plistlib.loads((SITE / 'local.cpa.model-quality.plist').read_bytes())
            with (SITE / 'logs/stdout.log').open('ab') as stdout, (SITE / 'logs/stderr.log').open('ab') as stderr:
                child = subprocess.Popen(plist['ProgramArguments'], cwd=SITE, stdin=subprocess.DEVNULL,
                    stdout=stdout, stderr=stderr, start_new_session=True, close_fds=True)
            PID_FILE.write_text(json.dumps({'pid': child.pid, 'executable': BINARY, 'mode': 'session'}, indent=2) + '\n')
            PID_FILE.chmod(0o600)
        deadline = time.monotonic() + 45
        waiter = threading.Event()
        while time.monotonic() < deadline:
            try:
                ready()
                print('CPA and model-quality are ready: http://127.0.0.1:8317/management.html (session process)')
                return
            except (OSError, ValueError):
                waiter.wait(.25)
        raise RuntimeError('CPA did not become ready; inspect local logs')
    elif command == 'status':
        if not pid:
            raise RuntimeError('CPA session process is not running')
        ready()
        print('CPA is reachable: http://127.0.0.1:8317/management.html; session PID', pid)
    else:
        raise RuntimeError('Expected start, stop or status')


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
