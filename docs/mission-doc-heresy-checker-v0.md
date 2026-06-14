# Mission - Doc/Heresy Checker v0 - Reviewed Draft

Status: reviewed draft, not yet implemented.

Source: imported first draft from an out-of-context agent on 2026-06-13, then
edited against the current repository state.

Related mission: `docs/mission-doc-truth-drift-ci-context-packet-v0.md`.
This document is the narrower checker spec inside that broader docs-truth
mission. The broader mission also owns the generated context packet and docs
drift updater strategy.

## One Sentence

A single fast Go checker should read the repo's Markdown corpus, classify each
doc with three kernel fields, detect self-model drift, then emit human and
machine reports. v0 warns only; it never fails CI.

It is one checker with two rule families, not two tools:

- **Structural family**: claim scope, evidence exemption, roots, freshness, and
  reachability.
- **Consistency family**: overclaim, retired vocabulary, VText agency collapse,
  and known current-vs-target drift.

The structural family is what makes the consistency family cheap. Once docs
have `claim_scope`, `is_evidence`, and `is_root`, retired-vocabulary scans stop
drowning in false positives from evidence and target docs. Descriptive roles may
exist in the manifest for humans, but the checker kernel does not branch on
them.

This checker is not a docs linter. It is a type-checker for Choir's self-model.
The docs graph is the system's representation of its own development state:
what code is current, which conjectures are open or promoted, which missions are
settled or pending, and what order the work DAG implies. Every rule asks one
question: has the self-model drifted from what it represents? Compression of the
self-model, meaning what ontology the project should keep, remains the owner's
job, not the checker's.

## Codex Review Summary

The imported draft has the right organ shape: one small checker, warn-only v0,
typed reports, discovery not repair, and no model-authored rewrites. The first
change required by repo evidence is that v0 should **not** require YAML
frontmatter on every doc. The second simplification is that doc roles must not
gate warnings. The checker branches only on `claim_scope`, `is_root`, and
`is_evidence`.

Actual repo facts from 2026-06-13:

- Markdown files found: 193 across the repo; 174 directly under `docs/`.
- Mission docs under `docs/`: 71.
- Ledger docs under `docs/`: 8.
- YAML frontmatter exists in only 3 Markdown files, all skill files:
  `skills/mission-gradient/SKILL.md`,
  `skills/cognitive-transform-portfolio/SKILL.md`, and
  `skills/parallax/SKILL.md`.
- Frontmatter keys in the wild are skill metadata keys: `name`, `description`,
  `version`, `metadata`, `author`, `license`.
- Markdown links and bare filename references both matter:
  71 files contain Markdown links, 137 contain bare `.md` mentions,
  with 463 Markdown links and 1396 bare Markdown filename mentions in the
  current corpus.
- Current Go command/service directories include `auth`, `gateway`, `maild`,
  `maildctl`, `platformd`, `proxy`, `purge-vtext-owner-aliases`, `sandbox`,
  `sourcecycled`, `vmctl`, and `zot`.
- CI and FlakeHub workflows intentionally ignore `docs/**` and top-level
  `*.md`. `AGENTS.md` says not to weaken those filters just to force docs-only
  CI.

Therefore v0 must start with an **external manifest plus inferred defaults**,
not a mass frontmatter migration.

## Design Corrections To The Imported Draft

### 1. Use An External Manifest First

The imported draft proposed mandatory frontmatter:

```yaml
schema_version: 0
kind: conformance | intent | evidence | successor
witness: ...
```

That is too disruptive for this repo today. The live corpus has almost no doc
frontmatter, and adding frontmatter to 180+ docs would create a huge metadata
churn mission before the checker proves value.

v0 should instead use:

```text
docs/doc-authority-manifest.yaml
```

The manifest classifies a small living set first. Unlisted docs get inferred
defaults and warnings, not errors.

Suggested v0 manifest fields:

```yaml
schema_version: 0
documents:
  - path: docs/choir-doctrine.md
    claim_scope: current
    is_root: [authority, entry]
    is_evidence: false
    annotations:
      doc_role: doctrine
      authority: apex
      lifecycle: living
    refresh_triggers:
      - doctrine_change
      - protected_surface_change

  - path: README.md
    claim_scope: mixed
    is_root: entry
    is_evidence: false
    annotations:
      doc_role: orientation
      authority: support
      lifecycle: living
    refresh_triggers:
      - cmd_service_topology_change
      - app_registry_change
      - doctrine_change

  - path: docs/current-architecture.md
    claim_scope: current
    is_root: entry
    is_evidence: false
    annotations:
      doc_role: current_state
      authority: support
      lifecycle: living
    witnesses:
      - cmd/**
      - internal/**
      - .github/workflows/**
    refresh_triggers:
      - cmd_service_topology_change
      - runtime_architecture_change
```

Frontmatter can come later if the manifest proves useful. The manifest keeps
meaning in one inspectable place and avoids editing every historical file.
Only `claim_scope`, `is_root`, and `is_evidence` gate warnings. `annotations`
and `refresh_triggers` are report metadata. `witnesses` is evidence payload:
the witness-liveness rule decides whether to inspect it from `claim_scope`, not
from the payload's mere presence.

### 2. Demote Roles From Kernel To Annotation

The draft's four kinds are too narrow for Choir's current docs, and the first
review pass overcorrected by introducing nine `doc_role` values. Those roles
are useful for humans and generated indexes, but they must not be checker
branches.

Allowed kernel fields:

- `claim_scope`: `current`, `target`, `historical`, or `mixed`.
- `is_root`: `false`, `entry`, `authority`, or both.
- `is_evidence`: boolean.

Descriptive annotations may include:

- `doctrine`: apex or domain doctrine.
- `operating_contract`: repo agent/workflow rules.
- `orientation`: high-read human/agent entrypoint.
- `current_state`: descriptive of code/staging/current reality.
- `target_architecture`: prescriptive or intended architecture.
- `mission`: active or resumable paradoc.
- `evidence`: historical proof/review/report/ledger.
- `generated_context`: generated packet or projection.
- `successor`: named future cleanup or scoped obligation.

The checker may print these annotations in reports, but no warning may depend
on them. If a future rule genuinely needs a role to decide whether to warn, that
rule must name itself as a kernel exception. v0 has zero role-gated exceptions.

Decision-cell count:

- before simplification: 9 roles x 4 claim scopes = 36 role/scope cells;
- after simplification: 4 claim-scope cells, with `is_evidence` as an exemption
  and `is_root` as reachability seed, not a role matrix.

Actual Delta V from this collapse: doc-role branches 9 -> 0; decision cells
36 -> 4; non-kernel manifest fields that gate checks 3+ -> 0.

### 3. Bind Target Protection To Reconciler Identity

The imported draft says:

> intent docs are agent-read-only.

The underlying warning is right, but the wording is too absolute for this repo.
Agents routinely write mission paradocs, target architecture drafts, and
successor docs under explicit owner/mission authority.

Correct rule:

> The future automated docs reconciler is mechanically forbidden from writing
> `claim_scope: target` docs.

This is an actor-identity rule, not a semantic-intent rule. The checker cannot
know why an agent edited a doc, but the future reconciler can know which actor
it is and what document scope it is trying to write. Mission-authority agents
editing target docs are outside this checker rule. The failure mode is not
"agent edits target." The failure mode is "the 24/7 docs-convergence actor
rewrites open-loop target architecture to match current code and erases the work
queue."

The reconciler does not exist yet. The rule is specced now so the guard exists
before that actor ships.

### 4. Split Authority Roots From Reachability Roots

The draft starts reachability from `root: true` docs, seeded by
`docs/choir-doctrine.md`.

That needs two root kinds inside the single `is_root` kernel field:

- **authority roots**: determine doctrine inheritance.
- **entry roots**: determine whether a doc is discoverable by normal readers.

Recommended v0 authority roots:

- `docs/choir-doctrine.md`
- `AGENTS.md`

Recommended v0 entry roots:

- `README.md`
- `docs/README.md`
- `docs/choir-doctrine.md`
- `AGENTS.md`
- `docs/current-architecture.md`
- `docs/platform-os-app-state.md`
- `docs/mission-portfolio-2026-06-11.md`

The orphan warning should be softer than the draft says. Many historical docs
are intentionally retained. v0 should report:

- unreachable `claim_scope: current` or `mixed` docs: warning;
- unreachable `claim_scope: target` docs: open-work visibility warning;
- unreachable `is_evidence: true` docs: info only;
- unreachable unclassified docs: collection candidates.

### 5. Link Extraction Must Include Bare Filenames

The draft asks whether a Markdown link parser is enough. It is not.

Current corpus evidence:

- Markdown links: 463.
- Bare `.md` mentions: 1396.

R3 needs both:

- Markdown links like `[docs/choir-doctrine.md](docs/choir-doctrine.md)`;
- relative links like `[source](./source-external-data-publication.md)`;
- bare paths like `docs/mission-portfolio-2026-06-11.md`;
- bare filenames resolved relative to the source doc and then `docs/`.

Do not treat every bare filename as an authoritative edge. The report should
mark edge source as `markdown_link`, `bare_path`, `manifest`, or `supersedes`.
Reachability can include all edge kinds in report-only v0; enforcement can later
prefer stronger edge kinds.

### 6. Witnesses Must Bootstrap Slowly

A mandatory `witness:` for every conformance/current-state doc is not feasible
without a large hand-authored pass.

Bootstrap only explicit manifest entries first:

- `README.md`
- `docs/current-architecture.md`
- `docs/platform-os-app-state.md`
- `docs/README.md`
- `docs/runtime-invariants.md`
- `docs/source-external-data-publication.md`

For these, use broad witness families:

- service topology: `cmd/*`, `.github/scripts/deploy-impact-classify`,
  `.github/workflows/ci.yml`, service pointer scripts.
- app catalog: `frontend/src/lib/apps/registry.ts`,
  `frontend/src/lib/Desktop.svelte`, app components.
- prompt/default policy: `internal/runtime/prompt_defaults/*`.
- source/Web Lens: `frontend/src/lib/BrowserApp.svelte`,
  `internal/store/browser.go`, `internal/types/browser.go`,
  source/content contracts.

The checker should first report "missing witness for current or mixed claim"
rather than requiring every doc to have one. That warning is advisory and should
not introduce another kernel field; it is derived from `claim_scope: current` or
`mixed` plus the absence of witness payload.

### 7. Seed Heresy Terms From Existing Doctrine

The draft's retired vocabulary table should be generated or copied from
`docs/heresy-detectors.md` and `docs/choir-doctrine.md`, not hand-paraphrased.

Seed families:

- H027 Trace app residue:
  `Trace app`, `Trace UI`, `Open Trace`, `appId: "trace"`, `data-trace-app`.
- H028 raw Terminal app residue:
  `Terminal app`, `raw Terminal`, `manual terminal`, `/api/terminal/ws`,
  `appId: "terminal"`.
- H029 Browser source-gathering residue:
  `Browser app`, `BrowserApp`, `browser_sessions`, `AppHint: "browser"`,
  `open_surface: "browser"`.
- VText/prompt forcing:
  terms from H010, H024, H026 in `docs/heresy-detectors.md`.
- continuation residue:
  `RunContinuation`, `run_continuations`, `/api/continuations`,
  `continuation-level`.

The scan should not target zero raw hits. It should target zero unclassified
current-claim hits.

### 8. CI Integration Must Respect Existing Path Filters

Current CI:

- `.github/workflows/ci.yml` ignores `docs/**` and top-level `*.md` on push and
  pull request.
- `.github/workflows/flakehub-publish-rolling.yml` also ignores docs/top-level
  markdown.
- `AGENTS.md` explicitly says docs-only commits are different and path filters
  should not be weakened merely to force docs-only CI.

Therefore v0 should not add a blocking docs-only CI job without an explicit
operating-contract change.

Acceptable first integration:

- local script: `scripts/doccheck` or `go run ./cmd/doccheck`;
- manual workflow dispatch;
- scheduled report-only workflow;
- PR check only when non-doc files already trigger CI;
- artifact upload with exit 0.

The checker itself should stay under 10 seconds. A Go implementation parsing
around 200 Markdown files should likely run in under one second; the 10-second
ceiling is still a useful hard limit.

## Revised v0 Spec

### Artifact

One Go command:

```text
cmd/doccheck
```

One optional wrapper:

```text
scripts/doccheck
```

Inputs:

- Markdown files under repo, excluding `.git`, `node_modules`, build artifacts,
  and generated reports unless explicitly included.
- `docs/doc-authority-manifest.yaml`.
- `docs/heresy-detectors.md` or a structured detector manifest derived from it.

Outputs:

- `doccheck-report.md`: human/ear-first report.
- `doccheck.json`: machine report.

Exit code:

- v0 always exits 0.

### Rule Families

#### R1 - Manifest Classification

Reads: `is_root` for manifest roots and `claim_scope` only when present. Does
not read `doc_role`.

Warn when a root, high-read entrypoint, or explicitly checked doc has no
manifest entry. Roots are configured by `is_root`; high-read entrypoints are
the fixed seed list for v0 (`README.md`, `docs/README.md`, `AGENTS.md`,
`docs/choir-doctrine.md`, `docs/current-architecture.md`,
`docs/platform-os-app-state.md`, `docs/mission-portfolio-2026-06-11.md`).

Report inferred defaults for unlisted docs:

- `docs/mission-*.md`: `claim_scope: mixed`.
- `docs/*.ledger.md`: `claim_scope: historical`, `is_evidence: true`.
- dated review/report docs: likely `claim_scope: historical`,
  `is_evidence: true`.
- `README.md`, `docs/README.md`, `AGENTS.md`: must be explicit.

Do not require YAML frontmatter in v0.

#### R2 - Witness Liveness

Reads: `claim_scope`. Does not read `doc_role`.

For `claim_scope: current` or `mixed` docs, check witness payload. Missing
witness payload is advisory. When payload exists, resolve its path/glob
patterns.

Warn when:

- witness glob matches nothing;
- a `claim_scope: current` or `mixed` doc's known witness changed after the doc,
  if using file mtimes or a cache;
- service/app/prompt inventories contradict a `claim_scope: current` or `mixed`
  claim.

The cache is an optimization, not truth.

#### R3 - Reachability

Reads: `claim_scope`, `is_root`, and `is_evidence`. Does not read `doc_role`.

Build a doc graph using:

- manifest roots;
- Markdown links;
- bare `.md` path/filename mentions;
- `supersedes` / `superseded_by`;
- generated context-packet source declarations once those exist.

Report unreachable docs by scope/evidence class. Do not fail:

- `is_evidence: true`: info only;
- `claim_scope: current` or `mixed`: warning;
- `claim_scope: target`: open-work visibility warning;
- unclassified: collection candidate.

#### R4 - Reconciler Target-Write Guard

Reads: `claim_scope` plus actor identity from the future reconciler write
attempt. Does not read `doc_role`.

When the automated docs reconciler actor exists, it must not write a
`claim_scope: target` doc. This is not an intent check. It is a durable actor
identity rule:

```text
if actor_id == docs_reconciler && target_doc.claim_scope == target:
    reject write
```

Mission-authority agents are outside this rule. The acceptance test for the
checker/reconciler boundary must include a deliberately planted
target-doc-converging edit and prove the guard catches it before any automatic
reconciler ships.

#### H1 - Retired Vocabulary In Current Claims

Reads: `claim_scope` and `is_evidence`. Does not read `doc_role`.

Scan detector terms against claim scope. Warn when retired vocabulary appears in
a doc with:

- `is_evidence: false`; and
- `claim_scope: current` or `mixed`;

unless the occurrence is explicitly marked historical, deprecated, detector
vocabulary, or successor-scope.

Evidence docs and target docs may retain the vocabulary when the occurrence is
part of evidence, a named successor, or an open-work description.

#### H2 - Overclaim

Reads: `claim_scope` and `is_evidence`. Does not read `doc_role`.

Warn on universal correctness/safety claims when `is_evidence: false` and the
claim is not scoped to evidence.

Examples:

- bad: "this is safe";
- better: "passed contract X under scope Y";
- better: "verified by test Z for behavior W."

This rule must have typed allowlists because doctrine docs discuss the concept
of overclaiming.

#### H3 - VText Agency Collapse

Reads: `claim_scope` and `is_evidence`. Does not read `doc_role`.

Warn when an `is_evidence: false` doc frames VText as a fixed workflow or
single-pass pipeline without an explicit conjecture edge. The firing condition
must be durable text plus scope/evidence classification, not guessed author
intent.

Seed from:

- `docs/vtext-agentic-invariants-2026-06-13.md`;
- H010/H024/H026 in `docs/choir-doctrine.md`;
- `internal/runtime/prompt_defaults/*` only when prompt-default policy is in
  the checked scope.

#### H4 - Current/Target Collapse

Reads: `claim_scope`. Does not read `doc_role`.

Warn when a `claim_scope: current` or `mixed` doc states target architecture as
already implemented without a current-evidence label.

Known seed:

- README service topology: "five Go services" is stale relative to
  `docs/current-architecture.md` and `.github/workflows/ci.yml`.
- `platformd -> corpusd` is target rename, not current code.
- `sourcecycled` is current daemon name; Source Cycle/source service is target
  or product vocabulary depending on context.

### Kernel Size And Ordering

The check logic should fit in roughly one sitting: about 120-180 lines of Go for
the kernel predicates before parsing/report formatting. If the kernel needs
more, the grammar is probably regrowing.

Before simplification, the reviewed draft implied a 9-role x 4-scope decision
matrix and allowed role, authority, lifecycle, and updater intent to influence
warnings. After simplification:

- doc-role branches: 0;
- decision cells: 4 claim-scope cells;
- semantic-intention firing conditions: 0;
- manifest fields that gate checks outside `claim_scope`, `is_root`, and
  `is_evidence`: 0.

The checker and reconciler must not ship in the same pass. The checker is the
safe mechanical half. The reconciler is the open-loop half that can diverge.
The checker ships first, and its acceptance test must catch a deliberately
planted target-doc-converging edit by the reconciler actor before the reconciler
is allowed to write docs.

### Report Format

`doccheck-report.md` should be readable aloud:

1. one-line verdict;
2. counts: docs scanned, manifest entries, inferred docs, warnings by rule;
3. heresy accounting:
   - discovered;
   - introduced;
   - repaired;
4. top risks in prose;
5. per-rule warnings:
   `path:line RULE message [fix hint]`;
6. collection candidates;
7. next suggested manifest entries.

Avoid tables in the human report. Tables can exist in `doccheck.json`.

### What v0 Does Not Do

- No auto-editing docs.
- No model judgment in the trusted checker path.
- No semantic intent inference.
- No blocking CI.
- No mass frontmatter migration.
- No vector index.
- No reconciler implementation.
- No generated context packet. That belongs to the broader
  `mission-doc-truth-drift-ci-context-packet-v0.md` after the checker/manifest
  can supply source truth.

## Suggested Goal String

```text
/goal Use Parallax on docs/mission-doc-heresy-checker-v0.md. Treat it as the simplified checker spec inside the broader docs truth mission. Resume from the Parallax State and append moves to docs/mission-doc-heresy-checker-v0.ledger.md. Current status is open_handoff: the spec has been simplified so checker logic branches only on claim_scope, is_root, and is_evidence; doc roles are manifest annotations only; the future docs reconciler is forbidden by actor identity from writing claim_scope: target docs. Preserve Choir Doctrine as apex, do not require mass YAML frontmatter in v0, use an external doc authority manifest first, keep v0 warn-only and exit 0, separate discovered/introduced/repaired heresy accounting, and do not weaken docs-only CI filters without explicit operating-contract reconciliation. First produce docs/doc-authority-manifest.yaml with kernel fields for the living high-read set, then implement cmd/doccheck or scripts/doccheck to emit doccheck-report.md and doccheck.json from manifest, link graph, witness liveness, and seeded heresy terms. The checker must ship before any reconciler, and its acceptance must catch a deliberately planted target-doc-converging edit by the reconciler actor. Settlement requires a report-only checker run over the repo, measured runtime under 10 seconds, reviewed baseline warnings, and no behavior/runtime code changes.
```

## Parallax State

status: open_handoff

mission conjecture: if a small report-only Go checker branches only on
`claim_scope`, `is_root`, and `is_evidence`, while leaving doc roles as human
annotations, then Choir can type-check its self-model without growing a brittle
docs ontology or an untrusted model-authored cleanup loop.

deeper goal (G): make docs truth maintainable by facts. Future agents should
know whether a claim is current, target, historical/evidence, or mixed, and the
system should surface stale claims and heresy circulation before they mislead
implementation work. Ontology compression remains the owner's job.

witness/spec (A/S): simplified spec for `cmd/doccheck` / `scripts/doccheck`,
external `docs/doc-authority-manifest.yaml` with kernel fields
`claim_scope`, `is_root`, and `is_evidence`, report-only outputs
`doccheck-report.md` and `doccheck.json`, seeded detector terms from existing
doctrine, and a future reconciler actor-identity guard for target docs.

invariants / qualities / domain ramp (I/Q/D): v0 warns only and exits 0; no
mass frontmatter migration; no auto-fixing; no model judgment in the trusted
path; no semantic intent inference; no weakening docs-only CI filters without
explicit approval; current and target claims stay separate; evidence docs may
retain old vocabulary; discovery does not count as repair; checker ships before
reconciler. Start with high-read docs and known H027-H029/VText detectors before
broadening.

variant (ranking function) V for this simplification pass: doc-role branches
9 -> 0; role/scope decision cells 36 -> 4; semantic-intention firing conditions
1 -> 0; non-kernel manifest fields that gate checks 3+ -> 0. The implementation
mission variant remains: missing manifest + missing checker command + missing
report outputs + missing seeded detector config + missing measured runtime +
unreviewed baseline warnings + unresolved CI/manual workflow decision.

budget: one review/spec iteration now; later implementation should be a small
process/test mission, not runtime behavior work.

authority / bounds: docs and future checker/process code only. Do not touch
runtime behavior. Do not add blocking CI until warn-only reports and operating
contract implications are reviewed.

mutation class / protected surfaces: yellow. Protected surfaces include docs
authority hierarchy, CI path filters, doctrine detector semantics, generated
reports, and future-agent context.

evidence packet: repo probe counts, frontmatter reality, link graph reality,
service topology evidence, CI path-filter evidence, simplified spec, rule audit
showing each rule's kernel fields, future doccheck runtime measurement,
generated reports, residual false-positive risks.

heresy delta:

- discovered: imported draft assumed mass frontmatter feasibility; current repo
  disproves that assumption. README service-topology drift remains a discovered
  doc heresy.
- introduced: none allowed.
- repaired: spec grammar simplified: doc roles are no longer warning gates,
  target-doc protection is actor-identity based, and semantic-intent firing is
  removed. No docs drift or code behavior is repaired until checker/manual doc
  updates land.

position / live conjectures / open edges:

- C1 active: external manifest first is cheaper and safer than per-file
  frontmatter.
- C2 supported for spec scope: claim scope plus `is_root` and `is_evidence` are
  the checker kernel; doc roles are annotation only.
- C3 active: reachability must parse Markdown links and bare doc filename
  mentions.
- C4 active: v0 report-only local/manual workflow is compatible with AGENTS;
  blocking docs-only CI is a later policy decision.
- C5 active: the automated reconciler must be forbidden by actor identity from
  writing `claim_scope: target` docs before it ships.
- Edge: exact living-doc manifest entries still require owner/repo review.
- Edge: overclaim and VText agency scans need allowlists to avoid punishing
  doctrine docs that discuss the forbidden patterns.

next move: send the simplified draft back to the originating agent for critique.
After iteration, decide whether to keep this as the narrower implementation
paradoc for the checker or merge its settled kernel into the broader docs-truth
mission.

ledger file: `docs/mission-doc-heresy-checker-v0.ledger.md`.

version / lineage: imported from out-of-context draft and edited against repo
evidence on 2026-06-13.

learning state: retained here for the iteration loop. Promote into the broader
docs-truth mission only after the second-pass review settles the manifest and
frontmatter decisions.

settlement: open_handoff for this simplification pass. Independent prover
review found grammar-regrowth risks in witness payload wording, R3's read audit,
and non-kernel manifest metadata; the follow-up patch removed those risks.
Implementation settlement still requires a reviewed spec accepted for
implementation, then a report-only checker run over the repo with runtime under
10 seconds, baseline warnings recorded, and a planted reconciler
target-doc-convergence edit caught.
