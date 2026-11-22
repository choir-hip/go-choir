#!/usr/bin/env python3
"""Rung 0b numbers from a results-b*.jsonl (internal/platform TestOfflineSliceReplayB).
   report-b.py RESULTS [--day YYYY-MM-DD]   (--day: print what the cleaner dropped and kept that day)"""
import json, collections, sys
path = sys.argv[1]
day = sys.argv[sys.argv.index("--day") + 1] if "--day" in sys.argv else None
rs = [json.loads(l) for l in open(path)]
def q(v, p):
    v = sorted(v); return v[int(p * (len(v) - 1))]
def dist(v):
    return "mean %.1f p10 %d p50 %d p90 %d p99 %d max %d" % (sum(v) / len(v), q(v, .1), q(v, .5), q(v, .9), q(v, .99), max(v))
def pct(a, b): return "%.1f%%" % (100 * a / b) if b else "n/a"
days = sorted({r["day"] for r in rs}); classes = sorted({r["class"] for r in rs})
if day:
    drop = collections.Counter(); kept = collections.Counter()
    for r in rs:
        if r["day"] == day:
            for k in r.get("dropped_keys", []): drop[k] += 1
            for k in r.get("kept_keys", []): kept[k] += 1
    print(day, "arrivals", sum(r["day"] == day for r in rs), "mean clean_count %.2f" % (sum(r["clean_count"] for r in rs if r["day"] == day) / sum(r["day"] == day for r in rs)))
    print("dropped:", drop.most_common(45)); print("kept (most frequent):", kept.most_common(45)); sys.exit()
print("source arrivals replayed:", len(rs))
cd = collections.Counter((r["class"], r["day"]) for r in rs)
print("class      " + " ".join(d[5:] for d in days) + "  total")
for c in classes: print("%-10s" % c, " ".join("%5d" % cd[(c, d)] for d in days), "%6d" % sum(cd[(c, d)] for d in days))
print("%-10s" % "all", " ".join("%5d" % sum(cd[(c, d)] for c in classes) for d in days), "%6d" % len(rs))
print("\nlist length (distinct entities + new names) per arrival:")
print("  deployed rules on stored text :", dist([r["raw_count"] for r in rs]), "| zero", pct(sum(r["raw_count"] == 0 for r in rs), len(rs)))
print("  after cleaning (no stories)   :", dist([r["clean_count"] for r in rs]), "| zero", pct(sum(r["clean_count"] == 0 for r in rs), len(rs)))
print("  before cap (with stories)     :", dist([r["pre_cap"] for r in rs]))
print("  final list (cap 12)           :", dist([len(r["final"]) for r in rs]))
print("  cap bites (pre-cap > 12)      :", pct(sum(r["pre_cap"] > 12 for r in rs), len(rs)))
dr = collections.Counter(); tot = 0
for r in rs:
    for k, v in r.get("dropped", {}).items(): dr[k] += v
    tot += r["mentions"]
print("  mentions", tot, "dropped by reason", dict(dr))
print("  by day: mean raw / clean / final:", ["%s %.1f/%.1f/%.1f" % (d[5:], *[sum(r[k] if k != "final" else len(r["final"]) for r in rs if r["day"] == d) / sum(r["day"] == d for r in rs) for k in ("raw_count", "clean_count", "final")]) for d in days])
kinds = collections.Counter(c["kind"] for r in rs for c in r["final"]); print("  final entries by kind:", dict(kinds))
fin = collections.Counter(); kept = collections.Counter()
for r in rs:
    for c in r["final"]:
        if c["kind"] != "story": fin[c["label"]] += 1
    for k in r.get("kept_keys", []): kept[k] += 1
print("\n30 most-proposed after cleaning (mention keys):", kept.most_common(30))
print("\n30 most-listed non-story entries in final lists:", fin.most_common(30))
s1 = [r["stories"][0]["cos"] for r in rs if r["stories"]]
print("\nnearest story cosine: n %d" % len(s1), "p10 %.2f p25 %.2f p50 %.2f p75 %.2f p90 %.2f" % tuple(q(s1, p) for p in (.1, .25, .5, .75, .9)), "| no story:", sum(not r["stories"] for r in rs))
for k in (5, 10, 20):
    s = [r["stories"][k - 1]["cos"] for r in rs if len(r["stories"]) >= k]
    print("  cosine of the %dth story: p10 %.2f p50 %.2f p90 %.2f" % (k, q(s, .1), q(s, .5), q(s, .9)))
o = collections.Counter(r["oracle"] for r in rs)
print("\noracle (stories citing the capture; fed = a story that existed before arrival cites it; new = only later stories cite it; none = no story cites it):", dict(o))
fed = [r for r in rs if r["oracle"] == "fed"]
print("fed arrivals:", len(fed))
for K in (5, 10, 20):
    h = sum(1 for r in fed if 0 < r["oracle_rank"] <= K); c = sum(1 for r in fed if 0 < r.get("recent_rank", 0) <= K)
    print("  K=%2d  nearest-by-vector hit %d/%d = %s    control (most recently revised stories) %d/%d = %s" % (K, h, len(fed), pct(h, len(fed)), c, len(fed), pct(c, len(fed))))
for c in classes:
    f = [r for r in fed if r["class"] == c]
    if f: print("  %-9s fed %4d  hit@5 %s @10 %s @20 %s   status %s" % (c, len(f), *[pct(sum(1 for r in f if 0 < r["oracle_rank"] <= K), len(f)) for K in (5, 10, 20)], dict(collections.Counter(r["oracle"] for r in rs if r["class"] == c))))
for d in days:
    f = [r for r in fed if r["day"] == d]; n = sum(r["day"] == d for r in rs)
    print("  %s arrivals %5d fed %4d hit@10 %s zero-clean %s" % (d, n, len(f), pct(sum(1 for r in f if 0 < r["oracle_rank"] <= 10), len(f)), pct(sum(r["clean_count"] == 0 for r in rs if r["day"] == d), n)))
