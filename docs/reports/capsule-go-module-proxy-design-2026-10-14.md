# Design: A Host Go Module Proxy for Engineering Capsules

Date: 2026-10-14. Mutation class: **red** (capsule egress policy, a new host
service, a new guest-to-host tap port). Status (2026-10-21): landed on main
(3c6347e81, c841fa474; host unit in `nix/node-b.nix`, tap port 8088). Other ecosystems' mirrors are item
NET-02 of the [checklist](../security-density-architecture-checklist-2026-10-21.md).

Decision source: the owner, 2026-10-14, on
[capsule-go-modules-decision-2026-10-14.md](capsule-go-modules-decision-2026-10-14.md):
"I like the Go module cache, and I like the better version with a small
[module] proxy on the host." That picks option B (no Go egress beyond the
platform) in its read-through form, which also answers B's open cost (a
change that adds a new module still needs the network).

Evidence: [c1e-b1-engineering-assignments-cancelled-on-arrival-2026-10-14.md](../problems/c1e-b1-engineering-assignments-cancelled-on-arrival-2026-10-14.md),
cause 3. proxy.golang.org answers every module zip request with a redirect
to `storage.googleapis.com`, which the L2 egress proxy refuses, so
`go build ./cmd/autoputer` fails in every implementation capsule and no
Choir change can reach Freeze.

## 1. Shape

```
capsule workload (go build)
  GOPROXY=http://goproxy.choir.internal,off   HTTP_PROXY=http://127.0.0.1:3128
        |  plain-HTTP GET, absolute URL
        v
broker loopback forwarder 127.0.0.1:3128  ->  /run/capsule/egress.sock
        v
guest executor EgressProxy (internal/capsule/egress.go)
  route "goproxy.choir.internal" -> <host tap IP>:8088 (configured, not resolved)
  GET/HEAD only, port 80 only, X-Choir-Capsule header set, budgets apply, logged
        v
host tap 10.200.X.1:8088  --DNAT-->  127.0.0.1:8088
        v
go-choir-goproxy (cmd/goproxy, internal/goproxy) on Node B
  serves only GOPROXY and sumdb-proxy paths; cache under /var/lib/go-choir/goproxy
  cache miss -> https://proxy.golang.org (redirects followed on the host,
                only to storage.googleapis.com) or https://sum.golang.org
```

### 1.1 The capsule side

`capsule.EgressProxyEnv()` (the L2 environment the broker hands to every
workload) gains three keys:

- `GOPROXY=http://goproxy.choir.internal,off`. The name is fixed; the
  capsule never learns or needs the host address. `,off` means a module the
  host proxy cannot serve fails loudly instead of falling back to a route
  that is blocked anyway.
- `GOSUMDB=sum.golang.org` (the default, stated explicitly). See §2.
- `GOFLAGS=-mod=mod`, so a build may record a missing `go.sum` line instead
  of refusing.

`yaegikernel.EgressProxyKeys` (what the session worker passes to the
commands a cell runs) gains the same three keys; the existing test pins the
two lists together. Both exec paths append the L2 environment after the
cell's own, so a cell cannot point the go command elsewhere (and cannot
replace `GOFLAGS` through the environment; flags on the command line still
work).

### 1.2 The allowlist change

The L2 policy gains one **route**, not a DNS suffix: the name
`goproxy.choir.internal` maps to an address the executor reads from its own
trusted configuration. For that name the proxy:

- accepts only plain-HTTP `GET`/`HEAD` on port 80 (a `CONNECT` is refused:
  there is no TLS to tunnel and nothing else lives there);
- dials the configured address directly. It does no DNS lookup and skips the
  public-address check, because the target is the host peer on the tap
  subnet, which is private by construction. The capsule cannot choose the
  address: it can only name the route;
- overwrites `X-Choir-Capsule` with the capsule id, so the host can attribute
  fetches (attribution, not authority);
- still counts the connection and its bytes against the capsule's budgets,
  and still writes one egress record per request.

Nothing else is added. `storage.googleapis.com` stays off the list.
`proxy.golang.org` and `sum.golang.org` stay on it for now: with
`GOPROXY=...,off` the go command no longer contacts them, and removing them
is a separate narrowing (residual R4).

Where the address comes from: `CHOIR_CAPSULE_GOPROXY_ADDR` if set, else the
host of `RUNTIME_GATEWAY_URL` (the tap peer, `10.200.X.1`) with port 8088.
Every guest already has `RUNTIME_GATEWAY_URL` from the kernel command line,
so no new boot parameter is needed. With neither, the route is absent and a
capsule's module fetch is refused with a reason that names the missing host
proxy.

### 1.3 The host side

**Service.** `cmd/goproxy` (logic in `internal/goproxy`, stdlib only, so the
Nix `vendorHash` does not change). Systemd unit `go-choir-goproxy` on Node B,
listening on `127.0.0.1:8088`. The vmmanager adds 8088 to
`tapReachableHostServicePorts`, which gives each tap an INPUT accept and a
PREROUTING DNAT `10.200.X.1:8088 -> 127.0.0.1:8088`, the same path the
gateway, vmctl, corpusd and maild use. The firewall keeps 8088 closed
externally (only 22/80/443 are open) and the bind is loopback, so only host
processes and guest taps reach it.

**Paths served** (everything else is a 404 with a reason, logged as
refused):

| path | handling |
|---|---|
| `/<mod>/@v/<ver>.info`, `.mod`, `.zip` with a canonical semver `<ver>` | cached (immutable) |
| `/<mod>/@v/<query>.info` (branch or other non-canonical query) | passed through, not cached |
| `/<mod>/@v/list`, `/<mod>/@latest` | passed through, not cached (mutable) |
| `/sumdb/sum.golang.org/supported` | 200, empty body |
| `/sumdb/sum.golang.org/tile/...` (full tiles) | cached (immutable) |
| `/sumdb/sum.golang.org/tile/....p/<w>` (partial tiles), `/latest`, `/lookup/<mod>@<ver>` | passed through, not cached |
| `/healthz` | cache size and entry count |

`<mod>` and `<ver>` are checked as escaped module paths and versions
(lowercase letters, digits, `-._~`, `!`+lowercase escapes; no empty, `.` or
`..` element; first element has a dot; bounded length). The upstream URL is
always `<fixed base> + <checked path>`; the proxy never takes a host, a
scheme or a full URL from the request.

**Redirects.** The upstream client follows at most five redirects, each
only to `https` and only to a host on a fixed list (`proxy.golang.org`,
`storage.googleapis.com` for module requests; `sum.golang.org` for sumdb).
This is the step the capsule could not do, now done on the host where it
is not a capsule egress decision.

**Cache.** `/var/lib/go-choir/goproxy/cache/<escaped mod>/@v/<ver>.<ext>`,
which is the `GOMODCACHE/cache/download` layout and therefore itself a valid
GOPROXY tree; sumdb tiles under `sumdb/sum.golang.org/tile/...`. Writes go
to a temp file in the same directory and are renamed into place; an existing
entry is never overwritten (append-only). Concurrent misses for one entry
share one upstream fetch.

**Size bound.** Measured from this commit's `go.sum` (1,617 lines: 275
module zips, 1,342 `go.mod` files): the full warm of every line caches
**453 MB** (`.info`, `.mod` and `.zip` files) and took 28 s from this Mac
against the real proxy.golang.org; `go build ./cmd/autoputer` needs 161
modules (483 requests: one `.info`, `.mod` and `.zip` each). So the
deployed set is under half a GiB, and the cache grows only by modules
engineering adds. The cap is **8 GiB** (flag `-max-bytes`); past it, the
oldest files by modification time are deleted down to 90% of the cap. The
pre-warm touches every file it verifies, so the deployed set is always the
youngest and is evicted last.

**Per-source limits.** Hits are unlimited. Upstream fetches (misses) are
limited to 4 at a time per source address (one source address is one
computer; the per-capsule budgets live in the guest egress proxy) and 16 at
a time in total. Response size caps: zip 500 MiB (Go's own module zip
limit), everything else 16 MiB.

**Logging.** One JSON line per request to the journal: time, source
address, capsule id (from `X-Choir-Capsule`), path, outcome (`hit`,
`fetched`, `passthrough`, `refused`, `not_found`, `upstream_error`,
`hash_mismatch`), status, bytes, duration.

### 1.4 Pre-warm

The package carries the deployed commit's `go.sum`
(`share/go-choir/go.sum`, copied in by the flake). On start the service
runs a background warm over it: for every `go.sum` line it makes sure the
`.mod` (and for module lines the `.info` and `.zip`) is in the cache,
fetching through its own read-through path, and checks each file's `h1:`
hash against the `go.sum` line (the `dirhash` Hash1 algorithm, done with
the standard library). A mismatch deletes the file and logs
`hash_mismatch`; it never fails the service. The warm also re-hashes files
already cached, so on-disk corruption is found at every deploy.

When it runs: the unit's `ExecStart` names the package's store path, which
changes with every host deploy, so `switch-to-configuration` restarts the
service (default `restartIfChanged`) and the warm runs against the new
`go.sum` at deploy time. A `go.sum` change always takes the host-OS deploy
class (`.github/scripts/deploy-impact-classify`: `go.mod|go.sum` marks the
guest boot contract, which sets `deploy_host_os`), so the warm cannot miss a
dependency change.

Why not `go mod download` in a oneshot, as first sketched: the in-service
warm needs no Go toolchain, no scratch `GOMODCACHE` (which `go mod download`
would extract, about 1 GiB), and no second writer of the cache tree; it
verifies against the same `go.sum` that `go mod download` would. The cache
layout is still the `go mod download` layout, so a later switch to a
`go mod download` warm (or `file://` mounting the tree into capsules) needs
no migration.

**restartIfChanged.** 2db4bc02 set `restartIfChanged = false` for auth,
proxy, maild, corpusd and the gateway because their `ExecStart` runs a
service *pointer* that the deploy updates after the switch, so a switch-time
restart ran the old binary. The goproxy unit runs the Nix store path
directly (no pointer), so a switch-time restart runs the new binary; the
default (`true`) is the consistent choice for the same reason. The service
drains in-flight requests for up to 45 s on SIGTERM.

## 2. Checksum verification: who verifies what

The capsule's go command verifies; the host proxy also verifies what it can
verify cheaply. Neither needs direct `sum.golang.org` egress.

- **Modules in the deployed `go.sum`** (the whole platform build): the
  capsule's go command checks every `.mod` and `.zip` against `go.sum` (it
  refuses a mismatch as a security error). The host proxy checks the same
  hashes on every miss and in the warm, and refuses to cache or serve a
  mismatch (502). So a poisoned upstream response never enters the cache for
  this set.
- **Modules not in `go.sum`** (a desk adds a dependency): the capsule's go
  command consults the checksum database through the proxy, using Go's
  built-in sumdb proxying: it first fetches
  `$GOPROXY/sumdb/sum.golang.org/supported`; a 200 makes it use
  `$GOPROXY/sumdb/sum.golang.org/{latest,lookup/...,tile/...}` for all
  checksum database traffic and never fall back to a direct connection
  (`cmd/go/internal/modfetch/sumdb.go`, design 25530 "proxying a checksum
  database"). The signed tree heads are checked against the `sum.golang.org`
  key compiled into the go command, and lookups are proven against tiles, so
  the host proxy cannot forge a checksum; at worst it can withhold one, and
  the build fails. The host proxy does not verify these modules itself; a
  bad upstream zip can enter the cache, but every capsule refuses it, so the
  harm is denial of that one module version until an operator deletes the
  entry (residual R3).

## 3. Failure modes (written before the code)

Capsule and egress side:

- F1. The capsule reaches something other than the host proxy through the
  new route (another host service, another port, an arbitrary IP).
  *Guard:* the route is a fixed name mapped to one configured address; the
  capsule supplies only a path. CONNECT to the name is refused.
- F2. A capsule names an IP literal or a DNS name that resolves to the host
  peer and reaches 8088 (or 8084) that way. *Guard:* unchanged: IP literals
  and private resolutions are refused; only the route name skips the check.
- F3. The route is used with POST or another upload method. *Guard:* plain
  HTTP stays GET/HEAD only.
- F4. The capsule forges `X-Choir-Capsule` to blame another capsule.
  *Guard:* the proxy overwrites it.
- F5. Requests through the route bypass the per-capsule byte and connection
  budgets or the egress record. *Guard:* the route shares the same admit,
  charge and record path.
- F6. The go env reaches the broker's direct-exec path but not a cell's
  commands (or the reverse). *Guard:* the existing key-list test.
- F7. No address is configured (local dev, an old guest) and the fetch
  hangs or goes somewhere else. *Guard:* the route is absent and the name is
  refused with a reason.

Host proxy side:

- F8. A request makes the proxy fetch an arbitrary URL (SSRF), including
  through `..`, encoded slashes, a full URL in the path, or a redirect.
  *Guard:* strict path grammar; upstream is a fixed base plus the checked
  path; redirects only to https hosts on a fixed list.
- F9. A path writes outside the cache directory. *Guard:* the same grammar
  (no `.`/`..` elements, no backslash, no NUL), and the cache path is built
  only from checked elements.
- F10. A mutable answer (`list`, `@latest`, a branch query, `latest`, a
  partial tile, a lookup) is cached and goes stale. *Guard:* only canonical
  `.info/.mod/.zip` and full tiles are cached.
- F11. An upstream 404/410 becomes a 5xx (the go command then reports the
  wrong error), or a 5xx becomes a 404. *Guard:* 404/410 pass through with
  their text; other upstream failures are 502.
- F12. A cached file is overwritten by a later fetch, or a half-written file
  is served after a crash. *Guard:* temp file + rename; an existing entry
  wins.
- F13. Two concurrent misses for one zip both fetch and race on the rename.
  *Guard:* one fetch per key in flight.
- F14. A poisoned or corrupt upstream response for a `go.sum` module is
  cached and served. *Guard:* h1 check on every miss and in the warm.
- F15. A huge upstream response fills the disk or memory. *Guard:* per-kind
  size caps while streaming; the 8 GiB cap with oldest-first eviction.
- F16. One computer's fetch storm starves the others or hammers upstream.
  *Guard:* per-source and global miss concurrency.
- F17. The warm blocks the service start or a deploy (minutes of
  downloading), or a warm failure crashes the service. *Guard:* the warm
  runs in the background and only logs.
- F18. A deploy's restart cuts a capsule's in-flight download. *Guard:*
  graceful drain on SIGTERM; a cut fetch is a failed `go build` the desk can
  rerun (residual R2).
- F19. The proxy cannot reach upstream at all. *Guard:* cached modules are
  still served; misses return 502 and are logged.

## 4. Standing questions

1. *Who settled this?* The owner, 2026-10-14 (quoted above). Placement
   details (port 8088, the route name, the in-service warm) are this
   design's choices.
2. *Settled-decision conformance.* Host services bind loopback and reach
   guests through tap DNAT (as gateway/vmctl/corpusd/maild do). Capsule
   egress stays the L2 proxy's decision, outside the capsule
   (engineering-network-grants §4.0b). No new store.
3. *Deletion citers.* Nothing is deleted.
4. *Production consumers.* Engineering capsules' go command; the warm.
5. *Single state authority.* The cache is a derived, disposable mirror of
   proxy.golang.org whose authority is `go.sum` plus the checksum database;
   deleting it loses only time.
6. *Artifact-verified success.* The staging proof (§6) is a frozen bundle
   whose `bin/autoputer` came from an in-capsule `go build`, plus the
   egress ledger lines naming `goproxy.choir.internal`.
7. *Fate-sharing.* A capsule Go build fate-shares with the host proxy and
   the tap path. If the proxy is down, cached modules cannot be served
   either (the cache is on the host). Nothing else depends on it; the proxy
   depends on nothing but its directory and, for misses, upstream.
8. *Restart durability.* The cache is on disk and survives restarts; the
   in-flight singleflight table and the semaphores are process-local and
   their loss is harmless. The warm reruns on every start.
9. *No-SSH operability.* Partly: the capsule's own egress ledger and the
   go command's errors show what failed. The host's per-request log is in
   the journal only (residual R5).

## 5. Red ceremony

- **Conjecture delta.** Before: engineering capsules cannot build Go
  without opening `storage.googleapis.com`. After: they can, with no new
  internet host reachable from a capsule, through one host route that
  serves only module paths.
- **Protected surfaces touched.** Capsule egress (L2 policy and env),
  vmmanager tap host-service ports, a new Node B host service, the flake
  service build list. Not touched: Texture canonical writes, gateway or
  provider calls, auth, run acceptance, checkpoint or route projection.
- **Admissible evidence.** Unit and local integration: the GOPROXY protocol
  against a fake upstream (redirects, cache hit/miss, refusals, sumdb
  proxy, hash checks), the egress route tests, and a real `go build` of
  this repo against the proxy with an empty module cache on this Mac. The
  capsule path itself is staging-only: local proof does not count for it.
- **Rollback.** Revert the commits. The capsule env loses `GOPROXY` (back
  to the default proxy.golang.org, which fails as before), the route and
  port disappear, and the unit is removed by the next switch;
  `/var/lib/go-choir/goproxy` can be deleted at leisure. No durable state
  depends on it.
- **Heresy delta.** Discovered: none new (the blocked build is cause 3 of
  the problem doc). Introduced: a guest-to-host tap port (8088) serving
  internet-derived bytes, mitigated by loopback bind, the path grammar and
  hash checks. Repaired: cause 3 (no module zips in capsules) once the
  staging proof passes.

## 6. Staging proof (after landing)

1. Landing loop: push, CI, host deploy (expect a host-OS switch: `nix/`,
   `flake.nix`, `cmd/`, `internal/` all change), staging identity on the new
   commit.
2. Host: `systemctl status go-choir-goproxy` active; journal shows the warm
   summary (`"event":"warm_done"`, `failed: 0`, `mismatch: 0`);
   `curl -s 127.0.0.1:8088/healthz` shows about 453 MB.
3. Drill: `node scripts/reeng_tracer.mjs --scenario engineering`. Expect
   engineering's `go -C /workspace/platform build -o
   /var/lib/artifact/release/bin/autoputer ./cmd/autoputer` to exit 0 and
   Freeze to succeed (no "no frozen runtime artifacts", no Forbidden module
   downloads).
4. Evidence: the capsule egress ledger
   (`/mnt/persistent/capsule-artifacts/egress/<capsule>.jsonl` in the
   guest) has `host: goproxy.choir.internal` lines with outcome `ok` and no
   `storage.googleapis.com` refusals; the host journal has `hit` lines
   carrying that capsule id.

## 6a. Local evidence (this Mac, 2026-10-14; not capsule-path proof)

- `go test ./internal/goproxy` (14 tests, F8-F17, including the real go
  command running `go mod download` through the proxy against a fake
  upstream that redirects zips to a second host) and
  `go test ./internal/capsule/...` (route tests F1-F7) pass, with `-race`.
- The built `cmd/goproxy` against the real proxy.golang.org, warming this
  commit's `go.sum`: `warm_done` with 1,617 entries, 1,617 fetched, 0
  failed, 0 mismatched, 453 MB, 27.6 s.
- Then `go build ./cmd/autoputer` with an empty `GOMODCACHE`,
  `GOPROXY=http://127.0.0.1:18088,off`, `GOSUMDB=sum.golang.org`,
  `GOFLAGS=-mod=mod`: exit 0, 178 MB binary, 483 requests, all cache hits,
  `go.sum` unchanged.
- The sumdb path, with a module outside this `go.sum`
  (`go get rsc.io/quote@v1.5.2`, empty `GOMODCACHE`): the go command took
  `/sumdb/sum.golang.org/supported`, then lookups and tiles through the
  proxy (full tiles cached, partial tiles and lookups passed through),
  verified real signed tree heads, and wrote correct `go.sum` lines.
- `nix derivation show` evaluates the `goproxy` package (binary plus
  `share/go-choir/go.sum`) and renders the `go-choir-goproxy.service` unit
  for `go-choir-b`. Nothing was built for Linux here.

Seen while measuring: `go mod download` with `-mod=mod` added 436 lines to
`go.sum` (sums for test-only dependencies of dependencies); `go build`
added none. A desk that runs `go mod download` in the workspace therefore
changes `go.sum`, and that change becomes part of its frozen subject
(residual R8).

## 7. Residuals

- R1. Not proven on Linux here: the tap DNAT to 8088, the systemd unit
  (`DynamicUser`, nested `StateDirectory`), and the flake build of the new
  package are checked by reading only; the first staging deploy proves them.
  A unit that cannot start fails the host deploy's switch, which is loud.
- R2. A deploy restarts the proxy (seconds of refusals). A capsule build in
  that window fails and must be rerun.
- R3. Modules outside `go.sum` are not hash-checked on the host; a bad
  upstream answer for one is cached and refused by every capsule until it is
  deleted by hand.
- R4. `proxy.golang.org` and `sum.golang.org` remain on the L2 allowlist and
  could now be removed.
- R5. The host request log is journal-only; an agent without SSH sees only
  the capsule side.
- R6. Verification capsules (network `forbidden`) still have no module
  source; an offline verifier rebuild needs the cache tree mounted read-only
  (`GOPROXY=file://...`), the original option B. The cache layout is chosen
  so that this needs no migration.
- R7. Busy computers that defer the landing deploy keep the old guest code
  (no route) until their next refresh.
- R8. `GOFLAGS=-mod=mod` lets a capsule's go command edit `go.mod`/`go.sum`
  instead of refusing; `go mod download` adds hundreds of `go.sum` lines.
  Such edits become part of the desk's frozen change. The engineering
  overlay could say "build, do not `go mod download`".
- R9. The guest-side file `internal/autoputer/capsule_executor_linux.go`
  (reads the address) was not compiled here: its package needs cgo for
  Linux, which this Mac cannot cross-compile. CI compiles it.


## Merged from docs/reports/capsule-go-modules-decision-2026-10-14.md (2026-10-14)

**Decision: How Engineering Capsules Get Go Modules.**
Date: 2026-10-14. For the owner. Found by the C1e B1 engineering drill (docs/problems/c1e-b1-engineering-assignments-cancelled-on-arrival-2026-10-14.md).

(Full text in git history at the path above.)
