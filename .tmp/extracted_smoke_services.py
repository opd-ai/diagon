import json
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import urlparse
from urllib.request import Request, urlopen


def parse_host_port(value):
    host, port = value.rsplit(":", 1)
    return host, int(port)


def host_port_path(value):
    parsed = urlparse(value)
    return parsed.hostname, parsed.port, parsed.path or "/"


with open("artifacts/stage-6-smoke-plan.json", "r", encoding="utf-8") as fh:
    plan = json.load(fh)

services = {item["service"]: item for item in plan["service_endpoints"]}
tunnels = {item["target_service"]: item for item in plan["tunnel_endpoints"]}

i2pd_host, i2pd_port = parse_host_port(services["i2pd"]["listen"])
paywall_host, paywall_port = parse_host_port(services["paywall"]["listen"])
store_host, store_port = parse_host_port(services["store"]["listen"])

_, _, i2pd_health_path = host_port_path(services["i2pd"]["health_url"])
_, _, paywall_health_path = host_port_path(services["paywall"]["health_url"])
_, _, store_health_path = host_port_path(services["store"]["health_url"])
_, _, paywall_smoke_path = host_port_path(plan["paywall_validation"]["url"])
_, _, store_checkout_path = host_port_path(plan["marketplace_access"]["url"])

store_tunnel_host, store_tunnel_port = parse_host_port(tunnels["store"]["listen"])
paywall_tunnel_host, paywall_tunnel_port = parse_host_port(tunnels["paywall"]["listen"])


class PaywallHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path != paywall_health_path:
            self.send_response(404)
            self.end_headers()
            return
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps({"status": "ok"}).encode("utf-8"))

    def do_POST(self):
        if self.path != paywall_smoke_path:
            self.send_response(404)
            self.end_headers()
            return
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps({"settled": True}).encode("utf-8"))

    def log_message(self, *_args):
        return


class StoreHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path != store_health_path:
            self.send_response(404)
            self.end_headers()
            return
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps({"status": "ok"}).encode("utf-8"))

    def do_POST(self):
        if self.path != store_checkout_path:
            self.send_response(404)
            self.end_headers()
            return

        req = Request(plan["paywall_validation"]["url"], method="POST", data=b"{}")
        with urlopen(req, timeout=5) as resp:
            payload = json.loads(resp.read().decode("utf-8"))

        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps({"checkout": "ok", "paywall": payload}).encode("utf-8"))

    def log_message(self, *_args):
        return


class I2PDHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path != i2pd_health_path:
            self.send_response(404)
            self.end_headers()
            return
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps({"status": "ok"}).encode("utf-8"))

    def log_message(self, *_args):
        return


def run_server(server):
    server.serve_forever()


def start_origin_servers():
    servers = {
        "i2pd": HTTPServer((i2pd_host, i2pd_port), I2PDHandler),
        "paywall": HTTPServer((paywall_host, paywall_port), PaywallHandler),
        "store": HTTPServer((store_host, store_port), StoreHandler),
    }
    for server in servers.values():
        threading.Thread(target=run_server, args=(server,), daemon=True).start()
    return servers


class ProxyHandler(BaseHTTPRequestHandler):
    target_host = ""
    target_port = 0

    def do_GET(self):
        with urlopen(f"http://{self.target_host}:{self.target_port}{self.path}", timeout=5) as resp:
            body = resp.read()
            self.send_response(resp.status)
            for key, value in resp.headers.items():
                if key.lower() != "transfer-encoding":
                    self.send_header(key, value)
            self.end_headers()
            self.wfile.write(body)

    def do_POST(self):
        body = self.rfile.read(int(self.headers.get("Content-Length", "0")))
        request = Request(f"http://{self.target_host}:{self.target_port}{self.path}", method="POST", data=body)
        with urlopen(request, timeout=5) as resp:
            payload = resp.read()
            self.send_response(resp.status)
            for key, value in resp.headers.items():
                if key.lower() != "transfer-encoding":
                    self.send_header(key, value)
            self.end_headers()
            self.wfile.write(payload)

    def log_message(self, *_args):
        return


def start_proxy_server(listen_host, listen_port, target_host, target_port):
    handler = type(
        f"Proxy_{listen_port}",
        (ProxyHandler,),
        {"target_host": target_host, "target_port": target_port},
    )
    server = HTTPServer((listen_host, listen_port), handler)
    threading.Thread(target=run_server, args=(server,), daemon=True).start()
    return server


def stop_servers(servers):
    for server in servers.values():
        server.shutdown()
        server.server_close()


origins = start_origin_servers()
proxies = {
    "store": start_proxy_server(store_tunnel_host, store_tunnel_port, store_host, store_port),
    "paywall": start_proxy_server(paywall_tunnel_host, paywall_tunnel_port, paywall_host, paywall_port),
}

for check in plan["health_checks"]:
    req = Request(check["url"], method=check["method"])
    with urlopen(req, timeout=5) as resp:
        assert resp.status == check["expected_status"]

req = Request(plan["marketplace_access"]["url"], method=plan["marketplace_access"]["method"], data=b"{}")
with urlopen(req, timeout=5) as resp:
    initial_result = {
        "status_code": resp.status,
        "response": json.loads(resp.read().decode("utf-8")),
        "wallet_mode": plan["wallet_mode"],
    }

stop_servers({**origins, **proxies})
origins = start_origin_servers()
proxies = {
    "store": start_proxy_server(store_tunnel_host, store_tunnel_port, store_host, store_port),
    "paywall": start_proxy_server(paywall_tunnel_host, paywall_tunnel_port, paywall_host, paywall_port),
}

for check in plan["graceful_restart"]["post_restart_checks"]:
    req = Request(check["url"], method=check["method"])
    with urlopen(req, timeout=5) as resp:
        assert resp.status == check["expected_status"]

req = Request(plan["marketplace_access"]["url"], method=plan["marketplace_access"]["method"], data=b"{}")
with urlopen(req, timeout=5) as resp:
    restart_result = {
        "status_code": resp.status,
        "response": json.loads(resp.read().decode("utf-8")),
    }

smoke_result = {
    "initial": initial_result,
    "after_restart": restart_result,
    "restart_validated": True,
}

with open("artifacts/stage-6-smoke.json", "w", encoding="utf-8") as fh:
    json.dump(smoke_result, fh, indent=2)

assert smoke_result["initial"]["status_code"] == 200
assert smoke_result["initial"]["response"]["paywall"]["settled"] is True
assert smoke_result["after_restart"]["status_code"] == 200
assert smoke_result["after_restart"]["response"]["paywall"]["settled"] is True

stop_servers({**origins, **proxies})
