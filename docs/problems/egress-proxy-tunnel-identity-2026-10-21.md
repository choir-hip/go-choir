# The egress proxy binds a tunnel to the name it was asked for, not to what flows inside

Date: 2026-10-21. Status: open. Found by: gpt-6.1-sol on the panel for
[delegate-one-round-build-2026-10-21.md](../design/delegate-one-round-build-2026-10-21.md).
Severity: P1 for `ecosystem_proxy` capsules. Mutation class of the fix: red.

## Problem

The capsule egress proxy (`internal/capsule/egress.go`) admits a tunnel
by the host name in the request and the address it resolves to, then
relays the bytes. Nothing ties what is spoken inside the tunnel to that
name. So the allowlist limits which front doors a capsule can knock on,
not which services it reaches through them. The allowlist's suffix match
also admits subdomains of every listed name.

The working detail is kept out of this public repository until the fix
lands (the repo rule: keep security detail out until remediated). It is
in the panel output outside the repo.

## Exposure

Every `ecosystem_proxy` (L2) capsule, which is the default for
implementation capsules. `claude`-mode capsules allow one host and are
exposed to the same class through it. Capsules with no egress are not
exposed.

## Fix directions (not chosen yet)

- Inspect the start of the tunneled session and refuse a name that is
  not the admitted one; cheap, closes the plain variant.
- A fetch service per ecosystem that terminates and constrains the
  request (GET/HEAD of package artifacts); the strong form.
- Exact host matching where a suffix is not needed.

## Evidence

Panel run 2026-10-21 (outside the repo); the reviewer's live probe was
against public endpoints, not a deployed capsule. A deployed probe from a
test computer's capsule is the admissible evidence for the fix.
