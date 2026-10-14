#!/usr/bin/env python3
"""Score an L2a fixture run against the labels (acceptance action 1).

The labels file lives outside the repo (the desks never see it). It maps
each labelled event to its captures by link, plus singletons. This reads,
from the test computer that polled the fixture:
  - which story each fixture capture went into (the cast that opened it or
    a Note that fed it);
  - which captures management declined;
  - each story's published version and the outlets and languages it cites.

It reports, against the bar that is not lowered:
  - stories per event (exactly one each);
  - merge recall over same-event capture pairs (>= 0.8);
  - false merges, captures of different events in one story (<= 1);
  - captures without a disposition (0);
  - per event, the published version cites >= 2 languages and >= 2 outlets.

usage: l2a_fixture_score.py --cred FILE --labels FILE --out FILE
"""
import argparse
import itertools
import json
import os
import sys
import urllib.parse

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from l2a_window_pairs import CAPTURE_ID, Client, norm_id, registered_domain, stories_by_capture  # noqa: E402

BAR = {"merge_recall": 0.8, "false_merges": 1}


def norm(url):
    p = urllib.parse.urlparse(url or "")
    # Feeds percent-encode non-Latin paths; the labels may not.
    return (p.hostname or "").lower() + urllib.parse.unquote(p.path).rstrip("/")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--cred", required=True)
    ap.add_argument("--labels", required=True)
    ap.add_argument("--out", required=True)
    args = ap.parse_args()
    c = Client(json.load(open(args.cred)))
    labels = json.load(open(args.labels))

    label_of, lang_of, outlet_of = {}, {}, {}
    for event, members in labels["events"].items():
        for m in members:
            label_of[norm(m["link"])] = event
            lang_of[norm(m["link"])] = m["lang"]
            outlet_of[norm(m["link"])] = m["outlet"]
    for i, m in enumerate(labels["singletons"]):
        label_of[norm(m["link"])] = "S%d" % i

    # The arrivals are the source's captures records in management's inbox
    # (sender source:<fixture>), not every capture whose link matches: the
    # desks re-capture the same pages while researching.
    owner = None
    for st in c.get("/api/computer-settings").get("settings") or []:
        owner = owner or st.get("owner_id")
    mgmt = "management:" + owner
    key_of = {}  # arrival capture id -> normalized link
    declined = set()
    recs = c.get("/api/inboxes/" + urllib.parse.quote(mgmt, safe="") + "/records?limit=500").get("records") or []
    for inbox in [mgmt] + ["source:" + labels.get("source_id", "fixture:l2a_multilingual_2026_10_14")]:
        rs = recs if inbox == mgmt else c.get("/api/inboxes/" + urllib.parse.quote(inbox, safe="") + "/records?limit=500").get("records") or []
        for r in rs:
            if str(r.get("sender_inbox", "")).startswith("source:"):
                for item in (r.get("captures") or {}).get("items") or []:
                    k = norm(item.get("url"))
                    if k in label_of:
                        key_of[norm_id(item["capture_id"])] = k
            disp = r.get("disposition") or {}
            if disp.get("kind") == "decline":
                declined |= {norm_id(x) for x in disp.get("captures") or []}

    story_of = stories_by_capture(c)

    by_label = {}
    for cid, k in key_of.items():
        by_label.setdefault(label_of[k], []).append(cid)

    events = {}
    same_pairs = merged = 0
    for event, cids in by_label.items():
        if event.startswith("S"):
            continue
        stories = sorted({story_of[c_] for c_ in cids if c_ in story_of})
        for a, b in itertools.combinations(cids, 2):
            same_pairs += 1
            if story_of.get(a) and story_of.get(a) == story_of.get(b):
                merged += 1
        cited = {"languages": set(), "outlets": set()}
        for doc in stories:
            pub = c.get("/api/platform/texture/publications?doc_id=" + urllib.parse.quote(doc))
            route = ((pub or {}).get("route") or {}).get("bare")
            if not route:
                continue
            ver = c.get("/api/platform/publications/resolve?route=" + urllib.parse.quote(route))
            for e in (ver.get("artifact") or {}).get("source_entities") or []:
                k = norm((e.get("target") or {}).get("uri"))
                if k in lang_of:
                    cited["languages"].add(lang_of[k])
                    cited["outlets"].add(outlet_of[k])
                elif k:
                    cited["outlets"].add(registered_domain("https://" + k))
        events[event] = {
            "captures": len(cids),
            "stories": stories,
            "cited_languages": sorted(cited["languages"]),
            "cited_outlets": sorted(cited["outlets"]),
            "ok": len(stories) == 1 and len(cited["languages"]) >= 2 and len(cited["outlets"]) >= 2,
        }

    false_merges = []
    members = {}
    for cid, doc in story_of.items():
        if cid in key_of:
            members.setdefault(doc, []).append(cid)
    for doc, cids in members.items():
        for a, b in itertools.combinations(cids, 2):
            la, lb = label_of[key_of[a]], label_of[key_of[b]]
            if la != lb:
                false_merges.append({"doc_id": doc, "a": la, "b": lb})

    undisposed = [label_of[k] for cid, k in key_of.items() if cid not in story_of and cid not in declined]
    recall = round(merged / same_pairs, 3) if same_pairs else None
    result = {
        "fixture_captures": len(key_of),
        "labelled_links": len(label_of),
        "merge_recall": recall,
        "same_event_pairs": same_pairs,
        "false_merges": len(false_merges),
        "undisposed": undisposed,
        "events": events,
        "false_merge_pairs": false_merges,
        "bar": BAR,
        "pass": bool(
            recall is not None and recall >= BAR["merge_recall"]
            and len(false_merges) <= BAR["false_merges"]
            and not undisposed
            and all(e["ok"] for e in events.values())
            and len(key_of) == len(label_of)
        ),
    }
    json.dump(result, open(args.out, "w"), indent=1, ensure_ascii=False)
    print(json.dumps({k: v for k, v in result.items() if k not in ("events", "false_merge_pairs")}))


if __name__ == "__main__":
    main()
