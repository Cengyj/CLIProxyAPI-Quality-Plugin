#!/usr/bin/env python3
"""Run an isolated, loopback-only native host with a fake OpenAI upstream."""
from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import re
import secrets
import shutil
import signal
import socket
import subprocess
import sys
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

ROOT = Path(__file__).resolve().parents[1]
DEMO = Path(os.environ.get("MQ_DEMO_DIR", str(ROOT / "artifacts/demo"))).resolve()
STATE = DEMO / "state.json"
ADMIN_KEY = "mq-demo-admin-not-a-live-secret"
BUSINESS_KEY = "mq-demo-client-not-a-live-key"


def read_state():
    try:
        return json.loads(STATE.read_text())
    except (OSError, ValueError):
        return {}


def write_state(state):
    temp = STATE.with_suffix(".tmp")
    temp.write_text(json.dumps(state, indent=2) + "\n")
    temp.replace(STATE)


def request(url, data=None, key=None):
    headers = {"Content-Type": "application/json"}
    if key:
        headers["Authorization"] = "Bearer " + key
    body = json.dumps(data).encode() if data is not None else None
    with urlopen(Request(url, body, headers=headers), timeout=2) as response:
        return json.load(response)


class FakeUpstream(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    mode = "correct"
    lock = threading.Lock()
    release = threading.Event()
    stop = threading.Event()
    control_token = ""
    calls = []

    def log_message(self, *args):
        pass

    def json_response(self, status, value):
        body = json.dumps(value).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path == "/stats":
            with self.lock:
                self.json_response(200, {"mode": self.mode, "calls": self.calls.copy()})
            return
        self.json_response(404, {"error": "fake route not found"})

    def do_POST(self):
        try:
            size = int(self.headers.get("Content-Length", "0"))
            if size > 2 << 20:
                self.json_response(413, {"error": "fake body too large"})
                return
            payload = json.loads(self.rfile.read(size) or b"{}")
        except (ValueError, OSError):
            self.json_response(400, {"error": "invalid fake request"})
            return
        if self.path == "/control":
            if self.headers.get("Authorization") != "Bearer " + self.control_token:
                self.json_response(403, {"error": "invalid local control token"})
                return
            if payload.get("stop"):
                self.json_response(200, {"stopping": True})
                self.stop.set()
                self.release.set()
                return
            mode = payload.get("mode", "correct")
            if mode not in {"correct", "incorrect", "empty", "truncated", "401", "429", "hold"}:
                self.json_response(400, {"error": "invalid fake mode"})
                return
            with self.lock:
                type(self).mode = mode
            if mode == "hold":
                self.release.clear()
            else:
                self.release.set()
            self.json_response(200, {"mode": mode})
            return
        if self.path not in {"/v1/chat/completions", "/chat/completions"}:
            self.json_response(404, {"error": "only fake chat completions supported"})
            return
        with self.lock:
            mode = self.mode
            self.calls.append({"model": payload.get("model"), "stream": bool(payload.get("stream")), "prompt_bytes": len(json.dumps(payload.get("messages", []))), "mode": mode})
        if mode == "hold":
            self.release.wait()
            mode = self.mode
        if mode in {"401", "429"}:
            self.json_response(int(mode), {"error": {"message": "offline mock failure", "type": "mock_error"}})
            return
        messages = payload.get("messages", [])
        text = "\n".join(str(message.get("content", "")) for message in messages)
        answer = "4"
        if "SVG" in text or "svg" in text.lower() or "visual" in payload.get("model", ""):
            answer = '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 600 300"><rect width="600" height="300" fill="#f2f6eb"/><style>.wheel{transform-box:fill-box;transform-origin:center;animation:turn 3s linear infinite}@keyframes turn{to{transform:rotate(360deg)}}</style><g fill="none" stroke="#28604a" stroke-width="6"><circle class="wheel" cx="200" cy="220" r="43"/><circle class="wheel" cx="390" cy="220" r="43"/><path d="M200 220 260 130 315 220 200 220M260 130 370 130 390 220M315 220 370 130"/></g><ellipse cx="295" cy="95" rx="48" ry="32" fill="white" stroke="#6e875f" stroke-width="3"/><path d="M325 77q10-44 42-25l12 14 74 17-81 17" fill="#efc768" stroke="#6e875f" stroke-width="3"/><circle cx="365" cy="63" r="4" fill="#193c2a"/><path d="M285 120 265 174 310 205" fill="none" stroke="#dfac4a" stroke-width="8"/><text x="30" y="35" font-size="15" fill="#6e875f">OFFLINE MOCK · SVG REVIEW</text></svg>'
            if "独立 HTML" in text:
                answer = '<!doctype html><html><head><meta charset="utf-8"><title>Offline pelican</title></head><body>' + answer + '</body></html>'
        else:
            if "苹果味 桃子味 西瓜味" in text:
                answer = "21"
            sums = re.search(r"A\+B=(\d+), B\+C=(\d+), A\+C=(\d+)", text)
            if sums:
                ab, bc, ac = map(int, sums.groups())
                answer = str((ab + bc - ac) // 2)
        if mode == "incorrect":
            answer = "999"
        elif mode == "empty":
            answer = ""
        model = payload.get("model", "mock-reasoner-v1")
        usage = {"prompt_tokens": 64, "completion_tokens": max(1, len(answer) // 4), "total_tokens": 64 + max(1, len(answer) // 4)}
        finish = "length" if mode == "truncated" else "stop"
        try:
            if payload.get("stream"):
                self.send_response(200)
                self.send_header("Content-Type", "text/event-stream")
                self.send_header("Cache-Control", "no-cache")
                self.send_header("Connection", "close")
                self.end_headers()
                self.wfile.write(b": offline-heartbeat\n\n")
                midpoint = len(answer) // 2
                for part in [answer[:midpoint], answer[midpoint:]]:
                    chunk = {"id": "mock-completion", "object": "chat.completion.chunk", "model": model, "choices": [{"index": 0, "delta": {"content": part}, "finish_reason": None}]}
                    self.wfile.write(("data: " + json.dumps(chunk) + "\n\n").encode())
                    self.wfile.flush()
                chunk = {"id": "mock-completion", "object": "chat.completion.chunk", "model": model, "choices": [{"index": 0, "delta": {}, "finish_reason": finish}], "usage": usage}
                self.wfile.write(("data: " + json.dumps(chunk) + "\n\ndata: [DONE]\n\n").encode())
                self.wfile.flush()
                self.close_connection = True
            else:
                self.json_response(200, {"id": "mock-completion", "object": "chat.completion", "model": model, "choices": [{"index": 0, "message": {"role": "assistant", "content": answer}, "finish_reason": finish}], "usage": usage})
        except (BrokenPipeError, ConnectionResetError):
            pass


def unused_port():
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        return listener.getsockname()[1]


def serve():
    DEMO.mkdir(parents=True, exist_ok=True)
    session = DEMO / (time.strftime("%Y%m%d-%H%M%S", time.gmtime()) + "-" + secrets.token_hex(3))
    session.mkdir()
    auth_dir, plugin_dir, data_dir = (session / name for name in ("auths", "plugins", "plugin-data"))
    for directory in (auth_dir, plugin_dir, data_dir):
        directory.mkdir()
    extension = "dylib" if sys.platform == "darwin" else "so"
    library = ROOT / f"artifacts/model-quality.{extension}"
    binary = ROOT / "artifacts/cli-proxy-api-quality"
    if not binary.is_file() or not library.is_file():
        raise RuntimeError("Build the patched host and native plugin first: sh scripts/build.sh")
    shutil.copy2(library, plugin_dir / library.name)
    host_port = unused_port()
    upstream = ThreadingHTTPServer(("127.0.0.1", 0), FakeUpstream)
    upstream.daemon_threads = True
    upstream_port = upstream.server_address[1]
    FakeUpstream.control_token = secrets.token_hex(24)
    thread = threading.Thread(target=upstream.serve_forever, daemon=True)
    thread.start()
    config = {
        "config-version": 8,
        "server": {"host": "127.0.0.1", "port": host_port, "discovery": {"enabled": False}, "kv-store": {"path": str(session / "kv.db")}},
        "management": {"allow-remote": False, "secret-key": ADMIN_KEY, "disable-control-panel": True, "disable-auto-update-panel": True},
        "access": {"api-keys": [BUSINESS_KEY]},
        "oauth": {"auth-dir": str(auth_dir)},
        "api-keys": {"openai-compatibility": [{"name": "mq-mock", "base-url": f"http://127.0.0.1:{upstream_port}/v1", "request-retry": 0, "keys": [{"api-key": "mq-fake-upstream-a", "proxy-url": "direct"}, {"api-key": "mq-fake-upstream-b", "proxy-url": "direct"}], "models": [{"name": "mock-reasoner-v1", "alias": "mq-reasoner", "thinking": {"levels": ["none", "low", "medium", "high"]}}, {"name": "mock-visual-v1", "alias": "mq-visual"}]}]},
        "plugins": {"enabled": True, "dir": str(plugin_dir), "configs": {"model-quality": {"enabled": True, "data_dir": str(data_dir), "scheduling_enabled": False, "automatic_pause": False}}},
        "requests": {"request-retry": 0, "proxy-url": "direct"},
        "logging": {"to-file": False, "request-log": False},
    }
    config_path = session / "mock-config.yaml"
    config_path.write_text(json.dumps(config, indent=2) + "\n")
    host_log = (session / "host.log").open("wb")
    env = {name: os.environ[name] for name in ("PATH", "HOME", "USER", "LOGNAME", "TMPDIR", "LANG", "LC_ALL") if name in os.environ}
    process = subprocess.Popen([str(binary), "--config", str(config_path), "--standalone", "--local-model"], cwd=session, env=env, stdin=subprocess.DEVNULL, stdout=host_log, stderr=subprocess.STDOUT)
    state = {"status": "starting", "supervisor_pid": os.getpid(), "host_pid": process.pid, "host_url": f"http://127.0.0.1:{host_port}", "upstream_url": f"http://127.0.0.1:{upstream_port}", "page_url": f"http://127.0.0.1:{host_port}/v0/resource/plugins/model-quality/index.html", "admin_key": ADMIN_KEY, "business_key": BUSINESS_KEY, "control_token": FakeUpstream.control_token, "session_dir": str(session)}
    write_state(state)
    signal.signal(signal.SIGTERM, lambda *_: FakeUpstream.stop.set())
    signal.signal(signal.SIGINT, lambda *_: FakeUpstream.stop.set())
    try:
        deadline = time.monotonic() + 60
        while not FakeUpstream.stop.is_set() and time.monotonic() < deadline:
            if process.poll() is not None:
                raise RuntimeError("Patched host exited during startup; see " + str(session / "host.log"))
            try:
                request(state["host_url"] + "/v8/management/plugins/model-quality/targets", key=ADMIN_KEY)
                state["status"] = "ready"
                write_state(state)
                break
            except (HTTPError, URLError, TimeoutError, OSError):
                FakeUpstream.stop.wait(0.2)
        if state["status"] != "ready":
            raise RuntimeError("Patched host plugin did not become ready; see " + str(session / "host.log"))
        while not FakeUpstream.stop.wait(0.5):
            if process.poll() is not None:
                raise RuntimeError("Patched host exited unexpectedly")
    finally:
        FakeUpstream.release.set()
        if process.poll() is None:
            process.terminate()
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
        upstream.shutdown()
        upstream.server_close()
        host_log.close()
        state["status"] = "stopped"
        state["host_pid"] = None
        write_state(state)


def start():
    DEMO.mkdir(parents=True, exist_ok=True)
    previous = read_state()
    if previous.get("status") == "ready":
        try:
            request(previous["upstream_url"] + "/stats")
            print(json.dumps(previous, indent=2))
            return
        except (URLError, TimeoutError, OSError):
            pass
    with (DEMO / "supervisor.log").open("ab") as output:
        child = subprocess.Popen([sys.executable, str(Path(__file__).resolve()), "serve"], cwd=ROOT, stdin=subprocess.DEVNULL, stdout=output, stderr=subprocess.STDOUT, start_new_session=True)
    deadline = time.monotonic() + 65
    waiter = threading.Event()
    while time.monotonic() < deadline:
        state = read_state()
        if state.get("supervisor_pid") == child.pid and state.get("status") == "ready":
            print(json.dumps(state, indent=2))
            return
        if child.poll() is not None:
            raise RuntimeError("Demo startup failed; see artifacts/demo/supervisor.log and session host.log")
        waiter.wait(0.2)
    raise RuntimeError("Demo startup did not finish; inspect logs and run scripts/demo.py stop")


def stop():
    state = read_state()
    if state.get("status") in {"ready", "starting"}:
        try:
            request(state["upstream_url"] + "/control", {"stop": True}, state["control_token"])
        except (URLError, TimeoutError, OSError):
            pass
        deadline = time.monotonic() + 15
        waiter = threading.Event()
        while time.monotonic() < deadline and read_state().get("status") != "stopped":
            waiter.wait(0.1)
    print(json.dumps(read_state(), indent=2))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=("start", "serve", "stop", "status"))
    arguments = parser.parse_args()
    try:
        if arguments.command == "serve":
            serve()
        elif arguments.command == "start":
            start()
        elif arguments.command == "stop":
            stop()
        else:
            print(json.dumps(read_state(), indent=2))
    except Exception as error:
        print(str(error), file=sys.stderr)
        sys.exit(1)
