#!/usr/bin/env python3
"""Merge recall of a paper window against the registered judge (station L2a).

The denominator of acceptance action 2: same-event, cross-outlet capture
pairs among the window's captures, as grouped by the registered judge
(docs/evidence/l2a-judge-registration-2026-10-14.md): gemini-3.8-flash
through the owner's omp CLI, two shuffled calls per block, a pair counts
only when both calls group it. The numerator: those pairs whose two
captures management put into the same story (the cast that opened it or a
Note that fed it). Pairs in one story that the judge never grouped are
listed as false-merge candidates, to be read by hand.

Credentials come from a JSON file with api_key and computer_id; the key is
never printed.

usage: l2a_window_pairs.py --cred FILE --start ISO --end ISO --out FILE
"""
import argparse
import itertools
import json
import random
import re
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request

HOST = "https://choir.news"
MODEL = "google-antigravity/gemini-3.8-flash"
SERVED = ("google-antigravity", "gemini-3.8-flash")
BLOCK, OVERLAP = 400, 100
TWO_LEVEL_SUFFIXES = {"co.uk", "com.au", "co.jp", "com.tw", "org.uk", "com.br", "co.kr", "com.cn", "ne.jp", "or.jp", "gov.uk", "ac.uk"}
CAPTURE_ID = re.compile(r"choir\.web_capture:[A-Za-z0-9:_\-]+")
PROMPT = """You group news items by real-world event. Two items are the same
event when they report the same incident or its direct development
(an arrest and the charges that follow; a strike and its casualty
count), in any language. Items on the same topic that report a
different incident are different events. Below are numbered items:
outlet, language, title. Answer only with JSON:
{"groups": [[numbers of items that report one event], ...]}. List
only groups of two or more items. Leave out items that match nothing.

"""


def registered_domain(uri):
    host = urllib.parse.urlparse(uri).hostname or ""
    parts = host.lower().split(".")
    if len(parts) >= 3 and ".".join(parts[-2:]) in TWO_LEVEL_SUFFIXES:
        return ".".join(parts[-3:])
    return ".".join(parts[-2:])


def script_of(text):
    """The same coarse script guess the intake records (kana wins over han)."""
    counts = {"kana": 0, "han": 0, "hangul": 0, "arabic": 0, "cyrillic": 0, "latin": 0}
    for ch in text:
        o = ord(ch)
        if 0x3040 <= o <= 0x30FF:
            counts["kana"] += 1
        elif 0x4E00 <= o <= 0x9FFF or 0x3400 <= o <= 0x4DBF:
            counts["han"] += 1
        elif 0xAC00 <= o <= 0xD7AF or 0x1100 <= o <= 0x11FF:
            counts["hangul"] += 1
        elif 0x0600 <= o <= 0x06FF:
            counts["arabic"] += 1
        elif 0x0400 <= o <= 0x04FF:
            counts["cyrillic"] += 1
        elif ch.isascii() and ch.isalpha():
            counts["latin"] += 1
    if counts["kana"]:
        return "kana"
    best = max(counts, key=counts.get)
    return best if counts[best] else "other"


class Client:
    def __init__(self, cred):
        self.key = cred.get("api_key") or cred.get("key")
        self.computer = cred.get("computer_id")

    def get(self, path):
        headers = {"Authorization": "Bearer " + self.key, "X-Choir-Computer": self.computer}
        for _ in range(10):
            try:
                req = urllib.request.Request(HOST + path, headers=headers)
                return json.load(urllib.request.urlopen(req, timeout=120))
            except urllib.error.HTTPError as err:
                if err.code in (502, 503, 504):
                    time.sleep(15)
                    continue
                raise
        raise RuntimeError("unavailable: " + path)


def last_assistant(obj):
    """The last assistant message in omp's JSON output, searched recursively."""
    found = None
    if isinstance(obj, dict):
        if obj.get("role") == "assistant":
            found = obj
        for v in obj.values():
            found = last_assistant(v) or found
    elif isinstance(obj, list):
        for v in obj:
            found = last_assistant(v) or found
    return found


def judge(lines):
    """One judge call; returns (groups as lists of line indexes, served ok)."""
    prompt = PROMPT + "\n".join(f"{i}. {line}" for i, line in enumerate(lines))
    out = subprocess.run(
        ["omp", "-p", "--mode", "json", "--model", MODEL, "--thinking", "high", "--no-session", "--no-tools", prompt],
        capture_output=True, text=True, timeout=900, cwd="/tmp",
    ).stdout
    msg = None
    for raw in out.splitlines():
        raw = raw.strip()
        if raw.startswith("{"):
            try:
                msg = last_assistant(json.loads(raw)) or msg
            except json.JSONDecodeError:
                continue
    if not msg or (msg.get("provider"), msg.get("model")) != SERVED:
        return [], False
    text = "".join(c.get("text", "") for c in msg.get("content", []) if isinstance(c, dict))
    m = re.search(r"\{.*\}", text, re.S)
    if not m:
        return [], True
    try:
        groups = json.loads(m.group(0)).get("groups") or []
    except json.JSONDecodeError:
        return [], True
    return [[i for i in g if isinstance(i, int) and 0 <= i < len(lines)] for g in groups], True


def pairs_of(groups):
    out = set()
    for g in groups:
        for a, b in itertools.combinations(sorted(set(g)), 2):
            out.add((a, b))
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--cred", required=True)
    ap.add_argument("--start", required=True, help="UTC RFC3339, e.g. 2026-10-13T23:53:00Z")
    ap.add_argument("--end", required=True)
    ap.add_argument("--out", required=True)
    args = ap.parse_args()
    c = Client(json.load(open(args.cred)))

    q = urllib.parse.urlencode({"since": args.start, "until": args.end})
    window = c.get("/api/captures?" + q)
    caps = {}
    for cap in window.get("captures") or []:
        if cap.get("in_window", True) and cap.get("capture_id") and cap.get("title"):
            caps[cap["capture_id"]] = cap
    items = sorted(caps.values(), key=lambda x: x.get("created_at") or "")
    ids = [x["capture_id"] for x in items]
    lines = [f"{registered_domain(x.get('url', ''))} | {script_of(x['title'])} | {x['title'].strip()}" for x in items]

    agreed, disputed, misrouted = set(), set(), 0
    start = 0
    while start < len(items):
        idx = list(range(start, min(start + BLOCK, len(items))))
        calls = []
        for seed in (1, 2):
            order = idx[:]
            random.Random(seed).shuffle(order)
            groups, ok = judge([lines[i] for i in order])
            if not ok:
                misrouted += 1
            calls.append({(min(order[a], order[b]), max(order[a], order[b])) for a, b in pairs_of(groups)})
        agreed |= calls[0] & calls[1]
        disputed |= calls[0] ^ calls[1]
        if start + BLOCK >= len(items):
            break
        start += BLOCK - OVERLAP
    disputed -= agreed

    # Which story each capture went into: the cast that opened it or a Note
    # that fed it, read from each story's Texture inbox.
    story_of = {}
    docs = c.get("/api/texture/documents?limit=500")
    docs = docs.get("documents") or docs.get("items") or docs
    for d in docs:
        doc_id = d.get("doc_id") or d.get("id")
        recs = c.get("/api/inboxes/" + urllib.parse.quote("texture:" + doc_id, safe="") + "/records?limit=500").get("records") or []
        for r in recs:
            if not str(r.get("sender_inbox", "")).startswith("management:"):
                continue
            for cid in CAPTURE_ID.findall(json.dumps(r.get("directive") or {})):
                story_of.setdefault(cid, doc_id)

    cross = [(a, b) for a, b in agreed if registered_domain(items[a].get("url", "")) != registered_domain(items[b].get("url", ""))]
    merged = [(a, b) for a, b in cross if story_of.get(ids[a]) and story_of.get(ids[a]) == story_of.get(ids[b])]
    grouped = {i for p in agreed | disputed for i in p}
    by_story = {}
    for i, cid in enumerate(ids):
        if cid in story_of:
            by_story.setdefault(story_of[cid], []).append(i)
    false_candidates = []
    for doc, members in by_story.items():
        for a, b in itertools.combinations(sorted(members), 2):
            if (a, b) not in agreed and (a, b) not in disputed:
                false_candidates.append({"doc_id": doc, "a": lines[a], "b": lines[b]})

    result = {
        "window": {"start": args.start, "end": args.end},
        "judge": {"model": MODEL, "registration": "docs/evidence/l2a-judge-registration-2026-10-14.md", "calls_not_served_by_registered_model": misrouted},
        "captures": len(items),
        "captures_with_story": sum(1 for cid in ids if cid in story_of),
        "captures_grouped_by_judge": len(grouped),
        "same_event_pairs": len(agreed),
        "disputed_pairs": len(disputed),
        "cross_outlet_same_event_pairs": len(cross),
        "merged_cross_outlet_pairs": len(merged),
        "merge_recall": round(len(merged) / len(cross), 3) if cross else None,
        "false_merge_candidates": len(false_candidates),
        "pairs": [{"a": lines[a], "b": lines[b], "same_story": (a, b) in set(merged)} for a, b in sorted(cross)],
        "false_merge_candidate_pairs": false_candidates,
    }
    json.dump(result, open(args.out, "w"), indent=1, ensure_ascii=False)
    print(json.dumps({k: v for k, v in result.items() if k not in ("pairs", "false_merge_candidate_pairs")}))


if __name__ == "__main__":
    main()
