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
    own commit and its own deploy); a guest network inventory report under docs/reports/
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
    - action: go test of internal/apihandler, internal/autoputer, internal/provideriface, internal/types, internal/store (focused), and go vet ./internal/...
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
  status: working
  slice: >-
    L2-06 steps 1 to 4 (problem record, launcher, deletion), then step 5,
    then the NET-01 inventory, then SEC-06
  source_ref: 26c4e1ed0
  deploy_identity: unknown to this lane
  candidate:
    id: none
    state: none
    ref: lane/b8-network
    base: 26c4e1ed0
    digest: none
    scope: []
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
    status: active
    evidence_refs: [docs/problems/obscura-untrusted-javascript-as-guest-root-2026-10-21.md]
  decision:
    what: delete Obscura and browsercontrol; Web Lens becomes a launcher (checklist default)
    kind: safety
    status: settled
    evidence_ref: launch checklist L2-06, owner default stands
    owner_ratification_ref: not_applicable
  belief:
    believed_state: >-
      The only caller of Obscura is the Web Lens frontend; the guest runs it
      as root with no address guard; nothing else needs it.
    main_uncertainty: >-
      Whether anything outside this tree (a stored prompt, an owner's saved
      window, a source entity) still sends the user to the backend mode; the
      launcher must degrade for those without a request to /api/browser.
    next_observation: the frontend test of the launcher with /api/browser requests failing the test
  blocker_or_risk: >-
    The frontend cannot be built in this worktree until node_modules are
    installed; the Playwright suite needs the local service stack, so the
    launcher is checked with a route-mocked spec and the deployed proof.
  next_action: write the launcher spec first, then the launcher

receipts: []
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
