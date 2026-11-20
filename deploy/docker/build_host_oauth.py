"""Build and package the Linux/amd64 host-OAuth edition inside HostOAuth.Dockerfile."""
import hashlib
import json
from pathlib import Path
import re
import subprocess
import zipfile

TAG = 'credential_ops_host_oauth'
MAIN = './cmd/model-quality'
OUT = Path('/out')


def output(*args):
    return subprocess.check_output(args, text=True).strip()


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    env = json.loads(output('go', 'env', '-json', 'GOOS', 'GOARCH', 'CGO_ENABLED', 'GOAMD64'))
    if env != {'GOOS': 'linux', 'GOARCH': 'amd64', 'CGO_ENABLED': '1', 'GOAMD64': 'v1'}:
        raise ValueError('the host-OAuth package requires Linux/amd64 v1 and CGO')
    flags = ['-mod=readonly', '-tags', TAG]
    deps = output('go', 'list', *flags, '-deps', MAIN).splitlines()
    if any(name.startswith(('github.com/cliproxyapi-plugins/model-quality/login',
                            'github.com/cli-proxy-api/model-quality-login-runtime')) for name in deps):
        raise ValueError('the host-OAuth entry point links the login runtime')
    subprocess.run(['go', 'test', *flags, './...'], check=True)
    subprocess.run(['go', 'vet', *flags, './...'], check=True)
    OUT.mkdir()
    library = OUT / 'model-quality.so'
    subprocess.run(['go', 'build', *flags, '-buildmode=c-shared', '-trimpath', '-buildvcs=false',
                    '-ldflags=-s -w', '-o', str(library), MAIN], check=True)
    library.with_suffix('.h').unlink()
    version = re.search(r'^const Version = "([0-9.]+)"$', Path('quality/types.go').read_text(), re.M)[1]
    archive = OUT / f'credential-ops-v{version}-linux-amd64.zip'
    # A fixed timestamp and Unix mode keep the ZIP independent of build time.
    entry = zipfile.ZipInfo(library.name)
    entry.create_system = 3
    entry.external_attr = 0o100755 << 16
    with zipfile.ZipFile(archive, 'w') as package:
        package.writestr(entry, library.read_bytes(), compress_type=zipfile.ZIP_DEFLATED, compresslevel=6)
    checksums = {file.name: digest(file) for file in (library, archive)}
    (OUT / 'SHA256SUMS').write_text(''.join(f'{sha}  {name}\n' for name, sha in checksums.items()))
    manifest = {
        'id': 'model-quality', 'plugin_version': version, 'host_version': '8.0.76',
        'platform': 'linux', 'arch': 'amd64', 'goamd64': 'v1', 'build_tag': TAG,
        'go_version': output('go', 'version'), 'build_libc': output('getconf', 'GNU_LIBC_VERSION'),
        'login_mode': 'host_oauth', 'embedded_login_runtime': False, 'checksums': checksums,
    }
    (OUT / 'build-manifest.json').write_text(json.dumps(manifest, sort_keys=True, indent=2) + '\n')


if __name__ == '__main__':
    main()
