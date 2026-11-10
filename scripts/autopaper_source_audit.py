#!/usr/bin/env python3
"""Read-only source and story audit of the published Autopaper (choir.news).

Reads only public pages and public JSON, with no key and no action on any
computer:
  GET /api/platform/publications/front?limit=100[&cursor=...]   (the list)
  GET /api/platform/publications/resolve?route=<permalink>      (one story)

Politeness: one request at a time, at most ~3 requests a second, every story
version cached on disk (versions are immutable), User-Agent names the audit.
Raw article bodies stay in the cache directory (outside the repo); the outputs
carry only counts, ids, the paper's own routes, source domains and titles cut
to 12 words.

Usage:
  scripts/autopaper_source_audit.py --since 7d --out docs/evidence/autopaper-source-audit-2026-11-10
  scripts/autopaper_source_audit.py --since 7d --out DIR --diff PREV/summary.json
  scripts/autopaper_source_audit.py --offline ...   # cache only, no network

Stdlib only. Method notes live in the evidence README.
"""
import argparse
import csv
import datetime as dt
import hashlib
import json
import os
import re
import statistics
import sys
import time
import urllib.error
import urllib.parse
import urllib.request
from collections import Counter, defaultdict

BASE = "https://choir.news"
UA = "choir-source-audit/1 (read-only; public pages only)"
MIN_INTERVAL = 0.34  # seconds between requests: under 3 a second
DEFAULT_CACHE = "/private/tmp/claude-501/audit-cache"
DEFAULT_BASELINE = os.path.join(
    os.path.dirname(os.path.abspath(__file__)), "..", "docs", "evidence",
    "autopaper-quality-audit-2026-11-09", "all_articles.csv")

# Headline values of the earlier audit (docs/problems/autopaper-article-quality-2026-11-09.md).
EARLIER = {
    "corpus": 5736,
    "share_under_300": 4487 / 5736,
    "share_under_150": 2930 / 5736,
    "dup_groups": 74,
    "dup_stories": 161,
    "non_english": 32,
    "no_section_share": 2974 / 5736,
    "placeholders": 33,
    "casting_brief_titles": 18,
    "adjacent_dup_cite_share": 169 / 663,
}

# ---------------------------------------------------------------- language

_STOP = {
    "en": "the and of to in is for on that with as at by from it this are was be has have an or not but will its their said says after over".split(),
    "fr": "le la les des du de un une et en est pour dans que qui sur au aux par avec pas se ses son sa plus sont ce cette il elle nous vous au du une".split(),
    "de": "der die das und ist von zu den mit sich des auf für nicht ein eine dem im als auch es an werden aus er sie bei nach wird".split(),
    "es": "el la los las del y en que un una por con para es se su al lo como más pero sus le ya fue este".split(),
    "pt": "os de do da dos das em que um uma por com para é se seu sua ao como mais mas foi não são uma".split(),
    "it": "il lo la gli le di del della dei delle e in che un una per con è si su al come più ma sono non".split(),
    "nl": "de het een en van in is dat op te voor met zijn niet aan er als ook bij door naar uit wordt".split(),
}
_STOPSETS = {k: set(v) for k, v in _STOP.items()}
_WORD = re.compile(r"[a-zà-ÿœ']+", re.I)


def detect_lang(text, min_hits=2):
    """Stopword vote. Returns a language code or 'und' when the text is too
    short or too close to call. Heuristic; compare run to run, not as truth."""
    toks = [t.lower().strip("'") for t in _WORD.findall(text or "")]
    if len(toks) < 4:
        return "und"
    hits = {k: sum(1 for t in toks if t in s) for k, s in _STOPSETS.items()}
    ranked = sorted(hits.items(), key=lambda kv: -kv[1])
    (best, bh), (_, sh) = ranked[0], ranked[1]
    if bh < min_hits or bh < 1.5 * sh:
        return "und"
    return best


# ---------------------------------------------------------------- domains

_SLD = {"co.uk", "org.uk", "gov.uk", "ac.uk", "com.au", "net.au", "org.au", "co.jp",
        "com.br", "co.nz", "co.za", "com.mx", "com.cn", "co.in", "com.tr", "co.kr", "com.sg", "com.hk",
        "com.ua", "org.ua", "co.il", "com.ar", "com.co", "com.pk", "com.ng", "co.ke", "com.eg", "com.ph", "co.id", "com.my",
        "com.vn", "com.pl", "co.th", "or.jp", "ne.jp", "go.jp", "com.pe", "com.ve", "com.bd", "com.np", "co.tz", "co.ug"}


SOCIAL = {"bsky.app", "x.com", "twitter.com", "reddit.com", "news.ycombinator.com", "t.me", "youtube.com", "facebook.com", "mastodon.social"}
PROXY = {"jina.ai", "europesays.com", "news.google.com", "archive.org", "archive.ph", "archive.is", "webcache.googleusercontent.com",
         "msn.com", "newsbreak.com", "flipboard.com", "yahoo.com"}
TWIN = {"bbc.co.uk": "bbc.com", "abcnews.com": "abcnews.go.com", "go.com": "abcnews.go.com", "abcnews.go.com": "abcnews.go.com"}


NOT_OUTLETS = {"ycombinator.com", "google.com", "reddit.com", "lobste.rs", "yahoo.com", "aol.com", "aol.co.uk", "msn.com",
               "bing.com", "duckduckgo.com", "flipboard.com", "newsbreak.com"}


def unwrap(uri):
    """Mirror internal/outlet unwrapProxy: r.jina.ai/<url>, web.archive.org/web/<ts>/<url>."""
    for marker in ("r.jina.ai/", "web.archive.org/web/"):
        if marker in (uri or "")[:40]:
            rest = uri.split(marker, 1)[1]
            if marker.startswith("web.") and "/" in rest:
                rest = rest.split("/", 1)[1]
            if rest.startswith(("http://", "https://")):
                return rest
    return uri


def outlet_of(uri):
    """Port of internal/outlet.Of for registrable hosts: '' for non-outlets
    (aggregators, search pages, Bluesky/Fediverse posts)."""
    uri = unwrap(uri)
    d = domain_of(uri)
    if not d or d in NOT_OUTLETS or d == "bsky.app":
        return ""
    return d


def domain_of(uri):
    uri = unwrap(uri)
    try:
        host = urllib.parse.urlparse(uri).hostname or ""
    except ValueError:
        return ""
    host = host.lower()
    if host.startswith("www."):
        host = host[4:]
    parts = host.split(".")
    if len(parts) >= 3 and ".".join(parts[-2:]) in _SLD:
        return ".".join(parts[-3:])
    if len(parts) >= 2:
        return ".".join(parts[-2:])
    return host


_URLDATE = re.compile(r"/(20\d\d)[/-](0[1-9]|1[0-2])[/-](0[1-9]|[12]\d|3[01])(?:/|-|$)")
_URLDATE2 = re.compile(r"[/-](20\d\d)(0[1-9]|1[0-2])([0-3]\d)(?:[/-]|$|\.)")


def url_date(uri):
    m = _URLDATE.search(uri or "") or _URLDATE2.search(uri or "")
    if not m:
        return None
    try:
        return dt.date(int(m.group(1)), int(m.group(2)), int(m.group(3)))
    except ValueError:
        return None


# ---------------------------------------------------------------- text checks

_ADJ = re.compile(r"\[(\d+)\]\s*\[\1\]")
_BRIEF = re.compile(r"^\s*(write the story|write a story|open a (new )?story|cast\b)", re.I)
_PLACE = re.compile(r"^\s*(test|probe block\.?|\[source \d+[^\]]*\])\s*$", re.I)
_ROUNDUP = re.compile(r"\b(round-?up|digest|briefing|newsletter|live updates?|what to know|week in|headlines|top stories|in brief|morning wire|daily wrap)\b", re.I)


def short_title(t, n=12):
    w = (t or "").split()
    return " ".join(w[:n]) + ("..." if len(w) > n else "")


def pub_key(pub_id):
    """First 9 hex chars of the id: the suffix carried by every route slug."""
    h = (pub_id or "").replace("pub-", "")
    return h[:9]


def title_tokens(t):
    return frozenset(w for w in re.findall(r"[a-z0-9]+", (t or "").lower()) if len(w) > 2)


def dup_groups(items, threshold=0.6, window_hours=48):
    """items: dicts with id,title,ts(datetime). Union-find over title Jaccard
    >= threshold within window_hours. Returns (groups, stories_in_groups)."""
    items = sorted(items, key=lambda x: x["ts"])
    toks = [title_tokens(x["title"]) for x in items]
    parent = list(range(len(items)))

    def find(i):
        while parent[i] != i:
            parent[i] = parent[parent[i]]
            i = parent[i]
        return i

    for i in range(len(items)):
        if not toks[i]:
            continue
        for j in range(i - 1, -1, -1):
            if (items[i]["ts"] - items[j]["ts"]).total_seconds() > window_hours * 3600:
                break
            if not toks[j]:
                continue
            inter = len(toks[i] & toks[j])
            union = len(toks[i] | toks[j])
            if union and inter / union >= threshold:
                parent[find(i)] = find(j)
    groups = defaultdict(list)
    for i in range(len(items)):
        groups[find(i)].append(items[i]["id"])
    multi = [g for g in groups.values() if len(g) > 1]
    return multi, sum(len(g) for g in multi)


# ---------------------------------------------------------------- fetching

class Fetcher:
    def __init__(self, cache_dir, offline=False):
        self.cache = cache_dir
        self.offline = offline
        self.last = 0.0
        self.n_net = 0
        self.n_cache = 0
        self.errors = Counter()
        os.makedirs(os.path.join(cache_dir, "stories"), exist_ok=True)
        os.makedirs(os.path.join(cache_dir, "front"), exist_ok=True)

    def _get(self, url):
        if self.offline:
            return {"_http": -1}
        for attempt in range(4):
            wait = MIN_INTERVAL - (time.time() - self.last)
            if wait > 0:
                time.sleep(wait)
            self.last = time.time()
            req = urllib.request.Request(url, headers={"User-Agent": UA, "Accept": "application/json"})
            try:
                with urllib.request.urlopen(req, timeout=40) as r:
                    self.n_net += 1
                    return json.loads(r.read().decode("utf-8"))
            except urllib.error.HTTPError as e:
                self.n_net += 1
                self.errors[e.code] += 1
                if e.code in (401, 403, 404):
                    return {"_http": e.code}
                time.sleep(2 ** attempt * 2)
            except (urllib.error.URLError, TimeoutError, json.JSONDecodeError):
                self.errors["net"] += 1
                time.sleep(2 ** attempt * 2)
        return {"_http": 0}

    def front_page(self, cursor, run_id):
        key = hashlib.sha1(((cursor or "") + run_id).encode()).hexdigest()
        path = os.path.join(self.cache, "front", key + ".json")
        if os.path.exists(path):
            self.n_cache += 1
            return json.load(open(path))
        q = {"limit": "100"}
        if cursor:
            q["cursor"] = cursor
        d = self._get(BASE + "/api/platform/publications/front?" + urllib.parse.urlencode(q))
        if "_http" not in d:
            json.dump(d, open(path, "w"))
        return d

    def story(self, permalink):
        key = hashlib.sha1(permalink.encode()).hexdigest()
        path = os.path.join(self.cache, "stories", key + ".json")
        if os.path.exists(path):
            self.n_cache += 1
            return json.load(open(path))
        d = self._get(BASE + "/api/platform/publications/resolve?" + urllib.parse.urlencode({"route": permalink}))
        if "_http" not in d:
            json.dump(d, open(path, "w"))
        return d


def parse_ts(s):
    if not s:
        return None
    s = re.sub(r"(\.\d{6})\d+", r"\1", s.replace("Z", "+00:00"))
    try:
        return dt.datetime.fromisoformat(s)
    except ValueError:
        return None


def parse_duration_or_time(s, now):
    m = re.fullmatch(r"(\d+)([dh])", s)
    if m:
        n = int(m.group(1))
        return now - (dt.timedelta(days=n) if m.group(2) == "d" else dt.timedelta(hours=n))
    t = parse_ts(s if "T" in s else s + "T00:00:00+00:00")
    if t is None:
        raise SystemExit("bad --since: " + s)
    return t if t.tzinfo else t.replace(tzinfo=dt.timezone.utc)


# ---------------------------------------------------------------- parsing

def parse_story(item, res):
    """item: one entry of the front list; res: the resolve JSON (or None).
    Returns the per-story record. Never keeps the body text."""
    rec = {
        "id": item["publication_id"],
        "key": pub_key(item["publication_id"]),
        "title": item.get("title", ""),
        "dek_lang": detect_lang(item.get("dek", "")),
        "title_lang": detect_lang(item.get("title", "")),
        "route": (item.get("route") or {}).get("permalink", ""),
        "updated_at": item.get("updated_at", ""),
        "revision": item.get("latest_revision", 0),
        "version_count": item.get("version_count", 1),
        "section": item.get("section") or "",
        "importance": item.get("importance"),
        "list_sources": item.get("source_count", 0),
        "words": item.get("words", 0),
        "fetched": bool(res and "_http" not in res),
    }
    rec["ts"] = parse_ts(rec["updated_at"])
    if not rec["fetched"]:
        return rec
    body = ((res.get("artifact") or {}).get("content")) or ""
    lines = body.split("\n")
    text = "\n".join(lines[1:]) if lines and lines[0].startswith("#") else body
    rec["body_lang"] = detect_lang(text, min_hits=4)
    rec["lang"] = rec["body_lang"] if rec["body_lang"] != "und" else (
        rec["title_lang"] if rec["title_lang"] != "und" else rec["dek_lang"])
    rec["adjacent_dup_cites"] = len(_ADJ.findall(body))
    rec["placeholder"] = bool(rec["words"] < 50 and (_PLACE.match(text.strip()) or _PLACE.match(rec["title"])))
    rec["brief_title"] = bool(_BRIEF.match(rec["title"]))
    rec["roundup"] = bool(_ROUNDUP.search(rec["title"]))
    ents = {}
    for e in res.get("source_entities") or []:
        ent = e.get("entity") or {}
        tgt = ent.get("target") or {}
        uri = tgt.get("uri") or ""
        sid = e.get("source_entity_id") or ent.get("source_entity_id")
        if sid and uri:
            ents[sid] = {"uri": uri, "domain": domain_of(uri), "outlet": outlet_of(uri),
                         "title_lang": detect_lang(((ent.get("display") or {}).get("title")) or ""),
                         "url_date": url_date(uri)}
    anchors = (((res.get("payload") or {}).get("manifest")) or {}).get("anchors") or []
    cites = Counter()
    anchor_ent = {}
    for a in anchors:
        sid = a.get("source_entity_id")
        if sid in ents:
            cites[sid] += 1
            anchor_ent[a.get("anchor_id")] = sid
    # Entities present but never anchored still count as cited sources of the page.
    rec["sources"] = [{"domain": ents[s]["domain"], "cites": cites.get(s, 0),
                       "outlet": ents[s]["outlet"], "title_lang": ents[s]["title_lang"], "url_date": ents[s]["url_date"]} for s in ents]
    uris = {ents[s]["uri"] for s in ents}
    rec["n_sources"] = len(uris)
    rec["all_domains"] = sorted({ents[s]["domain"] for s in ents if ents[s]["domain"]})
    rec["domains"] = sorted({ents[s]["outlet"] for s in ents if ents[s]["outlet"]})
    rec["n_outlets"] = len(rec["domains"])
    rec["n_anchors"] = len(anchors)
    chk = Counter()
    chk_dom = defaultdict(Counter)
    for c in ((res.get("checks") or {}).get("citations")) or []:
        st = c.get("judgement") or c.get("state") or "unknown"
        chk[st] += 1
        sid = anchor_ent.get(c.get("anchor_id"))
        if sid:
            chk_dom[ents[sid]["domain"]][st] += 1
    rec["checks"] = dict(chk)
    rec["checks_by_domain"] = {d: dict(v) for d, v in chk_dom.items()}
    v = res.get("version") or {}
    rec["published_at"] = v.get("published_at", "")
    return rec


# ---------------------------------------------------------------- aggregation

def med(xs):
    return round(statistics.median(xs), 2) if xs else None


def pct(a, b):
    return round(100.0 * a / b, 1) if b else None


def window_summary(stories, name, start, end, baseline_keys, with_dups=True):
    S = [s for s in stories if s["ts"] and start <= s["ts"] <= end]
    F = [s for s in S if s["fetched"]]
    out = {"window": name, "start": start.isoformat(), "end": end.isoformat(),
           "stories": len(S), "bodies_read": len(F)}

    words = [s["words"] for s in S]
    out["words"] = {"median": med(words), "p25": med(sorted(words)[:len(words) // 2]) if words else None,
                    "under_300": sum(w < 300 for w in words), "under_150": sum(w < 150 for w in words),
                    "under_50": sum(w < 50 for w in words)}
    out["language"] = dict(Counter(s.get("lang", s["title_lang"]) for s in F))
    out["language_title_only"] = dict(Counter(s["title_lang"] for s in S))
    out["no_section"] = sum(1 for s in S if not s["section"])
    out["roundup_titles"] = sum(1 for s in F if s["roundup"])
    out["placeholders"] = sum(1 for s in F if s["placeholder"])
    out["casting_brief_titles"] = sum(1 for s in S if _BRIEF.match(s["title"]))
    out["stories_with_adjacent_dup_cites"] = sum(1 for s in F if s["adjacent_dup_cites"])
    out["revised_stories"] = sum(1 for s in S if s["version_count"] > 1)
    out["latest_revision_median"] = med([s["revision"] for s in S])
    out["latest_revision_mean"] = round(statistics.mean([s["revision"] for s in S]), 2) if S else None

    # per story distribution
    ns = [s["n_sources"] for s in F]
    no = [s["n_outlets"] for s in F]
    out["sources_per_story"] = {"median": med(ns), "dist": dict(sorted(Counter(min(n, 8) for n in ns).items()))}
    out["outlets_per_story"] = {"median": med(no), "dist": dict(sorted(Counter(min(n, 8) for n in no).items()))}
    out["single_outlet_stories"] = sum(1 for n in no if n == 1)
    out["single_source_stories"] = sum(1 for n in ns if n == 1)
    out["zero_source_stories"] = sum(1 for n in ns if n == 0)
    out["two_outlet_stories"] = sum(1 for n in no if n == 2)
    out["median_words_by_outlets"] = {str(k): med([s["words"] for s in F if min(s["n_outlets"], 6) == k])
                                      for k in range(0, 7) if any(min(s["n_outlets"], 6) == k for s in F)}

    # outlet-count integrity: the gate (internal/outlet) already drops aggregators,
    # Bluesky posts and unwraps jina/archive proxies; n_outlets mirrors it. What
    # is left to check is twins and translation aggregators.
    def has(s, group):
        return any(d in group for d in s["all_domains"])
    out["outlet_integrity"] = {
        "stories_citing_a_non_outlet_domain_the_gate_ignores": sum(1 for s in F if any(d in NOT_OUTLETS or d == "bsky.app" for d in s["all_domains"])),
        "stories_citing_a_social_domain": sum(1 for s in F if has(s, SOCIAL)),
        "stories_citing_europesays": sum(1 for s in F if has(s, {"europesays.com"})),
        "stories_below_two_after_merging_twins": sum(1 for s in F if len({TWIN.get(d, d) for d in s["domains"]}) < 2),
        "stories_below_two_after_merging_twins_and_dropping_europesays": sum(
            1 for s in F if len({TWIN.get(d, d) for d in s["domains"] if d != "europesays.com"}) < 2),
        "stories_where_one_outlet_has_60pct_of_cites": sum(
            1 for s in F if s["domains"] and sum(x["cites"] for x in s["sources"]) and max(
                sum(x["cites"] for x in s["sources"] if x["outlet"] == d) for d in s["domains"]) >= 0.6 * sum(x["cites"] for x in s["sources"])),
    }

    nonen = [s for s in F if s.get("lang", "en") not in ("en", "und")]
    out["non_english_story_cited_domains"] = dict(Counter(d for s in nonen for d in s["domains"]).most_common(10))
    out["non_english_story_ids"] = [s["id"] for s in nonen]
    out["under_150_by_outlets"] = dict(sorted(Counter(min(s["n_outlets"], 6) for s in F if s["words"] < 150).items()))
    out["no_section_by_day"] = dict(sorted(Counter(s["ts"].date().isoformat() for s in S if not s["section"]).items()))
    out["stories_per_day"] = dict(sorted(Counter(s["ts"].date().isoformat() for s in S).items()))

    # per source domain
    dom_stories = defaultdict(set)
    dom_cites = Counter()
    dom_sole = Counter()
    dom_ages = defaultdict(list)
    dom_nonen_titles = Counter()
    dom_lang_known = Counter()
    dom_lang_mismatch = Counter()
    dom_failed = Counter()
    dom_checked = Counter()
    for s in F:
        sole = s["n_outlets"] == 1
        for src in s["sources"]:
            d = src["domain"]
            if not d:
                continue
            dom_stories[d].add(s["id"])
            dom_cites[d] += max(src["cites"], 1)
            if sole:
                dom_sole[d] += 1 if d in s["domains"] else 0
            if src["url_date"] and s["ts"]:
                dom_ages[d].append((s["ts"].date() - src["url_date"]).days)
            if src["title_lang"] != "und":
                dom_lang_known[d] += 1
                if src["title_lang"] != "en":
                    dom_nonen_titles[d] += 1
                if s.get("lang", "und") != "und" and src["title_lang"] != s["lang"]:
                    dom_lang_mismatch[d] += 1
        for d, v in s["checks_by_domain"].items():
            dom_checked[d] += sum(v.values())
            dom_failed[d] += v.get("failed", 0)
    # sole-source count once per story (the loop above counts each of its entities)
    sole_once = Counter()
    for s in F:
        if s["n_outlets"] == 1 and s["domains"]:
            sole_once[s["domains"][0]] += 1
    total_cites = sum(dom_cites.values()) or 1
    rows = []
    for d in sorted(dom_stories, key=lambda x: (-dom_cites[x], x)):
        ages = [a for a in dom_ages[d] if -1 <= a <= 3650]
        rows.append({
            "domain": d, "stories": len(dom_stories[d]), "cites": dom_cites[d],
            "share_of_cites_pct": pct(dom_cites[d], total_cites),
            "sole_source_stories": sole_once[d],
            "sole_source_share_pct": pct(sole_once[d], len(dom_stories[d])),
            "url_dated_items": len(ages), "median_url_age_days": med(ages),
            "cited_titles_non_english_pct": pct(dom_nonen_titles[d], dom_lang_known[d]),
            "cited_title_lang_known": dom_lang_known[d],
            "lang_mismatch_with_story": dom_lang_mismatch[d],
            "citation_checks": dom_checked[d], "citation_checks_failed": dom_failed[d],
        })
    out["total_cites"] = total_cites if dom_cites else 0
    out["distinct_domains"] = len(rows)
    out["domains"] = rows
    top = [r["share_of_cites_pct"] for r in rows[:10]]
    out["top10_domain_share_pct"] = round(sum(top), 1) if top else None
    allages = [a for d in dom_ages for a in dom_ages[d] if -1 <= a <= 3650]
    out["url_dated_cites_pct"] = pct(len(allages), sum(len(s["sources"]) for s in F))
    out["median_cited_item_age_days_url_dated"] = med(allages)

    # citation checks (Jev judgement exposed on the public page)
    tot = Counter()
    for s in F:
        tot.update(s["checks"])
    out["citation_check_totals"] = dict(tot)
    out["stories_with_a_failed_citation"] = sum(1 for s in F if s["checks"].get("failed"))

    # coverage
    per = defaultdict(Counter)
    for s in S:
        day = s["ts"].date().isoformat()
        per[day][s["section"] or "(none)"] += 1
    secs = Counter(s["section"] or "(none)" for s in S)
    out["sections"] = dict(secs.most_common())
    out["stories_per_section_per_day"] = {d: dict(per[d]) for d in sorted(per)}
    named = [v for k, v in secs.items() if k != "(none)"]
    out["section_skew"] = {
        "top_section": secs.most_common(1)[0][0] if secs else None,
        "top_section_share_pct": pct(secs.most_common(1)[0][1], len(S)) if secs else None,
        "named_section_max_over_min": round(max(named) / min(named), 1) if len(named) > 1 else None,
    }

    # near duplicates (same method as the earlier audit: title Jaccard >= 0.6, 48 h)
    if with_dups:
        groups, n = dup_groups([{"id": s["id"], "title": s["title"], "ts": s["ts"]} for s in S])
        out["dup_groups"] = len(groups)
        out["dup_stories"] = n
        out["dup_group_examples"] = [g for g in sorted(groups, key=len, reverse=True)[:5]]
    # new-only headline numbers
    return out


def headline(stories, baseline_keys):
    N = [s for s in stories if s["key"] not in baseline_keys and s["ts"]]
    F = [s for s in N if s["fetched"]]
    n = len(N)
    groups, ds = dup_groups([{"id": s["id"], "title": s["title"], "ts": s["ts"]} for s in N]) if N else ([], 0)
    h = {
        "new_stories": n, "bodies_read": len(F),
        "share_under_300": round(sum(s["words"] < 300 for s in N) / n, 4) if n else None,
        "share_under_150": round(sum(s["words"] < 150 for s in N) / n, 4) if n else None,
        "dup_groups": len(groups), "dup_stories": ds,
        "non_english": sum(1 for s in F if s.get("lang", "en") not in ("en", "und")),
        "non_english_by_lang": dict(Counter(s["lang"] for s in F if s.get("lang", "en") not in ("en", "und"))),
        "no_section_share": round(sum(1 for s in N if not s["section"]) / n, 4) if n else None,
        "placeholders": sum(1 for s in F if s["placeholder"]),
        "casting_brief_titles": sum(1 for s in N if _BRIEF.match(s["title"])),
        "adjacent_dup_cite_share": round(sum(1 for s in F if s["adjacent_dup_cites"]) / len(F), 4) if F else None,
    }
    return h


def load_baseline(path):
    keys = set()
    rows = []
    if not os.path.exists(path):
        return keys, rows
    for r in csv.DictReader(open(path, encoding="utf-8")):
        m = re.search(r"pub([0-9a-f]{9})\b", r["url"].split("/pub/texture/")[-1].split("/v")[0])
        if m:
            keys.add(m.group(1))
        rows.append(r)
    return keys, rows


def calibrate_dups(rows):
    items = []
    for r in rows:
        try:
            ts = dt.datetime.fromisoformat(r["date"]).replace(tzinfo=dt.timezone.utc)
        except ValueError:
            continue
        items.append({"id": r["url"], "title": r["title"], "ts": ts})
    g, n = dup_groups(items)
    return {"dup_groups": len(g), "dup_stories": n, "earlier_reported": [EARLIER["dup_groups"], EARLIER["dup_stories"]],
            "note": "date-only timestamps; the earlier audit's exact tokenisation is not in the repo"}


def calibrate_lang(rows):
    c = Counter(detect_lang(r["title"] + " " + "") for r in rows)
    return {"title_only_non_english": sum(v for k, v in c.items() if k not in ("en", "und")),
            "earlier_reported": EARLIER["non_english"]}


# ---------------------------------------------------------------- diff

def flat(d, prefix=""):
    out = {}
    for k, v in d.items():
        if k in ("domains", "stories_per_section_per_day", "non_english_story_ids"):
            continue
        if isinstance(v, dict):
            out.update(flat(v, prefix + str(k) + "."))
        elif isinstance(v, (int, float)) and not isinstance(v, bool):
            out[prefix + str(k)] = v
    return out


def diff_summaries(prev, cur):
    a, b = flat(prev), flat(cur)
    lines = ["| metric | previous | now | change |", "|---|---:|---:|---:|"]
    for k in sorted(set(a) | set(b)):
        if k.startswith("meta."):
            continue
        x, y = a.get(k), b.get(k)
        if x == y:
            continue
        ch = round(y - x, 4) if x is not None and y is not None else ""
        lines.append("| %s | %s | %s | %s |" % (k, x, y, ch))
    # domains: movers by share of cites, 7d window
    pd_ = {r["domain"]: r for r in (prev.get("window_7d") or {}).get("domains", [])}
    cd_ = {r["domain"]: r for r in (cur.get("window_7d") or {}).get("domains", [])}
    mv = sorted(set(pd_) | set(cd_), key=lambda d: -abs((cd_.get(d) or {}).get("share_of_cites_pct", 0) - (pd_.get(d) or {}).get("share_of_cites_pct", 0)))[:15]
    lines += ["", "Domain share-of-cites movers (7d window):", "", "| domain | previous % | now % |", "|---|---:|---:|"]
    for d in mv:
        lines.append("| %s | %s | %s |" % (d, (pd_.get(d) or {}).get("share_of_cites_pct", "-"), (cd_.get(d) or {}).get("share_of_cites_pct", "-")))
    return "\n".join(lines) + "\n"


# ---------------------------------------------------------------- main

def collect(fetcher, since, now, max_stories=0, first_publish=False):
    run_id = now.strftime("%Y%m%dT%H%M%S")
    items = []
    cursor = None
    while True:
        d = fetcher.front_page(cursor, run_id)
        if "_http" in d:
            print("front list unavailable: HTTP %s" % d["_http"], file=sys.stderr)
            break
        page = d.get("stories") or []
        items += page
        cursor = d.get("next_cursor")
        last = parse_ts(page[-1]["updated_at"]) if page else None
        print("  list page: %d stories, oldest %s" % (len(page), page[-1]["updated_at"][:16] if page else "-"), file=sys.stderr)
        if not cursor or not page or (last and last < since):
            break
    items = [i for i in items if (parse_ts(i["updated_at"]) or now) >= since]
    if max_stories:
        items = items[:max_stories]
    stories = []
    for n, it in enumerate(items, 1):
        route = (it.get("route") or {}).get("permalink")
        res = fetcher.story(route) if route else None
        rec = parse_story(it, res)
        if first_publish and rec["fetched"] and rec["revision"] > 1 and route:
            v1 = re.sub(r"/v\d+$", "/v1", route)
            r1 = fetcher.story(v1)
            rec["first_published_at"] = ((r1 or {}).get("version") or {}).get("published_at", "") if "_http" not in (r1 or {}) else ""
        else:
            rec["first_published_at"] = rec.get("published_at") if rec["revision"] <= 1 else ""
        stories.append(rec)
        if n % 100 == 0:
            print("  stories %d/%d (net %d, cache %d)" % (n, len(items), fetcher.n_net, fetcher.n_cache), file=sys.stderr)
    return stories


def write_outputs(out, stories, summary):
    os.makedirs(out, exist_ok=True)
    with open(os.path.join(out, "summary.json"), "w") as f:
        json.dump(summary, f, indent=1, sort_keys=True, default=str)
    with open(os.path.join(out, "stories.csv"), "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["id", "route", "title_12w", "updated_at", "first_published_at", "revision", "section", "words",
                    "sources", "outlets", "lang", "roundup_title", "adjacent_dup_cites", "failed_citation_checks",
                    "single_outlet_domain"])
        for s in sorted(stories, key=lambda x: x["updated_at"], reverse=True):
            w.writerow([s["id"], s["route"], short_title(s["title"]), s["updated_at"][:19], (s.get("first_published_at") or "")[:19],
                        s["revision"], s["section"], s["words"], s.get("n_sources", ""), s.get("n_outlets", ""),
                        s.get("lang", s["title_lang"]), int(s.get("roundup", False)), s.get("adjacent_dup_cites", ""),
                        (s.get("checks") or {}).get("failed", ""),
                        s["domains"][0] if s.get("n_outlets") == 1 else ""])
    with open(os.path.join(out, "domains_7d.csv"), "w", newline="") as f:
        rows = summary["window_7d"]["domains"]
        if rows:
            w = csv.DictWriter(f, fieldnames=list(rows[0].keys()))
            w.writeheader()
            w.writerows(rows)


def render_tables(summary):
    L = []
    for key in ("window_7d", "window_24h"):
        w = summary[key]
        L += ["### Window %s: %s to %s" % (w["window"], w["start"][:16], w["end"][:16]), "",
              "stories %d, bodies read %d; words median %s, under 300: %d, under 150: %d, under 50: %d; no section: %d; revised: %d" % (
                  w["stories"], w["bodies_read"], w["words"]["median"], w["words"]["under_300"], w["words"]["under_150"],
                  w["words"]["under_50"], w["no_section"], w["revised_stories"]),
              "", "sources per story %s; outlets per story %s; single-outlet stories %d; two-outlet stories %d" % (
                  w["sources_per_story"]["dist"], w["outlets_per_story"]["dist"], w["single_outlet_stories"], w["two_outlet_stories"]),
              "", "language (body, stopword vote): %s; roundup titles %d; placeholders %d; casting-brief titles %d; "
              "stories with adjacent duplicate cite markers %d; dup groups %s (%s stories)" % (
                  w["language"], w["roundup_titles"], w["placeholders"], w["casting_brief_titles"],
                  w["stories_with_adjacent_dup_cites"], w.get("dup_groups"), w.get("dup_stories")),
              "", "citation checks shown on the page: %s; stories with at least one failed check: %d" % (
                  w["citation_check_totals"], w["stories_with_a_failed_citation"]),
              "", "words by outlet count: %s" % w["median_words_by_outlets"],
              "", "outlet integrity: %s" % w["outlet_integrity"],
              "", "revisions: median latest_revision %s, mean %s; stories with more than one published version: %d" % (
                  w["latest_revision_median"], w["latest_revision_mean"], w["revised_stories"]),
              "", "Sections: %s; skew %s" % (w["sections"], w["section_skew"]), "",
              "Top domains (of %d, %d cites; top 10 hold %s%%):" % (w["distinct_domains"], w["total_cites"], w["top10_domain_share_pct"]), "",
              "| domain | stories | cites | share % | sole-source stories | sole % | url-dated | median url age d | cited titles non-en % | lang mismatch | checks failed/total |",
              "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|"]
        for r in w["domains"][:25]:
            L.append("| %s | %d | %d | %s | %d | %s | %d | %s | %s | %d | %d/%d |" % (
                r["domain"], r["stories"], r["cites"], r["share_of_cites_pct"], r["sole_source_stories"], r["sole_source_share_pct"],
                r["url_dated_items"], r["median_url_age_days"], r["cited_titles_non_english_pct"], r["lang_mismatch_with_story"],
                r["citation_checks_failed"], r["citation_checks"]))
        L += ["", "Stories per section per day:", "", "| day | sections |", "|---|---|"]
        for d, v in w["stories_per_section_per_day"].items():
            L.append("| %s | %s |" % (d, v))
        L.append("")
    n, e = summary["new_since_earlier_audit"], summary["earlier_audit"]
    L += ["### New stories only against the earlier audit", "", "| metric | earlier | new |", "|---|---:|---:|"]
    for k, nk in (("share_under_300", "share_under_300"), ("share_under_150", "share_under_150"), ("dup_groups", "dup_groups"),
                  ("non_english", "non_english"), ("no_section_share", "no_section_share"), ("placeholders", "placeholders"),
                  ("casting_brief_titles", "casting_brief_titles"), ("adjacent_dup_cite_share", "adjacent_dup_cite_share")):
        L.append("| %s | %s | %s |" % (k, round(e[k], 4) if isinstance(e[k], float) else e[k], n[nk]))
    L.append("")
    return "\n".join(L)


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--since", default="7d", help="window start: 7d, 36h, or an ISO date/time (UTC); the 24h window is always the last 24h")
    ap.add_argument("--out", required=True, help="output directory (summary.json, stories.csv, domains_7d.csv)")
    ap.add_argument("--diff", help="previous summary.json; writes diff.md next to the new summary")
    ap.add_argument("--cache", default=DEFAULT_CACHE, help="response cache (outside the repo; holds raw bodies)")
    ap.add_argument("--baseline", default=DEFAULT_BASELINE, help="the earlier audit's all_articles.csv")
    ap.add_argument("--now", help="ISO time to treat as now (default: the clock)")
    ap.add_argument("--max-stories", type=int, default=0, help="debug: stop after N stories")
    ap.add_argument("--first-publish", action="store_true", help="also try the v1 route of revised stories (older versions are mostly pruned: 404, 2107 of 2108 on 2026-11-10)")
    ap.add_argument("--offline", action="store_true", help="read the cache only")
    a = ap.parse_args(argv)

    now = parse_ts(a.now) if a.now else dt.datetime.now(dt.timezone.utc)
    since = parse_duration_or_time(a.since, now)
    f = Fetcher(a.cache, offline=a.offline)
    print("window %s -> %s" % (since.isoformat(), now.isoformat()), file=sys.stderr)
    stories = collect(f, since, now, a.max_stories, a.first_publish)
    bkeys, brows = load_baseline(a.baseline)
    w7 = window_summary(stories, "7d" if a.since == "7d" else a.since, since, now, bkeys)
    w24 = window_summary(stories, "24h", now - dt.timedelta(hours=24), now, bkeys)
    new = headline(stories, bkeys)
    summary = {
        "meta": {"run_at": now.isoformat(), "since": since.isoformat(), "paper": BASE,
                 "requests_network": f.n_net, "requests_cached": f.n_cache, "http_errors": dict((str(k), v) for k, v in f.errors.items()),
                 "stories_listed_in_window": len(stories), "bodies_unreadable": sum(1 for s in stories if not s["fetched"]),
                 "baseline_stories": len(bkeys),
                 "window_basis": "updated_at (time of the latest published version); first_published_at is in stories.csv"},
        "window_7d": w7, "window_24h": w24,
        "new_since_earlier_audit": new, "earlier_audit": EARLIER,
        "calibration_on_earlier_corpus": {"dups": calibrate_dups(brows) if brows else None,
                                          "lang": calibrate_lang(brows) if brows else None},
    }
    write_outputs(a.out, stories, summary)
    open(os.path.join(a.out, "tables.md"), "w").write(render_tables(json.loads(json.dumps(summary, default=str))))
    if a.diff:
        prev = json.load(open(a.diff))
        open(os.path.join(a.out, "diff.md"), "w").write(diff_summaries(prev, summary))
    print("done: %d stories, %d network requests, %d cached" % (len(stories), f.n_net, f.n_cache), file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
