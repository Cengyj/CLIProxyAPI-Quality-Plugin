"""Verify that a confirmed disabled record is retired for a verified active replacement."""

import json
import subprocess
import time


def verify_replacements(prefix, request, restart, root, models, credential, run, recorded_calls):
    rule = request(prefix + '/rules', {
        'name': 'Synthetic credential replacement', 'target_scope': 'all_enabled',
        'models': models, 'samples': 2, 'question_id': 'logic-boxes',
        'parameters': {'max_tokens': 128, 'reasoning_effort': 'high'},
        'action': 'disable_credential', 'auto_restore': True, 'upstream_error_actions': {'unauthorized': 'disable_credential'},
        'enabled': True, 'activation': 'keep',
    })
    fault = {'status': 401, 'error': {'code': 'invalid_api_key', 'message': 'Synthetic key rejected'}}
    filename = 'one.json'
    file = root / 'auths' / filename
    phases = []

    def current():
        return next(r for r in request(prefix + '/rules')['items'] if r['id'] == rule['id'])

    def handling():
        row, = current()['handling']
        assert row['auth_id'] == filename and row['state'] == 'disabled'
        return row

    def upload(data):
        request('/v8/management/credentials?name=' + filename, data)
        deadline = time.monotonic() + 20
        while time.monotonic() < deadline:
            target = next((t for t in request(prefix + '/targets')['items'] if t['auth_id'] == filename), None)
            if target and target.get('email') == data['email'] and target['runtime_status'] == 'active' and target['effective_available'] and not target['manual_disabled'] and set(models) <= set(target['models']):
                assert not json.loads(file.read_text()).get('disabled', False)
                return
            time.sleep(.15)
        raise AssertionError(('CPA did not expose the enabled replacement', filename, data['email']))

    def suspended(outcomes, email):
        rows = run(rule, outcomes, 'disable')
        assert len(rows) == 1 and rows[0]['action_status'] == 'credential_suspended' and rows[0]['credential_email'] == email
        return rows[0]

    wrong_answers = ('4', '4', '4', '5')
    first = suspended(wrong_answers, 'one@example.test')
    recovered_from = handling()['action_id']
    start = len(recorded_calls())
    recovered, = run(rule, write='enable')
    assert recovered['action_status'] == 'credential_restored' and not current().get('handling')
    assert recovered['config']['managed_probe']['disabled'] and recovered['credential_email'] == first['credential_email']
    assert recovered['config']['managed_probe']['fingerprint'] == first['config']['managed_probe']['fingerprint']
    assert [sample['quality_verdict'] for sample in recovered['samples']] == ['correct'] * 4
    recovery_calls = recorded_calls()[start:]
    assert [call['model'] for call in recovery_calls] == [model for model in models for _ in range(2)]
    assert all(call['disabled'] for call in recovery_calls)
    recovery = {'suspended_run': first['id'], 'action_id': recovered_from, 'restored_run': recovered['id'],
                'verdicts': [sample['quality_verdict'] for sample in recovered['samples']],
                'models_probed_while_disabled': [call['model'] for call in recovery_calls],
                'action_status': recovered['action_status']}
    old = suspended(wrong_answers, 'one@example.test')
    owner = handling()
    data = credential('current')
    for stage, outcomes in (('same filename, changed account', ('4', '4', '4', '5')),
                            ('same account, renewed access token', (fault,))):
        upload(data)
        before = len(recorded_calls())
        rows = run(rule)
        if not rows:
            detail = current()['last_scope_dispatch']['details'][0]
            raise AssertionError({'stage': stage, 'cpa_active': True, 'email': data['email'], 'skip_code': detail['code'],
                                  'skip_reason': detail['reason'], 'calls': len(recorded_calls())-before})
        passed, = rows
        assert len(passed['samples']) == 4 and all(s['quality_verdict'] == 'correct' for s in passed['samples'])
        assert passed['credential_email'] == data['email'] and not current().get('handling')
        passed_snapshot = passed['config']['managed_probe']
        old_snapshot = old['config']['managed_probe']
        assert passed_snapshot['auth_id'] == old_snapshot['auth_id'] == filename
        assert passed_snapshot['fingerprint'] != old_snapshot['fingerprint'] and not passed_snapshot['disabled']
        changed = suspended(outcomes, data['email'])
        next_owner = handling()
        assert next_owner['action_id'] != owner['action_id']
        assert changed['config']['managed_probe']['fingerprint'] == passed_snapshot['fingerprint']
        retained = request(prefix + '/runs/detail?id=' + old['id'])
        assert retained['credential_email'] == old['credential_email'] and retained['samples'] == old['samples']
        assert retained['action_status'] == old['action_status'] and retained['config']['managed_probe'] == old_snapshot
        audit = request(prefix + '/history?limit=100')['items']
        events = [e for e in audit if e['kind'] == 'credential_external_change' and e.get('run_id') == old['id'] and e.get('action_id') == owner['action_id']]
        assert len(events) == 1, ('missing durable release of superseded ownership', old['id'], owner['action_id'])
        phases.append({'stage': stage, 'old_run': old['id'], 'old_action': owner['action_id'], 'old_email': old['credential_email'],
                       'external_change_audit': events[0]['id'], 'new_email': passed['credential_email'], 'passed_run': passed['id'],
                       'correct_answers': len(passed['samples']), 'new_evidence_run': changed['id'], 'new_evidence_samples': len(changed['samples']),
                       'new_action': next_owner['action_id'], 'confirmed_status': changed['action_status']})
        old, owner = changed, next_owner
        data = {**data, 'access_token': data['access_token'] + '-renewed'}
    calls_before_restart = len(recorded_calls())
    restart()
    deadline = time.monotonic() + 30
    while True:
        try:
            if handling()['action_id'] == owner['action_id']:
                break
        except (subprocess.CalledProcessError, AssertionError, KeyError, StopIteration, ValueError):
            pass
        if time.monotonic() >= deadline:
            raise AssertionError('New ownership did not survive restart')
        time.sleep(.2)
    total = request(prefix + '/runs?rule_id=' + rule['id'])['total']
    assert run(rule) == [] and request(prefix + '/runs?rule_id=' + rule['id'])['total'] == total
    assert current()['last_scope_dispatch']['skipped'] == 0
    writes = [json.loads(line) for line in (root / 'writes.jsonl').read_text().splitlines()]
    assert [w['disabled'] for w in writes] == [True, False, True, True, True]
    assert all(w['method'] == 'PATCH' and w['name'] == filename and w['cpa_status'] == 200 for w in writes)
    assert len(recorded_calls()) == calls_before_restart and sum(c['disabled'] for c in recorded_calls()) == len(recovery_calls)
    return {'rule_id': rule['id'], 'target_scope': rule['target_scope'], 'auto_restore': True, 'rounds': phases,
            'matching_snapshot_recovery': recovery, 'model_calls': len(recorded_calls()),
            'calls_while_disabled': len(recovery_calls), 'plugin_writes': writes, 'restart_kept_error_suspension': True}
