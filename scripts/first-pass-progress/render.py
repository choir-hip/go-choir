#!/usr/bin/env python3
"""Render the first-pass progress sections of the status page from items.json.

Stage ladder (per slice): 0 queued, 1 building, 2 in review, 3 accepted,
4 landed, 5 deployed, 6 proved. An item's progress is the mean of its
slices' stage weights; superseded / deferred_owner items leave the
denominator. History is rebuilt from each slice's dated stage entries.

usage: render.py ITEMS_JSON PAGE_HTML
"""
import html
import json
import re
import sys
from datetime import datetime, timedelta, timezone

STAGES = ["Queued", "Building", "In review", "Accepted", "Landed", "Deployed", "Proved"]
WEIGHT = [0.0, 0.2, 0.35, 0.5, 0.65, 0.8, 1.0]
PASS_START = datetime(2026, 10, 21, 18, 0, tzinfo=timezone.utc)
e = html.escape


def ts(s):
    s = s.strip().replace("Z", "+00:00")
    if len(s) == 16:  # 2026-10-22T19:50+00:00 without seconds
        pass
    try:
        d = datetime.fromisoformat(s)
    except ValueError:
        d = datetime.fromisoformat(s[:16] + "+00:00")
    return d if d.tzinfo else d.replace(tzinfo=timezone.utc)


def live(item):
    return all(isinstance(s["stage"], int) for s in item["slices"])


def item_progress(item, at=None):
    vals = []
    for sl in item["slices"]:
        vals.append(WEIGHT[slice_stage(sl, at)])
    return sum(vals) / len(vals)


def slice_stage(sl, at=None):
    if at is None:
        return sl["stage"]
    best = 0
    for h in sl.get("history") or []:
        try:
            if ts(h["at"]) <= at and isinstance(h["stage"], int):
                best = max(best, h["stage"])
        except (KeyError, ValueError):
            continue
    return min(best, sl["stage"])


def item_stage(item):
    """The stage every slice has reached (the item's floor) and its ceiling."""
    st = [s["stage"] for s in item["slices"]]
    return min(st), max(st)


def render(data):
    items = data["items"]
    now = ts(data["as_of"])
    act = [i for i in items if live(i)]
    out_of = [i for i in items if not live(i)]
    n = len(act)
    pct = 100 * sum(item_progress(i) for i in act) / n

    # Item-equivalents by stage: each item is 1, split across its slices.
    share = [0.0] * 7
    for i in act:
        for s in i["slices"]:
            share[s["stage"]] += 1 / len(i["slices"])
    # At or beyond: an item counts when every slice is at least that stage.
    beyond = [sum(1 for i in act if item_stage(i)[0] >= k) for k in range(7)]
    started = [sum(1 for i in act if item_stage(i)[1] >= k) for k in range(7)]

    # History, every 30 minutes from the pass start, plus now.
    pts = []
    t = PASS_START
    while t < now:
        pts.append((t, 100 * sum(item_progress(i, t) for i in act) / n))
        t += timedelta(minutes=30)
    pts.append((now, pct))
    day0 = datetime(now.year, now.month, now.day, tzinfo=timezone.utc)
    at_day0 = 100 * sum(item_progress(i, day0) for i in act) / n
    moved = []
    for i in act:
        before = max(slice_stage(s, day0) for s in i["slices"])
        after = max(s["stage"] for s in i["slices"])
        if after > before:
            moved.append((i, before, after))

    pri = {}
    for p in ("P0", "P1", "P2"):
        grp = [i for i in act if i.get("priority") == p]
        if grp:
            pri[p] = (100 * sum(item_progress(i) for i in grp) / len(grp), len(grp),
                      sum(1 for i in grp if item_stage(i)[0] >= 6))

    h = []
    h.append('<section aria-label="Checklist progress" id="progress">')
    h.append('<h2>Progress: %.0f%% of the checklist</h2>' % pct)
    h.append('<p class="hero"><b>%.0f%%</b><span>weighted by stage across %d items; '
             '+%.0f points since midnight UTC, +%.0f since the pass began</span></p>'
             % (pct, n, pct - at_day0, pct - pts[0][1]))
    # Stacked stage bar (item-equivalents).
    segs = []
    for k in range(6, -1, -1):
        if share[k] <= 0:
            continue
        w = 100 * share[k] / n
        segs.append('<span class="st%d" style="width:%.2f%%" title="%s: %.1f item-equivalents"></span>'
                    % (k, w, STAGES[k], share[k]))
    h.append('<div class="sbar" role="img" aria-label="%s">%s</div>' % (
        e(", ".join("%s %.1f" % (STAGES[k], share[k]) for k in range(6, -1, -1))), "".join(segs)))
    h.append('<div class="legend">' + "".join(
        '<span><i class="dot st%d"></i>%s %s</span>' % (k, STAGES[k], ("%.1f" % share[k]).rstrip("0").rstrip("."))
        for k in range(6, -1, -1)) + '</div>')
    # Cumulative ladder.
    h.append('<div class="ladder">')
    for k in range(6, 0, -1):
        h.append('<div class="rung"><span class="lbl">%s or further</span>'
                 '<span class="track"><span class="fill st%d" style="width:%.2f%%"></span>'
                 '<span class="part st%d" style="width:%.2f%%"></span></span>'
                 '<span class="num"><b>%d</b> of %d<small>%s</small></span></div>'
                 % (STAGES[k], k, 100 * beyond[k] / n, k, 100 * (started[k] - beyond[k]) / n,
                    beyond[k], n, ("+%d partly" % (started[k] - beyond[k])) if started[k] > beyond[k] else ""))
    h.append('</div>')
    h.append('<p class="upd">An item counts at a stage when every part of it has reached that stage; the lighter extension counts items with some part there.</p>')
    # Priority tiles.
    h.append('<div class="pri">' + "".join(
        '<div><b>%.0f%%</b> %s<small>%d items, %d proved</small></div>' % (v[0], p, v[1], v[2])
        for p, v in pri.items()) + '</div>')
    # Sparkline.
    W, H, P = 640, 120, 6
    x0, x1 = pts[0][0], pts[-1][0]
    span = (x1 - x0).total_seconds() or 1
    ymax = max(10, ((max(v for _, v in pts) // 10) + 1) * 10)
    def xy(t, v):
        return (36 + (W - 36 - P) * (t - x0).total_seconds() / span, H - P - (H - 2 * P) * v / ymax)
    path = " ".join(("M" if j == 0 else "L") + "%.1f,%.1f" % xy(t, v) for j, (t, v) in enumerate(pts))
    dots = "".join('<circle cx="%.1f" cy="%.1f" r="7" class="hit"><title>%s UTC: %.0f%%</title></circle>'
                   % (*xy(t, v), t.strftime("%b %d %H:%M"), v) for t, v in pts)
    grid = "".join('<line x1="%d" x2="%d" y1="%.1f" y2="%.1f" class="grid"/><text x="%d" y="%.1f" class="ax" text-anchor="end">%d%%</text>'
                   % (36, W - P, xy(x0, g)[1], xy(x0, g)[1], 30, xy(x0, g)[1] + 3, g)
                   for g in range(0, int(ymax) + 1, 10 if ymax <= 50 else 20))
    lx, ly = xy(*pts[-1])
    h.append('<figure class="spark"><figcaption>Weighted progress over the pass (UTC), from each part\'s dated stage changes</figcaption>'
             '<svg viewBox="0 0 %d %d" role="img" aria-label="Progress rose from 0%% at %s to %.0f%% at %s">'
             '%s<path d="%s" class="line"/><circle cx="%.1f" cy="%.1f" r="4" class="end"/>%s</svg>'
             '<div class="axis"><span>%s</span><span>%s</span></div></figure>'
             % (W, H, x0.strftime("%b %d %H:%M"), pct, x1.strftime("%b %d %H:%M"), grid, path, lx, ly, dots,
                x0.strftime("%b %d %H:%M"), x1.strftime("%b %d %H:%M")))
    if moved:
        moved.sort(key=lambda m: (-(m[2] - m[1]), m[0]["id"]))
        h.append('<details class="sect"><summary>Moved since midnight UTC <span class="upd">%d items</span></summary><ul class="plain moved">' % len(moved))
        for i, a, b in moved:
            h.append('<li><span class="mono">%s</span> %s <span class="upd">%s → %s</span></li>'
                     % (e(i["id"]), e(i["title"]), STAGES[a], STAGES[b]))
        h.append('</ul></details>')
    if out_of:
        h.append('<p class="upd">Out of this pass: %s.</p>' % e(", ".join("%s (%s)" % (i["id"], i["slices"][0]["stage"]) for i in out_of)))
    h.append('</section>')
    progress = "\n".join(h)

    # Every item, grouped by section.
    secs = {}
    for i in items:
        secs.setdefault(i.get("section", "Other"), []).append(i)
    g = ['<section id="items"><h2>Every checklist item</h2>']
    for sec, grp in secs.items():
        lv = [i for i in grp if live(i)]
        sp = 100 * sum(item_progress(i) for i in lv) / len(lv) if lv else 0
        g.append('<details class="sect"><summary>%s <span class="upd">%.0f%% · %d items</span></summary>' % (e(sec), sp, len(grp)))
        for i in grp:
            if live(i):
                prog = item_progress(i)
                pips = "".join(
                    '<span class="pips" title="%s: %s">%s</span>' % (e(s["name"]), STAGES[s["stage"]], "".join(
                        '<i class="%s"></i>' % ("st%d" % s["stage"] if k <= s["stage"] else "off") for k in range(1, 7)))
                    for s in i["slices"])
                slices = "".join('<li><span class="pill-st st%d">%s</span> %s<span class="ev">%s</span></li>'
                                 % (s["stage"], STAGES[s["stage"]], e(s["name"]), e(s.get("evidence", "")))
                                 for s in i["slices"])
                g.append('<details class="item2"><summary><span class="mono">%s</span> <span class="ttl">%s</span>'
                         '<span class="meta2">%s · %s · %s</span><span class="pp">%s<b>%.0f%%</b></span></summary>'
                         '<ul class="plain sl">%s</ul>%s</details>'
                         % (e(i["id"]), e(i["title"]), e(i.get("priority", "")), e(i.get("class", "")), e(i.get("lane", "")),
                            pips, 100 * prog, slices,
                            ('<p class="nx">Next: %s</p>' % e(i["next"])) if i.get("next") else ""))
            else:
                g.append('<div class="item2 out"><span class="mono">%s</span> <span class="ttl">%s</span> <span class="upd">%s</span></div>'
                         % (e(i["id"]), e(i["title"]), e(str(i["slices"][0]["stage"]).replace("_", " "))))
        g.append('</details>')
    g.append('</section>')
    return progress, "\n".join(g), pct


CSS = """
/* progress-css:start */
:root { --st1:#86b6ef; --st2:#5598e7; --st3:#2a78d6; --st4:#1c5cab; --st5:#104281; --st6:#0d2a52; --st0: var(--queued); }
@media (prefers-color-scheme: dark) { :root:not([data-theme="light"]) { --st1:#184f95; --st2:#256abf; --st3:#3987e5; --st4:#6da7ec; --st5:#9ec5f4; --st6:#cde2fb; } }
:root[data-theme="dark"] { --st1:#184f95; --st2:#256abf; --st3:#3987e5; --st4:#6da7ec; --st5:#9ec5f4; --st6:#cde2fb; }
.st0{background:var(--st0)} .st1{background:var(--st1)} .st2{background:var(--st2)} .st3{background:var(--st3)} .st4{background:var(--st4)} .st5{background:var(--st5)} .st6{background:var(--st6)}
.hero { display:flex; align-items:baseline; gap:12px; flex-wrap:wrap; margin:4px 0 10px; }
.hero b { font-size:2.6rem; line-height:1; font-variant-numeric:tabular-nums; }
.hero span { color:var(--muted); }
.sbar { display:flex; gap:2px; height:14px; }
.sbar span { display:block; height:100%; }
.sbar span:first-child { border-radius:4px 0 0 4px; } .sbar span:last-child { border-radius:0 4px 4px 0; }
.ladder { display:grid; gap:6px; margin-top:14px; }
.rung { display:grid; grid-template-columns: 9.5rem 1fr 7.5rem; gap:10px; align-items:center; font-size:0.9rem; }
.rung .track { display:flex; height:10px; background:var(--line); border-radius:4px; overflow:hidden; }
.rung .fill { display:block; height:100%; }
.rung .part { display:block; height:100%; opacity:0.38; }
.rung .num { font-variant-numeric:tabular-nums; }
.rung .num small { color:var(--muted); margin-left:6px; }
.spark { margin:14px 0 0; }
.spark figcaption { color:var(--muted); font-size:0.85rem; margin-bottom:4px; }
.spark svg { width:100%; height:auto; display:block; }
.spark .line { fill:none; stroke:var(--st4); stroke-width:2; }
.spark .end { fill:var(--st4); stroke:var(--surface); stroke-width:2; }
.spark .grid { stroke:var(--line); stroke-width:1; }
.spark .ax { fill:var(--muted); font-size:10px; }
.spark .hit { fill:transparent; }
.spark .hit:hover { fill:var(--st4); fill-opacity:0.25; }
.spark .axis { display:flex; justify-content:space-between; color:var(--muted); font-size:0.8rem; }
.moved li { padding:3px 0; }
.item2 { border-top:1px solid var(--line); padding:6px 0; }
.item2 > summary { display:flex; flex-wrap:wrap; gap:4px 10px; align-items:center; cursor:pointer; list-style:none; }
.item2 > summary::-webkit-details-marker { display:none; }
.item2 .ttl { flex:1 1 14rem; }
.item2 .meta2 { color:var(--muted); font-size:0.82rem; }
.item2 .pp { display:flex; gap:6px; align-items:center; margin-left:auto; font-variant-numeric:tabular-nums; }
.item2 .pp b { min-width:2.8rem; text-align:right; }
.pips { display:inline-flex; gap:2px; }
.pips i { display:inline-block; width:8px; height:8px; border-radius:2px; }
.pips i.off { background:var(--line); }
.sl { margin:6px 0 0 0; font-size:0.88rem; }
.sl li { padding:2px 0; }
.sl .ev { display:block; color:var(--muted); font-size:0.8rem; margin-left:0.2rem; }
.pill-st { display:inline-block; font-size:0.75rem; padding:1px 7px; border-radius:9px; color:#fff; margin-right:4px; }
.pill-st.st0, .pill-st.st1 { color:#0d2a52; }
:root[data-theme="dark"] .pill-st.st4, :root[data-theme="dark"] .pill-st.st5, :root[data-theme="dark"] .pill-st.st6 { color:#0d1a2e; }
@media (prefers-color-scheme: dark) { :root:not([data-theme="light"]) .pill-st.st4, :root:not([data-theme="light"]) .pill-st.st5, :root:not([data-theme="light"]) .pill-st.st6 { color:#0d1a2e; } }
.nx { color:var(--muted); font-size:0.85rem; margin:4px 0 0; }
.item2.out { color:var(--muted); }
@media (max-width: 560px) { .rung { grid-template-columns: 7rem 1fr 5.5rem; font-size:0.82rem; } }
/* progress-css:end */
"""


def main():
    data = json.load(open(sys.argv[1]))
    page = open(sys.argv[2]).read()
    progress, items, pct = render(data)
    if "/* progress-css:start */" in page:
        page = re.sub(r"/\* progress-css:start \*/.*?/\* progress-css:end \*/", lambda m: CSS.strip(), page, flags=re.S)
    else:
        page = page.replace("</style>", CSS + "</style>", 1)
    for pat, new in ((r'<section aria-label="Checklist progress".*?</section>', progress),
                     (r'<section( id="items")?><h2>Every checklist item</h2>.*?</section>', items)):
        page, n = re.subn(pat, lambda m: new, page, count=1, flags=re.S)
        assert n == 1, pat
    open(sys.argv[2], "w").write(page)
    print("rendered %.1f%%" % pct)


if __name__ == "__main__":
    main()
