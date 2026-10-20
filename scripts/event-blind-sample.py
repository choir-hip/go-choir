#!/usr/bin/env python3
"""Blind sample for the event layer's false-merge bar (design 2026-10-16 s9, s19.2;
docs/problems/event-layer-false-merges-at-marginal-p-2026-10-16.md).

Takes the joined (counted, non-opening) memberships resolved since --since from
GET /api/autopaper/events on each named test computer, keeps one per
(event, member URL), samples --n of them pooled, and asks non-Claude judges
whether the opener and the member report the same event. A judge sees only the
two titles and URLs, never Jev's p or model. A false merge counts only when
every judge says DIFFERENT. The bar is a false-merge rate <= 5%.

The definition below is the resolver's (eventDefinition in
internal/agentcore/event_layer.go) verbatim; TestBlindSampleScriptCarriesTheResolverDefinition
fails if they drift. Do not edit one without the other.

Usage:
  scripts/event-blind-sample.py --since 2026-10-20T18:00:00Z [--n 40] [--out DIR] LABEL [LABEL ...]

LABEL names ~/.config/choir-qa/test-computers/LABEL.json (api_key, computer_id).
The OpenRouter key is read from $OPENROUTER_API_KEY, else .env in the repo root.
Nothing secret is printed or written. Reads staging; writes only --out.
"""
import argparse
import json
import os
import random
import re
import sys
import urllib.request

EVENT_DEFINITION = "One event is one incident, in one place, at one time: the occurrence itself, however many outlets report it and however its details are updated. A later development of it (a reaction, a response, an investigation, an outcome), another incident of the same story, place or actors, a live blog or roundup spanning several incidents, and an opinion or analysis piece about a topic are each a different event."
BAR = 0.05
DEFAULT_MODELS = "openai/gpt-6.1-sol,google/gemini-3.8-flash"
API = "https://choir.news"


def load_events(label):
    cfg = json.load(open(os.path.expanduser("~/.config/choir-qa/test-computers/%s.json" % label)))
    req = urllib.request.Request(
        API + "/api/autopaper/events?limit=300",
        headers={"Authorization": "Bearer " + cfg["api_key"], "X-Choir-Computer": cfg["computer_id"]},
    )
    return json.load(urllib.request.urlopen(req, timeout=60))["events"]


def distinct_joins(events, since):
    """One (event, member URL) per counted join resolved at or after since."""
    out, seen = [], set()
    for e in events:
        members = e["member_list"]
        opener = [m for m in members if m.get("opened")] or members[:1]
        if not opener:
            continue
        opener = opener[0]
        for m in members:
            if m is opener or m.get("tentative"):
                continue
            if (m.get("resolved_at") or m.get("captured_at") or "") < since:
                continue
            key = (e["event_id"], m["url"])
            if m["url"] == opener["url"] or key in seen:
                continue
            seen.add(key)
            out.append((e["event_id"], opener, m))
    return out


def openrouter_key():
    if os.environ.get("OPENROUTER_API_KEY"):
        return os.environ["OPENROUTER_API_KEY"]
    root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    for line in open(os.path.join(root, ".env")):
        mm = re.match(r'\s*(?:export\s+)?OPENROUTER_API_KEY\s*=\s*"?([^"\s]+)', line)
        if mm:
            return mm.group(1)
    sys.exit("no OPENROUTER_API_KEY in the environment or .env")


def judge(key, model, a, b):
    prompt = (
        "Two news items. Do they report the same real-world event? " + EVENT_DEFINITION +
        " Answer with one word: SAME, DIFFERENT, or UNSURE.\n\n"
        "A: %s\n   %s\nB: %s\n   %s" % (a["title"], a["url"], b["title"], b["url"])
    )
    body = {"model": model, "messages": [{"role": "user", "content": prompt}], "max_tokens": 400}
    req = urllib.request.Request(
        "https://openrouter.ai/api/v1/chat/completions",
        data=json.dumps(body).encode(),
        headers={"Authorization": "Bearer " + key, "Content-Type": "application/json"},
    )
    text = json.load(urllib.request.urlopen(req, timeout=120))["choices"][0]["message"]["content"] or ""
    w = re.search(r"\b(SAME|DIFFERENT|UNSURE)\b", text.upper())
    return w.group(1) if w else "UNPARSED"


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("labels", nargs="+")
    ap.add_argument("--since", required=True, help="only joins resolved at or after this UTC time (RFC3339): the deploy time of the change under test, so the window is fresh")
    ap.add_argument("--n", type=int, default=40)
    ap.add_argument("--models", default=DEFAULT_MODELS)
    ap.add_argument("--seed", type=int, default=20261020)
    ap.add_argument("--out", default=".")
    args = ap.parse_args()
    models = args.models.split(",")
    key = openrouter_key()

    pool = []
    for label in args.labels:
        joins = distinct_joins(load_events(label), args.since)
        print("%s: %d distinct-URL joins since %s" % (label, len(joins), args.since))
        pool += [(label,) + j for j in joins]
    random.seed(args.seed)
    sample = random.sample(pool, min(args.n, len(pool)))
    if len(sample) < args.n:
        print("WARNING: sample of %d is under the %d the bar assumes; report it as short, do not pad with older joins" % (len(sample), args.n))

    rows, both = [], {}
    tally = {m: {} for m in models}
    per = {l: [0, 0] for l in args.labels}  # [all-judges-DIFFERENT, sampled]
    for label, ev, a, b in sample:
        verdicts = []
        for m in models:
            try:
                v = judge(key, m, a, b)
            except Exception as e:  # a judge error is recorded, never counted as DIFFERENT
                v = "ERROR"
            tally[m][v] = tally[m].get(v, 0) + 1
            verdicts.append(v)
        bad = all(v == "DIFFERENT" for v in verdicts)
        per[label][1] += 1
        per[label][0] += bad
        rows.append({"computer": label, "event": ev, "verdicts": verdicts, "a": a["title"][:120], "a_url": a["url"],
                     "b": b["title"][:120], "b_url": b["url"], "p": b.get("p"), "all_different": bad})
        print("%-15s %-6s %s || %s" % ("/".join(v[:4] for v in verdicts), label[:6], a["title"][:55], b["title"][:55]))
    n_bad = sum(r["all_different"] for r in rows)
    n = max(1, len(rows))
    for m in models:
        print(m, tally[m], "DIFFERENT rate", round(tally[m].get("DIFFERENT", 0) / n, 3))
    for label, (x, y) in per.items():
        print("computer %s: %d of %d all-judges-DIFFERENT" % (label, x, y))
    rate = n_bad / n
    print("sample %d, all judges DIFFERENT %d, false-merge rate %.3f, bar %.2f -> %s" % (len(rows), n_bad, rate, BAR, "PASS" if rate <= BAR and len(rows) >= args.n else "FAIL (or short sample)"))
    print("Next: read every all_different row yourself (the tesla.com/roadster case is a correct member the title hides) and record both numbers.")
    os.makedirs(args.out, exist_ok=True)
    path = os.path.join(args.out, "event_blind_sample_%s.json" % args.since.replace(":", "").replace("-", ""))
    json.dump({"since": args.since, "models": models, "bar": BAR, "sample": len(rows), "false_merges": n_bad, "rows": rows}, open(path, "w"), indent=1)
    print("wrote", path)


if __name__ == "__main__":
    main()
