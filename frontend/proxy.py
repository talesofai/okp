import http.server
import urllib.request
import json
import os
import sys

OKP_BASE = 'https://okp.neta.art'
OKP_TOKEN = os.environ.get('OKP_API_TOKEN', 'tok')

class ProxyHandler(http.server.SimpleHTTPRequestHandler):
    def do_GET(self):
        # API proxy: forward to okp with auth header
        if self.path.startswith('/api/'):
            url = f"{OKP_BASE}{self.path}"
            req = urllib.request.Request(url)
            req.add_header('Authorization', f'Bearer {OKP_TOKEN}')
            req.add_header('Content-Type', 'application/json')
            try:
                with urllib.request.urlopen(req, timeout=30) as resp:
                    body = resp.read()
                    self.send_response(resp.status)
                    self.send_header('Content-Type', resp.headers.get('Content-Type', 'application/json'))
                    self.send_header('Access-Control-Allow-Origin', '*')
                    self.end_headers()
                    self.wfile.write(body)
            except Exception as e:
                self.send_response(502)
                self.send_header('Content-Type', 'application/json')
                self.end_headers()
                self.wfile.write(json.dumps({'error': str(e)}).encode())
            return

        # SPA fallback: all other routes → index.html
        if self.path == '/':
            self.path = '/index.html'
        elif not os.path.exists(self.translate_path(self.path)):
            self.path = '/index.html'

        return super().do_GET()

if __name__ == '__main__':
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 3000
    http.server.HTTPServer(('0.0.0.0', port), ProxyHandler).serve_forever()
