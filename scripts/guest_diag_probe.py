#!/usr/bin/env python3
"""Run the guest diagnostics for SEC-10 step 3 and gateway body caps step 11.

Run on Node B as root against a test computer whose guest started after the
deploy that carries the routes (a guest older than the deploy answers 404):

    python3 scripts/guest_diag_probe.py --guest-ip 10.200.X.2 [--probe dns|body-cap|all]
        [--expect-udp53 open|closed|any] [--json-out FILE] [--dry-run]

The image has no shell, so the owner-shell versions of these steps cannot be
run. Each probe asks one host-peer-only guest route (the host reaches it from
the tap's host address, like scripts/host_floor_probe.py and
/internal/diag/tcp-dial):

  dns       GET /internal/diag/dns. SEC-10 step 3: /etc/resolv.conf is the
            resolved stub (127.0.0.53); the runtime network unit ends with
            DNS=<tap host ip>; resolved's own server list is exactly that
            address (no fallback in use); the system resolver answers; and
            --expect-udp53 says whether a direct UDP 53 query to the fixed
            public resolver should get through (open before the SEC-10
            close, closed after).
  body-cap  GET /internal/diag/gateway-body-cap. FP7 step 11 (a) and (b): a
            chunked 1 MiB + 4 KiB search body and a model request declaring
            700 MiB with no body byte sent both answer 413 (a connection
            closed mid-upload also passes (a)). The guest sends them with its
            own token; this script never sees the token, and no provider is
            called (docs/problems/guest-proof-steps-have-no-route-2026-10-23.md).
            The route allows one run per 10 s.

--dry-run prints the URLs and the expectations and sends nothing. Exit 0 when
every check matches, 1 otherwise, 2 for a usage error. Read-only: the dns
route reads guest files and sends one DNS query; the body-cap route sends the
two refusals above. No credential is used or printed.
"""

import argparse
import ipaddress
import json
import sys
import urllib.error
import urllib.request


def fetch(url, timeout):
    try:
        with urllib.request.urlopen(url, timeout=timeout) as resp:
            return resp.status, json.load(resp)
    except urllib.error.HTTPError as exc:
        return exc.code, {"error": exc.read(512).decode("utf-8", "replace").strip()}
    except Exception as exc:  # the probe itself failed
        return None, {"error": f"probe: {exc}"}


class Report:
    def __init__(self):
        self.rows = []
        self.failed = 0

    def check(self, name, ok, detail=""):
        self.rows.append({"check": name, "match": bool(ok), "detail": detail})
        self.failed += 0 if ok else 1
        print(f"{'PASS' if ok else 'FAIL'}  {name:58s} {detail}")

    def info(self, name, detail):
        self.rows.append({"check": name, "info": True, "detail": detail})
        print(f"INFO  {name:58s} {detail}")


def judge_dns(rep, body, tap_host, expect_udp53):
    rc = body.get("resolv_conf", {})
    rep.check("resolv.conf is the resolved stub", rc.get("nameservers") == ["127.0.0.53"],
              f"nameservers={rc.get('nameservers')} {rc.get('error', '')}")
    nu = body.get("network_unit", {})
    rep.check("network unit ends with DNS=<tap host ip>", nu.get("dns") == [tap_host],
              f"DNS={nu.get('dns')} want [{tap_host}] {nu.get('error', '')}")
    rd = body.get("resolved", {})
    rep.check("resolved's server list is exactly the tap host (no fallback)", rd.get("nameservers") == [tap_host],
              f"nameservers={rd.get('nameservers')} {rd.get('error', '')}")
    rv = rd.get("resolvectl", {})
    if rv.get("available"):
        rep.check("resolvectl: link DNS servers are exactly the tap host", rv.get("dns_servers") == [tap_host],
                  f"dns_servers={rv.get('dns_servers')}")
        cur = rv.get("current_dns_server")
        rep.check("resolvectl: current DNS server is the tap host", cur == [tap_host], f"current={cur}")
        rep.info("resolvectl: configured fallback servers (listed, not in use)", f"{rv.get('fallback_dns_servers')}")
    else:
        rep.info("resolvectl", "not in the image; resolved's resolv.conf above is the evidence")
    sl = body.get("system_lookup", {})
    rep.check("system resolver answers (example.com)", sl.get("ok") is True,
              f"addrs={sl.get('addrs')} {sl.get('latency_ms')} ms {sl.get('error', '')}")
    u = body.get("direct_udp53", {})
    reach = u.get("reachable")
    if expect_udp53 == "any":
        rep.info("direct UDP 53 to the public resolver", f"reachable={reach} {u.get('error', '')}")
    else:
        rep.check(f"direct UDP 53 to the public resolver is {expect_udp53}",
                  (reach is True) == (expect_udp53 == "open"), f"reachable={reach} {u.get('error', '')}")
    rep.info("caller_ip (host peer)", f"{body.get('caller_ip')} (tap host {tap_host})")


def judge_body_cap(rep, body):
    legs = {leg.get("name"): leg for leg in body.get("probes", [])}
    s = legs.get("search_streamed", {})
    if s.get("status") is not None:
        rep.check("11(a) streamed search body over the cap answers 413", s["status"] == 413,
                  f"status={s['status']} {s.get('body_prefix', '')[:80]!r}")
    else:
        # Step 11(a): a 413, or the connection closed mid-upload.
        rep.check("11(a) streamed search body: connection closed mid-upload", bool(s.get("error")),
                  f"error={s.get('error')} sent={s.get('body_bytes_sent')}")
    m = legs.get("model_declared", {})
    rep.check("11(b) model body declaring 700 MiB answers 413 before a byte is read", m.get("status") == 413,
              f"status={m.get('status')} sent={m.get('body_bytes_sent')} {m.get('body_prefix', m.get('error', ''))[:80]!r}")
    rep.info("gateway", body.get("gateway"))
    rep.info("headers (search)", json.dumps(s.get("headers", {}), sort_keys=True)[:200])
    rep.info("headers (model)", json.dumps(m.get("headers", {}), sort_keys=True)[:200])


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--guest-ip", required=True)
    ap.add_argument("--guest-port", type=int, default=8085)
    ap.add_argument("--probe", choices=["dns", "body-cap", "all"], default="all")
    ap.add_argument("--expect-udp53", choices=["open", "closed", "any"], default="any")
    ap.add_argument("--json-out")
    ap.add_argument("--dry-run", action="store_true")
    ap.add_argument("--timeout", type=int, default=45)
    ap.add_argument("--connect-host", help=argparse.SUPPRESS)  # tests: reach a fake guest on another address
    args = ap.parse_args(argv)

    try:
        guest = ipaddress.ip_address(args.guest_ip)
    except ValueError as exc:
        print(f"usage: {exc}", file=sys.stderr)
        return 2
    tap_host = str(guest - 1)
    base = f"http://{args.connect_host or args.guest_ip}:{args.guest_port}"
    plan = []
    if args.probe in ("dns", "all"):
        plan.append(("dns", "/internal/diag/dns"))
    if args.probe in ("body-cap", "all"):
        plan.append(("body-cap", "/internal/diag/gateway-body-cap"))

    if args.dry_run:
        for name, path in plan:
            print(f"DRY   {name}: GET {base}{path}")
        print(f"DRY   tap host address {tap_host}; expect-udp53={args.expect_udp53}; nothing sent")
        return 0

    rep = Report()
    out = {"schema_version": 1, "kind": "guest_diag_probe", "guest_ip": args.guest_ip, "tap_host": tap_host,
           "probes": {}}
    for name, path in plan:
        print(f"-- {name}")
        status, body = fetch(base + path, args.timeout)
        out["probes"][name] = {"status": status, "body": body}
        if status == 404:
            rep.check(f"{name}: route present", False, "404: this guest started before the deploy carrying the route")
        elif status != 200:
            rep.check(f"{name}: route answered 200", False, f"status={status} {body.get('error', '')}")
        elif name == "dns":
            judge_dns(rep, body, tap_host, args.expect_udp53)
        else:
            judge_body_cap(rep, body)
    out["failed"] = rep.failed
    out["results"] = rep.rows
    if args.json_out:
        with open(args.json_out, "w", encoding="utf-8") as fh:
            json.dump(out, fh, indent=2)
    print(f"\n{sum(1 for r in rep.rows if r.get('match'))} matched, {rep.failed} did not", file=sys.stderr)
    return 1 if rep.failed else 0


if __name__ == "__main__":
    sys.exit(main())
