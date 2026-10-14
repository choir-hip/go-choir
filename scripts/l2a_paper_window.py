#!/usr/bin/env python3
"""Score one window of a paper computer's intake (station L2a).

Reads, through the owner's API, the stories opened or revised in a UTC
window and the published version of each, and reports:
  - outlets per published version (distinct registered domains of the
    version's source URIs; wire copies on one domain count once);
  - the share of versions citing two or more outlets;
  - management feeds (Notes naming a capture) committed in the window.

The denominator of same-event, cross-outlet capture pairs comes from the
registered judge over the window's captures; that step is added with the
judge (S0).

The same script scores the baseline and the after-window, so the two are
comparable. Credentials come from a JSON file with api_key and
computer_id; the key is never printed.

usage: l2a_paper_window.py --cred FILE --start ISO --end ISO --out FILE
"""
import argparse
import json
import re
import time
import urllib.error
import urllib.parse
import urllib.request

HOST = "https://choir.news"
TWO_LEVEL_SUFFIXES = {"co.uk", "com.au", "co.jp", "com.tw", "org.uk", "com.br", "co.kr", "com.cn", "ne.jp", "or.jp", "gov.uk", "ac.uk"}


def registered_domain(uri):
    host = urllib.parse.urlparse(uri).hostname or ""
    parts = host.lower().split(".")
    if len(parts) >= 3 and ".".join(parts[-2:]) in TWO_LEVEL_SUFFIXES:
        return ".".join(parts[-3:])
    return ".".join(parts[-2:])


class Client:
    def __init__(self, cred):
        self.key = cred.get("api_key") or cred.get("key")
        self.computer = cred.get("computer_id")

    def get(self, path, auth=True):
        headers = {"Authorization": "Bearer " + self.key, "X-Choir-Computer": self.computer} if auth else {}
        for attempt in range(10):
            try:
                req = urllib.request.Request(HOST + path, headers=headers)
                return json.load(urllib.request.urlopen(req, timeout=60))
            except urllib.error.HTTPError as err:
                if err.code in (502, 503, 504):
                    time.sleep(15)
                    continue
                raise
        raise RuntimeError("unavailable: " + path)


def in_window(ts, start, end):
    return bool(ts) and start <= ts[:19] < end


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--cred", required=True)
    ap.add_argument("--start", required=True, help="UTC, e.g. 2026-10-13T15:00:00")
    ap.add_argument("--end", required=True)
    ap.add_argument("--out", required=True)
    args = ap.parse_args()
    start, end = args.start[:19], args.end[:19]
    c = Client(json.load(open(args.cred)))

    docs = c.get("/api/texture/documents?limit=500")
    docs = docs.get("documents") or docs.get("items") or docs
    stories, feeds = [], []
    for d in docs:
        doc_id = d.get("doc_id") or d.get("id")
        recs = c.get("/api/inboxes/" + urllib.parse.quote("texture:" + doc_id, safe="") + "/records?limit=200").get("records") or []
        from_mgmt = [r for r in recs if str(r.get("sender_inbox", "")).startswith("management:")]
        if not any((r.get("directive") or {}).get("subtype") == "cast" for r in from_mgmt):
            continue  # not a story
        for r in from_mgmt:
            if (r.get("directive") or {}).get("subtype") != "note":
                continue
            body = json.dumps(r.get("directive") or {})
            caps = sorted(set(re.findall(r"choir\.web_capture:[A-Za-z0-9:_\-]+", body)))
            if caps and in_window((r.get("provenance") or {}).get("committed_at", ""), start, end):
                feeds.append({"doc_id": doc_id, "captures": len(caps)})
        opened, updated = d.get("created_at", ""), d.get("updated_at", "")
        if not (in_window(opened, start, end) or in_window(updated, start, end)):
            continue
        story = {"doc_id": doc_id, "title": d.get("title", ""), "opened_in_window": in_window(opened, start, end)}
        pub = c.get("/api/platform/texture/publications?doc_id=" + urllib.parse.quote(doc_id))
        route = ((pub or {}).get("route") or {}).get("bare")
        if route:
            ver = c.get("/api/platform/publications/resolve?route=" + urllib.parse.quote(route), auth=False)
            uris = [((e.get("target") or {}).get("uri") or "") for e in (ver.get("artifact") or {}).get("source_entities") or []]
            outlets = sorted({registered_domain(u) for u in uris if u.startswith("http")})
            story.update({"version": (ver.get("version") or {}).get("revision"), "outlets": outlets})
        else:
            story.update({"version": None, "outlets": []})
        stories.append(story)

    published = [s for s in stories if s["version"] is not None]
    multi = [s for s in published if len(s["outlets"]) >= 2]
    result = {
        "window": {"start": start, "end": end},
        "stories_touched": len(stories),
        "stories_opened": sum(1 for s in stories if s["opened_in_window"]),
        "published_versions": len(published),
        "versions_citing_2plus_outlets": len(multi),
        "share_2plus_outlets": round(len(multi) / len(published), 3) if published else None,
        "feeds_in_window": len(feeds),
        "stories": stories,
    }
    json.dump(result, open(args.out, "w"), indent=1, ensure_ascii=False)
    print(json.dumps({k: v for k, v in result.items() if k != "stories"}))


if __name__ == "__main__":
    main()
