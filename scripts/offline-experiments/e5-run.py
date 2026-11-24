#!/usr/bin/env python3
"""E5 (ledger C232): the batch model pass. Calls are strictly one after another.
  e5-run.py ARM INPUT OUT [--sample SAMPLE] [--limit N]
ARM: A (25 to a call, no retrieval), B (25 to a call, with neighbours' names and nearest stories),
     A5 (the sample at 5 to a call, arm A's prompt), A2 (the sample at 25 to a call in other batches).
The key comes from the environment (RIVER_API_KEY) and goes only in the request header; it is never printed or stored.
Every response's usage is recorded; the run stops when the running cost passes STOP dollars."""
import json, os, sys, time, random, re, urllib.request, urllib.error

URL = "https://api.river.ai/v1/chat/completions"
MODEL = "deepseek-ai/DeepSeek-V4.1-Flash-1017"
PRICE_IN, PRICE_CACHED, PRICE_OUT = 0.12, 0.0024, 0.48
STOP = 8.0

SYS_A = """You compress news items into symbols. For EACH item below give:
subjects - 5 to 12 subjects the item is about: people, organisations, places, events, products, ideas, laws, anything that could be a subject, in ONE list with no types. Use the commonest short name in English (a person by full name, an organisation by its usual name).
asserts - one short clause (at most 20 words) saying what the item asserts happened or is claimed.
Judge only from what is shown; look nothing up. Answer with a JSON array only, one object per item in the order given:
[{"id": "<item id>", "subjects": ["...", "..."], "asserts": "..."}]"""

SYS_B = """You compress news items into symbols. For EACH item below give:
subjects - 5 to 12 subjects the item is about: people, organisations, places, events, products, ideas, laws, anything that could be a subject, in ONE list with no types. Mark each subject "e": true when it is an existing name (it appears among the names of similar earlier items or in the nearby stories' titles shown with the item, or you are sure it is the same thing) or "e": false when it is new. When you mean the same thing as an existing name, reuse that name exactly, character for character; do not make a variant.
asserts - one short clause (at most 20 words) saying what the item asserts happened or is claimed.
Each item may come with "Known names" (subject names of its most similar earlier items) and "Nearby stories" (titles). Judge only from what is shown; look nothing up. Answer with a JSON array only, one object per item in the order given:
[{"id": "<item id>", "subjects": [{"name": "...", "e": true}], "asserts": "..."}]"""

def key():
    k = os.environ.get("RIVER_API_KEY")
    if not k:
        sys.exit("no RIVER_API_KEY in the environment")
    return k

def render(items, arm, names):
    parts = []
    for it in items:
        head = "[%s] %s | %s" % (it["id"], it["origin"], it["time"][11:16] + " UTC")
        body = ("Title: " + it["title"] + "\n" if it["title"] else "") + it["text"]
        extra = ""
        if arm == "B":
            ks = []
            for nid in it["neighbors"]:
                if nid in names:
                    ks.append("  - " + "; ".join(names[nid][:10]))
            if ks:
                extra += "\nKnown names (similar earlier items):\n" + "\n".join(ks)
            if it["stories"]:
                extra += "\nNearby stories: " + " || ".join(it["stories"])
        parts.append(head + "\n" + body + extra)
    return "\n\n".join(parts)

def call(k, system, content, effort):
    payload = {"model": MODEL, "temperature": 0, "max_tokens": 16000,
               "messages": [{"role": "system", "content": system}, {"role": "user", "content": content}]}
    if effort:
        payload["reasoning_effort"] = effort
    body = json.dumps(payload).encode()
    last = None
    for attempt in range(3):
        req = urllib.request.Request(URL, data=body, headers={"Authorization": "Bearer " + k, "Content-Type": "application/json", "Accept": "application/json"})
        t0 = time.time()
        try:
            with urllib.request.urlopen(req, timeout=600) as r:
                return time.time() - t0, json.loads(r.read().decode()), None
        except urllib.error.HTTPError as e:
            last = "http %d" % e.code
            if e.code in (400, 401, 403):
                break
        except Exception as e:
            last = type(e).__name__
        time.sleep(5 * (attempt + 1))
    return 0.0, None, last

def parse(txt):
    m = re.search(r"\[.*\]", txt, re.S)
    if not m:
        return None
    try:
        return json.loads(m.group(0))
    except Exception:
        return None

def cost(u):
    pin = u.get("prompt_tokens", 0)
    cached = (u.get("prompt_tokens_details") or {}).get("cached_tokens", 0) or 0
    return ((pin - cached) * PRICE_IN + cached * PRICE_CACHED + u.get("completion_tokens", 0) * PRICE_OUT) / 1e6

def main():
    arm, inp, out = sys.argv[1], sys.argv[2], sys.argv[3]
    sample = None
    limit = None
    if "--sample" in sys.argv:
        sample = [l.strip() for l in open(sys.argv[sys.argv.index("--sample") + 1]) if l.strip()]
    if "--limit" in sys.argv:
        limit = int(sys.argv[sys.argv.index("--limit") + 1])
    items = [json.loads(l) for l in open(inp)]
    byid = {it["id"]: it for it in items}
    if arm in ("A", "B"):
        order = [it["id"] for it in items]
        per = 25
    elif arm == "A5":
        order, per = sample, 5
    elif arm == "A2":
        order = list(sample)
        random.Random(20261125).shuffle(order)
        per = 25
    else:
        sys.exit("arm")
    batches = [order[i:i + per] for i in range(0, len(order), per)]
    if limit:
        batches = batches[:limit]
    system = SYS_B if arm == "B" else SYS_A
    k = key()
    done = {}
    names = {}
    total = 0.0
    if os.path.exists(out):
        for l in open(out):
            r = json.loads(l)
            done[r["batch"]] = r
            total += r.get("cost", 0)
            for o in r.get("parsed") or []:
                nm = [(s["name"] if isinstance(s, dict) else s) for s in o.get("subjects", [])]
                names[o["id"]] = nm
    f = open(out, "a")
    for bi, ids in enumerate(batches):
        if bi in done:
            continue
        if total > STOP:
            print("STOP: running cost %.2f over %.2f" % (total, STOP)); break
        its = [byid[i] for i in ids]
        content = render(its, arm, names)
        eff = "low"
        dt, resp, err = call(k, system, content, eff)
        retried = False
        if resp is None:
            retried = True
            dt, resp, err = call(k, system, content, None)
        rec = {"batch": bi, "ids": ids, "arm": arm, "seconds": round(dt, 1), "error": err, "retried_without_effort": retried}
        if resp is not None:
            msg = resp["choices"][0]["message"].get("content") or ""
            u = resp.get("usage", {})
            rec.update({"usage": u, "cost": cost(u), "raw": msg, "parsed": parse(msg), "model": resp.get("model")})
            total += rec["cost"]
            ok = rec["parsed"] is not None
            if not ok:  # one more try on a parse failure
                dt2, resp2, err2 = call(k, system, content, eff)
                if resp2 is not None:
                    msg2 = resp2["choices"][0]["message"].get("content") or ""
                    u2 = resp2.get("usage", {})
                    rec["retry_parse"] = {"usage": u2, "cost": cost(u2), "raw": msg2, "parsed": parse(msg2), "seconds": round(dt2, 1)}
                    rec["cost"] += cost(u2)
                    total += cost(u2)
                    if rec["retry_parse"]["parsed"] is not None:
                        rec["parsed"] = rec["retry_parse"]["parsed"]
            for o in rec.get("parsed") or []:
                nm = [(s["name"] if isinstance(s, dict) else s) for s in o.get("subjects", [])]
                names[o["id"]] = nm
        f.write(json.dumps(rec, ensure_ascii=False) + "\n")
        f.flush()
        print("arm %s batch %d/%d: %.1fs, parsed %s, running cost $%.3f" % (arm, bi + 1, len(batches), rec["seconds"], rec.get("parsed") is not None, total), flush=True)
    print("done, total cost $%.3f" % total)

main()
