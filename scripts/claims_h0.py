#!/usr/bin/env python3
"""E0's historical slice (H0 in docs/research/historical-precommitment-backtesting-2026-10-14.md):
three FOMC statements and three company earnings releases from 2026, each
paired with the later primary document that settles it.

  1. extract claims from each "said" document with the E0 extractors;
  2. quote-check them (quotematch);
  3. recall against hand-written gold questions (the guided metrics and the
     Committee's forward-looking sentences), written from the documents
     before any extraction was read;
  4. settle by count where the question is numeric (guided range vs the
     reported actual, values transcribed from verbatim outcome quotes that
     the script re-finds in the saved outcome documents);
  5. settle every forward-looking extracted claim with a blind judge given
     only the outcome document, and with no evidence at all (closed-book
     control, the leakage measure of backtesting §1.5);
  6. report extraction recall, count coverage, judge-vs-count agreement,
     closed-book knowledge, and the leakage class reachable.

Documents live under DATA/historical (fetched from federalreserve.gov and
SEC EDGAR; manifest with sha256 and witness times). Results without text
go to docs/evidence/claims-e0/h0-result.json.
"""
import collections
import json
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import claims_lib as L  # noqa: E402
import claims_prompts as P  # noqa: E402
from claims_e0 import EVID, EXTRACTORS, pmap, short  # noqa: E402

H = os.path.join(L.DATA, "historical")
OUT = os.path.join(L.DATA, "h0")

# Gold questions, written from the said documents. kind=count: a numeric
# range settled against a reported actual; kind=judged: a forward-looking
# sentence whose settlement needs reading. pattern: a string a recalling
# claim's quote must contain. approx: "approximately X" is operationalized
# as X plus or minus 2% (versioned choice h0-approx-2pct, stated in results).
GOLD = {
    "nvda-1": [
        {"q": "nvda-rev", "pattern": "91.0 billion", "kind": "count", "lo": 89.18, "hi": 92.82, "actual": 96.221, "aq": "Revenue $96,221"},
        {"q": "nvda-gm-gaap", "pattern": "74.9%", "kind": "count", "lo": 74.4, "hi": 75.4, "actual": 75.0, "aq": None},
        {"q": "nvda-gm-nongaap", "pattern": "74.9%", "kind": "count", "lo": 74.5, "hi": 75.5, "actual": 75.0, "aq": None},
        {"q": "nvda-opex-gaap", "pattern": "8.5 billion", "kind": "count", "lo": 8.33, "hi": 8.67, "actual": 8.408, "aq": "Operating expenses $8,408"},
        {"q": "nvda-opex-nongaap", "pattern": "8.5 billion", "kind": "count", "lo": 8.134, "hi": 8.466, "actual": 8.232, "aq": "Operating expenses $8,232"},
        {"q": "nvda-fy-tax", "pattern": "16.0%", "kind": "pending", "note": "full fiscal 2027 ends Jan 2027"},
    ],
    "csco-1": [
        {"q": "csco-rev", "pattern": "16.7 billion", "kind": "count", "lo": 16.7, "hi": 16.9, "actual": 17.252, "aq": "fourth quarter revenue of $17.3 billion"},
        {"q": "csco-gm", "pattern": "65.5%", "kind": "count", "lo": 65.5, "hi": 66.5, "actual": 66.3, "aq": None},
        {"q": "csco-opm", "pattern": "34%", "kind": "count", "lo": 34.0, "hi": 35.0, "actual": 35.9, "aq": None},
        {"q": "csco-eps-nongaap", "pattern": "1.16", "kind": "count", "lo": 1.16, "hi": 1.18, "actual": 1.22, "aq": "$4.9 billion or $1.22 per share"},
        {"q": "csco-eps-gaap", "pattern": "0.80", "kind": "count", "lo": 0.80, "hi": 0.85, "actual": 0.97, "aq": "$3.9 billion or $0.97 per share"},
        {"q": "csco-fy-rev", "pattern": "62.8 billion", "kind": "count", "lo": 62.8, "hi": 63.0, "actual": 63.3, "aq": "Total revenue was $63.3 billion"},
        {"q": "csco-fy-eps-gaap", "pattern": "3.16", "kind": "count", "lo": 3.16, "hi": 3.21, "actual": 3.33, "aq": "EPS $ 3.33"},
        {"q": "csco-fy-eps-nongaap", "pattern": "4.27", "kind": "count", "lo": 4.27, "hi": 4.29, "actual": 4.33, "aq": "EPS $ 4.33"},
    ],
    "crm-1": [
        {"q": "crm-rev", "pattern": "11.27 billion", "kind": "count", "lo": 11.27, "hi": 11.35, "actual": 11.345, "aq": None},
        {"q": "crm-eps-gaap", "pattern": "1.74", "kind": "count", "lo": 1.74, "hi": 1.76, "actual": 4.29, "aq": None},
        {"q": "crm-eps-nongaap", "pattern": "3.25", "kind": "count", "lo": 3.25, "hi": 3.27, "actual": 5.90, "aq": None},
        {"q": "crm-fy-rev", "pattern": "45.9 billion", "kind": "pending", "note": "fiscal 2027 ends Jan 2027"},
        {"q": "crm-fy-ss", "pattern": "slightly under 12%", "kind": "pending", "note": "fiscal 2027"},
        {"q": "crm-fy-opm", "pattern": "34.3%", "kind": "pending", "note": "fiscal 2027"},
        {"q": "crm-fy-ocf", "pattern": "4% - 5%", "kind": "pending", "note": "fiscal 2027"},
    ],
    "fomc-1": [
        {"q": "fomc-apr-adjustments", "pattern": "extent and timing of additional adjustments", "kind": "judged",
         "hand": "contradicted under the easing-bias reading (next move a cut): June held, September raised; vacuous under the 'will assess data' reading"},
        {"q": "fomc-apr-prepared", "pattern": "would be prepared to adjust", "kind": "judged", "hand": "unscorable: conditional, no criterion"},
        {"q": "fomc-apr-committed", "pattern": "strongly committed to supporting maximum employment", "kind": "judged", "hand": "unscorable: stated value, no horizon"},
    ],
    "fomc-2": [
        {"q": "fomc-jun-reserves", "pattern": "maintaining ample reserves", "kind": "primary_record", "hand": "kept: July statement continues the ample-reserves policy"},
        {"q": "fomc-jun-price-stability", "pattern": "will deliver price stability", "kind": "judged", "hand": "unscorable: no horizon or criterion"},
    ],
    "fomc-3": [
        {"q": "fomc-jul-reserves", "pattern": "maintaining ample reserves", "kind": "primary_record", "hand": "kept: September statement continues it"},
        {"q": "fomc-jul-price-stability", "pattern": "will deliver price stability", "kind": "judged", "hand": "unscorable: no horizon or criterion"},
    ],
}
FORWARD = {"forecast", "commitment", "intent"}


def blocks_of(doc_id):
    text = open(os.path.join(H, doc_id + ".txt")).read()
    paras = [p.strip() for p in re.split(r"\n\s*\n", text) if p.strip()]
    return [{"id": f"{doc_id}#h{i}", "text": p} for i, p in enumerate(paras)]


def norm(s):
    return re.sub(r"\s+", " ", s.replace("‑", "-").replace("–", "-")).lower()


def main():
    variant = sys.argv[1] if len(sys.argv) > 1 else "v0"
    prompt = {"v0": P.EXTRACT_V0, "v1": P.EXTRACT_V1_FORWARD}[variant]
    os.makedirs(OUT, exist_ok=True)
    man = json.load(open(os.path.join(H, "manifest.json")))
    pairs = {p["pair_id"]: p for p in man["pairs"]}
    # the outcome quotes the count values rest on must be in the saved documents
    missing = []
    for pid, qs in GOLD.items():
        otext = norm(open(os.path.join(H, pairs[pid]["outcome"]["id"] + ".txt")).read())
        for q in qs:
            if q.get("aq") and norm(q["aq"]) not in otext:
                missing.append(q["q"])
        stext = norm(open(os.path.join(H, pairs[pid]["said"]["id"] + ".txt")).read())
        for q in qs:
            if norm(q["pattern"]) not in stext:
                missing.append("pattern:" + q["q"])
    docs = []
    for pid, p in pairs.items():
        bl = blocks_of(p["said"]["id"])
        cap = {"capture_id": p["said"]["id"], "url": p["said"]["url"], "title": p["said"]["id"], "fetched_at": p["said"]["witnessed_at"], "blocks": bl}
        docs.append((pid, cap))

    from claims_e0 import render
    ext = {}
    for m in EXTRACTORS:
        def one(item, m=m):
            pid, cap = item
            doc, ids = render(cap, maxchars=16000)
            r = L.omp(m, prompt + doc)
            try:
                e = L.parse_json(r["text"])
            except ValueError:
                e = {"claims": []}
            return {"pair": pid, "doc": cap["capture_id"], "ids": ids, "claims": [c for c in e.get("claims") or [] if isinstance(c, dict)],
                    "usage": r["usage"], "truncated": len(ids) < len(cap["blocks"])}
        ext[m] = pmap(one, docs, 6)

    items = []
    for m in EXTRACTORS:
        for r, (pid, cap) in zip(ext[m], docs):
            items.append({"key": f"{short(m)}|{pid}", "blocks": [{"id": b["id"], "text": b["text"]} for b in cap["blocks"]],
                          "quotes": [{"id": f"c{i}", "quote": str(c.get("quote") or ""), "hint": r["ids"].get(str(c.get("block") or ""), "")} for i, c in enumerate(r["claims"])]})
    ver = L.claimslab("verify", {"items": items})
    retained = {(x["key"], x["id"]): x["check"]["retained"] for x in ver["results"]}

    # settlement by judge, blind and closed-book, for forward-looking retained claims
    tasks = []
    for m in EXTRACTORS:
        for r in ext[m]:
            p = pairs[r["pair"]]
            for i, c in enumerate(r["claims"]):
                if c.get("modality") in FORWARD and retained.get((f"{short(m)}|{r['pair']}", f"c{i}")):
                    tasks.append((m, r["pair"], i, c, p))

    def settle(t):
        m, pid, i, c, p = t
        readings = c.get("readings") or [{"gloss": c.get("quote"), "weight": 1}]
        claim = json.dumps({"said_at": p["said"]["published_at"], "claim": c.get("quote"), "readings": [{"r": k + 1, "gloss": rd.get("gloss")} for k, rd in enumerate(readings) if isinstance(rd, dict)],
                            "deadline": c.get("deadline")}, ensure_ascii=False)
        evidence = open(os.path.join(H, p["outcome"]["id"] + ".txt")).read()[:30000]
        blind = L.omp(L.GEMINI, P.SETTLE_V0 + "\nCLAIM\n" + claim + f"\n\nEVIDENCE (published {p['outcome']['published_at']})\n" + evidence)
        closed = L.omp(L.GEMINI, P.CLOSED_BOOK_V0 + "\nCLAIM\n" + claim)

        def parse(r):
            try:
                return L.parse_json(r["text"]).get("readings") or []
            except ValueError:
                return None
        return {"extractor": short(m), "pair": pid, "claim_index": i, "modality": c.get("modality"), "quote": c.get("quote"),
                "blind": parse(blind), "closed_book": parse(closed)}

    settles = pmap(settle, tasks, 8)

    # recall and count settlement per gold question
    res = {"documents": {pid: {"said": p["said"]["id"], "outcome": p["outcome"]["id"], "witness_kind": p["said"]["witness_kind"],
                               "witnessed_at": p["said"]["witnessed_at"], "said_sha256": p["said"]["sha256"]} for pid, p in pairs.items()},
           "integrity_missing": missing, "extract_prompt": variant, "extract_prompt_sha256": L.sha(prompt), "matcher": ver["matcher"], "approx_rule": "h0-approx-2pct", "extractors": {}}
    for m in EXTRACTORS:
        e = {"claims": 0, "retained": 0, "forward_retained": 0, "by_pair": {}, "questions": {}}
        for r in ext[m]:
            pid = r["pair"]
            kept = [c for i, c in enumerate(r["claims"]) if retained.get((f"{short(m)}|{pid}", f"c{i}"))]
            e["claims"] += len(r["claims"])
            e["retained"] += len(kept)
            e["forward_retained"] += sum(1 for c in kept if c.get("modality") in FORWARD)
            e["by_pair"][pid] = {"claims": len(r["claims"]), "retained": len(kept), "truncated_input": r["truncated"],
                                 "modalities": dict(collections.Counter(c.get("modality") for c in kept))}
            for q in GOLD[pid]:
                hit = [i for i, c in enumerate(r["claims"]) if retained.get((f"{short(m)}|{pid}", f"c{i}")) and norm(q["pattern"]) in norm(str(c.get("quote") or ""))]
                row = {"pair": pid, "kind": q["kind"], "recalled": bool(hit)}
                if q["kind"] == "count":
                    row["count_outcome"] = "confirmed" if q["lo"] <= q["actual"] <= q["hi"] else "contradicted"
                    row["direction"] = "within" if row["count_outcome"] == "confirmed" else ("above" if q["actual"] > q["hi"] else "below")
                    js = [s for s in settles if s["extractor"] == short(m) and s["pair"] == pid and s["claim_index"] in hit]
                    if js and js[0]["blind"]:
                        row["judge_outcome"] = js[0]["blind"][0].get("outcome")
                    if js and js[0]["closed_book"]:
                        row["closed_book"] = js[0]["closed_book"][0].get("outcome")
                    if hit and not js:
                        row["judge_outcome"] = "not_judged (extracted modality not forward-looking)"
                else:
                    row["hand_settlement"] = q.get("hand") or q.get("note")
                    js = [s for s in settles if s["extractor"] == short(m) and s["pair"] == pid and s["claim_index"] in hit]
                    if js and js[0]["blind"]:
                        row["judge_outcome"] = js[0]["blind"][0].get("outcome")
                        row["closed_book"] = (js[0]["closed_book"] or [{}])[0].get("outcome")
                e["questions"][q["q"]] = row
        sm = [s for s in settles if s["extractor"] == short(m)]
        e["judged_forward_claims"] = len(sm)
        e["blind_outcomes"] = dict(collections.Counter((s["blind"] or [{}])[0].get("outcome") for s in sm))
        e["closed_book_outcomes"] = dict(collections.Counter((s["closed_book"] or [{}])[0].get("outcome") for s in sm))
        cnt = [q for q in e["questions"].values() if q["kind"] == "count"]
        e["summary"] = {
            "gold_questions": len(e["questions"]),
            "recall_all": round(sum(q["recalled"] for q in e["questions"].values()) / len(e["questions"]), 3),
            "count_questions": len(cnt),
            "recall_count": round(sum(q["recalled"] for q in cnt) / len(cnt), 3),
            "judge_vs_count_agree": sum(1 for q in cnt if q.get("judge_outcome") == q["count_outcome"]),
            "judge_vs_count_compared": sum(1 for q in cnt if q.get("judge_outcome") in ("confirmed", "contradicted", "qualified", "unresolved")),
            "closed_book_knew": sum(1 for q in cnt if q.get("closed_book") in ("confirmed", "contradicted", "qualified")),
            "closed_book_right": sum(1 for q in cnt if q.get("closed_book") == q["count_outcome"]),
            "cost_list_usd": round(sum(L.list_cost(m, r["usage"]) or 0 for r in ext[m]), 5),
        }
        res["extractors"][short(m)] = e
    # the box-score shape this slice supports: count-settled guidance per company
    box = {}
    for pid, qs in GOLD.items():
        cq = [q for q in qs if q["kind"] == "count"]
        if cq:
            box[pairs[pid]["actor"]] = {"guided_items_settled": len(cq), "within_range": sum(1 for q in cq if q["lo"] <= q["actual"] <= q["hi"]),
                                        "above": sum(1 for q in cq if q["actual"] > q["hi"]), "below": sum(1 for q in cq if q["actual"] < q["lo"]),
                                        "pending": sum(1 for q in qs if q["kind"] == "pending"), "leakage_class": "L0 (count over SEC-timestamped primary records)"}
    res["guidance_box"] = box
    json.dump({"settles": settles, "extractions": {short(m): ext[m] for m in EXTRACTORS}}, open(os.path.join(OUT, f"h0-raw-{variant}.json"), "w"), ensure_ascii=False)
    json.dump(res, open(os.path.join(EVID, "h0-result.json" if variant == "v0" else f"h0-result-{variant}.json"), "w"), indent=1, ensure_ascii=False)
    print(json.dumps({k: v["summary"] for k, v in res["extractors"].items()}, indent=1))
    print("integrity missing:", missing)
    print(json.dumps(box, indent=1))


if __name__ == "__main__":
    main()
