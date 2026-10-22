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
    2026-10-22, later: the Obscura image removal (L2-06 step 5) is
    cherry-picked onto main 420aeae30 as d1af92301 on
    lane/fp6-obscura-removal (from c2da85986, which never landed); conflicts
    resolved against the zot removal; SBOM tests pass; pushed, not landed
    (see "Obscura image removal on main"). Earlier on 2026-10-22: SEC-06 replayed onto main and made observe-first on
    lane/fp6-sec06-on-main (e20c92b6c replay, d8d88ae5f tests, e0efea302 the
    off/observe/enforce mode, default observe; see "SEC-06 on main, observe
    first"); review round on 3b5c9e982 done (824a3f09e tests, e97b1b0db the
    per-computer enforce selector VM_CAPSULE_TUNNEL_ENFORCE_ONLY, empty, and
    the panel's edits; see "SEC-06 review round and per-computer enforce");
    pushed, not landed or deployed. Before that: all five asks built
    and committed on lane/b8-network; nothing is pushed or
    deployed. The orchestrator sends the red commits to a panel, lands them in
    order, and runs the deployed proof below. Batch-1 review edits (GPT-6.1-sol,
    accept with edits) are made: memory=detail now has one request-wide
    deadline and entry budget, batched directory reads, chunked and capped
    mincore probing, a one-at-a-time gate and reported truncation; a stored
    snapshot is shown and imported only while the address is the one it was
    taken from (import always uses that original URL). The frontend spec was
    written but not run: playwright/node_modules are absent in this lane.
  source_ref: e0efea302 (lane/fp6-sec06-on-main, base origin/main 966d3f519); a38eb7f67 on lane/b8-network
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
    The SEC-06 half is now measured before it can refuse: observe writes the
    would_refuse verdict on the egress ledger.
    next_observation: the deployed proof steps below, run by FP0 on staging;
      for SEC-06, the observe proof in "SEC-06 on main, observe first"
  blocker_or_risk: >-
    The image commit edits nix/autoputer-vm.nix, which the desk-boundary lane
    owns this pass (three Obscura-only removals); the frontend spec
    desktop-shell-core.spec.js needs the local service stack and was not run
    here. The stored browser-session accessors, types and six event kinds
    remain as frozen vocabulary.
  next_action: >-
    FP0 reviews and lands the commits in the order in "Landing order"
    (step 5 is d1af92301 on lane/fp6-obscura-removal, its own deploy, with
    the proof in "Obscura image removal on main"); for
    SEC-06, land lane/fp6-sec06-on-main (three code commits and the docs
    commit) in place of a38eb7f67 with one test computer named in
    VM_CAPSULE_TUNNEL_ENFORCE_ONLY, run the runbook in the problem record
    (observe on a second computer, enforce on the named one), and make
    enforce the image default only through the three-part gate there

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
    identity: d1af92301
    proof_refs: [.github/scripts/build-sboms-differential-test, .github/scripts/verify-sbom-candidate-test, .github/scripts/sbom-semantic-fingerprint-test]
    rollback_ref: git revert d1af92301
    disposition: >-
      pushed on lane/fp6-obscura-removal (cherry-pick of c2da85986 onto main
      420aeae30, which supersedes c2da85986 for landing); the three SBOM test
      scripts and the go tests that read the nix files pass locally; no nix
      on the lane VM, so the flake was not evaluated and the image not built
  - id: egress-tunnel-identity
    boundary: implement
    identity: a38eb7f67
    proof_refs: [internal/capsule/egress_tls_test.go]
    rollback_ref: git revert a38eb7f67
    disposition: committed; internal/capsule tests pass locally; six proxy tests fail with the check off; superseded for landing by the replay e20c92b6c
  - id: egress-tunnel-identity-observe
    boundary: implement
    identity: e0efea302
    proof_refs: [internal/capsule/egress_tls_mode_test.go, internal/capsule/egress_tls_mode_linux_test.go, scripts/capsule-tunnel-identity-probe.sh]
    rollback_ref: CHOIR_CAPSULE_EGRESS_TUNNEL_IDENTITY=off in nix/autoputer-vm.nix and a deploy; or git revert e0efea302 (enforce-always), then e20c92b6c (no check)
    disposition: pushed on lane/fp6-sec06-on-main (e20c92b6c, d8d88ae5f, e0efea302); go test ./internal/capsule and its -race run pass locally; not deployed
  - id: egress-tunnel-identity-enforce-selector
    boundary: implement
    identity: e97b1b0db
    proof_refs: [internal/vmmanager/capsule_tunnel_identity_test.go, internal/capsule/egress_tls_mode_test.go, internal/capsule/egress_tls_test.go, scripts/capsule-tunnel-identity-probe.sh]
    rollback_ref: empty VM_CAPSULE_TUNNEL_ENFORCE_ONLY in nix/node-b.nix and redeploy; or git revert e97b1b0db
    disposition: pushed on lane/fp6-sec06-on-main (824a3f09e tests, e97b1b0db code); go test of internal/capsule and internal/vmmanager and the -race run of internal/capsule, internal/autoputer and internal/vmmanager pass locally; the deployed runbook has not been run
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
capsule boundary; since 2026-10-22 landed as `lane/fp6-sec06-on-main`
instead, in observe mode: see "SEC-06 on main, observe first"). 0314e100b and the goal-file commit are docs.

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
5. **Image.** Land `d1af92301` (lane/fp6-obscura-removal) in place of
   `c2da85986`. The deploy's build output shows no obscura derivation; record the
   guest image size before and after. The exact steps are in "Obscura image
   removal on main" below.
6. **Egress (after a38eb7f67), on a test computer, from an implementation
   capsule.** Superseded on 2026-10-22: the check lands in observe, so the
   proof is the one in "SEC-06 on main, observe first" below; what follows is
   the enforce-flip proof. Ecosystem builds: `pip download` of a small package, `npm view
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

## SEC-06 on main, observe first (2026-10-22)

**Replayed onto main.** `a38eb7f67` (bind a CONNECT tunnel to its name) never
landed. It is replayed onto `origin/main` `966d3f519` with `git am -3` as
`e20c92b6c`; it applied cleanly over FP2's second batch (`capsule.go`,
`executor.go`, `types.go`), message kept, and `go test ./internal/capsule`
passes on it.

**Observe first.** The check refuses traffic and a wrong refusal breaks
builds (a GREASE Encrypted Client Hello, a client with no SNI), so under the
metamission rule it now has a mode: `off`, `observe` (default) or `enforce`,
set by `CHOIR_CAPSULE_EGRESS_TUNNEL_IDENTITY` in the guest daemon's
environment (`nix/autoputer-vm.nix`, beside `CHOIR_CAPSULE_MEMORY_POLICY`),
read at start in `internal/autoputer/capsule_executor_linux.go`, and written
by the executor on every egress policy it builds. Under observe each CONNECT
ledger entry carries `tunnel_identity` (`pass` or `would_refuse`) and
`tunnel_identity_reason`, a would-refuse is logged once per capsule and host,
and the tunnel is relayed unchanged. Enforce refuses as `a38eb7f67` did.

Commits on `lane/fp6-sec06-on-main`: `e20c92b6c` replay (red), `d8d88ae5f`
tests first (yellow), `e0efea302` the mode (red; conjecture delta, protected
surfaces, admissible evidence, rollback and heresy delta in its message),
then the docs commit carrying this section, the problem record's "Observe
first" section and `scripts/capsule-tunnel-identity-probe.sh`.

**FP0's deployed proof** has not been run. Its runbook, with the expected
results, is in [the problem record](../problems/egress-proxy-tunnel-identity-2026-10-21.md),
"Review round and per-computer enforce (2026-10-22)". Nothing in this file
records a result from it.

Residuals: `scripts/check-guest-data-fstrim.sh` could not run in this lane
(no `nix` binary on the VM), so the one-line `nix/autoputer-vm.nix` addition
is not evaluated here; the mode defaults to observe in code if the line is
absent. Exact host matching and the per-ecosystem fetch service stay as named
in the problem record.

## SEC-06 review round and per-computer enforce (2026-10-22)

The panel on `3b5c9e982` (Flash accept; DeepSeek and Sonnet accept with
edits). Under the owner rule of 2026-10-22, observe ships everywhere and
enforce runs on one selected test computer in the same deploy. Commits on
`lane/fp6-sec06-on-main`: `824a3f09e` (tests, yellow) and `e97b1b0db` (code,
red, ceremony in its message), then the docs commit with this section.

- **Selector:** vmctl's `VM_CAPSULE_TUNNEL_ENFORCE_ONLY`
  (`internal/vmmanager/capsule_tunnel_identity.go`). Name computer ids only:
  a `class:` token misses the platform computer's boot and the resume
  config. It writes the boot argument
  `choir.capsule_egress_tunnel_identity=enforce`. The guest boot script
  copies only that exact value into the daemon's environment file, which
  overrides the image's `observe`. `nix/node-b.nix` sets the list empty; FP0
  names one test computer at landing.
- **Edits:** ASCII-only name comparison (Sonnet F6). A boot WARNING for an
  unknown mode (F7). The full-record reader only for a checked CONNECT (F8).
  Comments and test header state the observe delay, which a test pins (F1),
  and the "complete first record within ten seconds" wording (DeepSeek F6).
- **Tests:** an upstream byte counter on every enforce refusal (M32). 9 KiB
  and 16384-byte hellos in both modes (M30). Duplicate extension, bytes
  after the extension block, bytes after the SNI list (M13, M20, M28). Pins
  for the image line and the boot-script case (DeepSeek F4). The selector on
  both boot paths, byte-identical boot args for computers not named, and
  node-b never `*`.
- **Probe:** `scripts/capsule-tunnel-identity-probe.sh observe|enforce`.
  - git drops the mirror's insteadOf rewrite (DeepSeek F2).
  - Precondition: a computer with no mirror grant, or one whose grant is not
    `mirrors_only` (DeepSeek F3).
  - The mismatch check needs the CONNECT answered 200 before it judges.
- **Global enforce gate** (Sonnet F2), in the problem record. Enforce becomes
  the image default only after three things hold: zero `would_refuse` for real
  toolchains on the observe ledgers; the selected computer passing under
  enforce; and a real claude-mode session showing `tunnel_identity` `pass`
  for `api.anthropic.com` on its own ledger.
- **Scope** (Sonnet F10): enforce closes the plain variant only (a hello
  naming another host). A Host header inside TLS is not seen. NET-02 is the
  durable fix.

Residuals: no `nix` binary on this VM, so neither nix file was evaluated.
The guest boot-script case arm was run in bash against three command lines.
The mode defaults to observe in code if the image line is absent.

## Obscura image removal on main (2026-10-22)

`c2da85986` (the image removal) was written on `26c4e1ed0` and never landed.
It is cherry-picked onto `origin/main` `420aeae30` as `d1af92301` on
`lane/fp6-obscura-removal`. The commit message carries the red ceremony.

**Conflicts.** All six files conflicted, and all for one reason: the source
predates the zot removal (`f2f66f9c3`), so each of its hunks kept zot. zot
stays gone. The flake and nix hunks are deletions on both sides. The vendorHash
changes since (`840e1a701`, `7c9e92bb5`) are on the shared Go module build.
Obscura is a Rust package with its own cargoHash, so no hash changes here. With
zot and obscura gone, the nine SBOM packages are all first-party, required and
semantic:

- the source's `SBOM_REQUIRED_PACKAGES` test seam stays; its default is the
  production set, and the verifier requires `.required == true` for every
  package;
- the test's optional package is `vmctl`, where the source used zot;
- the verifier requires a 64-hex semantic fingerprint for every package, and
  a new verifier case refuses an `exact:` one;
- `sbom-semantic-fingerprint-test` uses `not-a-listed-package` as its
  unlisted example, where it used obscura.

**References left.** No code path, Nix file, script, workflow or SBOM list
names Obscura. What remains is comments that cite the problem record
(`frontend/src/lib/BrowserApp.svelte`, `frontend/tests/browser-app.spec.js`,
`internal/apihandler/routes_test.go`), dated docs and evidence, and one
default label. `internal/store/browser.go` sets `Provider = "obscura"` in
`CreateBrowserSession`, which has no caller. The four browser-session Store
methods have no caller at all; they and their `*BrowserSessionOG` storage are
a residual for a store-owning station (the stored kind stays registered,
per must_preserve).

**Local proof.**

- The three SBOM test scripts print "all tests passed".
- `deploy-impact-classify-test` is ok.
- The go tests that read `flake.nix` or `nix/autoputer-vm.nix` pass: builder,
  capfloor, capsule, deskmanifest, doctool, hostbuild and runtimeprompts.
- `scripts/check-heresies.sh` exits 0.

There is no `nix` binary on the lane VM, so the flake was not evaluated and
the image was not built.

**FP0 proof steps (no secrets).**

1. **The guest image builds.** At `d1af92301` or its landed SHA, run
   `nix flake check --no-build` (or `nix eval --raw
   .#nixosConfigurations.go-choir-autoputer-vm.config.system.build.toplevel.drvPath`),
   then `guest=$(nix build --no-link --print-out-paths .#guest-image)`. The
   deploy's CI build is the same step. Record `du -sh` of
   `$guest/storedisk.erofs` against the previous deploy's. The checklist's
   research estimated about 508 MiB less; not measured here.
2. **No obscura path in the guest closure.** Each of these prints 0:
   - `nix-store -qR $(nix build --no-link --print-out-paths
     .#nixosConfigurations.go-choir-autoputer-vm.config.system.build.toplevel)
     | grep -ci -e obscura -e rusty_v8`
   - `grep -ci obscura $guest/guest-image-manifest`

   `nix eval .#packages.x86_64-linux.obscura` and
   `nix eval .#sbom.x86_64-linux.obscura` both fail with a missing attribute.
3. **A test computer boots it.** After the deploy, refresh one test computer
   onto the new image. Its health reports the landed commit (`build.json`),
   and an owner session can open the desktop and run one research Read. From an
   implementation capsule on that computer, `command -v obscura` finds nothing
   and `env | grep -ci obscura` prints 0. The SBOM job's `manifest.jsonl` for
   that run lists exactly the nine packages, all `required: true`.

Rollback: `git revert d1af92301` and a deploy; it is independent of the code
deletion.
