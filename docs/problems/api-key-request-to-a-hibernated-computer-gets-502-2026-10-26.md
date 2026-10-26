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

## Status

Recorded; not yet fixed. Successor: the density station D (this is D3's
client-side consequence).
