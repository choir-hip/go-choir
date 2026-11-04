# Package install: an owner installs a Library release on their computer (2026-11-04)

Status: design note, red (it changes a computer's running code). Owner
request, 2026-11-04: "Make library actually work now. Backend included.
Then I'll review the other apps through it." It builds on
[the Library design](library-package-propagation-2026-10-26.md) (a package
release is one `frontend/src/packages/<name>/` tree; published releases are
public, 2026-11-03; the owner's word is install, not adopt).

## What exists, and what this adds

- The catalog (`/api/catalog/*`), publish, and the release record
  (`internal/pkgrelease`) exist. The tree is `tree_patch`: a binary diff
  from the empty tree that rebuilds exactly `frontend/src/packages/<name>/`.
- The host-only blob read `GET /internal/platform/packages/releases/<id>/blobs/<sha>?owner_id=`
  lands on branch `package-export` (with the public catalog route). This
  work calls it and must land after it.
- P8 source adoption (`release_adoption.go`) staged a software release's
  patch and asked the desks, through the owner's prompt path, to apply it in
  an engineering capsule. Install keeps that half: the capsule, Freeze,
  Verify, the owner's decision, the materializer, its planned restart and
  rollback, unchanged.
- The published Library UI (`library-next`, pool's release `8810c72d…`)
  already calls `POST /api/computers/<id>/self-development/adoptions` with
  `{release_id, record}` and lists installs from `GET` on the same route
  (`adoptions[]`, each with `adoption_id`, `release_id`, `requested_at`,
  `publisher_computer_id`, `same_base`, `patch_matches_release`,
  `operation.state`). Today that POST answers `release_record_invalid` for a
  package record. So the backend takes the route the UI already calls.

Install replaces P8 (owner ruling, 2026-11-04: "If the existing adoption
endpoint isn't actually working, then get rid of it"). The adoptions route
keeps its path, because the Library UI, the capsule install fixture
(`scripts/choir-shot.mjs`, `scripts/fixtures/capsule-install.json`) and the
fixture parity test already call it; it now takes package releases only.
Renaming it is a later UI change. No parallel apply path: the only way code
changes on a computer stays a frozen, verified, owner-decided
self-development operation.

### Did software-release adoption ever work? (evidence)

- Once, in a drill: adopt-1400 (2026-10-15, same owner, both computers made
  by `scripts/reeng_tracer.mjs --scenario adopt`) reached `applied`; its
  serve and identity checks read false
  (docs/archive/problems/adopt-1400-adopter-applied-served-and-identity-read-too-early-2026-10-15.md).
- adopt-1418 the same day: 201, then the desks finished with no operation
  (docs/archive/problems/adopt-1418-adopter-desks-finish-without-an-operation-2026-10-15.md).
- The only real use, 2026-10-27: the owner's `release adopt` of release
  4320be52… answered 404 (cross-owner reads), and the owner paused release
  adoption for a design pass
  (docs/definitions/choir-consolidate-and-scale-metamission-2026-10-26.ledger.md).
- Staging today: `choir release list` answers `count: 0` for the owner; the
  keep-set accounting counts 4 `choir.software_release` records
  (docs/evidence/keep-set-accounting-2026-10-29.md), none in the keep set.
- Callers besides the Library UI: `choir release publish/adopt/adoptions`
  and `reeng_tracer.mjs --scenario adopt`. No desk, no nix unit, no CI job.

Verdict: not working, unused, superseded by the Library. Deleted in its
own commits: software-release publish and adoption end to end.

### Deletion citers (standing question: deletion citers)

Code: `internal/agentcore/release_adoption.go`, `release_source.go`,
`api_release.go` (release draft, adoptions handler), `effective_release.go`
(`adopted_release_id`), `api_self_development.go` (routes),
`engineering_assignment_runtime.go` (capsule input, kept for installs),
`runtime.go`, `run_context.go` (fixture text, kept), `internal/autoputer/run.go`
(`WithReleaseSource`), `internal/computerevent/release_client.go`,
`internal/proxy/releases.go` (publish, `/api/releases`),
`internal/proxy/self_development.go` (adoptions), `internal/platform/releases.go`
(corpusd routes), `internal/vmctl/reported_release.go` (`adopted_release_id`),
`internal/store/release_adoption.go` (kept: the install record),
`internal/objectgraph/registry.go` (kind kept: install records use it),
`cmd/choir/release.go`, `cmd/choir/registry_test.go`, and every `_test.go`
beside them. Scripts: `scripts/reeng_tracer.mjs` (adopt scenario),
`scripts/choir-shot.mjs` and its test (kept: they fixture the install route).
Docs: `skills/choir-cli/SKILL.md` (release rows), and history that stays as
history: the Library design, `release-adoption-is-model-mediated`,
`release-reads-not-owner-scoped`, `release-carries-the-cumulative-stack`,
the readiness and consolidate ledgers, `docs/reports/choir-cli-mcp-parity`.

## The path, end to end

1. **Owner act.** The Library's Install button (or `choir package install
   --release <id>`) POSTs `{release_id, record}` to the adoptions route. The
   proxy requires the owner act (`authorizeOwnerAct`, as for adopt).
2. **Proxy fetch and verify.** For a record whose schema is
   `choir.package_release.v1`, the proxy reads the release from corpusd as
   that owner (so only a release visible to them, and only a `listed` one),
   verifies the stored record against the id (`pkgrelease.Decode`,
   `VerifyID`), requires the supplied record to be the same bytes, reads the
   `tree_patch` blob and verifies it (`VerifyBlob`). It forwards
   `{release_id, record, tree_patch, action}` to the computer. A guest never
   reads the catalog; the proxy is the host side that may.
3. **Computer verify.** The computer re-verifies the record against the id
   and the patch against the record, then checks the content
   (`pkgrelease.VerifyContent`): confined to the package root, the root's tree
   is the record's tree, no added secret, the manifest validates.
4. **Three-way against its own tree.** L is the subtree at the package root
   in this computer's source HEAD, P the release's parent tree, R the
   release's tree.
   - L = R: already installed (200, the existing install).
   - L absent, or L = P: install.
   - L is the tree of an earlier Library install of the same package on this
     computer (another track's release): replace. This is how the owner
     reviews several tracks of one app in turn.
   - otherwise (a first-party package, or a local change): refused
     `package_name_taken`. The computer's tree is never overwritten.
   The L = P case trusts the record's declared `parent.tree`: that lineage
   is the publisher's assertion (corpusd checks it at publish), and the
   owner's act is still required to install.
5. **Install patch.** The computer computes the patch from L to R at full
   paths (to the empty tree for uninstall) and checks it applies to HEAD
   without writing (`release.PatchedTree`). The result holds R at the root
   by construction (the patch is the diff to R); the code checks that the
   apply succeeds, not a separate subtree compare.
6. **Stage and ask.** As P8: `release.json` and `source.patch` read-only
   under `<updater root>/adoptions/<install id>/`, mounted in engineering's
   capsule at `/selfdev/adoption/`; one owner request to the desks asks
   engineering to `git apply` it, run the frontend build (which runs
   `check-packages.mjs`), freeze, and change nothing else. A publisher's
   declared test command is never run. The install
   record (kind `package`, package, action, release tree) is written.
7. **Build, verify, decide, apply.** Engineering's capsule builds; management
   verifies; the owner approves the candidate in Texture; the materializer
   applies it with its planned restart. The app's directory is now in the
   computer's tree and its SPA, so `frontend/src/lib/apps/packages.ts` lists
   it in the launcher.
8. **Installed means the tree.** An install's `patch_matches_release` is
   true exactly when the computer's source HEAD holds R at the package root
   (an uninstall: holds nothing there). That is the artifact check, not the
   operation state.

**Uninstall** is the same path with `action: "uninstall"`: allowed only
when L = R for the named release, so it never removes a first-party package
or a local change. **Rollback** of an install or an uninstall is the
self-development rollback of its operation, unchanged.

## Restarts (CLAUDE.md, "Restarts End Work")

A crash before the decision fails the operation "interrupted by a restart"
(`closeSelfDevelopmentOperationsAfterCrash`); the install record stays, and
Install again opens a new attempt (a repeat returns the existing install
only while its operation is live or applied, or it has none yet and is under
two hours old). A crash in materialization settles from the updater journal;
the apply's own planned restart resumes. No boot-time resumer is added.

## Names and origins

- A release can only be installed under its own name. A name a first-party
  package holds is refused at step 4 unless the release's parent is that
  package's tree (a variant, design 3.x).
- `check-packages.mjs` refuses a static registry id, a reserved name, a
  duplicate id, and any origin other than `first_party`. The build in the
  capsule runs it, so such a release fails at build, nothing is applied, and
  the operation fails with the check's message. A third-party origin
  manifest is therefore not installable until runtime install (S12); the
  Library shows the refusal.

## Failure modes, written before the code

| # | Failure | Guard |
|---|---|---|
| F1 | Record substituted in transit or edited by the UI | id verified at proxy and computer; supplied record must equal corpusd's |
| F2 | Withdrawn or unseen release installed | corpusd read as the owner; `listed` required; else `release_not_published` |
| F3 | Blob bytes not the record's | `VerifyBlob` at proxy and computer |
| F4 | Content outside the package root, symlink, submodule, attributes | `VerifyContent` (check 1) |
| F5 | Secret added to the computer's tree | `AddedSecrets` on the tree patch |
| F6 | Overwrites a first-party package or a local change | three-way, `package_name_taken` |
| F7 | Patch does not apply to this computer's HEAD | `PatchedTree`, `patch_does_not_apply` |
| F8 | Engineering applies something else | staged bytes re-verified at mount (`adoptionInputDir`); "installed" is the tree check, not the operation |
| F9 | Build fails (bad name, origin, compile error) | the capsule build fails; nothing applied |
| F10 | Install stuck after a crash, Install says "already" forever | attempts; a dead attempt does not block a new one |
| F11 | Uninstall removes a first-party or locally changed package | uninstall only when L = R |
| F12 | Guest or desk installs without the owner | owner act at the proxy; the tree patch is supplied only by the proxy |
| F13 | Proxy forwards a 3 MiB body the guest refuses | body cap raised for package installs only |

## What this does not do

- It is not a one-press install: the owner approves the candidate in Texture
  after the desks build it (minutes to tens of minutes; model calls). The
  deterministic `package_adopt` job with no model call (Library design 6.4)
  needs a capsule build run outside a desk and a freeze outside an
  engineering assignment; estimate several days of red work. Recorded as the
  next step, not built here.
- The UI's outcome sheet reads `patch_matches_release` at the POST answer,
  before anything is built, and shows a warning. It should say the install
  has started and point at the candidate (a UI follow-up).
