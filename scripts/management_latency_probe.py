#!/usr/bin/env python3
"""Management latency over one UTC window (management delegation, slice S0).

Reads a computer through product routes only (no SSH, no /internal) and
reports, as one JSON document:

  - AC1: each capture arrival's time to its first disposition. An arrival
    is a captures record from a source (sender source:<id>) in
    management's inbox; its time is the record's committed_at. A
    disposition is a record management sent that names the capture:
      open    a Cast whose cast.captures names it, in a story's inbox;
      feed    a disposition of kind feed in a story's inbox;
      decline a disposition of kind decline in the source's inbox.
    Notes that name a capture only in their text (before intake.labels)
    count as feed (to a story) or decline (to a source) and are counted as
    unstructured. Opens and feeds are reported apart from declines: a
    decline cannot satisfy latency (design round 2, R2-10).
  - AC8: the disposition mix per arrival capture.
  - AC2: management turn durations (run created to finished), p50, p95 and
    max, with each turn's model, model-call count and call durations; also
    for intake-only turns (estimate, below).
  - Busy fraction: the share of the window management was in a run.
  - Cell durations from desk.cell events: p50, p99, max, and the counts
    over 60 s, 300 s and 600 s. Slice S1's default cell deadline is
    max(300 s, 2 x p99) of these (R2-19).
  - AC10 (estimate): per parent run, the records committed to
    management's inbox since the previous parent run started, by sender
    class. The route does not say which records a run read, so this is an
    estimate.

Routes: GET /api/computer-settings (owner id), GET
/api/inboxes/<inbox>/records?after=&limit=500 (paged by next_after),
GET /api/runs?channel_id=management:<owner>&limit=500, GET
/api/runs/<id>/events?after=&limit=1000, GET /api/texture/documents.

Stories are found from the documents list (newest 50), the receipts in
management's cell results ("rlm:cast:N to texture:<doc>"), and the story
ids management's cells name or read (a Note's receipt names no inbox), so
a story fed in the window is read even when the list no longer shows it.

Owner privacy: the output holds counts, timings and opaque ids only, never
titles or text. --counts-only drops the per-run and per-capture lists too
(use it on an owner's computer). Credentials come from a 0600 JSON file
with api_key (or key) and computer_id; the key is never printed.

usage: management_latency_probe.py --cred FILE --start ISO [--end ISO]
       [--out FILE] [--counts-only]
"""
import argparse
import datetime as dt
import json
import math
import os
import re
import stat
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

DEFAULT_HOST = "https://choir.news"
CAPTURE_ID = re.compile(r"(?:obj:)?choir\.web_capture:[A-Za-z0-9:_\-]+")
RECEIPT_INBOX = re.compile(r"rlm:[a-z_]+:\d+ to ((?:texture|source):[A-Za-z0-9:_\-.]+)")
UUID = r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}"
STORY_REF = re.compile(r"texture:(" + UUID + r")|doc_?[iI][dD]\W{1,6}(" + UUID + r")")
PAGE = 500
EVENT_PAGE = 1000


class Client:
    def __init__(self, cred, host):
        self.key = cred.get("api_key") or cred.get("key")
        self.computer = cred.get("computer_id")
        if not self.key or not self.computer:
            raise SystemExit("credential needs api_key (or key) and computer_id")
        self.host = host.rstrip("/")

    def get(self, path):
        headers = {"Authorization": "Bearer " + self.key, "X-Choir-Computer": self.computer}
        for _ in range(10):
            try:
                req = urllib.request.Request(self.host + path, headers=headers)
                return json.load(urllib.request.urlopen(req, timeout=120))
            except urllib.error.HTTPError as err:
                if err.code in (502, 503, 504):
                    time.sleep(15)
                    continue
                # The body can echo request context; print the status only.
                raise SystemExit(f"GET {path.split('?')[0]}: HTTP {err.code}") from None
            except urllib.error.URLError:
                time.sleep(15)
        raise SystemExit("unavailable: " + path.split("?")[0])

    def records(self, inbox):
        """Every record of one inbox, paged by seq."""
        out, after = [], 0
        q = urllib.parse.quote(inbox, safe="")
        while True:
            page = self.get(f"/api/inboxes/{q}/records?limit={PAGE}&after={after}")
            out += page.get("records") or []
            nxt = int(page.get("next_after") or 0)
            if nxt <= after:
                return out
            after = nxt

    def events(self, run_id):
        out, after = [], 0
        q = urllib.parse.quote(run_id, safe="")
        while True:
            page = self.get(f"/api/runs/{q}/events?limit={EVENT_PAGE}&after={after}")
            out += page.get("events") or []
            nxt = int(page.get("next_after") or 0)
            if nxt <= after:
                return out
            after = nxt


def parse_ts(s):
    """RFC 3339 with up to nanoseconds, or a bare ISO time read as UTC."""
    if not s:
        return None
    s = s.strip().replace("Z", "+00:00")
    m = re.match(r"^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(\.\d+)?(.*)$", s)
    if not m:
        return None
    frac = (m.group(2) or "")[:7]
    tz = m.group(3) or "+00:00"
    return dt.datetime.fromisoformat(m.group(1) + frac + tz).astimezone(dt.timezone.utc)


def iso(t):
    return t.strftime("%Y-%m-%dT%H:%M:%SZ") if t else None


def pct(values, p):
    """Nearest-rank percentile; None for an empty list."""
    if not values:
        return None
    v = sorted(values)
    k = max(0, math.ceil(p / 100.0 * len(v)) - 1)
    return v[k]


def dist(values, ps=(50, 95)):
    out = {"n": len(values)}
    for p in ps:
        out[f"p{p}"] = pct(values, p)
    out["max"] = max(values) if values else None
    return out


def norm_id(capture_id):
    return capture_id[4:] if capture_id.startswith("obj:") else capture_id


def committed_at(rec):
    return parse_ts((rec.get("provenance") or {}).get("committed_at") or "")


def sender_of(rec):
    return str(rec.get("sender_inbox") or (rec.get("provenance") or {}).get("agent_id") or "")


def sender_class(rec):
    s = sender_of(rec)
    head = s.split(":", 1)[0]
    if head == "source":
        return "captures" if rec.get("captures") else "source_other"
    if head == "texture":
        return "texture_" + str(rec.get("kind") or "record")
    return head or "unknown"


def owner_of(c):
    for st in c.get("/api/computer-settings").get("settings") or []:
        if st.get("owner_id"):
            return st["owner_id"]
    for r in c.get("/api/runs?limit=50").get("runs") or []:
        if str(r.get("agent_id", "")).startswith("management:"):
            return r["agent_id"].split(":", 1)[1]
    raise SystemExit("cannot read the owner id")


def load_cred(path):
    st = os.stat(path)
    if st.st_mode & (stat.S_IRWXG | stat.S_IRWXO):
        print(f"warning: {path} is readable beyond its owner (want 0600)", file=sys.stderr)
    with open(path) as f:
        return json.load(f)


def model_calls(events):
    """Per model call: (iteration, model, seconds), paired from the request
    loop.progress (no input_tokens) and the response one (input_tokens)."""
    calls, pending = [], {}
    for e in events:
        if e.get("kind") != "loop.progress":
            continue
        p = e.get("payload") or {}
        it = p.get("iteration")
        t = parse_ts(e.get("ts"))
        if "input_tokens" in p or "stop_reason" in p:
            start = pending.pop(it, None)
            if start is not None and t is not None:
                calls.append({"iteration": it, "model": p.get("model") or p.get("llm_model"),
                              "s": round((t - start).total_seconds(), 1),
                              "input_tokens": p.get("input_tokens")})
        elif "message_roles" in p or "messages" in p:
            pending[it] = t
    return calls


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    ap.add_argument("--cred", required=True, help="0600 JSON with api_key and computer_id")
    ap.add_argument("--start", required=True, help="UTC, e.g. 2026-10-14T06:51:00Z")
    ap.add_argument("--end", help="UTC; default now")
    ap.add_argument("--out", help="write the JSON here (default stdout)")
    ap.add_argument("--host", default=None, help="default: the credential's base_url, else " + DEFAULT_HOST)
    ap.add_argument("--counts-only", action="store_true", help="drop per-run and per-capture lists")
    args = ap.parse_args()

    cred = load_cred(args.cred)
    c = Client(cred, args.host or cred.get("base_url") or DEFAULT_HOST)
    start = parse_ts(args.start)
    end = parse_ts(args.end) if args.end else dt.datetime.now(dt.timezone.utc)
    if not start or not end or end <= start:
        raise SystemExit("need start < end")
    window_s = (end - start).total_seconds()
    owner = owner_of(c)
    mgmt = "management:" + owner

    # Management's inbox: arrivals and the wake estimate.
    mrecs = c.records(mgmt)
    arrivals = {}  # capture id -> {at, source, seq}
    sources = set()
    inbox_in_window = []
    for r in mrecs:
        at = committed_at(r)
        if at is None:
            continue
        snd = sender_of(r)
        if snd.startswith("source:"):
            sources.add(snd)
        if start <= at < end:
            inbox_in_window.append(r)
        if not snd.startswith("source:") or not (start <= at < end):
            continue
        for item in (r.get("captures") or {}).get("items") or []:
            cid = item.get("capture_id")
            if cid and norm_id(cid) not in arrivals:
                arrivals[norm_id(cid)] = {"at": at, "source": snd, "seq": r.get("inbox_seq")}

    # Management's runs (parent: agent id management:<owner>).
    runs = [r for r in c.get(f"/api/runs?channel_id={urllib.parse.quote(mgmt, safe='')}&limit=500").get("runs") or []
            if r.get("agent_id") == mgmt]
    for r in runs:
        r["_created"] = parse_ts(r.get("created_at"))
        r["_finished"] = parse_ts(r.get("finished_at")) if r.get("finished_at") else None
    runs.sort(key=lambda r: r["_created"])
    oldest_listed = runs[0]["_created"] if runs else None
    in_window = [r for r in runs if start <= r["_created"] < end]
    overlapping = [r for r in runs if r["_created"] < end and (r["_finished"] or end) > start]

    story_inboxes = set()
    turn_rows, cell_ms, cell_rows = [], [], []
    for r in overlapping:
        events = c.events(r["run_id"])
        cells = [e for e in events if e.get("kind") == "desk.cell"]
        for e in events:
            if e.get("kind") in ("tool.result", "tool.invoked"):
                text = json.dumps(e.get("payload") or {})
                for inbox in RECEIPT_INBOX.findall(text):
                    (story_inboxes if inbox.startswith("texture:") else sources).add(inbox)
                # A Note's receipt names no inbox, so the stories a cell
                # names or reads (texture:<doc>, doc_id <doc>) are read too.
                for doc in STORY_REF.findall(text):
                    story_inboxes.add("texture:" + (doc[0] or doc[1]))
        if r not in in_window:
            continue
        calls = model_calls(events)
        for e in cells:
            p = e.get("payload") or {}
            ms = int(p.get("duration_ms") or 0)
            cell_ms.append(ms)
            err = str(p.get("error") or "")
            cell_rows.append({"run_id": r["run_id"], "ms": ms,
                              "worker_died": ("read header" in err or "worker dead" in err),
                              "timed_out": ("timeout" in err or "deadline" in err),
                              "error_kind": (p.get("diag") or ("error" if err else ""))})
        fin = r["_finished"]
        turn_rows.append({
            "run_id": r["run_id"], "state": r.get("state"), "created_at": iso(r["_created"]),
            "finished_at": iso(fin),
            "duration_s": round(((fin or end) - r["_created"]).total_seconds(), 1),
            "open": fin is None,
            "model": (r.get("metadata") or {}).get("llm_model") or (r.get("metadata") or {}).get("model"),
            "models_called": sorted({x["model"] for x in calls if x["model"]}),
            "model_calls": len(calls),
            "call_s": dist([x["s"] for x in calls]),
            "cells": len(cells),
            "cell_max_ms": max([int((e.get("payload") or {}).get("duration_ms") or 0) for e in cells], default=0),
        })

    # The wake estimate: records since the previous parent run started.
    for i, row in enumerate(turn_rows):
        created = parse_ts(row["created_at"])
        prev = [r["_created"] for r in runs if r["_created"] < created]
        lo = prev[-1] if prev else None
        kinds = {}
        for rec in mrecs:
            at = committed_at(rec)
            if at is None or at > created or (lo is not None and at <= lo):
                continue
            k = sender_class(rec)
            kinds[k] = kinds.get(k, 0) + 1
        row["woke_by_estimate"] = kinds
        row["intake_only_estimate"] = bool(kinds) and set(kinds) == {"captures"}

    # Dispositions, read from every story inbox and every source inbox.
    docs = c.get("/api/texture/documents?limit=500")
    for d in docs.get("documents") or docs.get("items") or []:
        if d.get("doc_id"):
            story_inboxes.add("texture:" + d["doc_id"])
    disp = {}  # capture id -> list of (at, kind, structured)
    def note(cid, at, kind, structured):
        disp.setdefault(norm_id(cid), []).append((at, kind, structured))
    for inbox in sorted(story_inboxes):
        for rec in c.records(inbox):
            if not sender_of(rec).startswith("management:"):
                continue
            at = committed_at(rec)
            if at is None or at >= end:
                continue
            named = set()
            for cid in (rec.get("cast") or {}).get("captures") or []:
                note(cid, at, "open", True)
                named.add(norm_id(cid))
            d = rec.get("disposition") or {}
            if d.get("kind") in ("feed", "decline"):
                for cid in d.get("captures") or []:
                    note(cid, at, d["kind"], True)
                    named.add(norm_id(cid))
            if not named:
                kind = "open" if rec.get("kind") == "cast" else "feed"
                for cid in set(CAPTURE_ID.findall(json.dumps(rec.get("directive") or {}))):
                    note(cid, at, kind, False)
    for inbox in sorted(s for s in sources if s.startswith("source:")):
        for rec in c.records(inbox):
            if not sender_of(rec).startswith("management:"):
                continue
            at = committed_at(rec)
            if at is None or at >= end:
                continue
            d = rec.get("disposition") or {}
            if d.get("kind") in ("feed", "decline"):
                for cid in d.get("captures") or []:
                    note(cid, at, d["kind"], True)
            else:
                for cid in set(CAPTURE_ID.findall(json.dumps(rec.get("directive") or {}))):
                    note(cid, at, "decline", False)

    # AC1 join and AC8 mix.
    lat = {"open": [], "feed": [], "decline": []}
    unstructured = 0
    before_arrival = 0
    undisposed_age = []
    cap_rows = []
    for cid, a in arrivals.items():
        ds = sorted(x for x in disp.get(cid, []) if x[0] >= a["at"])
        if disp.get(cid) and not ds:
            before_arrival += 1
        if not ds:
            undisposed_age.append(round((end - a["at"]).total_seconds(), 1))
            cap_rows.append({"capture_id": cid, "arrived_at": iso(a["at"]), "first": None})
            continue
        at, kind, structured = ds[0]
        s = round((at - a["at"]).total_seconds(), 1)
        lat[kind].append(s)
        unstructured += 0 if structured else 1
        cap_rows.append({"capture_id": cid, "arrived_at": iso(a["at"]), "first": kind, "latency_s": s,
                         "structured": structured})
    open_feed = lat["open"] + lat["feed"]
    n_arr = len(arrivals)

    # Busy fraction: union of parent-run intervals clipped to the window.
    spans = sorted((max(r["_created"], start), min(r["_finished"] or end, end)) for r in overlapping)
    busy, cur_s, cur_e = 0.0, None, None
    for s, e in spans:
        if e <= s:
            continue
        if cur_e is None or s > cur_e:
            if cur_e is not None:
                busy += (cur_e - cur_s).total_seconds()
            cur_s, cur_e = s, e
        else:
            cur_e = max(cur_e, e)
    if cur_e is not None:
        busy += (cur_e - cur_s).total_seconds()
    arrivals_found_busy = sum(1 for a in {(v["seq"], v["at"]) for v in arrivals.values()}
                              if any(s <= a[1] < e for s, e in spans))
    arrival_records = len({v["seq"] for v in arrivals.values()})

    done = [t["duration_s"] for t in turn_rows if not t["open"]]
    intake_only = [t["duration_s"] for t in turn_rows if not t["open"] and t["intake_only_estimate"]]
    p99_cell = pct(cell_ms, 99)
    out = {
        "schema": "management_latency_probe.v1",
        "computer_id": c.computer,
        "window": {"start": iso(start), "end": iso(end), "seconds": round(window_s)},
        "notes": [
            "latency = first disposition committed_at - arrival record committed_at",
            "woke_by and intake_only are estimates: records committed since the previous parent run started",
            "runs: /api/runs lists at most 500; oldest listed " + (iso(oldest_listed) or "none"),
        ],
        "ac1_latency_s": {
            "open_and_feed": dist(open_feed),
            "open": dist(lat["open"]),
            "feed": dist(lat["feed"]),
            "decline": dist(lat["decline"]),
            "all_dispositions": dist(open_feed + lat["decline"]),
            "bound": {"p95_le_s": 300, "max_le_s": 900,
                      "p95_ok": (pct(open_feed, 95) or 0) <= 300 if open_feed else None,
                      "max_ok": (max(open_feed) <= 900) if open_feed else None},
            "undisposed": {"n": len(undisposed_age), "age_s": dist(undisposed_age),
                           "older_than_900s": sum(1 for x in undisposed_age if x > 900)},
            "unstructured_first_dispositions": unstructured,
            "disposed_only_before_arrival": before_arrival,
        },
        "ac8_mix": {
            "arrival_records": arrival_records,
            "captures": n_arr,
            "open": len(lat["open"]), "feed": len(lat["feed"]), "decline": len(lat["decline"]),
            "undisposed": len(undisposed_age),
            "share": {k: (round(len(lat[k]) / n_arr, 3) if n_arr else None) for k in lat},
        },
        "ac2_turns_s": {
            "parent_runs_started": len(turn_rows),
            "finished": dist(done),
            "intake_only_estimate": dist(intake_only),
            "open_runs": [t["run_id"] for t in turn_rows if t["open"]] if not args.counts_only
            else sum(1 for t in turn_rows if t["open"]),
            "states": {s: sum(1 for t in turn_rows if t["state"] == s) for s in sorted({t["state"] for t in turn_rows})},
            "over_600s": sum(1 for x in done if x > 600),
            "bound": {"max_le_s": 600, "p95_le_s": 120},
            "per_turn_max_model_call_s": dist([x for t in turn_rows for x in ([t["call_s"]["max"]] if t["call_s"]["max"] is not None else [])]),
            "models": sorted({m for t in turn_rows for m in t["models_called"]}),
        },
        "busy": {
            "seconds": round(busy), "fraction": round(busy / window_s, 3),
            "arrival_records": arrival_records, "arrival_records_found_busy": arrivals_found_busy,
        },
        "cells_ms": {
            "n": len(cell_ms), "p50": pct(cell_ms, 50), "p95": pct(cell_ms, 95), "p99": p99_cell,
            "max": max(cell_ms) if cell_ms else None,
            "over_60s": sum(1 for x in cell_ms if x > 60000),
            "over_300s": sum(1 for x in cell_ms if x > 300000),
            "over_600s": sum(1 for x in cell_ms if x > 600000),
            "worker_died": sum(1 for x in cell_rows if x["worker_died"]),
            "timed_out": sum(1 for x in cell_rows if x["timed_out"]),
            "s1_default_deadline_s": max(300, math.ceil(2 * (p99_cell or 0) / 1000.0)),
        },
        "ac10_wakes_estimate": {
            k: sum(t["woke_by_estimate"].get(k, 0) for t in turn_rows)
            for k in sorted({k for t in turn_rows for k in t["woke_by_estimate"]})
        },
        "inbox_records_in_window": {
            k: sum(1 for r in inbox_in_window if sender_class(r) == k)
            for k in sorted({sender_class(r) for r in inbox_in_window})
        },
        "queue": "n/a until management tasks exist (S2)",
    }
    if not args.counts_only:
        out["turns"] = turn_rows
        out["captures"] = sorted(cap_rows, key=lambda x: x["arrived_at"])
        out["slow_cells"] = sorted([x for x in cell_rows if x["ms"] > 60000], key=lambda x: -x["ms"])
    text = json.dumps(out, indent=1, sort_keys=False)
    if args.out:
        fd = os.open(args.out, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
        with os.fdopen(fd, "w") as f:
            f.write(text + "\n")
    else:
        print(text)
    a1 = out["ac1_latency_s"]["open_and_feed"]
    print(f"window {iso(start)}..{iso(end)}: {n_arr} captures in {arrival_records} arrivals; "
          f"open+feed p50/p95/max {a1['p50']}/{a1['p95']}/{a1['max']} s; "
          f"turns {out['ac2_turns_s']['finished']}; busy {out['busy']['fraction']}; "
          f"cells p50/p99/max {out['cells_ms']['p50']}/{out['cells_ms']['p99']}/{out['cells_ms']['max']} ms",
          file=sys.stderr)


if __name__ == "__main__":
    main()
