# Texture Live Supervision & Revision Surface Architecture
**Date:** 2026-08-20  
**Classification:** Green Architecture Doctrine  
**Authority:** `docs/choir-doctrine.md`, `docs/agent-product-doctrine.md`  

---

## 1. The Core Principle: Document-Driven Long-Horizon AI

Texture is not a chat interface, conversation history, or message broker. It is a live, collaborative, updating supervision and revision surface.

```
                  Exogenous Input (Prompt Bar / CLI)
                                 │
                                 ▼
                             Conductor
                       (Intake & Routing)
                                 │
                                 ▼
                              Texture
                (Living Document & Supervision Surface)
                  v0 ──► v1 ──► v2 ──► v3 ──► v4 ──► vn
                                 │
                        execution_request
                         Control Packet
                                 │
                                 ▼
                               Super
                       (Host-Side Governor)
                                 │
                          assign_co_super
                                 │
                                 ▼
                              CoSuper
                    (Disposable Capsule Actuator)
```

In Choir's product architecture:
1. **The unit of long-horizon work is a living document**, not an ephemeral conversational turn.
2. **Every trajectory state transition produces a new, self-contained document revision ($v_n$).**
3. **Citations open into transcluded content** (live source diffs, test receipts, candidate manifests, consensus verdicts) directly within the document viewer on `choir.news`.
4. **Supervision is observable in real-time**: Opening an active Texture document on `choir.news` allows human observers and other agents to follow the exact trajectory state, open work items, and verified evidence without reading raw log streams.

---

## 2. The Living Revision Progression Protocol

For an autonomous self-development trajectory, the document advances through discrete, human-readable revisions:

| Version | Lifecycle Stage | Content & Supervisory Evidence | Transcluded Citations |
| :--- | :--- | :--- | :--- |
| **$v_0$** | **User Intake** | Initial user prompt and objective as received by Conductor. | `[prompt:seed]` |
| **$v_1$** | **Scope & Trajectory Binding** | Formulated trajectory plan, target deliverables, and open work items. | `[trajectory:id]`, `[work:items]` |
| **$v_2$** | **Delegation & Capsule Provisioning** | Super acknowledges execution authority and opens CoSuper assignment. | `[capsule:id]`, `[binding:handle]` |
| **$v_3$** | **Authoring & Execution Evidence** | CoSuper reports work progress, modified file paths, and test execution receipts. | `[source:diff]`, `[test:receipts]` |
| **$v_4$** | **Bundle Freeze** | Immutable candidate bundle frozen with 5 required artifacts. | `[bundle:manifest]`, `[patch:content]` |
| **$v_5$** | **Qualified Consensus** | Multi-agent consensus panel evaluation under frozen decision policy. | `[consensus:votes]`, `[quorum:receipt]` |
| **$v_6$** | **Promotion / Acceptance** | Event appender commits promotion; new epoch and realization active. | `[event:head]`, `[epoch:receipt]` |
| **$v_7$** | **Verification / Falsification** | Post-promotion API/DB verification and test results. | `[verifier:logs]`, `[proof:receipt]` |
| **$v_8$** | **Restore / Supersession** | Candidate B supersession or acceptance-fenced restore back to baseline. | `[checkpoint:witness]`, `[restore:event]` |

---

## 3. Why Prior Trajectories Stalled at $v_1$

In prior runs on staging, the "Self-development supervision" document remained at $v_1$ because:
1. **Mailbox Chat Regression:** The runtime converted agent updates into chat messages (`buildCoagentUpdateUserMessages`) and dumped raw cancel reports as simulated user turns in model context, rather than issuing `ApplyTextureTurn` requests to update the Texture document.
2. **Missing Revision Publication on Agent State Changes:** When CoSuper progressed or failed, the error was handled entirely inside process-local tool loops without committing a supervisory document update.
3. **Fix:** Every major milestone in agent execution (assignment creation, test receipt generation, bundle freeze, consensus verdict) must call `ApplyTextureTurn` with a structured `Revision` record, ensuring the living document on `choir.news` immediately reflects the updated state.

---

## 4. The Self-Development Flywheel (Choir in Choir)

The long-term goal of Choir is the **automatic computer**: moving from changing Choir code in an external coding harness to using the `choir` CLI to have Choir VMs modify their own code autonomously.

```text
User / Developer                  Choir VM (Staging)
      │                                  │
      ├── choir run start ──────────────►│ Conductor Intake
      │   "implement feature X"          │       │
      │                                  │       ▼
      │                                  │ Texture Document Created (v0 -> v1)
      │                                  │       │
      │                                  │       ▼
      │                                  │ Super Orchestrates (v2)
      │                                  │       │
      │                                  │       ▼
      │                                  │ CoSuper Capsule Authors Code (v3)
      │                                  │       │
      │                                  │       ▼
      │                                  │ CoSuper Freezes Bundle (v4)
      │                                  │       │
      │                                  │       ▼
      │                                  │ Consensus Evaluates & Accepts (v5)
      │                                  │       │
      │                                  │       ▼
      │◄── Watch live on choir.news ─────┤ VM Promotes & Restarts Runtime (v6)
```

This creates a rapid, self-contained feedback loop where the user monitors live progress simply by opening the active Texture document in the browser, while the VM safely executes, tests, and promotes changes to its own environment.
