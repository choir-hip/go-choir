# Mission - Doc Truth, Drift CI, And Context Packet - v0

Status: successor paradoc stub.

Source: `README.md`, `docs/README.md`, `docs/current-architecture.md`,
`docs/choir-doctrine.md`, `docs/heresy-detectors.md`,
`docs/mission-heresy-detectors-ci-v0.md`,
`docs/mission-doc-heresy-checker-v0.md`, and the 2026-06-13 observation that the
README still claims "five Go services" while current architecture already names
`auth`, `proxy`, `gateway`, `vmctl`, `platformd`, `maild`, `sourcecycled`, plus
per-computer runtimes.

## Problem Record

The doctrine sweep repaired framing, but it did not give Choir a durable docs
truth-maintenance system. High-read docs can still drift behind code and
architecture. The README is the live example: it presents a stale five-service
topology and does not clearly distinguish current service names from target or
planned renames such as `platformd -> corpusd` and the `sourcecycled` /
Source Cycle / source-service boundary.

Manual sweeps are not enough. A human can prompt agents to "update all docs,"
but the system lacks:

- a deterministic current-vs-target document contract;
- a machine-readable doc authority manifest;
- executable heresy detectors with allowlists;
- service/app/prompt/source inventory checks against code;
- a generated context packet that gives future agents a compact, hierarchical
  understanding of the project;
- a cheap model-assisted docs reviewer that can find likely drift without being
  allowed to rewrite doctrine by itself.

The target is not a magic docs bot that edits everything. The first target is a
truth-maintenance pipeline: deterministic facts first, advisory model review
second, proposed updates third.

## Context Packet Conjecture

Choir needs one canonical generated context packet for agents and LLM tools.
This packet is the cached understanding of the project, compiled from higher
authority docs and code-derived inventories. It should be short enough to read
often, comprehensive enough to orient a new agent, and prefix-stable: if a model
only reads the first part, it still gets the right ontology and current-vs-target
boundaries.

Canonical generated artifact:

```text
docs/choir-context-packet.md
```

Optional projection:

```text
llms.txt
```

`docs/choir-context-packet.md` is the canonical artifact. If `llms.txt` exists,
it should be a thin generated projection or pointer, not a second source of
truth.

The packet should be generated, not hand-authored. Its source set should be
declared in a manifest and include hashes/commit refs so drift is visible. The
packet never overrides its sources. It is a compiled index and orientation
layer.

Truncation-aware hierarchy:

1. **Kernel**: one-screen thesis, root ontology, doctrine-of-doctrine,
   current-vs-target warning.
2. **Authority Map**: doctrine, AGENTS, current architecture, platform app
   state, active portfolio, domain invariants, evidence docs.
3. **Current State Snapshot**: current services, per-computer runtime, app
   surfaces, source/Web Lens, VText, Trace evidence, Super Console, promotion.
4. **Target Architecture Delta**: durable actors, trajectories/work items,
   continuation deletion, route promotion, CorpusD rename, source intake.
5. **Active Missions**: architecture spine, side missions, deferred product
   wedges.
6. **Heresy Inventory**: detector families, open residue, discovered vs
   introduced vs repaired.
7. **Verification And Landing Rules**: staging-first, docs-only caveat,
   evidence classes, problem-documentation-first.
8. **Source Map**: source docs, generated timestamp/commit, known omissions.

Every major claim in the packet should carry a label:

```text
[CURRENT] code/staging/current descriptive doc claim
[TARGET] intended architecture or planned rename
[HISTORICAL] retained evidence, not current instruction
[OPEN] unresolved or successor-scope question
```

This is how the packet stays useful under truncation without flattening current
and ideal architecture into one story.

## Suggested Goal String

```text
/goal Use Parallax on docs/mission-doc-truth-drift-ci-context-packet-v0.md. Treat it as the source paradoc for Doc Truth, Drift CI, and Context Packet. Resume from its Parallax State and append moves to docs/mission-doc-truth-drift-ci-context-packet-v0.ledger.md. Current status is open_handoff: the problem is documented, but no README repair, detector script, context compiler, or CI workflow exists yet. Keep Choir Doctrine as apex, separate current from target architecture at the sentence level, preserve historical evidence, keep discovered/introduced/repaired heresy accounting separate, and do not weaken docs-only CI path filters without explicitly reconciling AGENTS.md. First repair the high-read README/current-doc service-topology drift, then implement the smallest deterministic manifest-driven heresy detector report, then add a report-only context-packet compiler that generates docs/choir-context-packet.md with [CURRENT]/[TARGET]/[HISTORICAL]/[OPEN] labels and source provenance. Do not add model-written auto-updates until deterministic checks and review workflow exist. Settlement requires at least one high-read stale claim repaired, a structured doc authority manifest, executable heresy detector report, generated context packet, and a documented CI/manual-workflow decision.
```

## Parallax State

status: open_handoff

mission conjecture: if Choir installs a deterministic docs truth pipeline,
executable heresy detector CI, and a generated prefix-stable context packet,
then future agents will update docs from facts instead of preserving stale
stories, and high-read docs will stop drifting away from current and target
architecture.

deeper goal (G): make Choir's self-understanding durable. The system should
notice when docs lie, distinguish current reality from target architecture,
feed agents a compact correct context packet, and make docs maintenance a
standing evidence process rather than a heroic manual sweep.

witness/spec (A/S): a staged process/code change set:

- high-read manual repair for `README.md`, `docs/README.md`,
  `docs/current-architecture.md`, and `docs/platform-os-app-state.md` where
  current facts are already known;
- `docs/doc-authority-manifest.yaml` or equivalent, declaring document role,
  authority, claim scope, source dependencies, refresh triggers, and generated
  outputs;
- structured heresy detector manifest plus local/CI script with typed
  allowlists and `discovered` / `introduced` / `repaired` deltas. The narrower
  reviewed checker spec lives in `docs/mission-doc-heresy-checker-v0.md`;
- deterministic docs drift checks for service topology, app registry,
  prompt-default policy files, source/Web Lens terminology, and current-vs-target
  labels;
- context-packet compiler that emits `docs/choir-context-packet.md` from
  declared sources with provenance;
- optional generated `llms.txt` projection that points to or summarizes the
  canonical context packet;
- later cheap model-assisted docs review that emits advisory findings or a
  proposed patch, never silent doctrine edits.

invariants / qualities / domain ramp (I/Q/D):

- Choir Doctrine remains apex. Generated packets and model reviews do not
  override source docs.
- Current-state docs must not claim target architecture as shipped.
- Target docs must not imply current behavior without evidence.
- Historical evidence must stay visible but labeled.
- Detector counts are evidence, not ontology.
- Discovery, introduction, and repair remain separate.
- Cheap LLM review is advisory until deterministic checks and review workflow
  are trusted.
- Do not weaken existing docs-only CI path filters casually. `AGENTS.md` says
  docs-only commits intentionally skip normal CI. If this mission wants a
  blocking docs-only check, it must first update that operating contract with
  explicit approval. Safer first forms are local scripts, scheduled checks,
  manual workflow dispatch, and checks on code-touching PRs.
- Start simple: deterministic inventory/diff checks before model review, model
  review before auto-update, auto-update only as proposed patches.

variant (ranking function) V: stale high-read current-state claims + missing
doc authority manifest + missing executable heresy detector + missing docs drift
inventory checks + missing context packet compiler + missing generated context
packet + missing LLM advisory review + unresolved docs-only CI policy.

budget: one planning paradoc now; next implementation should be split into
small commits: manual high-read repair, deterministic detector script,
deterministic context compiler, then optional CI wiring.

authority / bounds: docs/process/test mission. Behavior code is out of scope
except small scripts/checks and CI configuration. Blocking docs-only CI requires
explicit reconciliation with `AGENTS.md`.

mutation class / protected surfaces: yellow. Protected surfaces include doctrine
hierarchy, CI behavior, docs-only path filters, generated docs, prompt/default
policy files, and future-agent context.

evidence packet: changed docs, detector manifest, generated baseline, sample
accepted discovery, sample rejected introduced heresy, service inventory report,
context packet output with source hashes, CI/manual workflow receipt, residual
false-positive/false-negative risks.

heresy delta:

- discovered: README stale service count; no docs drift oracle; no generated
  project context packet; old detector mission too narrow for the actual docs
  truth problem.
- introduced: none allowed.
- repaired: only count repair when stale high-read docs are corrected, checks
  exist, or generated context packet/CI receipts prove future drift will be
  detected. Naming this mission is discovery, not repair.

position / live conjectures / open edges:

- C1 active: deterministic checks should lead. LLM review without deterministic
  facts will generate plausible prose and can hide evidence.
- C2 active: a single canonical context packet is better than many hand-written
  "LLMs docs." `llms.txt` should be a generated projection if needed.
- C3 active: source docs need role/scope metadata before a compiler can safely
  summarize them.
- C4 active: current and target architecture must be separated at the sentence
  level, not merely by document title.
- C5 active: docs updater automation should first produce findings and proposed
  patches, not push silent rewrites.
- Edge: normal docs-only CI is intentionally skipped today. The mission must
  decide whether to preserve that policy with manual/scheduled checks or update
  the operating contract to allow blocking docs checks.
- Edge: cheap model selection and prompt design for docs review is unsettled;
  use deterministic compiler output as model input to keep costs bounded.

next move: manually repair the high-read README/current-doc service-topology
drift, then implement the smallest deterministic script that reads a manifest
and reports heresy detector deltas without failing. After that, add the context
packet compiler in report-only mode.

ledger file: `docs/mission-doc-truth-drift-ci-context-packet-v0.ledger.md`.

version / lineage: broadens `docs/mission-heresy-detectors-ci-v0.md` after the
README service-topology drift showed that heresy detection alone is insufficient
for docs truth maintenance.

learning state: retained here until implementation creates durable scripts,
generated artifacts, and CI/manual workflow receipts.

settlement: not claimed. Settlement for v0 requires at least one high-read stale
claim repaired, a structured doc authority manifest, executable heresy detector
report, generated `docs/choir-context-packet.md`, and a documented CI/manual
workflow decision that does not contradict `AGENTS.md`.
