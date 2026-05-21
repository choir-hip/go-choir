# Gateway Auth / Orphan VM Incident - 2026-05-21

## Summary

During owner review of the Apps & Changes experiment portfolio, the active
primary computer for `ymnath@choir-ip.com` surfaced:

```text
tool loop iteration 0: gateway call failed: gateway client: authentication required
```

This was not a global provider outage. The host gateway was healthy, the
provider credentials resolved, and the persisted gateway token for the affected
VM was present and accepted by the gateway when tested from the host.

The broken path was a stale VM lifecycle state:

- vmctl ownership registry said `vm-8ad4ce4cd5df1b6333e63ea1230992d5` was
  `stopped` with `stopped_by=vmctl-restart` from 2026-05-15.
- A Firecracker process for the same VM was still running as an orphan process
  under PID 1 for more than six days.
- The running guest sandbox had a gateway URL but was missing a usable
  `RUNTIME_GATEWAY_TOKEN`, so model calls reached the gateway with an invalid
  authorization header.
- Because vmctl believed the VM was stopped, ordinary registry reconciliation
  did not repair the live guest.

## Operational Repair

The repair preserved the VM's persistent state:

1. Terminated only the orphan Firecracker process for
   `vm-8ad4ce4cd5df1b6333e63ea1230992d5`.
2. Ran `e2fsck` on that VM's `data.img` after `resize2fs` correctly refused to
   boot an unclean filesystem image.
3. Resolved the same primary desktop through vmctl, which booted the same VM ID
   on a fresh route with a valid gateway credential.

Fresh evidence after repair:

- sandbox health reported `active_provider=gateway`;
- vmctl logs showed repeated `gateway client: inference succeeded`;
- gateway auth-denied logs stopped for the affected prompt path.

## Code Hardening

The patch adds three narrow hardening changes:

- `GatewayClient` now rejects an empty sandbox credential before making HTTP
  calls, returning `gateway client: missing sandbox credential` instead of
  sending an invalid authorization header.
- sandbox startup logs a clear warning when a gateway URL is configured without
  `RUNTIME_GATEWAY_TOKEN`.
- `/internal/vmctl/resume` now uses the same persisted-VM boot fallback as
  `/internal/vmctl/resolve` when vmctl has an ownership record but the VM
  manager has lost the in-memory instance.

Focused tests:

```text
nix develop -c go test -count=1 ./internal/gateway ./internal/vmctl
```

## Mission Learnings

The alternate-computer experiment portfolio produced useful artifacts, but the
review path failed as a product experience:

- Apps & Changes over-indexed on candidate VM preview, so a stale candidate
  route became a 502 retry wall instead of a graceful demo surface.
- The VText reports were technically accurate but too jargon-heavy for owner
  review.
- The store UI exposed package/adoption mechanics before plain-language value,
  screenshots, videos, and benchmarks.
- Non-active app suspension is still too eager for real review sessions.

The next review substrate should be demo-first:

- stable screenshots/video/plain-language explanations by default;
- live candidate try-out only as an explicit advanced action with clear wake,
  failure, and retry states;
- technical refs collapsed into an appendix;
- per-change VText reports written for human review first, machine audit
  second.
