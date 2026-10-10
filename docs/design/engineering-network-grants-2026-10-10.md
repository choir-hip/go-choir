# Engineering network grants — design

Date: 2026-10-10
Status: design proposal (not settled). Source: owner request 2026-10-10;
synthesis by a research agent. Under standing question 1 this is a
**proposal** until the owner ratifies it.
Mutation class of this document: `green`. Every implementation phase below is
`red` (capsule isolation, egress, new authority intents).
Invariant served: **O17 Capsule confinement**: "Capsule egress is
policy-mediated and recorded; a capsule's authority … is broker-owned and
bound to its activation, never widened by model-authored code"
(`docs/operational-invariants-register-2026-10-08.md:241`).

---

## 1. Problem and owner intent

The demo on 2026-10-10 asked Choir to "replicate a recent arXiv paper with
little compute." Research found good candidates. Engineering could not go
further, because its capsule has no network. It cannot clone the paper's
repo, `pip install` its requirements, `npm install`, or download a dataset.
Today the only way to get bytes into a capsule is for research to fetch text
and send it in a message. That path is bounded, but nothing governs it
(§2.6).

The owner wants the following:

- Engineering gets network access for **package registries (PyPI, npm, Go),
  Git, and niche packages**.
- **Management gates it with grants.** Engineering can ask for a grant at
  any time. Management either grants it or **delegates to research** to
  check whether it is warranted.
- Research also works out the **categories** of grant.
- `dangerous code.exe` must never be approved. `beautiful code.exe` or
  `beautiful code.md` still needs research before engineering acts on it,
  though "in some situations they could decide correctly." In short: **a
  name never decides.**
- The system should be robust, secure and intelligent, and should reuse
  existing infrastructure where it can.

Standing rule: agents cannot widen their own permissions. A grant is
therefore something a **trusted reducer mints after a management (or owner)
decision**. Engineering cannot assert it.

---

## 2. What exists today

### 2.1 Capsule isolation: what makes it "networkless"

| Mechanism | Where | What it does |
| --- | --- | --- |
| Network namespace | `internal/capsule/executor.go:416` (`Unshareflags: … CLONE_NEWNET …`), comment at `internal/capsule/namespace.go:26-35` | The broker and workload run in a fresh netns with no interfaces. This is the primary boundary. |
| seccomp socket filter | `internal/capsule/seccomp.go:14-56` | Default-deny allowlist. `socket()` is allowed only for `AF_UNIX` (`seccomp.go:53`). This is defense in depth: even with an interface, no `AF_INET` socket can be created. |
| Landlock | `internal/capsule/landlock.go:46-111` | Path confinement for broker and workload. |
| cgroup v2 | `internal/capsule/namespace.go:69-110` | Memory/CPU/pids budget; freeze, kill. |
| Overlay root | `executor.go:322-340` | The lower layer is the pinned source snapshot plus `CHOIR_CAPSULE_LOWER_ROOT=/` (`nix/autoputer-vm.nix:856`). The capsule therefore sees the guest image read-only, including `git`, `go`, `nodejs`, and `python3` with only document packages (`nix/autoputer-vm.nix:31-37, 806-827`). The toolchains are present; only the bytes are missing. |
| Read-only bind of trusted inputs | `executor.go:323-364` (`VerifierBundleDir` copied into the lower layer, then `MS_BIND` + `MS_RDONLY` remount) | **This is the precedent for mounting fetched dependencies.** A trusted runtime input is placed in the capsule as a read-only mount that the overlay upper layer cannot shadow. |
| Binding record | `internal/types/engineering_assignment.go:14-15, 89, 163-165` | `NetworkMode` must be `forbidden` or `none`. Runtime sets `forbidden` (`internal/agentcore/engineering_assignment_runtime.go:323, 465`). |
| Capability token | `internal/capsule/capability.go:14-28` | Ed25519-signed and has an **`ExternalAccess []string`** field that is currently unused. The runtime **requires it to be empty** for engineering (`engineering_assignment_runtime.go:718`). |
| Grant policy attestation | `engineering_assignment.go:185-214`; digest at `engineering_assignment_runtime.go:729` → `store.ComputeEngineeringGrantPolicyDigest(role, verbs, networkMode, filesystemMode, writable)` (`internal/store/engineering_assignments.go:377`) | The network mode is **already bound into the policy digest**. That digest flows into `CapsuleEffectBundle.CapabilityPolicyDigest` (`internal/capsule/transaction/builder.go:32`, set at `internal/agentcore/tools_capsule.go:330-347`). Widening network policy is therefore already a visible, digest-changing fact on the frozen candidate. |
| Role verb sets | `internal/capsule/roles.go:16-34` | Engineering: exec/file verbs in one capsule. Research: read-only. Management: none. Authorization consults `RoleVerbSets`, never the token payload. |

**Conclusion.** The capsule boundary is strong and layered: netns, then
seccomp, then Landlock. The cleanest way to add dependencies is to **leave
it unchanged** and bring bytes in through a trusted read-only mount (§4.1).

### 2.2 Host-mediated egress (research's path)

- Desk cells call `choir.FetchURL` and similar verbs. These cross the session
  socket as `StreamBrokerEgress` frames (`internal/yaegikernel/transport.go:43-49`,
  `sidecar.go:322-339`) and are served in `internal/agentcore/tools_desk.go:143-162`.
  `product_api` goes to management. Everything else goes to
  `rt.researchDeps.HostEgress` (`internal/researchtools/host_egress.go:22-79`).
  The cell never holds a socket.
- Only the **research** scope is issued handles for egress actions
  (`internal/yaegikernel/choir.go:79-92`). Desk module sets are at
  `choir.go:179-206`. Engineering has `WriteFile/Exec/Ask/Escalate/Cast`
  but no egress verbs.
- Metering: `EgressBudgetLedger` allows 64 calls and 32 MiB per activation
  (`internal/researchtools/egress_budget.go:12-47`).
- `fetch_url` is a GET with a 256 KiB body cap (`internal/researchtools/researchtools.go:782-842`).
  It uses the `httpClient` built at `internal/agentcore/tool_profiles.go:458`:
  a **plain `&http.Client{}` with no SSRF guard** (see §2.6, D1).
- A guarded client exists: `sourcefetch.Client` (`internal/sourcefetch/policy.go:21-48`)
  blocks loopback, private, link-local and CGNAT addresses and re-validates
  redirects. `import_url`/content paths use it (`internal/content/content.go:343-357`).
  It resolves a name, checks it, then **dials by hostname**
  (`policy.go:26-35`). That is a resolve-then-dial TOCTOU, so DNS
  rebinding can bypass it.

### 2.3 Grant, authority and decision infrastructure that can carry a grant

| Asset | Where | Reuse for egress grants |
| --- | --- | --- |
| Signed capsule capability with `ExternalAccess` | `capsule/capability.go:14-28` | Carry `egress-grant:sha256:<digest>` refs. Today it must be empty (`engineering_assignment_runtime.go:718`); relax that to "must equal the grant set bound in the assignment". |
| HMAC session handles scoped to broker actions | `internal/yaegikernel/handles.go:26-60`, issued in `NewChoirScope` (`choir.go:69-100`) | Add a `request_grant` action to the engineering scope and `decide_grant` to management. The handle scope is the authorization, not the desk name. |
| Commitment ledger acts | `internal/types/commitment.go` (precommit/report/resolve/disagreement/directive; `CommitmentAction.DeclaredCapabilities` at `:171-174`); cell verbs `Ask/Escalate/Precommit/Resolve` at `choir.go:717-830` | Make the request, the vetting and the decision **supervision-visible**: engineering's request is a precommit with declared capabilities, research's vetting is a report, management's decision is a resolve. Texture renders them. The ledger is the supervision surface, not the authority store. |
| Decision policies with seats and quorum | `internal/decisionpolicy/types.go:3-57` and three JSON policies (`reversible-selfdev-v1`, `irreversible-email-v1`, `human-required-v1`) | Model risk tiers as policies: seats (management, research, owner), `revocation_check_point: immediately_before_dispatch`, `forbidden_capabilities`, budget, expiry. `human-required-v1` is already the shape for tier 3. |
| Typed judgment via Jev (OpenRouter Decisions API) | `internal/agentcore/selfdev_candidate_judgment.go:28-133` (closed choice questions; answers in **integer basis points** because canonical JSON refuses floats, `:80-91`; fails closed on any missing answer, `:96-132`) | Research's vetting verdict uses the same form (§4.4). |
| Freeze bindings | `CapsuleEffectBundle.DependencyToolchainRefs` (`transaction/builder.go:40`), validated as execution refs only (`capsule-exec:` / `capsule-go-eval:`; `builder.go:119-142`); staged by `choir.Freeze(buildRecipeRef, testReceipts, dependencyToolchainRefs)` (`choir.go:528`) | Add a `dep-lock:sha256:` ref kind so a frozen candidate names the exact fetched bytes. The verifier capsule then remounts them offline. |
| Declarative network intent | `CoagentPacketActionSafety.Network ∈ {forbidden, allowed, required}` (`internal/types/evidence.go:140-144`; validated `internal/coagentpacket/packet.go:174, 273`) | Fine as engineering's **declaration** of need. It is unenforced and self-asserted, so it is never authority. |

**Missing:** a grant object and its reducer; a broker that fetches and
mirrors dependencies; a mount for fetched dependencies; risk signals; a
research verdict schema; revocation checks at fetch time; and a
`dep-lock` ref kind.

### 2.4 Package installs today

None from the network. `GOMODCACHE`/`GOPATH` live on the guest data volume
(`nix/autoputer-vm.nix:861-865`, `GOTOOLCHAIN=local`). The capsule can use
whatever is already in the image. `pip`/`npm` installs fail for lack of a
network. The computer ontology already treats this as source/build state:
"offline recipe/toolchain/dependencies" (`docs/computer-ontology.md:178`).
Fetched dependencies should land in the content-addressed blob store with
pin receipts (`:179`).

### 2.5 The guest VM itself

The capsule's netns is inside a Firecracker guest. The **guest** has open
internet egress: per-tap `FORWARD ACCEPT` plus `MASQUERADE`
(`internal/vmmanager/manager.go:3230-3249`). Since S1a only `10.200.0.0/16`
and spoofed sources are dropped (`manager.go` `ensureTapIsolationRules`).
Guest-to-host service ports 8082/8083/8084/8086/8087/8787 are explicitly
accepted (`manager.go:3366`). Earlier records:
`docs/problems/s0-tap-egress-unfiltered-2026-10-01.md` (it names "proxy-only
egress via a recorded capsule proxy" as the intended S1 direction) and
`docs/problems/s0-guest-reaches-host-internal-authority-2026-10-04.md`.

### 2.6 Problems found while researching (documentation-first)

These need their own `docs/problems/` records before any fix lands, per
CLAUDE.md "Problem Documentation First". This design doc does not create them.

- **D1 — `fetch_url` has no SSRF guard** (source-traced, not
  staging-confirmed). `tool_profiles.go:458` passes a plain `http.Client` to
  `newFetchURLTool`. A prompt-injected research cell could GET guest
  loopback services, or the host peer `10.200.X.1:{8082,8083,8084,8086,8087,8787}`,
  which are accepted on INPUT. The request would carry the guest's source IP
  identity. Per O15, transport-bound authority is still only partial. A
  replacement already exists (`sourcefetch.Client`), so **wire it in; do not
  patch around it** (CLAUDE.md "Check for an existing replacement").
- **D2 — `sourcefetch` resolve-then-dial TOCTOU** (`policy.go:26-35`). Fix it
  by dialing the validated IP, as Smokescreen and Claude Code's sandbox proxy
  do: resolve once, check the address, connect to that address.
- **D3 — an ungoverned content path into the capsule.** Engineering can
  `choir.Ask("research", …)`. Research can `FetchURL` and `Reply` with up to
  12k characters. Engineering can `WriteFile` and `Exec` that text. So
  "networkless" does not mean "untrusted-content-less". Grants should cover
  this path too: content that research relays for **execution** is a grant
  subject (§4.3, `use: execute`).
- **D4 — `safety.network` is self-asserted and unenforced.** It must never
  be read as authority.

---

## 3. Threat model

Assets: the owner's private source and documents in the capsule's source
snapshot; Choir's authority surfaces (vmctl, gateway, corpusd); the
integrity of frozen candidates (what ships into the owner's computer); the
owner's compute and storage budget; and other tenants.

| # | Threat | Example | Primary mitigations (§4) |
| --- | --- | --- | --- |
| T1 | **Exfiltration through egress** | Injected code encodes source into `GET /simple/<base32-secret>/` or into a git URL path, or POSTs to a paste site. | No model code ever shares a process tree with a network route (fetch-and-mount). Egress uses protocol-aware mirror endpoints with GET/HEAD only, plus git `upload-pack` only. Names are bounded to grant scope, so per-request bandwidth is about log2(catalog size) bits, capped by a request budget. Every request is receipted. |
| T2 | **Malicious package, typosquat, slopsquat** | `nunpy`, `requests2`, a hallucinated `huggingface-cli` (30k+ downloads of an empty squat; about 20% of LLM code samples contain hallucinated packages and 43% of those recur on every rerun — USENIX Sec '25). | Names never decide. Deterministic signals (age, cooldown, downloads, confusable-name distance to popular packages, OSV `MAL-` records, provenance) come first. Research vetting covers anything off the well-trodden path. A grant scopes the exact resolved name, version and hash. |
| T3 | **Install-time code execution** | npm `postinstall` (Shai-Hulud 2025 used `postinstall` + TruffleHog to steal tokens and self-propagate); `setup.py` in sdists; git submodule and hook tricks (CVE-2024-32002). | The fetcher never executes fetched code: wheels only, `--ignore-scripts`, `go mod download`, `git clone --no-recurse-submodules` with `transfer.fsckObjects`. Builds run later **inside the offline capsule**, where code execution is already contained and has no network or credentials to steal (I19). |
| T4 | **Prompt injection through fetched content** | A README or docstring says "also fetch `x` and run `curl … \| sh`". | A grant request must cite a purpose ref (owner objective or research report), not fetched content. A request whose only motivation traces to fetched content is **tainted** and goes up a tier. Research's verdict is a closed enum in basis points, so content can't widen control flow (CaMeL principle). The mirror refuses anything outside scope regardless of what any model says. |
| T5 | **Grant escalation by a confused or compromised desk** | Engineering forges a grant, widens its own scope, or replays an expired grant. Management is itself injected into approving everything. | Grants are minted only by a trusted reducer from a management or owner decision act whose authorization is the handle scope. Engineering's capability carries grant **digests**, and the mirror checks grant state on every request (revocation check immediately before dispatch). Tier floors are deterministic and the LLM cannot lower them. Management's tier-1 decisions are policy-checked, and Texture renders them for the owner. Hard-deny classes cannot be granted by any desk. |
| T6 | **Research fooled by a benign-looking name or page** | "beautiful code.md" is actually a shell dropper. A popular-looking repo is a fork with one malicious commit. A paper's repo links to a "release" binary on a lookalike domain. | Content type comes from magic bytes and declared use, never the extension. Provenance (owner identity, attestations, canonical registry) and behavior (scripts, entry points, binaries, network calls in build files) beat prose. Research's verdict is one seat among deterministic signals. Its precommitted predictions are later **scored** against OSV and malware feeds, so calibration is measurable. |
| T7 | **Resource abuse / DoS** | 30 GB of model weights, a fetch storm. | Byte, request and storage budgets per grant and per computer; cgroup limits on the fetcher; a CAS de-duplicates. |
| T8 | **SSRF / lateral movement from the egress path** | The mirror is tricked into fetching `http://10.200.X.1:8083/…` or `169.254.169.254`. | Fixed upstream host table per ecosystem. The mirror resolves DNS itself, refuses private, loopback, link-local and CGNAT addresses, dials the pinned IP, and allows no redirects off the allowlist (D1/D2 fixes are prerequisites). |
| T9 | **Unsafe data formats** | Pickled model weights (`.bin`, `.pt`, `.pkl`) execute code on `torch.load`. | Content class `model_weights_pickle` is tier 3 by default. `safetensors` and plain-data formats (CSV, Parquet, JSON, NPZ with `allow_pickle=False`) are tier 1–2. |
| T10 | **Reproducibility drift** | The candidate built against `numpy==2.1.0` today, the verifier pulls `2.1.1` tomorrow. | Lock manifests are content addressed (`dep-lock:sha256:`). The verifier mounts the identical CAS objects with no network. |

---

## 4. Proposed design

### 4.0 Principles

1. **Fetch-and-mount, not open network.** Model-authored code and a network
   route never coexist in one process tree. Trusted fetch tools get the
   network and produce content-addressed bytes. The offline capsule
   consumes them through a read-only mount. This mirrors Nix fixed-output
   derivations (network only when the output hash is pinned), Codex cloud
   (online setup phase, offline agent phase), and Bazel's downloader
   (sha256 required, URL policy rewrites to mirrors).
2. **Protocol-aware mirror over a generic CONNECT proxy.** A CONNECT proxy
   sees only a hostname. Claude Code's own docs warn that allowing
   `github.com` enables exfiltration and domain fronting unless TLS is
   terminated. A registry-aware read-through mirror (the
   devpi/Verdaccio/Athens/GOPROXY pattern) speaks plain HTTP to the
   sandbox and TLS upstream. It enforces method + path + name scope without
   installing a CA in the sandbox.
3. **Names never decide.** Tier comes from source identity, provenance,
   content class (magic bytes), intended use, behavior and reputation. A
   filename is display text.
4. **Deterministic floors; LLM judgment only above them.** Hard denies and
   tier floors are computed by trusted code. Research and management can
   approve **within** a tier, and the owner above it. No model can lower a
   floor.
5. **Grants are authority; the ledger is supervision.** The grant record is
   reducer-owned. Ledger acts make each request and decision visible and
   scoreable.
6. **Request digests, never trust the guest.** The mirror trusts only grant
   state that it reads from the trusted store, keyed to the fetch sandbox
   identity. The capsule's or fetcher's own claims are never trusted.

### 4.1 Architecture

```
 engineering cell ──choir.RequestGrant(spec)──► reducer ──► EgressGrant{requested}
        ▲                                         │            │
        │                                         ▼            ▼ (ledger act: precommit
        │                            management cell            w/ declared caps; Texture
        │                 choir.DecideGrant(ref, grant|deny|delegate|escalate)  renders)
        │                                         │
        │                    delegate ──► research cell: signals + inspect ──► Report
        │                                         │        (typed verdict, bp)
        │                    escalate ──► owner (human seat; human-required policy)
        │                                         ▼
        │                              EgressGrant{granted, scope, budget, expiry}
        │                                         │
        │       choir.Fetch(grantRef, items)  ┌───▼──────────────── guest core (trusted) ─┐
        │  ───────────────────────────────►   │ fetch sandbox (no model code):            │
        │                                     │   uv / npm --ignore-scripts / go mod      │
        │                                     │   download / git clone --mirror           │
        │                                     │      │ only route: unix sock → mirror     │
        │                                     │   egress mirror: per-ecosystem endpoints, │
        │                                     │   grant check per request, GET/HEAD,      │
        │                                     │   pinned-IP dial, receipts, budgets       │
        │                                     │      │                                    │
        │                                     │   CAS /mnt/persistent/deps/sha256/…       │
        │                                     │   dep-lock manifest (pin receipt)         │
        │                                     └───┬───────────────────────────────────────┘
        │                                         │ read-only bind (like VerifierBundleDir)
        └──── offline capsule: /deps (ro) + PIP_NO_INDEX=1 PIP_FIND_LINKS=/deps/pypi,
              npm ci --offline --cache /deps/npm, GOPROXY=file:///deps/goproxy,
              git clone /deps/git/<id>.bundle
```

- **Phase-1 location:** the mirror and fetch sandbox run in **guest core**,
  outside every capsule. From the capsule's point of view, guest core is the
  trusted host.
- **Phase-3 location:** the mirror moves to **Node B**, shared across
  computers, with a content-addressed cache. Guest tap egress is then
  default-denied except gateway, source service and mirror. That also closes
  the remaining O17/D2 gap, so a compromised guest core can no longer reach
  the internet directly. This is "host-side, never trusting the guest" in
  the full sense.
- **Fetch sandbox:** the same namespace stack as a capsule, with one
  difference. Its only route out is a Unix socket to the mirror, bridged by a
  tiny in-sandbox relay (Claude Code's Linux sandbox uses socat the same
  way). It runs **pinned resolver tools with typed arguments**, never a
  model-authored shell string, and execution of fetched content is disabled
  (§3 T3). Its outputs are hashed into the CAS by guest core, not by the
  sandbox.
- **Mount:** `SpawnSpec` gains `DependencyLockRefs []string`. Spawn
  materializes the union of CAS objects into `sourceLower/deps` and binds
  `/deps` read-only, as `VerifierBundleDir` already is (`executor.go:323-364`).
  For a live assignment, a new fetch produces a new lock. The broker remounts
  `/deps` (bind over a fresh read-only tree) between cells, and the remount is
  receipted. Alternatively, Phase 1 can require the capsule to be respawned.

**Why not open egress inside the work capsule?** Because it would require:
`AF_INET` in the workload seccomp filter (`seccomp.go:53`), a veth plus
nftables in the capsule netns, and CA injection for path policy. It would
also give model code a live exfiltration channel. It is kept only as an
optional Phase 4, for workflows that truly cannot be resolved ahead of time.

### 4.2 The grant object

```go
// types.EgressGrant — reducer-owned; objectgraph kind choir.egress_grant;
// lifecycle is append-only like EngineeringCapsuleFateStep.
type EgressGrant struct {
    Schema          string        // "choir.egress_grant/v1"
    GrantID         string        // "egress-grant:sha256:<digest of canonical scope+binding>"
    ComputerID, OwnerID, TrajectoryID string
    AssignmentID    string        // bound engineering assignment (grant dies with it)
    RequesterAgent  string        // engineering:<assignment>[:attempt-N]
    RequestRef      string        // ledger act ref of the request (precommit)
    Purpose         Purpose       // {objective_ref, rationale, taint: none|fetched_content}
    Scope           []ScopeItem   // see below; exact or predicate
    Use             string        // data | build_input | execute
    Methods         []string      // default GET, HEAD (+ git upload-pack)
    Budget          Budget        // max_requests, max_bytes, max_objects, storage_bytes
    NotBefore, ExpiresAt time.Time // default: assignment deadline, hard cap 24h
    Tier            int           // 0..3 — floor computed by trusted code, never lowered
    PolicyID, PolicyDigest string // decisionpolicy seat/quorum contract used
    SignalsRef      string        // deterministic signal bundle (CAS)
    EvidenceRefs    []string      // research reports, OSV/deps.dev snapshots
    Decision        Decision      // {by: management|owner|policy, act_ref, at, seats[]}
    Disposition     string        // requested|vetting|escalated|granted|denied|
                                  // exhausted|expired|revoked|interrupted
    LockRefs        []string      // dep-lock:sha256:… produced under this grant
    Usage           Usage         // requests, bytes, last receipt ref
}

type ScopeItem struct {
    Ecosystem string // pypi | npm | gomod | git | url
    // exact form:
    Name, Version, Digest string // e.g. numpy, 2.1.3, sha256:… (digest optional at request,
                                 // required in the lock)
    Repo, Commit          string // git: https://github.com/org/repo + 40/64-hex commit
    URL                   string // url: exact https URL (+ expected sha256 if known)
    // predicate form ("category grant"), e.g. category "pypi.popular.wheels.cooldown7d":
    Category              string
    Constraints           map[string]string // only_binary=true, max_version_age… etc.
    ContentClass          string // source_archive | wheel | npm_tarball | go_module |
                                 // dataset_plain | model_safetensors | model_pickle |
                                 // native_executable | document
}
```

**Where it lives, and the single authority (standing question 5).** The
`EgressGrant` record in the objectgraph store, reducer-owned, is the **only
authority**. Ledger acts mirror it for supervision:

- request = `precommit`, with `CommitmentAction.DeclaredCapabilities = ["egress:pypi:numpy==2.1.*"]`
  and a prediction such as "wheels suffice; ≤ 50 MB; no install scripts";
- vetting = research `report` carrying the typed verdict;
- decision = management `resolve` / owner `resolve`;
- revocation = a `directive` with subtype `retract`.

If a ledger projection disagrees with the grant record, the grant record
wins. The capability token carries `ExternalAccess = [GrantID…]`. The policy
digest folds in the grant-set digest, so
`CapsuleEffectBundle.CapabilityPolicyDigest` shows that a candidate was
built with granted bytes.

**Revocation and expiry.** Management and the owner can revoke at any time.
The mirror reads grant disposition **on every request** (the decision policy
`revocation_check_point: immediately_before_dispatch` pattern). Already-fetched
CAS objects stay, because they are immutable data. A revoked grant's locks
cannot be newly mounted unless a new grant covers them. Assignment terminal
state makes the grant `expired`. Budget exhaustion makes it `exhausted`.

**Restart (standing question 8; CLAUDE.md restart rule).** At a crash boot,
open grants and in-flight fetches are closed with fate `interrupted by a
restart` and are never re-dispatched. A planned-restart marker may resume
`granted` grants because their authority is durable. Partially fetched
objects are discarded unless their hash already verified into the CAS.

**Fate-sharing (standing question 7).** A mounted `/deps` depends only on
the local CAS, not on the mirror or on registry availability. Verification
and rebuilds therefore work offline, for as long as the CAS pin receipts
hold.

### 4.3 Risk tiers and the decision flow

The tier is computed in two steps. First, **trusted code** computes a
**floor** from the scope and deterministic signals. Then decision authority
is assigned:

| Tier | Who decides | Typical scope (all conditions) |
| --- | --- | --- |
| **0 — policy** | Reducer auto-grants once management casts work that names it (no extra turn) | Pre-approved **category** grants created by the owner or management from research's catalog. Example: `pypi.popular.wheels.cooldown7d` = name in a curated top-N set; version ≥ 7 days old; wheel-only; no OSV `MAL-`/critical advisory; whole transitive closure meets the same rule. Also: Go modules verified against `sum.golang.org`; exact-commit clones from repos already on the trust list. |
| **1 — management** | Management alone, with the signal bundle in view | Canonical registry, exact pin, package ≥ 90 days old and version ≥ 7 days, healthy download/dependent counts, no confusable-name hit, no install scripts / wheel available, provenance present or long track record, `use ∈ {data, build_input}`. Public git repo at an exact commit, from an owner account older than 1 year, linked from the paper or a canonical project page that research already cited. |
| **2 — research-vetted** | Management delegates to research; grants only on research `approve` with confidence ≥ threshold, otherwise escalates | Any soft-signal failure: new or low-reputation package; version younger than cooldown; confusable name; sdist-only or install scripts required (`npm` packages needing `postinstall`); native binaries inside wheels from low-reputation publishers; non-registry URL; datasets > 1 GB; `use: execute` of fetched scripts; **any request tainted by fetched content.** |
| **3 — owner** | Owner seat required (`human-required`-style policy); management and research recommend | `native_executable` from outside a canonical registry (release binaries, installers); `model_pickle` weights; credentials, private repos or authenticated endpoints; any method beyond read (POST/PUT outside git `upload-pack`); research `deny`, or `approve` with low confidence that management still wants; budget beyond computer defaults. |
| **Deny — no one** | Hard deny, no grant possible | PE/Mach-O executables (`MZ`/`\xCF\xFA…` magic) for a Linux capsule; `curl \| sh` installer flows; known-malicious (`MAL-`) versions; private, loopback or link-local destinations; non-HTTPS upstreams (except the local mirror); content whose hash contradicts the registry's published hash; packages yanked for security. |

**Decision flow.**

1. Engineering calls `choir.RequestGrant(spec)` (new intent; engineering
   handle gains `request_grant`). The spec is typed items plus `use` and
   `purpose_ref`. A free-form request becomes a `question` to management,
   never a grant.
2. The reducer validates the shape, computes the **signal bundle** (§4.4) by
   calling the mirror's metadata endpoints under a **metadata-only grant**
   that the reducer itself holds (GET JSON metadata, no artifact bytes),
   computes the tier floor, and writes `EgressGrant{requested}`.
   - Hard deny ends the flow here with a receipt and a ledger act that
     engineering sees, along with the reason.
3. Management wakes with the request and signal bundle and stages
   `choir.DecideGrant(ref, …)`:
   - `grant` (allowed only if tier ≤ 1, or tier 2 with a research
     `approve` attached);
   - `deny` (always allowed);
   - `delegate` (opens a research probe with the request);
   - `escalate` (owner).
   The reducer refuses any decision that exceeds management's authority
   for that tier.
4. When delegated, research runs vetting and returns the typed verdict
   (§4.4) as a `Report`. Management decides again with the verdict in view.
5. Once granted, engineering calls `choir.Fetch(grantRef, items)`. Guest
   core runs the fetch sandbox and returns `dep-lock:sha256:…` plus
   per-object receipts. `/deps` is (re)mounted read-only.
6. Engineering builds offline and cites the lock refs in `choir.Freeze(…)`.
   The verifier capsule mounts the same locks. Neither needs a network.

**Where engineering "decides correctly" on its own.** Engineering never
mints authority. Under a **category grant** (tier 0/1 predicate), though, it
can select any item that satisfies the predicate without another round
trip: "any popular wheel ≥ 7 days old, ≤ 2 GB total". That is
attenuation, not widening. The owner's "maybe in some situations they could
decide correctly" becomes the set of categories research has studied and
management or the owner has issued. That set grows as research's
precommitted judgments prove calibrated (§4.4, scoring).

**The "beautiful code" cases, concretely.**

- `dangerous code.exe` and `beautiful code.exe` get the same treatment. The
  mirror sniffs magic bytes. PE (`MZ`) means hard deny, whatever the name.
  An ELF from a GitHub release is `native_executable` (tier 3, owner) unless
  it is inside a canonical-registry wheel from a tier-1 project.
- `beautiful code.md` from an unknown gist: the content class comes from
  bytes. If it is really Markdown and `use: data`, it is tier 1, delivered as
  a **quarantined data file**. Engineering reads it as untrusted data (it
  may carry injection), and any request it triggers is tainted (tier ≥ 2).
  If the bytes are a shell script, or `use: execute`, it is tier 2: research
  must look at it first. "It's called .md" is never evidence.
- `beautiful-code` on PyPI, 3 days old, with a name one edit away from a
  popular package: cooldown fail plus confusable, so tier 2. Research checks
  maintainer identity, repo link, provenance, diff against the similarly
  named package and install hooks. A likely verdict is deny or "use
  `<real package>` instead."
- `numpy==2.1.3` wheels: popular, old, provenance and hashes known. Tier 0
  under the popular-wheels category, so no human in the loop.

### 4.4 What research checks, and the verdict format

**Deterministic signal bundle** (trusted code, no LLM, computed before any
judgment, stored in the CAS):

| Signal | Source |
| --- | --- |
| Existence, first-publish date, version publish date (cooldown), yanked status | Registry metadata: PyPI JSON/Simple API (PEP 691/700 `upload-time`), npm registry `time`, Go proxy `.info` |
| Downloads, dependents | deps.dev API; pypistats-style counts; npm downloads |
| Known malicious / vulnerable | OSV API (`MAL-` records from `ossf/malicious-packages`; CVEs), batch query over the whole resolved closure |
| Repo health | OpenSSF Scorecard (via deps.dev) |
| Provenance | PyPI PEP 740 attestations / Trusted Publishing; npm provenance (`npm audit signatures`); Go checksum DB; git commit exists on the canonical default branch or a tag; optional signed tags |
| Confusable name | Edit distance, keyboard and homoglyph distance, separator normalization to the top-N names in the ecosystem, plus known slopsquat lists |
| Install-time behavior | Presence of `setup.py`/`pyproject` build hooks for sdists, npm `preinstall/install/postinstall`, `binding.gyp`, entry points, executable bits |
| Content class | Magic-byte sniff of every fetched object; archive member listing; size |
| Closure facts | Transitive count, total bytes, licenses |

**Research vetting (tier 2).** Research uses its existing read-only verbs
(`SourceSearch`, `FetchURL` once D1 is fixed, `ImportURL`) to check the
claims behind the request: is this the repo the paper links? Is the PyPI
project's homepage the same org? Is there an independent mention? It may
read package files **as data** in the mirror's quarantine view, for example
`setup.py`, `package.json` scripts, or the diff against the previous
version. It then answers a fixed question set through the Jev decision API,
exactly like `candidateJudgmentQuestions`:

```json
{
  "is_canonical_source":          {"choices": ["yes","no","unclear"]},
  "name_confusable_with_other":   {"choices": ["no","yes"]},
  "publisher_identity_consistent":{"choices": ["yes","no","unclear"]},
  "install_time_execution":       {"choices": ["none","benign","suspicious"]},
  "content_matches_declared_use": {"choices": ["yes","no"]},
  "known_malicious_or_vulnerable":{"choices": ["no","yes"]},
  "needed_for_objective":         {"choices": ["yes","partly","no"]},
  "recommendation":               {"choices": ["approve","approve_with_constraints","deny","escalate"]}
}
```

The result is recorded with integer basis points, as in
`selfdev_candidate_judgment.go:80-91`:
`{choice, p_bp, confidence_bp, probabilities_bp}` per question, plus the
model ID, `signals_ref` and `evidence_refs`. Missing or malformed answers
fail closed to `escalate`. A research `approve` is valid only if every
safety question chose the safe answer with p ≥ 0.8 (8000 bp; to be tuned).
The verdict **cannot override a hard deny or lower the tier floor**.

**Scoring (making research better over time).** Research also stages a
`Precommit` for the grant:
"`P(this version receives a MAL-/critical advisory within 30 days)` = 0.02".
A scheduled resolver re-queries OSV after 30 days and resolves it. That gives
research's vetting a Brier-scoreable track record on the commitment ledger.
This is precommitment records applied to supply-chain judgment, and the
evidence that justifies moving categories down a tier.

**Categories (owner's "research could figure out the categories").** A
research mission produces a **category catalog**: ecosystem × content class
× reputation band × install behavior × use. Each entry carries a proposed
tier and its justification, with OSV/deps.dev/cooldown evidence. The owner
or management issues categories as tier-0/1 predicate policies. The catalog
is versioned and digested; grants cite the catalog digest they were decided
under.

### 4.5 The mirror's enforcement contract

For every request, the mirror:

1. Authenticates the caller by **transport identity**: the Unix socket is
   bound per fetch sandbox, and its peer credentials map to one
   `(assignment, grant set)`. It never uses headers or the source IP (O15).
2. Loads grant state from the trusted store and checks disposition, expiry
   and budget, immediately before dispatch.
3. Matches the request against an **ecosystem route table**:
   - PyPI `GET /pypi/simple/<name>/`, then `GET /pypi/files/<path>`;
     upstreams are only `pypi.org` and `files.pythonhosted.org`;
   - npm `GET /npm/<name>` and `/npm/<name>/-/<tarball>`;
   - Go `GET /goproxy/<module>/@v/…`, plus a checksum DB check;
   - git `GET /git/<host>/<owner>/<repo>/info/refs?service=git-upload-pack`
     and `POST …/git-upload-pack`. That is the only POST, the body is
     size-capped, and the repo is in scope;
   - url `GET` of one exact URL.

   Anything else returns 403, and the denial is receipted (GitHub Copilot's
   firewall similarly reports blocked hosts and the command back to the
   user).
4. Restricts names to scope: exact names for exact grants, membership in the
   catalog for category grants. This is what bounds exfiltration bandwidth.
5. Resolves upstream DNS itself, refuses private/loopback/link-local/CGNAT
   results, dials **the validated IP** with SNI for the canonical host, and
   allows redirects only to hosts in the same ecosystem table.
6. Streams the body into a CAS tempfile and verifies the registry's
   published hash (PyPI `sha256`, npm `integrity` SRI, Go `go.sum`/sumdb,
   git object ids) before admitting the object. A mismatch is a hard fail
   plus a problem record.
7. Writes a **fetch receipt**: method, route, upstream host and IP, status,
   bytes, content sha256, sniffed content class, grant ID, budget after the
   request, time. Receipts are host-derived (I23); the fetcher cannot omit
   them.

### 4.6 Freeze and reproducibility

- A `dep-lock` manifest is canonical JSON:
  `{schema, grant_ids, items:[{ecosystem, name, version|commit, url, sha256, content_class, size}], tool_versions, catalog_digest}`.
  Its ref is `dep-lock:sha256:<digest>`, and it is pinned in the blob store.
- `choir.Freeze(buildRecipeRef, testReceipts, dependencyToolchainRefs)`
  accepts `dep-lock:` refs alongside execution refs. That means extending
  `validExecutionRef` in `transaction/builder.go:131-142`, or, more cleanly,
  adding a sibling `DependencyLockRefs` field so execution refs keep their
  meaning.
- The verifier capsule spawns with the same `DependencyLockRefs`. A verifier
  that needs a byte not in the lock fails. It never fetches.
- `ComputeEngineeringGrantPolicyDigest` gains the sorted grant ID set as an
  input, so a candidate's `CapabilityPolicyDigest` changes whenever egress was
  granted.

---

## 5. Phased plan

Every phase runs the CLAUDE.md Landing Loop on staging (`choir.news`). Every
`red` phase carries full ceremony.

### Phase 0 — document, catalog, fix the guard (green → orange)

- **Green:** problem records for D1–D4 (§2.6); this design reviewed through
  `skills/agentic-consensus` at the definition gate; a throughline `/goal`
  file for Phase 1.
- **Green (research mission):** category catalog v0 for PyPI, npm, Go, git
  and URL/data classes, with evidence refs. Research can do this today with
  its existing verbs.
- **Orange (separate commit, after D1's record):** connect `sourcefetch.Client`
  to `fetch_url` (`tool_profiles.go:458`) and fix the D2 dial-by-IP. Rollback:
  revert the commit.
  Evidence: a staging probe where a research cell `FetchURL`s
  `http://10.200.X.1:8083/` and `http://127.0.0.1:…` is refused with a
  receipt, and a public URL still succeeds.

### Phase 1 — the smallest safe unblock: git + exact URLs + PyPI wheels, fetch-and-mount (red)

Scope: `git` at an exact commit, from public HTTPS repos; `url`, one exact
HTTPS URL with an optional expected sha256 (papers, small datasets); and
**PyPI wheels only** (`--only-binary=:all:`, resolved by a pinned `uv pip compile`
inside the fetch sandbox through the mirror). npm and Go come in Phase 2.

Work:

1. `types.EgressGrant` + store + reducer; intents `request_grant`
   (engineering), `decide_grant` (management), `fetch` (engineering);
   ledger-act mirroring; Texture rendering of request, decision and fetch
   receipts.
2. Tier computation, Phase-1 subset: hard-deny list; magic-byte sniff;
   cooldown, age and downloads from registry and deps.dev; OSV batch;
   a confusable-name check against a top-N list. Management decides tiers 0–1.
   Tier 2 goes to research through the existing `Ask` → `Report` path, with
   the verdict schema above. Tier 3 escalates to the owner.
3. Guest-core mirror (git, url and pypi routes) plus fetch sandbox (netns +
   Unix socket relay; trusted tools only) plus CAS + `dep-lock`.
4. `SpawnSpec.DependencyLockRefs` and the read-only `/deps` mount (respawn on
   a new lock is acceptable in Phase 1). `ExternalAccess` carries grant IDs.
   The policy digest includes them. Relax the assertion at
   `engineering_assignment_runtime.go:718` to require an exact match with the
   assignment's bound grant set.
5. `Freeze` accepts lock refs; the verifier mounts them.
6. Crash boot closes open grants and fetches as `interrupted by a restart`.

Protected surfaces: capsule spawn and mounts, the capability token,
engineering grant attestation, a new egress path from guest core, and run
acceptance (freeze bindings).

Rollback: revert the mission commits. The capsule netns and seccomp are
untouched, so revert restores exact prior isolation. CAS objects are
inert.

Acceptance evidence (deployed E2E, repeatable artifact):

- **Positive:** owner prompt "replicate arXiv:XXXX with little compute" →
  engineering requests `git <paper repo>@<commit>` + `numpy`/`scipy` wheels →
  tier 0/1 grant → fetch receipts → capsule runs
  `python -c "import numpy; …"` and the repo's smallest test **with no
  network**. Freeze carries the `dep-lock` ref. The verifier passes offline.
- **Negative:**
  - a request for `dangerous-code.exe` (PE bytes served by a test URL) is
    hard-denied with a receipt;
  - `beautiful-code.md` that is really a shell script with `use: execute`
    goes to research, which denies, and the ledger shows it;
  - a confusable PyPI name (`nunpy`-style test fixture) lands at tier 2;
  - an out-of-scope name through a granted mirror socket returns 403 with a
    receipt;
  - a revoked grant mid-assignment refuses the next request;
  - `curl https://1.1.1.1` inside the work capsule still fails (netns
    unchanged);
  - a crash restart during a fetch closes the grant as interrupted, and it
    is not resumed.

Heresy delta: discovered D1–D4. Introduced: none intended (the guest-core
mirror inherits D2's guest egress until Phase 3; record that as a residual).
Repaired: D3, since relayed content for execution now requires a grant.

### Phase 2 — signals, research verdicts, categories, npm and Go (red)

- Full signal bundle (provenance and attestations, Scorecard, install-script
  detection, closure-wide OSV), the Jev verdict for tier 2, research
  precommit-and-resolve scoring, and category (predicate) grants issued
  from catalog v1.
- npm route (`npm ci --ignore-scripts` offline from `/deps/npm`; packages
  needing install scripts are tier 2, and their scripts run only inside the
  offline capsule). Go route (`GOPROXY=file:///deps/goproxy`, sumdb verified
  by the mirror, not the capsule). sdists are tier 2 and built offline in
  the capsule.
- Live `/deps` remount between cells (no respawn).
- Evidence: an E2E with a predicate grant where engineering pulls three
  wheels without new requests; a calibration report built from resolved
  research precommits; an npm `postinstall` package shown inert at fetch
  time.

### Phase 3 — move enforcement to Node B; close guest egress (red)

- A Node B shared mirror with a content-addressed cache (cross-computer
  de-duplication; identity is the per-VM transport binding). Guest tap
  `FORWARD` becomes default-deny to the internet; only gateway, source
  service and mirror stay reachable. That closes the
  `s0-tap-egress-unfiltered` residual and makes the full "never trust the
  guest" claim true: grant checks run where a compromised guest cannot
  reach around them.
- Research's `fetch_url`/`import_url` egress then also rides a host-side
  proxy with its own (research) policy.
- Evidence: from inside a guest, a dial to `1.1.1.1:443` fails while the
  mirror and gateway succeed; the tier ladder from Phase 1–2 is re-proven
  end to end.

### Phase 4 (optional) — live proxied egress inside a work capsule (red)

Only if real workflows cannot be resolved ahead of time, for example
build systems that fetch lazily. The work would be: a veth plus nftables
limited to the mirror, `AF_INET` in the workload seccomp filter, and the
mirror as `HTTP(S)_PROXY` with path policy (CA injection, or plain-HTTP
mirror URLs only). Same grants. It would carry a stronger tier floor
(`use: execute` context). The default recommendation is **not to build
this**.

---

## 6. Open questions for the owner

1. **Mirror location for Phase 1:** guest core (fast to land, inherits open
   guest egress until Phase 3) or straight to Node B (slower, but enforcement
   lives outside the guest from day one)? Recommendation: guest core first,
   Phase 3 soon after.
2. **Tier-0 auto-grants:** may the reducer grant category items without a
   management turn (only rendered to you in Texture after the fact)? Or
   should every grant get at least a management decision?
3. **Defaults:** cooldown (7 days proposed), popularity band (top-N, with N
   to choose), per-grant byte budget (500 MB?) and per-computer `/deps`
   storage cap (10 GB?). Large ML wheels such as `torch` CPU (~200 MB+) and
   model weights stress these limits.
4. **Model weights and datasets:** do pickled weights stay tier 3 (owner)
   with `safetensors` at tier 1–2? Is a Hugging Face route wanted in
   Phase 1 or Phase 2?
5. **Pre-approval UX:** should you be able to say "always allow popular
   PyPI wheels" once, as a durable category grant that outlives an
   assignment? Or should categories be re-issued per trajectory?
6. **Private repos and credentials:** out of scope for now? If they come in,
   credentials stay mirror-side (proxy-side injection, as Claude Code's
   sandbox masks credentials). They never enter a capsule (I19).
7. **Shared verdicts:** a vetted `numpy==2.1.3` is global knowledge. Should
   verdicts and catalog entries be publishable to other computers later (the
   Autopaper / publication-and-adoption path), or stay per computer?
8. **Research reading package bytes:** confirm that research's "read-only
   world authority" includes reading quarantined package archives **as
   data**, for vetting.

---

## 7. Sources

Code (all paths relative to repo root) are cited inline above.

External:

- GitHub Copilot coding agent firewall: recommended allowlist, domain and URL
  entries, limits ("should not be considered a comprehensive security
  solution"), and reporting of blocked requests —
  https://docs.github.com/en/copilot/how-tos/use-copilot-agents/coding-agent/customize-the-agent-firewall ;
  allowlist reference: https://docs.github.com/en/enterprise-cloud@latest/copilot/reference/copilot-allowlist-reference
- OpenAI Codex cloud agent internet access: off by default, presets
  (71-domain "common dependencies"), GET/HEAD/OPTIONS method restriction,
  prompt-injection example — https://learn.chatgpt.com/docs/cloud/internet-access ;
  commentary: https://simonwillison.net/2025/Jun/3/codex-agent-internet-access/
- Claude Code sandboxing: proxy outside the sandbox; netns on Linux with a
  socat Unix-socket bridge; hostname-only filtering without TLS termination;
  domain-fronting warning for broad domains like `github.com`; local-address
  refusal; credential masking at the proxy —
  https://code.claude.com/docs/en/sandboxing ; runtime: https://github.com/anthropics/sandbox-runtime
- Stripe Smokescreen: CONNECT proxy, per-client ACL over mTLS identity,
  resolved-IP public-address check against SSRF, open/report/enforce modes —
  https://github.com/stripe/smokescreen ; https://fly.io/blog/practical-smokescreen-sanitizing-your-outbound-web-requests/
- Nix fixed-output derivations: network allowed only when the output hash
  is declared — https://nix.dev/manual/nix/2.25/language/advanced-attributes
- Bazel downloader config (allow/block/rewrite to mirrors) and mandatory
  integrity hashes — https://blog.aspect.build/configuring-bazels-downloader ;
  https://bazel.build/external/overview
- Dependency cooldowns: npm `min-release-age`, pnpm `minimumReleaseAge`,
  uv `exclude-newer`, pip `--uploaded-prior-to` —
  https://nesbitt.io/2026/03/04/package-managers-need-to-cool-down ;
  https://securitylabs.datadoghq.com/articles/dependency-cooldowns/
- pip hash-checking mode — https://pip.pypa.io/en/stable/topics/secure-installs/ ;
  npm `ignore-scripts` — https://docs.npmjs.com/cli/using-npm/config#ignore-scripts ;
  Go checksum database — https://go.dev/ref/mod#checksum-database
- PyPI attestations (PEP 740) and Trusted Publishing — https://peps.python.org/pep-0740/ ;
  https://blog.trailofbits.com/2024/11/14/attestations-a-new-generation-of-signatures-on-pypi/
- OSV API and OpenSSF malicious packages (`MAL-` records) — https://google.github.io/osv.dev/api/ ;
  https://github.com/ossf/malicious-packages ; https://openssf.org/?p=11003 ;
  deps.dev API — https://docs.deps.dev/api/ ; OpenSSF Scorecard — https://github.com/ossf/scorecard
- Shai-Hulud npm worm (postinstall credential theft, self-propagation, Sept 2025) —
  https://cycode.com/blog/shai-hulud-npm-supply-chain-attack/
- Slopsquatting / package hallucination ("We Have a Package for You!", USENIX
  Security 2025: 19.7% of samples hallucinate; 43% of hallucinated names recur
  on every one of 10 reruns) — https://arxiv.org/abs/2406.10279 ;
  https://en.wikipedia.org/wiki/Slopsquatting
- Git recursive-clone RCE (CVE-2024-32002) — https://nvd.nist.gov/vuln/detail/CVE-2024-32002
- Lethal trifecta (private data + untrusted content + exfiltration channel) —
  https://simonwillison.net/2025/Jun/16/the-lethal-trifecta/
- CaMeL, "Defeating Prompt Injections by Design" (untrusted data cannot change
  control flow; capability-tagged values) — https://arxiv.org/abs/2503.18813 ;
  https://simonwillison.net/2025/Apr/11/camel/
- Biscuit / macaroons (offline attenuation, caveats, public-key verification) —
  https://www.biscuitsec.org/ ; https://engineering.clever-cloud.com/blog/engineering/2021/04/12/introduction-to-biscuit/
  This design uses reducer-held grant records rather than bearer tokens,
  because every check is online at the mirror. Biscuit-style attenuation is
  the model for category → item narrowing if grants ever need to be verified
  offline.
