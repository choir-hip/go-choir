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
  `GET /internal/platform/published/ask` in that file (commit `2d44744e0`).
  The gate did not require a trailer for it; the commit carries
  `Security-Effect: none; ...` and `Security-Effect-Reviewed-By:
  deepseek-v4.1-flash; review 3` because the lane's review supplied them by
  hand. The review seat (DeepSeek, review 1 of that
  lane, 2026-11-23) named the same gap.
- `grep proxy nix/guest-security-surfaces.txt` prints nothing at `6229cd73a`.

## Belief

The surface list was drawn around the guest's own code (`internal/agentcore/`
and the image) and missed the host file that grants guests reach. The
omission is an oversight, not a decision.

## What the reading found (2026-11-23)

A guest reaches the host through the tap firewall, which decides which host
ports exist for it: tapReachableHostServicePorts lists seven, and each is a
door. The proxy (:8082), vmctl (:8083), the gateway (:8084), corpusd (:8086,
directly at the guest's platform URL and through the proxy's relay), maild
(:8087), the Go module proxy (:8088) and the package mirror (:8089). The last
two authorize no caller. Traced from how each authorizes a guest's request (tap
source address bound to a live computer by vmctl, a gateway bearer pinned to
the computer's address, or a signed event capability):

| File | Guest-reach decision? | Listed |
| --- | --- | --- |
| internal/proxy/guest_authority.go | yes: which /internal/platform paths a bound guest may call, the object-list scope, the capture body check | listed (host:) |
| internal/proxy/guest_feeds.go | yes: which feed paths a guest may poll and how the owner is forced to the bound one | listed (host:) |
| internal/proxy/guest_reach.go | yes: RegisterRoutes (every route a guest can name on :8082), the corpusd relay HandlePlatformObjectGraph and HandleGuestTexturePublication, moved whole from handlers.go and platform_publish.go | listed (host:) |
| internal/proxy/handlers.go, platform_publish.go (the rest) | no: the public API and the owner's publish path | no |
| internal/gateway (the whole package) | yes: the SEC-05 pin rule is `IdentityRegistry.CheckPeer` in config.go (bearer_binding.go only chooses the mode and counts); jev.go and anthropic_relay.go apply the pin themselves; guest_reach.go holds authenticateAutoputer, the runtime-peer rule, the guest address range and the route table; host_fetch.go the fetch grant and budget; the credential routes use isAuthorizedCredentialCaller in handlers.go. More than three files, so the package | listed (host:) |
| internal/platform/authority.go, objectgraph_handlers.go, key_escrow_http.go, event_capability.go | yes: tap-source binding and the bound-guest write rule; requireInternalOrBoundGuest; escrow capability; the signed capability verifier | listed (host:) |
| internal/platform/guest_reach.go | yes: corpusd's route table (RegisterRoutes) and admitPublishedRead, moved whole from handlers.go | listed (host:) |
| internal/platform per-route files | no (a residual): each applies the shared rule above to its own route; a handler body that calls bindTapCaller is not listed | not listed |
| internal/vmctl (the whole package) | yes: ownership.go GuestOwnershipForAddress is the one rule that binds a source address to a computer (the proxy and corpusd use it too); handlers.go isInternalCaller and internalTransport admit by transport; the bind helpers and route table are in guest_reach.go; capsule_reservations, self_development_route and reported_release bind guests. vmctl is a red surface already | listed (host:) |
| internal/maild/api.go | yes: the guest binding (tap source bound to a live ownership through vmctl, and to the asserted owner) for :8087 | listed (host:) |
| internal/maild other files | no (a residual): handlers call the binding in api.go | not listed |
| internal/goproxy (8088), internal/pkgmirror (8089) | no: they authorize no caller (the source address is attribution for the log, "never authority") and serve public packages; the upstream host allow-lists bound the host's fetches | out of scope |
| internal/vmmanager/hostfirewall.go, guest_egress.go | yes: per-tap INPUT and FORWARD rules, the anti-spoof drop, the mangle chain | listed (host:) |
| internal/vmmanager/guest_reach.go | yes: tapReachableHostServicePorts, moved whole from manager.go | listed (host:) |
| cmd/capsule-broker | no: it runs inside the guest and is in the floor | floor |
| internal/hostauth, internal/hostfetch, internal/gatewayruntime | already listed | already |

## Fix

`nix/guest-security-surfaces.txt` takes file entries (one path), directory
entries and `host:` entries; `scripts/check-guest-security-epoch` enforces:

- a `host:` entry is a host-side surface: an epoch raise does not excuse a
  change to it (rule 4);
- an entry that can never match fails, in a full run and in `--lists-only`:
  a path that does not exist, an empty path (a bare `host:`), a non-canonical
  one (`./a.go`, `..`, `//`, a leading `/`), and a path the matcher excludes
  (`_test.go`, a nested `testdata`, `.md`; the refusal is slightly wider than the matcher: a root `testdata` and a directory named like `foo_test.go` or `foo.md` are refused too, fail-closed and not a realistic surface). The dead `internal/eventtablegrant/` entry
  is removed (deleted by `bdfdbb65e`, nothing moved).

Round 1 of review moved the small guest-reach declarations of the five mixed
packages (six files: proxy has handlers.go and platform_publish.go) into a `guest_reach.go` of their package (a pure move of whole
declarations; the two seats confirmed it line-identical apart from imports,
a header comment and one blank line). Round 2 found that the rules those
declarations call live elsewhere (vmctl's ownership binding and transport
admission, the gateway's pin in config.go), and that each round finds more.
This round stopped enumerating function by function: where a package is a
guest-reach authority throughout (vmctl, gateway) the whole package is listed.

## What is covered and what is not

Covered, by entry: vmctl and gateway (whole packages); proxy guest_authority.go,
guest_feeds.go and guest_reach.go; platform authority.go, objectgraph_handlers.go,
key_escrow_http.go, event_capability.go and guest_reach.go; vmmanager
hostfirewall.go, guest_egress.go and guest_reach.go (the port list); maild/api.go.
The six mixed files in five packages (proxy, gateway, platform, vmctl handlers, vmmanager
manager.go) no longer hold the declarations that were moved (the table says
which). Others stay where they are: vmctl handlers.go keeps isInternalCaller
and internalTransport, and gateway handlers.go keeps isAuthorizedCredentialCaller
and isLocalhost; the whole-package entries for vmctl and gateway cover them.
The proxy and platform handlers.go files keep their per-route handler bodies,
which are not covered (see below). They may also hold other guest-reach code
that nobody has found.

## Remaining error (named residuals)

The list is a best-effort map, not a proof of completeness.

- Per-route handler bodies in platform, proxy and maild that call the bind
  helpers are not listed, and a new file that calls bindTapCaller,
  requireInternalOrBoundGuest*, a vmctl bind helper or maild's binding is not
  listed until someone lists it.
- A rule moved out of a listed file leaves the gate blind to it.
- The older entries (hostauth, dbcred, keyescrow, hostfetch, gatewayruntime,
  the guest runtime and the rest above the host: block) carry no `host:` marker
  because nobody has classified them as host or guest side; for any that run on
  the host an epoch raise still excuses a change.
- Listing the whole vmctl and gateway packages puts every commit to them under
  the gate (164 commits in 30 days across the two): the price of not
  enumerating. The route tables of the proxy and platform guest_reach.go files
  include routes that are not guest-reach.
- The per-computer mirror grant (internal/mirrorgrant, written by vmmanager's
  capsule_mirrors.go) is not listed; the 8085 DNAT cleanup in
  reconcileTapIsolation only shrinks reach and is not listed.
- Under the deployed base's copy of the script, which does not know `host:` or
  file entries, enforcement during landing rests on the head's copy.
