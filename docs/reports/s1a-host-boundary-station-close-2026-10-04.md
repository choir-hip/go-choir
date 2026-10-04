# S1a Closes — The Guest Is No Longer the Host

Written October fourth, 2026. This letter reports the S1a slice of the
supervised app development metamission — the host-boundary hotfix pulled
ahead of everything else when a source-traced review found that any guest
on staging could inherit host-internal and cross-tenant authority.

## The short version

The boundary holds. On two disposable staging computers, guest-originated
traffic now hits a wall where it should and a door where it must: tapping
the neighbor's tap times out, forged internal-caller headers get a 403,
the platform-signing mint and wire publish refuse at method level, and
the legitimate flows — gateway inference, bound-owner mail read, route
resolve, outbound egress, the product page — all still work. The cross-
tenant chain is closed on deployed evidence.

## What the defect was

A guest is a Firecracker VM on a /30 tap it shares with the host. Before
the fix, packets from a guest were DNAT'd to the host's loopback services
and carried headers the guest itself writes — X-Internal-Caller,
X-Authenticated-User — that those services trusted. Source tracing showed
the whole chain: a guest could list every computer on the box, refresh or
stop any of them, read another tenant's mail, and mint a platform-signed
update aimed at a victim's frontend. Open registration made it a live
exposure, which is why it jumped the queue.

## What the fix did

Two layers, landed together. At the network layer, each tap now drops
spoofed sources, drops all tap-to-tap forwarding, and stops DNATing the
host autoputer port; dead rules that never fired were removed. At the
authority layer, host-internal checks now bind the caller's real tap IP
to a live ownership — the header became a marker, not a credential — and
the sensitive mints went loopback-only, reachable from guests only
through a new bound lookup endpoint.

## The honest part

The proof itself was the hard part. To measure refusals you need a
caller inside the guest, and the natural channel — the management
console — turned out to be silently broken: the guest image resolves
"zot" to a third-party terminal agent, so the console spawns a UI shell,
not the repair session it was built for. The workaround is honest and
small: the guest's own diagnostics endpoint gained a tightly bounded
HTTP mode — GET-only, a three-second budget, and a header allowlist
limited to exactly the two forged-identity headers the matrix needs.
Every refusal and every green flow in the evidence file is truly
guest-originated through that oracle. The console defect is recorded as
its own problem; it does not block this slice.

## What is still open

Outbound internet stays open by design — default-deny egress is a later
slice. The runtime still runs as root, the gateway token is still on the
kernel command line, and the console exec surface needs its own repair.
Those are S1's floor, not S1a's. The mission now returns to the S0
reality probes on disposable computers; the same oracle that proved the
boundary will serve as their measuring stick.

## Receipts

- Evidence: docs/evidence/s1a-refusal-matrix-2026-10-04.json
- Problem + repair record:
  docs/problems/s0-guest-reaches-host-internal-authority-2026-10-04.md
- Deployed commits: a3f0d48e (boundary), b15f012a (oracle), d37408ee
  (matrix evidence)
- Genesis defect named for S0b:
  docs/problems/s0b-registration-computer-missing-genesis-2026-10-04.md
