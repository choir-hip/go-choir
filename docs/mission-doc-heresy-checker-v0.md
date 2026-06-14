# Mission - Doc/Heresy Checker v0 - Reviewed Draft

Status: reviewed draft, not yet implemented.

Source: imported first draft from an out-of-context agent on 2026-06-13, then
edited against the current repository state.

Related mission: `docs/mission-doc-truth-drift-ci-context-packet-v0.md`.
This document is the narrower checker spec inside that broader docs-truth
mission. The broader mission also owns the generated context packet and docs
drift updater strategy.

## One Sentence

A single fast Go checker should read the repo's Markdown corpus, classify docs
through a small external authority manifest, detect stale current-state claims,
orphaned non-evidence docs, and known doctrine heresies, then emit human and
machine reports. v0 warns only; it never fails CI.

It is one checker with two rule families, not two tools:

- **Structural family**: doc role, claim scope, authority, freshness, and
  reachability.
- **Consistency family**: overclaim, retired vocabulary, VText agency collapse,
  and known current-vs-target drift.

The structural family is what makes the consistency family cheap. Once docs are
typed, retired-vocabulary scans stop drowning in false positives from evidence
and successor docs.

## Codex Review Summary

The imported draft has the right organ shape: one small checker, warn-only v0,
typed reports, discovery not repair, and no model-authored rewrites. The main
change required by repo evidence is that v0 should **not** require YAML
frontmatter on every doc.

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
    doc_role: doctrine
    claim_scope: current
    authority: apex
    lifecycle: living
    root: true
    refresh_triggers:
      - doctrine_change
      - protected_surface_change

  - path: README.md
    doc_role: orientation
    claim_scope: mixed
    authority: support
    lifecycle: living
    root: true
    refresh_triggers:
      - cmd_service_topology_change
      - app_registry_change
      - doctrine_change

  - path: docs/current-architecture.md
    doc_role: current_state
    claim_scope: current
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

### 2. Replace Four Kinds With Repo-Aligned Roles

The draft's four kinds are too narrow for Choir's current docs.

Use roles that match `docs/README.md` and the doctrine upgrade:

- `doctrine`: apex or domain doctrine.
- `operating_contract`: repo agent/workflow rules.
- `orientation`: high-read human/agent entrypoint.
- `current_state`: descriptive of code/staging/current reality.
- `target_architecture`: prescriptive or intended architecture.
- `mission`: active or resumable paradoc.
- `evidence`: historical proof/review/report/ledger.
- `generated_context`: generated packet or projection.
- `successor`: named future cleanup or scoped obligation.

Use a separate `claim_scope`:

- `current`
- `target`
- `historical`
- `mixed`

This avoids the main ambiguity: a doc can be a mission with both current facts
and target architecture. A single `kind` cannot carry that safely.

### 3. Reframe Agent-Read-Only Intent

The imported draft says:

> intent docs are agent-read-only.

The underlying warning is right, but the wording is too absolute for this repo.
Agents routinely write mission paradocs, target architecture drafts, and
successor docs under explicit owner/mission authority.

Correct rule:

> Automated docs updaters must not rewrite target/intent docs to match current
> code. Agents may edit target/intent docs only under explicit mission authority,
> preserving the current-vs-target distinction.

The failure mode is not "agent edits intent." The failure mode is "agent tries
to converge open-loop target architecture to current implementation and erases
the work queue."

### 4. Split Authority Roots From Reachability Roots

The draft starts reachability from `root: true` docs, seeded by
`docs/choir-doctrine.md`.

That needs two root sets:

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

- unreachable living/current docs: loud warning;
- unreachable active missions: warning;
- unreachable evidence docs: info only;
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

Bootstrap only high-read current-state docs first:

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

The checker should first report "missing witness for living current_state doc"
rather than requiring every doc to have one.

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

Warn when a high-read or living doc has no manifest entry.

Report inferred defaults for unlisted docs:

- `docs/mission-*.md`: `doc_role: mission`, `claim_scope: mixed`.
- `docs/*.ledger.md`: `doc_role: evidence`, `claim_scope: historical`.
- dated review/report docs: likely `evidence`.
- `README.md`, `docs/README.md`, `AGENTS.md`: must be explicit.

Do not require YAML frontmatter in v0.

#### R2 - Witness Liveness

For manifest entries with `witnesses`, resolve path/glob patterns.

Warn when:

- witness glob matches nothing;
- a current-state doc's known witness changed after the doc, if using file
  mtimes or a cache;
- service/app/prompt inventories contradict a current-state claim.

The cache is an optimization, not truth.

#### R3 - Reachability

Build a doc graph using:

- manifest roots;
- Markdown links;
- bare `.md` path/filename mentions;
- `supersedes` / `superseded_by`;
- generated context-packet source declarations once those exist.

Report unreachable docs by lifecycle and role. Do not fail.

#### H1 - Retired Vocabulary In Current Claims

Scan detector terms against doc role/scope.

Warn when retired vocabulary appears in:

- `current_state`;
- `orientation`;
- `doctrine`;
- `operating_contract`;
- `generated_context`;

unless the occurrence is explicitly marked historical, deprecated, detector
vocabulary, or successor-scope.

Evidence docs and successor docs may retain the vocabulary.

#### H2 - Overclaim

Warn on universal correctness/safety claims when not scoped to evidence.

Examples:

- bad: "this is safe";
- better: "passed contract X under scope Y";
- better: "verified by test Z for behavior W."

This rule must have typed allowlists because doctrine docs discuss the concept
of overclaiming.

#### H3 - VText Agency Collapse

Warn when a non-evidence doc frames VText as a fixed workflow or single-pass
pipeline without an explicit conjecture edge.

Seed from:

- `docs/vtext-agentic-invariants-2026-06-13.md`;
- H010/H024/H026 in `docs/choir-doctrine.md`;
- `internal/runtime/prompt_defaults/*` only when prompt-default policy is in
  the checked scope.

#### H4 - Current/Target Collapse

Warn when a current-state or orientation doc states target architecture as
already implemented without a current-evidence label.

Known seed:

- README service topology: "five Go services" is stale relative to
  `docs/current-architecture.md` and `.github/workflows/ci.yml`.
- `platformd -> corpusd` is target rename, not current code.
- `sourcecycled` is current daemon name; Source Cycle/source service is target
  or product vocabulary depending on context.

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
- No blocking CI.
- No mass frontmatter migration.
- No vector index.
- No generated context packet. That belongs to the broader
  `mission-doc-truth-drift-ci-context-packet-v0.md` after the checker/manifest
  can supply source truth.

## Suggested Goal String

```text
/goal Use Parallax on docs/mission-doc-heresy-checker-v0.md. Treat it as the reviewed checker spec inside the broader docs truth mission. Resume from the Parallax State and append moves to docs/mission-doc-heresy-checker-v0.ledger.md. Current status is open_handoff: the imported out-of-context draft has been copied into the repo and edited against real corpus evidence, but no checker code exists. Preserve Choir Doctrine as apex, do not require mass YAML frontmatter in v0, use an external doc authority manifest first, keep v0 warn-only and exit 0, separate discovered/introduced/repaired heresy accounting, and do not weaken docs-only CI filters without explicit operating-contract reconciliation. First produce docs/doc-authority-manifest.yaml for the living high-read set, then implement cmd/doccheck or scripts/doccheck to emit doccheck-report.md and doccheck.json from manifest, link graph, witness liveness, and seeded heresy terms. Settlement requires a report-only checker run over the repo, measured runtime under 10 seconds, reviewed baseline warnings, and no behavior/runtime code changes.
```

## Parallax State

status: open_handoff

mission conjecture: if a small report-only Go checker can classify high-read
docs through an external manifest, detect stale current-state claims, and scope
heresy terms by doc role, then Choir can start maintaining documentation truth
without turning docs upkeep into a brittle all-doc rewrite or an untrusted
model-authored cleanup loop.

deeper goal (G): make docs truth maintainable by facts. Future agents should
know which docs are current, target, evidence, mission, or generated context,
and the system should surface stale claims and heresy circulation before they
mislead implementation work.

witness/spec (A/S): reviewed spec for `cmd/doccheck` / `scripts/doccheck`,
external `docs/doc-authority-manifest.yaml`, report-only outputs
`doccheck-report.md` and `doccheck.json`, and seeded detector terms from
existing doctrine.

invariants / qualities / domain ramp (I/Q/D): v0 warns only and exits 0; no
mass frontmatter migration; no auto-fixing; no model judgment in the trusted
path; no weakening docs-only CI filters without explicit approval; current and
target claims stay separate; evidence docs may retain old vocabulary; discovery
does not count as repair. Start with high-read docs and known H027-H029/VText
detectors before broadening.

variant (ranking function) V: missing manifest + missing checker command +
missing report outputs + missing seeded detector config + missing measured
runtime + unreviewed baseline warnings + unresolved CI/manual workflow decision.

budget: one review/spec iteration now; later implementation should be a small
process/test mission, not runtime behavior work.

authority / bounds: docs and future checker/process code only. Do not touch
runtime behavior. Do not add blocking CI until warn-only reports and operating
contract implications are reviewed.

mutation class / protected surfaces: yellow. Protected surfaces include docs
authority hierarchy, CI path filters, doctrine detector semantics, generated
reports, and future-agent context.

evidence packet: repo probe counts, frontmatter reality, link graph reality,
service topology evidence, CI path-filter evidence, edited spec, future
doccheck runtime measurement, generated reports, residual false-positive risks.

heresy delta:

- discovered: imported draft assumed mass frontmatter feasibility; current repo
  disproves that assumption. README service-topology drift remains a discovered
  doc heresy.
- introduced: none allowed.
- repaired: only the spec is repaired here. No docs drift or code behavior is
  repaired until checker/manual doc updates land.

position / live conjectures / open edges:

- C1 active: external manifest first is cheaper and safer than per-file
  frontmatter.
- C2 active: doc roles plus claim scope are better than the imported draft's
  four-kind type system.
- C3 active: reachability must parse Markdown links and bare doc filename
  mentions.
- C4 active: v0 report-only local/manual workflow is compatible with AGENTS;
  blocking docs-only CI is a later policy decision.
- Edge: exact living-doc manifest entries still require owner/repo review.
- Edge: overclaim and VText agency scans need allowlists to avoid punishing
  doctrine docs that discuss the forbidden patterns.

next move: send this reviewed draft back to the originating agent for critique.
After iteration, decide whether to merge it into
`mission-doc-truth-drift-ci-context-packet-v0.md` or keep it as the narrower
implementation paradoc for the checker.

ledger file: `docs/mission-doc-heresy-checker-v0.ledger.md`.

version / lineage: imported from out-of-context draft and edited against repo
evidence on 2026-06-13.

learning state: retained here for the iteration loop. Promote into the broader
docs-truth mission only after the second-pass review settles the manifest and
frontmatter decisions.

settlement: not claimed. Settlement requires a reviewed spec accepted for
implementation, then a report-only checker run over the repo with runtime under
10 seconds and baseline warnings recorded.
