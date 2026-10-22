---
definition_version: 4

# Station FP10 / NET-10 of the launch first pass
# (choir-launch-first-pass-metamission-2026-10-21.md): network policy for
# app agents. Station file and design in one. Authority: FP0's assignment of
# 2026-10-22 (green design first; the red code behind an off-by-default
# switch only if the design is small and fail closed, else stop with the
# build plan); SDC NET-10, NET-01, NET-05, ARCH-02, ARCH-03, ARCH-09 and
# capabilities 1-7 of "Networking must enable, not starve".
readiness: drafted

review:
  reviewer: none yet; FP0 sends the red commit to the bunch panel on the frozen diff
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-22T03:00:00Z
  source:
    canonical_ref: main 51cb3a8cd (rebased onto e04c66d6a)
    deploy_identity: unknown to this lane; FP0 reads staging identity
  worktrees:
    - path: /home/agent/lanes/fp10-net10-app-policy
      status: clean
      class: goal_candidate
      owner: FP10 NET-10
      touch: goal_owned
      recovery: branch lane/fp10-net10-app-policy from 51cb3a8cd

finish:
  deliver: >-
    One written answer to "how does an app package declare its agent's
    ingress and egress, where does the host enforce it, and how does the
    desk manifest carry it", built on NET-01's host fetch verb, NET-05's
    grant object and ARCH-02/ARCH-03's manifests; plus the first red slice:
    the typed declaration, its fail-closed validation, its translation into
    NET-05 grant requests and the pure widening diff, unwired and off.
  artifact: >-
    Commits on lane/fp10-net10-app-policy: the problem record and this file
    (green); the failure-mode tests (yellow); internal/appnet (red, unwired,
    off by default); the now card (green).
  acceptance:
    - action: go test ./internal/appnet ./internal/egressgrant ./internal/appdesk ./internal/deskmanifest
      proves: every failure mode in §6 is refused; the mode is off unless set; a grant request validates under egressgrant; any change that is not provably narrower is a widening
      evidence_class: local test
    - action: FP0's no-effect proof for the inert set (§8, step 0)
      proves: the unwired package changes no running path
      evidence_class: deployed proof
  rollback: >-
    git revert of the red commit; nothing imports internal/appnet, so there
    is no switch to flip and no state to clean.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    An app agent that needs one API can get exactly that API, approved by
    the owner entry by entry, enforced on the host where the guest cannot
    skip it, and an upgrade cannot widen it silently.
  goodharting_would_be: >-
    A manifest field the guest reads and the host never checks; an egress
    verb that falls back to research's open fetch; a widening diff that
    calls an unrecognised change "narrower"; ingress as a port into the
    guest.

homotopy:
  realism_axis: >-
    Pure validation and diff in unit tests (this station), then the
    install path writing requested grants on a test computer, then one
    fixture app desk calling one approved API through the host on staging,
    then the first real app agent (Mail) on a test computer.

boundaries:
  mutation_class: green (problem record, this file, now card); yellow (tests); red (internal/appnet, unwired)
  authority_sources:
    - SDC NET-10, NET-01, NET-05, ARCH-02, ARCH-03, ARCH-09 and "Networking must enable, not starve" (owner, 2026-10-21)
    - docs/design/app-agent-network-policy-2026-10-21.md (the earlier NET-10 design; this file supersedes its §2 field list and §7 order)
    - docs/design/egress-grant-object-2026-10-21.md (NET-05)
    - docs/design/app-package-carries-app-desk-2026-10-21.md (ARCH-03)
    - lane/fp10-net01-host-fetch, docs/definitions/choir-launch-fp10-net01-host-fetch-2026-10-21.md (NET-01, not on main)
  must_preserve:
    - Research's open read stays open (no allowlist for research, capability 1). An app desk's allowlist is its own path, never a narrowing of research's.
    - A manifest declares, it never grants; only an owner act decides a grant (NET-05).
    - Ingress never connects into the guest; it lands on a host service and reaches the desk as a record.
    - Fail closed everywhere; off by default; unwired in this station.
    - No store schema change, no boot-path change, no gateway or vmctl change in this station.
  excluded:
    - Wiring into appdesk.Validate, the install path, the store, the owner act, Texture, the gateway and maild (§7, W1 to W7, each its own red mission).
    - NET-04 (closing the guest NIC); non-mail ingress services; per-use confirmation for money and irreversible effects.
  protected_surfaces: [none touched in this station; the plan touches gateway and provider calls, auth (owner acts), the object store schema]

now:
  status: checkpoint_incomplete
  slice: >-
    Design and build plan committed; W0 (internal/appnet) built, tested
    locally and pushed, unwired and off. Bunch 11: flash accept, Devin
    accept with edits; the edits are d84590b45 (tests) and aad05f37e (fix),
    and this file records the Parse size cap as W1's. Owner ruling Q3
    (owner notes §40): preset policies; §5.1 maps them onto egressgrant
    and names W2's changes (docs only). FP0 reviews the
    delta, lands W0 with an inert set under that set's no-effect proof
    (§8 step 0). W1 to W7 are not started.
  source_ref: aad05f37e
  deploy_identity: unknown to this lane
  candidate:
    id: fp10-net10-w0
    state: ready
    ref: lane/fp10-net10-app-policy
    base: e04c66d6a
    digest: none
    inert: true (a new unwired package; nothing imports it)
    scope:
      - internal/appnet/appnet.go
      - internal/appnet/appnet_test.go
      - docs/problems/app-package-cannot-declare-network-2026-10-22.md
      - docs/definitions/choir-launch-fp10-net10-2026-10-22.md
      - docs/design/app-agent-network-policy-2026-10-21.md (one status line)
  conjecture:
    id: C-declare-is-a-grant-request
    claim: >-
      An app manifest's network block needs no second validator: every
      egress entry is an egressgrant app-desk declaration at L1, and the
      host can enforce it with egressgrant.Check alone.
    test: W5's host route on a test computer (§8 steps 2-4)
    edge: missing_oracle
    delta_o: the host's refusal lines for out-of-scope uses in §8 step 4
    scope_if_supported: app desks on granted test computers
    status: supported locally (validation and requests); enforcement untested
    evidence_refs: [internal/appnet/appnet_test.go]
  decision:
    what: >-
      W0 only; app egress at L1 on its own gateway route (not NET-01's open
      verb); ingress source mail only; any credential, money and
      irreversible refused until W7
    kind: safety
    status: settled for this pass (conservative defaults; owner questions in §9)
    evidence_ref: this file §2, §4, §9
    owner_ratification_ref: not_applicable
  belief:
    believed_state: >-
      The declaration and its rules exist as pure code; nothing enforces or
      reads them; appdesk still refuses all egress.
    main_uncertainty: >-
      Whether W1's install event shape (approved grant ids) fits S7-0's
      package event when it lands; whether host-side desk attribution
      (NET-03) is needed before the first real app agent.
    next_observation: FP0's delta review of aad05f37e
  blocker_or_risk: >-
    W5 depends on NET-01 landing (lane/fp10-net01-host-fetch); W1 depends on
    S7-0's install events reaching main.
  next_action: >-
    FP0 reviews the bunch 11 delta (d84590b45, aad05f37e) and lands W0
    inert. The owner's Q3 ruling (preset policies, §5.1) changes the W1
    and W2 plans only; W0 needs no change to land.

receipts:
  - id: problem-record-and-design
    boundary: define
    identity: d1809206a
    proof_refs: [docs/problems/app-package-cannot-declare-network-2026-10-22.md]
    rollback_ref: git revert d1809206a
    disposition: committed
  - id: tests-first
    boundary: implement
    identity: d708699ec
    proof_refs: [internal/appnet/appnet_test.go]
    rollback_ref: git revert d708699ec
    disposition: committed (does not build alone; the next commit adds the code)
  - id: appnet-w0
    boundary: implement
    identity: ef051b761
    proof_refs: [go test ./internal/appnet ok, -race ok, go vet clean; go test ./internal/egressgrant ./internal/appdesk ./internal/deskmanifest ok; no importer of internal/appnet]
    rollback_ref: git revert ef051b761
    disposition: committed
  - id: bunch-11-edits
    boundary: review
    identity: aad05f37e (tests d84590b45)
    proof_refs: [go test -race ./internal/appnet ./internal/egressgrant ok (GOMAXPROCS=2 GOFLAGS=-p=1), go vet clean]
    rollback_ref: git revert aad05f37e d84590b45
    disposition: committed
---

# FP10 NET-10: network policy for app agents

Problem: [app-package-cannot-declare-network-2026-10-22.md](../problems/app-package-cannot-declare-network-2026-10-22.md).
Earlier design: [app-agent-network-policy-2026-10-21.md](../design/app-agent-network-policy-2026-10-21.md)
(its rules §1, approval §3 and ingress flow §4 stand; its §2 field list and
§7 order are replaced by §2 and §7 here, because the code they anticipated
now exists).

## 1. What is on main, and on the lanes

| Piece | Where | State | What NET-10 uses |
|---|---|---|---|
| Desk manifest (ARCH-02) | `internal/deskmanifest` | landed | `Desk` row, `NewAppDesk`, `appDeskVerbs` (no egress verb), `KindApp`, desk_cell sandbox with an empty network namespace |
| App package carries its desk (ARCH-03) | `internal/appdesk` | landed, `Activate` has no caller, `CHOIR_APP_DESKS` off | `Manifest`, `Validate`, `Derive`, install event binds the manifest digest; `Egress []string` refused outright |
| Grant object (NET-05) | `internal/egressgrant` | landed, unwired | `Declaration` with `HolderAppDesk` and the app fields, `Request`, `Decide` (owner act only), `Check`, 90-day app lifetime cap, no wildcard and no L4 for app desks |
| Host fetch verb (NET-01) | `lane/fp10-net01-host-fetch` | built, not landed | `POST /provider/v1/fetch`, one hop per call, address guard on the host, admitted per VM ID, no grant id |
| Compute capsule (ARCH-09) | design | not built | its C8 rule: a compute spawn asking for egress is refused until NET-10 |
| Ingress | `internal/maild` | mail only | maild terminates Resend, verifies Svix, stores on the host |

## 2. The declaration (what an app package writes)

The desk member of the package manifest (`appdesk.Manifest`) gains one
optional member, `network`, replacing `egress_verbs`:

```json
"network": {
  "egress": [{
    "verb": "calendar.sync",
    "destinations": [{"scheme": "https", "host": "www.googleapis.com", "path_prefix": "/calendar/v3/"}],
    "methods": ["GET"],
    "effect_class": "read",
    "data_ceiling": "owner-private",
    "credential": "",
    "budget": {"period": "day", "max_requests": 2000, "max_download_bytes": 67108864, "max_upload_bytes": 1048576}
  }],
  "ingress": [{"source": "mail", "address": "calendar"}]
}
```

**Egress entry** (`appnet.EgressEntry`). Each entry is the NET-05
declaration minus the fields the platform fills in (holder, computer,
level, window): `verb`, `destinations`, `methods`, `effect_class`,
`operation_contract`, `data_ceiling`, `credential`, `budget`. The
platform sets holder `{app_desk, app:<package>}`, the installing
computer, level `L1` (one hop through the host fetch verb) and an
install-bound lifetime from the install time (§5.1; owner ruling Q3).
So one validator decides it:
`egressgrant.Request`. NET-10 adds only what is package-specific:

- the verb is `<package>.<name>`, so two packages never share a verb;
- `data_ceiling` is required (NET-05 leaves it optional);
- `effect_class` `money` and `irreversible` are refused in this pass:
  they need per-use owner confirmation, which does not exist;
- a `read` entry lists `GET` and `HEAD` only: POST is dual-use (a POST to
  calendar/v3 creates events) and OPTIONS is refused conservatively; a
  read-over-POST API is a new effect class, not a read (bunch 11 review);
- `credential` must name a platform-held credential; the registry is empty
  in this pass, so any credential is refused (the first credentialed app is
  its own red mission, with the gateway injecting the secret);
- the budget is per period (`day`), not per lifetime (§5.1);
- at most 16 entries, no duplicate verbs.

W0 as built (`internal/appnet` at aad05f37e) still has `renewal_days`
(1 to 90) and a lifetime budget, from before the ruling. W1 replaces them
as planned in §7; no code changes in this docs step.

**Ingress entry** (`appnet.IngressEntry`): `source` and `address` only.
The source names a host service from a closed list; the platform fixes
everything else (authenticity, taint, record shape, batching, volume), so a
package cannot weaken any of it:

| source | host service | authenticity | taint | record | address grammar |
|---|---|---|---|---|---|
| `mail` | maild | Svix HMAC (Resend) | untrusted_input (the package floor) | one observation record to the desk's inbox per batch, listing payload ids | `<package>` or `<package>.<suffix>`, lower case; disjoint because package names and suffixes never contain `.` (`+` stays maild's secret-alias marker) |

Webhooks and push are not sources yet (earlier design, open question 2:
none before launch). Parsing refuses unknown fields, so a manifest that
declares a `port`, `listen`, `forward` or `tunnel` is invalid, not ignored.
At most 8 entries; no duplicate `(source, address)`.

**Approval surface.** Every package desk is untrusted_input (appdesk's
floor), so every non-`read` egress entry is the untrusted-reader-with-write
pairing (earlier design §4); the approval card shows it on each such entry.

## 3. How the desk manifest carries it

- The **package manifest** carries the declaration (`appdesk.Manifest.Network`),
  and the install event binds its digest with the rest (ARCH-03 step 5). The
  owner's per-entry approval is recorded in the same install event as the
  list of approved `grant_id`s and approved ingress keys.
- The **desk row** (`deskmanifest.Desk`) gains `Egress []string`: the verbs
  whose grants are approved, filled by `appdesk.Derive` from the install
  event's approved list, never from the declaration. `Derive` stays a pure
  fold of tape facts: it does not read the mode, so a replay always derives
  the same row (the mode gates the install write, not the fold).
- The **verb**: one new choir verb, `Egress(verb, request)`, added to
  `appDeskVerbs`, dispatching the frame action `app_egress`. The daemon
  serves it only when the row's `Egress` lists the named verb
  (`FrameAllowed` plus a verb check), forwards it to the host with the
  grant id, and returns the response to the cell as data. The cell never
  names a URL outside the grant and has no socket (desk_cell's empty network
  namespace).
- **Compute capsules** of an app desk (ARCH-09) hold no egress in this
  pass; a grant never attaches to a capsule (delegation only narrows, and
  the compute capsule has no delegation path yet).

## 4. Where the host enforces it

The guest is not trusted: the daemon's check is the first gate, the host's
is the authority.

1. **Route.** A separate gateway route, `POST /provider/v1/app-egress`,
   reusing NET-01's executor (`internal/hostfetch`: address guard, one hop,
   size and time caps, a log line per hop). Separate so research's open
   read is untouched and the app path always needs a grant: no grant id is
   a 403, never a fallback to the open verb.
2. **Host grant table** (NET-05 §5.5). The proxy writes
   `(computer_id, grant_id, declaration digest, expires_at)` when it mints
   the owner's decide act, and a revocation set when the owner revokes or
   uninstalls. The guest sends the declaration and the grant id; the host
   recomputes the id, looks it up for the caller's computer (the VM ID the
   gateway already authenticates, mapped by vmctl), and runs
   `egressgrant.Check` on the hop's method, host, port and path. A redirect
   comes back as a 3xx; the next hop is checked again.
3. **Budget.** The host counts requests and bytes per grant per period
   (§5.1 b); past the period's budget it refuses until the period rolls,
   and the grant stays granted. Counted against the computer's NET-01 per-minute budget too.
4. **Operation contract.** For `visible-write`, the host checks the
   request body against the declared schema digest before dialing.
5. **Credential.** Injected by the gateway from the platform store; the
   guest never sees it (S4 principle 5). Not built in this pass.
6. **Ingress.** maild keeps a route table `(computer, desk, address)`
   written by the same owner act; mail to an address with no route is
   dropped on the host with a counted receipt; a routed batch reaches the
   desk as one record over the existing host-to-computer channel. Nothing
   listens in the guest.

Residual: the host knows the caller's computer, not its desk. A
compromised guest could spend app desk A's grant on desk B's behalf, but
only inside A's destinations, budget and window. Desk attribution on the
host waits for per-slot identity off the guest (NET-03).

## 5. Approval, upgrade and revocation

As the earlier design §3, with one rule made exact by `appnet.Widening`:
**an upgrade is narrower only if every change is provably narrower**;
anything the diff does not recognise is a widening. Per egress verb:
narrower means every new destination lies under an old one (same scheme,
host and port, path under the old prefix), methods a subset, effect class
and data ceiling not higher, the same credential and operation contract,
each budget field not higher over a period no shorter. A new verb or a new
ingress `(source, address)` widens; a removed one narrows. A widening entry
stays at its old approval (or off, if new) until the owner approves the
diff.

### 5.1 Preset policies (owner ruling Q3, owner notes §40, 2026-10-22)

The owner: "not renewed by me (human) but perhaps renewed by mgmt desk? Or
may be needless ceremony for network that is required for appagent desk
duties; other networking shouldn't be allowed anyway. I lean towards preset
policies for each appagent by default". Taken as:

- **Each app agent ships a preset network policy** in its reviewed
  package: the `network` block of §2. The owner approves it at install
  (one owner act for the package, listing the approved entries) and never
  renews it. The earlier design's §3.6 (renewal by owner act or a
  pre-approved policy) is withdrawn for app desks.
- **The preset is all the network an app desk has.** An app desk never
  requests a grant at run time; there is no `request_grant` for app desks.
  More network ships as a package upgrade.
- **Any widening is still an owner act** (§5's diff). A narrowing upgrade
  needs none.
- **A preset grant lives as long as its install.** It ends when the owner
  revokes it, the package is disabled or uninstalled, or an upgrade drops
  or replaces the entry. It does not expire on a clock.
- **Whether management re-checks preset grants is open, not a default**
  (§9 Q4). Nothing in the plan builds it.

**How a preset grant maps onto `internal/egressgrant` today, exactly.**
It does not fit without a change:

1. The grant id is the digest of the whole declaration, window included
   (`grantID` over `CanonicalJSON` of the `Declaration`, which carries
   `not_before` and `expires_at`). A renewed window is a different grant.
2. `validateDeclaration` requires a non-zero `not_before` and
   `expires_at`, `expires_at` after `not_before`, and at most
   `MaxAppDeskLifetime` (90 days) for an app desk.
3. `Decide` takes one owner-act receipt whose subject is exactly
   `{grant_id, decision}`, and refuses to grant at or past `expires_at`.
4. `Check` refuses at or past `expires_at` (`ErrExpired`); `Close` moves a
   granted grant to `expired` or `exhausted`, both terminal.
5. `Budget` (`max_requests`, `max_download_bytes`, `max_upload_bytes`) is a
   total for the grant's life; spending it is `exhausted`, terminal.

So with today's code a preset grant would be a 90-day grant that dies on
its clock, or on its budget, and comes back only by a new owner act per
grant: the renewal ceremony the owner ruled out, and an app agent that
stops working on day 90 or at its 2001st request.

**The W2 change it implies (planned, not built):**

- a. `Declaration.Lifetime`: `window` (today's rule, the only one for
  engineering) or `install` (app desks; required for `HolderAppDesk`). An
  `install` grant has `not_before` set to the install time and
  `expires_at` zero; validation refuses an `expires_at` on it and refuses
  `install` on any other holder. The id still digests the declaration, so a
  reinstall is new grants. `Check` skips the clock for `install`; `expired`
  is unreachable for it. `MaxAppDeskLifetime` goes away.
- b. `Budget.Period` (`day`), required for `install`: the host counts per
  period and refuses past the limit until the period rolls
  (`budget_period_exhausted` in the log), without closing the grant.
  `exhausted` stays terminal and stays for `window` grants only.
- c. A package decide: one owner-act subject
  `{package, manifest_digest, grant_ids, decision}` decides every listed
  preset grant at install (a subset if the owner approves only some
  entries). The NET-05 design already named a batch subject as a later
  change; this is it, bound to the install event.
- d. `Narrow(old, next)`: a narrowing upgrade's grant is granted without an
  owner act, carrying the old decision and `narrowed_from: <old id>`, only
  when `appnet.Widening` reports no widening for that entry; the old grant
  closes `revoked` with reason "narrowed by upgrade". A widening entry
  stays a `requested` grant until the owner decides it.
- e. Disable and uninstall revoke with `Revoke` (exists). A crash keeps
  granted preset grants (a standing intent); only an undecided request
  closes `interrupted` (exists).

## 6. Failure modes (written before the tests)

1. A verb not named `<package>.<name>`, or a duplicate verb.
2. A destination that egressgrant refuses: wildcard, IP literal,
   `localhost`, `*.internal`, a non-https scheme, a relative path prefix.
3. No effect class; `money` or `irreversible`; `visible-write` with no
   operation contract; a `read` entry listing anything but GET and HEAD.
4. No data ceiling, or an unknown one.
5. Any credential (the registry is empty).
6. Budget missing, zero or over the hard cap; renewal 0 or over 90 days
   (W0 as built; W1 replaces renewal with an unknown field and a missing or
   unknown `budget.period`, §5.1).
7. More than 16 egress or 8 ingress entries.
8. An ingress source other than `mail`; an address outside the package's
   namespace, including one in another package's namespace (package `mail`
   claiming `mail-calendar-inbox`); a duplicate `(source, address)`.
9. Unknown JSON fields (`port`, `listen`, `forward`, `tunnel`, `host_service`,
   `taint`) refused at parse.
10. Mode off (unset, or anything but `declare`): any non-empty policy refused.
11. A widening reported as narrower: a broader path, a new host, a new
    method, a higher effect class or ceiling, a changed credential or
    contract, a larger budget, a longer renewal (W1: a shorter budget
    period), a new verb or ingress key.
12. A grant request that egressgrant would not accept, or one whose holder
    is not `app:<package>`; any grant request minted with the mode off
    (`Requests` runs `Admit` first, so `Admit` is the only path).

## 7. Build plan (each its own red mission, off or observe first)

| Step | What | Touches | Size | Depends on |
|---|---|---|---|---|
| W0 (this station) | `internal/appnet`: types, `Parse`, `Validate`, `Admit(mode)`, `Requests`, `Widening`; unwired, `CHOIR_APP_NETWORK` off | new package | small | NET-05 (landed) |
| W1 | `appdesk.Manifest.Network` replaces `Egress`; bound the manifest's size before `appnet.Parse` (it has no input cap of its own; bunch 11 review); in `appnet`, drop `renewal_days` (then refused as an unknown field), add `budget.period`, `Requests` sets the install lifetime, `Widening` compares periods instead of renewal; `Validate` calls `appnet.Admit`; install event carries approved grant ids and ingress keys; `Derive` fills `Desk.Egress` | appdesk, appnet, deskmanifest | small | W0, W2a (the lifetime field), S7-0 install events |
| W2 | `egressgrant`: the install lifetime, per-period budgets, the package decide subject and `Narrow` (§5.1 a-e); grant store kind `choir.egress_grant` (online, new kind only); the package decide owner act at the proxy at install (NET-05 §5.1, §5.2) | egressgrant, store, owneract, proxy | medium | NET-05 |
| W3 | Texture approval card at install (the preset, per entry, with the pairing flag) and on a widening upgrade (the diff); no renewal card, no expiry reminder | Texture, frontend | medium | W1, W2 |
| W4 | `Egress` verb, `app_egress` frame action, daemon check | deskmanifest, yaegikernel | small | W1 |
| W5 | host grant table and `POST /provider/v1/app-egress`, `CHOIR_EGRESS_GRANTS=off\|observe\|enforce` | gateway, hostfetch | medium | NET-01 landed, W2 |
| W6 | maild route table and per-desk delivery from the install act | maild, vmctl channel | medium | W2, the email desk |
| W7 | credential injection at the gateway; per-use confirmation for money and irreversible | gateway, owneract | medium | W5 |

W0 is small and fail closed, so this station builds it. W1 to W7 are not
started here.

## 8. Deployed proof steps (FP0's)

**Step 0, now (W0 is inert).** W0 adds an unwired package: it rides an
inert set under the set's no-effect proof (the paper healthy and polling,
routes and observability flags unchanged, boot logs clean). Also confirm
`go list -deps ./cmd/... | grep internal/appnet` is empty on the landed SHA.

**When W1 to W6 land**, on a test computer `P` (never the paper), with
`CHOIR_APP_DESKS=on`, `CHOIR_APP_NETWORK=declare` and
`CHOIR_EGRESS_GRANTS=observe`, then `enforce`:

1. Install a fixture package `netfixture` declaring one `read` egress verb
   `netfixture.get` to `https://example.org/` (GET, a small daily budget)
   and one mail ingress `netfixture`. Expect one install event, one
   requested grant with lifetime `install` and no `expires_at`, one ingress
   route request, and the approval card showing both.
2. Before approval, call `Egress("netfixture.get", ...)` from the desk:
   expect the daemon's refusal (`verb not approved`) and no gateway line.
   POST `/provider/v1/app-egress` directly from P's guest root with the
   declaration: expect 403 `grant_not_granted` and one refused log line.
3. Approve (one package owner act). Call again: expect the page as data and one gateway line
   `app-egress computer=<V> grant=<id> host=example.org status=200`.
4. Out of scope: from the desk ask for `https://example.org.evil.test/`,
   `https://example.com/`, `http://169.254.169.254/` and a POST: each
   refused at the host with `use is outside the grant` (or the address
   guard), no dial.
5. Research unaffected: research Reads an arbitrary public URL on the same
   computer (capability 1) and it succeeds.
6. Ingress: send mail to `netfixture@<owner domain>`: one observation
   record in the desk's inbox, no listener in the guest
   (`ss -ltn` in P shows no new port). Mail to `netfixture.x@...`
   (unrouted) is dropped on the host with a counted receipt.
7. Upgrade: reinstall with the destination widened to `https://example.org/`
   plus `https://example.com/`: expect the widening held pending, the old
   grant still serving example.org, example.com refused.
8. Budget, revocation, uninstall: spend the daily budget: the next call is
   refused `budget_period_exhausted` and the grant stays `granted`. Narrow
   upgrade (drop the path to `https://example.org/a/`): the new grant is
   granted with no owner act and `narrowed_from` the old one. Revoke: the
   next call is refused. Uninstall: the ingress route is gone and mail is
   dropped with a receipt.
9. Restart: crash-restart P mid-call: the call closes "interrupted by a
   restart"; the grant (a standing intent) survives; no call is retried.

## 9. Owner questions (defaults in force, no blocking ask)

1. App-desk egress always at L1 through the host (default), or may an app
   desk ever hold L3 through the capsule proxy? Default: L1 only.
2. Ingress addresses are package-namespaced (`<package>[.suffix]`).
   Default: yes; a vanity alias is a later owner act.
3. Grant renewal for app desks. **Ruled (owner notes §40, 2026-10-22):**
   preset policies, approved at install, never renewed by the owner;
   widening stays an owner act (§5.1).
4. Should management re-check preset grants (for example after an
   incident, or flag ones unused for a long time)? **Open, no default**;
   nothing in the plan builds it. If adopted, management may only narrow
   (flag or revoke), never extend or widen.

Q1 and Q2 were confirmed in owner notes §40.

## Heresy delta

Discovered: the earlier NET-10 design states `internal/deskmanifest` does
not exist (stale since ARCH-02 landed); the host fetch verb cannot carry a
grant, so app egress needs its own route. Introduced: none. Repaired: none
(W0 is the declaration only). Bunch 11 discovered and repaired three
W0 defects: POST under read, overlapping ingress namespaces, grant requests
minted with the mode off. Owner ruling Q3 (§5.1) discovered that a NET-05
app-desk grant cannot outlive 90 days or its lifetime budget without a new
owner act per grant; W2 repairs it (planned).
