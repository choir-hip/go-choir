# S0 finding: the new internal read surface is protected only by a forgeable header

Date: 2026-10-01
Discovered by: S0a boundary panel (claude, omp-gemini38, omp-glm53-flash)
Station receiving it: S1-security-floor (amplify the tap finding)

## Evidence

The S0a instrument added two guest endpoints gated by
`guestInternalCaller`, which only checks for the header
`X-Internal-Caller: true`:

- `GET /internal/boot/timeline` — returns build/kernel identity, guest
  marks, mounts, nix-store layout, env presence, cmdline presence flags.
  Information disclosure only.
- `GET /internal/diag/tcp-dial?addr=IP:port` — performs an arbitrary TCP
  connect from the guest. Bounded to IP literals and a 2s timeout, but
  turns any reachable guest into a cross-vantage port scanner. Against a
  guest it becomes SSRF: a caller on the shared tap can make guest A
  probe `vmctl :8083` or another guest's listener, and the connection is
  attributed to A.

The public proxy strips `X-Internal-Caller` (`internal/proxy/handlers.go`),
so the surface is only reachable over the tap. S0a's own tap-reachability
receipt proves the tap is unfiltered guest-to-guest and guest-to-host —
any guest (or capsule workload inside one) can set the header and use the
endpoint.

The same header also gates `GET /internal/vmctl/boot-timeline?computer_id=…`
on the host's vmctl port, which returns any computer's merged receipt.
`GetOwnershipByComputerID`'s own contract notes callers must authorize
before invoking; this handler doesn't. Because tap→host :8083 is open,
any guest can read every computer's boot record (identity, replay volume,
paths) — again read-only, but an unauthenticated internal read of
per-computer state.

## Not a fix here

S0a needed both endpoints to collect the receipts; they are the probe
oracle. The fix belongs to S1, which owns the internal-caller contract:
bind the surface to a vmctl-only credential or drop tcp-dial once S0b
has used it. Documented now so the door is named before the security
floor builds on top of it.
