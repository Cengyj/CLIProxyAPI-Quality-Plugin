#!/usr/bin/env python3
"""Reproduce the patch in an isolated source tree and compare exact file bytes."""
from pathlib import Path
import hashlib,json,shutil,subprocess,tempfile
root=Path(__file__).resolve().parents[1]
manifest=json.loads((root/'host-patches/baseline.json').read_text())
source=Path(manifest['source']);working=root/'workspaces/CLIProxyAPI';patch=root/'host-patches/model-quality.patch'
changed=subprocess.check_output(['git','diff','--name-only','HEAD'],cwd=working,text=True).splitlines()
with tempfile.TemporaryDirectory(prefix='patch-verification-',dir=root/'workspaces') as directory:
    dest=Path(directory)
    for rel,digest in manifest['sha256'].items():
        data=(source/rel).read_bytes()
        if hashlib.sha256(data).hexdigest()!=digest:raise SystemExit('Source reference changed: '+rel)
        out=dest/rel;out.parent.mkdir(parents=True,exist_ok=True);shutil.copy2(source/rel,out)
    subprocess.run(['sh',str(root/'scripts/apply_host_patch.sh'),str(dest)],check=True)
    for rel in sorted(set(manifest['sha256'])|set(changed)):
        expected,actual=working/rel,dest/rel
        if expected.exists()!=actual.exists() or (expected.is_file() and expected.read_bytes()!=actual.read_bytes()):raise SystemExit('Reproduction differs: '+rel)
print('PASS: clean 2303-file archive baseline verified; patch applied; all resulting source files byte-identical.')
print('Changed/new files:',len(changed))
print('Patch SHA256:',hashlib.sha256(patch.read_bytes()).hexdigest())
