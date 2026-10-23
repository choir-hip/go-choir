#!/usr/bin/env python3
"""guest_diag_probe.py against a fake guest. No network beyond loopback.

  python3 -I scripts/guest_diag_probe_test.py

Ways the driver can fail, written first: --dry-run sends something; a good
guest fails a check; a guest with fallback DNS or a public-resolver unit
passes; a missing route (404, a guest older than the deploy) passes.
"""
import contextlib
import http.server
import importlib.util
import io
import json
import os
import threading
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("guest_diag_probe", os.path.join(HERE, "guest_diag_probe.py"))
gdp = importlib.util.module_from_spec(spec)
spec.loader.exec_module(gdp)

TAP = "127.0.0.1"  # guest-ip 127.0.0.2, tap host 127.0.0.1

GOOD_DNS = {
    "resolv_conf": {"nameservers": ["127.0.0.53"]},
    "network_unit": {"dns": [TAP]},
    "resolved": {"nameservers": [TAP], "resolvectl": {"available": True, "dns_servers": [TAP],
                                                      "current_dns_server": [TAP],
                                                      "fallback_dns_servers": ["1.1.1.1"]}},
    "system_lookup": {"ok": True, "addrs": ["93.184.216.34"]},
    "direct_udp53": {"reachable": False},
    "caller_ip": TAP,
}
GOOD_CAP = {"gateway": "http://x:8084", "probes": [
    {"name": "search_streamed", "status": 413, "body_prefix": "request body too large", "headers": {}},
    {"name": "model_declared", "status": 413, "body_prefix": "request body too large", "body_bytes_sent": 0, "headers": {}},
]}


def serve(responses):
    hits = []

    class H(http.server.BaseHTTPRequestHandler):
        def do_GET(self):
            hits.append(self.path)
            status, body = responses.get(self.path, (404, {}))
            data = json.dumps(body).encode()
            self.send_response(status)
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)

        def log_message(self, *a):
            pass

    srv = http.server.HTTPServer(("127.0.0.1", 0), H)
    threading.Thread(target=srv.serve_forever, daemon=True).start()
    return srv, hits


def run(argv):
    out, err = io.StringIO(), io.StringIO()
    with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
        code = gdp.main(argv)
    return code, out.getvalue()


class DriverTest(unittest.TestCase):
    def go(self, responses, *extra):
        srv, hits = serve(responses)
        self.addCleanup(srv.server_close)
        self.addCleanup(srv.shutdown)
        return run(["--guest-ip", "127.0.0.2", "--connect-host", "127.0.0.1", "--guest-port", str(srv.server_address[1]), *extra]) + (hits,)

    def test_dry_run_sends_nothing(self):
        srv, hits = serve({})
        self.addCleanup(srv.server_close)
        self.addCleanup(srv.shutdown)
        code, out = run(["--guest-ip", "127.0.0.2", "--connect-host", "127.0.0.1", "--guest-port", str(srv.server_address[1]), "--dry-run"])
        self.assertEqual(code, 0)
        self.assertEqual(hits, [])
        self.assertIn("/internal/diag/dns", out)
        self.assertIn("/internal/diag/gateway-body-cap", out)

    def test_good_guest_passes(self):
        code, out, _ = self.go({"/internal/diag/dns": (200, GOOD_DNS), "/internal/diag/gateway-body-cap": (200, GOOD_CAP)},
                               "--expect-udp53", "closed")
        self.assertEqual(code, 0, out)

    def test_udp_expectation_is_checked(self):
        code, out, _ = self.go({"/internal/diag/dns": (200, GOOD_DNS)}, "--probe", "dns", "--expect-udp53", "open")
        self.assertEqual(code, 1, out)

    def test_fallback_in_use_fails(self):
        bad = json.loads(json.dumps(GOOD_DNS))
        bad["resolved"]["nameservers"] = [TAP, "1.1.1.1"]
        bad["resolved"]["resolvectl"]["current_dns_server"] = ["1.1.1.1"]
        code, out, _ = self.go({"/internal/diag/dns": (200, bad)}, "--probe", "dns")
        self.assertEqual(code, 1, out)

    def test_unit_without_dns_fails(self):
        bad = json.loads(json.dumps(GOOD_DNS))
        bad["network_unit"] = {"dns": []}
        code, out, _ = self.go({"/internal/diag/dns": (200, bad)}, "--probe", "dns")
        self.assertEqual(code, 1, out)

    def test_missing_route_fails(self):
        code, out, _ = self.go({}, "--probe", "dns")
        self.assertEqual(code, 1, out)
        self.assertIn("404", out)

    def test_body_not_refused_fails(self):
        bad = json.loads(json.dumps(GOOD_CAP))
        bad["probes"][1]["status"] = 400
        code, out, _ = self.go({"/internal/diag/gateway-body-cap": (200, bad)}, "--probe", "body-cap")
        self.assertEqual(code, 1, out)

    def test_rate_limited_body_probe_fails(self):
        code, out, _ = self.go({"/internal/diag/gateway-body-cap": (429, {"error": "rate limited"})}, "--probe", "body-cap")
        self.assertEqual(code, 1, out)


if __name__ == "__main__":
    unittest.main()
