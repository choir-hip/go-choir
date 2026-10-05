# Frontend-by-digest proof: what the tape and binaries already establish, and the one line still missing

Date: 2026-10-05 · Station: S2 gap closure

## What already proves which binary serves (strong)

The execution-identity witness the panel asked for (nonce-bound sha256 of
`os.Executable()`) cannot be produced on the disposable right now: the
endpoint refuses with "incomplete or conflicting executable, realization,
epoch, closure, or deploy identity" — the guest's deploy-receipt identity
and the layered release's embedded buildinfo disagree by design on a layered
computer (the release binary is not the deployed binary). That disagreement
is the layering contract itself, so the endpoint is the wrong oracle here.

The positive proof that the *release binary* serves is already on the record,
three independent ways:

1. **Boot timeline executable path.** The guest's own boot receipt names the
   exec path it layered: `/mnt/persistent/choir-updater/store/j9kb9zxns1i8…-
   autoputer-0.1.0/bin/autoputer` for the 6fcb05e5 apply (boot id d541d292),
   and `…/jvbszni0i76…-autoputer-0.1.0/bin/autoputer` after the rollback.
   A base-serving guest would name the EROFS `/nix/store` path instead.
2. **Console layering lines.** `go-choir-autoputer: layering release
   /mnt/persistent/choir-updater/store/<release>-autoputer-0.1.0/bin/autoputer
   over private store` appears for each applied release (lan9hn9, j9kb9z,
   jvbszni0 across the run).
3. **Buildinfo commit in /health.** `.build.commit` moved 2f0e2cac →
   6fcb05e5 → 2811c779 exactly on the applies that swapped `current/`, while
   `deployed_commit` stayed at the base. The served commit is the release's
   embedded buildinfo, not the base's.

## What is actually happening (corrected 2026-10-05 ~14:00)

The staged 6fcb05e5 releases DO carry `frontend/index.html` — the offer
shape used in this run includes the probe's minimal SPA file
(`<title>s2-layered</title>`), and three staged release dirs on the
disposable contain it. So the "CI shape carries no frontend" premise in
the section above is wrong for the legs that mattered, and the marker
experiment failed for a different reason: the builder's `cleanSourceWith`
filter, not the offer.

The serving surface still answers `served SPA is underivable`, which now
points at the *guest-side* join, not the offer: either `current/` does not
point at one of the frontend-carrying release dirs at serve time, or the
in-guest `current/frontend` read fails. The `current` symlink itself is
unreadable from the host right now (dangling rendering after journal
replay), so even the pointer target is unconfirmed. That is the precise
next probe: resolve `current/` in-guest, confirm it names a release dir
containing `frontend/index.html`, and fetch `/` through the guest (not the
vmctl proxy, which never reaches the surface route).

## Smallest probe that closes it

In-guest: readlink `current/`, list `current/frontend/`, and curl `/`
from inside the guest network namespace. If the marker serves, the
executable+frontend join holds in one transaction (criterion 1's "new
backend and frontend"); if the base SPA serves, the claim fails and the
heresy stands. No new authority or code path is needed.

## Update 2026-10-05 ~12:55: marker-release attempt failed at the source filter

A `test(s2)` marker-comment commit (2d0c16c0, reverted as 5c033175 — main
is clean) was built into a release and applied green (serving 2d0c16c0,
ready). The marker byte was found **only inside the staged closure.nar**,
never materialized: the builder's `cleanSourceWith` filter passes only
`.js/.mjs/.ts/.svelte/.css/.html` plus package manifests, and the marker
lived in `frontend/index.html` at the repo root, which the built SPA
(`frontend/dist`, what ships in `$out/frontend`) does not include. The
release SPA is therefore byte-identical to the base SPA for this commit,
and the serving surface cannot discriminate them.

Correct retry: put the marker in a file the built SPA includes (e.g. the
Svelte title in `frontend/src/App.svelte`, if it renders into `dist`), or
assert on the release's `$out/frontend` tree hash versus the base's at
build time and serve that digest as the frontend identity. Either keeps the
proof inside builder-produced, base-absent bytes.

## Update 2026-10-05 ~13:00: base-absent-dependency leg is partially redundant

Reading the S0b substrate decision
(docs/problems/s2-builder-substrate-2026-10-04.md) against the leg text:
the selected builder is the **host service**, whose "build environment" is
the host nix store + network. "Disposal of the build environment" for a
host-service builder means the release must resolve with no builder
present at apply time — which is already the standing condition of every
leg in this run: the guest updater replays `closure.nar` from the staged
blob against the booted base's store paths, and the builder is never
consulted during apply. Every base-digest/base-commit refusal in the
10:27-10:29 legs already exercised "the builder cannot substitute a
base-absent dependency," because the release carried only its own digest
bindings and the guest refused them against the booted base.

What the leg text additionally demands, and what is genuinely unproven, is
narrower: **after a guest reboot** (projection rebuilt from replay, no
in-memory state), present a release whose closure names a store path absent
from the booted base and observe refusal before mutation. That is a
~10-minute leg once the disposable is back: mint with a `closure_digest`
whose nar is staged but whose entrypoint path is not in the base store
list, reboot the guest, push, and read the refusal. It needs no new code —
the `content-mutation` neg shape with a reboot inserted before the push.
Left for the next session with a healthy disposable.

## Update 2026-10-05 ~14:30: the join fails at stage, not at serve

Cold-reading the hibernated data disk settles the mechanism completely:

- `current/` names release `c8ef4c6c…` (`2811c779`, marker
  `s2acc-rb-20261005T132300Z`) — the pointer itself is healthy; the
  host-side "dangling symlink" rendering was an unreplayed-journal
  artifact, not disk corruption.
- That release dir contains `closure.nar`, `layering-entrypoint`, and
  `release-manifest.json` — **no `frontend/` dir**. The 2811c779 release
  was built by the older harness shape (closure.nar only, no SPA file).
- Exactly three staged release dirs carry `frontend/index.html`: the
  genesis baseline (full base SPA) and two 6fcb05e5 dirs carrying the
  probe's minimal `<title>s2-layered</title>` SPA — i.e. precisely the
  legs that used the frontend-carrying offer shape.

So the chain is: offer files → staged into the release dir → served from
`current/frontend`. The 2811c779 rollback serves the *release binary*
(proven three ways) with **no release frontend staged**, hence
`ComputerSurface` falls back to the baseline root, which is also absent
in the layered context — therefore `served SPA is underivable`. The
surface 503 is correct behavior for a frontend-less release; the defect
is that nothing requires a frontend at stage time.

This reframes the gap from "prove the frontend serves" to a concrete
contract hole: **a layered release without `frontend/index.html` applies
cleanly and then serves a 503 on `/`**. Either `stageRelease`/`verify`
must require the frontend file (making frontend presence part of the
release contract), or the surface must serve something defined in its
absence. The panel's "frontend-by-digest" leg then becomes: apply a
frontend-carrying release (any 6fcb05e5 dir already staged proves the
bytes exist; a fresh apply + `/` fetch shows them serving).

## Update 2026-10-05 ~16:38: shape proven, discriminator specified

Cold-booted the disposable (fresh resolve after the wedged FC was
killed) and fetched `/` headlessly through an SSH tunnel: HTTP 200 with
the staged minimal SPA bytes (`<title>s2-layered</title>`, `div#app`),
JS-evaluated build markers all null. Record `frontend-shape-proof` in
`s2-acceptance-20261005T163642Z.jsonl`.

This establishes the serving *shape* — `current/frontend/index.html`
from the staged release dir serves, not the base SPA and not a 503 —
and simultaneously confirms the panel's objection: the served bytes are
the harness-injected inline file, so this is staged-file serving, not
builder-output-by-digest serving. The discriminator is now executable,
not theoretical: the SAME fetch must return built-bundle bytes
(`dataset.choirBuildCommit`, `window.__CHOIR_BUILD__`, or the
`s2-frontend-marker` meta). The way to get them staged is a
frontend-carrying apply of a release whose staged `frontend/index.html`
is the built bundle — i.e. the `apply` leg with the release built from a
commit whose `frontend/dist` differs from base, not the inline `$spa`.

## Update 2026-10-05 ~17:00: fixed at stage time (4c4c9124)

Root cause, not a probe gap: `stageRelease` copied only manifest files,
so a release without an inline `frontend/index.html` file staged a dir
with no `frontend/` — and `ComputerSurface` then correctly 503'd. The
fix copies the materialized autoputer output root's `frontend/` tree
into the staged dir (regular files only, never symlinks), so
`current/frontend` joins the swapped exec for every release, whether or
not the offer carries a separate frontend file. Regression test
`TestApplyStagesReleaseFrontendFromOutputRoot` fails before, passes
after. The built-SPA marker assertion and the reboot-then-push leg can
now run against this behavior once it deploys.

## Update 2026-10-05 ~17:30: inline-$spa masks the daemon fix

Applied release `03cfc944` (serving healthy, same fc/boot) and fetched `/`
raw: the served bytes are the **harness-injected inline SPA**, not the
built bundle. The staged release dir carries `frontend/index.html` only
from the offer's inline `$spa` file. Two findings:

1. The daemon-side staging fix (`4c4c9124`) could not act in this run:
   the serving updater daemon is the **base image's** (pre-fix build),
   and the fix reaches the daemon only via a base refresh. Staging behavior
   is daemon-version-gated, not just offer-gated.
2. The inline-`$spa` offer shape **masks** the daemon fix: as long as the
   offer carries its own frontend file, the served bytes can never
   discriminate staged-built-tree serving from inline-file serving.

Next (both required for the leg to mean anything): (a) refresh the guest
onto a base carrying the fix; (b) apply with the inline frontend file
**omitted** from the offer, so the only frontend bytes that can serve are
the staged built tree — then the built-marker (or 503) is dispositive.

## Update 2026-10-05 ~17:52: same-commit releases are base-present by construction

Applied `8e29fe06` (no-inline-frontend offer) against base `8e29fe06`:
refused `layering entrypoint ... not materialized` — and the refusal
reason now crosses the boundary end to end (daemon → client → agent →
push response), closing the observability hole from this morning. The
refusal itself is correct behavior exposing a harness error, not a
product defect: the release's autoputer store path is byte-identical to
the base's (same commit built both), so the builder's delta excludes it
and the entrypoint can never materialize. A layered release must differ
from its base; same-commit applies are refused by design. The pending
transition discharged cleanly on the refusal (Outcome=refused path).

Retry: rebuild the same source against an older base (or apply an older
release against this base) so the entrypoint is base-absent, with the
inline frontend file omitted — then the staged `frontend/` can only be
the built tree and the marker fetch is dispositive.

## Update 2026-10-05 ~18:40: same-commit refusal re-confirmed with full reason chain

Applied `d32a7b23` (no-inline offer) against base `d32a7b23`: refused
`layering entrypoint ... not materialized`, and the reason now crosses
the whole chain (daemon → client → agent → push response). The refusal is
correct: the release path is byte-identical to the base path, so the
delta excludes it. This is the third independent confirmation that a
layered release must differ from its base — and that the harness must
rebuild per base, never reuse a cached nar across deploys.

## Closure 2026-10-05 ~19:05: built-frontend join green

Applied patch-built release `57605ac` (S2-f join: title patch on
`2811c779`, `code_commit` derived-patch) with the inline frontend file
**omitted** from the offer: serving healthy, same fc 189418, same boot
c643bbca. Headless fetch: `dataset.choirBuildCommit`,
`window.__CHOIR_BUILD__`, and `meta s2-frontend-marker` all name the
**patched** commit — bytes that exist only in the built bundle (Vite
`__CHOIR_BUILD_COMMIT__` define at build time), unreachable to any
harness injection. The staged `frontend/` can only be the daemon-staged
built tree. Record `apply-idxmarker-built-frontend` in
`s2-acceptance-20261005T190133Z.jsonl`.

Criterion 1's "new backend and frontend" now holds by digest: the served
executable is the release binary (buildinfo commit moves with
`current/` swaps) and the served frontend bytes are the release-built
bundle (marker names the patched commit). The heresy
(computer_surface.go:69-87 baseline fallback) is repaired for the
staged-frontend path: a frontend-less release can no longer be mistaken
for a joined one, because the proof shape requires the built marker.
