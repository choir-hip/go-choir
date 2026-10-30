#!/usr/bin/env python3
"""Merge recall of a paper window against the registered judge (station L2a).

The denominator of acceptance action 2: same-event, cross-outlet capture
pairs among the window's captures, as grouped by the registered judge
(docs/evidence/l2a-judge-registration-2026-10-14.md): gemini-3.8-flash
through the owner's omp CLI, two shuffled calls per block, a pair counts
only when both calls group it. The numerator: those pairs whose two
captures management put into a shared story (the cast that opened it or a
Note that fed it). Stories are found beyond the 50-document list cap
(story_doc_ids). Pairs in one story that the judge never grouped are
listed as false-merge candidates, to be read by hand. Also reported: Jev's
label coverage of the window's polls, re-captures of known pages, and the
production ratios behind the owner's principle that more sources should
mean only marginally more articles (captures and new stories per hour,
captures per new story, intake outlets per story).

Credentials come from a JSON file with api_key and computer_id; the key is
never printed.

usage: l2a_window_pairs.py --cred FILE --start ISO --end ISO --out FILE
"""
import argparse
import datetime as dt
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
            except (urllib.error.URLError, TimeoutError):
                # A busy computer can stall one read; reading ~300 story
                # inboxes meets that.
                time.sleep(15)
                continue
        raise RuntimeError("unavailable: " + path)


def norm_id(capture_id):
    """Capture ids appear with and without the obj: prefix."""
    return capture_id[4:] if capture_id.startswith("obj:") else capture_id


RECEIPT_STORY = re.compile(r"rlm:[a-z_]+:\d+ to texture:(" + r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}" + r")")
RECEIPT_TASK = re.compile(r"rlm:delegate:\d+ to (management-task:[A-Za-z0-9:_\-.]+)")
# Story doc ids are name-based (v5) UUIDs; run, loop and event ids are not.
DOC_UUID = re.compile(r"[0-9a-f]{8}-[0-9a-f]{4}-5[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}")


def paged(c, path, key):
    """Every item of a route paged by after/next_after."""
    out, after = [], 0
    sep = "&" if "?" in path else "?"
    while True:
        page = c.get(f"{path}{sep}limit=500&after={after}")
        out += page.get(key) or []
        nxt = int(page.get("next_after") or 0)
        if nxt <= after:
            return out
        after = nxt


def story_doc_ids(c, start=None, end=None):
    """Every story document management (or its tasks) may have disposed a
    capture into. GET /api/texture/documents lists only the 50 most recently
    updated documents, so a window scored an hour later loses most of its
    stories that way (docs/archive/problems/l2a-merge-recall-on-the-paper-2026-10-14.md,
    finding 1). The list is joined with every texture:<doc> in the receipts
    of management's runs and its tasks' runs, and every story id their cells
    and outputs name (a Note's receipt names no inbox). Without a window,
    every listed run is read."""
    docs = c.get("/api/texture/documents?limit=500")
    ids = {d.get("doc_id") or d.get("id") for d in (docs.get("documents") or docs.get("items") or [])}
    owner = None
    for st in c.get("/api/computer-settings").get("settings") or []:
        owner = owner or st.get("owner_id")
    if not owner:
        return {i for i in ids if i}

    def scan(runs):
        tasks = set()
        for r in runs:
            created, finished = r.get("created_at") or "", r.get("finished_at") or ""
            if start and finished and finished[:19] + "Z" < start:
                continue
            if end and created[:19] + "Z" > end:
                continue
            for e in paged(c, "/api/runs/" + urllib.parse.quote(r["run_id"], safe="") + "/events", "events"):
                if e.get("kind") not in ("tool.invoked", "tool.result"):
                    continue
                text = json.dumps(e.get("payload") or {})
                ids.update(RECEIPT_STORY.findall(text))
                ids.update(DOC_UUID.findall(text))
                tasks.update(RECEIPT_TASK.findall(text))
        return tasks

    mgmt = urllib.parse.quote("management:" + owner, safe="")
    tasks = scan(c.get(f"/api/runs?channel_id={mgmt}&limit=500").get("runs") or [])
    for task in sorted(tasks):
        scan(c.get("/api/runs?channel_id=" + urllib.parse.quote(task, safe="") + "&limit=5").get("runs") or [])
    return {i for i in ids if i}


def story_sets_by_capture(c, start=None, end=None, opened=None):
    """Every story each capture went into, read from each story's Texture
    inbox: the Cast that opened it (cast.captures) or a Feed disposition
    (disposition.captures), sent by management or one of its tasks. Older
    records name captures only in the directive text, so that is read too.
    Returns {capture id: {doc id: first committed_at}}; when opened is a
    dict, it is filled with {doc id: committed_at of its opening Cast}."""
    stories = {}
    for doc_id in sorted(story_doc_ids(c, start, end)):
        recs = paged(c, "/api/inboxes/" + urllib.parse.quote("texture:" + doc_id, safe="") + "/records", "records")
        for r in recs:
            sender = str(r.get("sender_inbox") or (r.get("provenance") or {}).get("agent_id") or "")
            if not (sender.startswith("management:") or sender.startswith("management-task:")):
                continue
            at = (r.get("provenance") or {}).get("committed_at") or ""
            if opened is not None and r.get("cast") is not None and r.get("inbox_seq") == 1:
                opened[doc_id] = at
            found = list((r.get("cast") or {}).get("captures") or [])
            disp = r.get("disposition") or {}
            if disp.get("kind") == "feed":
                found += disp.get("captures") or []
            found += CAPTURE_ID.findall(json.dumps(r.get("directive") or {}))
            for cid in found:
                if not str(cid).strip():
                    continue
                seen = stories.setdefault(norm_id(cid), {})
                if doc_id not in seen or at < seen[doc_id]:
                    seen[doc_id] = at
    return stories


def stories_by_capture(c, start=None, end=None):
    """The first story each capture went into (by commit time)."""
    return {cid: min(docs, key=docs.get) for cid, docs in story_sets_by_capture(c, start, end).items() if docs}


def label_coverage(c, start, end):
    """Jev's label coverage of the window's polls, from the schedules route."""
    out = {"polls": 0, "new": 0, "labelled": 0, "unlabelled": 0}
    scheds = c.get("/api/autopaper/sources/schedules")
    if isinstance(scheds, dict):
        scheds = scheds.get("schedules") or scheds.get("items") or []
    for sched in scheds or []:
        for u in sched.get("recent") or []:
            if start <= (u.get("at") or "")[:19] + "Z" < end:
                out["polls"] += 1
                for k in ("new", "labelled", "unlabelled"):
                    out[k] += int(u.get(k) or 0)
    return out


def window_arrivals(c, start, end):
    """The intake arrivals in [start, end): every source poll's captures
    record in management's inbox (sender source:<id>), paged by inbox seq.
    /api/captures lists only the newest 500 captures, research re-captures
    included, so it cannot serve an older window or count intake alone.
    An item whose URL an earlier arrival already named is marked recapture:
    a capture's identity is URL + text hash, so a feed whose item text
    churns (HN points and comments, live blogs) re-captures one page."""
    owner = None
    for st in c.get("/api/computer-settings").get("settings") or []:
        owner = owner or st.get("owner_id")
    inbox = urllib.parse.quote("management:" + owner, safe="")
    caps, after, seen_urls = {}, 0, set()
    while True:
        page = c.get(f"/api/inboxes/{inbox}/records?limit=500&after={after}")
        recs = page.get("records") or []
        if not recs:
            break
        for r in recs:
            if not str(r.get("sender_inbox", "")).startswith("source:"):
                continue
            at = (r.get("provenance") or {}).get("committed_at") or ""
            in_window = start <= at[:19] + "Z" < end
            for item in (r.get("captures") or {}).get("items") or []:
                url = item.get("url") or ""
                if in_window and item.get("capture_id") and item.get("title"):
                    caps[item["capture_id"]] = dict(item, created_at=at, recapture=url in seen_urls, source=r.get("sender_inbox"))
                seen_urls.add(url)
        nxt = int(page.get("next_after") or 0)
        if nxt <= after:
            break
        after = nxt
    return caps


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
        capture_output=True, text=True, timeout=1800, cwd="/tmp",
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

    caps = window_arrivals(c, args.start, args.end)
    items = sorted(caps.values(), key=lambda x: x.get("created_at") or "")
    ids = [norm_id(x["capture_id"]) for x in items]
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

    # Stories are read for runs up to two hours past the window, so a
    # capture disposed late in the window still counts.
    grace_end = (dt.datetime.fromisoformat(args.end.replace("Z", "+00:00")) + dt.timedelta(hours=2)).strftime("%Y-%m-%dT%H:%M:%SZ")
    opened = {}
    stories = story_sets_by_capture(c, args.start, grace_end, opened)
    first = {cid: min(docs, key=docs.get) for cid, docs in stories.items() if docs}

    def shared(a, b):
        return set(stories.get(ids[a]) or {}) & set(stories.get(ids[b]) or {})

    cross = [(a, b) for a, b in agreed if registered_domain(items[a].get("url", "")) != registered_domain(items[b].get("url", ""))]
    # A pair is merged when its captures share a story. A capture fed into
    # two stories (a roundup of two events) is in both.
    merged = [(a, b) for a, b in cross if shared(a, b)]
    merged_first = [(a, b) for a, b in cross if first.get(ids[a]) and first.get(ids[a]) == first.get(ids[b])]
    grouped = {i for p in agreed | disputed for i in p}
    by_story = {}
    for i, cid in enumerate(ids):
        for doc in stories.get(cid) or {}:
            by_story.setdefault(doc, []).append(i)
    false_candidates = []
    for doc, members in by_story.items():
        for a, b in itertools.combinations(sorted(members), 2):
            if (a, b) not in agreed and (a, b) not in disputed:
                false_candidates.append({"doc_id": doc, "a": lines[a], "b": lines[b], "a_id": ids[a], "b_id": ids[b]})

    # Production ratios (owner 2026-10-14: more sources should mean only
    # marginally more articles; sources per story is a crude proxy of
    # quality). Intake outlets count the window's captures only, not
    # research's citations.
    hours = (dt.datetime.fromisoformat(args.end.replace("Z", "+00:00")) - dt.datetime.fromisoformat(args.start.replace("Z", "+00:00"))).total_seconds() / 3600
    new_stories = [d for d, at in opened.items() if args.start <= at[:19] + "Z" < args.end]
    outlets = {}
    for i, cid in enumerate(ids):
        for doc in stories.get(cid) or {}:
            outlets.setdefault(doc, set()).add(registered_domain(items[i].get("url", "")))
    production = {
        "captures_per_hour": round(len(items) / hours, 1),
        "new_stories_per_hour": round(len(new_stories) / hours, 1),
        "captures_per_new_story": round(len(items) / len(new_stories), 2) if new_stories else None,
        "pages_per_new_story": round(len({x.get("url") for x in items}) / len(new_stories), 2) if new_stories else None,
        "source_schedules": len({x.get("source") for x in items} - {None}),
        "stories_receiving_captures": len(outlets),
        "intake_outlets_per_story": round(sum(len(v) for v in outlets.values()) / len(outlets), 2) if outlets else None,
        "share_of_stories_with_2_plus_intake_outlets": round(sum(1 for v in outlets.values() if len(v) >= 2) / len(outlets), 3) if outlets else None,
    }

    result = {
        "window": {"start": args.start, "end": args.end},
        "judge": {"model": MODEL, "registration": "docs/evidence/l2a-judge-registration-2026-10-14.md", "calls_not_served_by_registered_model": misrouted},
        "captures": len(items),
        "recaptures": sum(1 for x in items if x.get("recapture")),
        "production": production,
        "labels": label_coverage(c, args.start, args.end),
        "captures_with_story": sum(1 for cid in ids if stories.get(cid)),
        "story_documents_read": len({d for docs in stories.values() for d in docs}),
        "captures_grouped_by_judge": len(grouped),
        "same_event_pairs": len(agreed),
        "disputed_pairs": len(disputed),
        "cross_outlet_same_event_pairs": len(cross),
        "merged_cross_outlet_pairs": len(merged),
        "merge_recall": round(len(merged) / len(cross), 3) if cross else None,
        "merge_recall_first_story": round(len(merged_first) / len(cross), 3) if cross else None,
        "false_merge_candidates": len(false_candidates),
        "pairs": [{"a": lines[a], "b": lines[b], "a_id": ids[a], "b_id": ids[b], "same_story": (a, b) in set(merged),
                   "a_stories": sorted(stories.get(ids[a]) or {}), "b_stories": sorted(stories.get(ids[b]) or {})} for a, b in sorted(cross)],
        "false_merge_candidate_pairs": false_candidates,
    }
    json.dump(result, open(args.out, "w"), indent=1, ensure_ascii=False)
    print(json.dumps({k: v for k, v in result.items() if k not in ("pairs", "false_merge_candidate_pairs")}))


if __name__ == "__main__":
    main()
