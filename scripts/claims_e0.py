#!/usr/bin/env python3
"""The claims layer's offline study, E0 (docs/reports/claims-events-entities-ontology-2026-10-14.md
§12.3; results in docs/archive/reports/claims-e0-e6a-results-2026-10-14.md).

Steps, each idempotent (model calls are cached by claims_lib.omp):
  freeze    build the frozen evaluation set from a pull (claims_ledger_pull.py):
            text to DATA/e0/evalset.json, ids-only manifest to the repo
  overhead  measure each model's omp harness input overhead (for net cost)
  label     blind gold labels on the label split: gemini-3.8-flash, two passes
  extract   run the extractors (gpt-6-luna, deepseek-v4.1-flash) on the whole set
  verify    quote-check every extracted and labelled quote with quotematch
  judge     blind validity and settlement-feasibility audit of extracted claims
  wikidata  check the extractor's Wikidata guesses against Wikidata labels
  route     routing diagnostic on the after-window's judged pairs
  score     compute the metrics; write the ids-and-numbers results to the repo

Article text stays under DATA (default ~/.config/choir-qa/traces/claims).
"""
import argparse
import collections
import concurrent.futures as cf
import datetime as dt
import hashlib
import json
import os
import random
import re
import sys
import urllib.parse
import urllib.request

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import claims_lib as L  # noqa: E402
import claims_prompts as P  # noqa: E402

E0 = os.path.join(L.DATA, "e0")
EVID = os.path.join(L.REPO, "docs", "evidence", "claims-e0")
EXTRACTORS = [L.LUNA, L.DEEPSEEK]
MAXCHARS = 12000
SEED = 20261014
TWO_LEVEL = {"co.uk", "com.au", "co.jp", "com.tw", "org.uk", "com.br", "co.kr", "com.cn", "ne.jp", "or.jp", "gov.uk", "ac.uk", "com.ua", "org.ua", "com.mx", "com.ar"}


def short(model):
    return model.split("/")[-1]


def domain(url):
    host = (urllib.parse.urlparse(url or "").hostname or "").lower()
    parts = host.split(".")
    if parts and parts[0] == "www":
        parts = parts[1:]
    if len(parts) >= 3 and ".".join(parts[-2:]) in TWO_LEVEL:
        return ".".join(parts[-3:])
    return ".".join(parts[-2:])


def script_lang(text):
    counts = collections.Counter()
    for ch in text[:3000]:
        o = ord(ch)
        if 0x0400 <= o <= 0x04FF:
            counts["cyrillic"] += 1
        elif 0x3040 <= o <= 0x30FF:
            counts["kana"] += 1
        elif 0x4E00 <= o <= 0x9FFF:
            counts["han"] += 1
        elif 0xAC00 <= o <= 0xD7AF:
            counts["hangul"] += 1
        elif 0x0600 <= o <= 0x06FF:
            counts["arabic"] += 1
        elif ch.isalpha():
            counts["latin"] += 1
    if not counts:
        return "unknown"
    top = counts.most_common(1)[0][0]
    if top == "han" and counts["kana"] > 20:
        top = "kana"
    return {"cyrillic": "cyrillic", "kana": "ja", "han": "zh", "hangul": "ko", "arabic": "ar", "latin": "latin"}[top]


LATIN_WORDS = {"en": (" the ", " and ", " of ", " to "), "es": (" el ", " los ", " que ", " del "), "fr": (" le ", " les ", " des ", " est "),
               "de": (" der ", " die ", " und ", " nicht "), "pt": (" não ", " uma ", " dos ", " das "), "it": (" il ", " della ", " che ", " gli ")}


def latin_lang(text):
    t = " " + text.lower() + " "
    scores = {k: sum(t.count(w) for w in ws) for k, ws in LATIN_WORDS.items()}
    best = max(scores, key=scores.get)
    return best if scores[best] >= 2 else "latin"


def render(cap, maxchars=MAXCHARS):
    """The document as the models see it: a header and numbered blocks."""
    head = f"outlet: {domain(cap['url'])}\nurl: {cap['url']}\ntitle: {cap.get('title', '')}\nfetched_at: {cap.get('fetched_at', '')}\n\n"
    lines, total, ids = [], 0, {}
    for i, b in enumerate(cap["blocks"]):
        t = b["text"].strip()
        if not t:
            continue
        if total + len(t) > maxchars and lines:
            break
        lines.append(f"[b{i}] {t}")
        ids[f"b{i}"] = b["id"]
        total += len(t)
    return head + "\n\n".join(lines), ids


def load_evalset():
    return json.load(open(os.path.join(E0, "evalset.json")))


def pmap(fn, items, workers=8):
    out = [None] * len(items)
    with cf.ThreadPoolExecutor(workers) as pool:
        futs = {pool.submit(fn, it): i for i, it in enumerate(items)}
        done = 0
        for f in cf.as_completed(futs):
            i = futs[f]
            try:
                out[i] = f.result()
            except Exception as err:  # recorded per item
                out[i] = {"error": str(err)[:500]}
            done += 1
            if done % 20 == 0:
                print(f"  {done}/{len(items)}", flush=True)
    return out


# ---------------------------------------------------------------- freeze

def cmd_freeze(args):
    pull = args.pull
    stories = json.load(open(os.path.join(pull, "stories.json")))
    caps = json.load(open(os.path.join(pull, "captures.json")))["captures"]
    reg = json.load(open(os.path.join(L.REPO, "internal", "sources", "registry.json")))["sources"]
    reg_by_domain = {}
    for s in reg:
        reg_by_domain.setdefault(domain(s["url"]), s)
    cap_re = re.compile(r"obj:choir\.web_capture:[A-Za-z0-9_\-]+:key-[0-9a-f]{64}")
    scheduled, targeted, section = set(), set(), {}
    for st in stories:
        sec = (st.get("front") or {}).get("section") or "unsectioned"
        for r in st.get("texture_records") or []:
            if str(r.get("sender_inbox", "")).startswith("management:"):
                for cid in cap_re.findall(json.dumps(r.get("cast") or {}) + json.dumps(r.get("directive") or {}) + json.dumps(r.get("disposition") or {})):
                    scheduled.add(cid)
                    section.setdefault(cid, sec)
            if str(r.get("sender_inbox", "")).startswith("research:"):
                for cid in cap_re.findall(json.dumps(r.get("prediction") or {})):
                    targeted.add(cid)
                    section.setdefault(cid, sec)
    window = set(json.load(open(os.path.join(pull, "captures.json"))).get("newest_window") or [])
    pool, seen_url = [], set()
    for cid, c in sorted(caps.items(), key=lambda kv: kv[1].get("fetched_at") or ""):
        blocks = [b for b in c.get("blocks") or [] if b.get("text", "").strip()]
        chars = sum(len(b["text"]) for b in blocks)
        # a scheduled capture is a poll's item: named by management, or in
        # the newest window, not named by research, and a single feed block
        is_sched = cid in scheduled or (cid in window and cid not in targeted and len(blocks) == 1)
        if is_sched and chars < 150:
            continue
        if not is_sched and (len(blocks) < 2 or chars < 300):
            continue
        url = (c.get("url") or "").split("#")[0]
        canon = re.sub(r"[?].*$", "", url)
        if canon in seen_url:
            continue  # re-captures removed
        seen_url.add(canon)
        dom = domain(url)
        r = reg_by_domain.get(dom)
        sample_text = " ".join(b["text"] for b in blocks[:8])
        sl = script_lang(sample_text)
        lang = {"latin": latin_lang(sample_text), "cyrillic": "ru/uk"}.get(sl, sl)
        sel = "scheduled" if is_sched else "targeted"
        pool.append({"capture_id": cid, "url": url, "domain": dom, "lang": lang, "script": sl,
                     "section": section.get(cid) or ((r or {}).get("verticals") or ["unregistered"])[0],
                     "selection": sel, "registry_source": (r or {}).get("id"), "doc_hash": c.get("doc_hash"),
                     "fetched_at": c.get("fetched_at"), "title": c.get("title"), "blocks": blocks, "chars": chars,
                     "block_count": c.get("block_count"), "blocks_truncated": bool(c.get("blocks_truncated"))})
    rng = random.Random(SEED)
    rng.shuffle(pool)
    for p in pool:
        p["body_kind"] = "hn_stub" if p["blocks"][0]["text"].lstrip().startswith("Article URL:") else (
            "feed_item" if len(p["blocks"]) == 1 else "full_text")
    sched = [p for p in pool if p["selection"] == "scheduled"]
    targ = [p for p in pool if p["selection"] == "targeted"]

    def stratified(cands, want, cap, hn_cap):
        """Round-robin over languages (rarest first), preferring outlets and
        sections not yet chosen; at most cap per outlet and hn_cap HN stubs."""
        by_lang = collections.defaultdict(list)
        for p in cands:
            by_lang[p["lang"]].append(p)
        chosen, per_outlet, hn = [], collections.Counter(), 0
        while len(chosen) < want and any(by_lang.values()):
            for lang in sorted(by_lang, key=lambda k: (len(by_lang[k]), k)):
                if not by_lang[lang] or len(chosen) >= want:
                    continue
                secs = collections.Counter(x["section"] for x in chosen)
                by_lang[lang].sort(key=lambda x: (per_outlet[x["domain"]], secs[x["section"]]))
                p = by_lang[lang].pop(0)
                if per_outlet[p["domain"]] >= cap or (p["body_kind"] == "hn_stub" and hn >= hn_cap):
                    continue
                hn += p["body_kind"] == "hn_stub"
                per_outlet[p["domain"]] += 1
                chosen.append(p)
        return chosen

    s_chosen = stratified(sched, args.scheduled, 9, 15)
    t_chosen = stratified(targ, args.targeted, 4, 0)
    rng.shuffle(s_chosen)
    rng.shuffle(t_chosen)
    for p in s_chosen[:20]:
        p["split"] = "tune"
    for p in s_chosen[20:55] + t_chosen[:15]:
        p["split"] = "label"
    chosen = s_chosen + t_chosen
    for p in chosen:
        p.setdefault("split", "unlabelled")
    os.makedirs(E0, exist_ok=True)
    os.makedirs(EVID, exist_ok=True)
    json.dump(chosen, open(os.path.join(E0, "evalset.json"), "w"), ensure_ascii=False)
    manifest_items = [{k: p[k] for k in ("capture_id", "doc_hash", "domain", "lang", "script", "section", "selection",
                                          "registry_source", "fetched_at", "split", "chars", "block_count", "body_kind")} for p in chosen]
    body = json.dumps(manifest_items, sort_keys=True, ensure_ascii=False)
    manifest = {"schema": "claims_e0.evalset.v1", "frozen_at": dt.datetime.now(dt.timezone.utc).isoformat(),
                "seed": SEED, "source": "autopaper paper computer (computer-1355b83b...), read-only pull",
                "pool": {"eligible": len(pool), "scheduled": len(sched), "targeted": len(targ)},
                "manifest_sha256": hashlib.sha256(body.encode()).hexdigest(), "items": manifest_items}
    json.dump(manifest, open(os.path.join(EVID, "evalset-manifest.json"), "w"), indent=1, ensure_ascii=False)
    print(json.dumps({"chosen": len(chosen), "splits": collections.Counter(p["split"] for p in chosen),
                      "langs": collections.Counter(p["lang"] for p in chosen),
                      "sections": collections.Counter(p["section"] for p in chosen),
                      "selection": collections.Counter(p["selection"] for p in chosen),
                      "body_kind": collections.Counter(p["body_kind"] for p in chosen),
                      "outlets": len(set(p["domain"] for p in chosen)), "sha": manifest["manifest_sha256"]}, default=str))


# ---------------------------------------------------------------- models

def cmd_overhead(args):
    out = {}
    for m in EXTRACTORS + [L.GEMINI]:
        r = L.omp(m, "Reply with the single word OK.")
        out[m] = {"input": r["usage"].get("input"), "cacheRead": r["usage"].get("cacheRead"), "output": r["usage"].get("output"),
                  "omp_cost": (r["usage"].get("cost") or {}).get("total")}
    json.dump(out, open(os.path.join(E0, "overhead.json"), "w"), indent=1)
    print(json.dumps(out))


def cmd_label(args):
    items = [p for p in load_evalset() if p["split"] == "label"]

    def one(p):
        doc, ids = render(p)
        passes = []
        for k in (1, 2):
            r = L.omp(L.GEMINI, P.LABEL_V0 + f"(annotation pass {k})\n\n" + doc)
            try:
                claims = L.parse_json(r["text"]).get("claims") or []
            except ValueError:
                claims = None
            passes.append({"claims": claims, "usage": r["usage"], "served": r["served"], "seconds": r["seconds"]})
        return {"capture_id": p["capture_id"], "block_ids": ids, "passes": passes}

    res = pmap(one, items, args.workers)
    json.dump(res, open(os.path.join(E0, "labels.json"), "w"), ensure_ascii=False)
    print("labelled", sum(1 for r in res if "passes" in r), "errors", sum(1 for r in res if "error" in r))


def cmd_extract(args):
    items = load_evalset()
    models = [m for m in EXTRACTORS if not args.model or short(m) == args.model]
    for m in models:
        def one(p, m=m):
            doc, ids = render(p)
            r = L.omp(m, P.EXTRACT_V0 + doc)
            try:
                ext = L.parse_json(r["text"])
                if isinstance(ext, list):
                    ext = {"claims": ext}
                parse_ok = True
            except ValueError:
                ext, parse_ok = {"claims": [], "mentions": []}, False
            return {"capture_id": p["capture_id"], "extractor": m, "prompt_sha256": L.sha(P.EXTRACT_V0), "block_ids": ids,
                    "parse_ok": parse_ok, "extraction": ext, "usage": r["usage"], "served": r["served"], "seconds": r["seconds"],
                    "cached": r["cached"]}
        print("extract", m, flush=True)
        res = pmap(one, items, args.workers)
        json.dump(res, open(os.path.join(E0, f"extract-{short(m)}.json"), "w"), ensure_ascii=False)
        print(m, "ok", sum(1 for r in res if r.get("parse_ok")), "of", len(res))


def cmd_verify(args):
    caps = {p["capture_id"]: p for p in load_evalset()}
    items = []
    files = [f"extract-{short(m)}.json" for m in EXTRACTORS] + ["labels.json"]
    for f in files:
        path = os.path.join(E0, f)
        if not os.path.exists(path):
            continue
        for r in json.load(open(path)):
            if "error" in r:
                continue
            cap = caps[r["capture_id"]]
            blocks = [{"id": b["id"], "text": b["text"]} for b in cap["blocks"]]
            quotes = []
            if f.startswith("extract"):
                for i, c in enumerate((r.get("extraction") or {}).get("claims") or []):
                    if isinstance(c, dict):
                        quotes.append({"id": f"c{i}", "quote": str(c.get("quote") or ""), "hint": r["block_ids"].get(str(c.get("block") or ""), "")})
            else:
                for k, ps in enumerate(r["passes"]):
                    for i, c in enumerate(ps.get("claims") or []):
                        if isinstance(c, dict):
                            quotes.append({"id": f"p{k}c{i}", "quote": str(c.get("quote") or ""), "hint": r["block_ids"].get(str(c.get("block") or ""), "")})
            items.append({"key": f + "|" + r["capture_id"], "blocks": blocks, "quotes": quotes})
    out = L.claimslab("verify", {"items": items})
    json.dump(out, open(os.path.join(E0, "verify.json"), "w"), ensure_ascii=False)
    c = collections.Counter((r["key"].split("|")[0], r["check"]["retained"]) for r in out["results"])
    print(out["matcher"], dict(c))


def checks():
    out = collections.defaultdict(dict)
    for r in json.load(open(os.path.join(E0, "verify.json")))["results"]:
        f, cid = r["key"].split("|", 1)
        out[(f, cid)][r["id"]] = r["check"]
    return out


def judge_sample():
    ev = load_evalset()
    rng = random.Random(SEED + 1)
    unl = [p["capture_id"] for p in ev if p["split"] == "unlabelled"]
    rng.shuffle(unl)
    return [p["capture_id"] for p in ev if p["split"] == "label"] + unl[:30]


def cmd_judge(args):
    caps = {p["capture_id"]: p for p in load_evalset()}
    chk = checks()
    sample = set(judge_sample())
    for m in EXTRACTORS:
        f = f"extract-{short(m)}.json"
        rows = [r for r in json.load(open(os.path.join(E0, f))) if "error" not in r and r["capture_id"] in sample]

        def one(r):
            cap = caps[r["capture_id"]]
            doc, ids = render(cap)
            cl = (r.get("extraction") or {}).get("claims") or []
            kept = [(i, c) for i, c in enumerate(cl) if isinstance(c, dict) and chk[(f, r["capture_id"])].get(f"c{i}", {}).get("retained")]
            if not kept:
                return {"capture_id": r["capture_id"], "judged": [], "verdicts": []}
            lines = []
            for n, (i, c) in enumerate(kept, 1):
                lines.append(json.dumps({"n": n, "speaker": (c.get("speaker") or {}).get("surface") if isinstance(c.get("speaker"), dict) else c.get("speaker"),
                                         "carrier": c.get("carrier"), "quote": c.get("quote"), "stance": c.get("stance"),
                                         "modality": c.get("modality"), "readings": c.get("readings"),
                                         "expressed_confidence": c.get("expressed_confidence"), "deadline": c.get("deadline")}, ensure_ascii=False))
            prompt = P.JUDGE_V0 + doc + "\n\nCLAIMS\n" + "\n".join(lines)
            res = L.omp(L.GEMINI, prompt)
            try:
                verdicts = L.parse_json(res["text"]).get("verdicts") or []
            except ValueError:
                verdicts = None
            return {"capture_id": r["capture_id"], "judged": [i for i, _ in kept], "verdicts": verdicts, "usage": res["usage"], "served": res["served"]}

        print("judge", m, len(rows), flush=True)
        out = pmap(one, rows, args.workers)
        json.dump(out, open(os.path.join(E0, f"judge-{short(m)}.json"), "w"), ensure_ascii=False)


def cmd_wikidata(args):
    guesses = collections.Counter()
    for m in EXTRACTORS:
        for r in json.load(open(os.path.join(E0, f"extract-{short(m)}.json"))):
            for mn in (r.get("extraction") or {}).get("mentions") or []:
                if isinstance(mn, dict) and re.fullmatch(r"Q\d+", str(mn.get("wikidata_guess") or "").strip()):
                    guesses[(short(m), mn["wikidata_guess"].strip(), str(mn.get("surface") or ""))] += 1
    qids = sorted({q for _, q, _ in guesses})
    labels = {}
    for i in range(0, len(qids), 50):
        url = "https://www.wikidata.org/w/api.php?action=wbgetentities&format=json&props=labels|aliases&ids=" + "|".join(qids[i:i + 50])
        req = urllib.request.Request(url, headers={"User-Agent": "ChoirClaimsE0/0.1 (https://choir.news) offline study"})
        ents = json.load(urllib.request.urlopen(req, timeout=60)).get("entities") or {}
        for q, e in ents.items():
            names = {v["value"].lower() for v in (e.get("labels") or {}).values()}
            for al in (e.get("aliases") or {}).values():
                names |= {a["value"].lower() for a in al}
            labels[q] = names
    rows = []
    for (m, q, surface), n in guesses.items():
        names = labels.get(q, set())
        s = surface.lower().strip()
        ok = bool(names) and any(s == x or (len(s) > 3 and (s in x or x in s)) for x in names)
        rows.append({"extractor": m, "qid": q, "exists": q in labels and bool(names), "label_match": ok, "n": n, "surface": surface})
    json.dump(rows, open(os.path.join(E0, "wikidata.json"), "w"), ensure_ascii=False)
    by = collections.defaultdict(lambda: [0, 0, 0])
    for r in rows:
        by[r["extractor"]][0] += 1
        by[r["extractor"]][1] += r["label_match"]
        by[r["extractor"]][2] += r["exists"]
    print({k: {"guesses": v[0], "label_match": v[1], "exists": v[2]} for k, v in by.items()})


# ---------------------------------------------------------------- routing

def cmd_route(args):
    """Routing diagnostic on the L2a after-window (titles only): event pairs
    the registered judge grouped (cross-outlet), the 21 same-story pairs read
    as legitimate, and the 10 read as wrong (topic feeds)."""
    win = json.load(open(os.path.expanduser("~/.config/choir-qa/traces/l2a/after-1-pairs.json")))
    wrong_markers = ["Europe live", "Playco", "Parallel", "Legora"]
    pairs = []
    for p in win["pairs"]:
        pairs.append({"a": p["a"], "b": p["b"], "gold": "same_event", "source": "judge_cross_outlet", "control_merged": bool(p.get("same_story"))})
    for p in win["false_merge_candidate_pairs"]:
        txt = p["a"] + p["b"]
        doc = p.get("doc_id", "")
        wrong = (doc.startswith("1bdba291") and "Europe" in txt and "live" in txt) or (doc.startswith("b5db8e8d") and "Playco" in txt) or doc.startswith("00762957")
        pairs.append({"a": p["a"], "b": p["b"], "gold": "different_event" if wrong else "same_event", "source": "same_story_hand_read", "control_merged": True})
    titles = sorted({x for p in pairs for x in (p["a"], p["b"])})
    idx = {t: i + 1 for i, t in enumerate(titles)}

    def ev(t):
        r = L.omp(L.LUNA, P.EVENT_FROM_TITLE_V0 + t)
        try:
            return L.parse_json(r["text"])
        except ValueError:
            return {"gloss": t}
    events = pmap(ev, titles, args.workers)
    items_ev = "\n".join(f"{idx[t]}. " + json.dumps({k: e.get(k) for k in ("gloss", "when", "where", "participants", "topic", "kind")}, ensure_ascii=False) for t, e in zip(titles, events))
    items_t = "\n".join(f"{idx[t]}. {t}" for t in titles)
    out = {"n_items": len(titles), "n_pairs": len(pairs)}
    for name, prompt, model in (("event_structured", P.ROUTE_V0 + items_ev, L.DEEPSEEK), ("titles_only", P.ROUTE_TITLES_V0 + items_t, L.DEEPSEEK)):
        r = L.omp(model, prompt)
        g = L.parse_json(r["text"])
        member = {}
        for k, e in enumerate(g.get("events") or []):
            for n in e.get("items") or []:
                member[int(n)] = k
        res = collections.Counter()
        for p in pairs:
            same = member.get(idx[p["a"]], -1) == member.get(idx[p["b"]], -2)
            res[(p["gold"], p["source"], same)] += 1
        out[name] = {f"{g_}|{s}|{'merged' if m else 'apart'}": v for (g_, s, m), v in sorted(res.items())}
    ctrl = collections.Counter((p["gold"], p["source"], p["control_merged"]) for p in pairs)
    out["control_management"] = {f"{g_}|{s}|{'merged' if m else 'apart'}": v for (g_, s, m), v in sorted(ctrl.items())}
    json.dump({"pairs": pairs, "events": dict(zip(titles, events)), "result": out}, open(os.path.join(E0, "route.json"), "w"), ensure_ascii=False)
    json.dump(out, open(os.path.join(EVID, "route-result.json"), "w"), indent=1)
    print(json.dumps(out, indent=1))


# ---------------------------------------------------------------- score

def spans_of(chk, ids):
    out = []
    for i in ids:
        c = chk.get(i)
        if c and c.get("retained"):
            out.append({"block": c["result"]["block_id"], "start": c["result"]["start"], "end": c["result"]["end"]})
        else:
            out.append(None)
    return out


def cmd_score(args):
    ev = load_evalset()
    caps = {p["capture_id"]: p for p in ev}
    chk = checks()
    over = json.load(open(os.path.join(E0, "overhead.json")))
    labels = {r["capture_id"]: r for r in json.load(open(os.path.join(E0, "labels.json"))) if "passes" in r}
    # gold: claims both labelling passes found (verified spans overlapping >= 0.5)
    gold, agree_items = {}, []
    for cid, r in labels.items():
        lc = chk[("labels.json", cid)]
        pa = [(i, c) for i, c in enumerate(r["passes"][0]["claims"] or []) if isinstance(c, dict) and lc.get(f"p0c{i}", {}).get("retained")]
        pb = [(i, c) for i, c in enumerate(r["passes"][1]["claims"] or []) if isinstance(c, dict) and lc.get(f"p1c{i}", {}).get("retained")]
        sa = spans_of(lc, [f"p0c{i}" for i, _ in pa])
        sb = spans_of(lc, [f"p1c{i}" for i, _ in pb])
        agree_items.append({"key": cid, "gold": sa, "pred": sb})
        gold[cid] = {"a": pa, "b": pb, "sa": sa, "sb": sb}
    agree = L.claimslab("match", {"min_overlap": 0.5, "items": agree_items})
    for it in agree["items"]:
        g = gold[it["key"]]
        pairs = it["match"]["pairs"] or []
        g["consensus"] = [(g["sa"][a], bool(g["a"][a][1].get("material")) or bool(g["b"][b][1].get("material"))) for a, b in pairs]
        g["union"] = g["sa"] + [g["sb"][j] for j in range(len(g["sb"])) if j not in {b for _, b in pairs}]
    n_a = sum(len(g["sa"]) for g in gold.values())
    n_b = sum(len(g["sb"]) for g in gold.values())
    n_c = sum(len(g["consensus"]) for g in gold.values())
    result = {"schema": "claims_e0.results.v1", "computed_at": dt.datetime.now(dt.timezone.utc).isoformat(),
              "evalset_manifest_sha256": json.load(open(os.path.join(EVID, "evalset-manifest.json")))["manifest_sha256"],
              "matcher": json.load(open(os.path.join(E0, "verify.json")))["matcher"],
              "labels": {"labeller": L.GEMINI, "captures": len(gold), "pass_a_retained": n_a, "pass_b_retained": n_b, "consensus": n_c,
                         "material_consensus": sum(1 for g in gold.values() for _, mt in g["consensus"] if mt),
                         "self_agreement_f1": round(2 * n_c / (n_a + n_b), 3) if n_a + n_b else None,
                         "status": "model-assisted; needs human audit (protocol §5)"},
              "extractors": {}}
    rate_groups = {}
    # every extractor on its whole run, and on the captures all extractors
    # completed (deepseek's account ran out of funds part way, so the paired
    # comparison is the fair one)
    done = [{r["capture_id"] for r in json.load(open(os.path.join(E0, f"extract-{short(m)}.json"))) if "error" not in r} for m in EXTRACTORS]
    common = set.intersection(*done)
    result["common_captures"] = len(common)
    runs = [(m, short(m), None) for m in EXTRACTORS] + [(m, short(m) + "@common", common) for m in EXTRACTORS]
    for m, name, keep in runs:
        f = f"extract-{short(m)}.json"
        rows = [r for r in json.load(open(os.path.join(E0, f))) if "error" not in r and (keep is None or r["capture_id"] in keep)]
        e = {"captures": len(rows), "parse_ok": sum(1 for r in rows if r.get("parse_ok"))}
        attempted = retained = over_cap_captures = 0
        tiers, reasons, hint_mm = collections.Counter(), collections.Counter(), 0
        by_lang = collections.defaultdict(lambda: [0, 0])
        mods, basis, hedge, multi, pattr_low, n_ret_claims = collections.Counter(), collections.Counter(), 0, 0, 0, 0
        mentions = 0
        cost_omp, cost_net, tin, tout, secs = [], [], [], [], []
        ovr = (over[m]["input"] or 0) + (over[m]["cacheRead"] or 0)
        for r in rows:
            cl = [c for c in (r.get("extraction") or {}).get("claims") or [] if isinstance(c, dict)]
            attempted += len(cl)
            over_cap_captures += len(cl) > 12
            mentions += len((r.get("extraction") or {}).get("mentions") or [])
            cc = chk[(f, r["capture_id"])]
            lang = caps[r["capture_id"]]["lang"]
            for i, c in enumerate(cl):
                k = cc.get(f"c{i}") or {}
                by_lang[lang][0] += 1
                if k.get("retained"):
                    retained += 1
                    by_lang[lang][1] += 1
                    tiers[k["result"]["tier"]] += 1
                    hint_mm += bool(k.get("hint_mismatch"))
                    n_ret_claims += 1
                    mods[str(c.get("modality"))] += 1
                    basis[str(c.get("attribution_basis"))] += 1
                    hedge += str(c.get("expressed_confidence") or "asserted").strip().lower() != "asserted"
                    multi += len(c.get("readings") or []) > 1
                    try:
                        pattr_low += float(c.get("p_attr")) < 0.6
                    except (TypeError, ValueError):
                        pass
                else:
                    reasons[k.get("reason") or "unchecked"] += 1
            u = r.get("usage") or {}
            cost_omp.append((u.get("cost") or {}).get("total") or 0)
            cost_net.append(L.list_cost(m, u, ovr) or 0)
            tin.append((u.get("input") or 0) + (u.get("cacheRead") or 0) - ovr)
            tout.append(u.get("output") or 0)
            secs.append(r.get("seconds") or 0)
        e.update({"claims_attempted": attempted, "claims_retained": retained, "retained_rate": round(retained / attempted, 4) if attempted else None,
                  "tiers": dict(tiers), "not_retained": dict(reasons), "hint_mismatch_share": round(hint_mm / retained, 3) if retained else None,
                  "captures_over_cap": over_cap_captures, "claims_per_capture": round(attempted / len(rows), 2),
                  "retained_by_lang": {k: {"attempted": v[0], "retained": v[1], "rate": round(v[1] / v[0], 3) if v[0] else None} for k, v in sorted(by_lang.items())},
                  "modalities": dict(mods), "attribution_basis": dict(basis),
                  "hedged_share": round(hedge / n_ret_claims, 3) if n_ret_claims else None,
                  "multi_reading_share": round(multi / n_ret_claims, 3) if n_ret_claims else None,
                  "p_attr_below_0.6_share": round(pattr_low / n_ret_claims, 3) if n_ret_claims else None,
                  "mentions_per_capture": round(mentions / len(rows), 2),
                  "cost": {"omp_reported_usd_per_capture": round(sum(cost_omp) / len(rows), 6),
                           "list_usd_per_capture_net_of_harness": round(sum(cost_net) / len(rows), 6),
                           "net_input_tokens_mean": round(sum(tin) / len(rows)), "output_tokens_mean": round(sum(tout) / len(rows)),
                           "seconds_median": sorted(secs)[len(secs) // 2],
                           "list_usd_per_day_at_1700_captures": round(1700 * sum(cost_net) / len(rows), 2)}})
        rate_groups[f"{name}|retained"] = {"w": [1] * attempted, "y": [1.0] * retained + [0.0] * (attempted - retained)}
        # recall against gold on the label split
        items_all, items_mat, items_union = [], [], []
        for r in rows:
            cid = r["capture_id"]
            if cid not in gold:
                continue
            cl = [c for c in (r.get("extraction") or {}).get("claims") or [] if isinstance(c, dict)]
            pred = [s for s in spans_of(chk[(f, cid)], [f"c{i}" for i in range(len(cl))]) if s]
            g = gold[cid]
            items_all.append({"key": cid, "gold": [s for s, _ in g["consensus"]], "pred": pred})
            items_mat.append({"key": cid, "gold": [s for s, mt in g["consensus"] if mt], "pred": pred})
            items_union.append({"key": cid, "gold": g["union"], "pred": pred})
        ma = L.claimslab("match", {"min_overlap": 0.5, "items": items_all})
        mm = L.claimslab("match", {"min_overlap": 0.5, "items": items_mat})
        mu = L.claimslab("match", {"min_overlap": 0.5, "items": items_union})
        e["vs_gold"] = {"captures": len(items_all), "recall_consensus": round(ma["recall"], 3), "recall_material": round(mm["recall"], 3),
                        "precision_vs_union_of_passes": round(mu["precision"], 3), "gold_consensus": ma["total"]["gold"],
                        "gold_material": mm["total"]["gold"], "predicted": ma["total"]["predicted"]}
        rate_groups[f"{name}|recall_material"] = {"w": [1] * mm["total"]["gold"], "y": [1.0] * mm["total"]["true_positives"] + [0.0] * (mm["total"]["gold"] - mm["total"]["true_positives"])}
        # validity and feasibility from the blind judge
        jpath = os.path.join(E0, f"judge-{short(m)}.json")
        verdicts = []
        if os.path.exists(jpath):
            ext_by = {r["capture_id"]: r for r in rows}
            for j in json.load(open(jpath)):
                if not j.get("verdicts") or j["capture_id"] not in ext_by:
                    continue
                cl = [c for c in (ext_by[j["capture_id"]].get("extraction") or {}).get("claims") or [] if isinstance(c, dict)]
                vs = {int(v.get("n", 0)): v for v in j["verdicts"] if isinstance(v, dict)}
                for n, ci in enumerate(j["judged"], 1):
                    if n in vs:
                        verdicts.append({**vs[n], "claim_modality": cl[ci].get("modality") if ci < len(cl) else None, "capture_id": j["capture_id"]})
        def share(key, val=True):
            return [1.0 if v.get(key) is val or v.get(key) == val else 0.0 for v in verdicts]
        for key in ("is_claim", "speaker_correct", "modality_correct", "readings_ok", "specific"):
            ys = share(key)
            rate_groups[f"{name}|{key}"] = {"w": [1] * len(ys), "y": ys}
        assertions = [v for v in verdicts if v.get("correct_modality") == "assertion"]
        rate_groups[f"{name}|assertion_count_settleable"] = {"w": [1] * len(assertions), "y": [1.0 if v.get("settle_route") in ("primary_record", "independent_sources") else 0.0 for v in assertions]}
        rate_groups[f"{name}|assertion_primary_record"] = {"w": [1] * len(assertions), "y": [1.0 if v.get("settle_route") == "primary_record" else 0.0 for v in assertions]}
        judged_caps = len({v["capture_id"] for v in verdicts}) or 1
        commits = [v for v in verdicts if v.get("correct_modality") == "commitment"]
        forecasts = [v for v in verdicts if v.get("correct_modality") == "forecast"]
        e["judge"] = {"captures": len({v["capture_id"] for v in verdicts}), "claims_judged": len(verdicts),
                      "correct_modality": dict(collections.Counter(v.get("correct_modality") for v in verdicts)),
                      "settle_route": dict(collections.Counter(v.get("settle_route") for v in verdicts)),
                      "speaker_unclear": sum(1 for v in verdicts if v.get("speaker_correct") == "unclear"),
                      "commitments_per_100_captures": round(100 * len(commits) / judged_caps, 1),
                      "checkable_commitments_per_100_captures": round(100 * sum(1 for v in commits if v.get("specific") is True) / judged_caps, 1),
                      "checkable_commitment_share": round(sum(1 for v in commits if v.get("specific") is True) / len(commits), 3) if commits else None,
                      "forecasts_per_100_captures": round(100 * len(forecasts) / judged_caps, 1),
                      "checkable_forecasts_per_100_captures": round(100 * sum(1 for v in forecasts if v.get("specific") is True) / judged_caps, 1)}
        result["extractors"][name] = e
    rates = L.claimslab("rate", {"groups": rate_groups})
    for k, v in rates.items():
        m, metric = k.split("|")
        result["extractors"][m].setdefault("rates", {})[metric] = {kk: (round(vv, 3) if isinstance(vv, float) else vv) for kk, vv in v.items()}
    # labeller cost
    lab_cost = [sum(((p.get("usage") or {}).get("cost") or {}).get("total") or 0 for p in r["passes"]) for r in labels.values()]
    result["labels"]["omp_reported_usd_per_capture_two_passes"] = round(sum(lab_cost) / len(lab_cost), 5) if lab_cost else None
    wk = os.path.join(E0, "wikidata.json")
    if os.path.exists(wk):
        w = json.load(open(wk))
        for m in EXTRACTORS:
            rr = [x for x in w if x["extractor"] == short(m)]
            result["extractors"][short(m)]["wikidata_guess"] = {"distinct_guesses": len(rr), "exists": sum(x["exists"] for x in rr),
                                                                 "label_or_alias_match": sum(x["label_match"] for x in rr),
                                                                 "precision": round(sum(x["label_match"] for x in rr) / len(rr), 3) if rr else None}
    rp = os.path.join(EVID, "route-result.json")
    if os.path.exists(rp):
        result["routing_diagnostic"] = json.load(open(rp))
    json.dump(result, open(os.path.join(EVID, "results.json"), "w"), indent=1, ensure_ascii=False)
    print(json.dumps(result, indent=1)[:8000])


if __name__ == "__main__":
    ap = argparse.ArgumentParser()
    sub = ap.add_subparsers(dest="cmd", required=True)
    f = sub.add_parser("freeze")
    f.add_argument("--pull", default=os.path.join(L.DATA, "pull"))
    f.add_argument("--scheduled", type=int, default=130)
    f.add_argument("--targeted", type=int, default=70)
    for name in ("overhead", "label", "extract", "verify", "judge", "wikidata", "route", "score"):
        s = sub.add_parser(name)
        s.add_argument("--workers", type=int, default=8)
        s.add_argument("--model", default="")
    a = ap.parse_args()
    {"freeze": cmd_freeze, "overhead": cmd_overhead, "label": cmd_label, "extract": cmd_extract, "verify": cmd_verify,
     "judge": cmd_judge, "wikidata": cmd_wikidata, "route": cmd_route, "score": cmd_score}[a.cmd](a)
