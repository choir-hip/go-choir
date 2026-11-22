#!/usr/bin/env python3
"""Rung 2: name groups of unmatched arrivals with one model (River, DeepSeek V4.1 Flash 1017), many
groups to a call. Input is a JSONL of groups (cmd/offlineslice groups2) or a folder of the reader's
group files (slice-r0c/groups/NN.md). Output: one JSON line per call with the model's answer and usage.
  river-run.py groups2.jsonl|DIR OUT [--per-call N]
The key is read by the loader of scripts/event-blind-sample.py (environment, else the one named .env
line), here for RIVER_API_KEY; it goes only into the request header and is never printed or stored."""
import json, os, re, sys, time, glob, urllib.request, urllib.error

URL = "https://api.river.ai/v1/chat/completions"
MODEL = "deepseek-ai/DeepSeek-V4.1-Flash-1017"
PRICE_IN, PRICE_OUT = 0.12, 0.48  # dollars per million tokens (internal/llmcost/pricing.go), uncached

def key():
    if os.environ.get("RIVER_API_KEY"):
        return os.environ["RIVER_API_KEY"]
    root = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
    for line in open(os.path.join(root, ".env")):
        mm = re.match(r'\s*(?:export\s+)?RIVER_API_KEY\s*=\s*"?([^"\s]+)', line)
        if mm:
            return mm.group(1)
    sys.exit("no RIVER_API_KEY in the environment or .env")

INSTR = """You are reading groups of news items that a program put together because their texts read alike. Each group has up to 12 items, earliest first; each item has an outlet or account, a time and its text (a social post has no title). For EACH group decide:

kind - exactly one of:
  ONE: every item reports the same affair (the same happening, or successive reports of it).
  MOSTLY_ONE: most items report one affair; list the odd items by number in "odd".
  THEME: the items share actors or a topic but report different happenings (for example different crimes in one city, different bills in one legislature, different products).
  NOISE: no common affair; or page furniture, spam, automatic listings or feeds with no news in them.
main_point - one line (at most 25 words) saying what happened, in your own words; for THEME or NOISE say what the items have in common.
news - would a general newspaper cover the affair: "yes", "no" or "marginal". Judge the affair, not the reporting.

Judge only from what is shown; look nothing up. Answer with a JSON array only, one object per group, in the order given:
[{"id": "<group id>", "kind": "ONE|MOSTLY_ONE|THEME|NOISE", "odd": [item numbers], "main_point": "...", "news": "yes|no|marginal"}]
"""

def groups_from_jsonl(path):
    out = []
    for l in open(path):
        g = json.loads(l)
        items = []
        for m in g["members"]:
            items.append({"outlet": m["origin"], "time": m["time"][:16].replace("T", " ") + " UTC", "title": m["title"], "text": m["text"]})
        out.append({"id": g["id"], "n_items_shown": len(items), "items": items})
    return out

def groups_from_dir(d):
    out = []
    for p in sorted(glob.glob(os.path.join(d, "*.md"))):
        txt = open(p).read()
        gid = "r" + os.path.basename(p)[:-3]
        items = []
        for part in txt.split("## Item ")[1:]:
            title = re.search(r"- Title: (.*)", part).group(1)
            outlet = re.search(r"- Outlet: (.*)", part).group(1)
            when = re.search(r"- Time: (.*)", part).group(1)
            body = part.split("\n\n", 2)[-1].strip()
            items.append({"outlet": outlet, "time": when, "title": "" if title.startswith("(a social post") else title, "text": body})
        out.append({"id": gid, "n_items_shown": len(items), "items": items})
    return out

def render(groups):
    parts = []
    for g in groups:
        parts.append("=== GROUP %s ===" % g["id"])
        for i, it in enumerate(g["items"], 1):
            t = ("Title: " + it["title"] + "\n") if it["title"] else ""
            parts.append("[%d] %s | %s\n%s%s" % (i, it["outlet"], it["time"], t, it["text"]))
    return "\n\n".join(parts)

def call(k, content):
    body = json.dumps({"model": MODEL, "temperature": 0, "reasoning_effort": "high",
                       "messages": [{"role": "system", "content": INSTR}, {"role": "user", "content": content}]}).encode()
    for attempt in range(4):
        req = urllib.request.Request(URL, data=body, headers={"Authorization": "Bearer " + k, "Content-Type": "application/json", "Accept": "application/json"})
        t0 = time.time()
        try:
            with urllib.request.urlopen(req, timeout=600) as r:
                return r.status, time.time() - t0, json.loads(r.read().decode())
        except urllib.error.HTTPError as e:
            txt = e.read().decode()[:400]
            if e.code in (429, 500, 502, 503, 504) and attempt < 3:
                time.sleep([5, 15, 45][attempt]); continue
            return e.code, time.time() - t0, {"error": txt}
        except Exception as e:
            if attempt < 3:
                time.sleep([5, 15, 45][attempt]); continue
            return 0, time.time() - t0, {"error": str(e)[:300]}

def main():
    src, out_path = sys.argv[1], sys.argv[2]
    per = int(sys.argv[sys.argv.index("--per-call") + 1]) if "--per-call" in sys.argv else 25
    groups = groups_from_dir(src) if os.path.isdir(src) else groups_from_jsonl(src)
    k = key()
    out = open(out_path, "a")
    tin = tout = 0
    for b in range(0, len(groups), per):
        chunk = groups[b:b + per]
        st, sec, resp = call(k, render(chunk))
        rec = {"ids": [g["id"] for g in chunk], "status": st, "seconds": round(sec, 2)}
        if st == 200:
            rec["usage"] = resp.get("usage", {})
            rec["text"] = resp["choices"][0]["message"].get("content", "")
            tin += rec["usage"].get("prompt_tokens", 0); tout += rec["usage"].get("completion_tokens", 0)
        else:
            rec["err"] = resp
        out.write(json.dumps(rec, ensure_ascii=False) + "\n"); out.flush()
        print("call", b // per + 1, "status", st, "seconds", round(sec, 1), file=sys.stderr, flush=True)
    print("tokens in %d out %d, about $%.4f at uncached prices" % (tin, tout, tin * PRICE_IN / 1e6 + tout * PRICE_OUT / 1e6), file=sys.stderr)

main()
