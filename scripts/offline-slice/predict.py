#!/usr/bin/env python3
"""Kill line (d), offline: predictions by stories, settled by System 1, scored against a base rate.
  predict.py write  DIR   -> River call: 3 predictions for each of the 50 stories (as of the end of day 3)
  predict.py jobs   DIR   -> System 1 noul jobs: each prediction asked of the days 4-7 arrivals that were offered the story
  predict.py score  DIR   -> settle, Brier skill against the base rate, bootstrap over stories
Inputs in DIR: picked-asof.json, lookup.json (arrival -> top-5 stories), predictions.json, jobs/resp files.
Keys are read by river-run.py's and jev-run.py's loaders (the loader of scripts/event-blind-sample.py)."""
import json, os, sys, random, re, collections, importlib.util

HERE = os.path.dirname(os.path.abspath(__file__))
def load(name):
    spec = importlib.util.spec_from_file_location(name, os.path.join(HERE, name + ".py"))
    return spec

def river():
    src = open(os.path.join(HERE, "river-run.py")).read().replace("\nmain()\n", "\n")
    ns = {"__file__": os.path.join(HERE, "river-run.py")}
    exec(compile(src, "river-run.py", "exec"), ns)
    return ns

PROMPT = """These are stories a news site published, as they stood at the end of Tuesday 17 November 2026. For each, write exactly 3 predictions about what news sources will report on its subject from Wednesday 18 to Saturday 21 November 2026. Each prediction must be one specific development that one article could plainly state (not a vague theme), different from the other two, and each must have a probability between 0 and 1 that at least one source reports it in that window. Be calibrated: most specific developments do not happen; say so with low probabilities. Do not repeat facts the story already states.
Answer with JSON only: [{"id": "<story id>", "predictions": [{"claim": "<one sentence, stated as a fact that would be reported>", "p": <0..1>}, x3]}]
"""

def write(d):
    info = json.load(open(d + "/picked-asof.json"))
    parts = []
    for i, (sid, s) in enumerate(info.items(), 1):
        parts.append("=== STORY %d ===\nTitle: %s\nText: %s" % (i, s["title"], s["dek"]))
    ids = list(info)
    R = river()
    k = R["key"]()
    st, sec, resp = R["call"](k, PROMPT + "\n\n" + "\n\n".join(parts)) if False else (None, None, None)
    import urllib.request
    body = json.dumps({"model": R["MODEL"], "temperature": 0, "reasoning_effort": "high", "messages": [{"role": "user", "content": PROMPT + "\n\nUse these ids in order: " + json.dumps(list(range(1, len(ids) + 1))) + "\n\n" + "\n\n".join(parts)}]}).encode()
    req = urllib.request.Request(R["URL"], data=body, headers={"Authorization": "Bearer " + k, "Content-Type": "application/json"})
    r = json.loads(urllib.request.urlopen(req, timeout=900).read().decode())
    txt = r["choices"][0]["message"]["content"]
    arr = json.loads(re.search(r"\[.*\]", txt, re.S).group(0))
    preds = {}
    for a in arr:
        sid = ids[int(a["id"]) - 1]
        preds[sid] = a["predictions"][:3]
    json.dump({"preds": preds, "usage": r.get("usage")}, open(d + "/predictions.json", "w"), ensure_ascii=False)
    print("predictions for", len(preds), "stories;", r.get("usage"))

def jobs(d):
    lk = json.load(open(d + "/lookup.json"))
    P = json.load(open(d + "/predictions.json"))["preds"]
    offers = collections.defaultdict(list)
    for i, r in lk.items():
        if r["day"] >= "2026-11-18":
            for s in r["stories"]:
                offers[s[0]].append(i)
    pairs = []
    for sid, ps in P.items():
        for pi, p in enumerate(ps):
            for a in offers[sid]:
                pairs.append((sid, pi, a, p["claim"]))
    random.seed(1)
    out = []; cur = None
    def new():
        return {"id": "p%03d" % (len(out) + 1), "caps": {}, "q": {}, "items": []}
    cur = new()
    for sid, pi, a, claim in pairs:
        if len(cur["items"]) >= 120:
            out.append(cur); cur = new()
        if a not in cur["caps"]:
            cur["caps"][a] = "c%d" % (len(cur["caps"]) + 1)
        h = cur["caps"][a]
        qn = "q%d" % (len(cur["items"]) + 1)
        cur["q"][qn] = {"type": "noul", "instructions": "Does `state.captures.%s` report that: %s" % (h, claim)}
        cur["items"].append({"question": qn, "story": sid, "pred": pi, "arrival": a})
    if cur["items"]:
        out.append(cur)
    with open(d + "/pred-jobs.jsonl", "w") as f:
        for j in out:
            state = {"task": "Each capture is a news item. For each question, answer whether the capture reports the stated development.",
                     "captures": {h: {"title": lk[a]["title"], "text": lk[a]["lead"][:350]} for a, h in j["caps"].items()}}
            f.write(json.dumps({"id": j["id"], "state": state, "questions": j["q"], "items": j["items"]}, ensure_ascii=False) + "\n")
    print(len(pairs), "pairs in", len(out), "calls;", sum(1 for sid in P if offers[sid]), "of", len(P), "stories were offered to any arrival")

def score(d):
    P = json.load(open(d + "/predictions.json"))["preds"]
    J = {}
    for l in open(d + "/pred-jobs.jsonl"):
        j = json.loads(l); J[j["id"]] = j
    yes = collections.defaultdict(list)   # (story, pred) -> [p yes]
    n = 0
    for l in open(d + "/pred-resp.jsonl"):
        r = json.loads(l)
        if r["status"] != 200: print("failed", r["job"]); continue
        ans = r["body"]["answers"]
        for it in J[r["job"]]["items"]:
            a = ans.get(it["question"])
            if a is None: continue
            p = a.get("noul", a.get("answer", a.get("probability", a.get("p"))))
            if p is None:
                p = a.get("probabilities", {}).get("true") if isinstance(a.get("probabilities"), dict) else None
            if p is None: continue
            yes[(it["story"], it["pred"])].append(p); n += 1
    rows = []
    for sid, ps in P.items():
        for pi, p in enumerate(ps):
            outcome = 1 if any(x >= 0.5 for x in yes.get((sid, pi), [])) else 0
            rows.append((sid, float(p["p"]), outcome))
    base = sum(o for _, _, o in rows) / len(rows)
    def brier(rs, f): return sum((f(p) - o) ** 2 for _, p, o in rs) / len(rs)
    bm = brier(rows, lambda p: p); bb = brier(rows, lambda p: base)
    print("answers read", n, "| predictions", len(rows), "| settled true", sum(o for _, _, o in rows), "base rate %.3f" % base)
    print("Brier model %.4f, Brier base %.4f, skill %.3f" % (bm, bb, 1 - bm / bb if bb else float("nan")))
    by = collections.defaultdict(list)
    for r in rows: by[r[0]].append(r)
    stories = list(by)
    random.seed(7); sk = []
    for _ in range(2000):
        rs = [x for s in random.choices(stories, k=len(stories)) for x in by[s]]
        b0 = sum(o for _, _, o in rs) / len(rs)
        a = brier(rs, lambda p: p); c = brier(rs, lambda p: b0)
        if c > 0: sk.append(1 - a / c)
    sk.sort()
    print("bootstrap over stories (2000): skill 95%% interval [%.3f, %.3f]" % (sk[int(.025 * len(sk))], sk[int(.975 * len(sk))]))
    print("mean predicted p %.3f" % (sum(p for _, p, _ in rows) / len(rows)))

if __name__ == "__main__":
    {"write": write, "jobs": jobs, "score": score}[sys.argv[1]](sys.argv[2])
