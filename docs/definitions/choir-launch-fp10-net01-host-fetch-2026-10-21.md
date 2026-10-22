---
definition_version: 4

# Station FP10 of the launch first pass
# (choir-launch-first-pass-metamission-2026-10-21.md): the second half of
# batch B8, NET-01's host fetch service. Closing general guest egress
# (NET-04) is not this station.
readiness: drafted

review:
  reviewer: none yet; FP0 sends the red commits to the batch panel on the frozen diff
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-21T23:30:00Z
  source:
    canonical_ref: main 53a691be6 (FP6 batch 1 and the guest network inventory on the branch base)
    deploy_identity: unknown to this lane; FP0 reads staging identity
  worktrees:
    - path: /Users/wiz/Dev/lanes/fp10-net01-host-fetch
      status: clean
      class: goal_candidate
      owner: FP10
      touch: goal_owned
      recovery: branch lane/fp10-net01-host-fetch

finish:
  deliver: >-
    A granted computer's research Read, content import, podcast and YouTube
    fetches and source polls run their network hop on the host, behind one
    gateway verb with the address guard, budgets and a log line per fetch;
    any public URL stays readable; ungranted computers behave as today.
  artifact: >-
    Commits on lane/fp10-net01-host-fetch: this file and the problem record
    (green); the host verb POST /provider/v1/fetch and its executor (red);
    the guest transport and the per-computer grant written by vmctl (red);
    podcast search through the guarded client (orange); FP0 proof steps.
  acceptance:
    - action: go test ./internal/hostfetch, ./internal/sourcefetch, ./internal/gateway -run Fetch, ./internal/vmmanager -run HostFetch
      proves: the guard refuses private, loopback, metadata and host-own answers on the host; one hop per call; caps and budgets hold; the grant reaches only named computers
      evidence_class: local test
    - action: FP0 proof steps in the body (grant one test computer, cold boot, Read, import, poll, podcast search; gateway log lines; ungranted computer unchanged)
      proves: capabilities 1 and 3 pass through the host path on staging
      evidence_class: deployed proof
  rollback: >-
    Empty CHOIR_HOST_FETCH_COMPUTERS and cold-boot the computer (the guest
    fetches itself again); git revert of the commits removes the verb.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    The address guard and the fetch record live on the host, where a
    compromised guest cannot skip them, with no capability lost.
  goodharting_would_be: >-
    An allowlist; a host route that follows redirects without re-checking;
    a verb that a granted guest silently bypasses by falling back to direct
    egress; a budget so tight the paper's polls fail.

homotopy:
  realism_axis: >-
    Unit tests with an injected resolver and dialer, then one granted test
    computer on staging, then the paper's computer under its real poll load.

boundaries:
  mutation_class: green (docs); orange (podcast search client); red (gateway route, guest transport, vmctl credential-disk grant)
  authority_sources:
    - SDC NET-01 and "Networking must enable, not starve" (owner, 2026-10-21)
    - docs/reports/guest-network-inventory-2026-10-21.md
    - FP0's station brief (selectable per computer, off by default)
  must_preserve:
    - No allowlist; arbitrary public URLs readable.
    - Off by default; an ungranted computer's fetch path is unchanged.
    - A granted computer never falls back to direct egress (fails closed).
    - No guest image change; no firewall change; no store schema change.
  excluded:
    - NET-04 (closing the guest NIC), the DNS and time routes, the A9 console decision, MTProto.
  protected_surfaces: [gateway, vmctl credential disk, guest egress]

now:
  status: checkpoint_incomplete
  slice: >-
    Built, tested locally and pushed on the lane branch; off by default.
    FP0 reviews the red commit, lands, deploys, then grants one test
    computer and runs the proof steps below.
  source_ref: 117c72af0
  deploy_identity: unknown to this lane
  candidate:
    id: fp10-host-fetch
    state: ready
    ref: lane/fp10-net01-host-fetch
    base: 53a691be6
    digest: none
    scope:
      - internal/hostfetch/
      - internal/sourcefetch/policy.go
      - internal/gateway/handlers.go
      - internal/gateway/host_fetch.go
      - internal/gateway/host_fetch_test.go
      - internal/vmmanager/host_fetch.go
      - internal/vmmanager/host_fetch_test.go
      - internal/vmmanager/config.go
      - internal/vmmanager/manager.go
      - internal/autoputer/run.go
      - internal/autoputer/host_fetch_route.go
      - internal/agentcore/podcast.go
      - internal/agentcore/podcast_test.go
      - nix/node-b.nix
      - scripts/host-fetch-proof.py
  conjecture:
    id: C-one-seam
    claim: >-
      Every guarded public fetch in the guest builds its client with
      sourcefetch.Client, so one route there moves capabilities 1 and 3 to
      the host without touching each caller, and nothing is lost.
    test: the FP0 proof steps on a granted test computer, journal and API together
    edge: missing_oracle
    delta_o: >-
      The tap-side flow record for the granted computer during the proof
      (step 6) shows whether any public fetch still leaves the guest
      directly; a library that dials on its own would show there.
    scope_if_supported: granted computers at the deployed SHA
    status: testing
    evidence_refs: [docs/problems/guest-fetches-public-internet-itself-2026-10-21.md]
  decision:
    what: the verb lives on the gateway; the switch is a vmctl credential-disk grant; budget 600 hops a minute, 16 in flight per computer
    kind: safety
    status: settled
    evidence_ref: station brief (selectable per computer, off by default); visual-capture grant pattern
    owner_ratification_ref: not_applicable
  belief:
    believed_state: >-
      Off by default, ungranted computers unchanged; a granted computer's
      guarded fetches cross the gateway one hop at a time, refused there for
      private, metadata and host-own answers; podcast search is guarded.
    main_uncertainty: >-
      Whether 600 hops a minute covers the paper's peak (feeds plus article
      imports plus redirects); whether some caller sets headers the relay
      changes (Accept-Encoding is passed through as set).
    next_observation: FP0 proof step 4 journal counts on the granted computer
  blocker_or_risk: >-
    The gateway runs as root on Node B with no network hardening; the verb
    makes that process dial guest-chosen public addresses (no parsing on
    the host, but the residual is real). Not in this branch.
  next_action: FP0 reviews 2a73810d4 (red) with the batch panel, lands, runs the proof steps

receipts:
  - id: problem-record-and-station
    boundary: define
    identity: a108bc42e
    proof_refs: [docs/problems/guest-fetches-public-internet-itself-2026-10-21.md]
    rollback_ref: git revert a108bc42e
    disposition: committed
  - id: tests-first
    boundary: implement
    identity: 54da5ffa4
    proof_refs: [internal/hostfetch/host_test.go, internal/hostfetch/transport_test.go, internal/gateway/host_fetch_test.go, internal/vmmanager/host_fetch_test.go]
    rollback_ref: git revert 54da5ffa4
    disposition: committed (does not build alone; the next commit adds the code)
  - id: host-fetch-verb
    boundary: implement
    identity: 2a73810d4
    proof_refs: [go test ./internal/hostfetch ./internal/sourcefetch ./internal/gateway ./internal/vmmanager ./internal/sources ./internal/researchtools ./internal/content, all ok]
    rollback_ref: empty CHOIR_HOST_FETCH_COMPUTERS and cold boot; git revert 2a73810d4
    disposition: committed
  - id: podcast-search-guarded
    boundary: implement
    identity: 117c72af0
    proof_refs: [internal/agentcore/podcast_test.go TestPodcastSearchRefusesAPrivateProviderAddress, failed first then ok]
    rollback_ref: git revert 117c72af0
    disposition: committed
---

# FP10: NET-01's host fetch service

Problem: [guest-fetches-public-internet-itself-2026-10-21.md](../problems/guest-fetches-public-internet-itself-2026-10-21.md).

## What was built

- **The verb.** `POST /provider/v1/fetch` on the gateway
  (`internal/gateway/host_fetch.go`, work in `internal/hostfetch/host.go`).
  The guest sends `{method, url, header, body}`; the host validates the URL
  (`sourcefetch.ValidateURL`), dials through `sourcefetch.GuardedClient`
  with the host's own interface addresses added to the refused set, does
  one hop (redirects come back as 3xx), and relays status and headers in
  `X-Choir-Fetch-Status` / `X-Choir-Fetch-Headers` with the body. GET, HEAD
  and POST only (InnerTube posts). Caps: 1 MiB request, 32 MiB response
  (declared too long: 502; undeclared: the relay aborts), 90 s a hop.
  Budget of its own per computer: 600 hops a minute
  (`CHOIR_HOST_FETCH_PER_MINUTE` on the gateway) and 16 in flight. Log, one
  line per hop: `gateway: fetch computer=<gateway identity> method= host=
  status= bytes= ms=`, or `fetch refused ... reason=`, `fetch failed`,
  `fetch aborted`. Never the path or query.
- **The guest side.** `sourcefetch.SetRoute`, set once at the start of
  `autoputer.Run` from `hostfetch.GuestRoute`. Every client built by
  `sourcefetch.Client` then sends each hop to the gateway and keeps its own
  redirect re-check. Callers covered: research Read and fetch_url, content
  import and the Source Viewer snapshot, Texture media, podcast subscribe
  and refresh, podcast search (moved onto the guarded client by
  117c72af0), YouTube InnerTube and watch page and caption tracks, RSS and
  article imports, the Telegram preview, GDELT (inventory rows A1-A6,
  B1-B6). A granted computer with no gateway URL or token refuses every
  fetch; none falls back to its own egress.
- **The switch.** `CHOIR_HOST_FETCH_COMPUTERS` in go-choir-vmctl's
  environment (`nix/node-b.nix`, empty). A listed ComputerID, or
  `class:test` for every test computer, gets `host-fetch.json` on the
  credential disk at cold boot; the guest reads it next to its gateway
  token. A snapshot resume keeps the old disk: cold-boot to change.

Not changed: the guest image, any firewall rule, the guest NIC, the store.
Ungranted computers behave exactly as before.

## FP0 proof steps

Run after the red commit lands and the deploy's SHA is seen on staging.
`P` is a test computer's ComputerID (not the paper's for the first pass).

1. **Ungranted baseline (capability 3 unchanged).** With the list empty:
   `CHOIR_API_KEY=... scripts/host-fetch-proof.py api --computer P`, then on
   Node B `scripts/host-fetch-proof.py journal --since "<start time>"`.
   Expect every api check PASS and **no** `gateway: fetch` lines. The
   paper's schedules keep polling (the paper liveness probe stays ok).
2. **Grant one test computer.** Set
   `"CHOIR_HOST_FETCH_COMPUTERS=P"` in `nix/node-b.nix`, land, deploy, then
   cold-boot P (stop and start, not a snapshot resume). Expect in the
   vmctl journal `vmmanager: host fetch route for computer P (VM ...):
   granted (computer)` and in P's guest log `autoputer: host fetch route
   on (computer)`.
3. **Capabilities 1 and 3 through the host.** Run the api mode again for P.
   Then a research Read of an arbitrary public URL from P's research desk
   (for example ask research to Read
   `https://en.wikipedia.org/wiki/Special:Random` and
   `http://portquiz.net:8080/`, the second a non-default port).
   Expect every api check PASS and both Reads to return page text.
4. **The journal shows the hops.** `scripts/host-fetch-proof.py journal
   --since "<step 3 start>" --identity <P's gateway identity, vm-...>`.
   Expect `ok` hops for en.wikipedia.org, www.youtube.com, itunes.apple.com,
   feeds.npr.org, feeds.bbci.co.uk and the article hosts, and `refused`
   lines for the three refusal probes (169.254.169.254, localtest.me,
   10.0.0.1). Keep the artifact JSON.
5. **Adversarial (FP0's).** From P, Read Node B's own public address on
   :22 and :443 and a name that resolves to Node B's address. Expect
   `fetch refused ... reason="address: ..."`. Read a page that redirects
   to `http://169.254.169.254/` (any public redirector): expect the guest
   to refuse the second hop with no second gateway line.
6. **No direct egress from P.** During step 3, on Node B: conntrack or a
   tcpdump on P's tap for traffic from P's guest address to anything but
   its host peer `10.x.y.1` on ports 80 and 443. Expect none for the
   fetches above (web search and keyed APIs already go to the gateway).
7. **Budget headroom.** Read P's hop count a minute from the journal while
   a schedule polls; it must stay well under 600. Before granting the
   paper's computer, count the paper's peak (feeds per cycle plus article
   imports) and raise `CHOIR_HOST_FETCH_PER_MINUTE` if needed.
8. **Rollback drill.** Empty the list, deploy, cold-boot P: the guest log
   says `host fetch route off` and step 1 passes again.

Done for NET-01's host half when steps 1 to 8 pass on the test computer.
Granting the paper is a separate step after step 7's count. Closing the
guest's general egress is NET-04 and needs the DNS and time routes and the
A9 console decision first (inventory, "What this table says NET-01 has to
build").

## Residuals

- The gateway process runs as root on Node B with no address-family or IP
  hardening; the verb makes it dial guest-chosen public addresses. A
  dedicated unprivileged fetch user or service is the durable shape.
- GDELT's bare `io.ReadAll` (row A5) is capped only on the host route
  (32 MiB); an ungranted guest still reads without a cap.
- SDC NET-01's "Telegram's MTProto connection given its own host route"
  describes code that does not exist (inventory, Fact 1); nothing is built
  for it. FP0 or the owner should strike or rescope that clause.
- B3 (optional transcript APIs) would send their API key header through
  the verb if configured; they are unset in the image.
- `CHOIR_HOST_FETCH_PER_MINUTE` is a guess sized from the registry (about
  180 feeds a 15-minute cycle); step 7 measures it.
