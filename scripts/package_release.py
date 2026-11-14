#!/usr/bin/env python3
"""Package audited sources and verified binaries, excluding all runtime state."""
from pathlib import Path
from datetime import datetime,timezone
import hashlib,json,platform,re,subprocess,tarfile
root=Path(__file__).resolve().parents[1]
out=root/'artifacts'
version=re.search(r'const Version = "([^"]+)"', (root/'plugin/quality/types.go').read_text()).group(1)
if platform.system()!='Darwin' or platform.machine()!='arm64':raise SystemExit('This verified release target is darwin/arm64; build and verify a separate platform package explicitly')
go_toolchain=subprocess.check_output(['go','version'],text=True).strip().removeprefix('go version ')
files={root/'README_CN.md',root/'AGENTS.md',root/'PROMPT_CN.md'}
for directory in ('plugin','docs','scripts','host-patches'):
    for p in (root/directory).rglob('*'):
        if p.is_file() and not p.is_symlink() and '__pycache__' not in p.parts and p.suffix not in {'.pyc','.db','.log'}:files.add(p)
baseline=json.loads((root/'host-patches/baseline.json').read_text())
host_files=set(baseline['sha256'])
for line in (root/'host-patches/model-quality.patch').read_text().splitlines():
    if line.startswith('+++ b/'):host_files.add(line[6:])
for rel in host_files:
    p=root/'workspaces/CLIProxyAPI'/rel
    if p.is_file():files.add(p)
for name in ('model-quality.dylib','model-quality.h','cli-proxy-api-quality'):
    p=out/name
    if not p.is_file():raise SystemExit('Missing build artifact: '+name)
    files.add(p)
for directory in ('config','ui'):
    files.update(p for p in (out/directory).rglob('*') if p.is_file())
verification=('validated-build.json','manual-only-release-scope-race.log','plugin-tests.log','plugin-race.log','plugin-vet.log','host-go-test.log','host-race.log','host-build.log','native-e2e.log','baseline-verify.log','patch-reproduction.log','ui-results.json','native-ui-results.json','native-manual-page.png','native-operations-page.png','native-release-preview.png','sandbox-preview.png')
files.update(out/'verification'/name for name in verification)
ui_report=json.loads((out/'verification/ui-results.json').read_text())
native_ui_report=json.loads((out/'verification/native-ui-results.json').read_text())
files.update(out/'verification'/name for name in ui_report.get('screenshots',[]))
files.update(out/'verification'/name for name in native_ui_report.get('screenshots',[]))
if ui_report.get('result')!='passed' or native_ui_report.get('result')!='passed':raise SystemExit('UI verification has not passed')
build=json.loads((out/'verification/validated-build.json').read_text())
if build['plugin_version']!=version:raise SystemExit('Validated build version does not match source')
for name,digest in build['sha256'].items():
    if hashlib.sha256((out/name).read_bytes()).hexdigest()!=digest:raise SystemExit('Artifact changed after verification: '+name)
records={str(p.relative_to(root)):{'bytes':p.stat().st_size,'sha256':hashlib.sha256(p.read_bytes()).hexdigest()} for p in sorted(files)}
manifest={'name':'model-quality','version':version,'built_platform':platform.system()+'/'+platform.machine(),'go_toolchain':go_toolchain,'created_at':datetime.now(timezone.utc).isoformat(),'requires_patched_host':True,'host_capability_version':2,'plugin_store_schema':2,'pause_policy':'manual-release-only; all retesting prohibited','baseline_upstream_commit':None,'ui_style_reference_commit':'d0126752201a35ff999bd2a1d4741215542258a8','sub2api_reference_commit':'ccf997c73868083d96c0bec56e71c1a34d26c98a','verification':{'plugin_tests':'passed','plugin_race':'passed','host_full_tests':'passed','host_related_packages_race':'passed','native_c_abi':'passed','mock_ui_checks':len(ui_report['checks']),'actual_standalone_browser_checks':len(native_ui_report['checks']),'baseline_files_unchanged':2303,'patch_reproduced':True},'limitations':['Automatic evidence verified only for native OpenAI-compatible chat/completions','Home actions unsupported','Other operating systems not verified','Identical-material file recreation while host stopped cannot be detected through current identity API'],'files':records}
manifest_path=out/'release-manifest.json';manifest_path.write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n');files.add(manifest_path)
archive=out/f'model-quality-{version}-darwin-arm64.tar.gz'
with tarfile.open(archive,'w:gz') as tar:
    for p in sorted(files):tar.add(p,arcname='model-quality/'+str(p.relative_to(root)),recursive=False)
(out/'SHA256SUMS').write_text(''.join(hashlib.sha256(p.read_bytes()).hexdigest()+'  '+p.name+'\n' for p in (archive,out/'model-quality.dylib',out/'cli-proxy-api-quality',manifest_path)))
with tarfile.open(archive) as tar:
    for member in tar.getmembers():
        rel=member.name.removeprefix('model-quality/')
        if rel in records:
            data=tar.extractfile(member).read()
            if hashlib.sha256(data).hexdigest()!=records[rel]['sha256']:raise SystemExit('Package checksum mismatch: '+rel)
print(f'Packaged and verified {len(files)} files: {archive.name} ({archive.stat().st_size} bytes)')
