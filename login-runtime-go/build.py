#!/usr/bin/env python3
"""Build and self-check a native Go/Sobek login bundle without external interpreters."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parent


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--go', default='/usr/local/go/bin/go')
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    output = args.output.absolute()
    if output.exists():
        parser.error('--output must be a new directory')
    output.parent.mkdir(parents=True, exist_ok=True)
    stage = Path(tempfile.mkdtemp(prefix='.go-login-runtime-', dir=output.parent))
    try:
        (stage / 'bin').mkdir(mode=0o700)
        entrypoint = 'bin/login-runtime.exe' if sys.platform == 'win32' else 'bin/login-runtime'
        executable = stage / entrypoint
        subprocess.run([args.go, 'build', '-trimpath', '-buildvcs=false', '-ldflags=-s -w', '-o', str(executable), './cmd/login-runtime'], cwd=ROOT, check=True)
        executable.chmod(0o700)
        with tempfile.TemporaryDirectory(prefix='go-login-selftest-') as private:
            result = subprocess.run([str(executable), '--self-test'], env={'PATH': '/usr/bin:/bin', 'HOME': private, 'TMPDIR': private, 'LANG': 'en_US.UTF-8'}, capture_output=True, timeout=20, check=True)
        if result.stderr or len(result.stdout) > 12288:
            raise ValueError('native self-test produced invalid diagnostics')
        probe = json.loads(result.stdout)
        if probe['engine'] != 'go_sobek_protocol' or probe['runtime_version'] != '2.2.1' or probe['node_required'] or probe['browser_required']:
            raise ValueError('native self-test engine mismatch')
        for name in ('NOTICE.md', 'SOURCE_MANIFEST.json', 'UPSTREAM_SOURCE_MANIFEST.json'):
            shutil.copyfile(ROOT / name, stage / name)
        if (ROOT / 'licenses').exists():
            shutil.copytree(ROOT / 'licenses', stage / 'licenses')
        for file in stage.rglob('*'):
            file.chmod(0o700 if file.is_dir() or file == executable else 0o600)
        manifest = {'schema_version': 2, 'runtime_version': probe['runtime_version'], 'engine': probe['engine'], 'platform': probe['platform'], 'arch': probe['arch'], 'entrypoint': entrypoint, 'files': {p.relative_to(stage).as_posix(): digest(p) for p in sorted(stage.rglob('*')) if p.is_file()}}
        (stage / 'manifest.json').write_text(json.dumps(manifest, indent=2, sort_keys=True) + '\n')
        (stage / 'manifest.json').chmod(0o600)
        os.rename(stage, output)
        print(json.dumps({'manifest': str(output / 'manifest.json'), 'runtime_version': probe['runtime_version'], 'engine': probe['engine'], 'platform': probe['platform'], 'arch': probe['arch'], 'browser_required': False, 'node_required': False}))
    finally:
        if stage.exists():
            shutil.rmtree(stage)


if __name__ == '__main__':
    main()
