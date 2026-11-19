#!/usr/bin/env python3
"""Record source-only provenance for the CPA Go/Sobek runtime adaptation."""
import hashlib
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent
REFERENCE = Path('/Users/ceng/Downloads/toSub2-go-port-2026.11.17')

def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

if __name__ == '__main__':
    protocol = ['internal/protocol/client.go', 'internal/protocol/flow.go', 'internal/protocol/oauth.go', 'internal/protocol/totp.go', 'cmd/tosub2/sentinel.go', 'internal/protocol/totp_reset.go', 'internal/protocol/totp_reset_continue.go', 'internal/protocol/totp_response.go', 'internal/protocol/totp_status.go', 'internal/protocol/totp_reset_initial_enroll.go', 'internal/protocol/totp_enrollment_conflict.go', 'internal/protocol/totp_reset_capture.go', 'internal/protocol/totp_reset_verify.go', 'internal/protocol/flow_mfa.go', 'cmd/tosub2/totp_factor_credentials.go', 'internal/jobs/totp_completion.go', 'internal/protocol/totp_reset_evidence.go', 'internal/protocol/totp_reset_receipt.go', 'internal/protocol/totp_receipt_projection.go']
    if REFERENCE.is_dir():
        reference_hashes = {name: digest(REFERENCE / name) for name in protocol}
    else:
        reference_hashes = json.loads((ROOT / 'SOURCE_MANIFEST.json').read_text())['protocol_reference_sha256']
        if set(reference_hashes) != set(protocol) or any(len(value) != 64 for value in reference_hashes.values()):
            raise RuntimeError('Missing frozen reference provenance')
    result = {
        'schema_version': 1,
        'runtime_version': '2.2.1',
        'engine': 'go_sobek_protocol',
        'reference_engine_manifest': 'UPSTREAM_SOURCE_MANIFEST.json',
        'protocol_reference_sha256': reference_hashes,
        'adaptation': {
            'internal/login/client.go': 'Reference protocol client with MoeForge JSON/Base64 integration.',
            'internal/login/flow.go': 'Existing-account password/TOTP web session and OAuth selection adapted from reference flow; CPA owns PKCE, token exchange and commit.',
            'internal/login/input.go': 'CPA strict IPC input, proxy/TOTP validation, payload serialization and fixed diagnostics.',
            'internal/login/security.go': 'Reference web-account security endpoints and one-dispatch reset/logout workflow with same-session reconciliation.',
            'internal/login/security_response.go': 'Reference explicit mutation receipts and factor/session binding checks using MoeForge JSON.',
            'internal/login/security_journal.go': 'CPA-owned private write-ahead security evidence and pending-seed durability; no login-session persistence.',
            'cmd/login-runtime/main.go': 'Native CPA v1 NDJSON adapter and process ownership; same binary executes Sobek workers.',
        },
        'source_sha256': {p.relative_to(ROOT).as_posix(): digest(p) for p in sorted(ROOT.rglob('*')) if p.is_file() and p.name != 'SOURCE_MANIFEST.json' and '__pycache__' not in p.parts},
    }
    (ROOT / 'SOURCE_MANIFEST.json').write_text(json.dumps(result, indent=2, sort_keys=True) + '\n')
    print('Recorded native runtime sources:', len(result['source_sha256']))
