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
        if len(args) >= 2 and isinstance(args[1], int) and args[1] >= 400:
            print(f"[✗] HTTP {args[1]}")

    def _save_diagnostic_request(self, data):
        if not hasattr(self, "diagnostic_mode") or not self.diagnostic_mode:
            return

        try:
            diag_dir = "diagnostic"
            if not os.path.exists(diag_dir):
                os.makedirs(diag_dir)

            timestamp = datetime.datetime.now().strftime("%Y%m%d_%H%M%S_%f")[:-3]
            filename = f"request_{timestamp}.json"
            filepath = os.path.join(diag_dir, filename)

            with open(filepath, "w", encoding="utf-8") as f:
                json.dump(data, f, indent=2, ensure_ascii=False)

            print(f"[📋] Diagnostic saved: {filename}")

        except Exception as e:
            print(f"[✗] Failed to save diagnostic: {e}")

    def _inject_ultrathink(self, data):
        if "messages" not in data or not data["messages"]:
            return

        if "thinking" not in data:
            return

        last_msg = data["messages"][-1]
        if last_msg.get("role") != "user":
            return

        if not hasattr(self, "ultrathink_prompt") or not self.ultrathink_prompt:
            return

        content = last_msg.get("content")

        if isinstance(content, list):
            has_non_text_blocks = False
            for block in content:
                if isinstance(block, dict) and block.get("type") != "text":
                    has_non_text_blocks = True
                    break

            if has_non_text_blocks:
                return

        ultrathink = {"type": "text", "text": self.ultrathink_prompt}

        injected = False
        if isinstance(content, str):
            last_msg["content"] = [{"type": "text", "text": content}, ultrathink]
            preview = content[:20] + "..." if len(content) > 20 else content
            print(f"[✓] Injected prompt: {preview}")
            injected = True
        elif isinstance(content, list):
            content.append(ultrathink)
            text_blocks = [
                block.get("text", "")
                for block in content
                if isinstance(block, dict) and block.get("type") == "text"
            ]
            first_text = text_blocks[0] if text_blocks else ""
            preview = first_text[:20] + "..." if len(first_text) > 20 else first_text
            print(f"[✓] Injected prompt: {preview}")
            injected = True

        if injected:
            self._save_diagnostic_request(data)

    def do_POST(self):
        try:
            data = json.loads(
                self.rfile.read(int(self.headers["Content-Length"])).decode("utf-8")
            )

            self._inject_ultrathink(data)
            modified = json.dumps(data).encode("utf-8")

            req = Request(self.backend_url, data=modified, method="POST")
            for h, v in self.headers.items():
                if h.lower() not in ["host", "content-length"]:
                    req.add_header(h, v)
            req.add_header("Content-Length", str(len(modified)))

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
    while True:
        url = input("Backend API URL: ").strip()
        if url:
            return url.rstrip("/")

        print("[✗] Backend URL cannot be empty.")


def main():
    parser = argparse.ArgumentParser(description="Claude UltraThink Proxy")
    parser.add_argument(
        "--diagnostic",
        "-d",
        action="store_true",
        help="Enable diagnostic mode to save request bodies",
    )
    parser.add_argument(
        "--port",
        "-p",
        type=int,
        default=5280,
        help="Port to run the proxy on (default: 5280)",
    )

    args = parser.parse_args()

    backend_url = get_backend_url()
    port = args.port

    try:
        with open("ultrathink.txt", "r", encoding="utf-8") as f:
            ultrathink_prompt = f.read()
    except:
        print("[✗] Could not load ultrathink.txt")
        ultrathink_prompt = ""

    def make_handler(url, prompt, diagnostic):
        class CustomProxyHandler(ProxyHandler):
            backend_url = url
            ultrathink_prompt = prompt
            diagnostic_mode = diagnostic

        return CustomProxyHandler

    handler_class = make_handler(backend_url, ultrathink_prompt, args.diagnostic)

    with socketserver.TCPServer(("", port), handler_class) as httpd:
        print("\n🚀 Claude UltraThink Proxy")
        print(f"   Local:   http://localhost:{port}")
        print(f"   Backend: {backend_url}")
        if args.diagnostic:
            print(f"   📋 Diagnostic: enabled (saving to 'diagnostic/' directory)")
        print(f"\n   export ANTHROPIC_BASE_URL=http://localhost:{port}")
        print("\n   Press Ctrl+C to stop\n")

        try:
            httpd.serve_forever()
        except KeyboardInterrupt:
            print("\n👋 Shutting down...")


if __name__ == "__main__":
    main()
