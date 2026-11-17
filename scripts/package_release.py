#!/usr/bin/env python3
"""Package a named local candidate only after matching, recorded validation."""
import argparse
from datetime import datetime, timezone
import json
from pathlib import Path
import platform
import re
import subprocess
import tarfile

from release_support import ROOT, UI_NAMES, digest, host_sources, require_matching, source_files

REQUIRED = ('host-tests-fresh', 'host-race-fresh', 'host-compile', 'baseline', 'patch',
            'plugin-tests', 'plugin-race', 'plugin-vet', 'plugin-build',
            'native-lifecycle', 'question-logic', 'native-api', 'ui', 'native-ui', 'host-ui')


def verify_evidence(evidence):
    checks = {}
    files = set()
    for name in REQUIRED:
        report = evidence / (name + '.json')
        data = json.loads(report.read_text())
        if data.get('result') != 'passed' or data.get('exit_code') != 0 or data.get('changed_during_validation'):
            raise ValueError('Validation has not passed: ' + name)
        require_matching(data)
        for rel, expected in data.get('output_sha256', {}).items():
            if digest(ROOT / rel) != expected:
                raise ValueError('Build output changed: ' + rel)
        log = ROOT / data['log']
        if digest(log) != data['log_sha256']:
            raise ValueError('Validation log changed: ' + name)
        files.update((report, log))
        checks[name] = {
            'result': data['result'], 'started_at': data['started_at'], 'finished_at': data['finished_at'],
            'command': data['command'], 'exit_code': data['exit_code'],
            'record': str(report.relative_to(ROOT)), 'record_sha256': digest(report),
            'validated_inputs': len(data['input_sha256']),
        }
    for name, binary in (('host-compile', 'cli-proxy-api-quality'), ('plugin-build', 'model-quality.dylib')):
        data = json.loads((evidence / (name + '.json')).read_text())
        if data.get('output_sha256', {}).get('artifacts/' + binary) != digest(ROOT / 'artifacts' / binary):
            raise ValueError('Build record does not bind output: ' + name)
    ui_record = json.loads((evidence / 'ui.json').read_text())
    for name in ('contrast-results.json', 'polling-probe.json', 'stress-results.json', 'dialog-layout-results.json'):
        path = evidence / name
        if ui_record.get('output_sha256', {}).get(str(path.relative_to(ROOT))) != digest(path):
            raise ValueError('Browser command does not bind its measured evidence: ' + name)
        files.add(path)
    for name in ('ui-results.json', 'native-ui-results.json', 'host-ui-results.json'):
        path = evidence / name
        data = json.loads(path.read_text())
        command_name = 'native-ui' if name.startswith('native') else 'host-ui' if name.startswith('host') else 'ui'
        command_record = json.loads((evidence / (command_name + '.json')).read_text())
        if command_record.get('output_sha256', {}).get(str(path.relative_to(ROOT))) != digest(path):
            raise ValueError('Browser command does not bind its result report: ' + name)
        if data.get('result') != 'passed' or not data.get('checks'):
            raise ValueError('Browser verification has not passed: ' + name)
        for asset in UI_NAMES:
            if data.get('source_sha256', {}).get(asset) != digest(ROOT / 'plugin/ui' / asset):
                raise ValueError('Browser report source mismatch: ' + name + '/' + asset)
        if name in {'native-ui-results.json', 'host-ui-results.json'}:
            for binary in ('model-quality.dylib', 'cli-proxy-api-quality'):
                if data.get('binary_sha256', {}).get(binary, data.get('binary_sha256', {}).get('artifacts/' + binary)) != digest(ROOT / 'artifacts' / binary):
                    raise ValueError('Native browser report binary mismatch: ' + binary)
        if name == 'host-ui-results.json':
            if data.get('host_reference', {}).get('head') != 'd0126752201a35ff999bd2a1d4741215542258a8':
                raise ValueError('Host visual reference differs from the pinned revision')
            for asset in UI_NAMES:
                if data.get('native_resource_sha256', {}).get(asset) != digest(ROOT / 'plugin/ui' / asset):
                    raise ValueError('Host shell used a non-native or stale plugin resource: ' + asset)
        files.add(path)
        for screenshot_entry in data.get('screenshots', []):
            screenshot = screenshot_entry['file'] if isinstance(screenshot_entry, dict) else screenshot_entry
            candidate = (evidence / screenshot).resolve()
            if not candidate.is_relative_to(evidence.resolve()) or candidate.suffix != '.png':
                raise ValueError('Invalid screenshot path: ' + screenshot)
            if not candidate.is_file():
                raise ValueError('Missing screenshot: ' + screenshot)
            if data.get('screenshot_sha256', {}).get(screenshot) != digest(candidate):
                raise ValueError('Screenshot changed since browser verification: ' + screenshot)
            files.add(candidate)
        checks[name] = {'result': data['result'], 'checks': len(data['checks']),
                        'record': str(path.relative_to(ROOT)), 'record_sha256': digest(path)}
    native_api = evidence / 'native-api-results.json'
    native_api_data = json.loads(native_api.read_text())
    native_api_record = json.loads((evidence / 'native-api.json').read_text())
    if native_api_data.get('result') != 'passed' or not native_api_data.get('checks'):
        raise ValueError('Current question API validation has not passed')
    if native_api_record.get('output_sha256', {}).get(str(native_api.relative_to(ROOT))) != digest(native_api):
        raise ValueError('Native API command does not bind its result')
    for asset in UI_NAMES:
        if native_api_data.get('served_ui_sha256', {}).get(asset) != digest(ROOT / 'plugin/ui' / asset):
            raise ValueError('Native API served source mismatch: ' + asset)
    files.add(native_api)
    finish_path = evidence / 'finish-review.json'
    finish = json.loads(finish_path.read_text())
    if finish.get('result') != 'ship' or finish.get('blockers'):
        raise ValueError('Final independent visual review is not shippable')
    for asset in UI_NAMES:
        if finish.get('source_sha256', {}).get(asset) != digest(ROOT / 'plugin/ui' / asset):
            raise ValueError('Final visual review source mismatch: ' + asset)
    finish_text = evidence / 'finish-review.md'
    if finish.get('markdown_sha256') != digest(finish_text):
        raise ValueError('Final visual review text changed')
    files.update((finish_path, finish_text))
    for field in ('reviewed_screenshot_sha256', 'evidence_reports'):
        for rel, expected in finish.get(field, {}).items():
            reviewed = (evidence.parent / rel).resolve()
            if not reviewed.is_relative_to(evidence.resolve()) or digest(reviewed) != expected:
                raise ValueError('Final review references a changed or non-final artifact: ' + rel)
            files.add(reviewed)
    checks['finish-review'] = {'result': 'ship', 'record': str(finish_path.relative_to(ROOT)), 'record_sha256': digest(finish_path)}
    detector_path = evidence / 'design-detector.json'
    detector_findings = json.loads(detector_path.read_text())
    if not isinstance(detector_findings, list):
        raise ValueError('Unexpected design detector report format')
    files.add(detector_path)
    checks['design-detector'] = {
        'result': 'findings-recorded', 'findings': len(detector_findings),
        'record': str(detector_path.relative_to(ROOT)), 'record_sha256': digest(detector_path),
        'interpretation': 'Mechanical findings are retained; current disposition and rendered checks are documented in docs/UI_HOST_ALIGNMENT_REPORT_CN.md',
    }
    return checks, files


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--build-id', required=True)
    parser.add_argument('--evidence', type=Path, required=True)
    parser.add_argument('--output', type=Path)
    args = parser.parse_args()
    if not re.fullmatch(r'[a-zA-Z0-9][a-zA-Z0-9._-]{0,79}', args.build_id):
        parser.error('build-id must be a short filesystem-safe identifier')
    if platform.system() != 'Darwin' or platform.machine() != 'arm64':
        parser.error('This candidate target is darwin/arm64; verify other platforms separately')
    evidence = args.evidence.resolve()
    output = (args.output or ROOT / 'artifacts/candidates' / args.build_id).resolve()
    if output.exists():
        parser.error('candidate output already exists; choose a new build-id')
    version = re.search(r'const Version = "([^"]+)"', (ROOT / 'plugin/quality/types.go').read_text()).group(1)
    ui_version = re.search(r'"ui_version":"([^"]+)"', (ROOT / 'plugin/ui/index.html').read_text()).group(1)
    checks, files = verify_evidence(evidence)
    static = json.loads((ROOT / 'artifacts/ui/manifest.json').read_text())
    for name in UI_NAMES:
        expected = digest(ROOT / 'plugin/ui' / name)
        if digest(ROOT / 'artifacts/ui' / name) != expected or static.get(name, {}).get('sha256') != expected:
            raise ValueError('Static build differs from source: ' + name)
    files.update(ROOT / name for name in ('README_CN.md', 'AGENTS.md', 'PROMPT_CN.md', 'PRODUCT.md', 'DESIGN.md'))
    for directory in ('plugin', 'docs', 'scripts', 'host-patches'):
        files.update(source_files(ROOT / directory))
    files.update(host_sources())
    files.update(ROOT / 'artifacts' / name for name in ('model-quality.dylib', 'model-quality.h', 'cli-proxy-api-quality'))
    files.update(ROOT / 'artifacts/ui' / name for name in (*UI_NAMES, 'manifest.json'))
    files.add(ROOT / 'artifacts/config/model-quality.example.yaml')
    files.add(ROOT / '.impeccable/surfaces/plugin-ui-index-html.md')
    files.add(ROOT / '.impeccable/design.json')
    baseline_dir = evidence.parent / 'baseline'
    for rel in ('baseline.json', 'paired-clean/ui-results.json'):
        path = baseline_dir / rel
        if not path.is_file():
            raise ValueError('Missing baseline evidence: ' + str(path))
        files.add(path)
    before = json.loads((baseline_dir / 'paired-clean/ui-results.json').read_text())
    frozen = json.loads((baseline_dir / 'baseline.json').read_text())
    if before.get('result') != 'passed' or any(
            before.get('source_sha256', {}).get(name) != frozen['sha256'].get('plugin/ui/' + name)
            for name in UI_NAMES):
        raise ValueError('Before screenshots are not verified against the frozen baseline')
    paired = sorted((baseline_dir / 'paired-clean').glob('paired-*.png'))
    if len(paired) != 8 or any(not (evidence / path.name).is_file() for path in paired):
        raise ValueError('Eight matching before/after captures are required')
    after = json.loads((evidence / 'ui-results.json').read_text())
    for path in paired:
        if not before.get('capture_fixture_sha256', {}).get(path.name) or before['capture_fixture_sha256'][path.name] != after.get('capture_fixture_sha256', {}).get(path.name):
            raise ValueError('Before/after screenshot fixtures differ: ' + path.name)
        if before.get('screenshot_sha256', {}).get(path.name) != digest(path):
            raise ValueError('Baseline screenshot changed: ' + path.name)
    files.update(paired)
    files.update(baseline_dir / 'frozen/plugin/ui' / name for name in UI_NAMES)
    for name in ('comparison-desktop.png', 'comparison-mobile.png', 'comparison-provenance.json'):
        path = evidence.parent / name
        if not path.is_file():
            raise ValueError('Missing comparison presentation: ' + name)
        files.add(path)
    comparison = json.loads((evidence.parent / 'comparison-provenance.json').read_text())
    for rel, expected in comparison['sha256'].items():
        if digest(ROOT / rel) != expected:
            raise ValueError('Comparison presentation changed: ' + rel)
    reference = ROOT / 'workspaces/cpa-management-reference'
    files.update(reference / name for name in ('dist/index.html', 'LICENSE'))
    files.add(evidence.parent / 'reference-provenance.json')
    host_ui_data = json.loads((evidence / 'host-ui-results.json').read_text())
    if digest(reference / 'dist/index.html') != host_ui_data['host_reference']['html_sha256']:
        raise ValueError('Pinned host frontend build changed')
    records = {str(p.relative_to(ROOT)): {'bytes': p.stat().st_size, 'sha256': digest(p)} for p in sorted(files)}
    manifest = {
        'name': 'model-quality', 'version': version, 'ui_version': ui_version,
        'build_id': args.build_id, 'candidate_only': True,
        'built_platform': 'darwin/arm64',
        'go_toolchain': subprocess.check_output(['go', 'version'], text=True).strip(),
        'created_at': datetime.now(timezone.utc).isoformat(),
        'requires_patched_host': True, 'host_capability_version': 2, 'plugin_store_schema': 2,
        'pause_policy': 'manual-release-only; all retesting prohibited',
        'baseline_upstream_commit': None, 'baseline_manifest': 'host-patches/baseline.json',
        'ui_style_reference_commit': 'd0126752201a35ff999bd2a1d4741215542258a8',
        'sub2api_reference_commit': 'ccf997c73868083d96c0bec56e71c1a34d26c98a',
        'verification': checks,
        'limitations': ['macOS arm64 only; other operating systems are not verified',
                        'Browser coverage is limited to the versions identified in the browser reports',
                        'Automatic evidence verified only for native OpenAI-compatible chat/completions',
                        'Home quality actions are unsupported',
                        'Identical-material file recreation while the host is stopped cannot be detected through the current identity API'],
        'files': records,
    }
    output.mkdir(parents=True)
    validated = {
        'build_id': args.build_id, 'plugin_version': version, 'ui_version': ui_version,
        'created_at': manifest['created_at'], 'platform': 'darwin/arm64',
        'sha256': {name: digest(ROOT / 'artifacts' / name) for name in ('model-quality.dylib', 'model-quality.h', 'cli-proxy-api-quality')},
        'ui_source_sha256': {name: digest(ROOT / 'plugin/ui' / name) for name in UI_NAMES},
        'verification': checks,
    }
    validated_path = output / 'validated-build.json'
    validated_path.write_text(json.dumps(validated, ensure_ascii=False, indent=2) + '\n')
    records['artifacts/validated-build.json'] = {'bytes': validated_path.stat().st_size, 'sha256': digest(validated_path)}
    manifest_path = output / 'release-manifest.json'
    manifest_path.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n')
    archive = output / f'model-quality-{version}-ui-{ui_version}-{args.build_id}-darwin-arm64.tar.gz'
    with tarfile.open(archive, 'w:gz') as tar:
        for path in sorted(files):
            tar.add(path, arcname='model-quality/' + str(path.relative_to(ROOT)), recursive=False)
        tar.add(manifest_path, arcname='model-quality/artifacts/release-manifest.json', recursive=False)
        tar.add(validated_path, arcname='model-quality/artifacts/validated-build.json', recursive=False)
    with tarfile.open(archive) as tar:
        for rel, info in records.items():
            import hashlib
            if hashlib.sha256(tar.extractfile('model-quality/' + rel).read()).hexdigest() != info['sha256']:
                raise ValueError('Archive checksum mismatch: ' + rel)
    sums = [f'{digest(archive)}  {archive.name}', f'{digest(manifest_path)}  release-manifest.json', f'{digest(validated_path)}  validated-build.json']
    (output / 'PAYLOAD_SHA256SUMS').write_text(''.join(info['sha256'] + '  model-quality/' + rel + '\n' for rel, info in records.items()))
    sums.append(f"{digest(output / 'PAYLOAD_SHA256SUMS')}  PAYLOAD_SHA256SUMS")
    (output / 'SHA256SUMS').write_text('\n'.join(sums) + '\n')
    print(json.dumps({'archive': str(archive), 'manifest': str(manifest_path), 'files': len(records), 'ui_version': ui_version, 'plugin_version': version}, indent=2))


if __name__ == '__main__':
    main()
