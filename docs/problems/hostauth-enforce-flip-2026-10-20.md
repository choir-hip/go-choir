# Host-internal auth and owner acts: the enforce flip (2026-10-20)

Status: evidence gathered, flip staged. Owner ruling: owner notes §27
item 3 ("Agents may flip the security switches (hostauth, owner-act) from
observe to enforce after a clean day"). Rollout rule:
`nix/node-b.nix` (`internalAuthMode`, `ownerActMode`): flip once the
journal shows no "would refuse" for a full day of deploys and timers.

## Evidence, 24 h to 02:50 UTC

`hostauth: observe: would refuse` lines, grouped by route and caller:
35, all from operator shells during this session's maintenance work
(vmctl `hold`, `unhold`, `stop`, `refresh`, `maintenance-serve`, `list`,
`compact-sweep`, `compact-store`, over the vmctl socket and once over
127.0.0.1 at 20:56). Their timestamps match the session's operator
actions (20:30 to 01:50). No service, timer or deploy call was refused,
across several deploys in the window.

One latent gap: the new `go-choir-store-compaction` timer (d8ea02b07)
sends no credential. It has not run yet (first run 04:00 UTC); under
enforce it would be refused. Fixed before the flip: it loads
`choir-internal-token.header` with `LoadCredential=` and sends it, like
`go-choir-disk-gc`.

`owner-act: observe: would refuse`: 0 in 72 h. But the window holds no
owner acts at all (only key-load lines at service starts), so the zero is
not evidence. Owner-act stays in observe until owner traffic has passed
through it clean.

## Plan

1. Deploy the timer credential fix.
2. Confirm the 04:00 timer's `compact-sweep` call logs no "would refuse".
3. Flip `internalAuthMode` to `enforce` alone, as its own deploy; watch for
   `hostauth: enforce: refused` through the next deploy and timers.
   Operator shells send `-H @/var/lib/go-choir/secrets/internal-service-token.header`.
4. Owner-act: flip after a window with real owner acts and no refusals.

Rollback: `internalAuthMode = "observe"` (or `ownerActMode`), redeploy.
Mutation class: red (auth). Heresy delta: discovered, a timer added
tonight without the credential; repaired by the fix below.
