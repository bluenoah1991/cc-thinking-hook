#!/usr/bin/env python3

import argparse
import datetime
import http.server
import json
import os
import socketserver
import ssl
import urllib.request
from urllib.request import Request

import certifi


class ProxyHandler(http.server.BaseHTTPRequestHandler):

    def log_message(self, format, *args):
        pass

    def _save_request(self, headers, data):
        try:
            diag_dir = "diagnostic"
            if not os.path.exists(diag_dir):
                os.makedirs(diag_dir)

            timestamp = datetime.datetime.now().strftime("%Y%m%d_%H%M%S_%f")[:-3]
            filename = f"request_{timestamp}.json"
            filepath = os.path.join(diag_dir, filename)

            request_data = {
                "headers": dict(headers),
                "body": data
            }

            with open(filepath, "w", encoding="utf-8") as f:
                json.dump(request_data, f, indent=2, ensure_ascii=False)

            print(f"[+] Saved: {filename}")

        except Exception as e:
            print(f"[-] Failed to save: {e}")

    def do_POST(self):
        try:
            content_length = int(self.headers["Content-Length"])
            raw_data = self.rfile.read(content_length)

            data = json.loads(raw_data.decode("utf-8"))
            self._save_request(self.headers, data)

            req = Request(self.backend_url, data=raw_data, method="POST")
            for h, v in self.headers.items():
                if h.lower() != "host":
                    req.add_header(h, v)

            ctx = ssl.create_default_context(cafile=certifi.where())
            with urllib.request.urlopen(req, context=ctx) as resp:
                self.send_response(resp.status)

                for h, v in resp.headers.items():
                    if h.lower() not in ["connection", "transfer-encoding"]:
                        self.send_header(h, v)

                self.end_headers()
                self.wfile.write(resp.read())

        except Exception as e:
            self.send_response(500)
            self.send_header("Content-type", "application/json")
            self.end_headers()
            print(f"[-] {e}")
            error_response = {"error": str(e)}
            self.wfile.write(json.dumps(error_response).encode())


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("-u", "--url", required=True, help="Backend API URL")
    parser.add_argument("-p", "--port", type=int, default=5280, help="Port to run the proxy on")

    args = parser.parse_args()
    backend_url = args.url.rstrip("/")
    port = args.port

    def make_handler(url):
        class CustomProxyHandler(ProxyHandler):
            backend_url = url
        return CustomProxyHandler

    handler_class = make_handler(backend_url)

    with socketserver.TCPServer(("", port), handler_class) as httpd:
        print(f"\n[*] Request Logger Proxy")
        print(f"   Local:   http://localhost:{port}")
        print(f"   Backend: {backend_url}")
        print(f"   Diagnostic directory: diagnostic/")
        print(f"\n   export ANTHROPIC_BASE_URL=http://localhost:{port}")
        print("\n   Press Ctrl+C to stop\n")

        try:
            httpd.serve_forever()
        except KeyboardInterrupt:
            print("\n[*] Shutting down...")


if __name__ == "__main__":
    main()
