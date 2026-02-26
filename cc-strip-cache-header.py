#!/usr/bin/env python3

import argparse
import http.server
import json
import socketserver
import ssl
import urllib.request
from urllib.request import Request

import certifi

ZENMUX_BACKEND = "https://zenmux.ai/api/anthropic"
STRIP_HEADER = "prompt-caching-scope-2026-01-05"


class ProxyHandler(http.server.BaseHTTPRequestHandler):

    def log_message(self, format, *args):
        pass

    def do_POST(self):
        try:
            body = self.rfile.read(int(self.headers["Content-Length"]))

            url = self.backend_url.rstrip("/") + self.path
            req = Request(url, data=body, method="POST")
            for h, v in self.headers.items():
                if h.lower() in ["host", "content-length"]:
                    continue
                if h.lower() == STRIP_HEADER.lower():
                    continue
                req.add_header(h, v)
            req.add_header("Content-Length", str(len(body)))

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
            error_response = {"error": str(e)}
            self.wfile.write(json.dumps(error_response).encode())


def get_backend_url():
    url = input(f"Backend API URL [{ZENMUX_BACKEND}]: ").strip()
    if not url:
        return ZENMUX_BACKEND
    return url.rstrip("/")


def main():
    parser = argparse.ArgumentParser(description="Strip prompt-caching-scope header proxy")
    parser.add_argument(
        "--port",
        "-p",
        type=int,
        default=5281,
        help="Port to run the proxy on (default: 5281)",
    )

    args = parser.parse_args()
    backend_url = get_backend_url()
    port = args.port

    def make_handler(url):
        class CustomProxyHandler(ProxyHandler):
            backend_url = url

        return CustomProxyHandler

    handler_class = make_handler(backend_url)

    with socketserver.TCPServer(("", port), handler_class) as httpd:
        print(f"\nStrip Cache Header Proxy")
        print(f"  Local:   http://localhost:{port}")
        print(f"  Backend: {backend_url}")
        print(f"  Stripping: {STRIP_HEADER}")

        print(f"\n{'='*64}")
        print(f"  Windows CMD")
        print(f"{'='*64}")
        print(f'  set ANTHROPIC_BASE_URL=http://localhost:{port}')
        print(f'  set ANTHROPIC_AUTH_TOKEN=sk-ss-v1-xxx')
        print(f'  set CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1')
        print(f'  set API_TIMEOUT_MS=30000000')
        print(f'  set ANTHROPIC_API_KEY=')
        print(f'  set ANTHROPIC_DEFAULT_HAIKU_MODEL=anthropic/claude-haiku-4.5:amazon-bedrock')
        print(f'  set ANTHROPIC_DEFAULT_SONNET_MODEL=anthropic/claude-sonnet-4.6')
        print(f'  set ANTHROPIC_DEFAULT_OPUS_MODEL=anthropic/claude-opus-4.6:google-vertex')

        print(f"\n{'='*64}")
        print(f"  macOS / Linux")
        print(f"{'='*64}")
        print(f'  export ANTHROPIC_BASE_URL=http://localhost:{port}')
        print(f'  export ANTHROPIC_AUTH_TOKEN=sk-ss-v1-xxx')
        print(f'  export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1')
        print(f'  export API_TIMEOUT_MS=30000000')
        print(f'  export ANTHROPIC_API_KEY=')
        print(f'  export ANTHROPIC_DEFAULT_HAIKU_MODEL=anthropic/claude-haiku-4.5:amazon-bedrock')
        print(f'  export ANTHROPIC_DEFAULT_SONNET_MODEL=anthropic/claude-sonnet-4.6')
        print(f'  export ANTHROPIC_DEFAULT_OPUS_MODEL=anthropic/claude-opus-4.6:google-vertex')

        print(f"\n  Replace sk-ss-v1-xxx with your ZenMux API Key")
        print(f"  Subscription: sk-ss-v1-xxx | Pay-as-you-go: sk-ai-v1-xxx")
        print(f"  Get your key at: https://zenmux.ai/platform/subscription")
        print(f"\n  Press Ctrl+C to stop\n")

        try:
            httpd.serve_forever()
        except KeyboardInterrupt:
            print("\nShutting down...")


if __name__ == "__main__":
    main()
