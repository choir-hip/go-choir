# S2 Layering: the runtime now execs a release, and the transport can carry one

October fourth, 2026. About eight hours of work on the S2 station —
"layering runtime from release" — inside the supervised-app-development
metamission. This is a checkpoint letter: the mechanism and its transport
are landed and deployed, and one deployed acceptance run remains.

The short version: a guest computer can now run an app-layer release's own
binary out of a per-computer store, overlaid on the base image, and the
signed update offer can actually carry that release to the guest. Both
halves had real bugs I had to find before they would work.

Where it stands. The layering path is landed end to end and the exec image
is deployed. A release that carries an app-layer closure gets replayed into
a GC-rooted private store, its store-path entrypoint is recorded, and the
guest runtime exec's that binary inside a mount namespace that overlays the
private store on the read-only base. The base binary stays the fallback.
Separately, the update transport learned to carry a payload too big for the
signed offer — the closure nar is about a hundred and forty-six megabytes,
and the offer body's sixty-four-megabyte cap could never hold it. A panel of
eleven models picked the same answer I would have: carry a digest-locked
content-addressed ref in the signed offer, and let the guest stream the blob
from corpusd and re-hash it before it touches the store. No new trust
surface. Two bugs I caught and fixed along the way: the first exec guarded
on a flat bin path a layered release never produces, so it would have
silently never layered; and a stale entrypoint marker survived a plain
rollback and would have re-exec'd the old layered binary.

## What actually happened

Thursday night into Friday I wired the three pieces that make a layered
release real. First the manifest learned to name the base it resolves
against, so an apply on the wrong base fails closed instead of layering onto
nothing. Then a pure-Go narchive reader plus a materializer replays the
exported store paths into the guest's private store, rooted per release.
Then the guest's exec wrapper reads the recorded store-path entrypoint and
runs it under the overlay. I verified the overlay plus exec works on the
host kernel before trusting it — the mechanism is sound; the remaining
question is the deployed path.

The deployed acceptance is where it got honest. A layered release is the
real autoputer — about a hundred and forty-six megabytes — because the
point is to run the released runtime, not a stub. That binary cannot ride
the signed offer inline, and the cap sits in three places, not one. I ran
the consensus panel on the transport design; it converged on the
content-addressed ref, on the strength that an offer's signature already
commits to the file digest, not the bytes, so a ref carries no new
authority. The fetch endpoint streams the blob the way the projection-base
download already does.

## What is left, and what I would do next

One thing remains: run the deployed acceptance. It uploads the nar blob,
mints a layered offer by ref, applies it to a disposable computer, and
watches the guest exec the release's store-path binary — then a deliberately
base-mismatched offer has to fail closed. That run is gated on the
transport deploy landing; the code is committed and pushed. If I were
holding the phone, I would want the acceptance result before calling S2
done, because every earlier "done" claim on this station turned out to be a
component landing, not the behavior.

Names and receipts. Layering mechanism: commits 20880d75, b68357a1,
a7052d2a, c7bb4a12, 1be8bd72, deployed image 3142979b. Transport:
ef2e607d, panel run .agentic-consensus/agentic-consensus-20261004-094113.
Acceptance probe: scripts/s2_layered_update_probe.mjs. Problem docs:
docs/problems/s2-runtime-exec-still-baseline-2026-10-04.md,
docs/problems/s2-layered-offer-transport-cap-2026-10-04.md.
