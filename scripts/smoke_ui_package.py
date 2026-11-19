"""Check a packaged UI on an empty, network-isolated official CPA container."""
import argparse
import base64
import hashlib
import json
import secrets
import subprocess
import tempfile
import time
import uuid
import zipfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
HOST = 'cliproxyapi:8.0.76-local'
CURL = 'golang:1.26-bookworm'
PREFIX = '/v0/management/plugins/model-quality'


def docker(*args, **kwargs):
    return subprocess.run(['docker', *args], stdout=subprocess.PIPE, check=True, **kwargs).stdout


def digest(data):
    return hashlib.sha256(data).hexdigest()


def check(package, output, quality_scope=False):
    manifest = json.loads((package / 'build-manifest.json').read_bytes())
    archive = package / f'model-quality-v{manifest["plugin_version"]}-linux-arm64.zip'
    assert digest(archive.read_bytes()) == manifest['checksums'][archive.name]
    name = 'mq-ui-check-' + uuid.uuid4().hex[:12]
    key = secrets.token_urlsafe(32)

    def get(route, auth=False, payload=None, idempotency=None, method=None):
        args = ['run', '--rm', '--pull=never', '--network', f'container:{name}', '-i', CURL, 'curl', '-qsS', '--noproxy', '*', '--max-time', '5', '-w', '\n%{http_code}']
        # Send the disposable token through stdin, not process arguments.
        if auth:
            args.extend(['--header', '@-'])
        if payload is not None:
            args.extend(['--header', 'Content-Type: application/json', '--data-binary', json.dumps(payload)])
        if method:
            args.extend(['-X', method])
        if idempotency:
            args.extend(['--header', 'Idempotency-Key: ' + idempotency])
        raw = docker(*args, 'http://127.0.0.1:18379' + route, input=(f'Authorization: Bearer {key}\n'.encode() if auth else b''), stderr=subprocess.PIPE)
        body, status = raw.rsplit(b'\n', 1)
        return int(status), body

    def fetch(route):
        status, body = get(route, True)
        assert status == 200, (route, status)
        return json.loads(body)

    with tempfile.TemporaryDirectory(prefix='mq-ui-check-') as tmp:
        root = Path(tmp)
        for directory in ['auths', 'plugins', 'data']:
            (root / directory).mkdir()
        with zipfile.ZipFile(archive) as z:
            assert z.namelist() == ['model-quality.so']
            library = z.read('model-quality.so')
            assert digest(library) == manifest['checksums']['model-quality.so']
            (root / 'plugins/model-quality.so').write_bytes(library)
        config = {
            'config-version': 8, 'server': {'host': '127.0.0.1', 'port': 18379, 'discovery': {'enabled': False}, 'kv-store': {'path': '/probe/kv.db'}},
            'management': {'secret-key': key, 'disable-control-panel': True, 'disable-auto-update-panel': True},
            'oauth': {'auth-dir': '/probe/auths'}, 'plugins': {'enabled': True, 'dir': '/probe/plugins', 'configs': {'model-quality': {'enabled': True, 'data_dir': '/probe/data', 'scheduling_enabled': False}}},
        }
        if quality_scope:
            config['api-keys'] = {'openai-compatibility': [{'name': 'catalog-only', 'base-url': 'http://127.0.0.1:18887/v1', 'keys': [{'api-key': 'fixture-catalog-only'}], 'models': [{'name': 'offline-model', 'alias': 'offline-model'}]}]}
            for synthetic in ('fixture-one', 'fixture-two'):
                (root / 'auths' / (synthetic + '.json')).write_text(json.dumps({'type':'kimi', 'access_token':synthetic, 'base_url':'http://127.0.0.1:18887'}))
        (root / 'config.yaml').write_text(json.dumps(config))
        docker('run', '-d', '--pull=never', '--network', 'none', '--name', name, '-e', 'TZ=Asia/Kathmandu', '--mount', f'type=bind,source={root},target=/probe', HOST, './CLIProxyAPI', '--config', '/probe/config.yaml', '--standalone', '--local-model')
        try:
            phases = []
            for phase in ['load', 'restart']:
                if phase == 'restart':
                    docker('restart', name)
                until = time.monotonic() + 30
                while time.monotonic() < until:
                    try:
                        status, body = get('/v8/management/plugins', True)
                        entries = json.loads(body)['plugins'] if status == 200 else []
                        if any(e['id'] == 'model-quality' and e['registered'] and e['effective_enabled'] for e in entries):
                            break
                    except (subprocess.CalledProcessError, ValueError):
                        pass
                    time.sleep(.2)
                else:
                    raise RuntimeError(f'Isolated CPA did not register the plugin: {phase}')
                entry = next(e for e in entries if e['id'] == 'model-quality')
                assert entry['metadata']['version'] == manifest['plugin_version']
                assert get(PREFIX + '/settings')[0] == 401
                assert get('/v0/resource/plugins/model-quality/login/accounts')[0] == 404
                capabilities = fetch(PREFIX + '/login/capabilities')
                runtime = capabilities['runtime']
                assert runtime['runtime_version'] == manifest['runtime_version']
                converted = False
                if quality_scope:
                    assert get(PREFIX + '/rules/scheduling', payload={'expected': False, 'enabled': True}, method='PUT')[0] == 401
                    settings = fetch(PREFIX + '/settings')
                    assert settings['scheduling_enabled'] is False and not settings['capabilities']['quality_pause']
                    preview = PREFIX + '/rules/question-preview?id='
                    assert get(preview + 'logic-candy')[0] == 401
                    assert all('answer' not in q for q in fetch(PREFIX + '/questions')['items'])
                    assert fetch(preview + 'logic-candy')['answer'] == '21'
                    assert get(preview + 'random-linear', True)[0] == 404
                    targets = fetch(PREFIX + '/targets')['items']
                    active = [t for t in targets if t['runtime_status'] == 'active' and t['effective_available']]
                    assert len(targets) == 3 and len(active) == 2 and all(t['models'] for t in active), targets
                    registered_models = ['kimi-k2.5', 'kimi-k2.6']
                    assert all(set(registered_models) <= set(t['models']) for t in active)
                    if phase == 'load':
                        body = {'name': 'scope-fixture', 'enabled': True, 'target_scope': 'all_enabled', 'models': registered_models, 'question_id': 'logic-boxes', 'samples': 2, 'parameters': {'reasoning_effort': 'high'}}
                        status, data = get(PREFIX + '/rules', True, body)
                        assert status == 200, (status, data)
                        rule = json.loads(data)
                        assert rule['target_scope'] == 'all_enabled' and not rule.get('auth_id') and fetch(PREFIX + '/runs')['total'] == 0
                        assert rule['cron'] == '*/30 * * * *' and rule['max_results'] == 100 and rule['next_run'].startswith('0001-') and not rule['scheduling_enabled']
                        assert not any(k in rule for k in ('interval_seconds', 'timezone', 'retention_days'))
                        for extra in ({'cron': '65 * * * *'}, {'cron': ''}, {'max_results': 201}, {'parameters': {'reasoning_effort': 'auto'}}, {'parameters': {'reasoning_effort': 'minimal'}}):
                            assert get(PREFIX + '/rules', True, dict(body, **extra))[0] == 400
                        retired = dict(body, id=rule['id'], revision=rule['revision'], parameters={'reasoning_effort': 'minimal'})
                        assert get(PREFIX + '/rules', True, retired, method='PUT')[0] == 400
                        scope_path = PREFIX + '/rules/scheduling'
                        for expected, enabled, wanted in [(False, True, 200), (False, True, 409), (True, False, 200)]:
                            assert get(scope_path, True, {'expected': expected, 'enabled': enabled}, method='PUT')[0] == wanted
                        assert fetch(PREFIX + '/settings') == settings
                        # Capture only the selected fields from this offline fixture, never real credentials or prompts.
                        fake = """from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
import json
class Mock(BaseHTTPRequestHandler):
 def do_POST(self):
  payload=json.loads(self.rfile.read(int(self.headers['Content-Length'])));model=payload['model']
  key=self.headers.get('Authorization','')
  if key not in ('Bearer fixture-one','Bearer fixture-two') or not model: self.send_error(400); return
  with open('/tmp/calls','a') as f: f.write(json.dumps([key,model,payload.get('thinking')])+ '\\n')
  if payload.get('stream'):
   part={'id':'offline','object':'chat.completion.chunk','model':model,'choices':[{'index':0,'delta':{'role':'assistant','content':'offline sample'},'finish_reason':None}]}
   chunks=[json.dumps(part)];part['choices']=[{'index':0,'delta':{},'finish_reason':'stop'}];chunks.extend([json.dumps(part),'[DONE]']);b=''.join('data: '+c+'\\n\\n' for c in chunks).encode();mime='text/event-stream'
  else:
   b=json.dumps({'model':model,'choices':[{'index':0,'message':{'role':'assistant','content':'offline sample'},'finish_reason':'stop'}], 'usage':{'prompt_tokens':2,'completion_tokens':1,'total_tokens':3}}).encode();mime='application/json'
  self.send_response(200);self.send_header('Content-Type',mime);self.send_header('Content-Length',str(len(b)));self.end_headers();self.wfile.write(b)
ThreadingHTTPServer(('127.0.0.1',18887),Mock).serve_forever()"""
                        mock = name + '-mock'
                        docker('run', '-d', '--pull=never', '--name', mock, '--network', f'container:{name}', CURL, 'python3', '-u', '-c', fake)
                        try:
                            history, total_calls, observed = {}, 0, {}
                            # These two Kimi models expose only low/high; never promote these wire observations to a universal mapping.
                            for requested, wire in [('high','high'),('low','low'),('medium','low'),('xhigh','high'),('max','high')]:
                                if requested != 'high':
                                    params = {'reasoning_effort':requested,'stream':requested in ('medium','xhigh')}
                                    update = dict(body, id=rule['id'], revision=rule['revision'], parameters=params)
                                    status, data = get(PREFIX + '/rules', True, update, method='PUT')
                                    assert status == 200, (status, data)
                                    rule = json.loads(data)
                                assert rule['parameters']['reasoning_effort'] == requested
                                status, data = get(PREFIX + '/rules/run', True, {'id': rule['id'], 'revision': rule['revision']})
                                receipt = json.loads(data)
                                assert status == 202 and receipt['count'] == 2 and receipt['skipped'] == 1, receipt
                                batch = receipt['scope_batch_id']; history[batch] = requested
                                batch_path = PREFIX + '/runs?source=scheduled_probe&scope_batch_id=' + batch
                                for _ in range(20):
                                    children = fetch(batch_path)['items']
                                    if len(children) == 2 and all(r['status'] == 'completed' for r in children): break
                                    time.sleep(.1)
                                assert {r['config']['auth_id'] for r in children} == {t['auth_id'] for t in active}
                                assert all(r['status'] == 'completed' and r['config']['models'] == registered_models and r['config']['parameters']['reasoning_effort'] == requested and len(r['samples']) == 4 and all(s['execution_status'] == 'completed' and s['output'] == 'offline sample' and not s['comparable'] for s in r['samples']) for r in children), (requested, [(r['status'],r.get('error'),r['samples']) for r in children])
                                calls = [json.loads(line) for line in docker('exec', mock, 'cat', '/tmp/calls').decode().splitlines()][total_calls:]
                                assert sorted((key, model) for key, model, _ in calls) == sorted((key, model) for key in ('Bearer fixture-one','Bearer fixture-two') for model in ('k2.5','k2.6') for _ in range(2)), calls
                                assert all(thinking == {'type':'enabled','effort':wire} for _, _, thinking in calls), (requested, calls)
                                observed[requested] = calls[0][2]['effort']; total_calls += len(calls)
                        finally:
                            docker('rm', '-f', mock)
                        body.pop('enabled')
                        empty_write = dict(body, models=['not-registered'], name='no matching model', activation='start', cron='15 10 * * *', max_results=7, parameters={'reasoning_effort':'max'})
                        status, data = get(PREFIX + '/rules', True, empty_write); assert status == 200
                        empty = json.loads(data); assert empty['next_run_local'].endswith('T10:15:00+05:45') and empty['next_run'].endswith('T04:30:00Z')
                        assert empty['timer_active'] and empty['enabled'] and empty['scheduling_enabled'] and not fetch(PREFIX + '/settings')['scheduling_enabled']
                        status, data = get(PREFIX + '/rules/run', True, {'id': empty['id'], 'revision': empty['revision']})
                        assert status == 200 and json.loads(data) == {'count': 0, 'skipped': 3}
                    else:
                        stored_runs = fetch(PREFIX + '/runs')
                        assert stored_runs['total'] == len(stored_runs['items']) == 2 * len(history) == 10
                        assert all(r['config']['parameters']['reasoning_effort'] == history[r['config']['scope_batch_id']] for r in stored_runs['items'])
                        stored_rules = fetch(PREFIX + '/rules')['items']; assert len(stored_rules) == 2
                        assert all(r['parameters']['reasoning_effort'] == 'max' for r in stored_rules)
                        saved = next(r for r in stored_rules if r['id'] == empty['id'])
                        assert all(saved[k] == empty[k] for k in ('cron','max_results','next_run','next_run_local','parameters','target_scope','timer_active'))
                        assert saved['timer_active'] and not next(r for r in stored_rules if r['id'] == rule['id'])['timer_active']
                        assert not fetch(PREFIX + '/settings')['scheduling_enabled']
                        stop = dict(empty_write, id=saved['id'], revision=saved['revision'], activation='stop')
                        status, data = get(PREFIX + '/rules', True, stop, method='PUT')
                        assert status == 200 and not json.loads(data)['timer_active'] and json.loads(data)['next_run'].startswith('0001-')
                    history_page = fetch(PREFIX + '/runs?source=scheduled_probe&rule_id=' + rule['id'] + '&offset=8&limit=1')
                    assert history_page['total'] == 10 and len(history_page['items']) == 1 and history_page['items'][0]['config']['rule_id'] == rule['id']
                    assert fetch(PREFIX + '/runs?source=scheduled_probe&rule_id=' + empty['id'])['total'] == 0
                    assert fetch(PREFIX + '/runs?source=manual_test&rule_id=' + rule['id'])['total'] == 0
                elif capabilities.get('credential_conversion'):
                    claims = {'exp': 1999999999, 'email': 'conversion@example.test', 'https://api.openai.com/auth': {'chatgpt_account_id': 'synthetic-account', 'chatgpt_user_id': 'synthetic-user'}}
                    token = 'e30.' + base64.urlsafe_b64encode(json.dumps(claims).encode()).decode().rstrip('=') + '.synthetic'
                    source = json.dumps({'accounts': [{'platform': 'openai', 'type': 'oauth', 'credentials': {'access_token': token}}]})
                    preview = PREFIX + '/login/conversion/preview'
                    upload = PREFIX + '/login/conversion/upload'
                    assert get(preview, payload={'source': source})[0] == 401
                    assert get(upload, payload={'source': source, 'indexes': [0]}, idempotency='unauthenticated')[0] == 401
                    status, data = get(preview, True, {'source': source})
                    assert status == 200 and json.loads(data)['ready'] == 1
                    candidate = json.loads(data)['items'][0]
                    assert candidate['credential']['type'] == 'codex' and 'id_token' not in candidate['credential']
                    status, data = get(upload, True, {'source': source, 'indexes': [0]}, 'synthetic-' + phase)
                    result = json.loads(data)
                    assert status == 200 and result['items'][0]['status'] == 'uploaded', (status, result)
                    assert result['items'][0]['enabled'] is True
                    files = fetch('/v0/management/auth-files')['files']
                    assert len(files) == 1 and files[0]['name'] == candidate['filename']
                    if phase == 'load':
                        converted_hash = digest((root / 'auths' / candidate['filename']).read_bytes())
                    else:
                        assert converted_hash == digest((root / 'auths' / candidate['filename']).read_bytes())
                    converted = True
                assert not fetch(PREFIX + '/login/accounts')['items']
                assert not fetch(PREFIX + '/login/jobs')['items']
                hashes = {}
                for resource, expected in manifest['ui_source_sha256'].items():
                    status, data = get('/v0/resource/plugins/model-quality/' + resource)
                    assert status == 200 and digest(data) == expected == digest((ROOT / 'plugin/ui' / resource).read_bytes()), resource
                    hashes[resource] = expected
                phases.append({'phase': phase, 'registered': True, 'effective_enabled': True, 'version': entry['metadata']['version'], 'runtime': runtime['runtime_version'], 'ui_source_sha256': hashes, 'unauthenticated_status': 401, 'accounts': 0, 'jobs': 0, 'synthetic_conversion_uploaded': converted, 'scope_verified': quality_scope, 'rule_history_filter': quality_scope, **({'mock_calls':total_calls, 'models':registered_models, 'requested_to_wire':observed} if quality_scope and phase == 'load' else {})})
            output.write_text(json.dumps({'image': HOST, 'network': 'none', 'ui': manifest['ui_version'], 'zip_sha256': digest(archive.read_bytes()), 'phases': phases}, indent=2) + '\n')
            print('Isolated CPA verified registration, six embedded assets, authentication, runtime, and restart.')
        except BaseException:
            # Only this disposable, offline fixture can be inspected on failure.
            diagnostic = subprocess.run(['docker', 'logs', '--tail', '100', name], capture_output=True, text=True)
            output.with_suffix('.diagnostics.log').write_text((diagnostic.stdout + diagnostic.stderr).replace(key, '<redacted>'))
            raise
        finally:
            docker('rm', '-f', name)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('package', type=Path)
    parser.add_argument('output', type=Path)
    parser.add_argument('--quality-scope', action='store_true')
    args = parser.parse_args()
    check(args.package, args.output, args.quality_scope)
