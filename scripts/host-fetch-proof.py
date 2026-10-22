#!/usr/bin/env python3
"""Deployed proof for the host fetch route (SDC NET-01, station FP10).

docs/definitions/choir-launch-fp10-net01-host-fetch-2026-10-21.md, section
"FP0 proof steps", says when to run each mode. Two modes:

  api      drive capability 3 (content import, a YouTube transcript, podcast
           search and subscribe, an RSS schedule) and the refusal probes on a
           computer, through the public API. Needs CHOIR_API_KEY in the
           environment (never printed) and --computer. Writes a JSON artifact.
  journal  on Node B: summarize the gateway's per-hop lines
           ("gateway: fetch ...") and vmctl's grant lines since --since.

Usage:
  CHOIR_API_KEY=... scripts/host-fetch-proof.py api --computer computer-... \\
      [--host https://choir.news] [--out host-fetch-proof-api.json]
  scripts/host-fetch-proof.py journal --since "2026-10-22 10:00" \\
      [--identity vm-...] [--out host-fetch-proof-journal.json]

The api mode proves the capability; only the journal proves the hop went
through the host. Run both for a granted computer, and the api mode alone on
an ungranted one (its journal must show no fetch lines for it).
"""
import argparse
import collections
import json
import os
import re
import subprocess
import sys
import time
import urllib.error
import urllib.request

PUBLIC_PAGE = "https://en.wikipedia.org/wiki/Hypertext_Transfer_Protocol"
YOUTUBE_URL = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
PODCAST_FEED = "https://feeds.npr.org/510289/podcast.xml"
RSS_SOURCE = "rss:bbc_world"
# Each must be refused: cloud metadata, a public name that resolves to
# loopback, and a private literal.
REFUSALS = [
    "http://169.254.169.254/latest/meta-data/",
    "http://localtest.me/",
    "http://10.0.0.1/",
]


def call(host, key, computer, path, method="GET", body=None, timeout=120):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(host + path, data=data, method=method)
    req.add_header("Authorization", "Bearer " + key)
    req.add_header("X-Choir-Computer", computer)
    if data is not None:
        req.add_header("Content-Type", "application/json")
    started = time.time()
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read()
            status = resp.status
    except urllib.error.HTTPError as e:
        raw, status = e.read(), e.code
    except Exception as e:  # network error: record, do not raise
        return {"path": path, "status": None, "error": type(e).__name__, "ms": int((time.time() - started) * 1000)}
    try:
        parsed = json.loads(raw)
    except ValueError:
        parsed = raw[:400].decode("utf-8", "replace")
    return {"path": path, "status": status, "body": parsed, "ms": int((time.time() - started) * 1000)}


def summarize(body, keys):
    if not isinstance(body, dict):
        return body
    return {k: body.get(k) for k in keys if k in body}


def run_api(args):
    key = os.environ.get("CHOIR_API_KEY", "").strip()
    if not key or not args.computer:
        print("CHOIR_API_KEY (environment) and --computer are required", file=sys.stderr)
        return 2
    h, c = args.host.rstrip("/"), args.computer
    out = {"mode": "api", "host": h, "computer": c, "started_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()), "checks": []}

    def check(name, result, ok):
        result["name"], result["ok"] = name, bool(ok)
        out["checks"].append(result)
        print(f"{'PASS' if ok else 'FAIL'} {name} status={result.get('status')} ms={result.get('ms')}")

    r = call(h, key, c, "/api/content/import-url", "POST", {"url": PUBLIC_PAGE})
    check("import a public page", r, r["status"] == 200)
    r["body"] = summarize(r.get("body"), ["id", "title", "status", "canonical_url"])

    r = call(h, key, c, "/api/content/import-url", "POST", {"url": YOUTUBE_URL})
    check("import a YouTube transcript", r, r["status"] == 200)
    r["body"] = summarize(r.get("body"), ["id", "title", "status", "canonical_url"])

    r = call(h, key, c, "/api/podcast/search?q=planet+money&limit=3")
    ok = r["status"] == 200 and isinstance(r.get("body"), dict) and r["body"].get("provider_status") == "success"
    check("podcast search (iTunes)", r, ok)
    r["body"] = summarize(r.get("body"), ["provider", "provider_status", "warnings"])

    r = call(h, key, c, "/api/podcast/subscriptions", "POST", {"feed_url": PODCAST_FEED})
    check("podcast subscribe (feed fetch)", r, r["status"] in (200, 201))
    r["body"] = summarize(r.get("body"), ["feed_url", "title", "error"])

    r = call(h, key, c, "/api/autopaper/sources/schedules", "POST", {"source_id": RSS_SOURCE, "interval_minutes": 15, "reason": "FP10 host fetch proof"})
    check("start an RSS schedule", r, r["status"] in (200, 201))
    deadline = time.time() + args.poll_wait
    polled = None
    while time.time() < deadline:
        time.sleep(15)
        s = call(h, key, c, "/api/autopaper/sources/schedules")
        rows = s.get("body", {}).get("schedules", []) if isinstance(s.get("body"), dict) else []
        row = next((x for x in rows if x.get("source_id") == RSS_SOURCE), None)
        if row and row.get("recent"):
            polled = {"path": s["path"], "status": s["status"], "ms": s["ms"], "body": row}
            break
    polled_ok = polled is not None and polled["body"].get("consecutive_failures", 1) == 0
    check("the RSS schedule polled without failure", polled or {"status": None}, polled_ok)
    call(h, key, c, f"/api/autopaper/sources/schedules?source_id={RSS_SOURCE}", "DELETE")

    for url in REFUSALS:
        r = call(h, key, c, "/api/content/import-url", "POST", {"url": url})
        check(f"refuse {url}", r, r["status"] is not None and r["status"] >= 400)

    out["finished_at"] = time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())
    with open(args.out, "w") as f:
        json.dump(out, f, indent=2)
    print(f"artifact: {args.out}")
    return 0 if all(x["ok"] for x in out["checks"]) else 1


LINE = re.compile(r"gateway: fetch(?: (?P<kind>refused|failed|aborted))? computer=(?P<id>\S+)(?: method=(?P<method>\S*))?(?: host=(?P<host>\S*))?(?: status=(?P<status>\d+))?")


def run_journal(args):
    def journal(unit):
        cmd = ["journalctl", "-u", unit, "--since", args.since, "--no-pager", "-o", "cat"]
        return subprocess.run(cmd, capture_output=True, text=True, check=True).stdout.splitlines()

    hops = collections.Counter()
    hosts = collections.defaultdict(collections.Counter)
    refusals = []
    for line in journal(args.gateway_unit):
        m = LINE.search(line)
        if not m or (args.identity and m["id"] != args.identity):
            continue
        kind = m["kind"] or "ok"
        hops[(m["id"], kind)] += 1
        hosts[m["id"]][m["host"] or "-"] += 1
        if kind != "ok":
            refusals.append(line[line.index("gateway: fetch"):])
    grants = [l for l in journal(args.vmctl_unit) if "host fetch route for computer" in l]
    out = {
        "mode": "journal",
        "since": args.since,
        "hops": [{"identity": i, "kind": k, "count": n} for (i, k), n in sorted(hops.items())],
        "hosts": {i: dict(c.most_common(40)) for i, c in hosts.items()},
        "refusals": refusals[-50:],
        "grants": grants[-20:],
    }
    with open(args.out, "w") as f:
        json.dump(out, f, indent=2)
    for row in out["hops"]:
        print(f"{row['identity']} {row['kind']} {row['count']}")
    print(f"grant lines: {len(grants)}; refusal lines: {len(refusals)}; artifact: {args.out}")
    return 0


def main():
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = p.add_subparsers(dest="mode", required=True)
    a = sub.add_parser("api")
    a.add_argument("--host", default="https://choir.news")
    a.add_argument("--computer", required=True)
    a.add_argument("--poll-wait", type=int, default=300, help="seconds to wait for the RSS schedule's first poll")
    a.add_argument("--out", default="host-fetch-proof-api.json")
    j = sub.add_parser("journal")
    j.add_argument("--since", required=True)
    j.add_argument("--identity", default="", help="the gateway identity (vm-...) of one computer")
    j.add_argument("--gateway-unit", default="go-choir-gateway")
    j.add_argument("--vmctl-unit", default="go-choir-vmctl")
    j.add_argument("--out", default="host-fetch-proof-journal.json")
    args = p.parse_args()
    return run_api(args) if args.mode == "api" else run_journal(args)


if __name__ == "__main__":
    sys.exit(main())
