# /api/compute/status: "immutable identity evidence unavailable" after a self-development promote (2026-10-26)

Status: documented, not fixed. Mutation class of this doc: green. A fix would be orange
(proxy read path, no write authority).

## Observation

2026-10-26 ~23:25 UTC, staging https://choir.news, bearer API key, GET
`/api/compute/status?computer_id=<id>`: `status: "degraded"`, warning
`immutable identity evidence unavailable`, `current_computer.immutable_identity` null, for both
QA computers:

- b@c `computer-6c50d6e566143ae32cdb660faa60e24d` (user 5885aafc-..., slot generation 12)
- d@e `computer-59c902c5d46caecc0a180a8557164e72` (user c061de6f-..., slot generation 5)

Reproduced again after b@c woke (still degraded). Build headers: proxy f6bd05924, autoputer 4831fe67.

## Cause (evidenced, not hypothesis)

`currentImmutableIdentity` (internal/proxy/compute_status.go:261) asks vmctl for the route
resolution, then joins it. The join requires `len(resolution.CodeClosure.SourceCommit) == 40`
(a git SHA-1). The route slot of both computers is a self-development promote whose code closure
carries a 64-hex value.

Evidence, read-only (GET on the vmctl unix socket from node-b, host token read on the host and
never printed or sent to a guest; the route resolve endpoint is vmctl `HandleResolveComputerVersionRoute`):

| computer | slot generation | latest receipt kind | committed_at | source_commit length | code_closure.created_at |
|---|---|---|---|---|---|
| b@c | 12 | promote (idempotency key `selfdev-route:...`) | 17:49:55Z | 64 | 17:42:57Z |
| d@e | 5 | promote | 22:57:06Z | 64 | 22:56:15Z |

For b@c every other join leg holds (slot id = latest receipt id, receipt.new = slot.current,
committed generation = slot generation, closure ref = code ref). The 40-length test is the only
failing leg, so the proxy returns "route slot and receipt evidence do not join" internally; it
discards the error text, which is why nothing is logged (proxy and vmctl journals show no
related line; only vmctl's normal "cv-route non-loopback caller" S1a lines at 22:57 and 23:09,
which are the guests' own self-development reads).

Why 64: `internal/agentcore/self_development_materializer.go:407` builds the closure with
`sourceTreeDigest` (`source-tree:sha256:<64hex>`) as `sourceCommit`. The closure validator
(`computerversion.validSourceCommit`, input_resolver.go:207) accepts 40 or 64 hex, so vmctl
stores and serves it happily. The proxy's `!= 40` check (added in e4efcf4a0, 2026-07-21,
"converge durable work lifecycle authority") was written for platform-update closures
(`internal/platform/platform_update.go:216`, a real git commit) and never learned about the
self-development closure shape.

Consequence: any computer whose route slot was last advanced by a self-development apply reports
degraded. Platform-follow slots (40-hex) are unaffected. A route-absent computer is fine
(returns `{route_absent:true}`, non-nil).

## Questions asked

- Bearer-only? No. The cookie path calls the same function keyed on `authResult.UserID`; it
  fails identically for any user in this state. (Not exercised: only the two bearer keys were
  available; no owner session was touched.)
- Other consumer: `GET /api/acceptance/execution-identity` (`execution_identity.go:419`) calls the
  same join and returns 503 "execution identity route join unavailable", so `choir identity`
  will fail for these computers once they are active. (Not exercised: it requires an
  `acceptance:read`/admin key and an active computer; the path `/api/execution/identity` I first tried is 404, wrong route.)
- Regression from today's trains? No. bearer-wake f6bd05924, X1 cadence, D2 retention and the
  jailer waves do not touch this code (last commits to compute_status.go / route_client.go are
  10-10 to 10-14; route_authority.go last changed 10-24). The trigger is data: the QA
  computers did a self-development promote today (17:49Z and 22:57Z). It looks new only because
  these are the first computers anyone checked after such a promote.
  Hypothesis (unverified): the owner's computers, if their last route advance was a
  self-development apply, show the same degradation; I did not read them.

## Impact

- `/api/compute/status` is "degraded" with a false-positive-class warning for those computers;
  any monitor or acceptance proof that treats `status != ok` or a null `immutable_identity` as
  failure will fail, and the identity/acceptance route is unusable for them.
- No runtime effect: vmctl routing, wake, and the guest are unaffected.
- It hides the real signal: the degraded flag no longer separates "evidence missing" from
  "evidence shaped differently".

## Smallest fix options (not applied)

1. Proxy: accept 40 or 64 hex (reuse the validator's rule, ideally an exported
   `computerversion.ValidSourceCommit`), and expose the field as-is. One-line change plus an
   E2E on a self-development-promoted computer. Orange.
2. Same, but name the kind: report `CodeCommit` only for 40-hex and add `SourceTreeDigest` for
   64-hex, since the 64-hex value is a source-tree digest, not a commit. Better truth, slightly
   larger; consumers of `code_commit` need a note.
3. Independent of 1/2: log the join error (proxy currently drops it) and return a distinct
   warning ("route evidence does not join" vs "route authority unavailable"). Green/orange.

Recommended: 2 (or 1 as the stopgap) plus 3.

## b@c persistent data image capacity

Readings (`/api/compute/status`, 23:25Z, then 23:2xZ after wake):

| source | used | total/cap | avail | used % |
|---|---|---|---|---|
| host (hibernated, data.img allocated bytes) | 22,810,963,968 (21.2 GiB) | 34,359,738,368 (32 GiB) | 11.5 GB | 66.4% |
| guest (active, filesystem) | 15,583,993,856 (14.5 GiB) | 33,501,757,440 | 17.9 GB | 46.5% |

- Threshold: `persistentdisk.WarningUsedBytes` = 7 GiB absolute used (usage.go:12); critical when
  avail <= 512 MiB. The default cap is 8 GiB; this computer's cap has been raised to 32 GiB. The
  warning is therefore absolute, not percentage: it has been on since usage passed 7 GiB, and is
  not an indication of imminent exhaustion at a 32 GiB cap.
- Host allocation exceeds guest-used by about 7.2 GB: the sparse image keeps blocks the guest
  has freed (no discard/trim reclaim). The host figure can only be larger than the guest figure.
  When hibernated, status shows the host number, so the warning and "66%" look worse than the
  live filesystem.
- What consumes it: not determined. No read-only per-directory breakdown API exists for the
  guest, and the rules bar guest block-device access, SQL and `du`/`find` of the artifacts
  root. Hibernate logs show the snapshot's own disk delta is small (disk_bytes 0.7 to 2.7 GB per
  snapshot, 08:xx to 23:14), so recurring growth is not obviously the snapshot path. This QA
  computer ran heavy agent sessions (self-development capsules, builds) today, which would
  explain a few GB of caches and event/tape data. Hypothesis only.
- Trajectory: only two observations, no history, so no growth rate. Headroom is 17.9 GB
  (guest) against a 512 MiB critical line. Even at several GB per day it is days to weeks
  from a write failure. The host-side figure (11.5 GB to cap) is the nearer bound if the guest
  does not reuse freed blocks: a guest writing into previously unallocated regions grows the
  host file until the 32 GiB cap, then ENOSPC at the host. Hypothesis, unmeasured.
- Urgency: not urgent. Cheap follow-ups: record guest used bytes on an interval for b@c to get a
  slope; a one-time discard/fstrim on the guest data volume would show how much of the 7.2 GB
  host-over-guest gap is reclaimable; surface in status when `source: host` is the larger,
  stale number.

## Heresy delta

discovered: proxy join assumes a git SHA-1 where the route authority admits two source-id
shapes (validator vs consumer disagree); proxy drops the join error. introduced: none.
repaired: none.
