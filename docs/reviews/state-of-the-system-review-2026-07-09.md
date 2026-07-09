# State of the System Review — 2026-07-09

**Date:** 2026-07-09
**Reviewer:** Claude Code architecture-review session (four parallel read-only
surveys: backend/runtime, user-facing surfaces, autopaper forensics,
doctrine/mission corpus).
**Authority:** review/support only. `docs/choir-doctrine.md` (apex) and the
active umbrella mission `docs/definitions/og-dolt-heresy-completion-2026-07-08.md`
win on any conflict. No code changes were made.
**Prompted by:** owner request — "so much has happened recently, I feel like
we're losing the plot… take stock and understand where we are," with the goal
of moving to self-development (developing Choir with the choir CLI and the
choir UI).

---

## Verdict in one paragraph

The plot is intact, and as of 2026-07-09 it is more coherent than it felt on
2026-07-07. The "losing the plot" sensation was already correctly diagnosed in
`docs/assessment-overall-state-2026-07-07.md`: the mismatch is **narrative
drift between docs and code, not missing functionality**. The pipeline the
owner believes "never worked" (corpusd, sourcecycled) is in fact deployed,
healthy, and fetching; what has never worked end-to-end since June 27 is
**article production**, and the outage is substrate (the World Wire platform
VM's embedded-Dolt-in-Firecracker boot loop plus a proxy/vmctl timeout
mismatch), not pipeline code. The ~12 refactors mostly aimed at the wrong
layer. The response — collapsing ~25 open mission edges into one governed
umbrella mission with falsifiable definitions — is the right shape, and
Phase A of that mission cleared its exit panel on 2026-07-09. The system sits
between autonomy Level 3 and Level 4; Level 5 ("Choir-in-Choir"
self-development) is gated on three concrete things: enforcing heresy
detectors (Phase B), deleting the dual paths (Phases B–C), and a load-bearing
promotion-over-ComputerVersion path (Phase D). The last open decision node,
**D-STORE**, was settled by the owner on 2026-07-09: all in on Dolt; the six
storage-inventory questions are now Phase C/D verification tasks, not gates.

---

## 1. The plot, reconstructed (era timeline)

Long-running mission *forms* (how autonomous work is authored):

| Era | Form | Status |
| --- | --- | --- |
| MissionGradient | compass + invariants + belief-state updates | legacy baseline |
| Parallax | paradoc: conjecture / witness / variant / ledger / settlement | legacy reference |
| **Definition** | executable definition graph run via `/goal <doc>.md` | **current** |

Campaign chronology (evidence-dated):

- **2026-06-04** — source/publication data contract frozen
  (`docs/source-external-data-publication.md`).
- **2026-06-11** — durable-actor rearchitecture portfolio
  (`docs/mission-portfolio-2026-06-11.md`); `internal/actor` built against
  `specs/actor_protocol.tla`.
- **2026-06-22/23** — source-centric `update_coagent` deletion; source
  entities become native object-graph objects; 12 QA passes chasing
  rendering defects; root cause repeatedly `H_deploy` (staging ran stale
  code).
- **2026-06-26** — overnight autoradio mission: 22 hours, ~228 commits,
  ~10.8k LOC; the entire wire pipeline built; ended "WORKING (V=8), not
  settled" after 9 repair-deploy-discover cycles
  (`docs/mission-overnight-autoradio-platform-checklist-v0-report-2026-06-26.md`).
- **2026-06-27** — last day new articles were produced. `road-ahead`
  sequencing doc: runtime refactor → mutation hardening → **choir-in-choir**.
- **2026-06-29** — news-live mission; edition-alias bootstrap fix; the
  `choir` CLI born (Track D).
- **2026-06-30** — problem doc opened: platform VM unhealthy, no new
  articles (`docs/problem-platform-vm-unhealthy-no-new-articles-v0.md`).
- **2026-07-03/04** — "autoputer before autopaper" pivot; autopaper tabled;
  substrate-independent audited computer (SIAC) definition adopted:
  **ComputerVersion = (CodeRef, ArtifactProgramRef)**, VMs are merely
  materializers.
- **2026-07-07** — `assessment-overall-state-2026-07-07.md`: the pivotal
  self-diagnosis (see §4).
- **2026-07-08** — current umbrella mission
  `docs/definitions/og-dolt-heresy-completion-2026-07-08.md` supersedes and
  absorbs the open edges; Universal Wire → World Wire (narrative rename).
- **2026-07-09** — Phase A exit gate cleared after five panel rounds
  (commit `21b159b`); W2 timeout hardening staging-proven
  (`docs/evidence/w2-timeout-staging-proof-2026-07-09.md`).

**Current position: umbrella mission `og-dolt-heresy-completion-2026-07-08`,
Phase A complete, entering Phase B.**

---

## 2. System map (what actually runs)

Production topology (`nix/node-b.nix`; dev mirror in `start-services.sh`):

```
                        Caddy :443
                          │
        ┌─────────────────┼──────────────────────┐
        │                 │                      │
   static frontend    proxy :8082            maild :8087
   (web desktop)      auth-gating reverse proxy
                          │
        ┌────────────┬────┴──────────┬───────────────┐
        │            │               │               │
   auth :8081    vmctl :8083     corpusd :8086   (route resolver:
   JWT/API keys  Firecracker VM  platform Dolt    still hard-codes
        │        lifecycle       object graph +   platform VM identity
   gateway :8084     │           publication      = heresy H031)
   LLM broker        ▼               ▲
   (creds stay   Firecracker VM      │ platform-dolt sql-server :13306
    off guest)   └─ sandbox :8085    │
                    actorruntime +   │
                    internal/runtime │
                         ▲           │
                         │ vmctl UDS │
                 sourcecycled :8787 ─┘
                 RSS/Telegram/GDELT ingestion
```

Deployed binaries (packaged in `flake.nix`, systemd on node-b): `auth`,
`proxy`, `vmctl`, `gateway`, `sandbox`, `corpusd`, `sourcecycled`, `maild`,
`maildctl`, `zot`. Everything else in `cmd/` compiles in CI but is tooling,
dev harness, or not yet integrated.

---

## 3. Component inventory (the names the owner uses)

| Component | What it actually is | Status |
| --- | --- | --- |
| **autoputer** | The persistent owner computer. Ontology term + three green TLA+ specs (`specs/autoputer_lifecycle.tla` et al.) + promotion contract code. | **Not a binary.** The sandbox→autoputer rename never happened; `cmd/autoputer` does not exist. Conjectures C-C1..C-C4 open. |
| **choir CLI** (`cmd/choir`) | Phase-1 HTTP client over `/api/*` with `choir_sk_` keys. Verbs: `run start/status`, `trajectories`, `texture read/history/revisions`, `search`, `wire`, `api-key`. | **Working** for submit/read/observe. No write, no work-item, no fork/promote/rollback verbs (deliberately gated on the promotion spec + staging proof). |
| **choir base** (`internal/base`, `cmd/baseharness`/`basecompare`/`baseobserve`) | Content-addressed substrate: journal + blob store + tree + planner + `/api/base/` API, fully tested; consumed by ~40 `base_*_contract.go` files in `internal/computerversion` for promotion evidence. | **Dev-complete, not deployed.** No production daemon serves it; "partially built" is accurate only at the integration layer. |
| **autopaper** | Not a service — the *publication output* of the autoputer: sources → agents → Texture drafts → editions (definition doc 2026-07-03). | **Tabled by owner decision 2026-07-04**, pending autoputer/substrate health. Pipeline code is complete and CI-green. |
| **corpusd** (`cmd/corpusd`) | 53-line shell over `internal/platform` + `internal/objectgraph` (both substantial and tested). Platform Dolt service. | **Deployed and healthy** (640 artifact manifests, 640 blobs, 103 texture docs). Not the failure. |
| **sourcecycled** (`cmd/sourcecycled`) | 1,384-line timer daemon (RSS/Telegram/GDELT) dispatching into the active VM via the vmctl socket. In-memory store; state wiped on restart. | **Deployed and fetching** (22 RSS + 25 Telegram per cycle) but downstream-dead: dispatch is a silent no-op whenever the platform VM chain is down. |
| **World Wire platform VM** | The materialized computer that synthesizes/publishes editions (`internal/runtime/wire_*.go`, `universal_wire.go` run inside it). | **Down.** 1048+ reboots, `recovery_failed` since Jul 3, no new articles since Jun 27. Second embedded-Dolt-in-Firecracker corruption incident in a month; ext4 errors on `data.img`. This — not pipeline code — is the outage. |
| **actor runtime** (`internal/actor`, `internal/actorruntime`) | Durable-actor concurrency core, spec-backed, wired into `cmd/sandbox` with **no legacy fallback** (`adapter.go:202`). | **Cut over.** But business logic still lives in `internal/runtime` (~106k LOC, 3,746-line `runtime.go`) — the live application layer, not a zombie. |
| **capsules / Nucleus** (`internal/capsule`, `cmd/capsule-host`/`capsule-broker`) | Effect-chamber runtime ("capsule runtime v14" is the squash-base commit). | **Newest code, zero deployment wiring.** Built by CI only. |
| **frontend web desktop** (`frontend/`) | Svelte SPA, registry-first app model, 20 apps, WebAuthn/passkey auth, 70 Playwright specs. Matches its spec doc (`docs/frontend-app-building-api.md`) — a rare no-drift zone. | **Most polished surface.** Gaps: no trajectory/trace app, no work/mission UI. |
| **Texture app (web)** | Versioned-artifact editor with history/compare/source panel; 25+ dedicated specs. | **Working; daily-driver quality.** |
| **Mac desktop app** (`cmd/desktop`, Wails v3 alpha) | Wrapper embedding `frontend/dist`; cloud mode (choir.news) or local mode (spawns full stack); Safari-bridge WebAuthn; real Base sync engine in `internal/desktop`. | **Real but unshipped** dev build. |
| **ChoirFileProvider** (`macos/`) | Finder File Provider extension over a Unix-socket bridge to the Go sync engine. | **Stub-to-partial**; manual Xcode build; desktop app runs fine without it. |
| **MCP server** | Designed in `docs/design-choir-headless-surface-v0.md`. | **Not built.** |

---

## 4. The autopaper question, answered

> "corpusd and sourcecycled never really worked. We did like 12 refactors to
> make them work and where are we now?"

The refactors were real (06-04 contract → 06-11 actor substrate → 06-22/23
source-entity migration → 06-26 overnight build ×9 repair cycles → 06-29
news-live → 07-03/04 spec-first pivot → 07-08 umbrella). But the 07-07
assessment's finding stands up to this review's evidence:

1. **corpusd works.** Host-side platform Dolt is healthy; corpusd is a thin
   shell over tested libraries.
2. **sourcecycled works** as far as its own remit: it fetches on schedule.
   Its projection into the object graph is a **silent no-op when the platform
   VM chain is down** (`cmd/sourcecycled/main.go:283-288`), and it reports
   healthy anyway — which is exactly why it *felt* like it never worked.
3. The wire pipeline code, after five generations, is now **one coherent
   CI-green generation** living in `internal/runtime/wire_*.go` +
   `internal/wirepublish`.
4. The actual production outage is **(a)** the platform VM's embedded Dolt
   corrupting under Firecracker (twice in a month) and **(b)** the
   180s-client/10s-proxy timeout mismatch turning every hang into a 502.
   (b) was fixed 2026-07-09 (W2: bounded 60s path, fast 504, staging-proven).
   (a) is open: doctrine says rebuild the embedded Dolt from
   `corpusd head + code ref + blob root`, do not fsck a third time.
5. Three load-bearing premises of the July autoputer mission were
   **inverted** (actor runtime already fully wired; `internal/runtime` is the
   live app layer, not deletable dead code; object graph is additive, not a
   migration). The rename and deletion work planned on those premises never
   happened — correctly so.

**Path back to live articles** (per the umbrella mission): request-path
hardening (done) → route over ComputerVersion instead of VM identity (H031,
Phase D) → rebuild the platform computer's embedded Dolt from corpusd →
**one staging proof that an edition publishes end-to-end** (conjecture C-B2,
open). The world-wire store's current data is declared junk; it stands up
fresh, no migration.

---

## 5. Self-development readiness (the owner's stated goal)

Autonomy ladder position: **between Level 3 and Level 4.** Staging is the
acceptance environment (L3); the L4 invariant layer (heresy detectors) exists
but runs discovery-only in CI — it reports, it does not catch.

**What the loop looks like today, concretely:**

- Working now: `choir run start "<prompt>"` → conductor routes to an
  appagent → `choir texture revisions <doc>` reads the output (~10s) →
  `choir trajectories` shows causality. The web desktop's prompt bar drives
  the same path; the Texture app is a real editor; Super Console gives an
  authenticated PTY.
- The realistic daily driver for developing Choir *in* Choir today is:
  **choir CLI (submit/observe) + web Texture app (read/edit) + prompt bar.**

**The gaps, mapped to where they're already scheduled:**

| Gap | Blocks | Scheduled |
| --- | --- | --- |
| Heresy detectors report-only (no fail-on-regression) | L4 | Phase B/E |
| Dual paths live: parent/child (H001–05), continuations (H006–08), texture-forcing (H009–24b) | trustworthy substrate | Phases B–C kill waves |
| `DoltPromotionAdapter` has zero `cmd/` callers — no binary can execute an atomic route-flip promotion | the entire back half of the self-dev loop | Phase D |
| Proxy route resolver hard-codes platform VM identity (H031) | routing over ComputerVersion | Phase D |
| Retrieval search returns 0 for terms that exist ("the audited computer can't find its own evidence") | agent self-orientation | Phase E |
| No trajectory/trace app, no work/mission app in the web desktop | UI half of self-dev | unscheduled |
| CLI has no write/lifecycle/work-item verbs; MCP server unbuilt | headless half of self-dev | design doc gates lifecycle verbs on promotion proof |
| `internal/runtime` god object; per-app extraction unbuilt | parallel candidate-VM development | road-ahead critical path |
| ~~D-STORE storage-fork decision~~ **settled 2026-07-09: all in on Dolt** | — | six inventory questions → Phase C/D verification |

**Assessment:** the self-development pipeline in `README.md` (prompt →
conductor → capsule against a forked ComputerVersion → AppChangePackage →
verifier evidence → promotion/rollback) is implemented up through "capsule"
only as unwired code, and from "promotion" only as contracts + a green TLA+
spec. The front third is live; the back two-thirds are the umbrella mission's
Phases B–D. This review found no evidence contradicting that sequencing.

---

## 6. Standing risks and debt

1. **Platform VM substrate** — the only recurring production-severity
   failure class (2 embedded-Dolt corruption incidents in a month). SIAC is
   the correct response; until a materializer can be rebuilt from
   `(CodeRef, ArtifactProgramRef)` on demand, every VM is a pet.
2. **Promotion path inert** — highest-value capability, unproven end-to-end.
3. **Detectors don't enforce** — 9 live heresy clusters, zero at
   fail-on-regression.
4. **Corpus integrity** — `checkpoint_incomplete` ledgers exist; the
   umbrella mission forbids citing them as complete. Doc truth discipline is
   improving (doccheck in CI) but the drift class that caused the July 7
   crisis is not yet mechanically prevented.
5. **Squashed history** — the repo base is a single squash commit
   (`138626e "capsule runtime v14"`); forensic reconstruction now depends
   entirely on the docs/ledger corpus. That corpus carried this review — it
   is worth its cost.
6. **Debris** — 11 throwaway `.mjs` scripts at `frontend/` root;
   `transfer-cookies.mjs` embeds a real-looking `choir_refresh` token value
   and should be deleted (and that token rotated if ever live).
   `internal/sourcegraph/web_capture_graph.go` duplicates
   `internal/cycle/web_capture_graph.go` — planned deletion never executed.
7. **Spec staleness** — `specs/README.md` says the actor spec is "COMING"
   while `specs/actor_protocol.tla` exists and model-checks; only the
   promotion spec is current relative to code.

---

## 7. Recommendations (non-binding)

1. **~~Answer D-STORE~~ — done.** Settled by owner statement 2026-07-09
   (all in on Dolt); recorded in the umbrella mission doc. The six
   storage-inventory questions are now verification tasks inside Phase C/D.
2. **Stay on the umbrella mission's phase order.** Phase B (detector
   enforcement + first kill wave) directly converts "losing the plot" energy
   into mechanical guarantees.
3. **Treat the platform-computer rebuild as the first real SIAC exercise:**
   rebuilding the World Wire VM from `corpusd head + code ref + blob root`
   is both the article-production fix and the first proof that a computer is
   its ComputerVersion, not its disk image.
4. **For self-development ergonomics, two small high-leverage additions**
   (neither currently scheduled): a read-only Trajectory app in the web
   desktop (the data already flows through `/api/trajectories`), and a
   `choir` verb for work items/missions. Both are observe-side and do not
   collide with the gated lifecycle verbs.
5. **Delete the frontend root `.mjs` debris** and rotate the leaked refresh
   token if it was ever valid.
6. **Fix `specs/README.md`** to describe the specs that actually exist.
