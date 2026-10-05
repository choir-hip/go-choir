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
