#!/usr/bin/env python3
"""Watch a computer's boot window from Node B and say what held it.

docs/problems/paper-unreachable-minutes-after-every-boot-2026-10-22.md:
after every boot the paper's /health hangs for minutes. This probe samples,
every --interval seconds, from runtime start until /health has answered 200
for --settle seconds (or --max-minutes pass):

  - GET <guest>/health, 5 s timeout;
  - GET <guest>/api/runtime/observability?goroutines=1 with the read-only
    headers vmctl uses (X-Internal-Caller, X-Authenticated-User,
    X-Authenticated-Computer), 20 s timeout: the engine mutex's live holder,
    unanswered reads, memory, and key frames counted in the goroutine dump;
  - GET <guest>/internal/debug/pprof/procmem (RSS by process), when
    --procmem is given. The guest gates it on the host source address; it
    gets the marker header only, never the host-internal token.

Each sample gets a verdict (classify): healthy; h4_online_dolt_gc (the dump
shows the online Dolt GC while /health fails); store_held:<op> (another
engine holder held 10 s or more); process_stalled (observability did not
answer either: H2 or H3, read procmem and memory); health_failing_unattributed.

Output: one JSON line per sample to --out, each failing sample's goroutine
dump beside it (<out>.goroutines-<n>.txt), and a summary on stdout. Read
only. It sends no host credential (docs/problems/
host-internal-token-sent-to-guests-by-fp0-probes-2026-10-22.md).

usage: paper_boot_window_probe.py --guest http://10.x.y.2:8085 \
         --owner-id OWNER --computer-id COMPUTER --out FILE \
         [--procmem] [--interval 10] [--settle 60] [--max-minutes 15]
"""
import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.request

try:
    sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
    from choir_internal import internal_headers
except (ImportError, NameError):  # piped alone to the host: the marker only
    def internal_headers(url):
        return {"X-Internal-Caller": "true"}

# Frames whose presence in a dump attributes a hang. Matched with "(" so a
# longer name that starts the same is not counted.
FRAMES = [
    "runOnlineDoltGC",
    "database/sql.(*DB).conn",
    "engineMutex).lockAs",
    "HandleHealth",
    "finalizeBatch",
    "sweepActorWakes",
    "WarmSearchIndex",
    "ValidateDerivedKeys",
]
HOLD_ATTRIBUTE_MS = 10_000


def frame_counts(dump):
    return {f: dump.count(f + "(") for f in FRAMES}


def classify(s):
    health = s.get("health") or {}
    if health.get("status") == 200:
        return "healthy"
    obs = s.get("obs")
    if not obs or obs.get("timeout") or obs.get("status") != 200:
        return "process_stalled"
    if (obs.get("frames") or {}).get("runOnlineDoltGC", 0) > 0:
        return "h4_online_dolt_gc"
    live = obs.get("engine_mutex_live") or {}
    if live.get("holder_op") and live.get("held_ms", 0) >= HOLD_ATTRIBUTE_MS:
        return "store_held:" + live["holder_op"]
    return "health_failing_unattributed"


def summarize(samples):
    verdicts, first_failure, recovered = {}, None, None
    for s in samples:
        v = classify(s)
        verdicts[v] = verdicts.get(v, 0) + 1
        if v != "healthy" and first_failure is None:
            first_failure = s["t"]
        if v == "healthy" and first_failure is not None and recovered is None:
            recovered = s["t"]
    return {"first_failure_t": first_failure, "recovered_t": recovered, "verdicts": verdicts}


def get(url, headers, timeout):
    try:
        with urllib.request.urlopen(urllib.request.Request(url, headers=headers), timeout=timeout) as resp:
            return {"status": resp.status, "body": resp.read()}
    except urllib.error.HTTPError as err:
        return {"status": err.code, "body": b""}
    except Exception as err:  # timeouts, refused connections
        return {"timeout": True, "error": type(err).__name__}


def take_sample(args, n, t0):
    s = {"t": round(time.time() - t0, 1), "at": time.strftime("%H:%M:%S", time.gmtime())}
    h = get(args.guest + "/health", {}, 5)
    s["health"] = {k: v for k, v in h.items() if k != "body"}
    headers = {"X-Internal-Caller": "paper-boot-window-probe",
               "X-Authenticated-User": args.owner_id, "X-Authenticated-Computer": args.computer_id}
    o = get(args.guest + "/api/runtime/observability?goroutines=1", headers, 20)
    obs = {k: v for k, v in o.items() if k != "body"}
    if o.get("status") == 200:
        try:
            body = json.loads(o["body"])
        except ValueError:
            body = {}
        dump = body.get("goroutines") or ""
        obs["frames"] = frame_counts(dump)
        obs["engine_mutex_live"] = body.get("engine_mutex_live")
        obs["unanswered"] = body.get("unanswered")
        obs["memory"] = body.get("memory")
        if s["health"].get("status") != 200 and dump:
            with open(f"{args.out}.goroutines-{n}.txt", "w") as f:
                f.write(dump)
    s["obs"] = obs
    if args.procmem:
        url = args.guest + "/internal/debug/pprof/procmem"
        p = get(url, internal_headers(url), 10)
        s["procmem"] = json.loads(p["body"]) if p.get("status") == 200 else {k: v for k, v in p.items() if k != "body"}
    s["verdict"] = classify(s)
    return s


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--guest", required=True)
    ap.add_argument("--owner-id", required=True)
    ap.add_argument("--computer-id", required=True)
    ap.add_argument("--out", required=True)
    ap.add_argument("--procmem", action="store_true")
    ap.add_argument("--interval", type=float, default=10)
    ap.add_argument("--settle", type=float, default=60)
    ap.add_argument("--max-minutes", type=float, default=15)
    args = ap.parse_args()
    args.guest = args.guest.rstrip("/")
    t0, samples, healthy_since, n = time.time(), [], None, 0
    with open(args.out, "a") as out:
        while time.time() - t0 < args.max_minutes * 60:
            s = take_sample(args, n, t0)
            n += 1
            samples.append(s)
            out.write(json.dumps(s) + "\n")
            out.flush()
            print(f"{s['at']} t={s['t']} {s['verdict']}", flush=True)
            failed_before = any(x["verdict"] != "healthy" for x in samples)
            if s["verdict"] == "healthy":
                healthy_since = s["t"] if healthy_since is None else healthy_since
                if failed_before and s["t"] - healthy_since >= args.settle:
                    break
            else:
                healthy_since = None
            time.sleep(args.interval)
    print(json.dumps(summarize(samples)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
