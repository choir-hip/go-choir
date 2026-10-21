---
definition_version: 4

# Station FP6 of the launch first pass
# (choir-launch-first-pass-metamission-2026-10-21.md): the first half of batch
# B8, the host-mediated network. Authority: the owner's ruling of 2026-10-21
# that the security and density checklist comes before anything for the
# iPhone app, relayed by the orchestrator (FP0) as a change of lane; the
# launch checklist's L2-06 (default: delete Obscura) and the SDC items below.
readiness: drafted

review:
  reviewer: none yet; the orchestrator sends the red commits to a panel on the frozen diff
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-21T19:10:00Z
  source:
    canonical_ref: origin/main 26c4e1ed0 (the first pass leaves the iPhone app out)
    deploy_identity: unknown to this lane; FP0 reads staging identity
  worktrees:
    - path: /Users/wiz/go-choir
      status: dirty
      class: other_agent_wip
      owner: unknown (untracked sourcecycled, ograph-bench, cmd/desktop/frontend/)
      touch: forbidden
      recovery: leave in place
    - path: scratchpad/lanes/b11-mobile-server
      status: clean
      class: other_agent_wip
      owner: the abandoned mobile-server lane (FP5 before the change of lane)
      touch: forbidden
      recovery: leave in place; FP0 cleans it
    - path: scratchpad/lanes/b8-network
      status: clean
      class: goal_candidate
      owner: FP6
      touch: goal_owned
      recovery: branch lane/b8-network, created from 26c4e1ed0

finish:
  deliver: >-
    The guest no longer runs a headless browser on untrusted pages; Web Lens
    opens the original in the owner's own browser and shows a Source Viewer
    snapshot made by the guarded import path; there is one written inventory
    of every outbound destination a guest reaches; and the egress proxy's
    tunnel-identity gap (SEC-06) is closed or has a chosen, testable design.
  artifact: >-
    Commits on lane/b8-network: the problem record and this file (green);
    the Web Lens launcher (orange); the deletion of internal/browsercontrol,
    its routes, config, callers and tests, and the two guest environment
    lines (red); the image change that drops the obscura package (red, its
    own commit and its own deploy); docs/reports/guest-network-inventory-2026-10-21.md
    linked from SDC NET-01 (green); and the SEC-06 change or design (red or
    green, decided in the item).
  acceptance:
    - action: >-
        With an owner session or a read:runtime API key, GET
        /api/browser/capabilities and POST /api/browser/sessions on staging
      proves: the routes are gone (404), so no guest path starts Obscura
      evidence_class: deployed proof
    - action: >-
        Playwright on staging: open Web Lens, enter a public URL, read the
        Open original link's href, target and rel, press the Source Viewer
        snapshot button, and record the network log
      proves: the launcher works, the original opens as a top-level page,
        the snapshot comes from /api/content/import-url, and no /api/browser
        request is made
      evidence_class: deployed proof
    - action: >-
        Research Read of an arbitrary public URL, content import, and one
        source poll on staging after the deploy
      proves: capabilities 1 and 3 of "Networking must enable, not starve"
        still pass (nothing the deletion touches is on those paths)
      evidence_class: deployed proof
    - action: the paper's schedules and polls read before and after the deploy
      proves: the deletion did not take the paper down
      evidence_class: deployed proof
    - action: >-
        On a test computer, from an implementation capsule: pip download of a
        small package, npm view, git ls-remote over https, one huggingface_hub
        download; then a TLS client that CONNECTs to github.com and sends the
        server name pypi.org (steps below)
      proves: the tunnel-identity binding (SEC-06) refuses a name mismatch and
        leaves the ecosystem toolchains working
      evidence_class: deployed proof
    - action: the deploy's image build output, before and after the image commit
      proves: the obscura derivation and its closure are gone, and by how much the image shrank
      evidence_class: deployed proof
    - action: go test of internal/apihandler, internal/provideriface, internal/capsule; go vet of the touched packages; the Web Lens Playwright spec on a built frontend; the two SBOM test scripts
      proves: the tree builds without the package and the route table no longer carries the browser routes
      evidence_class: local test
  rollback: >-
    git revert of the commits, one rollback unit per commit. The image change
    (step 5) reverts independently of the code deletion. Restoring the two
    environment lines restores nothing by itself once the package is
    deleted: revert the deletion commit first.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the ways a signed-in stranger can make a guest-root process
    fetch an address of their choosing, and the places a guest reaches the
    internet that nobody has written down, while every capability in
    "Networking must enable, not starve" keeps working.
  goodharting_would_be: >-
    Routes that return 404 while the binary and its environment still ship;
    an inventory that lists the destinations someone remembered instead of the
    ones the code can dial; a launcher that works in a mocked page but opens
    through a path that still calls the guest; or an egress fix proved only
    against a unit test of a string match.

homotopy:
  realism_axis: >-
    Unit and route-table tests in the lane worktree, then a frozen candidate
    reviewed by a panel, then a staging deploy with the launcher exercised in
    a real browser, then the paper's real load on the deployed image.

boundaries:
  mutation_class: >-
    green (docs); orange (the Web Lens launcher); red (route deletion, guest
    environment, image contents, egress proxy)
  authority_sources:
    - owner ruling 2026-10-21 via FP0 (security and density before the iPhone app)
    - launch checklist L2-06 and batch B8; SDC SEC-06, SEC-07, SEC-15, NET-01, NET-02, NET-05, NET-10
    - docs/choir-doctrine.md and CLAUDE.md
  must_preserve:
    - Research Read, content import and source polls (sourcefetch paths) are untouched.
    - The web_lens open-surface value stays accepted when read; stored source entities that carry it open the launcher.
    - The choir.browser_session object kind stays registered so stored objects keep validating.
    - Public internet egress stays open until NET-01's host routes exist; this station changes no firewall rule.
    - No boot-path step that scans or rewrites og_objects; nothing here touches the store schema.
    - Problem documentation first; the test before the code.
  excluded:
    - Closing general guest egress (NET-04) and the host fetch service itself (NET-01 implementation), which the inventory prepares.
    - Package mirrors beyond goproxy (NET-02), the grant object (NET-05) and app-agent network policy (NET-10): this station inventories and designs only.
    - The Obscura image removal rides its own deploy, not the code deletion's.
  protected_surfaces: [guest boundary and egress, deployment image, the capsule egress proxy, routes served to owners]

now:
  status: checkpoint_incomplete
  slice: >-
    All five asks built and committed on lane/b8-network; nothing is pushed or
    deployed. The orchestrator sends the red commits to a panel, lands them in
    order, and runs the deployed proof below. Batch-1 review edits (GPT-6.1-sol,
    accept with edits) are made: memory=detail now has one request-wide
    deadline and entry budget, batched directory reads, chunked and capped
    mincore probing, a one-at-a-time gate and reported truncation; a stored
    snapshot is shown and imported only while the address is the one it was
    taken from (import always uses that original URL). The frontend spec was
    written but not run: playwright/node_modules are absent in this lane.
  source_ref: a38eb7f67
  deploy_identity: unknown to this lane
  candidate:
    id: fp6-network
    state: ready
    ref: lane/b8-network
    base: 26c4e1ed0
    digest: none
    scope:
      - frontend/src/lib/BrowserApp.svelte
      - frontend/src/lib/source-url.ts
      - frontend/tests/browser-app.spec.js
      - internal/apihandler/routes.go
      - internal/autoputer/run.go
      - internal/provideriface/
      - internal/capsule/egress.go
      - internal/capsule/egress_tls.go
      - nix/autoputer-vm.nix
      - flake.nix
      - .github/scripts/
  conjecture:
    id: C-obscura-unused
    claim: >-
      No product path needs Obscura: deleting it removes the browser half of
      SEC-07 and half of SEC-15 without losing a launch capability.
    test: >-
      A tree-wide grep shows one caller, the Web Lens Snapshot button (done
      in the problem record); after the deploy, research Read, content
      import and a poll still pass, and the launcher shows a snapshot from the
      guarded import path.
    edge: missing_oracle
    delta_o: >-
      A count of browser.session.* events per computer on the tape, read
      through the CLI, would show whether anyone used the button; production
      SQL is off limits, so the check is the deployed proof above.
    scope_if_supported: the guest image and routes at the deployed SHA
    status: testing
    evidence_refs: [docs/problems/obscura-untrusted-javascript-as-guest-root-2026-10-21.md]
  decision:
    what: delete Obscura and browsercontrol; Web Lens becomes a launcher (checklist default)
    kind: safety
    status: settled
    evidence_ref: launch checklist L2-06, owner default stands
    owner_ratification_ref: not_applicable
  belief:
    believed_state: >-
      The only caller of Obscura was the Web Lens frontend; it ran as guest
      root with no address guard; the deletion builds and passes its tests;
      the egress tunnel is bound to its CONNECT name in tests.
    main_uncertainty: >-
      Whether a real toolchain in a deployed capsule sends something the
      ClientHello check refuses (a GREASE Encrypted Client Hello, no SNI), and
      whether the frontend's route-mocked spec matches the deployed desktop.
    next_observation: the deployed proof steps below, run by FP0 on staging
  blocker_or_risk: >-
    The image commit edits nix/autoputer-vm.nix, which the desk-boundary lane
    owns this pass (three Obscura-only removals); the frontend spec
    desktop-shell-core.spec.js needs the local service stack and was not run
    here. The stored browser-session accessors, types and six event kinds
    remain as frozen vocabulary.
  next_action: FP0 reviews and lands the commits in the order in "Landing order"

receipts:
  - id: problem-record-and-goal
    boundary: define
    identity: 53de206e8
    proof_refs: [docs/problems/obscura-untrusted-javascript-as-guest-root-2026-10-21.md]
    rollback_ref: git revert 53de206e8
    disposition: committed
  - id: web-lens-launcher
    boundary: implement
    identity: 126ba5e48
    proof_refs: [frontend/tests/browser-app.spec.js]
    rollback_ref: git revert 126ba5e48
    disposition: committed; 12 route-mocked specs pass locally
  - id: browsercontrol-deleted
    boundary: implement
    identity: e57b7e3cb
    proof_refs: [internal/apihandler/routes_test.go]
    rollback_ref: git revert e57b7e3cb
    disposition: committed; apihandler and provideriface tests pass locally
  - id: guest-network-inventory
    boundary: implement
    identity: b780689e9
    proof_refs: [docs/reports/guest-network-inventory-2026-10-21.md]
    rollback_ref: git revert b780689e9
    disposition: committed
  - id: obscura-image
    boundary: implement
    identity: c2da85986
    proof_refs: [.github/scripts/build-sboms-differential-test, .github/scripts/verify-sbom-candidate-test]
    rollback_ref: git revert c2da85986
    disposition: committed; SBOM scripts pass locally; image not built here
  - id: egress-tunnel-identity
    boundary: implement
    identity: a38eb7f67
    proof_refs: [internal/capsule/egress_tls_test.go]
    rollback_ref: git revert a38eb7f67
    disposition: committed; internal/capsule tests pass locally; six proxy tests fail with the check off
---

# FP6: the host-mediated network, first half

This is the lane that takes the guest's browser away and writes down the
rest of what the guest can reach. The order is the orchestrator's: problem
record, launcher, deletion, inventory, then SEC-06. Each commit is one
rollback unit.

**What changes for an owner.** Web Lens stops being a second browser inside
the computer. It becomes a launcher with two buttons: open the original page
in the device's own browser, and read a Source Viewer snapshot that the
computer made through the same guarded import path research uses.

**What changes for the computer.** No headless browser, no JavaScript engine
beside the daemon, no `/api/browser` routes, about 508 MiB less image
(measured by the checklist's research, to be re-measured at step 5).

**What this lane does not do.** It does not close guest egress. The
inventory (NET-01) is the table that lets a later station do that safely.

## Landing order and deployed proof (written for FP0; no secrets)

Order: 126ba5e48 (launcher; works with or without the routes), e57b7e3cb
(route and package deletion), b780689e9 and 53de206e8 (docs, any time),
c2da85986 (the image; its own deploy, touches `nix/autoputer-vm.nix` on three
Obscura lines only), a38eb7f67 (egress; its own deploy as a red change on the
capsule boundary). 0314e100b and the goal-file commit are docs.

1. **Identity.** Staging reports the pushed SHA (the usual deploy check).
2. **Routes gone.** With a `read:runtime` API key: `GET /api/browser/capabilities`
   returns 404; with a `write:runtime` key: `POST /api/browser/sessions`
   with body `{}` returns 404. Without a key both return 401 as before.
3. **Web Lens, in a browser against staging.** Sign in with the test account;
   open Web Lens; enter `https://example.com/`; assert the Open original link
   has `href="https://example.com/"`, `target="_blank"` and a `rel` that
   contains `noopener` and `noreferrer`; press Source Viewer snapshot; assert
   the Source window shows page text and the network log has one
   `POST /api/content/import-url` and no request under `/api/browser`. Enter
   `javascript:alert(1)` and assert an error and no link.
4. **Capabilities kept.** Research Read of an arbitrary public URL from the
   owner's computer; `POST /api/content/import-url` with `{"url":"https://example.com"}`
   returns 200 with text; read `GET /api/autopaper/sources/schedules` before
   and after the deploy and see the same schedules polling.
5. **Image.** The deploy's build output shows no obscura derivation; record the
   guest image size before and after.
6. **Egress (after a38eb7f67), on a test computer, from an implementation
   capsule.** Ecosystem builds: `pip download` of a small package, `npm view
   left-pad`, `git ls-remote https://github.com/golang/go HEAD`, one
   `huggingface_hub` file download; each succeeds and the egress ledger shows
   outcome `ok`. Mismatch: run

   ```python
   import socket, ssl
   s = socket.create_connection(("127.0.0.1", 3128))
   s.sendall(b"CONNECT github.com:443 HTTP/1.1\r\nHost: github.com:443\r\n\r\n")
   print(s.recv(200))
   try:
       ssl.create_default_context().wrap_socket(s, server_hostname="pypi.org")
       print("UNEXPECTED: handshake completed")
   except Exception as e:
       print("refused:", type(e).__name__)
   ```

   Expected: `200 Connection Established`, then `refused:`; the ledger shows
   outcome `refused` with a reason naming the SNI and the CONNECT host, and
   no bytes from the upstream. If an ecosystem build is refused, the ledger
   reason says whether it was a GREASE Encrypted Client Hello or no SNI;
   revert `a38eb7f67` and report the tool.
