#!/usr/bin/env python3
"""Install the verified account-security plugin on the existing official local CPA."""
from datetime import datetime
import json
import os
from pathlib import Path
import shutil
import sys
from zoneinfo import ZoneInfo

import upgrade_local_official as support

ROOT = Path(__file__).resolve().parents[1]
SITE = ROOT / 'artifacts/local-cpa'
DIST = ROOT / 'artifacts/official-plugin'
EVIDENCE = ROOT / 'artifacts/account-security-0.15.0/deployment'
CHECKER = ROOT / 'scripts/check_local_official.py'
CONTROL = ROOT / 'scripts/local_cpa.sh'


def check(name, plugin, runtime):
    path = EVIDENCE / name
    support.run([sys.executable, CHECKER, '--phase', 'after', '--expected-plugin', plugin,
                 '--expected-runtime', runtime, '--output', path])
    report = json.loads(path.read_text())
    support.require(report['status'] == 'passed', 'local verification failed')
    return report


def main():
    os.umask(0o077)
    EVIDENCE.mkdir(parents=True, exist_ok=True)
    support.EVIDENCE = EVIDENCE
    support.require(not (EVIDENCE / 'deployment.json').exists(), 'deployment already recorded')
    manifest = json.loads((DIST / 'build-manifest.json').read_text())
    verified = json.loads((ROOT / 'artifacts/account-security-0.15.0/official-install/report.json').read_text())
    support.require(manifest['plugin_version'] == '0.15.0' and manifest['runtime_version'] == '2.1.0', 'unexpected build')
    support.require(verified['status'] == 'passed' and verified['archive']['library_sha256'] == manifest['library_sha256'], 'missing matching installation verification')
    support.require(support.digest(DIST / 'model-quality.dylib') == manifest['library_sha256'], 'package checksum changed')
    before = check('preflight.json', '0.14.0', '2.0.1')
    support.require(not before['account_settings']['inspection_enabled'] and not before['account_settings']['auto_relogin_enabled'], 'account automation must be quiescent')
    stamp = datetime.now(ZoneInfo('Asia/Shanghai')).strftime('%Y%m%d-%H%M%S')
    backup = SITE / 'backups' / ('account-security-0.15.0-' + stamp)
    backup.mkdir(parents=True, mode=0o700)
    state = {'status': 'preparing', 'backup': str(backup), 'plugin_version': '0.15.0', 'ui_version': '9.2.0', 'runtime_version': '2.1.0', 'started_at': datetime.now(ZoneInfo('Asia/Shanghai')).isoformat()}
    stopped = backed_up = False
    try:
        support.run(['sh', CONTROL, 'stop'])
        support.confirm_stopped()
        stopped = True
        for name in ['plugins/model-quality.dylib', 'config.yaml', 'kv.db', 'deployment.json', 'deployment-checks.json', 'access-info.json']:
            source = SITE / name
            if source.is_file():
                target = backup / name
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(source, target)
        shutil.copytree(SITE / 'plugin-data', backup / 'plugin-data', symlinks=True)
        shutil.copytree(SITE / 'auths', backup / 'auths', symlinks=True)
        backed_up = True
        host_hash = support.digest(SITE / 'bin/cli-proxy-api')
        config_hash = support.digest(SITE / 'config.yaml')
        support.install(DIST / 'model-quality.dylib', SITE / 'plugins/model-quality.dylib')
        support.run(['sh', CONTROL, 'start'])
        after = check('after.json', '0.15.0', '2.1.0')
        state['preservation_checks'] = support.compare(before, after)
        support.require(after['login_capabilities']['security_actions'] == {'reset_totp': True, 'logout_all': True}, 'security actions are unavailable')
        support.require(support.digest(SITE / 'bin/cli-proxy-api') == host_hash and support.digest(SITE / 'config.yaml') == config_hash, 'host or configuration changed')
        state.update(status='passed', plugin_sha256=manifest['library_sha256'], official_host_sha256=host_hash, config_sha256=config_hash,
                     account_count=after['accounts']['count'], bound_account_count=after['accounts']['bound_count'], job_count=after['jobs']['count'],
                     real_login_performed=False, security_actions_executed=0, model_calls_made=0, credential_contents_unchanged=True)
        deployment = json.loads((SITE / 'deployment.json').read_text())
        deployment.update(plugin_version='0.15.0', ui_version='9.2.0', runtime_version='2.1.0', plugin_sha256=manifest['library_sha256'], account_security_update=state)
        temporary = SITE / 'deployment.security-new.json'
        support.record(temporary, deployment)
        os.replace(temporary, SITE / 'deployment.json')
    except BaseException as error:
        state.update(status='failed', error=type(error).__name__ + ': ' + str(error))
        if stopped:
            try:
                support.run(['sh', CONTROL, 'stop'])
                support.confirm_stopped()
                if backed_up:
                    for name in ['plugins/model-quality.dylib', 'config.yaml', 'kv.db', 'deployment.json', 'deployment-checks.json']:
                        previous = backup / name
                        if previous.is_file():
                            support.install(previous, SITE / name, previous.stat().st_mode & 0o777)
                    os.rename(SITE / 'plugin-data', backup / 'failed-plugin-data')
                    shutil.copytree(backup / 'plugin-data', SITE / 'plugin-data', symlinks=True)
                support.run(['sh', CONTROL, 'start'])
                state['rollback'] = 'previous plugin and state restored; host and credential files untouched'
            except BaseException as rollback_error:
                state['rollback_error'] = str(rollback_error)
        raise
    finally:
        state['finished_at'] = datetime.now(ZoneInfo('Asia/Shanghai')).isoformat()
        support.record(EVIDENCE / 'deployment.json', state)
    print(json.dumps({'status': state['status'], 'account_count': state['account_count'], 'job_count': state['job_count'], 'backup': str(backup)}, ensure_ascii=False))


if __name__ == '__main__':
    main()
