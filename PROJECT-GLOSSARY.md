# Project Glossary

**Last Updated:** 2026-04-30
**Purpose:** Canonical terminology for `go-choir`, including prior names and nearby synonyms that still appear in code, docs, or conversation.

---

## Canonical Terms

### Automatic Computer

**Definition:** The deployed product frame for Choir: a living-document desktop, publishing network, global citation workspace, and later compute economy.

**Current implementation rule:** do not build CHIPS mechanics yet. Preserve the provenance, citation, artifact, trajectory, model, VM, and compute-usage data that the later economy needs.

---

### dark factory

**Definition:** The mostly background agent and VM production system behind the web desktop.

**What it contains:**
- researchers
- `super` and `cosuper`
- active and background VMs
- hot-path agent delivery plus durable handoff records
- artifacts, findings, diffs, tests, publications, and Trace events

**Important rule:** the user should experience apps and living documents first. The factory exists to advance work and produce artifacts, not to become the primary UI.

---

### `conductor`

**Definition:** The intake/router agent for top-level user or connector input.

**What it does:**
- receives prompt-bar input first
- later receives email/chat/connector input first
- decides what appagent or flow should own the work

**Prior / nearby names:**
- intake router
- top-level router
- conductor task

---

### `vtext`

**Definition:** The primary version-native document app and appagent.

**What it does:**
- owns canonical document state
- turns prompts, edits, and worker updates into new versions
- is the main cumulative work surface for the user

**Prior / nearby names:**
- `etext`
- writer
- writer appagent
- versioned text editor

**Canonical rule:** use `vtext`, not `etext` or writer, for the product concept.

---

### publication

**Definition:** The boundary where private user work becomes platform-visible artifact state.

**What it does:**
- moves selected living-document/artifact state from per-user private Dolt into platform-visible publication records
- creates citeable material for the future global memory layer
- preserves provenance rather than flattening work into anonymous content

---

### citation graph

**Definition:** The platform-level network of citations between published artifacts.

**Why it matters:** It is the future global memory and value signal. Recursive citation value and CHIPS economics are later layers, but citation candidates and evidence provenance should be preserved now.

---

### `vtext agent`

**Definition:** The agent that owns the canonical `vtext` document state and writes new versions.

**What it does:**
- writes canonical versions
- spawns workers when needed
- receives addressed worker updates threaded in by the runtime
- synthesizes new document versions

**Prior / nearby names:**
- writer
- writer agent
- appagent for etext

---

### `appagent`

**Definition:** A user-facing app that has grown into an agent-owned domain with durable state, prompts, or dynamic behavior.

**Examples:**
- `vtext` appagent
- future browser appagent, if URL-bar prompts or conductor control need durable agent behavior
- future mail appagent
- future calendar appagent
- possible future Trace appagent if trajectory volume requires agentic search and dynamic UI

**Topology rule:** many appagents may coexist as peers in one user microVM, each with its own durable perspective.

**Prior / nearby names:**
- host agent
- top-level app worker

---

### app

**Definition:** A user-facing desktop surface.

**Important rule:** not every app is an appagent. Apps can remain simple display/control surfaces until they need durable domain ownership, prompts, dynamic UI, or agentic behavior.

**Examples:**
- browser
- file browser
- terminal
- audio/video/image display apps
- PDF/EPUB readers

---

### `super`

**Definition:** The per-user execution-oriented agent with the broadest tool surface.

**What it does:**
- handles execution-heavy or tool-heavy work
- can delegate further with coagent tools
- can coordinate with researchers and appagents
- can request `vmctl` resources such as background VM forks and promotions

**Topology rule:**
- there should generally be one top-level `super` per user
- `super` is the privileged orchestration root for mutable execution
- mutable work should happen in background VMs, not by editing the live desktop directly

**Prior / nearby names:**
- supervisor
- terminal agent
- terminalagent
- execution coordinator

**Idiomatic:** use `super`, not supervisor, when referring to the intended agent role.

**Current-phase note:** high reliance on `super`/`cosuper` is acceptable while the end-to-end factory is still being made real. Repeated privileged actions should later graduate into narrower tools, workers, or appagents.

---

### `researcher`

**Definition:** A research-oriented worker agent.

**What it does:**
- gathers current/external information
- reads local context
- persists evidentiary material into embedded Dolt
- sends findings back as addressed updates, usually through a typed findings handoff tool
- does not own canonical document text

**Topology rule:** researchers should usually come from a shared pool within a user microVM.

**Prior / nearby names:**
- research worker
- research agent

---

### `worker`

**Definition:** A non-canonical agent that performs delegated sub-work for an appagent or `super`.

**What it does:**
- reads context
- performs assigned work
- sends back structured updates/results/findings

**Important rule:** workers do not directly author canonical `vtext` document text and do not send document patches to `vtext`.

**Examples:**
- `researcher`
- `super`
- `cosuper`
- future specialized workers with specific tools, roles, and capabilities

---

### `work`

**Definition:** A unit of agentic effort or causal activity inside Choir.

**What it does:**
- gives us a way to talk about what agents are advancing
- may happen sequentially or concurrently with other work
- should preserve causal relationships without forcing a rigid workflow graph

**Important rule:** prefer modeling generic work plus messages, timestamps, actors, and causes over inventing overly specific workflow tables too early.

**Prior / nearby names:**
- task
- subtask
- job
- delegation

---

### `trajectory`

**Definition:** The full causal path started by one user request and continued through conductor routing, appagent ownership, worker delegation, and later revisions.

**What it includes:**
- prompt-bar input
- `conductor`
- the owning appagent, usually `vtext`
- delegated workers and their messages
- later user revisions and agent-authored versions for that same document/work surface

**Important rule:** this is the primary thing Trace should show as one coherent unit.

**Prior / nearby names:**
- workflow
- session thread
- end-to-end request

---

### `loop`

**Definition:** One individual LLM/tool execution record inside a larger trajectory.

**Important rule:** use `loop` for a single execution record; do not use `run` as the primary product term for this concept.

**Prior / nearby names:**
- run
- task record
- execution loop

---

### `task`

**Definition:** Legacy compatibility wording for a runtime handle/record, not a preferred product concept.

**Important rule:** in user-facing behavior and MAS semantics, prefer `trajectory`, `loop`, `work`, `delegation`, `agent`, or `version`. Use `task` only when discussing old code, compatibility layers, or trivia.

**Prior / nearby names:**
- runtime task
- task record
- execution handle

---

### `version`

**Definition:** A canonical document state in `vtext`.

**Examples:**
- `v0` = initial user-prompt-created document
- `v1` = conductor framing note included in the `vtext` spawn/delegation call
- `v2+` = later user edits and `vtext`-authored revisions

**Important rule:** versions are the main state transitions, not chat turns.

---

### `user-authored version`

**Definition:** A version created from a batch of user edits when the user hits Revise.

**Prior / nearby names:**
- edit batch
- user edit snapshot

---

### `agent-authored version`

**Definition:** A version authored by the `vtext` agent after synthesis.

**Prior / nearby names:**
- writer revision
- appagent revision

**Important rule:** the first agent-authored version should usually arrive promptly, even before worker evidence comes back. Later evidence can produce further agent-authored versions.

---

### worker update

**Definition:** Structured output from a worker to an appagent or super.

**Examples:**
- findings
- evidence
- source references
- artifact refs
- branch or commit refs
- preview refs
- test results
- questions
- constraints
- proposal summaries

**Important rule:** worker updates are inputs to document synthesis. They are not canonical document text and not patches to be blindly applied.

---

### `Revise` button

**Definition:** The explicit control inside `vtext` that finalizes the user’s current edit batch into a new user-authored version and re-engages the `vtext` agent.

**Important rule:** multiple user edits before Revise are one version.

**Prior / nearby names:**
- Prompt button
- Prompt / Version button

---

### `prompt bar`

**Definition:** The bottom-bar input for top-level user requests.

**Important rule:** it should always route through `conductor`.

**Prior / nearby names:**
- bottom prompt bar
- conductor input

---

### `coagent tools`

**Definition:** The tools agents use to spawn peer/child agents and send addressed work over coordination channels.

**Examples:**
- `spawn_agent`
- `cast_agent`
- `cancel_agent`

**Important distinction:** these are the generic coordination primitives. Role-specific tools such as `submit_research_findings` can sit above them to give an agent a clearer telos and a tighter handoff schema.

**Prior / nearby names:**
- co-agent tools
- channel tools

**Important rule:** role/tool matching should be enforced in code. If a role should not have shell, writable filesystem, or privileged delegation access, those tools should not be present.

---

### `coordination channel`

**Definition:** The durable coordination log used by related agents to exchange updates, findings, and coordination messages.

**Prior / nearby names:**
- channel
- work channel
- shared channel

**Important rule:** channels remain useful for audit and trace, but runtime-owned inbox delivery decides what an agent actually receives next.

---

### `dumb data, smart models`

**Definition:** A core modeling principle for Choir.

**What it means:**
- keep stored data structures generic and legible
- store facts, versions, timestamps, actors, messages, and causal relationships
- avoid baking brittle algorithms or overfit workflow logic into the schema
- let models process the data intelligently, with the policy expressible in prompts

**Important implication:**
- we should not feel pressure to encode concepts like `work_edges` just because relationships exist conceptually
- we should still preserve enough information to reconstruct sequential and concurrent causality between pieces of work

**Prior / nearby names:**
- dumb data smart models
- generic data, prompted policy

---

### `Trace`

**Definition:** The app/surface used to inspect trajectories, loops, delegations, tool calls, and message flow in the MAS.

**Important rule:** Trace is a development/debugging helper, not part of the core product loop. It should center the trajectory as the primary unit and show individual loops as children inside it.

**What it is not:**
- not the same thing as old Rust Trace
- not necessarily a dense graph UI

**Goal:** visual enough to explain what happened quickly, without forcing the user to read every message.

**Design direction:**
- use geometry
- use topology
- use temporality
- use color
- support filtering, querying, and agentic inspection

**Future direction:**
- Trace may become an appagent after trajectory volume is high enough to require agentic search and dynamic UI.

---

### `prompt management`

**Definition:** The per-user system for inspecting and editing prompts inside Choir.

**What it does:**
- exposes editable prompts for conductor, `vtext`, and worker roles
- persists them as per-user sandbox state
- eventually becomes a first-class app in the desktop

**Important rule:** prompt configuration is per-user and belongs inside the sandbox, not as a host-global setting.

**Prompting style rule:** prompts should be subtle. Prefer a few strong positive instructions over long negative rule lists.

---

### `MAS`

**Definition:** Multiagent system.

**In this repo:** the interacting set of `conductor`, appagents like `vtext`, and workers such as `researcher` and `super`.

---

### `sandbox`

**Definition:** The runtime service/process that currently hosts the local agent runtime and desktop app APIs.

**Current reality:**
- host-process fallback locally
- target runtime later lives inside per-user microVMs

---

### `vmctl`

**Definition:** The host-side VM lifecycle and ownership service.

**What it should own:**
- user VM lifecycle
- VM ownership/routing support
- later Firecracker orchestration on supported hosts

---

### `user VM`

**Definition:** The per-user microVM that holds the user’s runtime, state, and appagents.

**Prior / nearby names:**
- per-user microVM
- primary sandbox VM

---

### `worker VM`

**Definition:** A fork of the user's active VM used for delegated/background worker execution.

**Prior / nearby names:**
- child VM
- worker microVM

---

### embedded Dolt

**Definition:** The per-user in-process Dolt database used inside the sandbox/user runtime.

**Important distinction:**
- embedded Dolt = per-user runtime storage
- platform/server Dolt = possible later shared/published storage

**Important rule:** `vtext` content and version metadata live canonically in embedded Dolt, but the filesystem should expose a manifestation, alias, or shortcut so the document appears naturally in the file browser and opens into a new `vtext` window.

---

### platform Dolt

**Definition:** The platform-level Dolt database for platform-visible state.

**What it should own:**
- account/user/tenant metadata
- VM lifecycle, capacity, and routing records
- platform VM pool records
- publication records
- public artifact metadata
- citation graph
- compute accounting
- later CHIPS/token economy state

**Important distinction:** platform Dolt does not replace per-user embedded Dolt. The private desktop/appagent state remains user-local until explicitly published or packaged for worker execution.

**Important rule:** platform Dolt is a ledger, not a global message bus. Cross-VM work should use direct transport or relays for live delivery and write compact durable facts for routing, recovery, provenance, publication, citation, and compute accounting.

---

### active VM

**Definition:** The user-facing desktop VM available while a user is actively using Choir.

**What it hosts:**
- web desktop runtime
- visible apps
- visible appagents
- per-user embedded Dolt
- private app and document state

**Important rule:** free users still get an active VM while using the product. Risky system-changing work should move to a background VM when possible.

---

### background VM

**Definition:** A fork of the user's active VM for requested background or risky work.

**What it does:**
- runs development, testing, package installs, filesystem mutation, deploy preparation, and other work that could destabilize the active desktop
- can run while the user is offline
- may be 24/7 for higher paid tiers
- can merge back into active state or be promoted to active while the previous active snapshot remains available for rollback

---

### shared worker VM

**Definition:** Deferred cost-optimization idea, not a current architecture primitive.

**Current rule:** do not design near-term runtime behavior around shared worker VMs. Use active VMs and capacity-gated background VM forks first.

---

### platform VM pool

**Definition:** Platform-level VM capacity for public/unauthenticated and shared serving work.

**What it does:**
- serves published `vtext` artifacts
- hosts public readers/previews/renderers
- avoids hydrating private user active VMs for public publication reads

**Current rule:** add this during the publication pass, not before vtext stabilization.

---

### CHIPS

**Definition:** The planned token for the later Automatic Computer economy.

**Current implementation rule:** CHIPS, wallets, staking, token-denominated billing, and public citation scoring are non-goals for the current phase. Preserve the accounting and provenance inputs; do not implement the economy yet.

---

### transclusion

**Definition:** Inline embedding of referenced content or artifacts into the main `vtext` document flow.

**Examples:**
- quoted text snippets
- citations
- images
- video
- audio
- interactive elements

---

### citation

**Definition:** A transclusion reference rendered inline, often as a superscript, which can expand into embedded referenced content.

**Important rule:** citations are not sidebar-native in the target UX.

---

## Terms To Avoid As Primary Names

Use these only when talking about history or compatibility:

- `etext`
- writer
- run
- task
- supervisor
- terminal agent
- Factory Droid / factory workflows as an architectural reference

Preferred replacements:

- `vtext`
- `vtext agent`
- trajectory
- loop
- `super`
- `conductor`

---

## Canonical Short Summary

If we need the shortest consistent language:

- top-level input goes to `conductor`
- one prompt-bar request starts one `trajectory`
- `conductor` usually spawns `vtext`
- the user prompt becomes `v0`
- the conductor's framing note becomes `v1`
- `vtext` spawns workers like `researcher` or `super` as needed
- workers send structured updates back over coagent tools
- those worker and appagent executions are `loops` inside the trajectory
- the `vtext` agent writes new canonical versions
- users can always edit and hit Revise to create a new user-authored version
