---
definition_version: 2

start:
  captured_at: 2026-08-13T03:25:35Z
  source:
    canonical_ref: main@0e48720295b0cda4d72ca748a8304c7e495bd027
    deploy_identity: staging proxy 633131aa0521bd1a427f335e147610a314829886 observed via https://choir.news/health at 2026-08-13T03:24Z
  worktree_inventory:
    status: reconciled
    evidence_ref: "git status --short returned no paths at 2026-08-13T03:23Z before this Definition candidate"
    preservation_rule: preserve unrelated work and classify every new dirty path before implementation
  worktrees:
    - path: .
      status: clean
      class: goal_candidate
      owner: owner + mission lead
      touch: goal_owned
      paths_or_digest: "Definition opening scope: docs/choir-doctrine.md, docs/agent-product-doctrine.md, docs/memo-persistent-rlm-actors-2026-08-09.md, this Definition, and the three registries"
      recovery: revert the docs-only opening commit; no runtime state changes in the opening boundary
  candidates:
    - id: rlm-kernel-opening
      ref: current worktree
      base: 0e48720295b0cda4d72ca748a8304c7e495bd027
      scope: [docs]
      disposition: active
      evidence_ref: current worktree diff
  observed_artifact:
    - claim: "Durable actors, work items, guest-local capsules, transaction tape, capability brokers, typed coagent updates, Texture evidence sources, and the reversible self-development envelope exist as separate substrates; no private Go actor kernel is implemented."
      evidence_ref: "docs/computer-ontology.md:67-97; docs/definitions/choir-supervised-self-development-effects-2026-08-11.md start.observed_artifact"
    - claim: "Researcher has no Bash; CoSuper shell/filesystem/build work is already intended to cross capsule broker verbs, but current Researcher and CoSuper activation behavior remains role-specific and tool-oriented."
      evidence_ref: "docs/agent-product-doctrine.md Authority Boundaries; internal/agentcore/tool_profiles.go"
    - claim: "Current coagent delivery separates model-authored typed payload from runtime-loaded authority, but log, delivery, wake, supervision, and settlement are not yet one general typed obligation mutation."
      evidence_ref: "docs/memo-persistent-rlm-actors-2026-08-09.md:399-403,564-592"
    - claim: "Yaegi is not currently a Choir dependency or production runtime."
      evidence_ref: "repository search for yaegi/interp.New found only docs at start"
  unknowns:
    - exact Yaegi safe-package allowlist and wrapper-generation mechanism
    - exact opaque-handle and module API needed to prevent confused-deputy and cross-workstream reads
    - exact continuation schema, orchestration graph schema, and privacy-aware transclusion range schema
    - exact capsule resource limits that preserve useful local concurrency while bounding fork, deadlock, disk, memory, and output exhaustion
    - whether the current reversible-effects Definition will be complete before this successor reaches its first effect-bearing staging gate

finish:
  deliver: "Choir agents can inhabit private programmable Go activations: an implementation CoSuper dynamically writes reusable orchestration code, uses direct Bash inside its existing effect capsule, delegates and communicates through typed durable operations, survives activation death, and exposes exact consequential receipts that supervision cites and Texture transcludes."
  artifact: "A deployed common private-Go actor kernel and first complete effects-capable CoSuper profile: one private Yaegi interpreter per bounded activation; assignment-scoped module manifest and activation capabilities; Go-only access to Choir capabilities; direct Bash plus Go execution through one capsule broker; durable assignments/messages/continuations; inert reusable source artifacts; normalized citable orchestration receipts; supervision excerpt projection; and Texture evidence transclusions."
  acceptance:
    - action: "Static capability and import refusal suite executes adversarial Go cells for unknown imports, unsafe/reflection/process/network/filesystem escapes, forged, cross-workstream, or stale handles, copied source under a weaker profile, leaked broker credentials, rich returned host objects, unauthorized material passed through allowed model/connector calls, and direct subprocess impersonation."
      proves: "The module vocabulary does not substitute for authority, forbidden ambient capabilities refuse, persisted source carries no authority, tainted or private inputs cannot cross scope through an allowed deputy, and subprocesses cannot inherit activation capabilities."
      evidence_class: focused security tests plus independent security review
    - action: "Capsule resource experiment runs interpreted infinite loops, goroutine and child-process leaks, deadlocks, fork pressure, memory/disk/output exhaustion, cancellation, and activation kill."
      proves: "The guest-local capsule—not Yaegi—contains arbitrary model-authored Go and all descendant processes without damaging the persistent computer or trusted reducers."
      evidence_class: VM/capsule experiment and resource receipts
    - action: "Focused product-path activation runs a sealed implementation CoSuper assignment with only Go and direct Bash model-facing operations; the actor authors a reusable Go function, invokes multiple commands through direct Bash and Go execution, dynamically assigns at least two independent actors, exchanges typed objection or evidence updates, and freezes source/evidence artifacts."
      proves: "The common kernel supports adaptive programming and real multiagent orchestration rather than merely evaluating Go or translating fixed tools into Go syntax."
      evidence_class: deployed staging trajectory and trace graph
    - action: "Kill that activation while work and delegated assignments remain open; rewarm the same accountable actor with a fresh interpreter and a different model permitted by current computer policy; finish using only durable artifacts, messages, obligations, and continuation state."
      proves: "Actor and workstream continuity do not depend on Yaegi heap, hidden conversation state, goroutines, subprocesses, or one model."
      evidence_class: deployed staging restart/recovery proof
    - action: "Inject activation death and retry before and after the one authoritative agent-update append and each log, delivery, wake, supervision, and obligation projection; reconstruct from the authoritative mutation identity."
      proves: "Retries are idempotent, delivery is neither lost nor duplicated, projections deterministically recover with causal parents intact, and settlement derives from durable obligation authority rather than Trace or report prose."
      evidence_class: focused fault-injection tests plus deployed staging restart proof
    - action: "Fetch the normalized orchestration graph and supervision report; verify the host-selected salient set includes every command with effects or failure, assignment, objection/message, refusal, activation death, recovery, verification, and bundle-freeze receipt, including an inconvenient event not selected by the actor."
      proves: "Consequential execution is causally complete and the actor cannot curate away dissent, failure, or elevated action."
      evidence_class: deployed API/CLI artifact proof
    - action: "Fetch a Texture version created while work is open and a terminal version; inspect exact immutable transclusions of at least one command/output, one inter-agent message or objection, and one death/recovery or verification receipt. Attempt actor-controlled omission and over-redaction; host privacy policy must remove secrets and unauthorized private content without removing the receipt identity, event class, disposition, or existence of inconvenient evidence."
      proves: "The owner-readable canonical artifact carries exact execution evidence during work; privacy enforcement is host-owned rather than an actor-controlled hiding path; Texture remains the canonical writer and receipts remain evidence."
      evidence_class: deployed API/CLI plus human-readable inspection
    - action: "After the predecessor effects Definition is complete and this Definition is atomically promoted, freeze the exact CapsuleEffectBundle from the CoSuper activation, independently verify it, and pass it into the existing external acceptance/materialization path without any Accept, Materialize, Checkpoint, or Route operation exposed to Yaegi."
      proves: "The RLM is an authoring upgrade inside the audited computer and does not collapse proposal, verification, acceptance, materialization, checkpoint, or route authority."
      evidence_class: deployed staging proof gated by predecessor goal.complete and registry promotion
    - action: "Run a Researcher-profile conformance probe against the same kernel: only Go-cell evaluation is model-facing; search/fetch/document transforms/artifacts/evidence/work/messaging succeed through narrow modules; Bash, Go execution, raw HTTP, arbitrary filesystem access, and copied CoSuper source requiring those capabilities refuse."
      proves: "The kernel generalizes by capability profile and assignment rather than by duplicating persona loops or granting role-name privilege."
      evidence_class: deployed staging profile proof
  rollback: "Disable the private-Go activation profiles and restore the prior Researcher/CoSuper activation selection; leave source, trace, message, and continuation artifacts inert and reassign every open work item through the existing trajectory substrate. Revert mission commits for repository rollback. Do not roll back canonical events; any accepted computer effect uses the predecessor Definition's acceptance-fenced forward restore."
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, staging_build_identity, deployed_activation, forced_death_recovery, security_refusals, supervision_trace, texture_transclusions, inert_or_accepted_bundle]
  not_done_when:
    - Yaegi evaluates Go but the actor still depends on ambient duplicate tools for Choir capabilities
    - only local tests or an interpreter demo pass
    - CoSuper works but the kernel or module policy cannot express the restricted Researcher profile
    - a hidden conversation, interpreter heap, live process, or mutable capsule file is required after rewarm
    - direct Bash and Go execution use different broker, receipt, transaction, cancellation, or policy paths
    - any subprocess inherits a Choir capability, credential, canonical-state path, or control socket
    - actor-authored source can retain authority across activations or profiles
    - a consequential command, message, dissent, refusal, or failure can be omitted from the host-derived salient set
    - supervision prose copies evidence without immutable refs or Texture transclusion can mutate authority
    - the RLM can accept, materialize, checkpoint, route, or directly mutate canonical computer state
    - persona-specific Researcher or CoSuper activation paths remain available for new accretion without an explicit deletion clock and refusal gate
    - predecessor reversible-effects acceptance is incomplete but this Definition claims effect-bearing completion
    - the predecessor is not goal.complete or all three registries have not atomically promoted this Definition
    - dirty work or a candidate remains undispositioned

boundaries:
  mutation_class: red
  authority_sources:
    - owner directive in this conversation on 2026-08-12 to promote doctrine then author the mission
    - docs/choir-doctrine.md
    - docs/agent-product-doctrine.md
    - docs/computer-ontology.md
    - docs/standing-questions.md
    - docs/definitions/choir-supervised-self-development-effects-2026-08-11.md
    - AGENTS.md
  must_preserve:
    - one stable ComputerID and canonical event chain; one guest event appender
    - trajectory/work-item state is settlement and restart authority; actor memory is never the sole copy
    - one typed authoritative mutation drives consequential message log, delivery, wake, supervision, and obligation projections
    - Yaegi imports present vocabulary; current activation capabilities and a trusted broker authorize each operation
    - arbitrary model-authored execution stays inside disposable guest-local capsules with hard resource limits
    - restricted profiles receive Go only; general execution is assignment-scoped
    - direct Bash and Go execution share one broker and child processes inherit no Choir authority
    - persisted source is inert and every activation resolves fresh modules, handles, heads, policy, and capabilities
    - activation-local concurrency never substitutes for durable organization or waiting
    - host-selected receipts are immutable evidence; Texture alone owns canonical document versions
    - frozen CapsuleEffectBundle remains the only self-development candidate
    - acceptance, materialization, checkpoint, route, external sends, publish, and pay remain outside interpreted code
    - no provider credentials enter model-authored code or capsule subprocesses
    - no parallel ambient model-tool path duplicates a capability exposed through Go
    - problem-documentation-first precedes repair code for every newly observed reliable platform failure
  excluded:
    - replacing the persistent actor, trajectory, work-item, canonical event, capsule, or Texture substrate
    - a generic unrestricted Go, Python, JavaScript, shell, HTTP, filesystem, database, or plugin environment for every actor
    - restoring or serializing Yaegi heaps, goroutines, channels, subprocesses, or half-completed calls
    - making every local operation durable or recording every instruction/syscall
    - full connector and Email convergence, Texture migration to the kernel, or Super quorum redesign beyond interfaces needed by the first slice
    - promoting adaptive source automatically into trusted compiled libraries
    - production rollout before staging acceptance and owner decision
    - weakening the active reversible-effects Definition or executing effect-bearing acceptance before its deployed completion
  protected_surfaces:
    - capsule execution, transaction tape, effect-bundle construction, and resource containment
    - actor activation, passivation, cancellation, restart, and model-policy resolution
    - role/capability profiles, activation capabilities, broker credentials, and provider calls
    - typed messages, assignments, obligations, delivery, wake, and work settlement
    - Trace/evidence semantics, salient excerpt selection, and privacy/redaction
    - Texture canonical writes, evidence citations, and transclusions
    - verification and external self-development acceptance boundary
  completion_evidence_floor:
    - focused deterministic tests
    - adversarial security review bound to a frozen candidate
    - guest-local VM/capsule containment experiment
    - pushed source, green CI, deployed staging identity
    - deployed sealed assignment and exact trajectory/activation/work/message/artifact/trace IDs
    - deployed cross-model forced-death recovery
    - deployed supervision and Texture artifact inspection
    - predecessor-complete externally accepted effect trajectory as specified
  conjecture_delta:
    - "C5/C6/C7: one durable actor kernel uses disposable private model-authored Go activations while trajectory/workstream owns continuity and settlement."
    - "C13: reusable model-authored Go can carry organizational learning without carrying authority."
  heresy_delta:
    discovered:
      - import allowlists alone would be an unsound sandbox or authority system
      - optional Yaegi beside a richer ambient tool loop does not test the RLM architecture
    introduced: []
    repaired_when_complete:
      - persona-specific Researcher and CoSuper activation loops
      - duplicated ambient tool and Go capability paths
      - opaque orchestration that supervision can only narrate retrospectively
      - shell or subprocess paths not causally joined to capsule receipts
      - settlement-relevant coordination left only in string messages or Trace

measures:
  - name: common-kernel profile coverage
    kind: gate
    baseline: no private-Go profile
    desired: effects-capable CoSuper and restricted Researcher conformance use the same kernel with different manifests
    decision_use: determines whether persona-loop deletion is authorized
    cannot_prove: product usefulness, containment, restart recovery, or evidence completeness
  - name: consequential receipt coverage
    kind: gate
    baseline: command, message, wake, obligation, and Texture evidence exist on separate partial paths
    desired: every acceptance-listed consequential event appears with causal parents in the normalized graph and host salient set
    decision_use: blocks supervision and Texture acceptance when an event class is missing
    cannot_prove: that supervisor judgment or Texture prose is correct
  - name: orchestration ergonomics
    kind: telemetry
    baseline: unknown
    desired: record Go correction turns, Bash/Go execution ratio, tokens, latency, reusable functions, and failed module requests for the sealed assignment
    decision_use: revise module APIs and decide whether future persistent Python or JavaScript earns a profile
    cannot_prove: security, authority correctness, or mission completion
  - name: context virtualization
    kind: weak_signal
    baseline: current role loops carry orchestration primarily in model/tool context
    desired: bulk data and intermediate results remain in artifacts/Go state while the actor receives bounded views
    decision_use: compare the common kernel with the prior loop and identify missing primitives
    cannot_prove: restart durability, correctness, or containment

now:
  status: blocked_incomplete
  slice: owner-ratified successor authority; runtime work is blocked until predecessor completion and atomic registry promotion
  question: none
  reconciliation:
    observed_at: 2026-08-13T03:30:02Z
    source_ref: main@0e48720295b0cda4d72ca748a8304c7e495bd027 plus reviewed docs candidate
    deploy_identity: staging proxy 633131aa0521bd1a427f335e147610a314829886
    authority_identities:
      - owner directive 2026-08-12
      - docs/choir-doctrine.md current candidate
      - docs/agent-product-doctrine.md current candidate
      - docs/definitions/choir-supervised-self-development-effects-2026-08-11.md working
    policy_resolution_ref: not_applicable until first activation rehearsal
    worktree_inventory_ref: opening git status receipt plus current docs candidate
    status: reconciled
  candidate:
    id: rlm-kernel-opening
    state: reviewed
    ref: current worktree
    owner: mission lead
    base: 0e48720295b0cda4d72ca748a8304c7e495bd027
    digest: none; no runtime candidate exists while blocked
    scope: [docs]
  decision:
    selected: "Promote the common private-Go kernel and layered capability boundary; first implement one complete CoSuper profile, then prove restricted Researcher conformance on the same kernel."
    kind: architecture
    status: settled
    source: owner
    evidence_ref: owner directive and deliberation in conversation on 2026-08-12
    owner_ratification_ref: not_applicable
    recorded_at: 2026-08-13T03:25:35Z
    consequence: doctrine and this successor Definition may land; no runtime implementation or capsule experiment may begin before predecessor goal.complete and atomic registry promotion
  evidence_refs:
    - docs/memo-persistent-rlm-actors-2026-08-09.md
    - docs/choir-doctrine.md
    - docs/agent-product-doctrine.md
    - docs/computer-ontology.md
    - docs/definitions/choir-supervised-self-development-effects-2026-08-11.md
    - \"agentic-consensus session receipt: .agentic-consensus/agentic-consensus-20260812-232742/manifest.tsv; two succeeded, three timed out; raw outputs gitignored\"
  blocker_or_risk: "Blocked on predecessor goal.complete plus atomic promotion in docs/ACTIVE.md, docs/mission-graph.yaml, and docs/doc-authority-manifest.yaml. The consensus panel had two successful reviewers and three timeouts; all must-fix findings from successful reviews were adjudicated, but raw outputs are gitignored session diagnostics."
  next_action: "Complete the supervised-self-development-effects Definition; then reconcile this goal against landed doctrine and current runtime, atomically promote it in all three registries, select exact safe-package/module/resource/continuation/trace contracts, and prepare the first isolated implementation candidate."

receipts:
  - id: rlm-kernel-doctrine-definition-opening
    boundary: define
    commit_or_artifact: \"opening review candidate fbdb26524bd0ec98790f70ecaff7234b07e006d1d83417b48211b54a88a60c8d over base 0e48720295b0cda4d72ca748a8304c7e495bd027; findings adjudicated into the final frozen docs bundle named in the terminal handoff\"
    proof_refs:
      - \"doccheck live passed 11-document reading packet with zero warnings at 2026-08-13T03:30:02Z\"
      - \"Ruby YAML parse passed docs/mission-graph.yaml and docs/doc-authority-manifest.yaml\"
      - \"agentic consensus: Gemini and GPT-5.6 Sol completed; Codex, Cursor, and DeepSeek timed out; state, early-route, current-vs-target, privacy, terminology, and durable-mutation findings repaired\"
    rollback_ref: revert the docs-only opening change
    disposition: owner-ratified successor recorded blocked_incomplete; no runtime implementation authorized
    problem_ref: not_applicable
    authorization_ref: owner directive 2026-08-12
    candidate_or_evidence_refs:
      - docs/memo-persistent-rlm-actors-2026-08-09.md
      - docs/choir-doctrine.md
      - docs/agent-product-doctrine.md
    landing:
      source_commit: not_applicable until committed
      ci_ref: docs truth check local pass
      deploy_ref: not_applicable for docs-only opening
      environment_identity: not_applicable
      deployed_acceptance: not_applicable
    registry_conformance_ref: \"docs/ACTIVE.md + docs/mission-graph.yaml + docs/doc-authority-manifest.yaml; doccheck live pass 2026-08-13T03:30:02Z\"

view:
  path: none
  generator: none
---

# Private Go Actor Kernel

The model receives a real private programming environment, not a growing menu of
loosely composed tools. Restricted actors use Go alone. An effects-capable
implementation assignment additionally receives Bash inside the existing effect
capsule. Both Go execution and direct Bash cross the same trusted broker.

The kernel is intentionally not the durable actor, the security boundary, or the
state authority. Actors and workstreams survive; activations die. Capsules
contain arbitrary computation. Activation capabilities and brokers authorize
operations. Trusted reducers own shared state. Texture turns exact receipts
into an owner-readable canonical account without converting evidence into
authority.
