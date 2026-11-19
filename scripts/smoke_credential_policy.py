"""Exercise credential policies on an unchanged, network-isolated CPA with fake credentials."""
import argparse
import hashlib
import json
import secrets
import subprocess
import tempfile
import time
import uuid
import zipfile
from pathlib import Path

HOST = 'cliproxyapi:8.0.76-local'
CURL = 'golang:1.26-bookworm'
PREFIX = '/v0/management/plugins/model-quality'


def docker(*args, **kwargs):
    return subprocess.run(['docker', *args], check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, **kwargs).stdout


def main(package, output):
    manifest = json.loads((package / 'build-manifest.json').read_text())
    archive = package / f'model-quality-v{manifest["plugin_version"]}-linux-arm64.zip'
    assert hashlib.sha256(archive.read_bytes()).hexdigest() == manifest['checksums'][archive.name]
    name = 'mq-policy-check-' + uuid.uuid4().hex[:10]
    mock = name + '-upstream'
    key = secrets.token_urlsafe(32)

    def request(path, body=None, method=None):
        args = ['run', '--rm', '--pull=never', '--network', 'container:' + name, '-i', CURL, 'curl', '-qsS', '--noproxy', '*', '--max-time', '10', '-w', '\n%{http_code}', '--header', '@-']
        if body is not None:
            args += ['--header', 'Content-Type: application/json', '--data-binary', json.dumps(body)]
        if method:
            args += ['-X', method]
        raw = docker(*args, 'http://127.0.0.1:18379' + path, input=f'Authorization: Bearer {key}\n'.encode())
        raw, status = raw.rsplit(b'\n', 1)
        data = json.loads(raw)
        assert int(status) in (200, 202), (path, status, data)
        return data

    def rule(action, restore=False, scope='all_enabled'):
        return request(PREFIX + '/rules', {'name': 'isolated policy check', 'target_scope': scope, 'models': models, 'samples': 1, 'question_id': 'logic-boxes', 'judge': {'mode': 'deterministic'}, 'parameters': {'max_tokens': 128, 'reasoning_effort': 'high'}, 'action': action, 'auto_restore': restore, 'enabled': True, 'activation': 'keep'})

    def run(rule):
        receipt = request(PREFIX + '/rules/run', {'id': rule['id'], 'revision': rule['revision']})
        if not receipt['count']:
            return []
        until = time.monotonic() + 25
        while time.monotonic() < until:
            rows = request(PREFIX + '/runs?scope_batch_id=' + receipt['scope_batch_id'])['items']
            if rows and all(r['status'] not in ('queued', 'running') and not r['action_status'].endswith('_pending') for r in rows):
                return rows
            time.sleep(.1)
        raise AssertionError('Synthetic rule did not finish')

    with tempfile.TemporaryDirectory(prefix='mq-policy-offline-') as tmp:
        root = Path(tmp)
        for directory in ['auths', 'plugins', 'data']:
            (root / directory).mkdir()
        with zipfile.ZipFile(archive) as z:
            (root / 'plugins/model-quality.so').write_bytes(z.read('model-quality.so'))
        def credential(name):
            return {'type': 'codex', 'access_token': 'offline-only-' + name, 'account_id': 'synthetic-' + name, 'email': name + '@example.test', 'base_url': 'http://127.0.0.1:18887/v1'}
        (root / 'auths/one.json').write_text(json.dumps(credential('one')))
        config = {'config-version': 8, 'server': {'host': '127.0.0.1', 'port': 18379, 'discovery': {'enabled': False}, 'kv-store': {'path': '/probe/kv.db'}}, 'management': {'secret-key': key, 'disable-control-panel': True, 'disable-auto-update-panel': True}, 'oauth': {'auth-dir': '/probe/auths'}, 'plugins': {'enabled': True, 'dir': '/probe/plugins', 'configs': {'model-quality': {'enabled': True, 'data_dir': '/probe/data', 'scheduling_enabled': False}}}}
        (root / 'config.yaml').write_text(json.dumps(config))
        (root / 'mode').write_text('wrong')
        (root / 'fake.py').write_text('''from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
from pathlib import Path
import json
class Fake(BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def do_POST(self):
  p=json.loads(self.rfile.read(int(self.headers['Content-Length'])))
  mode=Path('/probe/mode').read_text();model=p['model']
  disabled=any(json.loads(f.read_text()).get('disabled') for f in Path('/probe/auths').glob('*.json'))
  with Path('/probe/calls.jsonl').open('a') as f:f.write(json.dumps({'model':model,'mode':mode,'disabled':disabled})+'\\n')
  if mode=='incomplete': event={'type':'response.output_text.delta','delta':'5'}
  else:event={'type':'response.completed','response':{'status':'completed','model':model,'output':[{'type':'message','status':'completed','content':[{'type':'output_text','text':'5' if mode=='wrong' else '4'}]}],'usage':{'input_tokens':2,'output_tokens':1,'total_tokens':3}}}
  b=('data: '+json.dumps(event)+'\\n\\n').encode()
  self.send_response(200);self.send_header('Content-Type','text/event-stream');self.send_header('Content-Length',str(len(b)));self.end_headers();self.wfile.write(b)
ThreadingHTTPServer(('127.0.0.1',18887),Fake).serve_forever()
''')
        docker('run', '-d', '--pull=never', '--network', 'none', '--name', name, '-e', 'MODEL_QUALITY_CPA_URL=http://127.0.0.1:18379', '--mount', f'type=bind,source={root},target=/probe', HOST, './CLIProxyAPI', '--config', '/probe/config.yaml', '--standalone', '--local-model')
        try:
            docker('run', '-d', '--pull=never', '--name', mock, '--network', 'container:' + name, '--mount', f'type=bind,source={root},target=/probe', CURL, 'python3', '-u', '/probe/fake.py')
            until = time.monotonic() + 30
            while time.monotonic() < until:
                try:
                    targets = request(PREFIX + '/targets')['items']
                    if len(targets) == 1 and targets[0]['models']:
                        break
                except (subprocess.CalledProcessError, AssertionError, ValueError):
                    pass
                time.sleep(.2)
            else:
                raise AssertionError('Synthetic credential did not register')
            models = targets[0]['models'][:2]
            assert targets[0]['email'] == 'one@example.test'
            assert len(models) == 2
            settings = request(PREFIX + '/settings')
            assert settings['capabilities']['credential_management'] and settings['capabilities']['management_session']
            pause = rule('disable_credential', True)
            rows = run(pause)
            assert len(rows) == 1 and rows[0]['action_status'] == 'credential_suspended', rows
            assert rows[0]['credential_email'] == 'one@example.test'
            assert json.loads((root / 'auths/one.json').read_text())['disabled'] is True
            (root / 'mode').write_text('correct')
            rows = run(pause)
            if not rows:
                blob=(root/'data/quality.db').read_bytes();at=blob.find(b'"operation":"disable_credential","state":"disabled"');start=blob.rfind(b'{"auth_id":',0,at)
                stored=json.JSONDecoder().raw_decode(blob[start:].decode('utf-8',errors='replace'))[0] if at>=0 else {}
                files=request('/v8/management/credentials')['files']
                print(json.dumps({'stored_snapshot':stored.get('snapshot'),'runtime':[{k:a.get(k) for k in ('id','name','auth_index','disabled','updated_at','created_at','modtime','status','unavailable','base_url')} for a in files],'targets':request(PREFIX+'/targets')['items']},indent=2),flush=True)
            assert len(rows) == 1 and rows[0]['action_status'] == 'credential_restored', rows
            assert json.loads((root / 'auths/one.json').read_text())['disabled'] is False
            calls = [json.loads(line) for line in (root / 'calls.jsonl').read_text().splitlines()]
            assert [c['disabled'] for c in calls] == [False, False, True, True], calls
            (root / 'mode').write_text('incomplete')
            rows = run(pause)
            assert rows[0]['action_status'] == 'insufficient_evidence'
            assert not json.loads((root / 'auths/one.json').read_text())['disabled']
            (root / 'mode').write_text('wrong')
            rows = run(pause)
            assert rows[0]['action_status'] == 'credential_suspended', rows
            request('/v8/management/credentials/status', {'name': 'one.json', 'auth_index': targets[0]['auth_index'], 'disabled': True}, 'PATCH')
            (root / 'mode').write_text('correct')
            before = len((root / 'calls.jsonl').read_text().splitlines())
            assert run(pause) == []
            assert len((root / 'calls.jsonl').read_text().splitlines()) == before
            assert json.loads((root / 'auths/one.json').read_text())['disabled']
            request('/v8/management/credentials/status', {'name': 'one.json', 'auth_index': targets[0]['auth_index'], 'disabled': False}, 'PATCH')
            current = next(r for r in request(PREFIX + '/rules')['items'] if r['id'] == pause['id'])
            held = current['handling'][0]
            request(PREFIX + '/rules/handling/acknowledge', {'rule_id': pause['id'], 'auth_id': held['auth_id'], 'action_id': held['action_id']})
            directory = rule('disable_credential', True, 'all_credentials')
            request('/v8/management/credentials/status', {'name': 'one.json', 'auth_index': targets[0]['auth_index'], 'disabled': True}, 'PATCH')
            # Wait for CPA's file watcher before freezing a fresh manual-disable round.
            time.sleep(1)
            (root / 'mode').write_text('wrong')
            assert run(directory)[0]['action_status'] == 'credential_kept_disabled'
            assert json.loads((root / 'auths/one.json').read_text())['disabled']
            (root / 'mode').write_text('correct')
            assert run(directory)[0]['action_status'] == 'credential_restored'
            assert not json.loads((root / 'auths/one.json').read_text())['disabled']
            assert all(json.loads(line)['disabled'] for line in (root / 'calls.jsonl').read_text().splitlines()[-4:])
            removal = rule('delete_credential')
            (root / 'mode').write_text('wrong')
            rows = run(removal)
            assert rows[0]['action_status'] == 'credential_removed', rows
            assert not (root / 'auths/one.json').exists()
            assert request('/v8/management/credentials')['files'] == []
            assert run(removal) == []
            request('/v8/management/credentials?name=two.json', credential('two'))
            until = time.monotonic() + 10
            while time.monotonic() < until:
                if request(PREFIX + '/targets')['items']:
                    break
                time.sleep(.2)
            rows = run(removal)
            assert len(rows) == 1 and rows[0]['action_status'] == 'credential_removed', rows
            assert not (root / 'auths/two.json').exists()
            request('/v8/management/credentials?name=three.json', credential('three'))
            until = time.monotonic() + 10
            while time.monotonic() < until:
                if any(t.get('models') for t in request(PREFIX + '/targets')['items']):
                    break
                time.sleep(.2)
            rows = run(directory)
            assert len(rows) == 1 and rows[0]['action_status'] == 'credential_suspended', rows
            orphan = rows[0]['id']
            request('/v8/management/credentials?name=three.json', method='DELETE')
            assert run(directory) == []
            until = time.monotonic() + 10
            while time.monotonic() < until:
                saved = next(r for r in request(PREFIX + '/rules')['items'] if r['id'] == directory['id'])
                if not saved.get('handling'):
                    assert saved['enabled'] and saved['revision'] == directory['revision']
                    break
                time.sleep(2)
            else:
                raise AssertionError('Deleted credential retained its suspension')
            assert orphan in [r['id'] for r in request(PREFIX + '/runs')['items']]
            cleared = request(PREFIX + '/rules/run-history', method='DELETE')
            assert cleared['deleted'] > 0 and cleared['kept'] == 0, cleared
            assert request(PREFIX + '/runs?source=scheduled_probe')['total'] == 0
            before = len((root / 'calls.jsonl').read_text().splitlines())
            docker('restart', name)
            until = time.monotonic() + 30
            while True:
                try:
                    if request(PREFIX + '/settings')['capabilities']['management_session']:
                        break
                except (subprocess.CalledProcessError, AssertionError, json.JSONDecodeError):
                    pass
                if time.monotonic() >= until:
                    raise AssertionError('Plugin did not become ready after restart')
                time.sleep(.2)
            assert run(removal) == []
            assert orphan not in [r['id'] for r in request(PREFIX + '/runs')['items']]
            assert before == len((root / 'calls.jsonl').read_text().splitlines())
            output.write_text(json.dumps({'plugin_version':manifest['plugin_version'], 'host':'unchanged official 8.0.76', 'network':'none', 'only_synthetic_credentials':True, 'suspend_and_whole_round_restore':True, 'recovery_probes_remained_disabled':True, 'incomplete_never_mutates':True, 'operator_disable_preserved':True, 'manual_resolution_stops_rule':True, 'exact_file_removal':True, 'dynamic_new_credentials':True, 'restart_no_replay':True, 'fake_model_calls':before, 'live_account_calls':0},indent=2)+'\n')
            print('Official CPA management policy smoke checks passed; all credentials and upstream responses were synthetic.')
        finally:
            for container in [mock,name]:
                subprocess.run(['docker','rm','-f',container],stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)

if __name__ == '__main__':
    parser=argparse.ArgumentParser();parser.add_argument('package',type=Path);parser.add_argument('output',type=Path);args=parser.parse_args();main(args.package.resolve(),args.output.resolve())
