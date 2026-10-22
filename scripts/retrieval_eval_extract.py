#!/usr/bin/env python3
"""Build the query file of retrieval-eval-v1 from read-only research traces.

Definition: docs/evidence/retrieval-eval-v1.md. Input is the read-only
extract of the paper's research runs (default
~/.config/choir-qa/traces/search/paper-research-events.json, outside the
repo). Output is JSON lines of queries and judged ids only: no article text,
no search result text.

Two classes come from the traces:

  R1c  each choir.SearchWireCorpus call with a literal query; relevant:
       the capture ids the same run read with choir.ReadCapture, and the
       URLs it read with choir.Read, after the call.
  R1w  each choir.SearchStart call with a literal query; relevant: the
       URLs the run read with choir.Read after it and before its next
       SearchStart cell.

These are silver labels ("the run went on to read it"). The definition
says how the blind judge replaces them; the file records which applies.

usage: retrieval_eval_extract.py [--events FILE] [--out FILE] [--web-sample 400]
"""
import argparse
import hashlib
import json
import os
import re
import sys

STR = r'"((?:[^"\\]|\\.)*)"|`([^`]*)`'
CALL = re.compile(r'choir\.(SearchWireCorpus|SearchStart|ReadCapture|Read)\(\s*(?:' + STR + ')')
CAPTURE_ID = re.compile(r'obj:choir\.web_capture:[A-Za-z0-9_=-]+:[A-Za-z0-9_.~-]+')
ASSIGN = re.compile(r'(\w+)\s*:?=\s*"(obj:choir\.web_capture:[^"]+|https?://[^"]+)"')


def cell_sources(evs):
    """The source of each cell of one run, in event order.

    tool.invoked is a bus-only line since journal cut R3: a new tape's
    tool.result carries the call's arguments. An old tape has the invoked row
    (and a result without arguments); a tape that crosses the cutover has
    both, and a call is counted once."""
    ordered = sorted(evs, key=lambda e: e.get("seq", 0))
    invoked = {(e.get("payload") or {}).get("call_id") for e in ordered if e.get("kind") == "tool.invoked"}
    sources = []
    for e in ordered:
        p = e.get("payload") or {}
        if e.get("kind") == "tool.invoked" or (
            e.get("kind") == "tool.result" and isinstance(p.get("arguments"), dict) and p.get("call_id") not in invoked
        ):
            sources.append((p.get("arguments") or {}).get("source", "") or "")
    return sources


def literal(m):
    if m.group(2) is not None:
        try:
            return json.loads('"' + m.group(2) + '"')
        except json.JSONDecodeError:
            return m.group(2)
    return m.group(3)


def shape(q):
    tags = []
    if '"' in q:
        tags.append("quoted")
    if "site:" in q:
        tags.append("site")
    if re.search(r'(?<![\w])[\w]{2}(?![\w])', q):
        tags.append("short_token")
    if re.search(r'[^\x00-ɏ\s\W]', q):
        tags.append("non_latin")
    if re.search(r'https?://|www\.', q):
        tags.append("url")
    return tags or ["plain"]


def split_of(q):
    h = int(hashlib.sha256(q.strip().lower().encode()).hexdigest()[:8], 16)
    return "held_out" if h % 10 < 3 else "tuning"


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--events", default=os.path.expanduser("~/.config/choir-qa/traces/search/paper-research-events.json"))
    ap.add_argument("--out", default="-")
    ap.add_argument("--web-sample", type=int, default=400)
    args = ap.parse_args()
    data = json.load(open(args.events))
    out = []
    for run_id, evs in sorted(data["events"].items()):
        calls = []  # (cell index, name, literal arg)
        for ci, src in enumerate(cell_sources(evs)):
            names = {m.group(1): m.group(2) for m in ASSIGN.finditer(src)}
            for m in CALL.finditer(src):
                calls.append((ci, m.group(1), literal(m)))
            # ReadCapture(pressID, ...) with the id in a variable.
            for m in re.finditer(r'choir\.ReadCapture\(\s*(\w+)\s*,', src):
                if m.group(1) in names:
                    calls.append((ci, "ReadCapture", names[m.group(1)]))
            for m in re.finditer(r'choir\.Read\(\s*(\w+)\s*\)', src):
                if m.group(1) in names and names[m.group(1)].startswith("http"):
                    calls.append((ci, "Read", names[m.group(1)]))
        for i, (ci, name, arg) in enumerate(calls):
            if name not in ("SearchWireCorpus", "SearchStart") or not arg or not arg.strip():
                continue
            caps, urls = [], []
            for cj, n2, a2 in calls[i + 1:]:
                if name == "SearchStart" and n2 == "SearchStart" and cj > ci:
                    break
                if n2 == "ReadCapture" and CAPTURE_ID.fullmatch(a2 or ""):
                    caps.append(a2)
                elif n2 == "Read" and (a2 or "").startswith("http"):
                    urls.append(a2)
            out.append({
                "class": "R1c" if name == "SearchWireCorpus" else "R1w",
                "query": arg.strip(),
                "run_id": run_id,
                "shape": shape(arg),
                "relevant": {"capture_ids": sorted(set(caps)), "urls": sorted(set(urls))},
                "judgment": "silver:read-after-query",
            })
    # One row per (class, normalized query); relevant ids are unioned.
    merged = {}
    for row in out:
        key = (row["class"], " ".join(row["query"].lower().split()))
        if key not in merged:
            merged[key] = row
            row["runs"] = [row.pop("run_id")]
            continue
        m = merged[key]
        m["runs"].append(row["run_id"])
        for k in ("capture_ids", "urls"):
            m["relevant"][k] = sorted(set(m["relevant"][k]) | set(row["relevant"][k]))
    rows = list(merged.values())
    corpus = [r for r in rows if r["class"] == "R1c"]
    web = [r for r in rows if r["class"] == "R1w" and (r["relevant"]["urls"] or r["relevant"]["capture_ids"])]
    # Stratified by shape: rarer shapes first, then by query hash, so the
    # sample is reproducible.
    order = {"url": 0, "non_latin": 1, "site": 2, "quoted": 3, "short_token": 4, "plain": 5}
    web.sort(key=lambda r: (min(order[s] for s in r["shape"]), hashlib.sha256(r["query"].encode()).hexdigest()))
    by_shape = {}
    for r in web:
        by_shape.setdefault(min(r["shape"], key=lambda s: order[s]), []).append(r)
    sample, k = [], 0
    while len(sample) < args.web_sample and any(by_shape.values()):
        for s in sorted(by_shape, key=lambda s: order[s]):
            if by_shape[s] and len(sample) < args.web_sample:
                sample.append(by_shape[s].pop(0))
        k += 1
    final = corpus + sample
    for i, r in enumerate(final):
        r["id"] = "rev1-%s-%04d" % (r["class"].lower(), i)
        r["split"] = split_of(r["query"])
        r["runs"] = sorted(set(r["runs"]))
    sink = sys.stdout if args.out == "-" else open(args.out, "w")
    for r in final:
        sink.write(json.dumps({k: r[k] for k in ("id", "class", "split", "query", "shape", "relevant", "judgment", "runs")}, ensure_ascii=False) + "\n")
    print("retrieval-eval-v1: %d R1c, %d R1w (of %d with a read), %d held out" % (
        len(corpus), len(sample), len(web), sum(r["split"] == "held_out" for r in final)), file=sys.stderr)


if __name__ == "__main__":
    main()
