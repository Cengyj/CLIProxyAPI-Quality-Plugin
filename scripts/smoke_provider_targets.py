"""Check configured-provider selection through an isolated, unmodified CPA."""
import http.client
import threading
import hashlib
import json
import shutil
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path


MODEL = 'fixture-shared-alias'
ANSWERS = {'alpha-1': '11', 'alpha-2': '21', 'beta-1': '31'}


class ProviderFixture:
    def __init__(self, root, config):
        self.root = root
        self.calls_path = root / 'provider-calls.jsonl'
        self.calls_path.touch()
        shutil.copyfile(__file__, root / 'provider-fixture.py')
        auth = {'type': 'codex', 'email': 'ordinary@example.test', 'account_id': 'fixture-file',
                'access_token': 'fixture-file-only', 'expired': '2099-01-01T00:00:00Z'}
        (root / 'auths/ordinary.json').write_text(json.dumps(auth))
        self.credential_hashes = self.file_hashes()
        config['api-keys'] = {
            'openai-compatibility': [
                {'name': name, 'base-url': f'http://127.0.0.1:18887/{name}/v1',
                 'headers': {'X-Fixture-Channel': f'channel-{name}'},
                 'models': [{'name': f'{name}-wire-model', 'alias': MODEL}, {'name': f'{name}-business', 'alias': f'{name}-business'}],
                 'keys': [{'api-key': 'fixture-key-' + key} for key in ANSWERS if key.startswith(name + '-')]}
                for name in ('alpha', 'beta')
            ],
            'codex': [{'base-url': 'http://127.0.0.1:18887/codex/v1',
                       'models': [{'name': 'fixture-codex-only', 'alias': 'fixture-codex-only'}],
                       'keys': [{'api-key': 'fixture-key-codex'}]}],
        }
        self.rule = None
        self.checks = []
        self.manual_runs = {}
        self.manual_history = {}
        self.results = []
        self.batches = []

    def file_hashes(self):
        return {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in (self.root / 'auths').glob('*.json')}

    def calls(self):
        return [json.loads(line) for line in self.calls_path.read_text().splitlines()]

    def verify(self, phase, get, fetch, docker, container, curl_image, prefix):
        mock = container + '-providers'
        docker('run', '-d', '--pull=never', '--name', mock, '--network', 'container:' + container,
               '--mount', f'type=bind,source={self.root},target=/probe', curl_image, 'python3', '-u', '/probe/provider-fixture.py')
        try:
            def request(path, body, method='POST', expected=(200, 202)):
                status, data = get(prefix + path, True, body, method=method)
                assert status in expected, (path, status, json.loads(data))
                return json.loads(data)

            targets = fetch(prefix + '/targets')['items']
            assert len(targets) == 5
            file, = [t for t in targets if not t.get('is_provider')]
            assert (file['auth_id'], file['provider']) == ('ordinary.json', 'codex')
            assert file['effective_available'] and file['runtime_status'] == 'active'
            configured = [t for t in targets if t.get('is_provider')]
            assert all(t['runtime_status'] == 'registered' and t['effective_available'] and t['provider_name'] for t in configured)
            codex, = [t for t in configured if t['provider'] == file['provider']]
            assert codex['auth_id'] != file['auth_id'] and 'fixture-codex-only' in codex['models']
            full_config = fetch('/v8/management/config/api-keys')
            routes_digest = hashlib.sha256(json.dumps(full_config, sort_keys=True).encode()).hexdigest()
            by_index = {t['auth_index']: t for t in configured}
            routes = {}
            for group in full_config['openai-compatibility']:
                for key in group['keys']:
                    suffix = key['api-key'].removeprefix('fixture-key-')
                    t = by_index[key['auth_index']]
                    assert t['provider_name'] == group['name'] and MODEL in t['models']
                    routes[suffix] = t
            assert set(routes) == set(ANSWERS)
            assert not any(f'fixture-key-{suffix}' in json.dumps(targets) for suffix in [*ANSWERS, 'codex'])

            def check_manual_run(key):
                source, target, before = 'manual_test', routes[key], len(self.calls())
                receipt = request('/runs', {'auth_id': target['auth_id'], 'provider': target['provider'], 'models': [MODEL],
                                            'samples': 1, 'question_id': 'logic-candy', 'judge': {'mode': 'manual'},
                                            'parameters': {'stream': True}})
                deadline = time.monotonic() + 20
                while True:
                    run = fetch(prefix + '/runs/detail?id=' + receipt['run_id'])
                    if run['status'] not in ('queued', 'running', 'cancelling'):
                        break
                    if time.monotonic() >= deadline:
                        raise AssertionError('Provider test did not finish')
                    time.sleep(.1)
                assert run['status'] == 'completed' and run['source'] == source, (run['status'], run.get('error'), run.get('block_reason'))
                sample, = run['samples']
                channel = key.split('-')[0]
                call, = self.calls()[before:]
                assert call == {'key': key, 'path': f'/{channel}/v1/chat/completions', 'model': channel + '-wire-model',
                                'channel_header': 'channel-' + channel, 'stream': source == 'manual_test'}, (target['auth_id'], call)
                assert sample['execution_status'] == 'completed' and sample['output'] == ANSWERS[key]
                assert not sample.get('comparable') and run['config']['auth_id'] == target['auth_id']
                assert run['config']['parameters']['stream'] and run['config']['judge']['mode'] == 'manual'
                self.manual_runs[run['id']] = {'auth_id': target['auth_id'], 'provider_name': target['provider_name'],
                                               'subject_auth_index': target['auth_index']}
                self.results.append({'phase': phase, 'source': source, **call})

            def config_writes():
                path = self.root / 'config-writes.jsonl'
                return [json.loads(line) for line in path.read_text().splitlines()] if path.exists() else []

            group = routes['alpha-1']['provider_group_id']
            def body(**overrides):
                return dict({'name': 'group test', 'enabled': True, 'object_types': ['provider'],
                        'target_scope': 'single', 'target_group': group, 'models': [MODEL], 'samples': 1,
                        'question_id': 'logic-candy', 'action': 'disable_credential', 'auto_restore': True},
                        **overrides)

            def dispatch(saved, *, calls, writes, conflict=False):
                start, initial_writes = len(self.calls()), len(config_writes())
                receipt = request('/rules/run', {'id': saved['id'], 'revision': saved['revision']})
                assert receipt.get('scope_batch_id') or receipt.get('run_id'), receipt
                url = ('/runs?scope_batch_id=' + receipt['scope_batch_id']) if receipt.get('scope_batch_id') else None
                end = time.monotonic() + 25
                while time.monotonic() < end:
                    rows = fetch(prefix + url)['items'] if url else [fetch(prefix + '/runs/detail?id=' + receipt['run_id'])]
                    if rows and all(r.get('completed_at') for r in rows) and len(config_writes()) >= initial_writes + writes and not any(r['action_status'].endswith('_pending') for r in rows):
                        break
                    time.sleep(.1)
                actual_calls = self.calls()[start:]
                assert sorted(x['key'] for x in actual_calls) == sorted(calls), (actual_calls, [(r['status'],r['action_status'],r.get('error'),r.get('skip_detail'), r.get('preflight_detail')) for r in rows])
                actual_writes = config_writes()[initial_writes:]
                assert len(actual_writes) == writes and all(x['conditional'] and x['status'] == (412 if conflict else 200) for x in actual_writes), (actual_writes, [(r['config']['auth_id'],r['action_status']) for r in rows])
                return rows, receipt

            def business_models():
                status, reply = get('/v1/models')
                assert status == 200, status
                return {m['id'] for m in json.loads(reply)['data']}

            def operator_state(disabled):
                groups = fetch('/v8/management/config/api-keys/openai-compatibility')
                next(g for g in groups if g['name'] == 'alpha')['disabled'] = disabled
                status, _ = get('/v8/management/config/api-keys/openai-compatibility', True, groups, method='PUT')
                assert status == 200
                deadline = time.monotonic() + 8
                while ('alpha-business' in business_models()) == disabled:
                    if time.monotonic() >= deadline:
                        raise AssertionError('CPA business route did not reflect operator change')
                    time.sleep(.1)

            def read_group():
                return next(g for g in fetch('/v8/management/config/api-keys')['openai-compatibility'] if g['name'] == 'alpha')

            if phase == 'load':
                self.routes_digest = routes_digest
                for objects, counts in [(['oauth'],(1,0)), (['provider'],(0,4)), (['oauth','provider'],(1,4))]:
                    preview = request('/rules/scope-preview', body(object_types=objects, target_scope='all_enabled', target_group='', action='observe_only', auto_restore=False))
                    assert preview['verified'] and (preview['candidates']['oauth'],preview['candidates']['provider']) == counts, preview
                blocked = request('/rules', body(auto_restore=False))
                self.rule = request('/rules', body(name='authorized alpha', target_scope='all_enabled', target_group='', authorized_groups=[group]))
                assert not self.calls() and not config_writes()
                for key in ('alpha-2', 'beta-1'):
                    check_manual_run(key)
                rows, _ = dispatch(blocked, calls=['alpha-1','alpha-2'], writes=0)
                assert not read_group().get('disabled')
                self.checks.append('two group members tested with zero writes before explicit group permission')
                rows, _ = dispatch(self.rule, calls=['alpha-1','alpha-2','beta-1'], writes=1)
                assert read_group()['disabled'] and 'alpha-business' not in business_models() and 'beta-business' in business_models()
                self.checks.append('one wrong route disabled the complete group after all its answers')
                disabled = [t for t in fetch(prefix + '/targets')['items'] if t.get('provider_group_id') == group]
                assert len(disabled) == 2 and all(t['manual_disabled'] for t in disabled)
                preview = request('/rules/scope-preview', self.rule)
                assert preview['recovery_count'] == 2, (preview, [(t['auth_id'],t.get('credential_handling'),t['auth_index']) for t in disabled], [(r['config']['auth_id'],r['action_status']) for r in rows], [a for a in fetch(prefix + '/history')['items'] if 'uncertain' in str(a)])
                rows, receipt = dispatch(self.rule, calls=['alpha-1','alpha-2','beta-1'], writes=0)
                assert receipt['recovery_count'] == 2 and read_group()['disabled']
                assert any(s['evidence']['path'] == 'provider_config_probe' for r in rows for s in r['samples'])
                self.checks.append('single passing key left disabled group unchanged; both keys probed while disabled')
                (self.root/'all-alpha-pass').touch()
                rows, _ = dispatch(self.rule, calls=['alpha-1','alpha-2','beta-1'], writes=1)
                assert not read_group().get('disabled')
                self.checks.append('all keys passed and one conditional write restored the group')
                assert {'alpha-business','beta-business'} <= business_models()

                all_objects = request('/rules', body(name='manually disabled', target_scope='all_credentials', target_group='', authorized_groups=[group]))
                observer = request('/rules', body(name='observe manually disabled', target_scope='all_credentials', target_group='', action='observe_only', auto_restore=False))
                operator_state(True)
                rows, receipt = dispatch(observer, calls=['alpha-1','alpha-2','beta-1'], writes=0)
                assert receipt.get('recovery_count',0) == 0 and read_group()['disabled'] and 'alpha-business' not in business_models()
                rows, receipt = dispatch(all_objects, calls=['alpha-1','alpha-2','beta-1'], writes=1)
                assert receipt.get('recovery_count',0) == 0 and not read_group().get('disabled')
                self.checks.append('manual group disable probed in all scope with zero writes; authorized all-pass round restored it')

                (self.root/'all-alpha-pass').unlink()
                (self.root/'competing-write').touch()
                rows, _ = dispatch(self.rule, calls=['alpha-1','alpha-2','beta-1'], writes=1, conflict=True)
                assert any(r['action_status']=='provider_config_conflict' for r in rows) and not read_group().get('disabled')
                self.checks.append('concurrent CPA config edit rejected the conditional write with 412; no retry')
                error_rule = request('/rules', body(name='group 401', authorized_groups=[group], auto_restore=False,
                    action='observe_only',upstream_error_actions={'unauthorized':'disable_credential'}))
                (self.root/'upstream-unauthorized').touch()
                # Group-wide HTTP evidence can stop a sibling before it starts.
                receipt = request('/rules/run', {'id':error_rule['id'],'revision':error_rule['revision']})
                deadline = time.monotonic()+12
                while time.monotonic() < deadline:
                    rows = fetch(prefix + '/runs?scope_batch_id='+receipt['scope_batch_id'])['items']
                    if any(r['action_status']=='credential_suspended' for r in rows): break
                    time.sleep(.1)
                assert any(r['action_status']=='credential_suspended' and r.get('upstream_condition')=='unauthorized' for r in rows)
                assert read_group()['disabled']
                operator_state(False)
                (self.root/'upstream-unauthorized').unlink()
                before = len(self.calls()),len(config_writes()),fetch(prefix+'/runs')['total']
                for _ in range(2):
                    receipt = request('/rules/run', {'id':error_rule['id'],'revision':error_rule['revision']})
                    assert receipt['count'] == receipt['skipped'] == 0
                assert before==(len(self.calls()),len(config_writes()),fetch(prefix+'/runs')['total'])
                self.checks.append('confirmed group 401 stayed excluded after manual enable with zero repeated calls, writes or records')
            else:
                assert not read_group().get('disabled')
                check_manual_run('alpha-1')
                preview = request('/rules/scope-preview', self.rule)
                assert preview['verified'] and preview['recovery_count'] == 0
            self.batches = self.checks
            history = fetch(prefix + '/runs?source=manual_test&view=summary')
            assert history['total'] == len(self.manual_runs)
            assert {row['id'] for row in history['items']} == set(self.manual_runs)
            for row in history['items']:
                expected = self.manual_runs[row['id']]
                detail = fetch(prefix + '/runs/detail?id=' + row['id'])
                for record in (row, detail):
                    assert record['source'] == 'manual_test' and record['id'] == row['id']
                    identity = {field: record['config'].get(field) for field in expected}
                    assert identity == expected, (phase, row['id'], expected, identity)
            self.manual_history[phase] = dict(self.manual_runs)
            assert self.file_hashes() == self.credential_hashes
            after_config = fetch('/v8/management/config/api-keys')
            for cfg in (after_config, full_config):
                for entry in cfg['openai-compatibility']:
                    if entry.get('disabled') is False:
                        entry.pop('disabled')
            assert after_config == full_config, 'unrelated configuration or routes changed'
            return {'tested_calls': self.results, 'directory_scopes': self.batches, 'config_writes': config_writes(), 'manual_history': self.manual_history,
                    'credential_hashes_unchanged': True, 'upstream_config_unchanged': True}
        finally:
            docker('rm', '-f', mock)


class MockUpstream(BaseHTTPRequestHandler):
    def do_POST(self):
        request = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
        key = self.headers.get('Authorization', '').removeprefix('Bearer fixture-key-')
        key = key if key in ANSWERS else 'unknown'
        answer = '21' if key.startswith('alpha-') and Path('/probe/all-alpha-pass').exists() else ANSWERS.get(key, '')
        if key.startswith('alpha-') and Path('/probe/all-alpha-pass').exists() and request.get('stream'):
            answer = ANSWERS[key]
        stream = request.get('stream') is True
        record = {'path': self.path, 'key': key, 'model': request.get('model'), 'channel_header': self.headers.get('X-Fixture-Channel'), 'stream': stream}
        with Path('/probe/provider-calls.jsonl').open('a') as calls:
            calls.write(json.dumps(record) + '\n')
        response = {'id': 'fixture', 'model': request.get('model')}
        if stream:
            response['object'] = 'chat.completion.chunk'
            frames = []
            for text in answer:
                choice = {'index': 0, 'delta': {'content': text}, 'finish_reason': None}
                frames.append(json.dumps({**response, 'choices': [choice]}))
            last = {'index': 0, 'delta': {}, 'finish_reason': 'stop'}
            frames.extend([json.dumps({**response, 'choices': [last]}), '[DONE]'])
            data = ''.join('data: ' + frame + '\n\n' for frame in frames).encode()
        else:
            response['object'] = 'chat.completion'
            response['choices'] = [{'index': 0, 'message': {'role': 'assistant', 'content': answer}, 'finish_reason': 'stop'}]
            data = json.dumps(response).encode()
        unauthorized = key == 'alpha-1' and Path('/probe/upstream-unauthorized').exists()
        if unauthorized: data = b'{"error":{"code":"unauthorized"}}'
        self.send_response(200 if key in ANSWERS and not unauthorized else 401)
        self.send_header('Content-Type', 'text/event-stream' if stream else 'application/json')
        self.send_header('Content-Length', str(len(data)))
        self.end_headers()
        self.wfile.write(data)


class AuditManagement(BaseHTTPRequestHandler):
    def log_message(self, *_):
        pass

    def forward(self):
        upstream = http.client.HTTPConnection('127.0.0.1', 18379, timeout=10)
        try:
            body = self.rfile.read(int(self.headers.get('Content-Length', 0)))
            marker = Path('/probe/competing-write')
            if self.command == 'PUT' and '/config/api-keys/' in self.path and marker.exists():
                marker.unlink()
                # Append a YAML comment, as an independent operator saving the
                # config would. Its contents stay semantically identical.
                with Path('/probe/config.yaml').open('a') as f:
                    f.write('\n# concurrent fixture edit\n')
            upstream.request(self.command, self.path, body or None, dict(self.headers))
            response = upstream.getresponse()
            payload = response.read()
            if self.command in ('PUT', 'PATCH', 'DELETE') and '/config/' in self.path:
                with Path('/probe/config-writes.jsonl').open('a') as output:
                    print(json.dumps({'method':self.command,'status':response.status,'conditional':bool(self.headers.get('If-Match'))}), file=output)
            self.send_response(response.status)
            for name, value in response.getheaders():
                if name.lower() in ('etag','content-type','content-encoding'):
                    self.send_header(name, value)
            self.send_header('Content-Length', str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)
        finally:
            upstream.close()

    do_GET = do_POST = do_PUT = do_PATCH = do_DELETE = forward


if __name__ == '__main__':
    audit = ThreadingHTTPServer(('127.0.0.1', 18888), AuditManagement)
    threading.Thread(target=audit.serve_forever, daemon=True).start()
    ThreadingHTTPServer(('127.0.0.1', 18887), MockUpstream).serve_forever()
