#!/usr/bin/env python3
"""Probe the host egress floor (SEC-03) from inside a running guest.

Run on Node B as root after the FP3 group 2 deploy, against a test computer
whose VM started after that deploy (the DNS DNAT is added at VM start):

    python3 scripts/host_floor_probe.py --guest-ip 10.200.X.2 \
        --other-guest-ip 10.20Y.Z.2 [--json-out /tmp/host-floor-probe.json]

Each probe asks the guest's /internal/diag/tcp-dial (host-peer only; the
host reaches it from the tap's host address) to open one TCP connection, so
the verdict is the guest's view through the floor. Exit 0 when every
probe matches its expectation, 1 otherwise. Read-only: it opens TCP
connections and sends nothing on them. No credential is used or printed.

What TCP cannot show: UDP DNS (the guest uses the host resolver only once
FP2's guest line lands; until then public UDP 53 stays open) and IPv6
(checked by verify-host-hardening --expect-guest-floor).
"""

import argparse
import ipaddress
import json
import subprocess
import sys
import urllib.parse
import urllib.request


def public_address():
    out = subprocess.run(["ip", "-4", "route", "get", "1.1.1.1"], capture_output=True, text=True).stdout.split()
    return out[out.index("src") + 1] if "src" in out else None


def dial(guest_ip, guest_port, addr):
    url = f"http://{guest_ip}:{guest_port}/internal/diag/tcp-dial?" + urllib.parse.urlencode({"addr": addr})
    try:
        with urllib.request.urlopen(url, timeout=10) as resp:
            return json.load(resp)
    except Exception as exc:  # the probe itself failed, not the dial
        return {"ok": None, "error": f"probe: {exc}"}


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--guest-ip", required=True)
    ap.add_argument("--other-guest-ip", required=True, help="another live guest (the cross-tap checks)")
    ap.add_argument("--guest-port", type=int, default=8085)
    ap.add_argument("--json-out")
    args = ap.parse_args()

    guest = ipaddress.ip_address(args.guest_ip)
    other = ipaddress.ip_address(args.other_guest_ip)
    host = str(guest - 1)
    other_host = str(other - 1)
    public = public_address()

    # (name, addr, expected reachable, capability or floor item)
    probes = [
        ("own gateway", f"{host}:8084", True, "capability 2: search and models"),
        ("own vmctl", f"{host}:8083", True, "guest->vmctl"),
        ("own corpusd", f"{host}:8086", True, "Texture verification"),
        ("own maild", f"{host}:8087", True, "capability 6: Mail"),
        ("own proxy", f"{host}:8082", True, "publish path"),
        ("own goproxy", f"{host}:8088", True, "capability 5: Go modules"),
        ("own resolver over TCP", f"{host}:53", True, "SEC-10 host resolver"),
        ("internet https", "1.1.1.1:443", True, "capability 1: research reads"),
        ("internet dns over tcp", "8.8.8.8:53", True, "public TCP stays open"),
        ("host sshd on the tap address", f"{host}:22", False, "SEC-03 item 2"),
        ("host front door on the tap address", f"{host}:443", False, "SEC-03 item 2"),
        ("host http on the tap address", f"{host}:80", False, "SEC-03 item 2"),
        ("platform Dolt", f"{host}:13306", False, "SEC-03 item 2"),
        ("corpus Dolt", f"{host}:13307", False, "SEC-03 item 2"),
        ("another tap's host address", f"{other_host}:8084", False, "SEC-03 own tap only"),
        ("another guest", f"{other}:8085", False, "guest isolation"),
        ("metadata", "169.254.169.254:80", False, "SEC-03 item 3"),
        ("private range", "192.168.1.1:443", False, "SEC-03 item 3"),
        ("CGNAT", "100.64.0.1:443", False, "SEC-03 item 3"),
        ("internet smtp", "1.1.1.1:25", False, "no mail from Node B's address"),
    ]
    if public:
        probes += [
            ("host front door on the public address", f"{public}:443", False, "SEC-03 item 2"),
            ("host sshd on the public address", f"{public}:22", False, "SEC-03 item 2"),
        ]

    results, failed = [], 0
    for name, addr, want, why in probes:
        got = dial(args.guest_ip, args.guest_port, addr)
        ok = got.get("ok")
        match = ok is not None and bool(ok) == want
        failed += 0 if match else 1
        results.append({"probe": name, "addr": addr, "want_reachable": want, "reachable": ok,
                        "match": match, "why": why, "error": got.get("error", "")})
        print(f"{'PASS' if match else 'FAIL'}  {name:42s} {addr:24s} want={'open' if want else 'closed'} "
              f"got={ok} {got.get('error', '')[:80]}")
    report = {"schema_version": 1, "kind": "host_floor_probe", "guest_ip": args.guest_ip,
              "other_guest_ip": args.other_guest_ip, "public_address": public,
              "failed": failed, "results": results}
    if args.json_out:
        with open(args.json_out, "w", encoding="utf-8") as fh:
            json.dump(report, fh, indent=2)
    print(f"\n{len(results) - failed} matched, {failed} did not", file=sys.stderr)
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
