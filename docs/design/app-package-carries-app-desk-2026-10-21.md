# An app package carries its app desk (2026-10-21)

Status 2026-10-30: the desk manifest (`internal/deskmanifest`) and app packages have both landed; the "unbuilt" and "paused branch" statements below are stale.

Status: **design, green.** No code. SDC item ARCH-03
([security-density-architecture-checklist-2026-10-21.md](../security-density-architecture-checklist-2026-10-21.md)),
station [choir-launch-fp4-selfdev-architecture-2026-10-21.md](../definitions/choir-launch-fp4-selfdev-architecture-2026-10-21.md).
The red part (install registers a desk) **waits for the desk manifest of
ARCH-02**, which lane FP2 is building; this note names exactly what it needs
from that manifest so the two can meet, and what the red slice would be and how
it would be proved.

Owner direction: app agents are desks and "app agents get their own desks"
(owner notes §5, §36); doctrine I26 and CLAUDE.md "Desk Ontology" (an app desk
ships with its app package and is the sole agent writer of its app's object
kinds); the S7 addendum of 2026-10-21 in
[choir-appdev-s7-app-packages-2026-10-01.md](../archive/definitions/choir-appdev-s7-app-packages-2026-10-01.md);
owner ruling D1 (owner notes §23: whether a package is installed is a fact on
the computer's tape, not part of its release closure; install-time API version
check; install only from the host-built catalog until there is a sandboxed
frame).

## 1. What exists, and what is missing

- **The package manifest** (`choir.app-package/v1`) is on the paused branch
  `origin/feat/s7-0-app-packages` (4bc762102, 831 commits behind main;
  `internal/apppackage/manifest.go`, `docs/design/s7-0-app-packages-2026-10-14.md`).
  Fields: `schema, name, version, display, source, frontend, backend, build,
  data, restore, grants`. It has **no desk**. Package presence is the fold of
  `choir.app_package_event` objects on the computer's tape (D1, D2, D3 of that
  design); install takes a build digest, never bytes (D4); names held by a static
  app or the shell are reserved (`reservedNames`).
- **Desks are hardcoded** in five places (SDC ARCH-02):
  `internal/agentprofile/agentprofile.go`, `internal/agentcore/tool_profiles.go`,
  `internal/capsule/roles.go`, `internal/agentcore/inbox_records.go`,
  `internal/yaegikernel/choir.go`, plus the prompt overlays. ARCH-02 replaces that
  with one desk manifest table that confinement, role checks and budgets read.
- **Per-kind writers** now exist in the object graph (SDC ARCH-04, this lane,
  commit 5b4578e73): `KindRegistration.Writer`, a closed set of built-in writers
  and the pattern `desk:<name>` for an app desk, `RegisterKindChecked` (one valid
  writer per kind; no second writer for a registered kind), and a write gate that
  checks a declared writer.
- **Grants**: S4 defines the egress grant object; the S7 addendum says the
  package manifest declares grants, the owner approves at install, and a widened
  grant on upgrade needs a visible-diff re-approval. Those grants attach to the
  desk (SDC NET-10: app-agent ingress and egress in the desk manifest).

What is missing is the join: nothing says that installing a package registers its
desk, its object kinds and its grants as one act, or that uninstalling retires
them.

## 2. The design in one page

1. A package manifest gains an optional **`desk`** member: a desk manifest
   (ARCH-02's schema, embedded by value, not a second schema). A package with no
   `desk` is a window and an optional service with no agent. A package with a
   desk is an *app agent*.
2. **Install is a tape fact** (D1) and registers four things in one act: the
   package, the desk, the desk's object kinds (each with the desk as its one
   writer) and the desk's grants. The install event binds the **digest of the
   desk manifest** along with the build digest, so what runs is what the owner
   approved (the same discipline as the S6 gate).
3. **Nothing is registered in a writable table.** The desk set, the package kinds
   and the grants are all **derived at boot, and on every install event, from the
   fold of install events on the tape** (single authority; a pinned-head restore
   selects them by replay). The static registries (`DefaultRegistry`, the system
   desks) stay as they are and stay reviewable. The write gate and the daemon read
   `static ∪ derived`, where the derived part is a read-only overlay.
4. **A package desk is least-authority by construction.** It has no capsule,
   host, provider-routing, event-chain or promotion authority (the limits
   doctrine gives Texture); it reaches other desks only by semantic act (`Cast`,
   `Report`) through its own inbox; it writes only its own kinds; it reaches the
   network only through its approved grants; it never holds the engineering slot
   (it asks for engineering through management's admission, which is FIFO,
   ARCH-10).
5. **Uninstall retires.** Disable closes the desk's open obligations with the
   visible fate "retired", stops its worker class, and makes its kinds
   **unwritable** (the writer is retired) but readable. Purge deletes or retains
   the kinds' data per the manifest's `restore.on_purge` (`delete_data` or
   `retain_data`). A retired desk never resumes: a crash boot closes nothing new
   and re-derives the live set from the tape.

## 3. What the red slice needs from ARCH-02's desk manifest

The desk manifest is FP2's. The fields below are the ones this note depends on,
named as the SDC lists them (ARCH-02), with the property each needs. If FP2's
schema names them differently, only this table changes.

| Field | Why a package desk needs it | Property required |
|---|---|---|
| `desk_id` | the desk's identity and its writer name | grammar `desk:<name>`; `<name>` equals the package name; never one of `texture`, `management`, `engineering`, `research`; reserved names (the S7 `reservedNames` list) refused |
| `owned_object_kinds` | the kinds the desk alone writes | each kind namespaced `choir.app.<package>.<kind>`, so it can never collide with a built-in kind; each carries `owner_edits` (does the owner's own edit path also write it); registered through `RegisterKindChecked` |
| `verbs` | the `choir.*` module surface the desk's cells get | a closed list drawn from the app-desk verb set (no `Freeze`, `Verify`, `Complete`, host, capsule or provider verbs); the daemon refuses a cell calling a verb outside the list |
| `role_table` | per-frame-action role for the daemon's check (SEC-02) | derived from `desk_id` plus `verbs`; the package cannot supply a role that names a system desk |
| `sandbox_profile`, `taint_class` | confinement and worker pooling (SEC-01, ARCH-05, ARCH-06) | for a package-origin desk the **floor is the weakest profile and an untrusted taint class**; a manifest may ask for stricter, never weaker; no worker shared with a privileged desk |
| `egress_verbs` | the network the desk may use (NET-10) | exactly the grants the owner approved at install; a widened grant is a new install event needing re-approval |
| `compute_class`, `memory_class` | budgets (RES-01 to RES-04) | capped by the desk-class maximum; a package may not exceed it |
| `model_policy` | which models the desk's cells use | from the closed policy overlays; a package names an overlay, never a provider or a key |
| `addressing` | the desk's inbox name and routing | `<package>:<instance>` style, inside the package's own namespace; cannot shadow a system inbox |
| `wake_dormancy`, `restart_contract` | CLAUDE.md "Restarts End Work" | the contract is fixed by the platform for package desks: a crash closes the desk's open obligations "interrupted by a restart"; a planned restart resumes only through the marker; the manifest cannot opt out |

And three entry points on the manifest package, so install is a pure
validation followed by one write:

1. `Validate(manifest, platform) -> []Refusal`: pure, typed refusals (reserved
   id, un-namespaced kind, verb outside the app set, weaker-than-floor profile,
   ungranted egress). Install calls it before it appends anything.
2. `Derive(installEvents) -> DeskSet`: the fold that boot and every install event
   run. It is the only way the daemon learns a package desk exists.
3. `PackageOrigin(desk) -> bool`: the daemon applies the package trust floor to
   a desk that came from a package, with no flag the package can clear.

## 4. Install, step by step (the red slice)

1. The catalog entry is **host-built** from published source (D1 condition);
   install takes the build digest.
2. API version check (D1 condition): the manifest's declared frontend, backend
   and desk ABI versions are in the set this guest speaks.
3. `Validate` the package manifest and the embedded desk manifest. Any refusal
   ends the install; nothing is written.
4. **Owner approval at install** shows the desk's id, kinds, verbs, egress grants
   and classes. Under standing consensus on a test computer the existing
   consensus path stands in; the paper and the owner's computer stay
   propose-only (the S6 rule).
5. Append the install event: `{package, build digest, desk manifest digest,
   approved grants, seq, prev}` on the existing hash chain. It is a tape fact; no
   second head.
6. `Derive` runs: the desk set, the package kinds (with writer `desk:<name>`) and
   the grants update from the fold. The gate's registry overlay and the daemon's
   desk table are rebuilt from the same fold. A crash between 5 and 6 is
   harmless: boot runs `Derive`.
7. The desk's worker class and inbox come up lazily on first wake, never at boot
   (no boot-time resumer).

Uninstall mirrors it: a disable event, `Derive` drops the desk from the live set
and marks its kinds retired, open obligations close "retired".

## 5. Failure modes, written before any test (for the red slice)

1. **A package takes a system desk's name or a built-in kind.** `Validate`
   refuses reserved ids; kinds must be `choir.app.<package>.*`; `RegisterKindChecked`
   refuses a second writer. Test: a manifest naming `texture`, a manifest owning
   `choir.texture_revision`, both refused before any write.
2. **The package grants itself authority.** Verbs outside the app set, a profile
   weaker than the floor, egress beyond the approved grants. Test: each refused at
   `Validate`, and a cell calling an unlisted verb refused by the daemon at run time.
3. **What runs is not what was approved.** The desk manifest digest in the install
   event must equal the digest of the manifest the daemon loads. Test: a mutated
   desk manifest under an unchanged build digest refuses to load.
4. **A second authority for the desk set.** No writable desk table, no file the
   guest can edit. Test: delete every cache, boot, and `Derive` rebuilds the same
   set from the tape; a restore to a pre-install head has no such desk.
5. **A retired desk writes.** After disable, a write declaring `desk:<name>` is
   refused by the gate in enforce and counted in observe (the retired writer is
   a writer-gate violation, not a missing kind). Test: write after disable.
6. **Two packages claim one name or one kind.** The install event chain is
   ordered; the second is refused. Test: concurrent installs of the same name,
   exactly one wins.
7. **Restart.** A crash after install and before the desk's first wake: boot
   re-derives, no obligation exists, nothing resumes. A crash with open
   obligations: closed "interrupted by a restart". Test: the existing crash-boot
   tests extended to a package desk.
8. **A package desk starves the platform.** It cannot hold the engineering slot or
   a system desk's worker; memory and compute classes are capped by the desk-class
   maximum. Test: a package desk at its cap does not change a system desk's
   admission.
9. **Taint crosses a worker.** A package desk's worker is never shared with a
   privileged desk (ARCH-05, ARCH-06). Test: the worker class key differs.
10. **Egress widens silently on upgrade.** A new build whose desk manifest widens a
    grant is a new install event needing re-approval with a visible diff (S7
    addendum). Test: an upgrade with a wider grant stays pending.

## 6. What this lane does not do, and the order

- No ARCH-03 code until FP2's desk manifest lands: the install path would have to
  guess its schema, and a guessed schema is a second desk table.
- No package-kind overlay in the write gate yet: ARCH-04 deliberately left the
  global registry static. The overlay is part of the red slice, read-only and
  derived from the fold, behind the same observe-then-enforce flip.
- The paused S7-0 branch (`feat/s7-0-app-packages`, `internal/apppackage`) is
  the base of the package half. Its manifest needs the `desk` member (optional,
  embedded) and its `Reduce` needs the desk set as a second fold. It is 831
  commits behind main and is rebased by the S7-0 station, not here.
- Order: FP2's manifest, then S7-0 on a rebased branch, then this slice, then
  the first app desk (the podcast and reader app agents, launch item L3-03).

## 7. Open for the owner (defaults in force, no blocking ask)

1. Embed the desk manifest in the package manifest, or reference it by digest.
   Default: **embed** (one document, one digest, nothing to fetch at install).
2. Namespace app kinds as `choir.app.<package>.<kind>`. Default: **yes**; the
   existing kinds keep their names, and a first-party app converted to a package
   (S7-0) keeps its kinds under its current names through an explicit
   `legacy_kinds` list that names them one by one, validated against the static
   registry, never a prefix.
3. A first-party package desk (Texture itself) is a **system-tier package desk**
   that ships in the image, not an installed one: it is not in the install fold
   and keeps the platform trust floor of a system desk. Default: **yes**; Texture
   stays unpackaged until S7-0 says otherwise.

## Heresy delta

- discovered: the S7 package manifest and the desk plan were two documents that
  never mention each other; the only per-app writer rule was a private check.
- introduced: none.
- repaired: the join is designed; no code.
