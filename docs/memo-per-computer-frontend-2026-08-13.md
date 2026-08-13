# Memo: Per-Computer Frontend

**Status:** architecture synthesis; invariant asserted 2026-08-13
**Mutation class:** green (documentation only)
**Scope:** computer surface, restore set, serving join, and self-development of UI

## Thesis

A Choir computer includes the UI that renders that computer. When a user, or
that user's agents, builds any change, the change is scoped to that
`ComputerID` — source, guest release, VM-local state, **and frontend**.

A host-global SPA is not a computer. It is a shared platform accident. Treating
it as the computer's UI makes self-development of the surface either a platform
deploy (every computer sees it) or a no-op (the browser never reads the
bundle). Both fail the computer.

```text
User-authored change
  -> capsule freeze bound to ComputerID + base event head
  -> acceptance
  -> guest materialization of release, including computer-surface UI
  -> checkpoint that can derive the served bytes
  -> route CAS only after a serving join
  -> that computer's browser reads those bytes
```

No other computer observes the unpublished change. Restore of this computer
restages this UI at that head. A later platform deploy does not overwrite it.

## The Invariant

1. **User-authored frontend is computer surface.** Desktop shell, Texture,
   apps, Settings computer panes, and the hashed asset graph that render a
   computer belong to that computer.
2. **The same candidate pipeline applies.** Frontend source, offline SPA
   recipe, and runtime artifacts freeze into the capsule effect bundle, accept
   through the canonical event, and materialize through the root guest updater.
   CI pointer rotation is not a computer change.
3. **Restore restages that surface.** Returning computer X to head H serves H's
   UI for X, not today's host `frontend-current`.
4. **Divergence is allowed.** Computer Y may keep a different UI at the same
   wall-clock time. That is what "the computer can change itself" means for
   surfaces the user sees.
5. **A digest without a serving join fails the invariant.** Recording a
   frontend SHA on a checkpoint while Caddy still serves one host tree does not
   make the frontend per-computer.

Thin platform shell may remain host-global: TLS, Caddy bootstrap, `/auth/*`,
computer-picker chrome, proxy/vmctl/corpusd, NixOS host. Those are control
plane. They are not Texture, Desktop, apps, or Settings.

## Why This Is Not Optional

Choir's object is a persistent computer that may diverge from the platform
baseline: apps, agents, files, builds, Dolt state, prompts, runtime services,
and preferences already named in the computer ontology. The UI is how a person
uses that object. If the UI cannot diverge, the computer cannot change what
the user actually operates.

Consequences of a shared SPA:

- A capsule that authors Solitaire UI, a Texture renderer, or a Settings pane
  either lands where every computer's browser reads it, or lands inside the
  guest where no browser reads it. Current staging does the second.
- Restore of guest release and VM-local state returns APIs and rows, then
  paints them with whatever SPA CI last installed. That is not restoring the
  computer the user uses.
- Two computers cannot show two UIs. `desktop_id` is a query param on one
  origin; switching computers changes API routing, not the asset graph.
- Self-development of UI becomes a GitHub-main → Nix `.#frontend` →
  `install_frontend_pointer` host deploy. That path has no `ComputerID`, no
  capsule acceptance, and no restore.

The 2026-08-11 fork — "treat host frontend as platform control-plane **or**
fold it into a frontend-in-release successor" — is closed. The first arm is
not product ontology. The host SPA as today's serving fact remains true. It is
non-conformance, not destination.

## Current Non-Conformance

Live architecture is **host-global SPA + per-computer API**. Evidence:

- Caddy on Node B serves `/var/www/go-choir/frontend-current` with no
  `ComputerID` in the static path (`nix/node-b.nix:23-25,194-207`).
- `/assets/*` falls back to `frontend-previous`; `/*` is SPA fallback to
  `index.html`. Both are host directories.
- The proxy registers `/api/`, `/health`, and internal routes
  (`internal/proxy/handlers.go:1547-1551`). Authenticated `/api/*` reverse-
  proxies to a vmctl-resolved guest (`internal/proxy/handlers.go:653-677`).
  That pins which guest answers the API, not which `index.html` Caddy serves.
- Guest images have no frontend, `/var/www`, or Caddy
  (`nix/autoputer-vm.nix:654-656`). Capsule freeze copies only
  `var/lib/artifact/release/**` (`internal/capsule/executor.go:1086-1110`).
- Frontend updates are GitHub main → CI `.#frontend` → host pointer swap
  (`.github/workflows/ci.yml:976-1006`), classified independently of guest
  image refresh (`.github/scripts/deploy-impact-classify:156-158`).
- Production CodeClosure for self-dev is one artifact,
  `capsule-effect-bundle.json`; ArtifactProgram kind is
  `capsule_effect_bundle`
  (`internal/agentcore/self_development_materializer.go:162-175`).
- `ComputerVersion` is only `(CodeRef, ArtifactProgramRef)`
  (`internal/computerversion/types.go:40-43`). `ReconstructionDigest` hashes
  that version, effective head, and guest release digest
  (`internal/agentcore/self_development_materializer.go:323-327`). No frontend
  identity. No `ContentWitness` field.
- vmctl CAS projects `ComputerVersion` onto `computer:{owner}:{computer}`
  (`internal/vmctl/self_development_route.go:16-73`;
  `internal/routeledger/ledger.go:94-103`). No frontend join.
- Guest health probes marker, schema, reducer, and release digest
  (`internal/updater/runtime.go:155-171`), not served SPA bytes.
- `restorePrior` swaps the guest `current` pointer
  (`internal/updater/updater.go:554-571`). Caddy is untouched.
- Desktop embeds a second global `frontend/dist`
  (`cmd/desktop/main.go:24-25`). CI ignores `cmd/desktop/*` for Node B deploy.

The active effects Definition already records this as live fact and forbids
shipping UI in the current candidate, because the browser would never read it
(`docs/definitions/choir-supervised-self-development-effects-2026-08-11.md:36-37,537-538`).
That forbid is correct for **this** candidate. It is not permission to leave
the UI outside the computer.

Coupling break cases under current serving, if only the guest restores to head
H while the browser keeps today's SPA:

- Source contract generated from
  `internal/sourcecontract/source_contract_schema.json`
  (`frontend/scripts/generate-source-contract.mjs:8-9`).
- Texture structured renderer requires `choir.texture_doc.v1`
  (`frontend/src/lib/texture-structured-editor-doc.ts:3,230-231`).
- Unknown persisted `app_id` fails closed to "Unknown app"
  (`frontend/src/lib/AppHost.svelte:84-87`).
- SPA stamps `window.__CHOIR_BUILD__` from the Vite SHA
  (`frontend/src/lib/build-info.js:1-13`; `flake.nix:147-149`). Restoring a
  computer does not change that SHA.
- Computer selector is a query param on the same origin
  (`frontend/src/lib/desktop-selector.js:8-13`).

## How The Frontend Needs To Be Per-Computer

Exact APIs, directory layout, and Caddy stanzas belong in a successor
Definition. This memo names the envelope that Definition must satisfy.

### Split the surface

**Control plane (platform, OUT of restore)**

TLS, Caddy bootstrap, `/auth/*`, computer-picker chrome, proxy, vmctl,
corpusd, NixOS host. Documented as host software. May version independently of
any one computer.

**Computer surface (IN restore)**

Everything that renders that computer's state: Desktop, Texture, apps,
Settings computer panes, and the asset graph. Authored in a capsule. Frozen as
bundle files. Accepted. Staged by the root guest updater into that computer's
realization.

### Authoring and freeze

A computer-surface change is an ordinary capsule effect:

- `SourceTreeRef` includes frontend source that will render this computer.
- `BuildRecipeRef` is an offline SPA recipe executed in the capsule, not a
  host Nix `buildNpmPackage` side channel.
- `RuntimeArtifactRef` names `index.html` plus content-hashed assets.
- `RuntimeFiles` carry those bytes under the release prefix the updater
  already stages.
- Test and toolchain receipts bind the same capsule execution.

`CapsuleEffectBundle` already has those fields
(`internal/capsule/transaction/builder.go:26-50`). Freeze already copies
`var/lib/artifact/release/**`. The missing object is not a new bundle field.
The missing object is a **serving hop** that reads those files for that
computer.

### Serving hop

After materialization, the browser that is using computer X must receive X's
`index.html` and assets. Three topologies can satisfy that; the successor
Definition picks one and proves it.

1. **Guest static.** The guest serves the staged SPA. The proxy reverse-proxies
   HTML/assets to the vmctl-resolved guest the same way it already reverse-
   proxies `/api/*`.
2. **Host staging keyed by computer.** The updater (or a trusted host actuator
   driven by the same materialization receipt) writes a tree keyed by
   `ComputerID` + checkpoint digest. Caddy/proxy selects that tree only after
   route resolve, never `frontend-current`.
3. **Encapsulated origin.** Platform shell stays on the host origin. Computer
   UI loads from a per-computer origin or iframe whose bytes are the staged
   SPA. A versioned bootstrap API is the only contract between shell and
   surface.

Any of these is a serving join. None of them is "hash the SPA and keep serving
`/var/www/go-choir/frontend-current`."

### Checkpoint and fail-closed creation

A checkpoint that cannot derive the served UI is not a checkpoint of the
computer the user uses. Bind:

- existing: event heads, `CodeRef`, `ArtifactProgramRef`, guest `ReleaseDigest`
- required: frontend identity derivable from the release or an explicit
  frontend digest joined to it
- required later with the VM-local witness: content that the Definition
  already names for Dolt rows

Refuse creation when the served SPA is underivable. `ReconstructionDigest`
must change when the served UI changes. `ComputerVersion =
(CodeRef, ArtifactProgramRef)` remains code/artifact identity, not a complete
restore address; frontend identity is part of restore completeness, not a
third semantic writer.

### vmctl join

Route CAS greens only after the serving join: the bytes that would be served
for this computer equal the checkpoint binding. A green route that pairs an
old guest with a new host SPA, or a restored guest with today's CI SPA, is a
failed restore. Guest `/health` identity is not enough; health must cover
served SPA bytes or an equivalent materialization receipt.

### Restore

Documented restore already says: resolve checkpoint, quiesce, forward restore
intent, restage release, rebuild VM-local state, extractor match, route CAS
(`docs/definitions/choir-supervised-self-development-effects-2026-08-11.md:519-527`).
"Restage the release" must include the computer-surface SPA. Caddy/proxy must
point at that realization. A later main deploy must not move this computer
back onto `frontend-current`.

On failure keep the prior realization, including its UI. Partial never greens.

### Skew

The platform shell may be newer than a computer's UI. The contract is a
versioned bootstrap API or full encapsulation. Mismatch refuses the computer
route, keeps the prior realization, and never greens. A reversible-effect
policy cannot authorize an irreversible send merely because the shell is new.

### Authority

Semantic writer remains the guest `ComputerEventAppender`. Frontend staging is
materialization, not a new event log. Verifiers remain read-only. CI
`install_frontend_pointer` may continue to update **platform shell** assets
only after the split; it must not be the path that changes computer surface.

## False Solutions

These do not satisfy the invariant:

1. **Checkpoint SHA, same Caddy root.** The browser still reads one tree.
2. **`frontend/dist` in `RuntimeFiles`, still host-global Caddy.** Bytes sit
   in the guest; the user never sees them. This is the current Solitaire
   trap.
3. **Query-param computer on one SPA.** Routing the API is not routing the
   UI.
4. **Wails/desktop embed as the per-computer UI.** That is a third copy,
   compiled into `cmd/desktop`, ignored by Node B deploy, and outside Choir
   restore.
5. **Calling Desktop/Texture/apps "platform software"** so restore can ignore
   them. That deletes the computer's surface from the computer.
6. **Shipping UI in the current effects candidate** before the serving hop
   exists. The Definition's API-only forbid stays until a successor proves
   the hop.

## Sequencing

The active executable `/goal` remains
`choir-supervised-self-development-effects-2026-08-11`. Its candidate stays
API-only. Its restore proof stays VM-local + guest release until a successor
owns the serving envelope. Effects remain OFF until that Definition's own
gates pass.

Do **not** implement frontend-in-release as a local patch on the current
mission. The Caddy hop is host-global. Expanding this candidate would either
weaken the OUT claim without a serving proof, or smuggle a second deploy path
into restore.

A later owner-ratified Definition owns:

- control-plane vs computer-surface split in Caddy/proxy
- capsule SPA recipe and release layout
- checkpoint frontend identity and fail-closed creation
- vmctl serving join
- restore restage of UI
- skew policy
- deletion or quarantine of `install_frontend_pointer` as a computer-surface
  path

That Definition is sequenced after, or beside with disjoint surfaces from, the
effects proof. It is not concurrently the private-Go actor kernel.

## Current Non-Claims

This memo does not claim that:

- per-computer frontend serving is implemented;
- a checkpoint today binds UI bytes;
- guest `/health` covers the SPA;
- TLS, auth, NixOS, proxy, or vmctl themselves enter the restore set;
- frontend staging is a second canonical event writer;
- irreversible UI-adjacent effects require a human seat;
- the current effects Definition's API-only candidate is wrong for its own
  serving topology;
- recording `VITE_CHOIR_BUILD_SHA` is a serving join.

## Relationship To Doctrine

Owner direction 2026-08-13: this is invariant. The kernel claims are promoted
into Choir Doctrine (`C15`, `I25`), computer ontology restore-set language,
and agent product doctrine. This memo remains the architecture synthesis and
the evidence packet for the serving envelope. Implementation questions (which
of the three hops, exact schemas, resource limits) stay in the successor
Definition and cannot overrule doctrine.

The active effects Definition's `finish.completion_cutover.frontend-ownership`
item is decided: computer-surface frontend is per-computer, not platform
control-plane. Opening the successor and proving the serving join remain
cutover/successor work. Until that proof, do not teach "the whole computer
was put back" while the UI remains `frontend-current`.
