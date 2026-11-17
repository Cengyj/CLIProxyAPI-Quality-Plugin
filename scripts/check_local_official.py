#!/usr/bin/env python3
"""Collect private, GET-only evidence before or after the approved local upgrade.

Account credentials and the management key are read only into memory. The report
contains counts, opaque record IDs, and SHA256 digests, never secret material.
The service's account GET handlers may reconcile existing job metadata; this
script never requests inspection, login, credential saving, or any mutation.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import time
from urllib.error import HTTPError, URLError
from urllib.parse import urlencode, urlsplit
from urllib.request import HTTPRedirectHandler, ProxyHandler, Request, build_opener


ROOT = Path(__file__).resolve().parents[1]
SITE = ROOT / "artifacts/local-cpa"
BASE = "http://127.0.0.1:8317"
OLD_LOGIN = "/v8/management/credential-logins"
OLD_QUALITY = "/v8/management/plugins/model-quality"
NEW_QUALITY = "/v0/management/plugins/model-quality"
NEW_LOGIN = NEW_QUALITY + "/login"
NEW_JOB_FIELDS = {"auto_commit", "enable_credential", "credential_enabled", "action", "security_status", "totp_reset", "devices_logged_out", "reauthorize", "logout_all_devices"}
NEW_ACCOUNT_FIELDS = {"last_job_auto_commit", "last_job_credential_enabled", "security_pending", "last_job_action", "last_job_security_status", "last_job_totp_reset", "last_job_devices_logged_out"}
STABLE_ACCOUNT_FIELDS = ("id", "label", "email", "masked_email", "proxy_url", "auth_id", "auto_relogin")
TERMINAL = {"succeeded", "security_uncertain", "committed", "failed", "cancelled", "discarded", "expired", "interrupted", "interaction_required", "conflict", "saved_not_adopted", "commit_unknown"}
OPAQUE_ID = re.compile(r"^[A-Za-z0-9_.:-]{1,512}$")


class CheckFailure(Exception):
    """A deliberately secret-free diagnostic."""


def require(condition, message):
    if not condition:
        raise CheckFailure(message)


def canonical_digest(value):
    raw = json.dumps(value, sort_keys=True, ensure_ascii=False, separators=(",", ":"), allow_nan=False).encode()
    return hashlib.sha256(raw).hexdigest()


def file_digest(path):
    require(path.is_file() and not path.is_symlink(), "expected a regular non-symlink file")
    h = hashlib.sha256()
    with path.open("rb") as source:
        before = os.fstat(source.fileno())
        for chunk in iter(lambda: source.read(1 << 20), b""):
            h.update(chunk)
        after = os.fstat(source.fileno())
    current = path.stat()
    fields = lambda s: (s.st_dev, s.st_ino, s.st_size, s.st_mtime_ns)
    require(fields(before) == fields(after) == fields(current), "a source file changed during hashing; repeat the check")
    return h.hexdigest()


def opaque_id(value):
    require(isinstance(value, str) and OPAQUE_ID.fullmatch(value), "unexpected record identifier")
    return value


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise CheckFailure("management redirect refused")


class Reader:
    def __init__(self, phase, key):
        self.login = OLD_LOGIN if phase == "before" else NEW_LOGIN
        self.quality = OLD_QUALITY if phase == "before" else NEW_QUALITY
        self.key = key
        self.client = build_opener(ProxyHandler({}), NoRedirect())
        self.calls = []
        self.allowed = {
            "/v8/management/plugins", "/v8/management/credentials",
            "/v8/management/credentials/quality-pauses", "/management.html",
            "/v0/resource/plugins/model-quality/index.html",
            OLD_LOGIN + "/capabilities", NEW_LOGIN + "/capabilities",
            *[self.login + suffix for suffix in ("/accounts", "/accounts/detail", "/accounts/settings", "/jobs", "/logs")],
            *[self.quality + suffix for suffix in ("/rules", "/actions", "/settings", "/runs")],
        }

    def get(self, path, query=None, expected=200, want_json=True, authenticated=True):
        require(path in self.allowed and not urlsplit(path).netloc, "GET path is not on the verification allowlist")
        query = query or {}
        require(set(query) <= {"id", "offset", "limit"}, "unexpected verification query")
        suffix = "?" + urlencode(query) if query else ""
        headers = {"Accept": "application/json" if want_json else "text/html"}
        if authenticated:
            headers["Authorization"] = "Bearer " + self.key
        request = Request(BASE + path + suffix, headers=headers, method="GET")
        try:
            response = self.client.open(request, timeout=15)
        except HTTPError as error:
            response = error
        except (URLError, TimeoutError, OSError):
            raise CheckFailure("local management GET could not complete") from None
        with response:
            self.calls.append({"method": "GET", "path": path, "status": response.status})
            require(response.status == expected, "GET " + path + " returned HTTP " + str(response.status))
            raw = response.read((16 << 20) + 1)
            require(len(raw) <= 16 << 20, "management response exceeded verification limit")
        if not want_json or expected != 200:
            return raw
        try:
            return json.loads(raw)
        except (ValueError, UnicodeError):
            raise CheckFailure("management response was not valid JSON") from None

    def page(self, suffix):
        items, offset = [], 0
        for _ in range(100):
            data = self.get(self.quality + suffix, {"limit": 200, "offset": offset})
            part, total = data.get("items"), data.get("total")
            require(isinstance(part, list) and isinstance(total, int), "unexpected quality pagination schema")
            items.extend(part)
            offset += len(part)
            if offset >= total:
                require(offset == total, "quality collection changed during pagination")
                return items
            require(part, "quality pagination did not advance")
        raise CheckFailure("quality collection exceeds verification limit")


def physical_auths():
    root = SITE / "auths"
    require(root.is_dir() and not root.is_symlink(), "local auth directory is unavailable")
    digests, legacy_count = {}, 0
    for path in sorted(root.rglob("*.json")):
        relative = path.relative_to(root)
        if ".credential-login" in relative.parts:
            legacy_count += 1
            continue
        # Filenames may include account emails; hash names as well as contents.
        key = hashlib.sha256(relative.as_posix().encode()).hexdigest()
        digests[key] = file_digest(path)
    return {"count": len(digests), "files_by_relative_name_sha256": digests,
            "mapping_sha256": canonical_digest(digests), "legacy_helper_json_count": legacy_count}


def collect(reader, phase, report, expected_plugin=None, expected_runtime=None):
    plugins = reader.get("/v8/management/plugins").get("plugins", [])
    matches = [p for p in plugins if p.get("id") == "model-quality"]
    require(len(matches) == 1, "expected exactly one registered model-quality plugin")
    plugin = matches[0]
    version = plugin.get("metadata", {}).get("version")
    report["plugin"] = {"version": version, "registered": plugin.get("registered"), "effective_enabled": plugin.get("effective_enabled")}
    require(plugin.get("registered") and plugin.get("effective_enabled"), "model-quality plugin is not active")
    require(version == (expected_plugin or ("0.12.0" if phase == "before" else "0.14.0")), "unexpected plugin version for this upgrade phase")
    binary = SITE / "bin/cli-proxy-api"
    report["host_binary_sha256"] = file_digest(binary)
    if phase == "after":
        official_hash = file_digest(ROOT / "artifacts/official-plugin/cli-proxy-api-official")
        require(report["host_binary_sha256"] == official_hash, "installed host is not the verified unchanged official binary")

    caps = reader.get(reader.login + "/capabilities")
    runtime = caps.get("runtime", {})
    report["login_capabilities"] = {"available": caps.get("available"),
        "runtime": {key: runtime.get(key) for key in ("engine", "runtime_version", "schema_version")},
        "automatic_commit": caps.get("automatic_commit"), "commit_mode": caps.get("commit_mode"),
        "manual_oauth_commit": caps.get("manual_oauth_commit"), "security_actions": caps.get("security_actions", {}),
        "password_totp_available": caps.get("methods", {}).get("password_totp", {}).get("available")}
    require(caps.get("available") and report["login_capabilities"]["password_totp_available"], "native login capability is unavailable")
    require(runtime.get("engine") == "go_sobek_protocol", "login is not using the Go/Sobek engine")
    if phase == "after":
        require(runtime.get("runtime_version") == (expected_runtime or "2.0.1") and caps.get("automatic_commit") is True and caps.get("commit_mode") == "account_auto_commit", "new account auto-commit capability is not active")
        reader.get(OLD_LOGIN + "/capabilities", expected=404, want_json=False)
    else:
        reader.get(NEW_LOGIN + "/capabilities", expected=404, want_json=False)

    jobs = reader.get(reader.login + "/jobs").get("items")
    require(isinstance(jobs, list) and len(jobs) < 200, "job collection is invalid or may be truncated at 200 records")
    job_summary = {}
    for job in jobs:
        jid = opaque_id(job.get("id"))
        require(jid not in job_summary, "duplicate job identifier")
        legacy = {k: v for k, v in job.items() if k not in NEW_JOB_FIELDS}
        logs = reader.get(reader.login + "/logs", {"id": jid})
        require(isinstance(logs.get("entries"), list) and len(logs["entries"]) < 200, "job logs may be truncated at 200 records")
        job_summary[jid] = {"status": job.get("status"), "legacy_content_sha256": canonical_digest(legacy), "logs_sha256": canonical_digest(logs), "log_count": len(logs["entries"])}
    pending = [j for j in jobs if j.get("status") not in TERMINAL or j.get("status") in {"commit_unknown", "saved_not_adopted", "security_uncertain"} or j.get("candidate_id")]
    report["jobs"] = {"count": len(jobs), "pending_or_uncertain_count": len(pending), "items": job_summary,
                      "legacy_collection_sha256": canonical_digest({key: value["legacy_content_sha256"] for key, value in job_summary.items()})}
    require(not pending, "pending candidates or uncertain commits block this upgrade verification")

    accounts = reader.get(reader.login + "/accounts").get("items")
    require(isinstance(accounts, list), "unexpected account collection schema")
    require(not any(a.get("security_pending") for a in accounts), "unresolved account security operation blocks this upgrade")
    account_summary = {}
    for account in accounts:
        aid = opaque_id(account.get("id"))
        detail = reader.get(reader.login + "/accounts/detail", {"id": aid})
        credentials, view = detail.get("credentials"), detail.get("account")
        require(isinstance(credentials, dict) and isinstance(view, dict) and view.get("id") == aid, "account detail did not match its requested identifier")
        complete = all(isinstance(credentials.get(k), str) and credentials[k] for k in ("email", "password", "totp_secret"))
        require(complete, "an account has incomplete persisted credentials")
        stable = {k: view.get(k) for k in STABLE_ACCOUNT_FIELDS}
        legacy = {k: v for k, v in view.items() if k not in NEW_ACCOUNT_FIELDS | {"identity_version"}}
        identity = view.get("identity_version", "")
        account_summary[aid] = {"credentials_sha256": canonical_digest(credentials),
            "stable_configuration_sha256": canonical_digest(stable), "legacy_view_without_identity_sha256": canonical_digest(legacy),
            "details_sha256": canonical_digest(detail), "credentials_complete": True, "bound": bool(view.get("auth_id")),
            "identity_version_format": "sha256" if re.fullmatch(r"[0-9a-f]{64}", identity) else "uuid" if re.fullmatch(r"[0-9a-f-]{36}", identity) else "empty" if not identity else "other"}
    report["accounts"] = {"count": len(accounts), "bound_count": sum(item["bound"] for item in account_summary.values()),
        "stable_configuration_fields": list(STABLE_ACCOUNT_FIELDS), "items": account_summary,
        "credentials_collection_sha256": canonical_digest({key: value["credentials_sha256"] for key, value in account_summary.items()}),
        "stable_configuration_collection_sha256": canonical_digest({key: value["stable_configuration_sha256"] for key, value in account_summary.items()})}
    settings = reader.get(reader.login + "/accounts/settings")
    report["account_settings"] = {"complete_sha256": canonical_digest(settings),
        **{key: settings.get(key) for key in ("revision", "inspection_enabled", "auto_relogin_enabled", "interval_minutes")}}

    rules, actions, runs = reader.page("/rules"), reader.page("/actions"), reader.page("/runs")
    blockers = [r for r in rules if r.get("action_id") or r.get("blocked_auth_id")]
    report["quality"] = {"rules_count": len(rules), "blocked_rules_count": len(blockers), "actions_count": len(actions),
        "active_model_runs_count": sum(r.get("status") in {"queued", "running"} for r in runs), "model_runs_count": len(runs),
        "rules_sha256": canonical_digest(rules), "actions_sha256": canonical_digest(actions)}
    require(not blockers and not actions, "retained quality ownership blocks this upgrade verification")
    require(report["quality"]["active_model_runs_count"] == 0, "active model execution blocks this upgrade verification")
    if phase == "before":
        host_pauses = reader.get("/v8/management/credentials/quality-pauses").get("actions")
        require(isinstance(host_pauses, list), "unexpected host pause authority schema")
        report["quality"]["host_pause_actions_count"] = len(host_pauses)
        require(not host_pauses, "host quality pauses block this upgrade")
    else:
        reader.get("/v8/management/credentials/quality-pauses", expected=404, want_json=False)
        quality_caps = reader.get(reader.quality + "/settings").get("capabilities", {})
        report["quality"]["capabilities"] = {k: quality_caps.get(k) for k in ("mode", "quality_pause", "manual_release")}
        require(quality_caps.get("mode") == "official_observation" and quality_caps.get("quality_pause") is False, "official plugin advertised unsupported quality authority")

    report["physical_auth_files"] = physical_auths()
    if phase == "after":
        require(report["physical_auth_files"]["legacy_helper_json_count"] == 0, "legacy helper JSON remains inside the official credential scan directory")
    files = reader.get("/v8/management/credentials").get("files")
    require(isinstance(files, list), "unexpected official credential collection schema")
    report["host_credentials"] = {"count": len(files), "disabled_count": sum(bool(f.get("disabled")) or f.get("status") == "disabled" for f in files)}
    report["static_ui"] = {}
    for path in ("/management.html", "/v0/resource/plugins/model-quality/index.html"):
        body = reader.get(path, want_json=False, authenticated=False)
        require(body, "static UI resource is empty")
        report["static_ui"][path] = {"bytes": len(body), "sha256": hashlib.sha256(body).hexdigest()}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--phase", choices=("before", "after"), required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--expected-plugin")
    parser.add_argument("--expected-runtime")
    args = parser.parse_args()
    output = args.output.resolve()
    require(not output.exists(), "refusing to overwrite an existing evidence report")
    output.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    report = {"schema_version": 1, "phase": args.phase, "status": "running", "origin": BASE,
        "observed_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "scope": "Only allowlisted local GET requests; no login, inspection, model call, save, restart or deployment. Account GET handlers may reconcile existing job metadata. Secret material is hashed in memory and never emitted.",
        "normalization": {"jobs_ignored_new_fields": sorted(NEW_JOB_FIELDS), "accounts_ignored_new_fields": sorted(NEW_ACCOUNT_FIELDS)}}
    reader = None
    try:
        access_path = SITE / "access-info.json"
        require(access_path.is_file() and not access_path.is_symlink() and access_path.stat().st_mode & 0o077 == 0, "management access file must be a private regular file")
        access = json.loads(access_path.read_text())
        key = access.get("management_key")
        require(isinstance(key, str) and key and not any(c in key for c in "\r\n\x00"), "management access key is unavailable")
        reader = Reader(args.phase, key)
        collect(reader, args.phase, report, args.expected_plugin, args.expected_runtime)
        report["status"] = "passed"
    except CheckFailure as error:
        report["status"], report["error"] = "failed", str(error)
    except Exception as error:
        report["status"], report["error"] = "failed", "verification exception: " + type(error).__name__
    report["requests"] = reader.calls if reader else []
    fd = os.open(output, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "w") as destination:
        json.dump(report, destination, indent=2, ensure_ascii=False)
        destination.write("\n")
    print(json.dumps({"status": report["status"], "phase": args.phase, "report": str(output), "error": report.get("error")}, ensure_ascii=False))
    return 0 if report["status"] == "passed" else 1


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except CheckFailure as error:
        print(json.dumps({"status": "failed", "error": str(error)}))
        raise SystemExit(1)
