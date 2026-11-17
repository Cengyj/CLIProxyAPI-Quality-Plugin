#!/usr/bin/env python3
"""Apply the explicitly approved, backed-up upgrade of this local CPA instance."""
from __future__ import annotations

import argparse
from datetime import datetime
import hashlib
import json
import os
from pathlib import Path
import shutil
import socket
import subprocess
import sys
from zoneinfo import ZoneInfo


ROOT = Path(__file__).resolve().parents[1]
SITE = ROOT / "artifacts/local-cpa"
DIST = ROOT / "artifacts/official-plugin"
EVIDENCE = ROOT / "artifacts/login-oneclick-0.14.0/deployment"
CONTROL = ROOT / "scripts/local_cpa.sh"
CHECKER = ROOT / "scripts/check_local_official.py"


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def digest(path):
    h = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def record(path, value):
    with path.open("x", encoding="utf-8") as output:
        json.dump(value, output, ensure_ascii=False, indent=2)
        output.write("\n")
        output.flush()
        os.fsync(output.fileno())
    path.chmod(0o600)


def run(command):
    result = subprocess.run([str(x) for x in command], cwd=ROOT, capture_output=True, text=True)
    if result.returncode:
        with (EVIDENCE / "command-errors.log").open("a", encoding="utf-8") as output:
            output.write(Path(str(command[0])).name + "\n" + result.stdout + result.stderr + "\n")
        raise RuntimeError("deployment command failed: " + Path(str(command[0])).name)
    return result.stdout


def install(source, destination, mode=0o700):
    require(source.is_file() and not source.is_symlink(), "installation source must be a regular file")
    temporary = destination.with_name(destination.name + ".official-upgrade-new")
    require(not temporary.exists(), "staged installation path already exists")
    try:
        with source.open("rb") as incoming, temporary.open("xb") as outgoing:
            shutil.copyfileobj(incoming, outgoing)
            outgoing.flush()
            os.fsync(outgoing.fileno())
        temporary.chmod(mode)
        require(digest(source) == digest(temporary), "staged binary checksum mismatch")
        os.replace(temporary, destination)
    finally:
        if temporary.exists():
            temporary.unlink()


def confirm_stopped():
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as probe:
        probe.settimeout(2)
        require(probe.connect_ex(("127.0.0.1", 8317)) != 0, "port 8317 is still listening; refusing to touch running state")


def compare(before, after):
    paths = [
        ("account_settings", "complete_sha256"),
        ("accounts", "credentials_collection_sha256"),
        ("accounts", "stable_configuration_collection_sha256"),
        ("jobs", "legacy_collection_sha256"),
        ("physical_auth_files", "mapping_sha256"),
        ("quality", "rules_sha256"),
        ("quality", "actions_sha256"),
        ("quality", "model_runs_count"),
    ]
    checks = {}
    for section, key in paths:
        name = section + "." + key
        checks[name] = before[section][key] == after[section][key]
        require(checks[name], "preservation check failed: " + name)
    require(before["jobs"]["items"].keys() == after["jobs"]["items"].keys(), "job identity set changed")
    for key, job in before["jobs"]["items"].items():
        require(job["logs_sha256"] == after["jobs"]["items"][key]["logs_sha256"], "historical job log changed")
    require(before["accounts"]["items"].keys() == after["accounts"]["items"].keys(), "account identity set changed")
    for key, account in before["accounts"]["items"].items():
        current = after["accounts"]["items"][key]
        require(account["bound"] == current["bound"], "account binding was lost")
        require(current["identity_version_format"] == ("sha256" if account["bound"] else "empty"), "converted account binding is invalid")
    checks["all_historical_logs_preserved"] = True
    checks["account_bindings_preserved"] = True
    return checks


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--converter", required=True, type=Path)
    args = parser.parse_args()
    os.umask(0o077)
    converter = args.converter.resolve()
    require(converter.is_file(), "missing verified local conversion helper")
    EVIDENCE.mkdir(parents=True, exist_ok=True)
    require(not (EVIDENCE / "deployment.json").exists(), "this upgrade already has a deployment record")
    manifest = json.loads((DIST / "build-manifest.json").read_text())
    require(manifest["plugin_version"] == "0.14.0", "unexpected plugin release")
    require(digest(DIST / "model-quality.dylib") == manifest["library_sha256"], "plugin does not match tested package")
    native_report = json.loads((ROOT / "artifacts/login-oneclick-0.14.0/official-install/report.json").read_text())
    require(native_report["status"] == "passed" and native_report["archive"]["library_sha256"] == manifest["library_sha256"], "package has no matching official installation verification")
    require(digest(DIST / "cli-proxy-api-official") == native_report["binary_sha256_after"], "official host checksum mismatch")
    require(not (SITE / "plugin-data/accounts").exists(), "new account destination already exists")
    require((SITE / "auths/.credential-login/credential-login.db").is_file(), "legacy account database is missing")
    run([sys.executable, CHECKER, "--phase", "before", "--output", EVIDENCE / "preflight.json"])
    before = json.loads((EVIDENCE / "preflight.json").read_text())
    require(before["status"] == "passed", "preflight did not pass")
    require(not before["account_settings"]["inspection_enabled"] and not before["account_settings"]["auto_relogin_enabled"], "active account automation requires a separate quiescing step")
    timestamp = datetime.now(ZoneInfo("Asia/Shanghai")).strftime("%Y%m%d-%H%M%S")
    backup = SITE / "backups" / ("official-oneclick-0.14.0-" + timestamp)
    backup.parent.mkdir(mode=0o700, exist_ok=True)
    backup.mkdir(mode=0o700)
    state = {"started_at": datetime.now(ZoneInfo("Asia/Shanghai")).isoformat(), "backup": str(backup), "plugin_version": "0.14.0", "ui_version": "9.1.0", "runtime_version": "2.0.1", "status": "preparing"}
    moved_login = False
    stopped = False
    backup_complete = False
    try:
        run(["sh", CONTROL, "stop"])
        confirm_stopped()
        stopped = True
        # Copy the small mutable data first. The large legacy helper directory
        # is moved intact into the same stopped-instance backup afterwards.
        mutable = ["config.yaml", "access-info.json", "LOGIN_CN.txt", "kv.db", "local.cpa.model-quality.plist", "launch-mode", "deployment.json", "deployment-checks.json", "bin/cli-proxy-api", "plugins/model-quality.dylib"]
        for relative in mutable:
            source = SITE / relative
            if source.is_file():
                destination = backup / relative
                destination.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(source, destination)
        shutil.copytree(SITE / "plugin-data", backup / "plugin-data", symlinks=True)
        shutil.copytree(SITE / "auths", backup / "auths", symlinks=True, ignore=lambda directory, names: [".credential-login"] if Path(directory) == SITE / "auths" else [])
        config_digest = digest(SITE / "config.yaml")
        database_digest = digest(SITE / "auths/.credential-login/credential-login.db")
        os.rename(SITE / "auths/.credential-login", backup / "auths/.credential-login")
        moved_login = True
        backup_complete = True
        state["original_account_database_sha256"] = database_digest
        state["config_sha256"] = config_digest
        conversion = run([converter, "--source", backup / "auths/.credential-login/credential-login.db", "--auth-dir", SITE / "auths", "--destination", SITE / "plugin-data/accounts/credential-login.db", "--quality-state", SITE / "auths/quality-state.db", "--quality-db", SITE / "plugin-data/quality.db"])
        conversion = json.loads(conversion)
        record(EVIDENCE / "conversion.json", conversion)
        require(conversion.get("quality_checked") is True, "stopped-instance quality ownership was not checked")
        require(conversion["accounts"] == before["accounts"]["count"] and conversion["jobs"] == before["jobs"]["count"], "conversion record counts differ from preflight")
        require(conversion["account_bindings_converted"] == before["accounts"]["bound_count"], "conversion did not preserve every account binding")
        require(digest(backup / "auths/.credential-login/credential-login.db") == database_digest, "conversion modified the legacy source database")
        install(DIST / "cli-proxy-api-official", SITE / "bin/cli-proxy-api")
        install(DIST / "model-quality.dylib", SITE / "plugins/model-quality.dylib")
        run(["sh", CONTROL, "start"])
        run([sys.executable, CHECKER, "--phase", "after", "--output", EVIDENCE / "after.json"])
        after = json.loads((EVIDENCE / "after.json").read_text())
        require(after["status"] == "passed", "new instance verification failed")
        state["preservation_checks"] = compare(before, after)
        require(digest(SITE / "config.yaml") == config_digest, "CPA configuration changed during upgrade")
        state.update(status="passed", account_count=after["accounts"]["count"], bound_account_count=after["accounts"]["bound_count"], job_count=after["jobs"]["count"], official_host_sha256=after["host_binary_sha256"], plugin_sha256=manifest["library_sha256"], real_login_performed=False, model_calls_made=0, credential_contents_unchanged=True)
        deployment = json.loads((SITE / "deployment.json").read_text())
        deployment.update(plugin_version="0.14.0", ui_version="9.1.0", host_mode="official_unmodified", plugin_sha256=manifest["library_sha256"], official_oneclick_update=state)
        temporary = SITE / "deployment.official-new.json"
        record(temporary, deployment)
        os.replace(temporary, SITE / "deployment.json")
    except BaseException as error:
        state.update(status="failed", error=str(error))
        if stopped:
            try:
                run(["sh", CONTROL, "stop"])
                confirm_stopped()
                if backup_complete:
                    failed = backup / "failed-instance"
                    failed.mkdir(mode=0o700)
                    for relative in ["config.yaml", "kv.db", "deployment.json", "deployment-checks.json"]:
                        source = SITE / relative
                        if source.is_file():
                            shutil.copy2(source, failed / relative)
                        previous = backup / relative
                        if previous.is_file():
                            install(previous, source, previous.stat().st_mode & 0o777)
                    install(backup / "bin/cli-proxy-api", SITE / "bin/cli-proxy-api")
                    install(backup / "plugins/model-quality.dylib", SITE / "plugins/model-quality.dylib")
                    os.rename(SITE / "plugin-data", backup / "failed-plugin-data")
                    shutil.copytree(backup / "plugin-data", SITE / "plugin-data", symlinks=True)
                if moved_login:
                    os.rename(backup / "auths/.credential-login", SITE / "auths/.credential-login")
                run(["sh", CONTROL, "start"])
                state["rollback"] = "previous host and plugin restored; credentials left intact"
            except BaseException as rollback_error:
                state["rollback_error"] = str(rollback_error)
        raise
    finally:
        state["finished_at"] = datetime.now(ZoneInfo("Asia/Shanghai")).isoformat()
        record(EVIDENCE / "deployment.json", state)
    print(json.dumps({key: state[key] for key in ["status", "backup", "account_count", "bound_account_count", "job_count", "plugin_version", "ui_version"]}, ensure_ascii=False))


if __name__ == "__main__":
    try:
        main()
    except Exception as error:
        print("Local upgrade stopped: " + str(error), file=sys.stderr)
        sys.exit(1)
