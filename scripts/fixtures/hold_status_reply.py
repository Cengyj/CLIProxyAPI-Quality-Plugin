"""Hold CPA's confirmed status reply in the disposable restart fixture."""

from http.client import HTTPConnection
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from threading import Event


class Relay(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def forward(self):
        if not self.path.startswith('/v8/management/'):
            self.send_error(404)
            return
        data = self.rfile.read(int(self.headers.get('Content-Length', 0)))
        conn = HTTPConnection('127.0.0.1', 18379, timeout=10)
        try:
            conn.request(self.command, self.path, data, {
                'Content-Type': 'application/json', 'Authorization': self.headers['Authorization'],
            })
            response = conn.getresponse()
            body = response.read()
        finally:
            conn.close()
        if self.command == 'PATCH' and self.path == '/v8/management/credentials/status' and response.status == 200:
            Path('/probe/status-applied').touch()
            Event().wait(30)
        self.send_response(response.status)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    do_GET = do_POST = do_PATCH = forward


if __name__ == '__main__':
    ThreadingHTTPServer(('127.0.0.1', 18479), Relay).serve_forever()
