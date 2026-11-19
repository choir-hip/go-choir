#!/usr/bin/env python3
"""Read-only export of the paper's PUBLIC text for the first embedding pass (station V3).

Run on Node B as root, over ssh, from stdin, nothing installed and nothing written
there (the output goes to stdout):

    ssh -C node-b python3 - stories --owner O --computer C [--latest-only | --older-only] < scripts/embed-pass-export.py | gzip > stories.jsonl.gz
    ssh -C node-b python3 - captures --owner O --computer C [--after <id>] [--since <RFC3339>] < scripts/embed-pass-export.py | gzip >> captures.jsonl.gz

What it reads: PostgreSQL `graph` as the postgres system user (peer authentication, every session
`default_transaction_read_only = on`), and the plain text blobs corpusd wrote under
/var/lib/go-choir/platform-artifacts. Nothing under /var/lib/go-choir/secrets, no token, no environment.

Privacy: a capture leaves only when it passes exactly the test corpusd's full-text index applies
(internal/platform/capture_fulltext.go captureFullTextPage), and a stricter one:
  - the object is a choir.web_capture of the paper's owner and computer;
  - not a tombstone, not superseded, a text blob id of 64 hex;
  - no label of the object's metadata starts with "owner_private" (the index tests the prefix
    "owner_private:" on valid labels; this tests the bare prefix on every string under "labels", and
    drops a capture whose "labels" is present but is not a list of strings, which the index would
    read as no labels).
A story version leaves only when its publication is state "published", its manifest's access policy
allows the public route (visibility public, route empty or public: publicationAccessPolicyAllowsPublicRoute),
and it belongs to the paper's owner.
The counts of every refusal go to stderr as one JSON line at the end.

Passages are cut with a port of internal/capturefts/passages.go (checked 2026-11-19 against the Go function: identical spans
on Alice, a long-paragraph text, a CJK text and a whitespace text). A capture row is its lead passage (ordinal 0); a story row is
every passage of the version's text.
"""
import csv, hashlib, json, os, subprocess, sys

# The script holds no identity: the owner and computer are required arguments (--owner, --computer).
PAPER_OWNER = PAPER_COMPUTER = ""
ROOT = "/var/lib/go-choir/platform-artifacts"
TARGET, MAXP = 1800, 2400
MAX_INDEXED_BYTES = 256 << 10

# ---- passages: a port of internal/capturefts/passages.go, on UTF-8 bytes ----

def _is_space(c):
    return c in (0x20, 0x0A, 0x09, 0x0D, 0x0B, 0x0C)

def _runes(b):
    return len(b.decode("utf-8", "replace"))

def paragraphs(t):
    out, i, n = [], 0, len(t)
    while i < n:
        while i < n and _is_space(t[i]):
            i += 1
        start = end = i
        while i < n:
            if t[i] == 0x0A:
                j = i + 1
                while j < n and t[j] in (0x20, 0x09, 0x0D):
                    j += 1
                if j < n and t[j] == 0x0A:
                    break
            if not _is_space(t[i]):
                end = i + 1
            i += 1
        while end < n and (t[end] & 0xC0) == 0x80:
            end += 1
        if end > start:
            out.append((start, end))
    return out

_SENT = set(".!?。！？؟।")

def _decode_last(t, lo, hi):
    """The last rune of t[lo:hi] and its byte width."""
    k = hi - 1
    while k > lo and (t[k] & 0xC0) == 0x80:
        k -= 1
    return t[k:hi].decode("utf-8", "replace"), hi - k

def split_long(t, p):
    out, pos = [], p[0]
    while pos < p[1]:
        end, n = pos, 0
        while n < TARGET and end < p[1]:
            c = t[end]
            end += 1 if c < 0x80 else 2 if c < 0xE0 else 3 if c < 0xF0 else 4
            n += 1
        end = min(end, p[1])
        if end < p[1]:
            limit, cut, back, n = end, -1, end, 0
            while back > pos and n < TARGET // 4:
                r, w = _decode_last(t, pos, back)
                if r in _SENT:
                    cut = back
                    break
                if r.isspace() and cut < 0:
                    cut = back
                back -= w
                n += 1
            if cut > pos and cut < limit + 1:
                end = cut
        s, e = pos, end
        while s < e and _is_space(t[s]):
            s += 1
        while e > s and _is_space(t[e - 1]):
            e -= 1
        if e > s:
            out.append((s, e))
        pos = end
    return out

def passages(t):
    out, cs, ce, cr = [], -1, 0, 0
    for p in paragraphs(t):
        n = _runes(t[p[0]:p[1]])
        if n > MAXP:
            if cs >= 0:
                out.append((cs, ce))
            cs, ce, cr = -1, 0, 0
            out.extend(split_long(t, p))
            continue
        if cs >= 0 and cr + n > TARGET:
            out.append((cs, ce))
            cs, ce, cr = -1, 0, 0
        if cs < 0:
            cs, cr = p[0], 0
        ce = p[1]
        cr += n
    if cs >= 0:
        out.append((cs, ce))
    return out

# ---- reading ----

def psql_rows(sql):
    """Stream the rows of a read-only COPY ... CSV from PostgreSQL as the postgres user."""
    script = "SET default_transaction_read_only = on;\nCOPY (%s) TO STDOUT WITH (FORMAT csv);\n" % sql
    p = subprocess.Popen(["su", "postgres", "-s", "/bin/sh", "-c", "psql graph -v ON_ERROR_STOP=1 -q -f -"],
                         stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True, encoding="utf-8", errors="replace")
    p.stdin.write(script); p.stdin.close()
    for row in csv.reader(p.stdout):
        yield row
    if p.wait() != 0:
        sys.exit("psql failed")

def fetched_since(fetched_at, since):
    """True when fetched_at (RFC3339, maybe nanoseconds) is at or after since. An unreadable time is kept, not dropped."""
    from datetime import datetime, timezone
    def p(x):
        x = x.strip().replace("Z", "+00:00")
        if "." in x:
            head, rest = x.split(".", 1)
            n = 0
            while n < len(rest) and rest[n].isdigit():
                n += 1
            x = head + "." + rest[:n][:6].ljust(6, "0") + rest[n:]
        d = datetime.fromisoformat(x)
        return d if d.tzinfo else d.replace(tzinfo=timezone.utc)
    try:
        return p(fetched_at) >= p(since)
    except ValueError:
        return True

def lit(s):
    return "'" + s.replace("'", "''") + "'"

def private_label(meta):
    """True when the capture must not leave: any trace of an owner_private label, or labels in a shape we cannot read."""
    if "labels" not in meta:
        return False
    v = meta["labels"]
    if not isinstance(v, list):
        return v is not None
    for x in v:
        if not isinstance(x, str) or x.strip().lower().startswith("owner_private"):
            return True
    return False

class OutsideRoot(Exception):
    pass

def blob(path_rel, limit=None):
    """Read a file under ROOT and nowhere else: every read (manifest, story text, capture text) goes through here."""
    if not path_rel or path_rel.startswith("/") or ".." in path_rel.split("/") or "\0" in path_rel:
        raise OutsideRoot(path_rel)
    root = os.path.realpath(ROOT)
    real = os.path.realpath(os.path.join(root, path_rel))
    if not real.startswith(root + "/"):
        raise OutsideRoot(path_rel)
    with open(real, "rb") as f:
        return f.read(limit) if limit else f.read()

def emit(o):
    sys.stdout.write(json.dumps(o, ensure_ascii=False, separators=(",", ":")) + "\n")

def row(kind, key_id, ordinal, title, text_bytes, extra):
    text = text_bytes.decode("utf-8", "replace")
    o = {"kind": kind, "id": key_id, "ordinal": ordinal, "title": title, "text": text,
         "text_sha": hashlib.sha256(text.encode("utf-8")).hexdigest()}
    o.update(extra)
    emit(o)

# ---- the two exports ----

def captures(after, since, limit, counts):
    sql = ("select canonical_id, tombstone, superseded_by, metadata from og_objects "
           "where object_kind = 'choir.web_capture' and owner_id = %s and computer_id = %s%s "
           "order by canonical_id%s") % (lit(PAPER_OWNER), lit(PAPER_COMPUTER),
                                         " and canonical_id > " + lit(after) if after else "",
                                         " limit %d" % limit if limit else "")
    for cid, tomb, sup, meta_raw in psql_rows(sql):
        counts["seen"] += 1
        if tomb == "t" or sup.strip():
            counts["refused_tombstone_or_superseded"] += 1; continue
        try:
            meta = json.loads(meta_raw)
        except ValueError:
            counts["skipped_bad_metadata"] += 1; continue
        if not isinstance(meta, dict):
            counts["skipped_bad_metadata"] += 1; continue
        if private_label(meta):
            counts["refused_private"] += 1; continue
        if since and not fetched_since(meta.get("fetched_at", ""), since):
            counts["skipped_before_since"] += 1; continue
        h = str(meta.get("extracted_text_blob_id", "")).strip()
        h = h[7:] if h.startswith("sha256:") else h
        if len(h) != 64 or any(c not in "0123456789abcdef" for c in h):
            counts["skipped_no_text_blob"] += 1; continue
        try:
            text = blob("capture-bodies/%s/%s" % (h[:2], h), MAX_INDEXED_BYTES + 8)
        except OutsideRoot:
            counts["refused_outside_root"] += 1; continue
        except OSError:
            counts["skipped_unreadable_blob"] += 1; continue
        spans = passages(text)
        if not spans:
            counts["skipped_no_text"] += 1; continue
        s, e = spans[0]
        row("capture", cid, 0, str(meta.get("title", "")).strip(), text[s:e],
            {"published_at": meta.get("published_at", ""), "fetched_at": meta.get("fetched_at", ""), "passages": len(spans)})
        counts["exported"] += 1

def stories(limit, counts, group="all"):
    where_rn = {"latest": "where rn = 1", "older": "where rn > 1"}.get(group, "")
    sql = ("select vid, pid, pmeta, vmeta, body_ref, body_inline from ("
           "select v.canonical_id vid, p.canonical_id pid, p.metadata pmeta, v.metadata vmeta, p.updated_at pupd, "
           "coalesce(m.body_ref, '') body_ref, "
           "case when coalesce(m.body_ref, '') = '' then convert_from(m.body, 'UTF8') else '' end body_inline, "
           "row_number() over (partition by p.canonical_id order by (v.metadata::json->>'revision')::int desc, v.canonical_id desc) rn "
           "from og_objects v "
           "join og_edges ev on ev.to_id = v.canonical_id and ev.kind = 'has_version' and not ev.tombstone "
           "join og_objects p on p.canonical_id = ev.from_id and p.object_kind = 'choir.publication' and not p.tombstone "
           "join og_edges em on em.from_id = v.canonical_id and em.kind = 'has_manifest' and not em.tombstone "
           "join og_objects m on m.canonical_id = em.to_id "
           "where v.object_kind = 'choir.publication_version' and not v.tombstone and v.owner_id = %s) x %s "
           "order by (rn > 1), pupd desc, pid, rn%s") % (lit(PAPER_OWNER), where_rn, " limit %d" % limit if limit else "")
    seen = set()
    for vid, pid, pmeta_raw, vmeta_raw, body_ref, body_inline in psql_rows(sql):
        if vid in seen:
            continue
        seen.add(vid)
        counts["seen"] += 1
        try:
            pmeta, vmeta = json.loads(pmeta_raw), json.loads(vmeta_raw)
        except ValueError:
            counts["skipped_bad_metadata"] += 1; continue
        if pmeta.get("state") != "published" or not str(pmeta.get("route_path", "")).strip():
            counts["refused_not_published"] += 1; continue
        try:
            man = json.loads(body_inline) if body_inline else json.loads(blob(body_ref))
        except OutsideRoot:
            counts["refused_outside_root"] += 1; continue
        except (ValueError, OSError):
            counts["skipped_no_manifest"] += 1; continue
        pol = man.get("access_policy") or {}
        if str(pol.get("visibility", "")).strip().lower() != "public" or str(pol.get("route", "")).strip().lower() not in ("", "public"):
            counts["refused_not_public"] += 1; continue
        ref = str(man.get("storage_ref", "")).strip()
        if not ref:
            counts["skipped_no_text_blob"] += 1; continue
        try:
            text = blob(ref)
        except OutsideRoot:
            counts["refused_outside_root"] += 1; continue
        except OSError:
            counts["skipped_unreadable_blob"] += 1; continue
        title = str(man.get("title") or pmeta.get("title") or "").strip()
        spans = passages(text)
        if not spans:
            counts["skipped_no_text"] += 1; continue
        for i, (s, e) in enumerate(spans):
            row("story_passage", vid, i, title, text[s:e],
                {"publication_id": pmeta.get("publication_id", pid), "revision": vmeta.get("revision", 0),
                 "published_at": vmeta.get("published_at", ""), "passages": len(spans)})
            counts["passages"] += 1
        counts["exported"] += 1

def main():
    global PAPER_OWNER, PAPER_COMPUTER
    mode = sys.argv[1] if len(sys.argv) > 1 else ""
    argv = sys.argv[2:]
    opt = lambda k, d="": argv[argv.index(k) + 1] if k in argv else d
    limit, after, since = int(opt("--limit", "0")), opt("--after"), opt("--since")
    PAPER_OWNER, PAPER_COMPUTER = opt("--owner"), opt("--computer")
    if mode not in ("stories", "captures") or not PAPER_OWNER or not PAPER_COMPUTER:
        sys.exit("usage: stories|captures --owner O --computer C [--limit N] [--after ID] [--since RFC3339] [--latest-only|--older-only]")
    if "--latest-only" in argv and "--older-only" in argv:
        sys.exit("--latest-only and --older-only are exclusive")
    counts = {"mode": mode, "seen": 0, "exported": 0, "refused_private": 0, "refused_tombstone_or_superseded": 0,
              "refused_not_published": 0, "refused_not_public": 0, "refused_outside_root": 0, "skipped_bad_metadata": 0,
              "skipped_no_text_blob": 0, "skipped_unreadable_blob": 0, "skipped_no_text": 0, "skipped_no_manifest": 0,
              "skipped_before_since": 0, "passages": 0}
    if mode == "stories":
        stories(limit, counts, "latest" if "--latest-only" in argv else "older" if "--older-only" in argv else "all")
    else:
        captures(after, since, limit, counts)
    sys.stderr.write(json.dumps(counts) + "\n")

if __name__ == "__main__":
    main()
