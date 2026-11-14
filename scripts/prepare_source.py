from pathlib import Path
import hashlib,json,shutil
src=Path('/Users/ceng/Downloads/CLIProxyAPI-8.0.67')
root=Path(__file__).resolve().parents[1]
dst=root/'workspaces/CLIProxyAPI'
allowed_dirs={'cmd','internal','sdk','test','examples','docs','assets','.github'}
allowed_files={'AGENTS.md','CLAUDE.md','LICENSE','README.md','README_CN.md','README_JA.md','go.mod','go.sum','config.example.yaml','Dockerfile','.dockerignore','.gitignore','docker-build.sh','docker-build.ps1','docker-compose.yml','docker-compose.cluster.yml'}
excluded={'.git','auths','node_modules','logs','data','dist','target','__pycache__'}
manifest={}
for p in sorted(src.rglob('*')):
    rel=p.relative_to(src)
    if p.is_symlink() or not p.is_file(): continue
    if rel.parts[0] not in allowed_dirs and str(rel) not in allowed_files: continue
    if any(x in excluded for x in rel.parts): continue
    if p.name.startswith('.env') or p.name=='config.yaml' or p.suffix.lower() in {'.db','.sqlite','.log','.dylib','.so','.dll','.exe'}: continue
    data=p.read_bytes()
    if b'\0' in data[:4096] and rel.parts[0]!='assets': continue
    out=dst/rel
    out.parent.mkdir(parents=True,exist_ok=True)
    if not out.exists(): shutil.copy2(p,out)
    elif out.read_bytes()!=data: raise SystemExit('Refusing to replace edited '+str(rel))
    manifest[str(rel)]=hashlib.sha256(data).hexdigest()
(root/'host-patches/baseline.json').write_text(json.dumps({'source':str(src),'archive_version':'8.0.67 (directory label; no Git metadata)','git_commit':None,'sha256':manifest},indent=2)+'\n')
print('Copied and checksummed',len(manifest),'source/build files')
