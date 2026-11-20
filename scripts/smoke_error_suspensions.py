"""Verify terminal error suspensions across all scopes and through a restart."""

import json
import subprocess
import time
from collections import Counter
from datetime import datetime, timezone

from smoke_upstream_errors import POSITIVE


FAULTS = {**POSITIVE, 'unsupported-404': ('model_unsupported', 404, 'model_not_found', 'Synthetic model not found')}
SUSPENDED = {*FAULTS, 'catalog'}
ACCOUNTS = [*FAULTS, 'catalog', 'healthy']


def verify_error_suspensions(prefix, request, restart, root, targets, recorded_calls, mutation_log):
    models = ['gpt-6-astra', 'gpt-6.1-sol']
    directory = {row['auth_id'].removesuffix('.json'): row for row in targets}
    assert set(directory) == set(ACCOUNTS)
    assert all(set(models) <= set(row['models']) for name, row in directory.items() if name != 'catalog')
    assert directory['catalog']['models'] and models[1] not in directory['catalog']['models']
    config = {
        'name': 'Synthetic persistent error suspensions', 'target_scope': 'all_credentials',
        'models': models, 'samples': 2, 'question_id': 'logic-boxes', 'judge': {'mode': 'deterministic'},
        'parameters': {'max_tokens': 128, 'reasoning_effort': 'high'},
        'action': 'disable_credential', 'auto_restore': True,
        'upstream_error_actions': {fault[0]: 'disable_credential' for fault in FAULTS.values()},
        'enabled': True, 'activation': 'keep', 'cron': '* * * * *',
    }
    rule = request(prefix + '/rules', config)
    fixed = request(prefix + '/rules', {**config, 'name': 'Synthetic fixed scope', 'target_scope': 'single',
                                        'auth_id': directory['one']['auth_id'], 'provider': directory['one']['provider']})
    enabled = request(prefix + '/rules', {**config, 'name': 'Synthetic enabled scope', 'target_scope': 'all_enabled'})
    phases = []
    externally_enabled = set()
    read_cutoffs = {}
    monitored_indices = {name: {target['auth_index']} for name, target in directory.items()}

    def current():
        return next(row for row in request(prefix + '/rules')['items'] if row['id'] == rule['id'])

    def manual(selected):
        return request(prefix + '/rules/run', {'id': selected['id'], 'revision': selected['revision']})['scope_batch_id']

    def schedule_after(previous):
        next_run = datetime.fromisoformat(current()['next_run'].replace('Z', '+00:00'))
        time.sleep(max(0, min(55, (next_run - datetime.now(timezone.utc)).total_seconds() - 1)))
        deadline = time.monotonic() + 75
        while time.monotonic() < deadline:
            batch = current().get('last_scope_batch')
            if batch and batch != previous:
                return batch
            time.sleep(.5)
        raise AssertionError(('cron did not produce a batch', previous))

    def writes():
        path = root / 'writes.jsonl'
        return [json.loads(line) for line in path.read_text().splitlines()] if path.exists() else []

    def target_reads():
        path = root / 'target-reads.jsonl'
        return [json.loads(line) for line in path.read_text().splitlines()] if path.exists() else []

    def plan(first=False):
        accounts = {name + '@example.test': ['4'] * 32 for name in directory}
        if first:
            for name, (_, status, code, message) in FAULTS.items():
                accounts[name + '@example.test'][0] = {'status': status, 'error': {'code': code, 'type': code, 'message': message}}
        tmp = root / 'round.next'
        tmp.write_text(json.dumps({'start': len(recorded_calls()), 'accounts': accounts}))
        tmp.replace(root / 'round.json')
        return len(recorded_calls())

    def check(label, selected, batch, start, scheduled=False, first=False):
        deadline = time.monotonic() + 45
        while time.monotonic() < deadline:
            rows = request(prefix + '/runs?scope_batch_id=' + batch)['items']
            if rows and all(row.get('completed_at') and not row['action_status'].endswith('_pending') for row in rows):
                break
            time.sleep(.1)
        else:
            raise AssertionError((label, 'batch did not finish', batch))
        by_name = {row['config']['auth_id'].removesuffix('.json'): row for row in rows}
        assert set(by_name) == (set(ACCOUNTS) if first else {'healthy'}), (label, {name: row['status'] for name, row in by_name.items()})
        assert len(rows) == len(by_name), (label, 'duplicate activity for a credential', len(rows), by_name.keys())
        assert all(row['config']['rule_id'] == selected['id'] and bool(row['config'].get('scheduled_dispatch')) == scheduled for row in rows)
        seen = recorded_calls()[start:]
        counts = Counter(call['account'].split('@')[0] for call in seen)
        expected = Counter({'healthy': 4})
        if first:
            expected.update(FAULTS.keys())
        if counts != expected or any(call['disabled'] for call in seen):
            raise AssertionError({
                'phase': label, 'actual_calls': dict(counts), 'expected_calls': dict(expected),
                'calls_while_disabled': sum(call['disabled'] for call in seen),
                'confirmed_reenables': [row['name'] for row in writes() if row['method'] == 'PATCH' and not row['disabled'] and row['cpa_status'] == 200],
            })
        assert [sample['quality_verdict'] for sample in by_name['healthy']['samples']] == ['correct'] * 4
        monitored = target_reads()
        if first:
            read_cutoffs.update(dict.fromkeys(SUSPENDED, len(monitored)))
        for name, cutoff in read_cutoffs.items():
            extra = [entry for entry in monitored[cutoff:]
                     if entry.get('auth_index') in monitored_indices[name] or entry.get('name') == name + '.json']
            assert not extra, (label, name, extra)
        actions = writes()
        expect_methods = dict.fromkeys(SUSPENDED, 'PATCH')
        assert Counter((w['name'], w['method']) for w in actions) == Counter((name + '.json', method) for name, method in expect_methods.items()), (label, actions)
        assert all(w['cpa_status'] == 200 and (w['method'] == 'DELETE' or w['disabled'] is True) for w in actions)
        cpa_calls = Counter(mutation_log())
        assert cpa_calls == Counter({('PATCH', '/v8/management/credentials/status'): len(SUSPENDED) + len(externally_enabled)}), (label, cpa_calls)
        evidence = {}
        for account in ACCOUNTS:
            assert bool(json.loads((root / 'auths' / (account + '.json')).read_text()).get('disabled')) == (account in SUSPENDED - externally_enabled)
            if first and account in SUSPENDED:
                row = by_name[account]
                assert row['action_status'] == 'credential_suspended' and row['credential_action'] == 'disable_credential', (account, row)
                if account == 'catalog':
                    assert row['status'] == 'completed' and row['config']['catalog_missing_model'] in models and not row['samples'] and not row.get('skip_detail')
                    assert row['preflight_detail']['code'] == 'missing_model'
                else:
                    assert row['upstream_condition'] == FAULTS[account][0] and len(row['samples']) == 1
                evidence[account] = {'run_id': row['id'], 'status': row['status'], 'action_status': row['action_status'],
                                     'preflight_detail': row.get('preflight_detail'), 'upstream_condition': row.get('upstream_condition'),
                                     'catalog_missing_model': row['config'].get('catalog_missing_model'), 'sample_count': len(row['samples'])}
        saved = current()
        assert saved['auto_restore'] and saved['enabled']
        dispatched = saved if selected['id'] == rule['id'] else next(item for item in request(prefix + '/rules')['items'] if item['id'] == selected['id'])
        summary = dispatched['last_scope_dispatch']
        assert dispatched['last_scope_batch'] == batch and summary['matched'] == len(rows) and summary['skipped'] == 0 and not summary.get('details'), (label, summary)
        held = {item['auth_id'].removesuffix('.json'): item['action_id'] for item in saved['handling'] if item['state'] == 'disabled'}
        assert set(held) == SUSPENDED and all(held.values())
        if phases:
            assert held == phases[0]['owned_actions']
        phases.append({'phase': label, 'rule_id': selected['id'], 'target_scope': selected['target_scope'], 'scope_batch_id': batch, 'scheduled': scheduled, 'calls_by_account': dict(counts),
                       'matched': summary['matched'], 'skipped': summary['skipped'], 'healthy_sample_count': len(by_name['healthy']['samples']), 'credential_status_writes_total': len(actions),
                       'owned_actions': held, 'error_credentials': evidence})

    start = plan(first=True)
    check('initial error suspensions', rule, manual(rule), start, first=True)
    start = plan()
    batch = manual(rule)
    check('manual after suspension', rule, batch, start)
    start = plan()
    receipt = request(prefix + '/rules/run', {'id': fixed['id'], 'revision': fixed['revision']})
    assert receipt['count'] == 0 and receipt['skipped'] == 0 and not receipt.get('run_id') and not receipt.get('scope_batch_id'), receipt
    assert request(prefix + '/runs?rule_id=' + fixed['id'])['total'] == 0
    assert len(recorded_calls()) == start and len(writes()) == len(SUSPENDED)
    for name in ('one',):
        request('/v8/management/credentials/status', {'name': name + '.json', 'auth_index': directory[name]['auth_index'], 'disabled': False}, 'PATCH')
        externally_enabled.add(name)
    fixed_receipts = {'one': receipt}
    start = plan()
    check('all-enabled peer after suspension', enabled, manual(enabled), start)
    start = plan()
    rule = request(prefix + '/rules/timer', {'id': rule['id'], 'revision': rule['revision'], 'enabled': True}, 'PUT')
    batch = schedule_after(batch)
    check('cron before restart', rule, batch, start, scheduled=True)
    before_calls = len(recorded_calls())
    restart()
    deadline = time.monotonic() + 30
    while True:
        try:
            if current().get('last_scope_batch') == batch:
                break
        except (subprocess.CalledProcessError, AssertionError, StopIteration, ValueError):
            pass
        if time.monotonic() >= deadline:
            raise AssertionError('Plugin did not recover the previous batch after restart')
        time.sleep(.2)
    assert len(recorded_calls()) == before_calls
    start = plan()
    batch = manual(rule)
    check('manual after restart', rule, batch, start)
    start = plan()
    check('cron after restart', rule, schedule_after(batch), start, scheduled=True)
    request(prefix + '/rules/timer', {'id': rule['id'], 'revision': rule['revision'], 'enabled': False}, 'PUT')
    history = request(prefix + '/runs?source=scheduled_probe&limit=100')['items']
    originals = phases[0]['error_credentials']
    diagnostic_history = {name: [row['id'] for row in history if row['config']['auth_id'] == name + '.json'] for name in originals}
    assert all(ids == [originals[name]['run_id']] for name, ids in diagnostic_history.items()), diagnostic_history
    return {'scope': 'all_credentials', 'fixed_receipts': fixed_receipts,
            'retained_original_rows': diagnostic_history, 'operator_enabled': sorted(externally_enabled),
            'later_management_probes_or_downloads': dict.fromkeys(sorted(originals), 0),
            'auto_restore': True, 'models': models, 'samples_per_model': 2,
            'rounds': phases, 'cpa_writes': writes(), 'total_fake_model_calls': len(recorded_calls())}
