#!/usr/bin/env python3
"""E6a: the say-do record of Choir's own desks on the paper, from existing
ledger data, read-only (docs/reports/claims-events-entities-ontology-2026-10-14.md
§7, §12.3; results in docs/archive/reports/claims-e0-e6a-results-2026-10-14.md).

Input: a pull by claims_ledger_pull.py (published stories, their texture
and research inbox records, the captures they name). Nothing is written to
the computer and no score enters any desk's context (I26).

Say-do pairs, each settled by the cheapest independent method:

  management  M1  opening-Cast hypotheses (statement, probability) settled
                  by a blind judge (gemini-3.8-flash, never shown the
                  probability) from research's reported claims and the
                  published version's support-checked quotes; scored with
                  Brier and the Murphy decomposition; P4's rule
                  "unresolvable counts as wrong" applies, and the
                  resolved-only score is shown beside it.
  research    R1  every verbatim quote research hands Texture (text_quote
                  selectors on captured sources) checked against the capture
                  with quotematch: count, no model.
              R2  final Reports whose stop_reason says "completed" versus
                  how many of Texture's questions the report answers (blind
                  judge, on a sample).
  texture     T1  every citation in the current published versions: Texture
                  asserts the passage supports its sentence; the support
                  judge (Jev, system:support) and the quote check settle it.
  engineering     the paper has no engineering assignments: n = 0.

usage: claims_e6a.py [--pull DIR] [--workers N] [--coverage-sample N]
"""
import argparse
import collections
import json
import os
import random
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import claims_lib as L  # noqa: E402
import claims_prompts as P  # noqa: E402
from claims_e0 import pmap  # noqa: E402

OUT = os.path.join(L.DATA, "e6a")
EVID = os.path.join(L.REPO, "docs", "evidence", "claims-e6a")
YS = {"confirmed": 1.0, "qualified": 0.5, "contradicted": 0.0, "unresolved": 0.0}


def packet(rec):
    try:
        return json.loads((rec.get("prediction") or {}).get("hypothesis") or "")
    except ValueError:
        return None


def m1(stories, workers, model=L.GEMINI, sample=None):
    tasks = []
    for s in stories:
        hyps = []
        for r in s["texture_records"]:
            if str(r.get("sender_inbox", "")).startswith("management:") and r.get("cast"):
                for h in r["cast"].get("hypotheses") or []:
                    if isinstance(h.get("probability"), (int, float)) and h.get("statement"):
                        hyps.append({"record_id": r["record_id"], "id": h.get("id"), "statement": h["statement"], "p": float(h["probability"]),
                                     "committed_at": (r.get("provenance") or {}).get("committed_at"),
                                     "round": r.get("round"), "agent": (r.get("provenance") or {}).get("agent_id", "").split(":")[0]})
        if not hyps:
            continue
        withdrawn = set()
        for r in s["texture_records"]:
            for w in ((r.get("cast") or {}).get("withdrawals") or []):
                withdrawn.add(w.get("id"))
        claims = []
        for r in s["texture_records"]:
            if str(r.get("sender_inbox", "")).startswith("research:") and r.get("report"):
                claims += [c.get("text", "") for c in r["report"].get("claims") or []]
        quotes = [c["ref"].get("quote") for c in ((s.get("version") or {}).get("checks") or {}).get("citations") or []
                  if c.get("judgement") == "supported" and (c.get("ref") or {}).get("quote")]
        tasks.append({"doc_id": s["doc_id"], "pub": s["front"]["publication_id"], "section": s["front"].get("section"),
                      "hyps": hyps, "withdrawn": sorted(withdrawn), "claims": claims, "quotes": quotes})

    def one(t):
        if not t["claims"] and not t["quotes"]:
            return {"verdicts": [], "no_evidence": True}
        lines = "\n".join(f"{i}. {h['statement']}" for i, h in enumerate(t["hyps"], 1))
        ev = "RESEARCH FINDINGS\n" + "\n".join("- " + c for c in t["claims"][:40]) + "\n\nSUPPORT-CHECKED SOURCE QUOTES IN THE PUBLISHED STORY\n" + "\n".join("- " + q for q in t["quotes"][:40])
        r = L.omp(model, P.HYPOTHESIS_SETTLE_V0 + "\nHYPOTHESES\n" + lines + "\n\nEVIDENCE\n" + ev[:24000])
        try:
            return {"verdicts": L.parse_json(r["text"]).get("verdicts") or [], "usage": r["usage"], "served": r["served"]}
        except ValueError:
            return {"verdicts": None, "raw": r["text"][:300]}

    if sample:
        rng = random.Random(20261016)
        rng.shuffle(tasks)
        tasks = tasks[:sample]
    print("M1 judge calls:", model, len(tasks), flush=True)
    res = pmap(one, tasks, workers)
    rows = []
    for t, r in zip(tasks, res):
        vs = {int(v.get("n", 0)): v for v in (r.get("verdicts") or []) if isinstance(v, dict)}
        for i, h in enumerate(t["hyps"], 1):
            v = vs.get(i, {})
            out = v.get("outcome") if v.get("outcome") in YS else ("no_evidence" if r.get("no_evidence") else "unresolved")
            rows.append({**{k: h[k] for k in ("record_id", "id", "p", "committed_at", "agent")}, "doc_id": t["doc_id"], "section": t["section"],
                         "outcome": out, "judge_p": v.get("p"), "withdrawn": h["id"] in t["withdrawn"]})
    return rows, res


def r1(stories, captures):
    items, meta = {}, []
    for s in stories:
        for r in s["texture_records"]:
            if not (str(r.get("sender_inbox", "")).startswith("research:") and r.get("report")):
                continue
            pk = packet(r)
            if not isinstance(pk, dict):
                continue
            for src in pk.get("sources") or []:
                if not isinstance(src, dict):
                    continue
                cid = ((src.get("target") or {}).get("capture_id") or "").strip()
                for k, sel in enumerate(src.get("selectors") or []):
                    if not isinstance(sel, dict) or sel.get("kind") != "text_quote" or not sel.get("quote"):
                        continue
                    key = f"{r['record_id']}|{src.get('source_id')}|{k}"
                    cap = captures.get(cid)
                    meta.append({"key": key, "doc_id": s["doc_id"], "capture_id": cid, "has_capture_id": bool(cid),
                                 "capture_readable": bool(cap and cap.get("blocks")), "final": bool(r.get("final"))})
                    if cap and cap.get("blocks"):
                        if cid not in items:
                            items[cid] = {"key": cid, "blocks": [{"id": b["id"], "text": b["text"]} for b in cap["blocks"]], "quotes": []}
                        items[cid]["quotes"].append({"id": key, "quote": sel["quote"], "hint": ""})
    ver = L.claimslab("verify", {"no_cap": True, "items": list(items.values())}) if items else {"results": []}
    by = {x["id"]: x["check"] for x in ver["results"]}
    for m in meta:
        c = by.get(m["key"])
        if not m["has_capture_id"]:
            m["outcome"] = "no_capture_id"
        elif not m["capture_readable"]:
            m["outcome"] = "capture_unreadable"
        elif c["retained"]:
            m["outcome"] = "matched_" + c["result"]["tier"]
        else:
            m["outcome"] = c.get("reason") or "not_found"
    return meta


def r2(stories, workers, sample_n):
    rounds = []
    for s in stories:
        asks = {}
        for r in s["research_records"]:
            if r.get("kind") == "directive" and (r.get("directive") or {}).get("subtype") == "cast":
                try:
                    body = json.loads(r["directive"].get("body") or "{}")
                except ValueError:
                    body = {}
                qs = body.get("questions") if isinstance(body, dict) else None
                if qs:
                    asks[r.get("round")] = qs
        for r in s["texture_records"]:
            if str(r.get("sender_inbox", "")).startswith("research:") and r.get("final") and r.get("report"):
                rounds.append({"doc_id": s["doc_id"], "round": r.get("round"), "stop_reason": r["report"].get("stop_reason"),
                               "questions": asks.get(r.get("round")), "claims": [c.get("text", "") for c in r["report"].get("claims") or []],
                               "findings": [f.get("text", "") for f in r["report"].get("findings") or []]})
    stops = collections.Counter(x["stop_reason"] for x in rounds)
    cand = [x for x in rounds if x["stop_reason"] == "completed" and x["questions"]]
    rng = random.Random(20261014)
    rng.shuffle(cand)
    cand = cand[:sample_n]

    def one(x):
        prompt = P.COVERAGE_V0 + "\nQUESTIONS\n" + "\n".join(f"{i}. {q}" for i, q in enumerate(x["questions"], 1)) + \
            "\n\nREPORT\n" + "\n".join("- " + c for c in x["claims"] + x["findings"])[:20000]
        r = L.omp(L.GEMINI, prompt)
        try:
            return L.parse_json(r["text"]).get("verdicts") or []
        except ValueError:
            return None
    print("R2 judge calls:", len(cand), flush=True)
    res = pmap(one, cand, workers)
    rows = []
    for x, vs in zip(cand, res):
        if not isinstance(vs, list):
            continue
        for v in vs:
            if isinstance(v, dict) and v.get("answer") in ("answered", "partial", "unanswered"):
                rows.append({"doc_id": x["doc_id"], "round": x["round"], "answer": v["answer"]})
    return {"final_reports": len(rounds), "stop_reasons": dict(stops), "sampled_completed_rounds": len(cand), "rows": rows}


def t1(stories):
    rows = []
    for s in stories:
        for c in ((s.get("version") or {}).get("checks") or {}).get("citations") or []:
            rows.append({"doc_id": s["doc_id"], "state": c.get("state"), "quote": c.get("quote"), "judgement": c.get("judgement")})
    return rows


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--pull", default=os.path.join(L.DATA, "pull"))
    ap.add_argument("--workers", type=int, default=8)
    ap.add_argument("--coverage-sample", type=int, default=120)
    ap.add_argument("--second-scorer-sample", type=int, default=60)
    a = ap.parse_args()
    os.makedirs(OUT, exist_ok=True)
    os.makedirs(EVID, exist_ok=True)
    stories = [s for s in json.load(open(os.path.join(a.pull, "stories.json"))) if s.get("doc_id")]
    captures = json.load(open(os.path.join(a.pull, "captures.json")))["captures"]

    m1rows, m1raw = m1(stories, a.workers)
    r1rows = r1(stories, captures)
    r2res = r2(stories, a.workers, a.coverage_sample)
    t1rows = t1(stories)

    # management M1: calibration
    scored = [x for x in m1rows if x["outcome"] != "no_evidence"]
    resolved = [x for x in scored if x["outcome"] != "unresolved"]
    br = L.claimslab("brier", {"groups": {
        "strict_unresolved_wrong": {"p": [x["p"] for x in scored], "y": [YS[x["outcome"]] for x in scored]},
        "resolved_only": {"p": [x["p"] for x in resolved], "y": [YS[x["outcome"]] for x in resolved]},
        "resolved_binary": {"p": [x["p"] for x in resolved if x["outcome"] != "qualified"],
                            "y": [YS[x["outcome"]] for x in resolved if x["outcome"] != "qualified"]}}})
    bins = collections.OrderedDict((k, []) for k in ("<0.5", "0.5-0.7", "0.7-0.85", "0.85-0.95", ">=0.95"))
    for x in resolved:
        p = x["p"]
        k = "<0.5" if p < 0.5 else "0.5-0.7" if p < 0.7 else "0.7-0.85" if p < 0.85 else "0.85-0.95" if p < 0.95 else ">=0.95"
        bins[k].append(x)
    calib = {k: {"n": len(v), "mean_p": round(sum(x["p"] for x in v) / len(v), 3) if v else None,
                 "observed": round(sum(YS[x["outcome"]] for x in v) / len(v), 3) if v else None} for k, v in bins.items()}
    rate_in = {"m1_confirmed_share_resolved": {"w": [1] * len(resolved), "y": [YS[x["outcome"]] for x in resolved]}}

    # research R1: quote fidelity
    r1c = collections.Counter(x["outcome"] for x in r1rows)
    checkable = [x for x in r1rows if x["outcome"].startswith("matched") or x["outcome"] in ("not_found", "over_cap", "empty")]
    rate_in["r1_quote_matched"] = {"w": [1] * len(checkable), "y": [1.0 if x["outcome"].startswith("matched") else 0.0 for x in checkable]}
    rate_in["r1_quote_exact"] = {"w": [1] * len(checkable), "y": [1.0 if x["outcome"] == "matched_exact" else 0.0 for x in checkable]}
    # research R2: completion versus coverage
    ys = {"answered": 1.0, "partial": 0.5, "unanswered": 0.0}
    rate_in["r2_question_coverage"] = {"w": [1] * len(r2res["rows"]), "y": [ys[x["answer"]] for x in r2res["rows"]]}
    # texture T1: support
    judged = [x for x in t1rows if x["judgement"] in ("supported", "failed")]
    rate_in["t1_supported"] = {"w": [1] * len(judged), "y": [1.0 if x["judgement"] == "supported" else 0.0 for x in judged]}
    # score the scorer: a second blind settler (gpt-6-luna) on a seeded sample
    # of stories; agreement with gemini per hypothesis, and the desk's Brier
    # under each scorer on the same hypotheses
    m1b, _ = m1(stories, a.workers, model=L.LUNA, sample=a.second_scorer_sample)
    g_by = {(x["record_id"], x["id"]): x for x in m1rows}
    pairs = [(g_by[(x["record_id"], x["id"])], x) for x in m1b if (x["record_id"], x["id"]) in g_by and x["outcome"] != "no_evidence"]
    agree = sum(1 for g, b in pairs if g["outcome"] == b["outcome"])
    pos = lambda o: o in ("confirmed", "qualified")  # noqa: E731
    agree_bin = sum(1 for g, b in pairs if pos(g["outcome"]) == pos(b["outcome"]))
    cross = L.claimslab("brier", {"groups": {
        "desk_p_under_gemini": {"p": [g["p"] for g, _ in pairs], "y": [YS[g["outcome"]] for g, _ in pairs]},
        "desk_p_under_luna": {"p": [b["p"] for _, b in pairs], "y": [YS[b["outcome"]] for _, b in pairs]}}})
    second = {"scorer": L.LUNA, "hypotheses": len(pairs), "exact_agreement": round(agree / len(pairs), 3) if pairs else None,
              "binary_agreement_confirmed_or_qualified": round(agree_bin / len(pairs), 3) if pairs else None,
              "luna_outcomes": dict(collections.Counter(b["outcome"] for _, b in pairs)),
              "gemini_outcomes_same_items": dict(collections.Counter(g["outcome"] for g, _ in pairs)), "brier": cross}
    # self-claims about the desk's own work, read as self-scores with an
    # implied probability of 1, against independent settlement
    ys_cov = [{"answered": 1.0, "partial": 0.5, "unanswered": 0.0}[x["answer"]] for x in r2res["rows"]]
    self_scores = {
        "research_completed_claim": L.claimslab("brier", {"groups": {"p1": {"p": [1.0] * len(ys_cov), "y": ys_cov}}})["p1"],
        "research_verbatim_quote_claim": L.claimslab("brier", {"groups": {"p1": {"p": [1.0] * len(rate_in["r1_quote_matched"]["y"]), "y": rate_in["r1_quote_matched"]["y"]}}})["p1"],
        "texture_citation_supports_claim": L.claimslab("brier", {"groups": {"p1": {"p": [1.0] * len(rate_in["t1_supported"]["y"]), "y": rate_in["t1_supported"]["y"]}}})["p1"],
        "explicit_self_scores_on_records": {"report_settled": 0, "resolve": 0, "scores": 0,
                                            "note": "no desk settles or scores its own records on the paper; self-scores here are implied (p=1) by the desk's own claims"},
    }
    rates = L.claimslab("rate", {"groups": rate_in})

    box = {
        "computer": "autopaper paper (computer-1355b83b...)", "stories_with_doc": len(stories),
        "scorer_blind": "judges never see the desk's probability or identity; nothing is written back (I26)",
        "model_ids_on_records": dict(collections.Counter("present" if (r.get("provenance") or {}).get("model_id") else "empty"
                                                         for st in stories for r in st["texture_records"] + st["research_records"])),
        "management": {
            "M1_cast_hypotheses": {
                "n": len(m1rows), "stories": len({x['doc_id'] for x in m1rows}), "no_research_evidence": len(m1rows) - len(scored),
                "outcomes": dict(collections.Counter(x["outcome"] for x in m1rows)),
                "mean_p": round(sum(x["p"] for x in scored) / len(scored), 3) if scored else None,
                "hedge_share_p_0.4_0.6": round(sum(1 for x in scored if 0.4 <= x["p"] <= 0.6) / len(scored), 3) if scored else None,
                "withdrawn": sum(1 for x in m1rows if x["withdrawn"]),
                "brier": br, "calibration_resolved": calib, "confirmed_share_resolved": rates["m1_confirmed_share_resolved"],
            },
        },
        "research": {
            "R1_quote_fidelity": {"quotes": len(r1rows), "outcomes": dict(r1c), "matched": rates["r1_quote_matched"], "exact": rates["r1_quote_exact"]},
            "R2_completed_vs_coverage": {"final_reports": r2res["final_reports"], "stop_reasons": r2res["stop_reasons"],
                                         "sampled_completed_rounds": r2res["sampled_completed_rounds"], "questions_judged": len(r2res["rows"]),
                                         "answers": dict(collections.Counter(x["answer"] for x in r2res["rows"])),
                                         "coverage": rates["r2_question_coverage"]},
        },
        "texture": {"T1_citation_support": {"citations": len(t1rows), "judgement": dict(collections.Counter(x["judgement"] for x in t1rows)),
                                            "quote_state": dict(collections.Counter(x["quote"] for x in t1rows)), "supported": rates["t1_supported"]}},
        "score_the_scorer_M1": second,
        "self_scores": self_scores,
        "engineering": {"assignments": 0, "note": "no engineering runs or assignments on the paper computer; E6a for engineering needs a computer that runs engineering work"},
    }
    json.dump({"m1": m1rows, "m1_raw": m1raw, "m1_second": m1b, "r1": r1rows, "r2": r2res, "t1": t1rows}, open(os.path.join(OUT, "e6a-raw.json"), "w"), ensure_ascii=False)
    json.dump(box, open(os.path.join(EVID, "box-score.json"), "w"), indent=1)
    print(json.dumps(box, indent=1)[:6000])


if __name__ == "__main__":
    main()
