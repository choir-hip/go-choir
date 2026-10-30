#!/usr/bin/env python3
"""Retrospective precommitment ledger for throughline goal files.

Reads every ``docs/definitions/*.md`` goal file, extracts its acceptance
criteria, bridge/value conjecture, stated probabilities and receipts, and
settles each goal and each criterion *retrospectively from the file's own
status and receipts only*, by the fixed rules below. No model judgment, no
network, no repo state other than the goal files. Output is deterministic.

Usage (from anywhere; paths resolve against the repo root, which is the
parent of this script's directory unless ``--root`` is given)::

    python3 -I scripts/goal_precommitments.py              # write the ledger
    python3 -I scripts/goal_precommitments.py --check      # exit 1 if stale
    python3 -I scripts/goal_precommitments.py --json out.json

Outputs ``docs/calibration/goal-ledger.md`` (generated; never hand-edit).
Dependencies: Python 3 stdlib + PyYAML (``yaml.safe_load``).

Mutation class: green (docs + offline script; no runtime behavior).

RULES (version ``goal-ledger-rules/1``)
=======================================

Parsing
-------
P1  Frontmatter is the text between a leading ``---`` line and the next line
    that starts with ``---``. If no closing line exists, the whole remainder
    is read as one YAML document (P1b; some goal files are pure YAML after
    an opening ``---``). A file that does not start with ``---`` is
    ``no_frontmatter``; a YAML error is ``parse_error``; a non-mapping
    document is ``not_mapping``. None of these crash the run. For those files
    a regex reads ``status:`` (the first two-space-indented ``status:`` line
    inside the top-level ``now:`` block) and ``readiness:`` (first top-level line) as *hints* that
    are reported but never scored.

Field extraction (with every fallback for older corpus shapes)
--------------------------------------------------------------
E1  title: ``finish.deliver``; else ``finish.outcome`` (definition_version 3
    files); else ``definition_id``; else the file stem.
E2  readiness: top-level ``readiness``, normalized to its first word
    (``reviewed (convergent ...)`` -> ``reviewed``; ``draft`` -> ``drafted``).
    Missing (all definition_version 3 files predate the readiness ladder)
    -> ``none``.
E3  status: ``now.status``, normalized to its first word, lowercased, with
    trailing punctuation removed (``settled — spine complete`` ->
    ``settled``). Missing -> ``none``.
E4  acceptance criteria, in order of preference:
      a. ``finish.acceptance`` list. Item shapes: mapping with
         ``action``/``proves``/``evidence_class`` (standard); a plain string
         (action = the string); a single-key mapping such as
         ``{"P2d1 reducer test": "..."}`` (action = key, proves = value);
         any other mapping (action = its compact JSON).
      b. ``finish.metamission_stations`` (older metamission shape): one
         criterion per station, action = "station <id> lands",
         proves = the station's ``deliver``.
      c. ``metamission.stations`` when ``finish.acceptance`` is absent: as b.
      d. otherwise zero criteria.
E5  bridge conjecture claim: first non-empty of ``now.conjecture.claim``,
    ``now.conjecture.statement``, ``now.conjecture.bridge``, top-level
    ``conjecture.claim`` — skipping a claim that starts with "see " (a
    pointer to the top-level block). Status: ``now.conjecture.status``, else
    ``now.conjecture.verdict``, else ``conjecture.status``, first word.
E6  value: ``value.better_means`` and ``value.goodharting_would_be``.
E7  stated probabilities: (a) any mapping key in {p, p_yes, probability,
    prob, credence, bridge_p} anywhere in the frontmatter whose value is a
    number in [0, 1]; (b) a ``precommit`` block anywhere; (c) the raw
    frontmatter text matching ``\\bp\\s*=\\s*0?\\.\\d+`` or ``p(yes)``.
    Measurements such as "97% occupancy" are deliberately NOT counted:
    they are observations, not forecasts.
E8  receipts: top-level ``receipts`` list; items may be mappings or strings.
    A non-list ``receipts`` counts as zero receipts.

Receipt classes
---------------
A receipt is TERMINAL-CLASS when any of:
R1  mapping: ``boundary``, ``kind``, ``id`` or ``receipt_id`` contains
    "terminal" (e.g. ``boundary: terminal``, ``kind: station_terminal``);
R2  mapping: non-empty ``terminal_receipt``, ``landed_station`` or
    ``landed_receipts``;
R3  mapping: non-placeholder ``deployed_acceptance`` (top level or under
    ``landing``); placeholders are empty, none, not_applicable, n/a,
    pending, unknown;
R4  mapping: ``kind`` mentions "completion panel" and ``result`` or
    ``disposition`` starts with "approved"/"accept";
R5  string: the label before the first ":" contains "terminal", equals
    "deployed_acceptance", or starts with "settled" or "deployed landing";
R5b string: the label starts with "landed" and the text is
    deployed-evidence (see below) and positive;
R6  mapping: ``kind`` contains "deployed_acceptance"/"deployed acceptance"
    and the receipt is positive (see POS);
R7  mapping: ``kind: outcome`` with ``status: settled``;
R8  mapping: ``boundary: finish.acceptance`` with non-empty ``proof_refs``.
A receipt is DEPLOYED-EVIDENCE when R3, R6, a mapping ``kind`` containing
"deployed", a string label in {deployed_acceptance, deployed landing,
deployed identity}, or its text matches ``build.commit``, ``deployed``,
``redeploy``, ``staging`` or ``choir.news`` (deliberately broad: it only
gates rule C2, and a receipt in this corpus that names staging is a
deployed observation; the cost is that a receipt saying "not yet deployed"
would also pass).
POS: a receipt is positive unless its ``result``/``disposition``/``status``
(mapping) or its text (string) starts with partial, blocked, failed, fail,
sent back, refused, open, rejected or pending; a mapping whose status is
``open`` is not positive.

Goal outcome (from now.status + receipts only)
----------------------------------------------
G0  unparseable file, or status none, or a status word outside the
    vocabularies below -> ``unscorable``.
G1  status in {complete, landed, settled, closed} (success claims) and >= 1
    terminal-class receipt -> ``met``.
G2  success claim with no terminal-class receipt -> ``claimed`` (a sixth
    class: the file says done, but carries no receipt a rule can read).
G3  status in {blocked_incomplete, abandoned, failed} -> ``not_met``; bare
    ``blocked`` -> ``not_met`` unless readiness is intent/drafted, in which
    case ``open`` (the file never became executable, so it cannot have
    failed its acceptance).
G4  status ``superseded`` with no terminal-class receipt -> ``not_met``;
    with one -> ``partial``.
G5  status ``checkpoint_incomplete`` with >= 1 receipt -> ``partial``;
    with zero receipts -> ``open``.
G6  status in {working, in_progress, intent, drafted, deferred, pending,
    reviewed, executable} -> ``open``.

Criterion settlement
--------------------
C1  NAMED: a receipt names criterion i when its text matches
    ``acceptance (action|item|criterion|check) #?i`` or
    ``finish.acceptance[i-1]``, or contains the criterion's normalized
    action or proves text (only when that text is >= 24 characters).
    A named criterion with a positive receipt -> ``met`` (``met_named``),
    whatever the goal outcome. A negative naming receipt never settles a
    criterion (a later receipt may supersede it).
C2  goal ``met``: every not-yet-met criterion -> ``met``
    (``met_by_goal_receipt``), EXCEPT when ``finish.landing.required`` is
    true, the goal has no deployed-evidence receipt, and the criterion's
    ``evidence_class`` mentions deploy/staging/live/production/choir.news ->
    ``unresolved`` (``landing_receipt_missing``).
C3  goal ``not_met`` by G3 (blocked) -> remaining criteria ``not_met``
    (``not_met_by_goal``).
C4  goal ``not_met`` by G4 (superseded) -> remaining criteria
    ``unresolved`` (``superseded``): supersession is not refutation.
C5  every other case -> ``unresolved`` with the goal outcome as reason.

Base rates and the climatology baseline
---------------------------------------
No goal file records a probability, so there is no track record to score.
The script instead computes the base rate a planner could have used and the
Brier score of that constant forecast (the climatology baseline). Outcome
o = 1 for met, 0 otherwise, over three populations:
  resolved  met (1) vs not_met (0);
  strict    met (1) vs not_met, partial, claimed (0)  — unresolved-as-no;
  lenient   met or claimed (1) vs not_met (0).
Brier of constant p over outcomes o_i: mean((p - o_i)^2); with p equal to
the in-sample rate b this is b(1-b). Also reported: the leave-one-out
climatology (each goal forecast with the rate of the others — an honest
out-of-sample figure), the family-conditional leave-one-out climatology (p
= the rate of the goal's other family members, or the global
leave-one-out rate for a singleton family) and the uninformative p = 0.5 reference (0.25). Groupings:
readiness, family (the token after "choir-" with trailing digits removed,
so pd1..pd4 -> pd), month (YYYY-MM from the file-name date) and
definition_version. THIS IS A BASELINE, NOT A TRACK RECORD.

Determinism: files sorted by name; JSON written with sorted keys; floats
rounded to 4 places; the only date is ``--date`` (default 2026-10-14).
"""

from __future__ import annotations

import argparse
import glob
import json
import os
import re
import sys

import yaml

RULES_VERSION = "goal-ledger-rules/1"
DEFAULT_DATE = "2026-10-14"

SUCCESS = {"complete", "landed", "settled", "closed"}
FAILED = {"blocked_incomplete", "abandoned", "failed"}
OPENISH = {"working", "in_progress", "intent", "drafted", "deferred",
           "pending", "reviewed", "executable"}
PLACEHOLDERS = {"", "none", "not_applicable", "n/a", "na", "pending",
                "unknown", "null"}
PROB_KEYS = {"p", "p_yes", "probability", "prob", "credence", "bridge_p"}
NEG_PREFIX = ("partial", "blocked", "failed", "fail", "sent back", "refused",
              "open", "rejected", "pending")
POS_STATUS = {"closed", "landed", "settled", "satisfied", "complete", "done"}
DEPLOY_EC = re.compile(r"deploy|staging|live|production|choir\.news", re.I)
DEPLOY_TEXT = re.compile(
    r"build\.commit|deployed|redeploy|\bstaging\b|choir\.news", re.I)
NAMED_IDX = re.compile(
    r"acceptance\s+(?:action|item|criterion|check)\s*#?\s*(\d+)", re.I)
NAMED_BRACKET = re.compile(r"finish\.acceptance\[(\d+)\]")
TEXT_PROB = re.compile(r"\bp\s*=\s*0?\.\d+|\bp\(yes\)", re.I)


# ---------------------------------------------------------------- helpers

def first_word(v) -> str:
    if v is None:
        return "none"
    s = str(v).strip().lower()
    if not s:
        return "none"
    w = re.split(r"[\s;,(—]+", s)[0]
    return w.strip(".:;,") or "none"


def norm_readiness(v) -> str:
    w = first_word(v)
    return "drafted" if w == "draft" else w


def norm_text(s) -> str:
    return re.sub(r"\s+", " ", str(s or "")).strip().lower()


def compact(v) -> str:
    return json.dumps(v, sort_keys=True, default=str, ensure_ascii=False)


def is_placeholder(v) -> bool:
    if v is None:
        return True
    if isinstance(v, (list, dict)):
        return len(v) == 0
    return norm_text(v) in PLACEHOLDERS


def r4(x):
    return None if x is None else round(x, 4)


def family_of(stem: str) -> str:
    parts = stem.split("-")
    tok = parts[1] if len(parts) > 1 and parts[0] == "choir" else parts[0]
    return re.sub(r"\d+$", "", tok) or tok


def month_of(stem: str) -> str:
    m = re.search(r"(\d{4}-\d{2})-\d{2}$", stem)
    return m.group(1) if m else "unknown"


def split_frontmatter(text: str):
    if not text.startswith("---"):
        return None
    lines = text.split("\n")
    for i in range(1, len(lines)):
        if lines[i].startswith("---"):
            return "\n".join(lines[1:i])
    return "\n".join(lines[1:])  # P1b: an unclosed '---' opens a YAML document


def hints_from_raw(text: str) -> dict:
    h = {}
    m = re.search(r"^readiness:\s*(.+)$", text, re.M)
    if m:
        h["readiness"] = norm_readiness(m.group(1))
    in_now = False
    for line in text.split("\n"):
        if re.match(r"^now:\s*$", line):
            in_now = True
            continue
        if in_now and re.match(r"^\S", line):
            break
        m = re.match(r"^  status:\s*(.+)$", line) if in_now else None
        if m:
            h["status"] = first_word(m.group(1))
            break
    return h


# ---------------------------------------------------------------- extraction

def extract_acceptance(d: dict):
    fin = d.get("finish") if isinstance(d.get("finish"), dict) else {}
    out = []
    acc = fin.get("acceptance")
    if isinstance(acc, list):
        for item in acc:
            if isinstance(item, dict) and ("action" in item or "proves" in item):
                out.append({"action": str(item.get("action") or ""),
                            "proves": str(item.get("proves") or ""),
                            "evidence_class": str(item.get("evidence_class") or ""),
                            "shape": "standard"})
            elif isinstance(item, dict) and len(item) == 1:
                k, v = next(iter(item.items()))
                out.append({"action": str(k), "proves": str(v or ""),
                            "evidence_class": "", "shape": "single_key_mapping"})
            elif isinstance(item, dict):
                out.append({"action": compact(item), "proves": "",
                            "evidence_class": "", "shape": "other_mapping"})
            else:
                out.append({"action": str(item), "proves": "",
                            "evidence_class": "", "shape": "string"})
        return out, "finish.acceptance"
    stations = fin.get("metamission_stations")
    src = "finish.metamission_stations"
    if not isinstance(stations, list):
        mm = d.get("metamission")
        stations = mm.get("stations") if isinstance(mm, dict) else None
        src = "metamission.stations"
    if isinstance(stations, list):
        for s in stations:
            if isinstance(s, dict):
                out.append({"action": "station %s lands" % s.get("id"),
                            "proves": str(s.get("deliver") or s.get("path") or ""),
                            "evidence_class": "", "shape": "station"})
        return out, src
    return out, "none"


def extract_conjecture(d: dict):
    now = d.get("now") if isinstance(d.get("now"), dict) else {}
    nc = now.get("conjecture") if isinstance(now.get("conjecture"), dict) else {}
    top = d.get("conjecture") if isinstance(d.get("conjecture"), dict) else {}
    claim, src = "", "none"
    for key, val in (("now.conjecture.claim", nc.get("claim")),
                     ("now.conjecture.statement", nc.get("statement")),
                     ("now.conjecture.bridge", nc.get("bridge")),
                     ("conjecture.claim", top.get("claim"))):
        s = norm_text(val)
        if s and not s.startswith("see "):
            claim, src = re.sub(r"\s+", " ", str(val)).strip(), key
            break
    status = nc.get("status") or nc.get("verdict") or top.get("status")
    return claim, src, first_word(status) if status else "none"


def find_probabilities(node, path="", found=None):
    if found is None:
        found = []
    if isinstance(node, dict):
        for k, v in node.items():
            p = "%s.%s" % (path, k) if path else str(k)
            if str(k) == "precommit":
                found.append({"path": p, "value": "precommit block"})
            if str(k).lower() in PROB_KEYS and isinstance(v, (int, float)) \
                    and not isinstance(v, bool) and 0 <= v <= 1:
                found.append({"path": p, "value": v})
            find_probabilities(v, p, found)
    elif isinstance(node, list):
        for i, v in enumerate(node):
            find_probabilities(v, "%s[%d]" % (path, i), found)
    return found


def receipt_text(r) -> str:
    return norm_text(r if isinstance(r, str) else compact(r))


def receipt_positive(r) -> bool:
    if isinstance(r, str):
        return not norm_text(r).startswith(NEG_PREFIX)
    if norm_text(r.get("status")) == "open":
        return False
    for k in ("result", "disposition", "status"):
        v = norm_text(r.get(k))
        if v and v.startswith(NEG_PREFIX):
            return False
    return True


def string_label(r: str) -> str:
    return norm_text(r.split(":", 1)[0]) if ":" in r else ""


def is_terminal(r) -> str:
    """Return the rule id that makes r terminal-class, or ''."""
    if isinstance(r, str):
        lab = string_label(r)
        if "terminal" in lab or lab == "deployed_acceptance" \
                or lab.startswith("settled") or lab.startswith("deployed landing"):
            return "R5"
        if lab.startswith("landed") and DEPLOY_TEXT.search(r) \
                and receipt_positive(r):
            return "R5b"
        return ""
    if not isinstance(r, dict):
        return ""
    for k in ("boundary", "kind", "id", "receipt_id"):
        if "terminal" in norm_text(r.get(k)):
            return "R1"
    for k in ("terminal_receipt", "landed_station", "landed_receipts"):
        if not is_placeholder(r.get(k)):
            return "R2"
    land = r.get("landing") if isinstance(r.get("landing"), dict) else {}
    if not is_placeholder(r.get("deployed_acceptance")) \
            or not is_placeholder(land.get("deployed_acceptance")):
        return "R3"
    kind = norm_text(r.get("kind"))
    res = norm_text(r.get("result") or r.get("disposition"))
    if "completion panel" in kind and res.startswith(("approved", "accept")):
        return "R4"
    if ("deployed_acceptance" in kind or "deployed acceptance" in kind) \
            and receipt_positive(r):
        return "R6"
    if kind == "outcome" and norm_text(r.get("status")) == "settled":
        return "R7"
    if norm_text(r.get("boundary")) == "finish.acceptance" \
            and not is_placeholder(r.get("proof_refs")):
        return "R8"
    return ""


def is_deployed_evidence(r) -> bool:
    if isinstance(r, str):
        if string_label(r) in ("deployed_acceptance", "deployed landing",
                               "deployed identity"):
            return True
        return bool(DEPLOY_TEXT.search(r))
    if not isinstance(r, dict):
        return False
    if is_terminal(r) in ("R3", "R6"):
        return True
    if "deployed" in norm_text(r.get("kind")):
        return True
    return bool(DEPLOY_TEXT.search(compact(r)))  # D-text


def receipt_summary(r) -> dict:
    if isinstance(r, str):
        return {"shape": "string", "label": string_label(r),
                "text": re.sub(r"\s+", " ", r).strip()[:240]}
    if not isinstance(r, dict):
        return {"shape": type(r).__name__}
    out = {"shape": "mapping"}
    for k in ("id", "receipt_id", "kind", "boundary", "status", "result",
              "disposition", "proof_refs", "deployed_acceptance", "landing",
              "evidence_ref"):
        if k in r and r[k] is not None:
            v = r[k]
            out[k] = v if isinstance(v, (list, dict)) else \
                re.sub(r"\s+", " ", str(v)).strip()[:240]
    return out


# ---------------------------------------------------------------- settlement

def goal_outcome(status: str, readiness: str, receipts: list, terminal: list):
    if status in SUCCESS:
        return ("met", "G1") if terminal else ("claimed", "G2")
    if status in FAILED:
        return "not_met", "G3"
    if status == "blocked":
        if readiness in ("intent", "drafted"):
            return "open", "G3-pre-executable"
        return "not_met", "G3"
    if status == "superseded":
        return ("partial", "G4") if terminal else ("not_met", "G4")
    if status == "checkpoint_incomplete":
        return ("partial", "G5") if receipts else ("open", "G5")
    if status in OPENISH:
        return "open", "G6"
    return "unscorable", "G0"


def named_indexes(receipt, criteria):
    txt = receipt_text(receipt)
    hits = set()
    for m in NAMED_IDX.finditer(txt):
        hits.add(int(m.group(1)) - 1)
    for m in NAMED_BRACKET.finditer(txt):
        hits.add(int(m.group(1)))
    for i, c in enumerate(criteria):
        for field in ("action", "proves"):
            t = norm_text(c[field])
            if len(t) >= 24 and t in txt:
                hits.add(i)
    return {i for i in hits if 0 <= i < len(criteria)}


def settle_criteria(g: dict, raw_receipts: list):
    crit = g["acceptance"]
    for c in crit:
        c["outcome"], c["reason"], c["named_by"] = "unresolved", g["outcome"], []
    for r in raw_receipts:
        if not receipt_positive(r):
            continue
        rid = (r.get("id") or r.get("receipt_id")) if isinstance(r, dict) else None
        for i in sorted(named_indexes(r, crit)):
            crit[i]["outcome"], crit[i]["reason"] = "met", "met_named"
            crit[i]["named_by"].append(str(rid or string_label(r) or "receipt"))
    for c in crit:
        if c["outcome"] == "met":
            continue
        if g["outcome"] == "met":
            if g["landing_required"] and not g["has_deployed_evidence"] \
                    and DEPLOY_EC.search(c["evidence_class"] or ""):
                c["outcome"], c["reason"] = "unresolved", "landing_receipt_missing"
            else:
                c["outcome"], c["reason"] = "met", "met_by_goal_receipt"
        elif g["outcome"] == "not_met" and g["outcome_rule"] == "G3":
            c["outcome"], c["reason"] = "not_met", "not_met_by_goal"
        elif g["outcome"] == "not_met" and g["outcome_rule"] == "G4":
            c["outcome"], c["reason"] = "unresolved", "superseded"


# ---------------------------------------------------------------- per file

def analyze_file(path: str) -> dict:
    stem = os.path.splitext(os.path.basename(path))[0]
    with open(path, encoding="utf-8") as fh:
        text = fh.read()
    g = {"id": stem, "path": os.path.join("docs", "definitions",
                                          os.path.basename(path)),
         "family": family_of(stem), "month": month_of(stem),
         "parse": "ok", "parse_error": "", "hints": {},
         "title": stem, "readiness": "none", "definition_version": "none",
         "status_raw": "", "status": "none", "is_metamission": False,
         "member_of": "", "acceptance": [], "acceptance_source": "none",
         "conjecture_claim": "", "conjecture_source": "none",
         "conjecture_status": "none", "better_means": "",
         "goodharting_would_be": "", "probabilities": [],
         "receipts": [], "receipt_count": 0, "terminal_receipts": [],
         "has_deployed_evidence": False, "landing_required": False,
         "outcome": "unscorable", "outcome_rule": "G0"}
    fm = split_frontmatter(text)
    if fm is None:
        g["parse"] = "no_frontmatter"
        g["hints"] = hints_from_raw(text)
        return g
    try:
        d = yaml.safe_load(fm)
    except yaml.YAMLError as exc:
        g["parse"] = "parse_error"
        first = str(exc).strip().split("\n")
        loc = ""
        mark = getattr(exc, "problem_mark", None)
        if mark is not None:
            loc = " (frontmatter line %d, column %d)" % (mark.line + 1,
                                                         mark.column + 1)
        g["parse_error"] = first[0] + loc
        g["hints"] = hints_from_raw(fm)
        return g
    if not isinstance(d, dict):
        g["parse"] = "not_mapping"
        return g

    fin = d.get("finish") if isinstance(d.get("finish"), dict) else {}
    now = d.get("now") if isinstance(d.get("now"), dict) else {}
    val = d.get("value") if isinstance(d.get("value"), dict) else {}
    title = fin.get("deliver") or fin.get("outcome") or d.get("definition_id") or stem
    g["title"] = re.sub(r"\s+", " ", str(title)).strip()
    g["readiness"] = norm_readiness(d.get("readiness"))
    g["definition_version"] = str(d.get("definition_version", "none"))
    g["status_raw"] = re.sub(r"\s+", " ", str(now.get("status") or "")).strip()
    g["status"] = first_word(now.get("status"))
    g["is_metamission"] = "metamission" in d or "metamission" in stem
    g["member_of"] = str(d.get("member_of") or "")
    g["acceptance"], g["acceptance_source"] = extract_acceptance(d)
    (g["conjecture_claim"], g["conjecture_source"],
     g["conjecture_status"]) = extract_conjecture(d)
    g["better_means"] = re.sub(r"\s+", " ", str(val.get("better_means") or "")).strip()
    g["goodharting_would_be"] = re.sub(
        r"\s+", " ", str(val.get("goodharting_would_be") or "")).strip()
    probs = find_probabilities(d)
    for m in TEXT_PROB.finditer(fm):
        probs.append({"path": "text", "value": m.group(0)})
    g["probabilities"] = probs
    land = fin.get("landing") if isinstance(fin.get("landing"), dict) else {}
    g["landing_required"] = norm_text(land.get("required")) == "true"

    raw = d.get("receipts") if isinstance(d.get("receipts"), list) else []
    g["receipt_count"] = len(raw)
    g["receipts"] = [receipt_summary(r) for r in raw]
    terms = []
    for i, r in enumerate(raw):
        rule = is_terminal(r)
        if rule:
            rid = (r.get("id") or r.get("receipt_id")) if isinstance(r, dict) else None
            terms.append({"index": i, "rule": rule,
                          "id": str(rid or string_label(r) or "receipt")})
    g["terminal_receipts"] = terms
    g["has_deployed_evidence"] = any(is_deployed_evidence(r) for r in raw)
    g["outcome"], g["outcome_rule"] = goal_outcome(
        g["status"], g["readiness"], raw, terms)
    settle_criteria(g, raw)
    return g


# ---------------------------------------------------------------- statistics

def brier_const(p, outcomes):
    if not outcomes:
        return None
    return sum((p - o) ** 2 for o in outcomes) / len(outcomes)


def population(goals, variant):
    """Return [(goal, o)] for the named population variant."""
    out = []
    for g in goals:
        oc = g["outcome"]
        if variant == "resolved" and oc in ("met", "not_met"):
            out.append((g, 1 if oc == "met" else 0))
        elif variant == "strict" and oc in ("met", "not_met", "partial", "claimed"):
            out.append((g, 1 if oc == "met" else 0))
        elif variant == "lenient" and oc in ("met", "claimed", "not_met"):
            out.append((g, 1 if oc in ("met", "claimed") else 0))
    return out


def variant_stats(goals, variant):
    pop = population(goals, variant)
    n = len(pop)
    os_ = [o for _, o in pop]
    if n == 0:
        return {"n": 0}
    b = sum(os_) / n
    loo = None
    if n > 1:
        tot = sum(os_)
        loo = sum(((tot - o) / (n - 1) - o) ** 2 for o in os_) / n
    fam = {}
    for g, o in pop:
        fam.setdefault(g["family"], []).append(o)
    cond = None
    if n > 1:
        tot, acc = sum(os_), 0.0
        for g, o in pop:
            v = fam[g["family"]]
            if len(v) > 1:
                p = (sum(v) - o) / (len(v) - 1)
            else:
                p = (tot - o) / (n - 1)
            acc += (p - o) ** 2
        cond = acc / n
    return {"n": n, "yes": sum(os_), "base_rate": r4(b),
            "climatology_brier": r4(brier_const(b, os_)),
            "loo_climatology_brier": r4(loo),
            "family_conditional_brier": r4(cond),
            "half_brier": r4(brier_const(0.5, os_))}


def grouped(goals, key):
    groups = {}
    for g in goals:
        groups.setdefault(str(g[key]), []).append(g)
    rows = []
    for k in sorted(groups):
        gs = groups[k]
        counts = {c: sum(1 for g in gs if g["outcome"] == c) for c in OUTCOMES}
        res = population(gs, "resolved")
        strict = population(gs, "strict")
        rows.append({"group": k, "n": len(gs), **counts,
                     "resolved_rate": r4(sum(o for _, o in res) / len(res)) if res else None,
                     "strict_rate": r4(sum(o for _, o in strict) / len(strict)) if strict else None})
    return rows


OUTCOMES = ("met", "claimed", "partial", "not_met", "open", "unscorable")


def conjecture_table(goals):
    """Bridge-conjecture status (E5) counted per goal outcome."""
    t = {}
    for g in goals:
        if g["parse"] != "ok":
            continue
        row = t.setdefault(g["conjecture_status"], {k: 0 for k in OUTCOMES})
        row[g["outcome"]] += 1
    return [{"status": k, **t[k]} for k in sorted(t)]
CRIT_OUTCOMES = ("met", "not_met", "unresolved")


def build(root: str, date: str) -> dict:
    paths = sorted(glob.glob(os.path.join(root, "docs", "definitions", "*.md")))
    goals = [analyze_file(p) for p in paths]
    crit = [c for g in goals for c in g["acceptance"]]
    reasons = {}
    for c in crit:
        reasons[c["reason"]] = reasons.get(c["reason"], 0) + 1
    crit_counts = {k: sum(1 for c in crit if c["outcome"] == k) for k in CRIT_OUTCOMES}
    cm, cn = crit_counts["met"], crit_counts["not_met"]
    summary = {
        "files": len(goals),
        "parse": {k: sum(1 for g in goals if g["parse"] == k)
                  for k in ("ok", "parse_error", "no_frontmatter", "not_mapping")},
        "outcomes": {k: sum(1 for g in goals if g["outcome"] == k) for k in OUTCOMES},
        "criteria": {"total": len(crit), **crit_counts,
                     "reasons": dict(sorted(reasons.items())),
                     "met_share_of_settled": r4(cm / (cm + cn)) if cm + cn else None},
        "goals_with_probability": sum(1 for g in goals if g["probabilities"]),
        "criteria_with_probability": 0,
        "goals_with_conjecture_claim": sum(1 for g in goals if g["conjecture_claim"]),
        "goals_with_value": sum(1 for g in goals if g["better_means"]),
        "baseline": {v: variant_stats(goals, v)
                     for v in ("resolved", "strict", "lenient")},
        "by_readiness": grouped(goals, "readiness"),
        "by_family": grouped(goals, "family"),
        "by_month": grouped(goals, "month"),
        "by_definition_version": grouped(goals, "definition_version"),
        "by_metamission": grouped(goals, "is_metamission"),
        "conjecture_by_outcome": conjecture_table(goals),
    }
    return {"generator": "scripts/goal_precommitments.py",
            "rules_version": RULES_VERSION, "date": date,
            "summary": summary, "goals": goals}


# ---------------------------------------------------------------- rendering

def fmt(x):
    if x is None:
        return "–"
    if isinstance(x, float):
        return "%.3f" % x
    return str(x)


def cell(s: str, n: int = 0) -> str:
    s = str(s).replace("|", "\\|").replace("\n", " ")
    return s if not n or len(s) <= n else s[: n - 1] + "…"


def render(data: dict) -> str:
    s = data["summary"]
    L = []
    L.append("# Goal Ledger: Throughline Goals as Retrospective Precommitments")
    L.append("")
    L.append("Generated by `scripts/goal_precommitments.py`; do not edit by hand. "
             "Regenerate with `python3 -I scripts/goal_precommitments.py` and "
             "check freshness with `--check`.")
    L.append("")
    L.append("- Date: %s" % data["date"])
    L.append("- Rules: `%s` (every rule is stated in the script docstring and in "
             "`docs/archive/reports/goal-precommitments-2026-10-14.md`)" % data["rules_version"])
    L.append("- Source: `docs/definitions/*.md`, settled from each file's own "
             "`now.status` and `receipts` only")
    L.append("")
    L.append("**The base rates and Brier figures below are a climatology "
             "baseline, not a track record: no goal file recorded a "
             "probability.**")
    L.append("")
    L.append("## Summary")
    L.append("")
    L.append("| Measure | Value |")
    L.append("|---|---|")
    L.append("| Goal files | %d |" % s["files"])
    for k, v in s["parse"].items():
        L.append("| Parse: %s | %d |" % (k, v))
    for k, v in s["outcomes"].items():
        L.append("| Goal outcome: %s | %d |" % (k, v))
    c = s["criteria"]
    L.append("| Acceptance criteria (parsed files) | %d |" % c["total"])
    for k in CRIT_OUTCOMES:
        L.append("| Criteria %s | %d |" % (k, c[k]))
    for k, v in c["reasons"].items():
        L.append("| Criteria reason: %s | %d |" % (k, v))
    L.append("| Criteria met share of settled (met / (met + not_met)) | %s |"
             % fmt(c["met_share_of_settled"]))
    L.append("| Goals with any stated probability | %d |" % s["goals_with_probability"])
    L.append("| Goals with a bridge conjecture claim | %d |" % s["goals_with_conjecture_claim"])
    L.append("| Goals with `value.better_means` | %d |" % s["goals_with_value"])
    L.append("")
    L.append("## Climatology baseline (constant forecast = base rate)")
    L.append("")
    L.append("| Population | n | met | Base rate | Brier (in-sample) | Brier (leave-one-out) "
             "| Brier (family, leave-one-out) | Brier (p = 0.5) |")
    L.append("|---|---|---|---|---|---|---|---|")
    labels = {"resolved": "resolved: met vs not_met",
              "strict": "strict: met vs not_met + partial + claimed",
              "lenient": "lenient: met + claimed vs not_met"}
    for v in ("resolved", "strict", "lenient"):
        b = s["baseline"][v]
        if not b.get("n"):
            L.append("| %s | 0 | – | – | – | – | – | – |" % labels[v])
            continue
        L.append("| %s | %d | %d | %s | %s | %s | %s | %s |" % (
            labels[v], b["n"], b["yes"], fmt(b["base_rate"]),
            fmt(b["climatology_brier"]), fmt(b["loo_climatology_brier"]),
            fmt(b["family_conditional_brier"]), fmt(b["half_brier"])))
    L.append("")
    for title, key in (("By readiness", "by_readiness"), ("By family", "by_family"),
                       ("By month", "by_month"),
                       ("By definition_version", "by_definition_version"),
                       ("Metamission spine vs station", "by_metamission")):
        L.append("### %s" % title)
        L.append("")
        L.append("| Group | n | met | claimed | partial | not_met | open | unscorable "
                 "| resolved rate | strict rate |")
        L.append("|---|---|---|---|---|---|---|---|---|---|")
        for r in s[key]:
            L.append("| %s | %d | %d | %d | %d | %d | %d | %d | %s | %s |" % (
                r["group"], r["n"], r["met"], r["claimed"], r["partial"],
                r["not_met"], r["open"], r["unscorable"],
                fmt(r["resolved_rate"]), fmt(r["strict_rate"])))
        L.append("")
    L.append("### Bridge conjecture status by goal outcome")
    L.append("")
    L.append("The bridge conjecture's status (rule E5), as hand-maintained. A met goal whose bridge is still `proposed` or "
             "`active` landed without anyone settling the deeper claim.")
    L.append("")
    L.append("| Conjecture status | " + " | ".join(OUTCOMES) + " |")
    L.append("|---|" + "---|" * len(OUTCOMES))
    for r in s["conjecture_by_outcome"]:
        L.append("| %s | %s |" % (r["status"], " | ".join(str(r[k]) for k in OUTCOMES)))
    L.append("")
    L.append("## Goals")
    L.append("")
    L.append("| Goal | Readiness | Status | Outcome (rule) | Criteria | Met | Not met "
             "| Unresolved | p? | Receipts | Terminal |")
    L.append("|---|---|---|---|---|---|---|---|---|---|---|")
    for g in data["goals"]:
        cr = g["acceptance"]
        L.append("| `%s` | %s | %s | %s (%s) | %d | %d | %d | %d | %s | %d | %d |" % (
            g["id"], g["readiness"], cell(g["status"]), g["outcome"], g["outcome_rule"],
            len(cr), sum(1 for c in cr if c["outcome"] == "met"),
            sum(1 for c in cr if c["outcome"] == "not_met"),
            sum(1 for c in cr if c["outcome"] == "unresolved"),
            "y" if g["probabilities"] else "n", g["receipt_count"],
            len(g["terminal_receipts"])))
    L.append("")
    L.append("## Parse errors")
    L.append("")
    bad = [g for g in data["goals"] if g["parse"] != "ok"]
    if not bad:
        L.append("None.")
    else:
        L.append("These files are counted as `unscorable`. The hints are read by "
                 "regex and are never scored.")
        L.append("")
        L.append("| Goal | Parse | Error | Hint: readiness | Hint: status |")
        L.append("|---|---|---|---|---|")
        for g in bad:
            L.append("| `%s` | %s | %s | %s | %s |" % (
                g["id"], g["parse"], cell(g["parse_error"] or "–", 120),
                g["hints"].get("readiness", "–"), g["hints"].get("status", "–")))
    L.append("")
    L.append("## Unscorable and claimed goals")
    L.append("")
    rows = [g for g in data["goals"]
            if g["outcome"] in ("unscorable", "claimed") and g["parse"] == "ok"]
    if not rows:
        L.append("None.")
    else:
        L.append("| Goal | Outcome | Raw status | Why |")
        L.append("|---|---|---|---|")
        for g in rows:
            why = ("status word outside the rule vocabulary"
                   if g["outcome"] == "unscorable"
                   else "success status with no terminal-class receipt")
            L.append("| `%s` | %s | %s | %s |" % (
                g["id"], g["outcome"], cell(g["status_raw"], 90), why))
    L.append("")
    return "\n".join(L)


# ---------------------------------------------------------------- main

def main(argv=None) -> int:
    here = os.path.dirname(os.path.abspath(__file__))
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--root", default=os.path.dirname(here),
                    help="repo root (default: parent of scripts/)")
    ap.add_argument("--date", default=DEFAULT_DATE)
    ap.add_argument("--out", default=os.path.join("docs", "calibration", "goal-ledger.md"),
                    help="ledger path, relative to --root")
    ap.add_argument("--json", dest="json_path", default=None,
                    help="also dump the full extraction as JSON to this path")
    ap.add_argument("--check", action="store_true",
                    help="exit 1 if the ledger on disk differs from a regeneration")
    a = ap.parse_args(argv)
    data = build(a.root, a.date)
    md = render(data)
    out = os.path.join(a.root, a.out)
    if a.check:
        try:
            with open(out, encoding="utf-8") as fh:
                current = fh.read()
        except OSError:
            current = None
        if current != md:
            print("goal ledger is stale: %s (regenerate with "
                  "python3 -I scripts/goal_precommitments.py)" % a.out, file=sys.stderr)
            return 1
        print("goal ledger is current: %s" % a.out)
        return 0
    os.makedirs(os.path.dirname(out), exist_ok=True)
    with open(out, "w", encoding="utf-8") as fh:
        fh.write(md)
    if a.json_path:
        with open(a.json_path, "w", encoding="utf-8") as fh:
            json.dump(data, fh, sort_keys=True, indent=2, ensure_ascii=False,
                      default=str)
            fh.write("\n")
    s = data["summary"]
    print("wrote %s: %d files, outcomes %s, criteria %s" % (
        a.out, s["files"], compact(s["outcomes"]),
        compact({k: s["criteria"][k] for k in ("total",) + CRIT_OUTCOMES})))
    return 0


if __name__ == "__main__":
    sys.exit(main())
