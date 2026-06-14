# Public Pulse App Mission v0

This paradoc defines the usage/health dashboard mission for launch. The app
should make Choir's public operational pulse visible to everyone without
creating private surveillance data. The intended product stance is radical
transparency: aggregate launch facts are public; private user behavior is not
collected, stored, or exposed.

Working name: **Pulse**.

Rationale: "Metrics" is accurate but sounds like an internal admin console.
"Pulse" reads as a public product health and adoption surface. The app can use
the title "Choir Pulse" and describe itself through its data, not a marketing
or admin framing. The implementation can still use a stable app id such as
`pulse` and API namespace such as `/api/pulse/*`.

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

status: open_handoff

mission conjecture: if Choir ships a public Pulse app that exposes only
aggregate, classification-aware operational facts under strict no-surveillance
invariants, then launch operators and users can see adoption and health without
creating private analytics liability.

deeper goal (G): launch next week with trust-preserving transparency: the
owner can monitor real adoption and system health, users can see the same
public facts, and Choir avoids accumulating private behavioral data that would
become a liability.

witness/spec (A/S): a public read-only Choir app named Pulse plus a minimal
aggregate API/model that reports launch usage, computer health, storage
pressure, and freshness. The app must be visible from the product shell, safe
for public preview, and backed by aggregate-only data.

invariants / qualities / domain ramp (I/Q/D): no content telemetry; no
per-user behavioral drilldown; no IP/geolocation/device/referrer collection;
no private operator-only analytics superset; real/test/internal classification
is explicit and testable; all displayed metrics are public-safe; app UI is
quiet, operational, and not a generic SaaS dashboard. Ramp from fixture
aggregate JSON, to local app surface, to staging aggregate endpoint, to public
staging proof with seeded real/test accounts.

variant (ranking function) V: 8 open obligations:
1. decide final public app name and app id;
2. define account classification rules and protected test identities;
3. define v0 aggregate metric schema;
4. prove schema contains no private content or per-user behavior;
5. implement aggregate API or projector;
6. implement Pulse app surface in the Choir app registry;
7. add tests for classification and privacy invariants;
8. prove staging/public read-only behavior and freshness. Current V=8.

budget: one launch-prep implementation mission. Solvency: feasible if v0 stays
to aggregate counts and existing operational data; insolvent if it expands into
general analytics, attribution funnels, or per-user support drilldown.

authority / bounds: planning document only until implementation begins. Future
implementation is yellow/orange depending on whether it only adds aggregate
read APIs and UI or changes auth/account storage. Do not add new row-level
analytics collection without explicit owner approval and a new privacy
conjecture.

mutation class / protected surfaces: current doc is green. Future app/API work
is yellow for tests/schema and orange for product API/UI. Protected surfaces
include auth account data, VM ownership/state, VText/Trace/content stores,
session/auth logs, frontend app registry, public API routes, and any launch
dashboard storage.

evidence packet: app registry entry, aggregate schema, privacy-invariant tests,
classification tests covering `example.com`, `example.test`, `a@b.com`,
`b@c.com`, and `yusefnathanson@me.com`, staging endpoint response, screenshot
or browser proof of the public app, deployed commit identity, and a negative
proof that the endpoint/app does not expose content, email lists, IPs, or
per-user timelines.

heresy delta: discovered the launch need for adoption/health visibility;
introduced risk that "metrics" could become private surveillance if scoped
poorly; repaired only when the app proves public aggregate transparency without
private telemetry.

position / live conjectures / open edges: the name "Pulse" is the current best
candidate because it communicates public health rather than admin analytics.
The hardest edge is low-count launch privacy: exact totals may reveal that a
small alpha cohort changed. The owner's product stance accepts public aggregate
facts, but the app should still avoid narrow cohort breakdowns that identify
people by implication.

next move: inspect current auth/account and vmctl data model, then draft the
v0 Pulse aggregate schema and classification function before implementing UI.

ledger file: docs/mission-public-pulse-app-v0.ledger.md

version / lineage: v0 opened before the Node B Nix-store retention mission at
the owner's request. Related completed mission:
`docs/mission-node-b-storage-retention-v0.md`. Related pending mission:
`docs/mission-node-b-nix-store-retention-v0.md`.

learning state: retain Pulse privacy doctrine here until implemented; promote
stable rules into the app/API docs or operating contract after staging proof.

settlement: settled only when Pulse is available as a public read-only Choir
app on staging, real/test/internal classification is tested, displayed metrics
are aggregate-only and public-safe, no private analytics store is introduced,
and a public proof shows useful launch health/adoption data without exposing
content, email lists, IPs, or per-user behavior.

## Suggested Goal String

```text
/goal Use Parallax on docs/mission-public-pulse-app-v0.md. Treat it as the launch usage dashboard app source program before running the Node B Nix-store retention paramission. Current status is open_handoff: working name is Pulse / Choir Pulse, app id likely pulse, product stance is radical transparency with public aggregate facts and no private surveillance data. Build a public read-only Choir app and aggregate API/model for real-user counts, new/active real-user buckets, primary-computer health, launch reliability counters, storage split, and freshness. Invariants: no prompts, docs, traces, messages, source histories, generated artifacts, per-user timelines, IPs, geolocation, user agents, referrers, device fingerprints, session replay, email lists, or private operator-only analytics superset. Classification must separate real users from codex_agentic_test example.com/example.test accounts, protected_test a@b.com/b@c.com, internal accounts, and unknown review-needed accounts. First next move: inspect current auth/account and vmctl data model, then draft the v0 Pulse aggregate schema and classification function before implementing UI. Ledger: docs/mission-public-pulse-app-v0.ledger.md. Settlement requires public staging Pulse app, tested classification/privacy invariants, aggregate-only public-safe metrics, no new private analytics store, deployed proof, and evidence that content/email/IP/per-user behavior are not exposed.
```
