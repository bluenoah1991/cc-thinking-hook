#!/usr/bin/env python3
import json, http.server, socketserver, urllib.request, os
from urllib.request import Request


class ProxyHandler(http.server.BaseHTTPRequestHandler):

    def log_message(self, format, *args):
        if "200" in format % args:
            print(format % args)

    def _inject_ultrathink(self, data):
        if "messages" not in data or not data["messages"]:
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

        if isinstance(content, str):
            last_msg["content"] = [{"type": "text", "text": content}, ultrathink]
        elif isinstance(content, list):
            content.append(ultrathink)

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

            with urllib.request.urlopen(req) as resp:
                self.send_response(resp.status)
                [
                    self.send_header(h, v)
                    for h, v in resp.headers.items()
                    if h.lower() not in ["connection", "transfer-encoding"]
                ]
                self.end_headers()
                self.wfile.write(resp.read())
        except:
            print("Proxy error")


def get_backend_url():
    while True:
        url = input("Backend API URL: ").strip()
        if url:
            return url.rstrip("/")
        print("Error: Backend URL cannot be empty.")


def main():
    backend_url = get_backend_url()
    PORT = 5280

    try:
        with open("ultrathink.txt", "r", encoding="utf-8") as f:
            ultrathink_prompt = f.read()
    except:
        print("Warning: Could not load ultrathink.txt")
        ultrathink_prompt = ""

    def make_handler(url, prompt):
        class CustomProxyHandler(ProxyHandler):
            backend_url = url
            ultrathink_prompt = prompt

        return CustomProxyHandler

    handler_class = make_handler(backend_url, ultrathink_prompt)

    with socketserver.TCPServer(("", PORT), handler_class) as httpd:
        print()
        print("=== Claude UltraThink Proxy Started ===")
        print()
        print(f"Proxy: http://localhost:{PORT}")
        print(f"Backend: {backend_url}")
        print()
        print(f"export ANTHROPIC_BASE_URL=http://localhost:{PORT}")
        print()
        print("Press Ctrl+C to stop")
        print()
        try:
            httpd.serve_forever()
        except KeyboardInterrupt:
            print("\nShutting down...")


if __name__ == "__main__":
    main()
