#!/usr/bin/env python3
"""ICRL calibration experiment: settlement writer and measurement
(docs/archive/reports/icrl-calibration-experiment-2026-10-14.md; precommitment §3).

Subcommands:
  selftest   the arm and held-out rules agree with internal/calibration
             (values pinned in calibration_test.go), and the metric code
             reproduces E6a's management ECE (0.144) from its bins.
  baseline   recompute E6a's management numbers from e6a-raw.json (the
             metric code's reproducibility check), written to
             docs/archive/evidence/icrl-calibration/baseline.json.
  pending    read the computer's calibration view (GET /api/calibration
             ?detail=1) and save it; print counts per desk and arm.
  settle     settle pending hypotheses (frozen M1 judge) and completed
             research Reports (frozen R2 coverage judge) from a ledger pull
             (claims_ledger_pull.py); dry run unless --write, which POSTs
             /api/calibration/settlements. --write writes to the computer:
             it needs the lead's go-ahead.
  measure    per desk, arm and model: n, Brier, ECE, signed gap, bins,
             reliability/resolution, hedge share; the story-clustered
             bootstrap for management's ECE difference; the stop rules and
             the verdict of §3. JSON to docs/evidence/icrl-calibration/.

The key is read from the credential file and never printed. Judges never see
the desk's probability or identity (blind, as in E6a).

usage:
  icrl_calibration.py selftest
  icrl_calibration.py baseline [--raw FILE]
  icrl_calibration.py pending  --cred FILE
  icrl_calibration.py settle   --cred FILE [--pull DIR] [--min-age-hours 2] [--write]
  icrl_calibration.py measure  --cred FILE|--view FILE --since ISO [--until ISO]
                               [--phase NAME] [--harm-only]
"""
import argparse
import collections
import datetime as dt
import hashlib
import json
import math
import os
import random
import sys
import urllib.error
import urllib.request

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

HOST = "https://choir.news"
REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
EVID = os.path.join(REPO, "docs", "evidence", "icrl-calibration")
DATA = os.path.expanduser(os.environ.get("CLAIMS_DATA", "~/.config/choir-qa/traces/claims"))
SCORER_M1 = "e6a-m1-gemini-v0"   # one record per (scorer, target); a new judge is a new name
SCORER_R2 = "e6a-r2-gemini-v0"
JUDGE_MODEL = "google-antigravity/gemini-3.8-flash"
BINS = [("<0.5", 0.0, 0.5), ("0.5-0.7", 0.5, 0.7), ("0.7-0.85", 0.7, 0.85), ("0.85-0.95", 0.85, 0.95), (">=0.95", 0.95, 1.0000001)]
YS = {"confirmed": 1.0, "qualified": 0.5, "contradicted": 0.0}
COVERAGE = {"answered": 1.0, "partial": 0.5, "unanswered": 0.0}
# Preregistered thresholds (experiment §3).
N_PER_ARM = 500
DELTA_ECE = 0.03
BRIER_TOL = 0.005
RES_TOL = 0.01
HARM_BRIER = 0.03
BOOT = 2000
SEED = 20261014


# ---- the rules shared with internal/calibration ----

def arm(thread):
    return "digest" if hashlib.sha256(("choir-icrl-v1:" + thread.strip()).encode()).digest()[0] % 2 == 0 else "control"


def held_out(key):
    return hashlib.sha256(("choir-icrl-heldout-v1:" + key).encode()).digest()[0] % 10 == 0


def bin_of(p):
    for i, (_, lo, hi) in enumerate(BINS):
        if lo <= p < hi:
            return i
    return 0 if p < 0 else len(BINS) - 1


def measure(rows):
    """Brier, ECE over the five bins, mean p, observed, Murphy reliability
    and resolution over the same bins, hedge share. rows: [(p, y)]."""
    n = len(rows)
    if n == 0:
        return {"n": 0}
    brier = sum((p - y) ** 2 for p, y in rows) / n
    mp = sum(p for p, _ in rows) / n
    my = sum(y for _, y in rows) / n
    acc = [[0, 0.0, 0.0] for _ in BINS]
    for p, y in rows:
        a = acc[bin_of(p)]
        a[0] += 1
        a[1] += p
        a[2] += y
    ece = rel = res = 0.0
    bins = []
    for (label, _, _), (k, sp, sy) in zip(BINS, acc):
        if not k:
            continue
        bp, by = sp / k, sy / k
        ece += k / n * abs(bp - by)
        rel += k / n * (bp - by) ** 2
        res += k / n * (by - my) ** 2
        bins.append({"label": label, "n": k, "mean_p": round(bp, 4), "observed": round(by, 4)})
    return {"n": n, "brier": brier, "ece": ece, "mean_p": mp, "observed": my, "gap": mp - my,
            "reliability": rel, "resolution": res, "uncertainty": my * (1 - my),
            "hedge_share": sum(1 for p, _ in rows if 0.4 <= p <= 0.6) / n, "bins": bins}


def cluster_bootstrap(a, b, stat, seed=SEED, reps=BOOT):
    """95% interval of stat(control) - stat(digest), resampling stories.
    a, b: {story: [(p, y)]} for control and digest."""
    rng = random.Random(seed)
    ka, kb = sorted(a), sorted(b)
    diffs = []
    for _ in range(reps):
        ra = [r for s in (rng.choice(ka) for _ in ka) for r in a[s]]
        rb = [r for s in (rng.choice(kb) for _ in kb) for r in b[s]]
        ma, mb = measure(ra), measure(rb)
        if ma["n"] and mb["n"]:
            diffs.append(ma[stat] - mb[stat])
    diffs.sort()
    if not diffs:
        return None
    return [diffs[int(0.025 * len(diffs))], diffs[int(0.975 * len(diffs)) - 1]]


# ---- the computer ----

class Client:
    def __init__(self, cred):
        self.key = cred["api_key"]
        self.computer = cred["computer_id"]

    def call(self, method, path, body=None, timeout=180):
        headers = {"Authorization": "Bearer " + self.key, "X-Choir-Computer": self.computer}
        data = None
        if body is not None:
            data = json.dumps(body).encode()
            headers["Content-Type"] = "application/json"
        req = urllib.request.Request(HOST + path, data=data, headers=headers, method=method)
        try:
            return json.load(urllib.request.urlopen(req, timeout=timeout))
        except urllib.error.HTTPError as err:
            raise RuntimeError(f"{method} {path}: HTTP {err.code} {err.read()[:300]!r}")


def load_view(args):
    if getattr(args, "view", None):
        return json.load(open(args.view))
    view = Client(json.load(open(args.cred))).call("GET", "/api/calibration?detail=1")
    os.makedirs(os.path.join(DATA, "icrl"), exist_ok=True)
    stamp = dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    json.dump(view, open(os.path.join(DATA, "icrl", f"calibration-{stamp}.json"), "w"))
    return view


# ---- subcommands ----

def cmd_selftest(_):
    pinned = {"texture:doc-1": "digest", "management-task:t-1": "digest", "research:doc-1": "control", "management:owner-1": "control"}
    for t, want in pinned.items():
        assert arm(t) == want, (t, arm(t), want)
    for k, want in {"rec-1#H1": False, "rec-2#H1": True, "sp-abc": False}.items():
        assert held_out(k) == want, (k, want)
    rows = []
    for n, p, y in [(57, 0.373, 0.623), (250, 0.569, 0.752), (176, 0.744, 0.844), (67, 0.878, 0.903), (9, 0.964, 0.833)]:
        rows += [(p, y)] * n
    m = measure(rows)
    assert m["n"] == 559 and round(m["ece"], 3) == 0.144, m
    print("selftest: arm and held-out rules match internal/calibration; E6a ECE 0.144 reproduced")


def cmd_baseline(args):
    raw = json.load(open(args.raw))
    m1 = raw["m1"]
    resolved = [(x["p"], YS[x["outcome"]]) for x in m1 if x["outcome"] in YS]
    strict = [(x["p"], YS.get(x["outcome"], 0.0)) for x in m1 if x["outcome"] != "no_evidence"]
    out = {"source": "E6a e6a-raw.json (paper, 2026-10-13/14), judge gemini-3.8-flash HYPOTHESIS_SETTLE_V0",
           "management_hypotheses_resolved": measure(resolved), "management_hypotheses_strict": measure(strict)}
    os.makedirs(EVID, exist_ok=True)
    json.dump(out, open(os.path.join(EVID, "baseline.json"), "w"), indent=1)
    r = out["management_hypotheses_resolved"]
    print(f"baseline management (resolved): n={r['n']} ECE={r['ece']:.3f} Brier={r['brier']:.3f} "
          f"mean p={r['mean_p']:.2f} observed={r['observed']:.2f} resolution={r['resolution']:.4f}")


def cmd_pending(args):
    view = load_view(args)
    obs = view.get("observations") or []
    pend = view.get("pending") or []
    print("setting:", view.get("setting"), "computed_at:", view.get("computed_at"))
    print("observations:", collections.Counter((o["desk"], o["claim_type"], o.get("arm")) for o in obs))
    print("pending:", collections.Counter((p["desk"], p["claim_type"]) for p in pend))


def _age_hours(stamp):
    try:
        t = dt.datetime.fromisoformat(stamp.replace("Z", "+00:00"))
    except (AttributeError, ValueError):
        return 0.0
    return (dt.datetime.now(dt.timezone.utc) - t).total_seconds() / 3600


def cmd_settle(args):
    import claims_lib as L  # noqa: E402  (omp runner and its call cache)
    import claims_prompts as P  # noqa: E402
    from claims_e0 import pmap  # noqa: E402
    client = Client(json.load(open(args.cred)))
    view = client.call("GET", "/api/calibration?detail=1")
    pend = view.get("pending") or []
    stories = {s["doc_id"]: s for s in json.load(open(os.path.join(args.pull, "stories.json"))) if s.get("doc_id")}

    # Hypotheses: one judge call per Cast (all its pending hypotheses), with
    # research's findings and the published story's support-checked quotes.
    casts = collections.OrderedDict()
    for p in pend:
        if p["claim_type"] == "hypothesis" and _age_hours(p.get("committed_at")) >= args.min_age_hours:
            casts.setdefault(p["record_id"], []).append(p)
    tasks = []
    for rid, hyps in casts.items():
        doc = (hyps[0].get("story") or "").split(":", 1)[-1]
        s = stories.get(doc)
        if not s:
            continue
        claims = [c.get("text", "") for r in s["texture_records"] if str(r.get("sender_inbox", "")).startswith("research:") and r.get("report")
                  for c in r["report"].get("claims") or []]
        quotes = [c["ref"].get("quote") for c in ((s.get("version") or {}).get("checks") or {}).get("citations") or []
                  if c.get("judgement") == "supported" and (c.get("ref") or {}).get("quote")]
        if claims or quotes:
            tasks.append({"record_id": rid, "hyps": hyps, "claims": claims, "quotes": quotes})

    def judge_hyps(t):
        lines = "\n".join(f"{i}. {h['statement']}" for i, h in enumerate(t["hyps"], 1))
        ev = "RESEARCH FINDINGS\n" + "\n".join("- " + c for c in t["claims"][:40]) + \
            "\n\nSUPPORT-CHECKED SOURCE QUOTES IN THE PUBLISHED STORY\n" + "\n".join("- " + q for q in t["quotes"][:40])
        r = L.omp(L.GEMINI, P.HYPOTHESIS_SETTLE_V0 + "\nHYPOTHESES\n" + lines + "\n\nEVIDENCE\n" + ev[:24000])
        verdicts = {int(v.get("n", 0)): v for v in (L.parse_json(r["text"]).get("verdicts") or []) if isinstance(v, dict)}
        answers = {}
        for i, h in enumerate(t["hyps"], 1):
            out = (verdicts.get(i) or {}).get("outcome")
            if out in ("confirmed", "qualified", "contradicted", "unresolved"):
                answers[h["hypothesis_id"]] = out
        return answers

    # Completed research Reports: the questions of the Cast that opened the
    # round, against the Report's claims and findings.
    reports = [p for p in pend if p["claim_type"] == "completed" and _age_hours(p.get("committed_at")) >= args.min_age_hours]
    rtasks = []
    for p in reports:
        doc = (p.get("story") or "").split(":", 1)[-1]
        s = stories.get(doc)
        if not s:
            continue
        rep = next((r for r in s["texture_records"] if r.get("record_id") == p["record_id"]), None)
        if not rep:
            continue
        qs = None
        for r in s["research_records"]:
            if r.get("kind") == "directive" and (r.get("directive") or {}).get("subtype") == "cast" and r.get("round") == rep.get("round"):
                try:
                    qs = json.loads(r["directive"].get("body") or "{}").get("questions")
                except (ValueError, AttributeError):
                    qs = None
        if qs:
            rtasks.append({"record_id": p["record_id"], "questions": qs,
                           "text": [c.get("text", "") for c in rep["report"].get("claims") or []] + [f.get("text", "") for f in rep["report"].get("findings") or []]})

    def judge_coverage(t):
        prompt = P.COVERAGE_V0 + "\nQUESTIONS\n" + "\n".join(f"{i}. {q}" for i, q in enumerate(t["questions"], 1)) + \
            "\n\nREPORT\n" + "\n".join("- " + c for c in t["text"])[:20000]
        vs = [v.get("answer") for v in (L.parse_json(L.omp(L.GEMINI, prompt)["text"]).get("verdicts") or []) if isinstance(v, dict)]
        ys = [COVERAGE[a] for a in vs if a in COVERAGE]
        return round(sum(ys) / len(ys), 4) if ys else None

    print(f"settle: {len(tasks)} Casts, {len(rtasks)} completed Reports (of {len(pend)} pending)", flush=True)
    hyp_out = pmap(judge_hyps, tasks, args.workers)
    cov_out = pmap(judge_coverage, rtasks, args.workers)
    m1 = [{"target_record_id": t["record_id"], "claim_type": "hypothesis", "answers": a}
          for t, a in zip(tasks, hyp_out) if isinstance(a, dict) and a and "error" not in a]
    r2 = [{"target_record_id": t["record_id"], "claim_type": "completed", "coverage": c}
          for t, c in zip(rtasks, cov_out) if isinstance(c, float)]
    os.makedirs(os.path.join(DATA, "icrl"), exist_ok=True)
    stamp = dt.datetime.now(dt.timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    json.dump({"m1": m1, "r2": r2}, open(os.path.join(DATA, "icrl", f"settle-{stamp}.json"), "w"))
    print(f"settlements: {len(m1)} Casts, {len(r2)} Reports", flush=True)
    if not args.write:
        print("dry run: nothing written (pass --write to post)")
        return
    for scorer, items in ((SCORER_M1, m1), (SCORER_R2, r2)):
        for i in range(0, len(items), 200):
            res = client.call("POST", "/api/calibration/settlements",
                              {"scorer": scorer, "model_id": JUDGE_MODEL, "settlements": items[i:i + 200]})
            print(scorer, {k: (len(v) if isinstance(v, list) else v) for k, v in res.items()})


def cmd_measure(args):
    view = load_view(args)
    obs = [o for o in view.get("observations") or [] if (o.get("committed_at") or "") >= args.since
           and (not args.until or (o.get("committed_at") or "") < args.until)]
    for o in obs:
        o["arm"] = o.get("arm") or arm(o["thread"])
        if o["arm"] != arm(o["thread"]):
            raise SystemExit(f"arm mismatch for {o['key']}: guest {o['arm']}, script {arm(o['thread'])}")
    groups = collections.defaultdict(list)
    for o in obs:
        groups[(o["desk"], o["claim_type"], o["arm"])].append(o)
        groups[(o["desk"], o["claim_type"], o["arm"], "model:" + (o.get("model_id") or "unknown"))].append(o)
        groups[(o["desk"], o["claim_type"], o["arm"], "held_out" if o.get("held_out") else "shown")].append(o)
    table = {" / ".join(k): measure([(o["p"], o["y"]) for o in v]) for k, v in sorted(groups.items())}
    # The persistent management desk is one unit: reported, not pooled (§3).
    prim = [o for o in obs if o["desk"] == "management" and o["claim_type"] == "hypothesis" and o["thread"].startswith("management-task:")]
    by_arm = {a: collections.defaultdict(list) for a in ("control", "digest")}
    for o in prim:
        by_arm[o["arm"]][o.get("story") or o["thread"]].append((o["p"], o["y"]))
    flat = {a: [r for rows in by_arm[a].values() for r in rows] for a in by_arm}
    mc, md = measure(flat["control"]), measure(flat["digest"])
    result = {"phase": args.phase, "since": args.since, "until": args.until, "n_per_arm": {"control": mc["n"], "digest": md["n"]},
              "control": mc, "digest": md, "table": table}
    if mc["n"] and md["n"]:
        d_ece = mc["ece"] - md["ece"]
        d_brier = md["brier"] - mc["brier"]
        ci_ece = cluster_bootstrap(by_arm["control"], by_arm["digest"], "ece")
        ci_brier = cluster_bootstrap(by_arm["digest"], by_arm["control"], "brier")  # digest - control
        result.update({"delta_ece": d_ece, "delta_ece_ci95": ci_ece, "brier_digest_minus_control": d_brier,
                       "brier_diff_ci95": ci_brier, "resolution_digest_minus_control": md["resolution"] - mc["resolution"]})
        harm = ci_brier is not None and d_brier > HARM_BRIER and ci_brier[0] > 0
        result["harm_stop"] = harm
        if not args.harm_only:
            full = mc["n"] >= N_PER_ARM and md["n"] >= N_PER_ARM
            c1 = d_ece >= DELTA_ECE and ci_ece is not None and ci_ece[0] > 0
            c2 = d_brier <= BRIER_TOL
            c3 = md["resolution"] - mc["resolution"] >= -RES_TOL
            against = full and ci_ece is not None and (ci_ece[1] < DELTA_ECE or (d_brier > BRIER_TOL and ci_brier is not None and ci_brier[0] > 0))
            if harm:
                verdict = "stopped for harm: evidence against"
            elif c1 and c2 and c3 and full:
                verdict = "evidence for H-ICRL"
            elif c1 and c2 and not c3 and full:
                verdict = "calibration by hedging (not evidence for H-ICRL)"
            elif against:
                verdict = "evidence against H-ICRL"
            elif not full:
                verdict = "incomplete sample"
            else:
                verdict = "inconclusive"
            result["conditions"] = {"delta_ece": c1, "brier_not_worse": c2, "resolution_kept": c3, "full_sample": full}
            result["verdict"] = verdict
    os.makedirs(EVID, exist_ok=True)
    name = f"measure-{args.phase or 'adhoc'}.json"
    json.dump(result, open(os.path.join(EVID, name), "w"), indent=1)
    print(json.dumps({k: v for k, v in result.items() if k != "table"}, indent=1, default=str)[:4000])


def main():
    ap = argparse.ArgumentParser()
    sub = ap.add_subparsers(dest="cmd", required=True)
    sub.add_parser("selftest")
    b = sub.add_parser("baseline")
    b.add_argument("--raw", default=os.path.join(DATA, "e6a", "e6a-raw.json"))
    p = sub.add_parser("pending")
    p.add_argument("--cred", required=True)
    s = sub.add_parser("settle")
    s.add_argument("--cred", required=True)
    s.add_argument("--pull", default=os.path.join(DATA, "pull"))
    s.add_argument("--min-age-hours", type=float, default=2.0)
    s.add_argument("--workers", type=int, default=8)
    s.add_argument("--write", action="store_true")
    m = sub.add_parser("measure")
    g = m.add_mutually_exclusive_group(required=True)
    g.add_argument("--cred")
    g.add_argument("--view", help="a saved GET /api/calibration?detail=1 response")
    m.add_argument("--since", required=True)
    m.add_argument("--until")
    m.add_argument("--phase", default="")
    m.add_argument("--harm-only", action="store_true")
    args = ap.parse_args()
    {"selftest": cmd_selftest, "baseline": cmd_baseline, "pending": cmd_pending, "settle": cmd_settle, "measure": cmd_measure}[args.cmd](args)


if __name__ == "__main__":
    main()
