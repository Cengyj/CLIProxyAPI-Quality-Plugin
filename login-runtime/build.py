#!/usr/bin/env python3
"""Build an offline installable runtime from explicitly supplied local tools."""
import argparse
import hashlib
import json
import os
import pathlib
import shutil
import signal
import subprocess
import tempfile

ROOT = pathlib.Path(__file__).resolve().parent


def digest(path):
    with path.open('rb') as stream:
        hasher = hashlib.sha256()
        for chunk in iter(lambda: stream.read(1024 * 1024), b''):
            hasher.update(chunk)
        return hasher.hexdigest()


def copy_file(source, target, executable=False):
    if source.is_symlink() or not source.is_file():
        raise ValueError('source must be a regular file: ' + str(source))
    target.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    shutil.copyfile(source, target)
    target.chmod(0o700 if executable else 0o600)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--node', type=pathlib.Path, required=True)
    parser.add_argument('--playwright', type=pathlib.Path, required=True)
    parser.add_argument('--browser', type=pathlib.Path, required=True)
    parser.add_argument('--node-license', type=pathlib.Path, required=True)
    parser.add_argument('--output', type=pathlib.Path, required=True)
    args = parser.parse_args()
    node, browser, output = args.node.resolve(), args.browser.resolve(), args.output.absolute()
    if output.exists():
        parser.error('--output must be a new directory; install upgrades by atomic directory replacement')
    env = {'PATH': '/usr/bin:/bin', 'LANG': 'en_US.UTF-8'}
    details = json.loads(subprocess.check_output([str(node), '-p', 'JSON.stringify({version:process.version,platform:process.platform,arch:process.arch})'], env=env, timeout=10))
    if int(details['version'].lstrip('v').split('.')[0]) < 22 or details['platform'] not in ('darwin', 'linux'):
        parser.error('runtime supports Node >=22 on macOS/Linux only')
    expected = json.loads((ROOT / 'playwright-files.json').read_text())
    found = {file.relative_to(args.playwright).as_posix() for file in args.playwright.rglob('*') if file.is_file()}
    if found != set(expected) or any(digest(args.playwright / name) != value for name, value in expected.items()):
        parser.error('Playwright source differs from the pinned 1.62.1 npm tarball')
    output.parent.mkdir(parents=True, exist_ok=True)
    stage = pathlib.Path(tempfile.mkdtemp(prefix='.login-runtime-', dir=output.parent))
    try:
        for name in ('worker.mjs', 'core.mjs', 'reference-flow.mjs', 'totp.mjs', 'process-scope.mjs', 'package.json', 'package-lock.json', 'SOURCE_MANIFEST.json', 'playwright-files.json'):
            copy_file(ROOT / name, stage / name)
        for name in ('toSub2-MIT.txt', 'playwright-Apache-2.0.txt'):
            copy_file(ROOT / 'licenses' / name, stage / 'licenses' / name)
        copy_file(args.node_license, stage / 'licenses' / 'node-LICENSE.txt')
        copy_file(node, stage / 'bin' / 'node', executable=True)
        for name in expected:
            copy_file(args.playwright / name, stage / 'node_modules' / 'playwright-core' / name)
        with tempfile.TemporaryDirectory(prefix='login-runtime-selftest-') as private:
            process = subprocess.Popen([str(stage / 'bin' / 'node'), str(stage / 'worker.mjs'), '--self-test'], cwd=stage, env={**env, 'HOME': private, 'TMPDIR': private, 'MQ_LOGIN_BROWSER_PATH': str(browser)}, stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
            try:
                stdout, stderr = process.communicate(timeout=35)
            finally:
                try:
                    os.killpg(process.pid, signal.SIGKILL)
                except ProcessLookupError:
                    pass
                process.wait()
        if process.returncode or stderr or len(stdout) > 12288:
            raise ValueError('runtime self-test failed; no bundle installed')
        probe = json.loads(stdout)
        if probe.get('engine') != 'official_browser_forms' or probe.get('browser_sha256') != digest(browser):
            raise ValueError('runtime self-test identity mismatch')
        files = {file.relative_to(stage).as_posix(): digest(file) for file in sorted(stage.rglob('*')) if file.is_file()}
        manifest = {'schema_version': 1, 'runtime_version': probe['runtime_version'], 'engine': 'official_browser_forms', 'platform': details['platform'], 'arch': {'x64': 'amd64'}.get(details['arch'], details['arch']), 'node': 'bin/node', 'entrypoint': 'worker.mjs', 'files': files, 'browser': {'executable_path': str(browser), 'sha256': probe['browser_sha256'], 'version': probe['browser_version']}}
        (stage / 'manifest.json').write_text(json.dumps(manifest, ensure_ascii=True, sort_keys=True, indent=2) + '\n')
        (stage / 'manifest.json').chmod(0o600)
        os.rename(stage, output)
        print(json.dumps({'manifest': str(output / 'manifest.json'), 'platform': manifest['platform'], 'arch': manifest['arch'], 'runtime_version': manifest['runtime_version'], 'node_version': details['version'], 'browser_version': probe['browser_version'], 'files': len(files)}))
    finally:
        if stage.exists():
            shutil.rmtree(stage)


if __name__ == '__main__':
    main()
