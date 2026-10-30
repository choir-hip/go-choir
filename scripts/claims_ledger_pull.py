#!/usr/bin/env python3
"""Read-only pull of a paper computer's published stories, desk records and
captures, for the claims layer's offline study (E0) and the desks' say-do
eval (E6a). docs/archive/reports/claims-e0-e6a-results-2026-10-14.md.

Reads, through the owner's API and the public publication routes:
  - every published story on the front (paginated), resolved to its
    current published version (text, citations, checks, source entities);
  - each story's texture:<doc> and research:<doc> inbox records (casts,
    reports, support verdicts, management replies);
  - every capture those records and versions name, plus the computer's
    newest captures (GET /api/captures lists the newest 500), each with
    its blocks.

Writes nothing to the computer. The output holds article text, so it goes
outside the repo (default ~/.config/choir-qa/traces/claims/pull). The key
is read from the credential file and never printed.

usage: claims_ledger_pull.py --cred FILE [--out DIR] [--workers N]
"""
import argparse
import concurrent.futures as cf
import datetime as dt
import json
import os
import re
import time
import urllib.error
import urllib.parse
import urllib.request

HOST = "https://choir.news"
CAPTURE_ID = re.compile(r"obj:choir\.web_capture:[A-Za-z0-9_\-]+:key-[0-9a-f]{64}")
BLOCK_DOC = re.compile(r"\bp-([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})-")


class Client:
    def __init__(self, cred):
        self.key = cred["api_key"]
        self.computer = cred["computer_id"]

    def get(self, path, auth=True, timeout=180):
        headers = {"Authorization": "Bearer " + self.key, "X-Choir-Computer": self.computer} if auth else {}
        last = None
        for attempt in range(6):
            try:
                req = urllib.request.Request(HOST + path, headers=headers)
                return json.load(urllib.request.urlopen(req, timeout=timeout))
            except urllib.error.HTTPError as err:
                last = f"HTTP {err.code}"
                if err.code in (429, 502, 503, 504):
                    time.sleep(10 * (attempt + 1))
                    continue
                raise RuntimeError(f"{last} {path[:120]}")
            except (urllib.error.URLError, TimeoutError) as err:
                last = str(err)
                time.sleep(10 * (attempt + 1))
        raise RuntimeError(f"unavailable ({last}): {path[:120]}")


def inbox_records(c, inbox):
    out, after = [], 0
    for _ in range(50):
        page = c.get("/api/inboxes/" + urllib.parse.quote(inbox, safe="") + f"/records?after={after}&limit=1000")
        out.extend(page.get("records") or [])
        after = page.get("next_after") or 0
        if not after:
            break
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--cred", required=True)
    ap.add_argument("--out", default=os.path.expanduser("~/.config/choir-qa/traces/claims/pull"))
    ap.add_argument("--workers", type=int, default=4)
    args = ap.parse_args()
    os.makedirs(args.out, exist_ok=True)
    c = Client(json.load(open(args.cred)))
    started = dt.datetime.now(dt.timezone.utc).isoformat()

    front, cursor = [], ""
    for _ in range(200):
        page = c.get("/api/platform/publications/front" + ("?cursor=" + urllib.parse.quote(cursor) if cursor else ""), auth=False)
        front.extend(page.get("stories") or [])
        cursor = page.get("next_cursor")
        if not cursor:
            break
    print(f"front: {len(front)} published stories", flush=True)

    cache = os.path.join(args.out, "stories")
    os.makedirs(cache, exist_ok=True)

    def story(s):
        path = os.path.join(cache, s["publication_id"] + ".json")
        if os.path.exists(path):
            return json.load(open(path))
        try:
            ver = c.get("/api/platform/publications/resolve?route=" + urllib.parse.quote(s["route"]["bare"]), auth=False)
        except Exception as err:  # recorded; a rerun retries it
            return {"front": s, "doc_id": None, "version": {}, "texture_records": [], "research_records": [], "errors": [f"resolve: {err}"]}
        docs = BLOCK_DOC.findall(json.dumps(ver.get("payload") or {}))
        doc_id = max(set(docs), key=docs.count) if docs else None
        rec = {"front": s, "doc_id": doc_id, "version": ver, "texture_records": [], "research_records": [], "errors": []}
        if doc_id:
            for name in ("texture", "research"):
                try:
                    rec[name + "_records"] = inbox_records(c, f"{name}:{doc_id}")
                except Exception as err:  # recorded, not fatal
                    rec["errors"].append(f"{name}: {err}")
        if not rec["errors"]:
            json.dump(rec, open(path, "w"), ensure_ascii=False)
        return rec

    stories = []
    with cf.ThreadPoolExecutor(args.workers) as pool:
        for i, rec in enumerate(pool.map(story, front)):
            stories.append(rec)
            if i % 25 == 0:
                print(f"stories: {i + 1}/{len(front)}", flush=True)
    json.dump(stories, open(os.path.join(args.out, "stories.json"), "w"), ensure_ascii=False)

    ids = set()
    for rec in stories:
        ids.update(CAPTURE_ID.findall(json.dumps(rec, ensure_ascii=False)))
    newest = c.get("/api/captures?since=" + urllib.parse.quote((dt.datetime.now(dt.timezone.utc) - dt.timedelta(days=30)).strftime("%Y-%m-%dT%H:%M:%SZ")))
    window = [x["capture_id"] for x in newest.get("captures") or []]
    captures = {x["capture_id"]: x for x in newest.get("captures") or [] if x.get("blocks")}
    todo = sorted(ids - set(captures))
    print(f"captures: {len(window)} newest listed, {len(todo)} more named by stories", flush=True)
    future = (dt.datetime.now(dt.timezone.utc) + dt.timedelta(days=365)).strftime("%Y-%m-%dT%H:%M:%SZ")

    def batch(chunk):
        q = "&".join("id=" + urllib.parse.quote(i, safe="") for i in chunk)
        return c.get(f"/api/captures?since={future}&{q}").get("captures") or []

    chunks = [todo[i:i + 40] for i in range(0, len(todo), 40)]
    with cf.ThreadPoolExecutor(max(1, args.workers // 2)) as pool:
        for got in pool.map(batch, chunks):
            for x in got:
                captures[x["capture_id"]] = x
    json.dump({"started_at": started, "newest_window": window, "captures": captures},
              open(os.path.join(args.out, "captures.json"), "w"), ensure_ascii=False)
    print(json.dumps({"stories": len(stories), "with_doc": sum(1 for s in stories if s["doc_id"]),
                      "captures": len(captures), "with_blocks": sum(1 for x in captures.values() if x.get("blocks"))}))


if __name__ == "__main__":
    main()
