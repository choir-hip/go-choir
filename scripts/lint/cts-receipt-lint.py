#!/usr/bin/env python3
"""Mechanical receipt/ledger linter for the texture-supervision definition.

Catches the defect classes that seven consecutive panel gates (v4.4-2..v4.7-2)
returned on the same witness: receipt field completeness, boundary enum, hex
widths, evidence count regressions, now-card placement, and corrupt prose.
This is the substrate fix so receipts are mechanically acceptable before a panel.
"""
import re, sys, pathlib

ROOT = pathlib.Path(__file__).resolve().parents[2]
DEF = ROOT / "docs/definitions/choir-continuous-texture-supervision-2026-08-07.md"
EVID = ROOT / "docs/evidence/continuous-texture-supervision-definition-consensus-2026-08-07.md"
BOUNDARY = {"define", "implement", "terminal"}
REQUIRED = ["boundary", "commit_or_artifact", "proof_refs", "rollback_ref",
            "disposition", "problem_ref", "authorization_ref",
            "candidate_or_evidence_refs", "landing", "registry_conformance_ref"]

errors = []
def err(msg): errors.append(msg)

d = DEF.read_text()
e = EVID.read_text()
receipts = re.split(r"(?m)^  - id: ", d)[1:]
receipts = [("id:" + b.split("\n", 1)[0].strip(), b) for b in receipts]
for rid, b in receipts:
    for f in REQUIRED:
        if not re.search(rf"(?m)^    {re.escape(f)}:", b):
            err(f"{rid}: missing required field {f}")
    m = re.search(r"(?m)^    boundary: (\w+)", b)
    if m and m.group(1) not in BOUNDARY:
        err(f"{rid}: boundary {m.group(1)!r} not in {sorted(BOUNDARY)}")
    rb = re.search(r"(?m)^    rollback_ref: main@([0-9a-f]+)", b)
    if rb and len(rb.group(1)) != 40:
        err(f"{rid}: rollback_ref hex {len(rb.group(1))} != 40")

for name, txt in (("definition", d), ("evidence", e)):
    for pat, warn in [(r"main@[0-9a-f]{41}", "41-hex rollback"), (r"[0-9a-f]{65}", "65-hex")]:
        hits = re.findall(pat, txt)
        if hits: err(f"{name}: {warn}: {hits[:2]}")

for phrase in ("Gap Codex", "byte/blood", "bloomberg", "andnd", "green window"):
    if phrase in d: err(f"definition has {phrase!r}")
    if phrase in e: err(f"evidence has {phrase!r}")

for i, l in enumerate(d.split("\n"), 1):
    if l.startswith("next_action:"):
        err(f"definition line {i}: next_action at top level, must be under now:")
for i, l in enumerate(e.split("\n"), 1):
    if "seven ok" in l and not any(w in l for w in ("said", "reported", "corrected", "undercount", "under-count", "REPAIR", "Verdict")):
        err(f"evidence line {i}: unquoted 'seven ok': {l.strip()[:90]}")

print(f"receipts: {len(receipts)}")
print(f"errors: {len(errors)}")
for x in errors: print("  -", x)
sys.exit(1 if errors else 0)
