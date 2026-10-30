# An API key request to a hibernated computer gets 502 and does not wake it

Observed 2026-10-26 17:30 UTC on staging (build d385633b6), by FP0.

## What happened

b@c (computer-6c50d6e5…) was hibernated. Two bearer-key requests failed:

- `choir self-dev list` returned `http 502 {"error":"computer authority unavailable"}`;
- `choir run start` (the prompt bar) returned `http 502 {"error":"failed to resolve user autoputer"}`.

The proxy log reads `computer authority unavailable … state="hibernated"
has_url=true cause=api key computer realization unavailable`. The request
succeeded only after an explicit `choir computer start`, which took 13 s
(a cold boot: the deploy at 16:5x voided the snapshot, the D2 problem).

## Why

By design. `resolveComputerURLForComputerTarget`
(internal/proxy/api_key_computer_authority.go) says a bearer request "never
re-resolves or wakes a logical desktop after authorization"; it serves only
an `active` realization, and recovery is the explicit computer route. A
cookie request does wake.

## Why it matters now

D3 (landed in train 17) hibernates idle computers after 10 minutes instead
of 30, and reaches primary computers. Every agent, the CLI and any external
API client will now meet a hibernated computer most of the time, and the
answer is a 502 whose text names neither the state nor the remedy. The
density station's acceptance says nobody makes room by hand; a client that
must call `computer start` first is the same hand step moved to the client.

## Options (not decided)

1. Keep the no-wake rule and make the refusal honest: 409 with
   `{"error":"computer_hibernated","state":"hibernated"}`; the CLI starts the
   computer and retries once when its key holds `computer:lifecycle`.
2. Wake on a bearer request when the key holds `computer:lifecycle`, through
   the same admission path as `computer start` (D1 makes room).
3. Wake on any bearer request for the key's own computer.

Option 1 keeps the authority rule the code states and is the conservative
default; option 2 is what density wants. Either is orange (proxy behavior),
tier 1.

## Decision (owner, 2026-10-26, in session)

The owner asked: "Why can't we wake a hibernated computer?" and wants a bearer
request to wake its computer. This chooses option 3 below, narrowed: a bearer
request wakes the exact computer its ownership join named, not any other.

### FP0's reading of fbc7ff5a0

The no-wake rule came from fbc7ff5a0 (2026-08-09, "security: bind API keys to
exact owned computers"). Its real invariant is that a bearer request never
re-resolves to a different realization than the exact (UserID, ComputerID)
ownership join it authorized. The cookie path resolves by (user, desktop), so
a replaced or remapped desktop could send a bearer request to a computer the
key was never bound to. That is what the commit forbade. The no-wake was a
side effect of the implementation ("serve only the joined active URL"), not
the invariant.

### Chosen behaviour

On a bearer request whose joined computer is `hibernated` or `stopped`:

1. wake that exact computer through the path the explicit `computer start`
   route uses (`vmctl.ResolveDesktopContext` for the joined (UserID,
   DesktopID)), so vmctl admission and D1's make-room apply; no new wake
   mechanism;
2. wait for it boundedly (the existing `vmctlOpContext` bound, 60 s by
   default), concurrent requests for one (UserID, ComputerID) sharing one wake;
3. re-run the exact ownership join (`LookupComputerContext(UserID,
   ComputerID)`) against the realized computer and require the same
   ComputerID, UserID, DesktopID, kind and an `active` state with a URL; any
   mismatch refuses;
4. serve the request from the re-joined realization.

A wake that is refused (admission, recovery refusal), times out, or lands on a
mismatching realization answers `503` with
`{"error":"computer_waking_failed","state":"<state before the wake>","cause":"..."}`
(plus `Retry-After`), never the opaque 502. A state that cannot be woken by a
request (`booting`, `stopping`, `degraded`, `failed`) gets the same 503 body
naming that state. No feature switch; rollback is a git revert.

Routes that mutate the guest as an owner act (`restore`, `replace-workspace`,
`rematerialize-from-tape`, `import-residue-snapshot`, `bootstrap-chain`) keep
the strict "active only" rule: the cookie path does not wake for them either
(rematerialize falls to cold recovery), and a wake is not part of what the
owner authorized there.

## Failure modes

- The wake resolves a different computer (desktop remapped or replaced between
  the join and the wake): the post-wake join compares ComputerID, UserID,
  DesktopID and kind; mismatch refuses with 503, and the request is never sent.
- A key bound to computer X can never wake Y: the wake target is the joined
  `target`, whose ComputerID is checked against the key's binding before and
  after; an owner-wide key must still name its target before any lookup.
- Admission refuses the wake (no room): 503 with the cause, no storm of
  retries inside the proxy.
- Wake exceeds the bound: 503 naming the timeout; the vmctl wake continues, so
  a retry shortly after is served.
- Wake storm: concurrent bearer requests for one computer share one in-flight
  wake (singleflight keyed by user and computer); a waiter whose request is
  cancelled leaves without cancelling the wake.
- An owner-stopped computer is restarted by the next bearer request: the same
  as a cookie request; recorded as a consequence, not a defect.
- A cause string must not leak vmctl addresses or credentials: causes are
  classified (timeout, vmctl-authored refusal text, unreachable, mismatch).
- `run start` (POST /api/prompt-bar) goes through `resolveComputerURLForComputerTarget`
  (handlers.go), so it wakes by the same path; its old text "failed to resolve
  user autoputer" came from `writeResolveError`.

## Review 339 round: owner acts and residuals

Owner acts never wake (conservative): restore, replace-workspace,
rematerialize-from-tape, import-residue-snapshot, bootstrap-chain, settle and
resume producer reports and checkpoints, self-development approve, rollback,
adopt and rebase-retry, and release and package publish all use the strict
active-only resolver. Reads (self-development operations, producer-report and
checkpoint listing, and the general API routes) wake. The route is checked
before the wake. A wake refused by a vmctl recovery refusal renders the wake
body (with reason, kind and Retry-After), and `cause` is fixed text per known
refusal kind, never vmctl's own text.

Residuals, recorded and not fixed here:

- Remap-window wake of Y (DeepSeek F1, Devin F5): the wake names the joined
  (UserID, DesktopID); if the desktop is remapped to another computer Y between
  the join and the resolve, vmctl may wake Y. The post-wake re-join refuses to
  serve it, but Y has been started.
- Shared-flight waiters are not revalidated against their own join (DeepSeek
  F3): a waiter takes the flight's re-joined result, keyed by user and computer,
  without its own desktop selectors being re-checked after the wake.
- Non-primary desktops cannot wake (DeepSeek F4): `ResolveDesktopContext`
  routes only the primary desktop, and the test fixture masks it by waking by
  desktop name.
- Wakes bypass the lifecycle durable-intent ledger (Devin F4): `computer start`
  records prepare and complete; a bearer wake records nothing.
- A `read:runtime` key can now cause a start (DeepSeek): a read scope is enough
  to wake a hibernated computer.

## Status

Decided 2026-10-26; implemented on lane/bearer-wake (red: proxy to vmctl
lifecycle). Successor: the density station D (this is D3's client-side
consequence).

Superseded in part 2026-10-30: `degraded` and `failed` are woken by a request
too, under vmctl's crash-loop bound
(docs/problems/crashed-computer-stays-down-until-owner-start-2026-10-30.md).
`booting` and `stopping` still are not.
