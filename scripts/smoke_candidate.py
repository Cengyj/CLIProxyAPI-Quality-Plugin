#!/usr/bin/env python3
"""Unpack a candidate and exercise its actual host/library in an isolated fake installation."""
import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tarfile
import tempfile
import threading
import time
from urllib.error import HTTPError
from urllib.request import Request, urlopen

from release_support import UI_NAMES, digest


def request(url, body=None, key=None, method=None, raw=False, expected=200):
    headers = {'Content-Type': 'application/json'}
    if key:
        headers['Authorization'] = 'Bearer ' + key
    req = Request(url, json.dumps(body).encode() if body is not None else None, headers, method=method)
    try:
        response = urlopen(req, timeout=5)
    except HTTPError as error:
        if error.code != expected:
            raise
        response = error
    with response:
        assert response.status == expected, (response.status, expected)
        data = response.read()
        return data if raw else json.loads(data)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('archive', type=Path)
    parser.add_argument('--report', type=Path, required=True)
    args = parser.parse_args()
    archive = args.archive.resolve()
    report_path = args.report.resolve()
    if report_path.exists():
        parser.error('smoke report already exists; preserve it and choose a new path')
    checks = []
    report = {'result': 'failed', 'started_at': datetime.now(timezone.utc).isoformat(),
              'archive': archive.name, 'archive_sha256': digest(archive), 'checks': checks,
              'scope': 'unpacked local darwin/arm64 candidate; offline fake credentials and upstream only'}
    try:
        with tempfile.TemporaryDirectory(prefix='model-quality-package-smoke-') as temporary:
            dest = Path(temporary)
            with tarfile.open(archive) as tar:
                for member in tar.getmembers():
                    path = Path(member.name)
                    assert member.isfile() and not path.is_absolute() and '..' not in path.parts
                    assert path.parts[0] == 'model-quality'
                # Every member was checked above: regular files only, no absolute or parent paths.
                tar.extractall(dest)
            install = dest / 'model-quality'
            manifest = json.loads((install / 'artifacts/release-manifest.json').read_text())
            for rel, info in manifest['files'].items():
                assert digest(install / rel) == info['sha256'], rel
            report.update(ui_version=manifest['ui_version'], plugin_version=manifest['version'],
                          build_id=manifest['build_id'], payload_files=len(manifest['files']))
            checks.append('every unpacked payload file matches the release manifest')
            demo_script = install / 'scripts/demo.py'
            env = os.environ.copy()
            env['MQ_DEMO_DIR'] = str(dest / 'isolated-runtime')
            def demo_command(command):
                result = subprocess.run([sys.executable, str(demo_script), command], cwd=install,
                                        env=env, capture_output=True, text=True, timeout=75)
                if result.returncode:
                    raise RuntimeError(command + ': ' + result.stderr)
                return json.loads(result.stdout)
            state = None
            try:
                state = demo_command('start')
                base = state['host_url']
                key = state['admin_key']
                api = lambda route, body=None, method=None, expected=200: request(
                    base + '/v8/management/plugins/model-quality' + route,
                    body, key, method, expected=expected)
                stats = lambda: request(state['upstream_url'] + '/stats')
                mode = lambda value: request(state['upstream_url'] + '/control', {'mode': value}, state['control_token'])
                served = {}
                for name in UI_NAMES:
                    data = request(base + '/v0/resource/plugins/model-quality/' + name, raw=True)
                    expected = (install / 'plugin/ui' / name).read_bytes()
                    assert data == expected == (install / 'artifacts/ui' / name).read_bytes(), name
                    served[name] = hashlib.sha256(data).hexdigest()
                report['served_sha256'] = served
                checks.append('loaded package library serves all five UI assets byte-identical to source and static distribution')
                request(base + '/v8/management/plugins/model-quality/targets', expected=401)
                targets = api('/targets')['items']
                target = next(item for item in targets if 'mq-reasoner' in item['models'])
                assert len(stats()['calls']) == 0
                checks.append('public assets and authenticated reads consume no model calls; unauthenticated management returns 401')
                config = {'auth_id': target['auth_id'], 'provider': target['provider'], 'models': ['mq-reasoner'],
                          'question_id': 'logic-boxes', 'samples': 1,
                          'parameters': {'max_tokens': 128, 'stream': True}, 'judge': {'mode': 'deterministic'}}
                def finish(run_id):
                    deadline = time.monotonic() + 20
                    while time.monotonic() < deadline:
                        run = api('/runs/detail?id=' + run_id)
                        if run['status'] not in ('running', 'queued'):
                            return run
                        threading.Event().wait(.05)
                    raise RuntimeError('offline run did not finish')
                created = api('/runs', {**config, 'idempotency_key': 'package-manual'}, expected=202)
                run = finish(created['run_id'])
                assert run['status'] == 'completed'
                assert run['samples'][0]['quality_verdict'] == 'correct' and run['samples'][0]['comparable']
                assert api('/actions')['items'] == []
                checks.append('package native C ABI completes fixed-credential streaming evaluation without a quality action')
                api('/presets', {'name': 'Package smoke preset', 'config': config})
                settings = api('/settings')
                api('/settings', {**settings, 'automatic_pause': True, 'scheduling_enabled': False}, 'PUT')
                rule = api('/rules', {**config, 'name': 'Package smoke guard', 'action': 'pause_credential',
                                     'enabled': True, 'scheduling_enabled': False, 'automatic_pause': True,
                                     'fail_threshold': 3})
                assert len(stats()['calls']) == 1
                checks.append('saving a preset, settings and a rule makes no model calls')
                mode('incorrect')
                for _ in range(3):
                    finish(api('/rules/run', {'id': rule['id']}, expected=202)['run_id'])
                action = api('/actions')['items'][0]
                assert action['auth_id'] == target['auth_id']
                assert next(t for t in api('/targets')['items'] if t['auth_id'] == target['auth_id'])['quality_paused']
                calls_before = len(stats()['calls'])
                api('/runs', {**config, 'idempotency_key': 'package-blocked'}, expected=409)
                api('/rules/run', {'id': rule['id']}, expected=409)
                assert len(stats()['calls']) == calls_before == 4
                checks.append('three complete incorrect rounds pause the credential and block new manual/rule evaluation without upstream calls')
                binding = {'rule_id': rule['id'], 'action_id': action['action_id'], 'action_revision': action['action_revision']}
                api('/actions/preview', binding)
                api('/actions/release', {**binding, 'pause_rule': True})
                assert api('/actions')['items'] == []
                released = next(item for item in api('/rules')['items'] if item['id'] == rule['id'])
                assert released['status'] == 'paused' and not released['enabled'] and released['fail_count'] == 0
                api('/rules/run', {'id': rule['id']}, expected=409)
                assert len(stats()['calls']) == calls_before
                checks.append('manual owned release clears the pause and old failure count while keeping detection stopped')
                report['upstream_calls'] = len(stats()['calls'])
                report['result'] = 'passed'
            finally:
                stopped = demo_command('stop')
                if state:
                    assert stopped['status'] == 'stopped'
                    report['cleanup'] = 'only the temporary unpacked installation was stopped and removed'
    except Exception as error:
        report['result'] = 'failed'
        report['error'] = str(error)
        raise
    finally:
        report['finished_at'] = datetime.now(timezone.utc).isoformat()
        report_path.parent.mkdir(parents=True, exist_ok=True)
        report_path.write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
        sums = archive.parent / 'SHA256SUMS'
        if report_path.parent == archive.parent and sums.is_file():
            with sums.open('a') as output:
                output.write(digest(report_path) + '  ' + report_path.name + '\n')
        print(json.dumps(report, ensure_ascii=False, indent=2))


if __name__ == '__main__':
    main()
