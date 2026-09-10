#!/usr/bin/env python3
"""Standing-gate consumer for the mission-2 V1 inventory (query_version 1).

Reads pinned-query TSV hits on stdin, checks them against the frozen artifact:
  1. every frozen anchor text still occurs in its file (stale-anchor check);
  2. every live sweep hit is covered by a frozen anchor (set-difference check);
  3. unknown-verification rows: non-allowlisted fail always; allowlisted fail
     only in freeze mode (CI mode warns).

Usage: v1-inventory-queries.py <manifest> | v1-inventory-gate.py <frozen> <ci|freeze>
Prints WARN:/FAIL:/ROWS: lines; exits 0 unless FAIL lines were emitted.
"""
import json
import sys

frozen_path, mode = sys.argv[1], sys.argv[2]
doc = json.load(open(frozen_path))
rows = doc["rows"]
fails, warns = [], []

anchor_index = {}  # (file, text) -> [keys]
file_cache = {}


def lines_of(path):
    if path not in file_cache:
        try:
            with open(path, encoding="utf-8", errors="replace") as fh:
                file_cache[path] = fh.read().splitlines()
        except FileNotFoundError:
            file_cache[path] = None
    return file_cache[path]


for r in rows:
    src = lines_of(r["file"])
    if src is None:
        fails.append(f"missing file: {r['file']} (row {r['key']})")
        continue
    for t in r["texts"]:
        anchor_index.setdefault((r["file"], t), []).append(r["key"])
        if not any(t in line for line in src):
            # fall back to stripped containment (freeze stored stripped text)
            if not any(t.strip() in line.strip() for line in src):
                fails.append(f"stale anchor: {r['file']}: {t[:100]} (row {r['key']})")

for line in sys.stdin:
    parts = line.rstrip("\n").split("\t")
    if len(parts) != 4:
        fails.append(f"malformed hit: {line[:120]}")
        continue
    f, _n, _q, t = parts
    if f == "MISSING":
        fails.append(f"manifest file missing from tree: {t}")
        continue
    if (f, t) not in anchor_index:
        fails.append(f"unanchored hit: {f}: {t[:120]} (classify in a Define update)")

for r in rows:
    if r["verification"] != "unknown":
        continue
    if "allowlist" in r:
        msg = f"allowlisted unknown (freeze blocker): {r['key']} [{r['allowlist']}]"
        if mode == "freeze":
            fails.append(msg)
        else:
            warns.append(msg)
    else:
        fails.append(f"unknown row without allowlist: {r['key']}")

for w in warns:
    print(f"WARN:{w}")
for fl in fails:
    print(f"FAIL:{fl}")
print(f"ROWS:{len(rows)}")
sys.exit(1 if fails else 0)
