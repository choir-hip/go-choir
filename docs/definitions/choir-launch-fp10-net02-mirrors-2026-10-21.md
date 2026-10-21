---
definition_version: 4

# Station FP10 of the launch first pass
# (choir-launch-first-pass-metamission-2026-10-21.md), this lane's slice:
# SDC NET-02, package mirrors beyond goproxy. Authority: the owner's
# statements that start the metamission, relayed by the orchestrator (FP0)
# as this lane's assignment.
readiness: drafted

review:
  reviewer: none yet; FP0 sends the red commits to a panel on the frozen diff
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-10-21T23:30:00Z
  source:
    canonical_ref: main 53a691be6
    deploy_identity: unknown to this lane; FP0 reads staging identity
  worktrees:
    - path: lanes/fp10-net02-mirrors
      status: clean
      class: goal_candidate
      owner: FP10
      touch: goal_owned
      recovery: branch lane/fp10-net02-mirrors

finish:
  deliver: >-
    Host read-through mirrors for npm, PyPI, crates and git that follow the
    goproxy pattern, each off by default; a per-computer switch that points
    a computer's L2 capsules at them; and a second switch that removes the
    mirrored registries from that computer's CONNECT allowlist, so FP0 can
    make capsule package egress mirrors only, one computer at a time.
  artifact: >-
    Commits on lane/fp10-net02-mirrors: this file and the problem record
    (green); internal/pkgmirror and cmd/pkgmirror, the host service (red);
    the guest switch, egress route, tap port and Nix unit (red).
  acceptance:
    - action: >-
        go test ./internal/pkgmirror ./internal/capsule ./internal/vmmanager
        -run (the new tests), in the lane worktree
      proves: the grammar, upstream, verification and switch contracts below hold against fakes
      evidence_class: local test
    - action: >-
        On staging, with one ecosystem turned on in the host unit and one test
        computer on the switch: from an implementation capsule, npm install of
        a small package, uv pip install of a small wheel, git clone of a small
        public repository; with mirrors only on, the same, plus a CONNECT to
        registry.npmjs.org that is refused; the pkgmirror log shows fetched
        then hit across two computers
      proves: the toolchains work through the mirror, the CONNECT path is closed for mirrored registries, and the cache is shared
      evidence_class: deployed proof
    - action: the paper's schedules and polls read before and after the deploy
      proves: the deploy did not take the paper down
      evidence_class: deployed proof
  rollback: >-
    Each switch off is the first rollback (empty the computer list, drop the
    ecosystem flags, deploy). Then git revert of the commits; the service
    holds no state another unit reads, and its cache directory may be
    deleted.
  landing:
    required: true
    environment: staging
    required_receipts: [pushed_commit, ci, deploy, environment_identity, deployed_acceptance]

value:
  better_means: >-
    Minimize the package and source traffic that leaves a capsule as an
    opaque tunnel, while every toolchain capsules use today keeps working.
  goodharting_would_be: >-
    A mirror that only passes bytes through (no grammar, no hash check); a
    switch that sets the client variables but leaves CONNECT open and calls
    it mirrors only; or a proof against fakes alone.

homotopy:
  realism_axis: >-
    Unit tests against fake upstreams, then a frozen candidate reviewed by a
    panel, then one test computer on staging with real registries, then the
    test computer under real engineering work.

boundaries:
  mutation_class: green (docs); red (a new host service, vmctl's tap rules and credential disk, the capsule egress route and environment)
  authority_sources:
    - the metamission's owner statements via FP0 (this lane's assignment)
    - SDC NET-02; grants design principle 2, §4.5; goproxy design (docs/reports/capsule-go-module-proxy-design-2026-10-14.md)
    - docs/choir-doctrine.md and CLAUDE.md
  must_preserve:
    - Capsule egress is unchanged on every computer the switch does not name.
    - Every mirror ecosystem is off unless its host flag is given.
    - go-choir-goproxy and its route are untouched.
    - No boot-path step scans or rewrites og_objects; nothing here touches the store.
    - No boot-time resumer; the mirror holds no obligations.
    - Problem documentation first; the failure list before the tests, the tests before the code.
  excluded:
    - Closing guest egress (NET-04), the host fetch service (NET-01), the grant object (NET-05).
    - Hosts with no mirror (conda, download.pytorch.org, Hugging Face, arXiv, Zenodo, doi.org, githubusercontent.com): they stay on CONNECT.
    - cargo in the guest image (a Nix change and build; none here).
  protected_surfaces: [host services on the tap, vmctl credential disk, capsule egress proxy and route table, capsule environment]

now:
  status: checkpoint_incomplete
  slice: >-
    Built and committed on lane/fp10-net02-mirrors, nothing deployed: the host
    mirror (npm, PyPI, crates, git; every ecosystem off), the per-computer
    grant through vmctl's credential disk, the egress route
    mirror.choir.internal on tap port 8089, client settings for granted
    ecosystems only, and mirrors only as its own switch. Every switch ships
    empty, so the deploy changes no computer's egress. A local smoke of the
    service against the real registries passed for all four ecosystems
    (npm and git with their real clients). The rest is FP0's: panel, land,
    deploy, flip one test computer, run the probe.
  source_ref: lane/fp10-net02-mirrors
  deploy_identity: unknown to this lane
  candidate:
    id: fp10-net02-mirrors
    state: ready
    ref: lane/fp10-net02-mirrors
    base: 53a691be6
    digest: none
    scope:
      - internal/pkgmirror/
      - cmd/pkgmirror/
      - internal/mirrorgrant/
      - internal/capsule/ (mirror.go, executor.go, egress.go, capsule.go)
      - internal/vmmanager/ (capsule_mirrors.go, manager.go, config.go)
      - internal/yaegikernel/broker.go
      - cmd/capsule-broker/ (jobs.go, main.go, session_worker.go)
      - internal/autoputer/capsule_executor_linux.go
      - flake.nix, nix/node-b.nix
      - scripts/capsule-mirror-probe.sh
  conjecture:
    id: C-mirrors-only-per-computer
    claim: >-
      Capsule package egress for npm, PyPI and git can be made mirrors only on
      one computer, with the toolchains still working, without changing any
      other computer.
    test: the probe below in route mode, then in only mode, on one test computer
    edge: missing_oracle
    delta_o: the deployed probe; a unit test cannot show the real toolchains through the real route
    scope_if_supported: the test computer, then class:test
    status: testing
    evidence_refs: [internal/pkgmirror/server_test.go, internal/pkgmirror/git_test.go, internal/capsule/mirror_test.go]
  decision:
    what: git over the dumb protocol (GET only), not smart HTTP (a POST to git-upload-pack on the route)
    kind: safety
    status: settled for this pass (the conservative default; see residual 1)
    evidence_ref: this file, "Residuals"
    owner_ratification_ref: not_applicable
  belief:
    believed_state: >-
      The service, grant, route and client settings work against fakes and,
      for the service, against the real registries; no computer is affected
      until FP0 names one.
    main_uncertainty: >-
      Whether uv and pip in a real capsule accept the http index with the
      settings given (neither client exists in this lane's environment), and
      whether the Nix edits evaluate (no Nix here).
    next_observation: the probe in route mode on one test computer
  blocker_or_risk: >-
    Nix not parsed or built in this lane. Shallow git clones fail through the
    git mirror, so git under mirrors only breaks `git clone --depth 1`.
  next_action: FP0 sends the red commits to a panel, then lands and proves them in the order under "Landing and proof"

receipts:
  - id: problem-record-and-goal
    boundary: define
    identity: 4f7eb10c8
    proof_refs: [docs/problems/capsule-package-egress-is-not-mirrors-only-2026-10-21.md]
    rollback_ref: git revert 4f7eb10c8
    disposition: committed
  - id: pkgmirror-npm-pypi-crates
    boundary: implement
    identity: 27acbe0cb
    proof_refs: [internal/pkgmirror/server_test.go]
    rollback_ref: git revert 27acbe0cb
    disposition: committed; 14 tests pass, 3 fail with the hash comparison disabled
  - id: pkgmirror-git
    boundary: implement
    identity: 6a8ec365c
    proof_refs: [internal/pkgmirror/git_test.go]
    rollback_ref: git revert 6a8ec365c
    disposition: committed; 7 tests pass under -race; the pack-survival check fails 3/3 with maintenance left on
  - id: guest-switch-route-wiring
    boundary: implement
    identity: 9d7f6f923
    proof_refs: [internal/capsule/mirror_test.go, internal/mirrorgrant/grant_test.go, internal/vmmanager/capsule_mirrors_test.go]
    rollback_ref: empty CHOIR_CAPSULE_MIRROR_COMPUTERS and deploy; then git revert 9d7f6f923
    disposition: committed; tests pass; Nix unbuilt
---

# FP10: package mirrors beyond goproxy (NET-02)

## Design

One host service, `go-choir-pkgmirror` (`cmd/pkgmirror`,
`internal/pkgmirror`), listens on `127.0.0.1:8089`; vmmanager DNATs tap port
8089 to it, as 8088 for goproxy. Capsules reach it through one egress route,
`mirror.choir.internal`, with a path prefix per ecosystem. Each ecosystem
is on only when its flag is given (`-npm`, `-pypi`, `-crates`, `-git`); an
ecosystem that is off answers 404 for its prefix.

| Prefix | Serves | Upstream (fixed) | Cached and verified |
| --- | --- | --- | --- |
| `/npm/` | packuments (passed through) and tarballs | `registry.npmjs.org` | tarballs, against the packument's `dist.integrity` (sha512) |
| `/pypi/` | PEP 691 simple index, rewritten to the mirror's file paths, as JSON or PEP 503 HTML; package files | `pypi.org`, `files.pythonhosted.org` | files, against the BLAKE2b-256 in their path and the index's sha256 |
| `/crates/` | sparse index (passed through), its own `config.json`, `.crate` downloads | `index.crates.io`, `static.crates.io` | crates, against the index entry's `cksum` (sha256) |
| `/git/` | git's dumb HTTP protocol from bare clones the service keeps | `github.com`, `gitlab.com`, `codeberg.org`, `owner/repo` only | the bare clone, fetched with `transfer.fsckObjects`; refreshed at most once a minute |

Everything is GET or HEAD; git is the dumb protocol so that no POST
(`git-upload-pack`) ever crosses the route, which keeps the egress route's
GET/HEAD rule intact.

Guest side, as visual capture does it: vmctl's
`CHOIR_CAPSULE_MIRROR_COMPUTERS` (ComputerIDs or `class:test`; empty by
default) writes a grant onto the computer's credential disk at cold boot
naming `CHOIR_CAPSULE_MIRROR_ECOSYSTEMS` and `CHOIR_CAPSULE_MIRRORS_ONLY`.
The guest executor reads it; for a granted computer it adds the route and
hands the broker the client settings for the granted ecosystems
(`npm_config_registry`, `UV_DEFAULT_INDEX`/`PIP_INDEX_URL`, a cargo source
replacement, git `insteadOf` through `GIT_CONFIG_*`). With mirrors only, the
mirrored registries' hosts leave that computer's CONNECT allowlist.

## Ways it can fail (written before the tests)

1. A path outside an ecosystem's grammar reaches an upstream (traversal,
   `%2e%2e`, a query, a second scheme, a host in the path).
2. A crafted name or version escapes the cache directory.
3. An upstream redirect leads off the fixed host table, or to plain http.
4. A tarball, file or crate whose bytes do not match the registry's hash
   enters the shared cache; or one whose hash is unknown does.
5. An upstream body larger than the cap fills the host disk or memory.
6. A cache entry is replaced or half-written (crash, two fetches at once).
7. A disabled ecosystem serves anything.
8. A method other than GET or HEAD is served.
9. The PyPI index rewrite points a client at the internet, drops the hash,
   or lets an upstream URL outside `files.pythonhosted.org/packages/` through.
10. A git repository name runs a git option (`-u...`), reaches another host,
    a local path or another protocol; git follows a redirect; a hook runs;
    a clone grows past its cap.
11. The dumb git paths serve a file outside the repository's objects, refs
    and info.
12. The guest switch turns on for a computer it does not name, or a
    snapshot resume changes it (it changes only at cold boot).
13. Mirrors only removes a host the mirrors do not replace, or leaves one
    they do.
14. A client variable for an ecosystem that is not granted reaches a job
    (it would point the tool at a route that refuses).
15. The capsule names the route address, or the route carries a CONNECT.

## Landing and proof (for FP0)

Land in this order; each red commit goes to the panel first.

1. `4f7eb10c8` (green) and the station's closing docs commit.
2. `27acbe0cb`, `6a8ec365c` (red): the service code. Inert alone: nothing
   builds or runs it.
3. `9d7f6f923` (red): the Nix unit, the flake package, the tap port, the
   grant and the capsule side. The deploy starts `go-choir-pkgmirror` with
   no ecosystem; the vmctl lists are empty. Check: CI builds
   `goServices` with `cmd/pkgmirror`; on Node B the unit is active and
   `curl -s 127.0.0.1:8089/healthz` shows `"ecosystems":[]`; the paper's
   schedules and polls read the same before and after.

Proof, one test computer (no secrets anywhere):

1. In `nix/node-b.nix` set the unit's `-ecosystems npm,pypi,git` and, in
   go-choir-vmctl's Environment, `CHOIR_CAPSULE_MIRROR_COMPUTERS=<test
   ComputerID>` and `CHOIR_CAPSULE_MIRROR_ECOSYSTEMS=npm,pypi,git`. Land,
   deploy, then cold-boot the test computer (stop and start; a snapshot
   resume keeps the old credential disk). vmctl logs `capsule mirrors for
   computer ...: granted (computer) git,npm,pypi`; the guest logs
   `capsule executor: package mirrors: granted (computer)`.
2. From an implementation capsule on that computer (a capsule job), run
   `bash scripts/capsule-mirror-probe.sh route npm,pypi,git`. Expect the
   summary line `"failures":0`. The capsule's egress ledger shows the
   mirror requests as host `mirror.choir.internal` with outcome ok, and
   `journalctl -u go-choir-pkgmirror` shows `fetched` records.
3. Set `CHOIR_CAPSULE_MIRRORS_ONLY=1`, land, deploy, cold-boot the computer,
   and run `bash scripts/capsule-mirror-probe.sh only npm,pypi,git`. Expect
   `"failures":0`; the egress ledger shows refused CONNECTs to
   registry.npmjs.org, pypi.org and github.com, and `huggingface.co` still
   ok.
4. Sharing: add a second test computer (or `class:test`), cold-boot it, run
   the probe there; the unit's log shows `hit` for the tarballs, files and
   repository the first computer fetched.
5. Rollback drill: empty the computer list, deploy, cold-boot; the probe's
   `npm-env-absent` and `pypi-env-absent` checks pass and the mirror name is
   refused.

## Residuals

1. **Shallow and partial git clones fail through the git mirror.** The dumb
   protocol has neither. With git in mirrors only, `git clone --depth 1`
   (common in agents) breaks; full clones work. The alternative is smart
   HTTP: the mirror runs `git upload-pack` on its own copy, and the egress
   route admits one POST path (`.../git-upload-pack`, body capped). The POST
   body would reach only the host's copy, never the forge. That widens the
   route's GET/HEAD rule, so it is a decision for FP0 or the owner, not
   taken here; until then, leave git out of mirrors only or accept full
   clones.
2. **cargo is not in the guest image**, so the crates mirror has no client.
   When cargo ships, its source replacement is a config file (CARGO_HOME),
   not an environment variable.
3. **yarn and pnpm** are not pointed at the mirror; under npm mirrors only
   their registry hosts are refused too (they would reach npm's content
   around the mirror otherwise).
4. **Metadata is not cached**: packuments and indexes pass through to the
   registry on every request; only artifacts are shared.
5. **Names still leave**: the package and repository names a capsule asks
   for reach the registry as the mirror's requests. Bounding names is the
   grant object's job (NET-05).
6. **Git packs accumulate**: the mirror never repacks (a repack would
   delete packs a reader is using); a busy repository gains a pack per
   refresh until the LRU cap or a re-clone. A re-clone-and-swap is the
   later fix.
7. **Host git on untrusted content**: bare copies, no checkout, fsck on,
   hooks and redirects off, a dynamic user; still a new parser on the host.
8. **Hosts with no mirror** stay on CONNECT: githubusercontent.com, conda,
   download.pytorch.org, Hugging Face, arXiv, Zenodo, doi.org.
9. A failed git refresh serves the last good copy and logs it as a hit.
10. **Not verified here**: Nix evaluation and build; uv and pip in a real
    capsule; the cell worker's passthrough on a real computer. Local test
    runs of `cmd/capsule-broker`, `internal/yaegikernel` and one
    `internal/vmmanager` test fail identically on the base commit (this
    sandbox's worker spawn and `/bin/cat`).
