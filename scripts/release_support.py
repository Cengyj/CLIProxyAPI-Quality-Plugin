"""Shared, source-only release inventories and integrity helpers."""
from pathlib import Path
import hashlib
import json

ROOT = Path(__file__).resolve().parents[1]
UI_NAMES = ('index.html', 'app.js', 'client.js', 'view.js', 'style.css')


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def host_sources(root=ROOT):
    baseline = json.loads((root / 'host-patches/baseline.json').read_text())
    names = set(baseline['sha256'])
    for line in (root / 'host-patches/model-quality.patch').read_text().splitlines():
        if line.startswith('+++ b/'):
            names.add(line[6:])
    directory = root / 'workspaces/CLIProxyAPI'
    known = {directory / name for name in names}
    unexpected = sorted(p for p in directory.rglob('*')
                        if p.is_file() and '.git' not in p.parts and p not in known)
    if unexpected:
        raise ValueError('Host files are missing from the baseline/patch inventory: ' +
                         ', '.join(str(p.relative_to(directory)) for p in unexpected[:8]))
    return [directory / name for name in sorted(names) if (directory / name).is_file()]


def source_files(directory):
    allowed = {'.go', '.mod', '.sum', '.html', '.js', '.css', '.md', '.json', '.yaml', '.yml', '.sh', '.py', '.cjs', '.h', '.c', '.patch'}
    return [p for p in directory.rglob('*') if p.is_file() and not p.is_symlink()
            and not {'__pycache__', '.git', 'auths', 'node_modules', 'plugin-data', '.cache', 'playwright-report', 'test-results'}.intersection(p.parts) and (p.suffix in allowed or p.name in {'LICENSE', 'NOTICE', 'CPA_LICENSE'})]


def snapshot(scope, root=ROOT):
    files = set()
    if scope in {'host', 'native', 'all'}:
        files.update(host_sources(root))
        files.update(root / 'host-patches' / name for name in ('baseline.json', 'model-quality.patch'))
    if scope in {'plugin', 'native', 'all'}:
        files.update(source_files(root / 'plugin'))
    if scope in {'ui', 'all'}:
        files.update(root / 'plugin/ui' / name for name in UI_NAMES)
        files.add(root / 'scripts/verify_ui.cjs')
    if scope == 'native':
        files.update(root / 'artifacts' / name for name in ('model-quality.dylib', 'cli-proxy-api-quality'))
        files.update(root / 'scripts' / name for name in ('demo.py', 'verify_native_ui.cjs'))
    return {str(p.relative_to(root)): digest(p) for p in sorted(files)}


def require_matching(record, root=ROOT):
    expected = record['input_sha256']
    current = snapshot(record['scope'], root)
    changed = sorted(rel for rel in expected.keys() | current.keys()
                     if expected.get(rel) != current.get(rel))
    if changed:
        raise ValueError('Validated input inventory changed: ' + ', '.join(changed[:8]))
