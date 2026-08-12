---
definition_version: 2
definition_id: choir-instruction-substrate-prune-2026-08-11
execution_mode: scope_disjoint_maintenance

start:
  captured_at: 2026-08-11T21:30:00Z
  source:
    canonical_ref: main@f1fdaf7c
    deploy_identity: "staging https://choir.news frontend and proxy 914f7a5d976a, proxy status ok, deploy time 2026-08-11T18:11:01Z. This mission does not change deployed product behavior; staging identity is captured for reconciliation only."
  worktree_inventory:
    status: reconciled
    evidence_ref: 2026-08-11 read-only git status after f1fdaf7c; clean single worktree /Users/wiz/go-choir
    preservation_rule: Preserve every non-primary worktree and all unrelated WIP. This Definition owns itself, the beads store, cmd/doccheck, and the instruction-packet documents it names.
  worktrees:
    - path: /Users/wiz/go-choir
      status: clean
      class: goal_candidate
      owner: owner-and-session
      touch: goal_owned
      paths_or_digest: [docs/definitions/choir-instruction-substrate-prune-2026-08-11.md, .beads/, cmd/doccheck/, AGENTS.md, docs/ACTIVE.md, docs/mission-graph.yaml, docs/doc-authority-manifest.yaml]
      recovery: revert the mission commits
  candidates:
    - id: none
  observed_artifact:
    - claim: "Beads is half-wired, not merely dormant. .beads/hooks/ contains pre-commit, pre-push, post-checkout, post-merge, and prepare-commit-msg, but .git/hooks contains no installed hook, so none of them run. The bd binary is present on PATH. The store has not been written since 2026-07-01."
      evidence_ref: "ls .git/hooks (samples only); ls .beads/hooks; which bd -> /opt/homebrew/bin/bd; .beads/last-touched 2026-07-01"
    - claim: "The store holds 198 issues — 93 open, 13 in_progress, 92 closed — with no record updated after 2026-07-01. Sampled open work belongs to a superseded era (store consolidation and corpusd rename, unified object graph, nucleus capsule runtime, beads kanban UI, desktop local VM), all predating the self-development-effects arc."
      evidence_ref: ".beads/issues.jsonl parsed 2026-08-11; sample of 18 most-recent open/in_progress records"
    - claim: "doccheck actively validates the beads store and reports drift: rule R8 emits 'beads/mission-graph binding drift: 116 mg:* epics vs 21 graph nodes' plus three open-conjecture epic findings. The integration is maintained in cmd/doccheck/beads.go and cmd/doccheck/beads_rule.go with tests."
      evidence_ref: doccheck.json rule R8; cmd/doccheck/beads.go; cmd/doccheck/beads_rule.go; cmd/doccheck/beads_rule_test.go
    - claim: "The headline warning count conflates severities. The 141 findings are 31 severity=warning and 110 severity=info. All 31 real warnings are heresy-detector rules: H1 (23), H3 (4), H5 (3), H2 (1). No R-rule produces a real warning."
      evidence_ref: doccheck.json severity split, 2026-08-11
    - claim: "74 findings come from gitignored scratch directories (.agentic-consensus/, .beads/). The corpus walk skips a hardcoded set — .git, node_modules, vendor, dist, test-results, .gstack — which does not include gitignored paths, so panel output and the beads store are scanned as corpus. docs/archive produces zero findings and is already correctly handled."
      evidence_ref: cmd/doccheck/main.go:541-546,1315-1320; .gitignore:77 (.agentic-consensus/); doccheck.json path distribution
    - claim: "Real warnings are concentrated, not diffuse: docs/definitions/og-dolt-heresy-completion-2026-07-08.md (12), docs/texture-agentic-invariants-2026-06-13.md (6), docs/definitions/choir-seam-repair-2026-07-10.md (4), docs/choir-prompting-invariants.md (3)."
      evidence_ref: doccheck.json path histogram, 2026-08-11
    - claim: "One real warning lands on the active executable Definition: H1 retired vocabulary 'lease' at docs/definitions/choir-supervised-self-development-effects-2026-08-11.md:400, inside the restore procedure's materialization-lease step."
      evidence_ref: doccheck.json; the cited line
    - claim: "The mandatory instruction packet is approximately 19,200 words: choir-doctrine 4,374; skills/definition 2,774; computer-ontology 2,489; agent-product-doctrine 1,925; AGENTS 1,665; ACTIVE 1,431; choir-vision 1,195; semantic-registry 881; standing-questions 780. CLAUDE.md is a symlink to AGENTS.md and adds no duplicate reading."
      evidence_ref: "wc -w over the packet named in AGENTS.md; git ls-files -s CLAUDE.md shows mode 120000 symlink"
    - claim: "The tracked docs corpus is 293 markdown files: 177 archive, 54 evidence, 28 root, 24 definitions, 8 problems, 2 legal. doccheck scans 383 docs against 53 manifest entries, inferring 330."
      evidence_ref: find docs -name '*.md'; doccheck.json docs_scanned/manifest_entries/inferred_docs
  problems_documented:
    - id: beads-half-wired-2026-08-11
      problem: "Beads occupies the worst position available: unused for six weeks with no installed hooks, while cmd/doccheck still validates it and reports 116-vs-21 binding drift on every run. It also duplicates work the Definition route map and mission-graph already carry, which is the dual-path shape doctrine I5 rejects."
      evidence_ref: "see observed_artifact beads claims; docs/choir-doctrine.md I5"
      consequence: "Resolve in one direction. This Definition removes beads after triage rather than completing the integration, because the Definition/route-map/mission-graph triple already owns dependency-ordered work and a second owner would have to be kept honest against it forever."
    - id: doccheck-signal-conflation-2026-08-11
      problem: "The report's headline number treats 110 info findings as warnings and scans gitignored scratch as corpus, so a run reports 141 when 31 are actionable. A number nobody can act on is a number nobody reads, and the checker gates docs-only CI."
      evidence_ref: "cmd/doccheck/main.go:340-352 (R3 emits Severity info); main.go:541-546 skip list; doccheck.json"
      consequence: "Separate severities in the summary and exclude gitignored paths from the corpus walk. Do not suppress rules to lower the number."
  unknowns:
    - "Whether any of the 106 open/in_progress beads issues names work that is still real and has no home in a Definition, mission-graph node, or docs/problems entry. Answered by triage in step 1; the delete does not proceed until every issue has a disposition."
    - "Whether 'lease' is genuinely retired vocabulary or the H1 detector is over-matching a legitimate current term. Decides whether step 4 edits the Definition or the detector."
    - "How much of the ~19,200-word instruction packet is load-bearing invariant versus restatement. Answered only by the receipted pass in step 5, where every removal must name the document that absorbs the claim."

finish:
  deliver: "The instruction substrate is single-owner and legible before the effects mission starts. Beads is removed after every open issue is dispositioned; doccheck reports a number that is actionable and scans only the tracked corpus; the real warnings are cleared at their source; and the mandatory reading packet is measurably smaller with no invariant left homeless."
  artifact: "Pushed commits 2bda3a2e, 8697c0ab, 006e39c0, and 4865181e carry the triage record, beads removal, doccheck signal correction, source-warning repairs, and receipted packet reduction. Full manual CI run 31548636633 passed all selected gates, including the race matrix and Docs Truth Check. The source-bearing landing deployed 006e39c0 to staging; staging health reports proxy status ok and that commit identity."
  acceptance:
    - action: "Beads triage: disposition every open and in_progress issue as (a) already delivered, (b) superseded by a later decision, (c) still real and relocated to a named Definition, mission-graph node, or docs/problems entry, or (d) explicitly abandoned. Record the disposition table with per-issue ids."
      proves: "The delete is a decision on evidence, not a bet. Nothing real is lost silently."
      evidence_class: repo artifact
    - action: "Beads removal: delete .beads/ from the tree, remove the R8 rule and beads reader from cmd/doccheck with their tests, and confirm no remaining reference to the store in code, workflows, or docs."
      proves: "One owner for work tracking. The drift warning is retired by removing the second owner, not by silencing the rule."
      evidence_class: deployed proof
    - action: "doccheck signal fix: exclude gitignored paths from the corpus walk and report severity=warning separately from severity=info in the summary line and report. Show the same run before and after."
      proves: "The headline number is actionable. Expect roughly 31 warnings against the tracked corpus where 141 was reported."
      evidence_class: deployed proof
    - action: "Clear the real warnings at source: resolve all H1/H2/H3/H5 findings, including the 'lease' hit on the active effects Definition, by correcting the document or correcting an over-matching detector — with the choice justified per finding."
      proves: "The remaining count is true rather than tuned."
      evidence_class: repo artifact
    - action: "Reading-packet reduction: reduce the mandatory packet from its 19,200-word baseline, and for every removed or relocated passage name the document that now carries the claim. Produce a diff-level receipt table."
      proves: "The packet is smaller because it is less redundant, not because invariants were dropped."
      evidence_class: repo artifact
    - action: "Invariant conservation check: enumerate the doctrine invariants and semantic-registry laws before and after, and show the sets are equal."
      proves: "No invariant lost its home. This is the acceptance that makes step 5 safe."
      evidence_class: repo artifact
    - action: "Effects-Definition readiness: re-run doccheck and confirm the active effects Definition carries zero findings, and that ACTIVE.md, mission-graph.yaml, and doc-authority-manifest.yaml still resolve it as the executable entrypoint."
      proves: "The prep mission left the mission it was preparing for in a runnable state."
      evidence_class: deployed proof
  rollback: "Revert the mission commits through origin/main and CI. The beads store remains recoverable from git history at f1fdaf7c and earlier; the triage record is durable documentation independent of the store, so a later decision to re-adopt beads does not require the deleted JSONL."
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deployed_acceptance]
  not_done_when:
    - Beads was deleted without a disposition for every open and in_progress issue.
    - A beads issue naming still-real work was closed by deletion rather than relocated to a named home.
    - The warning count fell because rules were suppressed, thresholds raised, or findings downgraded rather than because documents were fixed or scratch stopped being scanned.
    - The gitignore exclusion is broad enough to stop the checker seeing tracked docs.
    - The reading packet shrank by deleting an invariant that no other document now carries.
    - The effects Definition, its supplement, or the three registries were altered beyond what this mission's warnings require.
    - The prep mission grew scope and delayed the effects mission it exists to unblock.
    - Source changes to cmd/doccheck landed without full CI, or docs-only commits forced a deploy path.

boundaries:
  mutation_class: orange
  authority_sources: [owner-ratified decisions, docs/choir-doctrine.md, docs/agent-product-doctrine.md, docs/computer-ontology.md, docs/standing-questions.md, AGENTS.md]
  must_preserve:
    - Every doctrine invariant and semantic-registry law keeps a home. Relocation is allowed; loss is not.
    - The active effects Definition stays executable throughout and is not blocked by this mission.
    - Detector strength is preserved. Warnings are cleared by fixing documents, or by narrowing a detector that is demonstrably over-matching with the demonstration recorded.
    - docs/archive stays excluded and untouched; historical documents are not rewritten to satisfy current-vocabulary rules.
    - The Docs Truth Check keeps gating docs-only pushes, and cmd/doccheck changes take the full CI path.
  excluded:
    - Any change to the effects Definition's design, scope, or route map.
    - Migrating work tracking to a replacement tool. Removing beads leaves Definitions plus mission-graph as the single owner; introducing a third system is out.
    - Rewriting docs/archive or docs/evidence content.
    - Product or runtime behavior of the computer.
  protected_surfaces: [cmd/doccheck and the Docs Truth Check gate, docs/choir-doctrine.md, docs/semantic-registry.md, docs/computer-ontology.md, AGENTS.md, the three registries, the active effects Definition]
  completion_evidence_floor: [repo artifact, deployed proof for CI-gating changes]

measures:
  - name: actionable warning count
    kind: gate
    baseline: "31 warnings + 110 info reported as '141 warnings'"
    desired: "warnings and info reported separately; warnings at 0 against the tracked corpus"
    decision_use: gates step 5; a checker nobody reads cannot protect a packet being pruned
    cannot_prove: that the documents are correct, only that the detectors are quiet
  - name: beads disposition coverage
    kind: gate
    baseline: 0 of 106
    desired: 106 of 106 dispositioned, with relocations named
    decision_use: unlocks the delete
    cannot_prove: that the relocated work will be done
  - name: instruction packet size
    kind: weak_signal
    baseline: 19179 words across the packet named in AGENTS.md
    desired: materially smaller with invariant sets provably equal
    decision_use: inspect what a fresh agent must read before acting; never advances complete alone
    cannot_prove: that the remaining packet is understood or sufficient
  - name: invariant set equality
    kind: gate
    baseline: "current doctrine invariants and SEM laws enumerated"
    desired: before-set equals after-set
    decision_use: the safety fence on packet reduction; a mismatch blocks the mission
    cannot_prove: that each invariant is well-placed

now:
  status: complete
  slice: "The prep mission is complete. Beads was triaged before deletion; its store and doccheck integration are gone; doccheck now scans the non-ignored corpus and separates warnings from info; all actionable H findings are resolved; and the mandatory packet is smaller with invariant conservation recorded."
  question: "Resolved: 106/106 open and in_progress records have durable dispositions; 0 warnings remain; the packet is 16,600 words versus the recorded 19,179-word baseline; and doctrine I1-I16 plus SEM-01-SEM-09 are equal before and after."
  reconciliation:
    observed_at: 2026-08-11
    source_ref: main@4865181e
    deploy_identity: "staging 006e39c0f4801ef6f2ffb3e1162c29f25e4f0939; deploy 2026-08-11T23:59:35Z; final receipt commit 4865181e is docs-only"
    authority_identities: [docs/choir-doctrine.md, docs/agent-product-doctrine.md, docs/computer-ontology.md, docs/standing-questions.md, AGENTS.md]
    policy_resolution_ref: not_applicable
    worktree_inventory_ref: "2026-08-11 final status: unrelated WIP preserved in docs/definitions/documentation-authority-reduction-2026-07-09.md and docs/definitions/choir-sandbox-autoputer-rename-2026-08-11.md"
    status: reconciled
  candidate:
    id: none
    state: none
  decision:
    selected: "Prep mission before the effects mission. Beads is removed after full triage rather than adopted, because Definitions plus mission-graph already own dependency-ordered work and a second owner would be a permanent dual path. doccheck is fixed for signal rather than suppressed. The instruction packet is pruned subtractively with a per-removal receipt and an invariant-conservation gate."
    kind: architecture
    status: proposed
    source: orchestrator
    evidence_ref: "docs/evidence/choir-instruction-substrate-prune-triage-2026-08-11.md; docs/evidence/choir-instruction-substrate-prune-doccheck-2026-08-11.md; docs/evidence/choir-instruction-substrate-prune-packet-2026-08-11.md"
    owner_ratification_ref: "owner direction 2026-08-11: 'either we go all in on beads or we delete it. and the docs-based codeflow needs a pruning and streamlining anyway'; drafting requested. The implementation follows that direction without introducing a replacement tracker."
    recorded_at: 2026-08-11
    consequence: "The effects mission can start against one work owner, actionable doccheck output, and a reduced packet. The effects Definition and its route map were not altered by this prep mission."
  evidence_refs:
    - docs/evidence/choir-instruction-substrate-prune-triage-2026-08-11.md
    - docs/evidence/choir-instruction-substrate-prune-doccheck-2026-08-11.md
    - docs/evidence/choir-instruction-substrate-prune-packet-2026-08-11.md
    - "go run ./cmd/doccheck --mode=full: 317 docs, 0 warnings, 37 info findings"
    - "go run ./cmd/doccheck --mode=live: passed; 10 content documents plus router"
    - "go test ./cmd/doccheck && go run ./cmd/doccheck --mode=full: passed in the source-bearing landing before final doc-only receipt correction"
  blocker_or_risk: "The prep mission's remaining risk is ordinary maintenance: the effects mission still requires its own runtime and staging evidence. This mission did not touch product behavior or the effects Definition's design."
  next_action: "Start the effects Definition; do not re-open the retired beads store or re-run this prep mission."

receipts:
  - id: beads-triage-2026-08-11
    artifact: docs/evidence/choir-instruction-substrate-prune-triage-2026-08-11.md
    coverage: "106/106 open and in_progress records"
    dispositions: "12 already delivered; 94 superseded; 0 relocated; 0 abandoned"
    delete_gate: passes
  - id: beads-removal-2026-08-11
    artifact: "commits 8697c0ab and 006e39c0"
    coverage: ".beads/ removed; R8 reader/rule/tests and unused migration commands removed; no current beads references remain"
    verification: "git status and scoped reference audit after landing"
  - id: doccheck-signal-2026-08-11
    artifact: docs/evidence/choir-instruction-substrate-prune-doccheck-2026-08-11.md
    coverage: "baseline 143 findings (31 warning, 112 info) to corrected 315 tracked docs (0 warning, 38 info) in the same-run receipt"
    verification: "current full run: 317 docs, 0 warnings, 37 info findings; current live run passed"
  - id: packet-conservation-2026-08-11
    artifact: docs/evidence/choir-instruction-substrate-prune-packet-2026-08-11.md
    coverage: "16,600 words versus the recorded 19,179-word baseline; I1-I16 and SEM-01-SEM-09 equal before and after"
    verification: "exact nine-path wc measurement and explicit before/after set comparison"
  - id: source-landing-2026-08-11
    artifact: "origin/main at 4865181e; source-bearing staging deploy at 006e39c0"
    coverage: "Source-bearing commits 8697c0ab and 006e39c0 pushed; staging health reports proxy status ok at 006e39c0; final docs-only receipt commit 4865181e pushed."
    verification: "GitHub Actions manual full CI run 31548636633 passed all selected gates, including race tests and Docs Truth Check; prior push run 31546940207 deployed successfully but its rerun was blocked only by missing differential-SBOM artifact."

view:
  path: none
  generator: none

---

# Instruction Substrate Prune — Prep Definition

Prep mission for
[choir-supervised-self-development-effects-2026-08-11.md](choir-supervised-self-development-effects-2026-08-11.md).
It exists so that the effects mission starts against a substrate with one work
owner, a checker whose output is actionable, and an instruction packet a fresh
agent can read without spending its first tokens on redundancy.

## What is being fixed

**Beads is half-wired.** Its hooks are not installed, nothing has written to it
since 2026-07-01, and its 106 open issues belong to a superseded era. But
`cmd/doccheck` still validates it and reports 116-vs-21 binding drift every run.
It also duplicates what the Definition route map and mission-graph already own.
Resolve in one direction: triage, then remove.

**doccheck reports a number nobody can act on.** A run says "141 warnings" when
31 are actionable — the other 110 are severity `info`, and 74 of all findings
come from gitignored scratch (`.agentic-consensus/`, `.beads/`) that the walk's
hardcoded skip list does not cover. Separate the severities, exclude gitignored
paths, then fix what remains.

**The mandatory packet is ~19,200 words.** That is what a fresh `/goal` agent
reads before it can act. Some of it is invariant bought with failure receipts
and must stay; some is restatement across doctrine, ontology, product doctrine,
and the semantic registry. Only a receipted pass can tell them apart.

## Order and why

1. **Beads triage (green).** Disposition all 106 open and in_progress issues:
   delivered, superseded, relocated to a named home, or abandoned. Produce the
   table. Nothing is deleted before it exists.
2. **Beads removal (orange).** Delete the store and hooks; remove the R8 rule and
   beads reader from `cmd/doccheck` with their tests; confirm no dangling
   references in code, workflows, or docs. Source change — full CI.
3. **doccheck signal fix (orange).** Exclude gitignored paths from the corpus
   walk; report warnings and info separately. Show the same run before and after.
   Source change — full CI.
4. **Clear real warnings (green).** Resolve the 31 H-rule findings at source,
   including the `lease` hit on the active effects Definition. Per finding,
   justify whether the document or the detector was wrong. Do not lower the
   number by weakening rules.
5. **Reading-packet reduction (green).** Subtractive with receipts: every removed
   or relocated passage names the document that now carries the claim. Gated by
   the invariant-conservation check — enumerate doctrine invariants and SEM laws
   before and after and show the sets are equal.

Steps 1–4 are mechanical and low-risk. Step 5 is the one that can do damage, and
it runs last, behind a checker that has been made trustworthy by steps 3 and 4.
That ordering is the point: do not prune the instruction packet while the
instrument that verifies it is reporting noise.

## Landing

Mixed. Steps 2 and 3 touch `cmd/doccheck`, which gates docs-only CI, so those
commits take the full Landing Loop. Steps 1, 4, and 5 are docs-only and take the
Docs Truth Check path. Do not force a deploy path for docs-only commits, and do
not land a `cmd/doccheck` change on the docs-only path.

## Stopping condition

The effects mission can start. Concretely: one work owner, zero real warnings
against the tracked corpus, a measurably smaller packet, and an invariant set
identical to the one this mission inherited.
