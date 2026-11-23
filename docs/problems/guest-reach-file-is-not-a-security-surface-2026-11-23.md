# The file that decides what a guest can reach is not a security surface

Recorded 2026-11-23.

## Problem

`internal/proxy/guest_authority.go` decides which host paths a guest computer
may call. It is not listed in `nix/guest-security-surfaces.txt`, and nothing
under `internal/proxy/` is. A commit that gives every guest a new host route
therefore passes `scripts/check-guest-security-epoch` with no
`Security-Effect` statement and no reviewer.

## Evidence

- The builder of `lane/ask-search` added a guest carve-out for
  `GET /internal/platform/published/ask` in that file and the gate asked for
  no trailer on that commit; the review seat (DeepSeek, review 1 of that
  lane, 2026-11-23) named the same gap.
- `grep proxy nix/guest-security-surfaces.txt` prints nothing at `6229cd73a`.

## Belief

The surface list was drawn around the guest's own code (`internal/agentcore/`
and the image) and missed the host file that grants guests reach. The
omission is an oversight, not a decision.

## Remaining error

Not yet read: whether other host files grant guest reach outside the list
(the gateway's guest routes, corpusd's handlers, the capsule broker). The fix
is a gate change with its own trailers and review: add the proxy's guest
authority file, and whatever that reading finds, to the surface list. Until
then a reviewer must look for guest-reach changes by hand.
