# Mission - Docs Truth System v1

This paradoc supersedes the broader open handoff in
`docs/mission-doc-truth-drift-ci-context-packet-v0.md` for the next docs truth
implementation slice. It keeps `cmd/doccheck` v0 as the existing report-only
instrument, but widens the target from "docs checker" to a docs truth system:
canonical focal docs, machine-readable mission DAG, canonical conjecture
register wiring, and heresy checking across docs and code.

## Problem Record

Choir now has a useful v0 docs checker, but the self-model is still too loose.
`scripts/doccheck` reports 803 warnings over 200 docs in under one second, but
most warnings are semantic drift rather than formatting. The largest buckets
are retired vocabulary and doctrine/heresy residue in current or mixed claims.
The repo also has the right source seeds, but they are not yet wired into one
system:

- `docs/choir-doctrine.md` is the apex doctrine.
- `AGENTS.md` is the repo operating contract.
- `docs/current-architecture.md` and `docs/platform-os-app-state.md` describe
  current state.
- `docs/mission-portfolio-2026-06-11.md` carries the mission DAG in prose.
- `docs/conjecture-assertion-ledger-2026-06.md` is the canonical assertion
  register candidate.
- `docs/heresy-detectors.md` defines detector families, but v0 doccheck only
  consumes them as Markdown detector terms.

The missing object is a deterministic truth spine that both humans and agents
can follow before editing code or docs.

## Target Shape

v1 should produce and check these artifacts:

- **Focal docs spine.** README -> docs index -> Choir Doctrine -> current
  architecture, mission portfolio, assertion ledger, heresy detector manifest,
  and domain invariants. Historical/evidence docs stay accessible, but they do
  not compete with the spine.
- **Mission graph.** A machine-readable DAG, initially
  `docs/mission-graph.yaml`, that records mission ids, paradoc paths, status,
  dependency edges, portfolio relation, and whether a mission is spine, side,
  evidence, or superseded.
- **Parallax integration.** Creating a new paradoc must add or update its
  mission graph node in the same pass, unless the mission is explicitly
  scratch/outside-repo. The graph is not a second mission log; it is the
  indexable DAG.
- **Conjecture register wiring.** `docs/conjecture-assertion-ledger-2026-06.md`
  is the single canonical assertion/conjecture register. Mission docs may carry
  local working conjectures, but promoted or supported claims must link to the
  register instead of creating parallel canonical ledgers.
- **Code and docs heresy checking.** Detector families should be structured and
  applied to docs, runtime prompts, Go, Svelte, Nix, scripts, and workflow files
  with typed contexts: `current-violation`, `implementation-transitional`,
  `explicitly-deprecated`, `historical-evidence`, and `detector-definition`.
- **Report-only baseline first.** v1 may fail on malformed graph/register
  syntax once the artifacts exist, but heresy deltas should remain report-only
  until the baseline has typed allow contexts and an explicit owner-approved
  fail-on-introduced policy.

## Parallax State

status: settled

mission conjecture: if v1 promotes doccheck from a Markdown warning reporter
into a docs truth system with a focal-doc spine, mission DAG, assertion register
wiring, and structured code/docs heresy baseline, then agents and human devs
will conceive of the codebase from the same canonical sources instead of
optimizing stale docs, parallel mission lists, or unclassified detector counts.
Status: supported for the v1 report-only scope by local checker output and
repo artifacts.

deeper goal (G): make Choir's self-understanding operational. The repo should
tell a new agent or human developer where doctrine lives, what architecture is
current, what missions are next and why, which conjectures are asserted versus
open, and which heresies are discovered, introduced, or repaired.

witness/spec (A/S): implement the first docs truth system slice:
`docs/mission-graph.yaml`; a clear focal-doc spine in README/docs index;
doccheck support for validating mission graph syntax and paradoc presence;
structured detector manifest or structured projection from
`docs/heresy-detectors.md`; code/docs detector scan with typed allow contexts;
and report output that joins mission graph, assertion register, link graph, and
heresy accounting.

invariants / qualities / domain ramp (I/Q/D): Choir Doctrine remains apex;
AGENTS.md remains operating contract; no generated packet or graph overrides a
source paradoc; historical/evidence docs remain readable but labeled; mission
graph is a DAG, not a second ledger; creating a new paradoc updates the graph;
local mission conjectures do not become canonical assertions without register
promotion; detector counts are evidence, not ontology; do not fail CI on the
existing baseline until allow contexts and introduced-delta policy are proven.
Ramp from graph seed plus docs-only checks, to code/detector baseline, to
report-only CI artifact, to eventual fail-on-introduced only after review.

variant (ranking function) V: original 8 open obligations:
1. seed machine-readable mission graph;
2. update Parallax instructions so new paradocs update the graph;
3. sharpen README/docs index focal spine around doctrine, architecture,
   portfolio, assertion ledger, and detector manifest;
4. define graph schema and doccheck validation;
5. define structured heresy detector data and typed allow contexts;
6. scan code plus docs and report detector contexts;
7. wire assertion register references so promoted claims have one home;
8. decide report-only versus fail-on-introduced CI policy with baseline
   evidence. Current V=0; last delta implemented graph/register validation,
   code/docs heresy baseline, focal-spine links, and report-only behavior.

budget: one implementation mission, spent. The mission deliberately did not
try to repair all existing warnings; it made them accountable through graph,
register, and typed code/docs detector context.

authority / bounds: yellow process/test/docs mission. Runtime behavior is out
of scope. CI failure semantics for heresy deltas require explicit owner
approval after baseline review. Updating the local Parallax skill is permitted
as operator tooling, but repo doctrine remains the durable source for project
rules.

mutation class / protected surfaces: yellow. Protected surfaces include
Choir Doctrine, AGENTS.md, README/docs index, mission portfolio, assertion
ledger, heresy detector manifest, doccheck CI behavior, and Parallax skill
instructions.

evidence packet: `docs/mission-graph.yaml` exists with 13 nodes and 13
dependency edges; `skills/parallax/SKILL.md` and the live Parallax skill both
require new/materially re-scoped paradocs to update the graph; README and
`docs/README.md` now point to the focal truth spine;
`docs/doc-authority-manifest.yaml` manifests the assertion ledger;
`cmd/doccheck` validates graph syntax, missing graph paths, dependency
references, cycles, and assertion register sections/ids; `scripts/doccheck`
reports graph/register/heresy-scan
sections; code/docs heresy baseline scanned 725 files, 10 detector families,
and 53 detector terms with typed contexts; `go test ./cmd/doccheck` passed;
`.github/scripts/deploy-impact-classify-test` passed; the docs/checker paths
alone classify as report-only, but the requested repo Parallax skill copy under
`skills/` currently classifies as sandbox host-service impact; `scripts/doccheck`
exited 0 with 202 docs, 803 warnings, and 4622ms runtime; `git diff --check`
passed.

heresy delta: discovered that docs warning counts alone are insufficient: the
repo needs a canonical mission DAG, canonical assertion register wiring, and
code-surface heresy detection. Introduced risk: a machine-readable graph can
become a second stale roadmap if Parallax does not update it. Repaired for v1:
new paradocs now have a Parallax graph-update rule, this paradoc is graphed,
and doccheck validates graph/register structure while preserving report-only
detector accounting.

position / live conjectures / open edges: doccheck is now a report-only docs
truth system slice, not just Markdown warning output. The mission graph is a
seed, so the report records 58 ungraphed historical/current paradocs without
making that a failure. The assertion ledger has 6 assertions, 7 invariant
candidates, and 5 open edges. The code/docs heresy baseline has 2413 findings:
1863 `current-violation`, 77 `detector-definition`, 36
`explicitly-deprecated`, 369 `historical-evidence`, and 68
`implementation-transitional`. The remaining edge is policy, not mechanics:
typed allow contexts and fail-on-introduced semantics need baseline review
before any CI failure gate.

next move: no required v1 work remains. A successor mission should review the
typed code/docs heresy baseline, classify allow contexts, and only then decide
whether to fail CI on newly introduced unaccepted current violations.

ledger file: docs/mission-docs-truth-system-v1.ledger.md

version / lineage: successor to
`docs/mission-doc-truth-drift-ci-context-packet-v0.md`,
`docs/mission-doc-heresy-checker-v0.md`, and
`docs/mission-heresy-detectors-ci-v0.md`. It also depends on
`docs/mission-portfolio-2026-06-11.md` and
`docs/conjecture-assertion-ledger-2026-06.md`.

learning state: durable rules promoted into Parallax skill, docs index,
mission graph, authority manifest, and doccheck report structure. Remaining
baseline-review policy should live in a successor paradoc.

settlement: settled only when the repo has a checked mission graph, Parallax
new-paradoc guidance updates that graph, focal docs point at the canonical
truth spine, doccheck validates graph/register structure, code and docs heresy
surfaces are scanned with typed contexts, and CI/report behavior remains
report-only unless an owner-approved introduced-heresy gate exists. Status:
satisfied for v1 by the evidence packet above.

post-settlement note (2026-06-14): the mission graph has since expanded from a
seed graph into a mission-corpus index with 71 nodes, 14 dependency edges, and
zero ungraphed mission-shaped docs under `go run ./cmd/doccheck`; this corrects
factual drift without reopening v1's original settlement claim.

## Suggested Goal String

```text
/goal Use Parallax on docs/mission-docs-truth-system-v1.md. Treat it as the settled v1 docs truth system mission. Current status is settled: docs/mission-graph.yaml exists and is validated by doccheck, Parallax guidance requires new/materially re-scoped paradocs to update the mission graph, README/docs index point at the focal truth spine, docs/doc-authority-manifest.yaml manifests the assertion ledger, doccheck validates the assertion register, code+docs heresy baseline scans 725 files with typed contexts, and report behavior remains warn-only/exit 0. Do not reopen v1 except to audit or correct factual drift. For new work, create a successor paradoc to review typed allow contexts and decide whether to fail CI on newly introduced unaccepted current violations. Ledger: docs/mission-docs-truth-system-v1.ledger.md.
```
