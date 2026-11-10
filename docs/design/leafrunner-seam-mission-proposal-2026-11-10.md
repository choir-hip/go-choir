# The LeafRunner seam: a mission proposal (2026-11-10)

Status: proposal, `readiness: drafted`. Not an executable goal file: it has no
review receipt and no owner authority, and it is not listed in
`docs/ACTIVE.md` or `docs/mission-graph.yaml`. Mutation class of this file:
green. Station source: X6 of
[the overnight definition](../definitions/choir-autopaper-overnight-2026-11-09.md)
(line 322, "the `LeafRunner` seam as a mission proposal").

Labels: **[C]** read in code at `origin/main` (313e5bae3), file:line given;
**[D]** stated in a design doc; **[I]** inferred; **[U]** unverified. Nothing
here was measured on a running computer.

## 0. Short answer

1. `LeafRunner` does not exist in code. A grep of the whole tree finds the word
   only in three docs [C]: the O1 slice of
   `outboard-capsules-arch-11-12-2026-10-22.md:216`, the FP10 station
   `choir-launch-fp10-arch-11-12-2026-10-21.md:132`, and the overnight
   definition `:322`. The seam is a named slice (O1), never built.
2. What the seam would cut is exactly one function: `runResearchChildWork`
   (`internal/agentcore/research_children.go:297-381`), the body of one
   research child. Everything above it (the Cast and Report records, the
   fan-out semaphore, the judge) is lessor-side and already separable;
   everything below it (model selection, the Texture lease, the desk-worker
   subprocess, the provider loop) is what the seam must hide.
3. The seam is not one-sized. A research child is a stateless, tapeless,
   read-only model loop that returns text. The engineering performer (the
   coding agent over MCP, `coding-agent-desk-over-mcp-2026-11-09.md` section
   3.2) is a long-lived, assignment-bound agent that writes records through the
   reducer. The proposal builds the seam for the first only, and says what
   observation would show the second shares it (section 7).
4. One kind of leaf must never cross the seam: the `research_texture` child
   (section 3.4). Its findings carry `owner_private:texture` text, and the
   outboard design's whole point is that private text and untrusted content do
   not meet outside the guest.
5. The old O2 text names `VMCTL_OUTBOARD_MODE=off`
   (`outboard-capsules-arch-11-12-2026-10-22.md:217`, the O2 row; the rollback paragraph repeats "the mode flag off"). That
   is a runtime switch, which CLAUDE.md ("No switches") now forbids. O2 must be
   restated before it is built; this mission does not need one.

## 1. The present state, from the code

### 1.1 The call path

| Step | Where | What it does |
|---|---|---|
| Cell calls `choir.Research` | `internal/yaegikernel/broker_protocol.go:166-210` (`ResearchChild`, `ResearchChildResult`) | the wire shape; result carries findings text, error, usage, `CastID`, `ReportID` |
| Host egress dispatch | `internal/agentcore/tools_desk.go:1496-1497` | `if action == yaegikernel.ActionResearch { return rt.runResearchChildren(...) }` |
| Fan-out | `research_children.go:178-237` | decode, `ResearchChildrenBounds` (max 16, `yaegikernel/broker_bounds.go:28,121`), deadline = min(cell deadline, 10 min `researchCallWindow` :84), goroutine per child behind a 4-wide semaphore (`researchChildConcurrency` :28), then one async judge call (`enqueueSubCallJudge` :231) |
| One child | `research_children.go:283-295` (`runResearchChild`) | role check, `openChildCast`, `runResearchChildWork`, `closeChildCast` |
| Cast before, Report after | `internal/agentcore/subcall_record.go:277-296` and `:301-330` | the Cast is appended before the child starts and a Cast that cannot be recorded means the child does not run; the Report is appended and only then are findings returned; unrecorded findings are blanked |
| The work | `research_children.go:297-381` | see 1.2 |

There is one non-test caller of `runResearchChildren` (`tools_desk.go:1497`) and
one of `runResearchChildWork` (`research_children.go:293`) [C, by grep]. The
test file `research_children_test.go` has four tests (`:78, :174, :244, :321`):
fan-out to child activations, one computer gate for all children's model calls,
the tree returning within its time, and spending reported.

### 1.2 What `runResearchChildWork` reaches for

Inputs: `ctx`, the parent `toolregistry.ExecutionContext`, the `researchLineage`
(owner id, inherited model selection and fallbacks, `:149-153`), the
`ResearchChild` spec, a role and a child key.

Each dependency below is an in-guest dependency:

| Dependency | Where | In-guest because |
|---|---|---|
| Model selection by name, pool or catalog id | `researchChildSelection` :453-496 reads `rt.modelPolicy` and `modelcatalog` | policy is per owner on this computer |
| Provider and call spacing | `spacedProvider{... rt.provider ..., rt.subRLMCallSpacer()}` :350, spacer :135 | one computer-wide 40/min gate (`agent-tree-budgets-2026-10-21.md`, "Model calls" row) |
| Quota ledger, incident hook, continuation chain | `rt.quotaLedger`, `rt.providerQuotaIncidentHookFor`, `rt.quotaContinuationChainFor` :364-368 | the computer's shared quota |
| Reading identity | `researchChildReadContext` :498-501 (`objectgraph.WithoutLeases`) | strips the parent's leases from the web child |
| Texture lease | `researchTextureChildContext` :509-529 (`rt.leases.IssueTextureSearch`, `End`) | the Texture custodian issues it for this call |
| The cell evaluator | `newDeskGoEvalTool(rt, workers, role)` :331-332, `tools_desk.go:585`; cells run in a re-exec'd `desk-worker` subprocess (`deskWorkerBinary`, `tools_desk.go:1041`; `cmd/desk-worker`) whose egress calls come back to the `Runtime` | the child's tools are host verbs served by the Runtime |
| Prompt | `rt.researchChildSystemPrompt(ownerID, role)` :425 | owner's prompt identity |
| Parent-run emit | `researchChildServedEmitter` :383 | writes a model-served line on the parent run |

Output: a `ResearchChildResult` with `Findings` (the model's final text),
`Error`, three token counts and `DurationMs` (:369-380).

So today a leaf is "a model loop on the guest's `Runtime`, with a subprocess for
its cells". Its output is already one bounded value, text plus usage, and its
effects on the computer are already confined: no run record, no channel
(`childExec.RunRecord = nil`, `ChannelID = ""` :309-310), no write verbs
(deskmanifest research_child row, cited at plan MA4.6), and the parent's tape
sees it only through the Cast and Report records. That confinement is why a seam
is cheap.

### 1.3 What assumes the in-process engineer

- **The model loop is a Go call on `rt.provider`**, not a message. An outboard
  runner has no `rt`. The loop would run host-side or machine-side; which is the
  main design choice (section 4.2).
- **The cell evaluator is a local subprocess** whose egress is served by the
  same `Runtime` that spawned it. An outboard machine needs those egress verbs
  (world read, search) served over the relay, not by a local `Runtime`.
- **Taint is a context value and a session label**, not a field of a request:
  `deskSessionWorkers.taintSession` (`tools_desk.go:195`) folds labels into the
  live session; the Texture child's Report is labelled after the fact
  (`subcall_record.go:307-310`). A seam must carry the taint class as data
  [D: outboard design, "Taint travels with the proposal"].
- **The Cast and Report are written by the Runtime, around the work**, so they
  are already on the right side of a seam: the lessor writes both, the leaf
  writes neither. This matches the outboard rule that nothing returned is
  applied without the desk's commit [D, outboard section 3.5].
- **Engineering is a different object.** `engineeringCastNetwork`
  (`engineering_assignment_runtime.go:784-790`) picks a capsule network mode,
  and the Claude delegate runs inside an engineering capsule under the executor
  (`rt.capsuleExecutor`, `runtime.go:353`). It is stateful, bound to an
  assignment and the reducer, and reaches the tape through desk calls
  [D: coding-agent doc, sections 1 and 2.2]. It does not pass through
  `runResearchChildWork`, and nothing in code would call a `LeafRunner` for it.

## 2. The seam, as the designs intend it

The outboard design, section 5: "O1: A `LeafRunner` seam in agentcore: research
children go through it; the only implementation is today's in-guest activation.
Behavior unchanged; proposed-act shape defined. orange, `internal/agentcore`
only, none; buildable now without FP8's files." Section 3.5 fixes the shape of
what returns: a typed proposed act with a size cap, delivered to the leasing
desk, validated and committed by that desk; taint travels with it.

The coding-agent doc makes a second consumer concrete: shape (b) registers a
"capsule driver" named `claude-code` as a package-level choice of the
engineering desk (MA4.10, plan row at
`modular-architecture-plan-2026-11-04.md:1122`). The plan's MA4.6 row (:1118)
already says children report to the parent as precommitment records, which is
what `openChildCast` and `closeChildCast` do.

## 3. What the seam must say

### 3.1 Request (lessor to runner)

A value with no pointer into the `Runtime`:

- `objective` (parent-authored text, by value) and `role` (`research_web` only
  for now);
- `taint_class` and the labels the leaf will carry, as data;
- `model` already resolved to a selection plus fallbacks (resolution stays
  lessor-side because it reads the owner's policy);
- an absolute `deadline`, and a `usage_ceiling` field that is tracked, not
  enforced (DEC-07: "track spending, never cap");
- the leaf's identity: `fan_id`, `index`, `child_key`, which the Cast already
  names (`subcall_record.go:277`).

### 3.2 Outcome (runner to lessor): the proposed act

`findings` (bounded text), `error`, `usage` (three counts), `duration_ms`,
`labels`, and the runner's own name. This is the field set of
`ResearchChildResult` minus `CastID`, `ReportID` and `WeightZero`, which the
lessor adds. The size cap on `findings` does not exist today: `closeChildCast`
passes `result.Findings` through as given [C: `subcall_record.go:301-330`]. The
outboard design requires one (its failure mode F10).

### 3.3 Where each existing step lands

| Step | Side of the seam |
|---|---|
| bounds check, role check, semaphore, deadline | lessor |
| `openChildCast`, `closeChildCast`, judge | lessor |
| model selection and fallbacks | lessor |
| call spacing, quota ledger, incident hook | lessor (the gateway forwards for the leaf; the leaf holds no bearer, OQ-2) |
| model loop and cell evaluation | runner |
| web read, search egress | runner asks the lessor's relay |

### 3.4 What never crosses: the Texture child

`researchTextureChildContext` issues a `texture.search` lease held by the
guest's custodian (:509-529), and the child's Report is labelled
`owner_private:texture` (`subcall_record.go:307-310`). The outboard machine is
for untrusted web content; the design's first slice is `untrusted_web` only
[D, O2]. So `LeafRunner.Run` must refuse a `research_texture` request on any
runner but the in-guest one, in the type, not in a flag. This is the one
correctness refusal the seam adds. It is stated here so that it does not arrive
later as a switch.

## 4. Options, cheapest first

### 4.1 Physics

- Bytes across the seam per leaf [I]: the request is an objective (hundreds of
  bytes to a few KB) plus a handful of ids; the outcome is findings text. I
  could not measure findings size from code; a typical research Report is
  plausibly 1 to 20 KB [U]. At 16 leaves per call (`MaxResearchChildren`) that
  is under 0.4 MB per call. The seam is never bandwidth-bound; the load is the
  model and web traffic, which the relay carries.
- Time: the child's wall budget is the lesser of 0.75 of the parent's remaining
  time and the 10-minute window [C: `researchChildShare` :86,
  `researchCallWindow` :84]. A machine restored from a template adds a start-up
  term to that budget. It is unmeasured (outboard design section 8: "O2
  measures it"). The seam does not depend on it; O2 does.
- Concurrency: 4 per call today (:28) protects guest memory. An outboard runner
  is where it could rise; the seam keeps the semaphore lessor-side so the
  number is the lessor's, not the runner's.

### 4.2 Where the model loop runs (the real choice)

| Option | The loop | Pros | Cons |
|---|---|---|---|
| A. Loop in the guest, cells outboard | `RunToolLoop` stays on `rt.provider`; only the cell evaluator moves | no gateway bearer in the machine; spacer and ledger unchanged | tool results (untrusted page text) flow back into the guest's model context, so the guest still holds the web's content in its own process |
| B. Loop in the machine | machine runs the loop; model calls relayed through the host gateway | untrusted content stays out of the guest until a validated proposed act arrives | needs SEC-05, ARCH-07 and a loop implementation outside `Runtime`; spacing and quota move to the gateway |
| C. In-guest only (today) | as now | nothing to build | no boundary |

O1 builds C behind an interface that can later hold B. A is a trap: it moves the
evaluator and keeps the exposure, so it satisfies the letter of "outboard" and
misses the point. It is the goodharting form named in 5.2.

## 5. The proposal

```yaml
mission: leafrunner-seam
readiness: drafted
slice: O1 of the outboard-capsules design
```

### 5.1 Deliver and finish

- **deliver:** research children run through one `LeafRunner` interface whose
  only implementation is today's in-guest activation, with the request and the
  proposed act defined as plain data, so a later runner (the outboard machine,
  or a coding agent) is a second implementer, not a rewrite.
- **start receipt (to be captured at execution, not yet observed):** `main` at
  the deploy tree; the four tests in `research_children_test.go` green; the
  count of `*Runtime` fields read by `runResearchChildWork`; no `LeafRunner`
  symbol in code. Any dirty worktree is classified before work.
- **finish:** (1) `internal/agentcore` defines `LeafRunner`, `LeafRequest` and
  `LeafOutcome`, and `runResearchChild` calls the runner where it now calls
  `runResearchChildWork`; (2) the in-guest runner is the old body moved, with no
  change of observable behavior; (3) `LeafOutcome.Findings` is capped and
  `closeChildCast` records the capped text; (4) the type keeps
  `research_texture` on the in-guest runner; (5) landed on `main`, CI green,
  deployed to staging.
- **acceptance:** the four existing tests pass unchanged; a new E2E on d@e runs
  a real research round and the tape holds the same Cast, Report and self-score
  record kinds and counts as a pre-change run of the same objective (compared by
  shape, since the text is a model's); and a test-only stub second runner
  returns a canned `LeafOutcome` whose findings reach the parent only through
  `closeChildCast`.
- **landing path:** commit, push, CI, deploy, staging build identity, the d@e
  round. Required: the change touches `internal/agentcore`, a runtime path.
- **rollback:** `git revert` of the one commit. No data migration, no setting.
  A research round in flight at the deploy is closed by the planned-restart
  rules and not resumed.
- **mutation class:** orange (runtime behavior; no protected surface: no vmctl,
  no Texture canonical write, no gateway). The Cast and Report writes are
  unchanged apart from the findings cap.

### 5.2 Value

- `better_means`: reduce what a leaf's work reads directly from the `Runtime`
  and context (the nine dependencies in 1.2) to the fields of one request value,
  while preserving that every child has a recorded Cast before it runs and a
  recorded Report before its findings return.
- `goodharting_would_be`: an interface with one method whose request struct
  holds `*Runtime` or the parent `ExecutionContext`, so the call compiles and
  nothing is separable; or option A (moving only the cell evaluator) called
  "outboard". The tell is the test: a second runner that needs `rt` to do its
  job has not been separated.

### 5.3 Realism axis

One real system, resolution rising along where the model loop is placed:
(0) in the guest behind the interface (this mission); (1) a second runner in
another process on the same computer, relaying model calls through the Runtime,
no new boundary; (2) the same on a leased machine (O2, red); (3) on another host
(O6). Each rung keeps the Cast, Report and proposed-act semantics. Rung 1 is the
cheapest honest test that the request and outcome are complete, and it is not a
fake island: it is rung 2 with the vsock relay replaced by a pipe.

### 5.4 Bridge conjecture and its edge

```text
If the research child's work is expressed as a LeafRequest -> LeafOutcome
function with no pointer into the Runtime (A), and the in-guest runner passes
the existing fan-out, gate, timing and spending tests unchanged (S), under the
Cast-before and Report-before-findings invariants (I), over web research
children (D), then an outboard runner is a second implementer and the red
slices O2 to O4 add a boundary without changing the leaf's contract (G).
```

- CLAIM: the field set of 3.1 and 3.2 is complete for a web research child.
- TEST: build rung 1 (a second runner that works only from `LeafRequest`), run
  the same objective through both runners on d@e, compare Cast, Report and usage
  records by shape.
- EDGE: **frame_lock**. The request is derived from reading one implementation.
  Anything the in-guest child gets implicitly (an env var, a context value whose
  key was not listed, the preamble map at :64, the cache scope at :439) would be
  missing from the request, and would show only when a runner without `rt` is
  tried. The in-guest runner passing its own tests does not shrink this edge; it
  is the same observer.
- ΔO: rung 1, the second runner that can only see a `LeafRequest`, written
  before O2. It costs no VM.
- SCOPE: web research children, `untrusted_web`. Nothing about Texture
  children, heavy compute (O5) or engineering.

### 5.5 Boundaries

- Changed: `internal/agentcore/research_children.go`,
  `internal/agentcore/subcall_record.go` (the cap), one new small file for the
  interface, and tests in `research_children_test.go`.
- Not touched: vmctl, vmmanager, the relay, the gateway, Texture, the
  deskmanifest rows, `yaegikernel` wire types (`ResearchChildResult` keeps its
  JSON, so the cell SDK and existing cells see no change).
- No runtime switch: the in-guest runner is simply the only implementation, and
  a second is registered by what the computer has, not by a flag.
- Crash and restart: the seam adds no resumer. A child interrupted by a crash is
  closed with the parent's tree at boot [D: outboard 3.6].
- Host-side bounds (`ResearchChildrenBounds`) stay above the seam so a cell
  cannot bypass them.

### 5.6 First three do-now items

1. **Pin the behavior first.** Before moving code, add a test beside the four
   existing ones that records, per child, the ordered record kinds appended to
   the store (Cast, Report, self-score) and the labels on the Texture child's
   Report, using the harness already in `research_children_test.go`. Run it on
   `main` and see it pass. Yellow.
2. **Extract the interface and the in-guest runner.** Define `LeafRequest`,
   `LeafOutcome` and `LeafRunner` in a new file in `internal/agentcore`; move
   `runResearchChildWork`'s body into `inGuestLeafRunner.Run`, with selection
   resolved before the call (so `researchChildSelection` stays lessor-side) and
   the Texture lease issued lessor-side and passed as an opaque handle; route
   `runResearchChild` through it. The type refuses `research_texture` on any
   other runner. Orange.
3. **Write the rung-1 runner in a test.** A `LeafRunner` in the test file that
   sees only a `LeafRequest`, calls a stub provider, returns a `LeafOutcome`, and
   cannot reach `rt`. Run the fan-out test through it. Anything it needs and
   cannot get is the edge shrinking: add it to the request and to section 3.
   Land the findings cap with this item, because the stub shows what the cap
   does to an over-long answer.

### 5.7 Stop conditions

- If the request needs more than a dozen fields, or a field that is itself a
  pointer to a service, stop and record why: the leaf's authority is still the
  Runtime's, and the outboard premise (a leaf with no authority) needs
  restating. Do not widen the interface to make the test pass.
- If the characterization test in item 1 cannot be made to pass on `main` first,
  the harness hides the behavior; fix the harness before the refactor.

## 6. Standing questions, answered

- **Decision provenance:** the seam is O1 of the outboard design (owner notes
  §36, DEC-05); the overnight definition line 322 asks for this proposal.
- **Settled-decision conformance:** no switch; sole writers unchanged (desks
  commit, the leaf proposes); the restart rule holds.
- **Single state authority:** the lessor's tape for the tree; the leaf has none.
- **Artifact-verified success:** records read back from the tape on d@e,
  compared with a pre-change run.
- **Fate-sharing:** a leaf shares its parent's deadline and fate, as today.
- **No-SSH operability:** unchanged; the research tree log line
  (`research_children.go:239-252`) remains the visible record.
- **Deletion citers:** `runResearchChildWork` is cited only by
  `runResearchChild` and the tests [C]; moving it deletes no public symbol.

## 7. What would decide the open parts

- Whether the engineering performer belongs behind the same seam. After rung 1,
  list the fields a coding-agent run needs that a `LeafRequest` lacks
  (assignment id, capsule id, a long-lived process, record authority through the
  reducer). If the list is only assignment binding and a reducer channel, a
  `LeafRunner` with a kind field may serve both; if it includes a stateful
  session or a wake path, the engineering driver is a separate seam (MA4.10's
  capsule driver) and the two share only the Cast, Report and proposed-act
  vocabulary. I expect the second [I]: the MCP design (its section 2.2) pulls
  desk calls to the host, the opposite direction to a leaf that returns one
  value.
- Whether option B (loop in the machine) is affordable: O2's per-leaf memory and
  start-up measurement, which does not exist.
- The size of the findings cap: take it from the p99 of Report text size in the
  d@e round; I have no such number.

## 8. What I could not determine

- Findings and Report sizes in practice (no tape read here).
- Whether anything outside `internal/` constructs a `ResearchChild` directly:
  grep found only the egress call at `tools_desk.go:1497` within `internal/`.
- The full grant list of the `research_child` deskmanifest row (I read the
  references to it, not the row).
