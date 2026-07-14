# Public Pulse App Mission v0

This paradoc defines the usage/health dashboard mission for launch. The app
should make Choir's public operational pulse visible to everyone without
creating private surveillance data. The intended product stance is radical
transparency: aggregate launch facts are public; private user behavior is not
collected, stored, or exposed.

Product name: **Pulse**. Public shell title: **Choir Pulse**.

Rationale: "Metrics" is accurate but sounds like an internal admin console.
"Pulse" reads as a public product health and adoption surface. The app uses the
title "Choir Pulse" and describes itself through its data, not a marketing or
admin framing. Metrics remains the ordinary noun for displayed counters. The
implementation uses the stable app id `pulse` and API namespace
`/api/pulse/*`.

## Product Shape

Pulse is a normal Choir app surface, visible to every user and safe for public
preview. It is not an owner-only analytics console. It should answer:

- How many real people are using Choir?
- Is the system healthy enough for those people to keep using it?
- Is launch growth creating operational pressure?
- Are test/Codex accounts and real accounts separated clearly?

The app should show aggregate counts and coarse operational health:

- real users total;
- new real users over 24h / 7d / 30d;
- active real users over 24h / 7d / 30d, based on coarse product activity or
  computer liveness buckets;
- real-user primary computers by state: active, hibernated, booting, failed,
  inaccessible;
- launch reliability counters: bootstrap failures, recovery attempts,
  auth/session errors, prompt submission errors, and deploy health;
- storage split: real-user VM state, Codex/test VM state, Nix/store headroom,
  manual snapshot bytes;
- public freshness: when metrics were computed and which deployed commit they
  describe.

Pulse should not show or store:

- prompts, document text, VText content, traces, messages, source histories, or
  generated artifacts;
- per-user timelines, per-user sessions, per-user action histories, or
  "user X did Y" records;
- IP addresses, geolocation, user agents, referrers, device fingerprints, or
  session replay;
- email addresses except possibly a public count of protected/internal/test
  accounts by class;
- hidden admin-only metrics that are more invasive than the public surface.

## Data Doctrine

Pulse should be built from aggregate operational facts that already exist or
from coarse rollups created specifically for public display. The ideal v0 does
not create a new row-level analytics event stream. If a counter cannot be
computed without storing private behavioral telemetry, omit the counter.

The classification boundary is load-bearing:

- `real`: launch/alpha users who are not Codex/internal/test identities;
- `codex_agentic_test`: `example.com`, `example.test`, and explicitly
  configured Codex-generated accounts;
- `protected_test`: `a@b.com`, `b@c.com`, and other owner-declared test
  identities that must not be confused with real launch adoption;
- `internal`: operator/platform accounts that are not external users;
- `unknown`: accounts that require review before appearing in real-user
  counts.

Pulse should make the classification rules visible enough that public users can
understand what is counted. It should avoid small, invasive breakdowns. For
example, "real users active this week" is acceptable; a table of each active
user's last activity is not.

## Parallax State

status: settled

mission conjecture: if Choir ships a public Pulse app that exposes only
aggregate, classification-aware operational facts under strict no-surveillance
invariants, then launch operators and users can see adoption and health without
creating private analytics liability. Status: supported for v0 launch scope by
staging app/API/browser proof.

deeper goal (G): launch next week with trust-preserving transparency: the
owner can monitor real adoption and system health, users can see the same
public facts, and Choir avoids accumulating private behavioral data that would
become a liability.

witness/spec (A/S): a public read-only Choir app named Pulse / Choir Pulse plus
a minimal aggregate API/model that reports launch usage, computer health,
storage pressure, and freshness. The app is visible from the product shell,
safe for public preview, and backed by aggregate-only data.

invariants / qualities / domain ramp (I/Q/D): no content telemetry; no
per-user behavioral drilldown; no IP/geolocation/device/referrer collection;
no private operator-only analytics superset; real/test/internal classification
is explicit and testable; all displayed metrics are public-safe; app UI is
quiet, operational, and not a generic SaaS dashboard. Ramp from fixture
aggregate JSON, to local app surface, to staging aggregate endpoint, to public
staging proof with seeded real/test accounts.

variant (ranking function) V: original 8 open obligations:
1. decide final public app name and app id;
2. define account classification rules and protected test identities;
3. define v0 aggregate metric schema;
4. prove schema contains no private content or per-user behavior;
5. implement aggregate API or projector;
6. implement Pulse app surface in the Choir app registry;
7. add tests for classification and privacy invariants;
8. prove staging/public read-only behavior and freshness. Current V=0; last
delta closed stale state/report mismatch after staging evidence was already
recorded in the ledger.

budget: one launch-prep implementation mission, spent. Solvency result: v0 was
feasible because it stayed to aggregate counts and existing operational data.
Future expansion into attribution funnels, private admin analytics, or per-user
support drilldown requires a new privacy conjecture.

authority / bounds: v0 implemented as a public aggregate app/API and UI. Do not
add new row-level analytics collection, private operator-only metrics, or
identity-level drilldown without explicit owner approval and a new privacy
conjecture.

mutation class / protected surfaces: doc state is green. The implemented
product slice was yellow/orange: tests/schema plus public product API/UI.
Protected surfaces were auth account data, VM ownership/state, VText/Trace and
content stores as negative boundaries, session/auth logs as negative
boundaries, frontend app registry, public API routes, and launch dashboard
storage.

evidence packet: behavior commit `9b39b84d9fa7d964847c2bb8cfb4bce7ba85f166`;
browser fix commit `a92498e2349bfb1965a577123e019d4e0c7da369`; CI/deploy run
`27509667041` for app/API and run `27509797114` for the frontend fix; staging
health reported deployed commit `a92498e2349bfb1965a577123e019d4e0c7da369`.
`https://choir.news/api/pulse/summary` returned HTTP 200 in `0.122590s`.
Signed-out browser proof rendered `REAL 3`, `CODEX TEST 376`, `PROTECTED TEST
2`, and `TOTAL 381`; screenshot saved outside repo at
`/tmp/pulse-staging-final.png`. Privacy grep found no raw email addresses,
`user_id`, `trace_id`, `session_id`, `ip_address`, or `user_agent` fields.

heresy delta: discovered the launch need for adoption/health visibility;
introduced risk that "metrics" could become private surveillance if scoped
poorly; repaired for v0 by shipping public aggregate transparency without a
private telemetry store.

position / live conjectures / open edges: "Pulse" is settled as the v0 product
name and "Choir Pulse" as the public title. The owner accepts public aggregate
facts, including small launch counts, as radical transparency. Still avoid
narrow cohort breakdowns that identify people by implication. Remaining
hardening axis is cache/rate-limiting for public storage sampling if traffic
rises; this does not block v0 launch visibility.

next move: no required Pulse v0 work remains before resuming the Node B
Nix-store retention paramission. Future Pulse v1 work should open a successor
mission for cache/rate-limit policy, richer public reliability counters, and
classification review workflow.

ledger file: docs/mission-public-pulse-app-v0.ledger.md

version / lineage: v0 opened before the Node B Nix-store retention mission at
the owner's request, implemented, and settled on staging. Related completed
mission: `docs/mission-node-b-storage-retention-v0.md`. Related resumed
mission: `docs/mission-node-b-nix-store-retention-v0.md`.

learning state: retain Pulse privacy doctrine here. Promote stable
classification/privacy rules into app/API docs or the operating contract only
after they recur in launch operations.

settlement: settled only when Pulse is available as a public read-only Choir
app on staging, real/test/internal classification is tested, displayed metrics
are aggregate-only and public-safe, no private analytics store is introduced,
and a public proof shows useful launch health/adoption data without exposing
content, email lists, IPs, or per-user behavior. Status: satisfied for v0 by
the evidence packet above.

## Suggested Goal String

```text
/goal Use Parallax on docs/mission-public-pulse-app-v0.md. Treat it as the settled v0 public usage-dashboard app mission opened before the Node B Nix-store retention paramission. Product name is Pulse / Choir Pulse, app id `pulse`, and product stance is radical transparency with public aggregate facts and no private surveillance data. Current status is settled for v0: public staging app/API shipped, classification/privacy invariants were tested, aggregate-only public-safe metrics rendered signed out, no private analytics store was introduced, and evidence shows content/email/IP/per-user behavior are not exposed. Do not reopen v0 unless auditing the evidence packet or correcting factual drift. For new work, create a successor Pulse v1 paradoc for cache/rate-limit policy, richer public reliability counters, or classification review workflow. Ledger: docs/mission-public-pulse-app-v0.ledger.md.
```
