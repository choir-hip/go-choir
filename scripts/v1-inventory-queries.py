#!/usr/bin/env python3
"""Pinned mechanical queries for the mission-2 V1 desk-vocabulary inventory.

query_version: 1 (bound in the frozen artifact header; bump on any pattern change
and re-freeze the artifact in the same Define update).

Emits TSV `file \\t line \\t query \\t text` for every role-field-anchored hit
over the frozen corpus manifest. Only the queries listed in QUERIES may carry
desk vocabulary into the inventory; prose, comments, and test fixtures are out
by construction (field-anchored patterns) and by corpus filter (non-test
production sources only).

Usage: v1-inventory-queries.py <manifest> [--corpus-root .]
"""
import re
import sys

QUERY_VERSION = 1

# Each query: (id, class, regex). Text matched against single stripped lines.
QUERIES = [
    ("Q1", "field-assign", re.compile(
        r"(ActorProfile|AgentProfile|AgentRole|AuthorityProfile|FromRole|"
        r"agent_role|agent_profile|requested_by_profile|TargetProfile|"
        r"callerProfile|targetProfile|coagentProfile)\s*[:=!]+\s*"
        r"\"?([A-Za-z_:][A-Za-z0-9_:.-]*)")),
    ("Q2", "role-literal", re.compile(
        r"(Role|Profile|role|profile)\s*:\s*"
        r"(\"super\"|\"co-super\"|\"cosuper\"|\"researcher\"|\"research\"|"
        r"\"engineering\"|\"management\"|\"texture\"|\"conductor\"|\"processor\"|"
        r"\"reconciler\"|\"email\"|\"verifier\"|\"owner\"|\"trusted-core\"|"
        r"'super'|'(co-super|cosuper)'|'researcher'|'texture'|'conductor')")),
    ("Q3", "canonical-const", re.compile(
        r"agentprofile\.(Super|CoSuper|Researcher|Texture|Processor|Reconciler|Conductor|Email|Canonical|PolicyFor|CanSpawn|CanMessage)")),
    ("Q4", "capsule-role", re.compile(
        r"capsule\.(RoleSuper|RoleCoSuper|RoleResearcher|RoleVerbSets)")),
    # Bare Role/Profile struct keys with non-literal values: generic carriers
    # transporting versioned values (ordered after Q1/Q3 so constants keep
    # their precise query attribution).
    ("Q12", "generic-carrier", re.compile(
        r"(?<![A-Za-z])(Role|Profile|AgentRole|AgentProfile)\s*:\s*"
        r"[a-zA-Z_][\w.()\[\]]*")),
    # Provider/LLM message roles and computer roles: distinct namespaces,
    # never desk vocabulary.
    ("Q13", "other-namespace", re.compile(
        r"msg\.Role|message\.Role|computerRole\(|warmness_class")),
    # Bare V1/V2 wire-token literals with optional identity-suffix.
    ("Q5", "wire-token", re.compile(
        r"['\"](super|co-super|cosuper|researcher|research|engineering)(:[^'\"]*)?['\"]")),
    # Prompt registry role ids and runtime overlay role ids.
    ("Q6", "prompt-role-id", re.compile(
        r"^role:\s*(\S+)\s*$")),
    # Computer-owned model-policy TOML role sections.
    ("Q7", "toml-roles", re.compile(
        r"\[roles\.([A-Za-z-]+)\]")),
    # Frozen compound protocol identifiers (never desk vocabulary).
    ("Q8", "frozen-compound", re.compile(
        r"researcher_update|researcher_confirmed|researcher_refuted|"
        r"researcher_qualified|persistent-super-recovery|lifecycle-researcher-"
        r"admission-recovery|co-super-(open|bind|report|cancel|capsule|restart|"
        r"system|fate)|assignment-report|lifecycle_texture_control|"
        r"super-runtime|co-super-runtime|rlm-co-super-runtime|"
        r"researcher-runtime|choir[.:]co.super|co-super-(grant|execution|fate|"
        r"report)|TerminalPropositionV1|terminal-proposition")),
    # Role-position owner tokens only (PrivacyClass owner is a separate frozen
    # namespace covered by Q9b; OwnerID and prose owners are not vocabulary).
    ("Q9", "owner-token", re.compile(
        r"(ActorProfile|AgentProfile|AgentRole|AuthorityProfile|"
        r"(?<![A-Za-z])Role|From)\s*:\s*\"owner\"")),
    ("Q9b", "privacy-owner", re.compile(
        r"PrivacyClass[^\"\n]*\"owner\"|privacy\s*==\s*\"owner\"")),
    ("Q10", "body-provenance", re.compile(
        r"CreatedBy:")),
    # Model catalog role routing metadata.
    ("Q11", "catalog-routing", re.compile(
        r"RecommendedFor:")),
]


def load_manifest(path):
    files = []
    with open(path) as fh:
        for line in fh:
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            files.append(line)
    return files


def main():
    manifest = sys.argv[1]
    root = "."
    if len(sys.argv) > 3 and sys.argv[2] == "--corpus-root":
        root = sys.argv[3]
    files = load_manifest(manifest)
    import os
    for f in files:
        p = os.path.join(root, f)
        try:
            with open(p, encoding="utf-8", errors="replace") as fh:
                for n, line in enumerate(fh, 1):
                    s = line.strip()
                    if not s:
                        continue
                    if s.startswith(("//", "#", "*", "<!--", "--", "/*")):
                        continue
                    for qid, _, rx in QUERIES:
                        if rx.search(s):
                            print(f"{f}\t{n}\t{qid}\t{s[:300]}")
                            break
        except FileNotFoundError:
            print(f"MISSING\t0\t-\t{f}")


if __name__ == "__main__":
    main()
