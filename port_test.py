import http.server
import threading
import time
import sys

PORTS = [80, 81, 8080, 8081, 8082, 8083, 8084, 8085, 3000, 3001, 5000, 5001, 4000, 9000, 9090, 15153, 443]

class Handler(http.server.BaseHTTPRequestHandler):
    protocol_version = 'HTTP/1.1'
    def do_GET(self):
        msg = f"PORT={self.server.server_address[1]},PATH={self.path}".encode()
        self.send_response(200)
        self.send_header('Content-Type', 'text/plain')
        self.send_header('Content-Length', str(len(msg)))
        self.end_headers()
        self.wfile.write(msg)
    do_POST = do_PUT = do_DELETE = do_PATCH = do_HEAD = do_OPTIONS = do_GET
    def log_message(self, *a): pass

results = {}

def start(port):
    try:
        srv = http.server.HTTPServer(('::', port), Handler)
        results[port] = 'OK'
        srv.serve_forever()
    except Exception as e:
        results[port] = f'FAIL: {e}'

for p in PORTS:
    t = threading.Thread(target=start, args=(p,), daemon=True)
    t.start()
    time.sleep(0.1)

time.sleep(1)
for p in sorted(results.keys()):
    print(f"Port {p}: {results[p]}", flush=True)

print("Waiting for requests...", flush=True)
while True:
    time.sleep(60)
