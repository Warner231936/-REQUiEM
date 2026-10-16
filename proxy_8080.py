import http.server
import urllib.request
import urllib.error
import threading
import socketserver

TARGET_HOST = "127.0.0.1"
TARGET_PORT = 15153
LISTEN_PORTS = [8080, 3000, 8081]

class DualStackServer(socketserver.ThreadingTCPServer):
    address_family = socketserver.socket.AF_INET6
    allow_reuse_address = True

class ReverseProxy(http.server.BaseHTTPRequestHandler):
    protocol_version = 'HTTP/1.1'

    def _proxy(self):
        target_url = f"http://{TARGET_HOST}:{TARGET_PORT}{self.path}"
        content_length = int(self.headers.get('Content-Length', 0))
        body = self.rfile.read(content_length) if content_length > 0 else None

        headers = {}
        for key, val in self.headers.items():
            if key.lower() not in ('host', 'content-length', 'connection', 'transfer-encoding'):
                headers[key] = val

        req = urllib.request.Request(target_url, data=body, method=self.command)
        for k, v in headers.items():
            req.add_header(k, v)
        req.add_header('Host', self.headers.get('Host', ''))

        try:
            resp = urllib.request.urlopen(req, timeout=30)
            data = resp.read()

            self.send_response(resp.status)
            for key, val in resp.getheaders():
                if key.lower() not in ('transfer-encoding', 'connection', 'content-encoding'):
                    self.send_header(key, val)
            self.send_header('Content-Length', str(len(data)))
            self.end_headers()
            self.wfile.write(data)
        except urllib.error.HTTPError as e:
            data = e.read()
            self.send_response(e.code)
            for key, val in e.headers.items():
                if key.lower() not in ('transfer-encoding', 'connection', 'content-encoding'):
                    self.send_header(key, val)
            self.send_header('Content-Length', str(len(data)))
            self.end_headers()
            self.wfile.write(data)
        except Exception as e:
            msg = str(e).encode()
            self.send_response(502)
            self.send_header('Content-Type', 'text/plain')
            self.send_header('Content-Length', str(len(msg)))
            self.end_headers()
            self.wfile.write(msg)

    do_GET = _proxy
    do_POST = _proxy
    do_PUT = _proxy
    do_DELETE = _proxy
    do_PATCH = _proxy
    do_HEAD = _proxy
    do_OPTIONS = _proxy

    def log_message(self, format, *args):
        pass

def start_server(port):
    server = http.server.HTTPServer(('::', port), ReverseProxy)
    server.socket.setsockopt(__import__('socket').SOL_SOCKET, __import__('socket').SO_REUSEADDR, 1)
    print(f"Proxy listening on [::]:{port} -> {TARGET_HOST}:{TARGET_PORT}")
    server.serve_forever()

if __name__ == '__main__':
    import socket
    for port in LISTEN_PORTS:
        t = threading.Thread(target=start_server, args=(port,), daemon=True)
        t.start()
    print("All proxy servers started. Listening on ports:", LISTEN_PORTS)
    import time
    while True:
        time.sleep(60)
