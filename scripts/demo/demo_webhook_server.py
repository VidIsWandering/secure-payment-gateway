#!/usr/bin/env python3

from http.server import BaseHTTPRequestHandler, HTTPServer
import json

class WebhookHandler(BaseHTTPRequestHandler):
    def do_POST(self):
        content_length = int(self.headers['Content-Length'])
        post_data = self.rfile.read(content_length)
        
        print("\n" + "="*50)
        print("🔔 NHẬN ĐƯỢC WEBHOOK TỪ PAYMENT GATEWAY")
        print("="*50)
        print(f"Headers:\n{self.headers}")
        
        try:
            payload = json.loads(post_data.decode('utf-8'))
            print(f"Body (JSON):\n{json.dumps(payload, indent=2)}")
        except json.JSONDecodeError:
            print(f"Body (Raw): {post_data.decode('utf-8')}")
            
        print("="*50 + "\n")

        self.send_response(200)
        self.send_header('Content-type', 'application/json')
        self.end_headers()
        self.wfile.write(json.dumps({"status": "received"}).encode())

def run(server_class=HTTPServer, handler_class=WebhookHandler, port=9000):
    server_address = ('', port)
    httpd = server_class(server_address, handler_class)
    print(f"Bắt đầu lắng nghe Webhook tại http://localhost:{port}/webhook ...")
    try:
        httpd.serve_forever()
    except KeyboardInterrupt:
        pass
    httpd.server_close()
    print("Đã dừng server.")

if __name__ == '__main__':
    run()
