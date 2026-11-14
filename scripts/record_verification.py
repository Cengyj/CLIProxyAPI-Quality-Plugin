#!/usr/bin/env python3
"""Run one validation command and bind its real exit status to unchanged inputs."""
import argparse
from datetime import datetime, timezone
import json
from pathlib import Path
import subprocess
import sys

from release_support import ROOT, digest, snapshot


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--name', required=True)
    parser.add_argument('--scope', choices=('host', 'plugin', 'ui', 'native', 'all'), required=True)
    parser.add_argument('--cwd', default='.')
    parser.add_argument('--artifact', action='append', default=[], help='Built output to hash after a successful command, relative to workspace')
    parser.add_argument('command', nargs=argparse.REMAINDER)
    args = parser.parse_args()
    command = args.command[1:] if args.command[:1] == ['--'] else args.command
    if not command:
        parser.error('a command is required after --')
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=True)
    report_path = output / (args.name + '.json')
    log_path = output / (args.name + '.log')
    if report_path.exists() or log_path.exists():
        parser.error('verification record already exists; use a new name or output directory')
    inputs = snapshot(args.scope)
    started = datetime.now(timezone.utc).isoformat()
    cwd = (ROOT / args.cwd).resolve()
    with log_path.open('wb') as log:
        result = subprocess.run(command, cwd=cwd, stdout=log, stderr=subprocess.STDOUT)
    final = snapshot(args.scope)
    changed = sorted(k for k in inputs.keys() | final.keys() if inputs.get(k) != final.get(k))
    record = {
        'result': 'passed' if result.returncode == 0 and not changed else 'failed',
        'started_at': started, 'finished_at': datetime.now(timezone.utc).isoformat(),
        'command': command, 'cwd': str(cwd.relative_to(ROOT)), 'scope': args.scope,
        'exit_code': result.returncode, 'changed_during_validation': changed,
        'input_sha256': inputs, 'log': str(log_path.relative_to(ROOT)),
        'log_sha256': digest(log_path),
        'output_sha256': {rel: digest(ROOT / rel) for rel in args.artifact},
    }
    report_path.write_text(json.dumps(record, indent=2) + '\n')
    print(json.dumps({'name': args.name, 'result': record['result'], 'report': str(report_path), 'changed': changed}))
    return 0 if record['result'] == 'passed' else 1


if __name__ == '__main__':
    sys.exit(main())
