"""Verify runtime error admission with an official business call to a synthetic upstream."""

import json
import time


def verify_runtime(prefix, request, root, models, recorded_calls, mutation_log):
    rule = request(prefix + '/rules', {
        'name': 'Synthetic error and cooldown', 'target_scope': 'all_enabled', 'models': models, 'samples': 2,
        'question_id': 'logic-boxes', 'parameters': {'max_tokens': 128, 'reasoning_effort': 'high'},
        'action': 'observe_only', 'upstream_error_actions': dict.fromkeys(('unauthorized', 'payment_required', 'quota_exhausted', 'model_unsupported'), 'disable_credential'),
        'enabled': True, 'activation': 'keep',
    })

    def program(outcomes):
        (root / 'round.json').write_text(json.dumps({'start': len(recorded_calls()), 'outcomes': outcomes}))

    def fault(status, code):
        return {'status': status, 'error': {'code': code, 'type': code, 'message': 'Synthetic ' + code}}

    def target():
        return next(item for item in request(prefix + '/targets')['items'] if item['auth_id'] == 'one.json')

    program([fault(401, 'invalid_api_key')] * 20)
    request('/v1/responses', {'model': models[0], 'input': 'Synthetic business request', 'stream': False}, allowed=(401, 403, 429, 502, 503))
    reached = recorded_calls()
    assert reached and all(row['outcome']['status'] == 401 and not row['disabled'] for row in reached), reached
    deadline = time.monotonic() + 10
    while time.monotonic() < deadline:
        current = target()
        if not current['manual_disabled'] and (current['runtime_status'] != 'active' or not current['effective_available']):
            break
        time.sleep(.15)
    else:
        raise AssertionError(('CPA business failure did not produce a visible runtime restriction', current))
    assert current['runtime_status'] == 'error' and not current['effective_available'] and 'native_cooldown' in current['reasons'], current
    assert not mutation_log()
    phases = []
    for label, outcomes, status, verdicts in (
        ('healthy during runtime error', ['4'] * 4, 'observe_only', ['correct'] * 4),
        ('ordinary 429 during runtime error', [fault(429, 'rate_limit_exceeded'), '4', '4', '4'], 'insufficient_evidence', ['unknown'] + ['correct'] * 3),
        ('quota 429 during runtime error', [fault(429, 'insufficient_quota')], 'credential_suspended', ['unknown']),
    ):
        state = target()
        assert state['runtime_status'] == 'error' and not state['effective_available'] and not state['manual_disabled'] and 'native_cooldown' in state['reasons'], (label, state)
        start = len(recorded_calls())
        program(outcomes + ['4'] * 4)
        receipt = request(prefix + '/rules/run', {'id': rule['id'], 'revision': rule['revision']})
        if receipt['count'] != 1:
            raise AssertionError({'phase': label, 'status': current['runtime_status'], 'effective_available': current['effective_available'],
                                  'reasons': current.get('reasons'), 'receipt': receipt, 'new_calls': len(recorded_calls())-start})
        deadline = time.monotonic() + 20
        while time.monotonic() < deadline:
            row, = request(prefix + '/runs?scope_batch_id=' + receipt['scope_batch_id'])['items']
            if row.get('completed_at') and not row['action_status'].endswith('_pending'):
                break
            time.sleep(.1)
        else:
            raise AssertionError(('managed probes did not finish', label))
        calls = recorded_calls()[start:]
        assert [c['outcome'] for c in calls] == outcomes and not any(c['disabled'] for c in calls)
        action = 'disable_credential' if status == 'credential_suspended' else None
        assert row.get('credential_action') == action, (label, row['action_status'], row.get('credential_action'))
        assert row['action_status'] == status and [s['quality_verdict'] for s in row['samples']] == verdicts
        assert row.get('upstream_condition') == ('quota_exhausted' if action else None)
        assert len(mutation_log()) == int(bool(action))
        assert bool(json.loads((root / 'auths/one.json').read_text()).get('disabled')) == bool(action)
        phases.append({'case': label, 'receipt': receipt, 'calls': len(calls), 'action_status': row['action_status'],
                       'runtime_status_before': state['runtime_status'], 'runtime_reasons_before': state['reasons'],
                       'upstream_condition': row.get('upstream_condition'), 'verdicts': verdicts,
                       'writes_so_far': len(mutation_log())})
    empty = json.loads((root / 'auths/one.json').read_text())
    empty.update(disabled=False, email='empty@example.test', access_token='offline-empty', account_id='synthetic-empty', excluded_models=['*'])
    request('/v8/management/credentials?name=empty.json', empty)
    deadline = time.monotonic() + 20
    while time.monotonic() < deadline:
        empty_target = next((item for item in request(prefix + '/targets')['items'] if item['auth_id'] == 'empty.json'), None)
        if empty_target and not empty_target.get('models') and not empty_target['manual_disabled']:
            break
        time.sleep(.15)
    else:
        raise AssertionError(('empty catalog fixture did not register', empty_target))
    start = len(recorded_calls())
    program(['4'] * 4)
    receipt = request(prefix + '/rules/run', {'id': rule['id'], 'revision': rule['revision']})
    assert receipt['count'] == 1 and receipt['skipped'] == 0, receipt
    deadline = time.monotonic() + 20
    while time.monotonic() < deadline:
        row, = request(prefix + '/runs?scope_batch_id=' + receipt['scope_batch_id'])['items']
        if row.get('completed_at'):
            break
        time.sleep(.1)
    else:
        raise AssertionError('empty catalog probe did not finish')
    assert row['config']['auth_id'] == 'empty.json' and not row.get('preflight_detail') and not row.get('skip_detail')
    assert [c['model'] for c in recorded_calls()[start:]] == [m for m in models for _ in range(2)]
    assert [s['quality_verdict'] for s in row['samples']] == ['correct'] * 4 and len(mutation_log()) == 1
    phases.append({'case': 'empty catalog, requested models', 'receipt': receipt, 'calls': 4,
                   'action_status': row['action_status'], 'models': models, 'writes_so_far': len(mutation_log())})
    write, = [json.loads(line) for line in (root / 'writes.jsonl').read_text().splitlines()]
    assert write['method'] == 'PATCH' and write['name'] == 'one.json' and write['auth_index'] == current['auth_index'] and write['disabled'] and write['cpa_status'] == 200, write
    return {'business_fault': 401, 'business_upstream_requests': len(reached), 'credential_before_probes': current,
            'rounds': phases, 'total_cpa_writes': mutation_log(), 'confirmed_write': write}
