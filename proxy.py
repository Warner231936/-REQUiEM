import http.server
import urllib.request
import urllib.error
import threading
import time

TARGET = "127.0.0.1:15153"
PORTS = [8080, 3000, 8081]

class Handler(http.server.BaseHTTPRequestHandler):
    protocol_version = 'HTTP/1.1'
    def _handle(self):
        length = int(self.headers.get('Content-Length', 0))
        body = self.rfile.read(length) if length > 0 else None
        req = urllib.request.Request(f"http://{TARGET}{self.path}", data=body, method=self.command)
        for h in ['Accept', 'Accept-Encoding', 'Accept-Language', 'User-Agent', 'Content-Type', 'Referer', 'Origin', 'Authorization', 'Cookie']:
            if h in self.headers:
                req.add_header(h, self.headers[h])
        req.add_header('Host', self.headers.get('Host', 'localhost:15153'))
        try:
            r = urllib.request.urlopen(req, timeout=30)
            data = r.read()
            self.send_response(r.status)
            for k, v in r.getheaders():
                if k.lower() not in ('transfer-encoding', 'connection', 'content-length'):
                    self.send_header(k, v)
            self.send_header('Content-Length', str(len(data)))
            self.end_headers()
            self.wfile.write(data)
        except urllib.error.HTTPError as e:
            data = e.read()
            self.send_response(e.code)
            for k, v in e.headers.items():
                if k.lower() not in ('transfer-encoding', 'connection', 'content-length'):
                    self.send_header(k, v)
            self.send_header('Content-Length', str(len(data)))
            self.end_headers()
            self.wfile.write(data)
        except Exception as e:
            err = str(e).encode()
            self.send_response(502)
            self.send_header('Content-Type', 'text/plain')
            self.send_header('Content-Length', str(len(err)))
            self.end_headers()
            self.wfile.write(err)
    do_GET = _handle
    do_POST = _handle
    do_PUT = _handle
    do_DELETE = _handle
    do_PATCH = _handle
    do_HEAD = _handle
    do_OPTIONS = _handle
    def log_message(self, *a):
        pass

def start(port):
    try:
        srv = http.server.HTTPServer(('0.0.0.0', port), Handler)
        print(f"Proxy on 0.0.0.0:{port} -> {TARGET}")
        srv.serve_forever()
    except Exception as e:
        print(f"Port {port} failed: {e}")

for p in PORTS:
    t = threading.Thread(target=start, args=(p,), daemon=True)
    t.start()

while True:
    time.sleep(60)
