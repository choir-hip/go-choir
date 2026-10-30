# Loopback Is a Credential on Node B

Status 2026-10-30: still a live credential record. The Dolt auth stage and Dolt residuals below are historical (Dolt is deleted); the remaining exposure is the host loopback services and the PostgreSQL listener. Design source for the new data system: [the data system frame](../design/choir-data-system-frame-2026-10-30.md).

Date: 2026-10-14. Source: the computer isolation security review
(docs/archive/reports/computer-isolation-security-review-2026-10-14.md, findings
F2 and F3, section 3.5), re-checked read-only on Node B at about 19:30 UTC
and against the code at origin/main 980ffaf4e. Status (2026-10-21): the
host-internal credential landed (44656ef90) and enforces (9a42a7abb); Dolt
auth is at stage "users" and owner acts are in observe; residuals below,
including the proxy object-graph route found 2026-10-21. Tracked as SEC-11
and SEC-12 in the [checklist](../security-density-architecture-checklist-2026-10-21.md).
Mutation class of any fix: **red** (auth/session, gateway credentials,
vmctl, deployment routing).

## The problem in one sentence

Every host-internal route on Node B decides "is this caller trusted?" from
the socket's source address alone (127.0.0.1, ::1, a unix socket, or the
test range 192.0.2.0/24), so every process in the host's network namespace
holds platform authority, and the two Dolt servers accept `root` with no
password from any local process.

## Who can call what today

"Internal" means: the request's `RemoteAddr` is loopback, empty/`@`/a path
(unix socket), or `192.0.2.*`. Some routes also want the header
`X-Internal-Caller: true`, which any caller can send, so it adds nothing
against a local attacker.

| service (port) | gate (code) | what an internal caller can do |
|---|---|---|
| vmctl (127.0.0.1:8083, unix `/run/go-choir/vmctl.sock`) | `isInternalCaller`, `internal/vmctl/handlers.go:1237-1255`: transport only, the header is ignored | stop, remove, hibernate, refresh, deploy-refresh, reclaim, hold, unhold, cold-recover, maintenance-serve any computer; list every ownership; resolve and look up any guest; relay to any guest through `/internal/vmctl/autoputer-proxy/`; apply ComputerVersion route projections (26 routes, `handlers.go:1261-1288`) |
| gateway (*:8084) | `isAuthorizedCredentialCaller`, `internal/gateway/handlers.go:709-715`: loopback + header | `/provider/v1/credentials/issue` (mint a gateway bearer for any computer), `/credentials/ensure` (import any `computerID:token` as a live identity and move the Jev pin, `config.go:381-398`), `/search/health`, `/search/health/reset`, `/breakers`, `/breakers/reset` |
| corpusd (*:8086) | `trustedInternalCaller`, `internal/platform/authority.go:21-63`: transport + header | every `/internal/platform/*` route: event heads, control key, checkpoints, file CAS, publications, object graph writes, key escrow, platform update, projection jobs, self-development modes |
| maild (*:8087) | `authenticatedInternalOwnerWithEmail`, `internal/maild/api.go:205-215`: loopback + header | act as any owner (`X-Authenticated-User`) with any email (`X-Authenticated-Email`) on the internal mail routes |
| every Go service | `serveDebugPprof`, `internal/server/debug_pprof.go:35` via `HostSourcedCaller` | CPU/heap/goroutine profiles and `cmdline` of the process |
| platform Dolt (127.0.0.1:13306) | none: `root@localhost`, empty password | read and write Store A: event heads, route slots, route ledger, escrow transparency |
| corpus Dolt (127.0.0.1:13307) | none: `root@localhost`, empty password | read and write Store B: the Autopaper object graph |

Live check, 19:30 UTC, from an SSH shell with no service identity:
`dolt --host 127.0.0.1 --port 13306 --no-tls --user root --password "" sql`
returned `mysql.user` (`root@localhost`, `__dolt_local_user__`,
`event_scheduler`) and the databases `platform` (13306) and `corpus`
(13307). The DSNs in `nix/node-b.nix` say the same:
`VMCTL_ROUTE_DSN=root@tcp(127.0.0.1:13306)/...`,
`CORPUSD_DOLT_DSN=root@tcp(127.0.0.1:13306)/...`, and the runbook's
`CORPUSD_CORPUS_DOLT_DSN=root@tcp(127.0.0.1:13307)/corpus...`.

## Who is "internal"

Every process in the host network namespace (live `ps`, 19:30 UTC):

- the Firecracker VMMs (4, root, in vmctl's mount namespace; F1);
- SearXNG (`searx`, DynamicUser), which fetches and parses untrusted web
  pages for the gateway;
- the Go module proxy (`go-choir-goproxy`, DynamicUser), which downloads
  and serves untrusted module archives to capsules;
- Qdrant (DynamicUser), Caddy (`caddy`);
- every root host service (auth, proxy, gateway, corpusd, maild, both Dolt
  servers, checkpointd, front-importance), each of which also reaches the
  others.

So a code-execution bug in SearXNG or goproxy, which run as unprivileged
users, is enough to destroy computers, mint gateway identities, impersonate
mail owners and rewrite the tape's heads: the unprivileged user does not
matter because loopback is the credential. The `192.0.2.*` allowance is a
test convenience compiled into production gates (`vmctl/handlers.go:1252`,
`platform/authority.go:33`); it cannot arrive on a real socket, so it is not
exploitable, but it is authority-by-location in production code. maild
has the same allowance (`isMaildLoopbackHost`), and the gateway's
runtime-peer rule (`isAuthorizedRuntimePeer`,
`internal/gateway/handlers.go:719-762`) accepts all three RFC 5737 ranges
(192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24) as if they were Choir
guests (found by the teaching-series review,
docs/learn/03-gateway-and-providers.md).

## The host services themselves (F3)

Live `systemctl show`, 19:30 UTC:

- auth, proxy, gateway, corpusd, maild, checkpointd, front-importance,
  goproxy and both Dolt servers have the full capability bounding set
  (`cap_chown cap_dac_override ... cap_sys_chroot`); auth, proxy, gateway,
  corpusd, maild, both Dolt servers, checkpointd and front-importance run
  as root (`User=` empty). Only searx, qdrant and goproxy are dynamic users.
- `commonServiceHardening` (`nix/node-b.nix:157-178`) sets
  `SystemCallFilter = [ "@system-service" "~@mount" "@privileged" ]`.
  systemd applies the list in order, so the third entry **re-allows**
  `@privileged` (systemd v257 `seccomp-util.c`: `@chown @clock @module
  @raw-io @reboot @swap acct bpf capset chroot pivot_root setuid setgroups
  ...`). The review saw `bpf`, `chroot`, `acct`, `add_key` allowed live.
- The proxy has `ReadWritePaths = [ auth-signing auth ]`
  (`node-b.nix:415`); it needs only `ed25519-key.pub`, yet it can read and
  replace the JWT private key `auth-signing/ed25519-key`.
- The gateway has `ReadWritePaths = [ "/var/lib/go-choir" ]`: every
  computer's `data.img`, every snapshot and every other service's secrets.
- vmctl (and so every VMM) has `ReadWritePaths = [ "/var/lib/go-choir" ]`
  and can read the JWT key, the provider credentials, the escrow key and
  the escrow operator tokens, none of which vmctl uses.
- No unit sets `ProtectHome`. No unit sets `SystemCallErrorNumber`, so a
  filtered call kills the process instead of failing.

## Why the services cannot simply move to non-root users yet

`/var/lib/go-choir` is `0750 root:root`, and it is the only thing that
keeps several world-readable files private (live `find -printf %m`):
`auth/` 0755 with `auth.db`, `auth.db-wal`, `auth.db-shm` 0644 (sessions
and API-key records); `vm-state/` 0755 (the review saw `data.img` 0644);
`platform-dolt/`, `corpus-dolt/`, `platform-artifacts/` 0755;
`sandbox-gateway-token.env`, `corpus-dsn.env`, `deploy.env` 0644. The
0755 directories come from `StateDirectory=` defaults
(`StateDirectoryMode` 0755), which override the tmpfiles `d ... 0750`
rules at every service start. A non-root service needs to traverse
`/var/lib/go-choir` (its binary pointer is
`/var/lib/go-choir/services/<name>/bin/<name>`, its state is under it), and
opening that directory to others would expose all of the above. The same
trap already bit goproxy
(docs/archive/problems/goproxy-unit-cannot-create-cache-2026-10-14.md). Moving
services to their own users therefore needs, in order: tighten the inner
modes, give `/var/lib/go-choir` a traversal group, then move each service
with a recursive `StateDirectory` chown.

Every file in the state directories of auth, proxy, gateway, corpusd,
maild and both Dolt servers is root-owned and owner-writable except two
immutable 0444 artifact blobs that vmctl writes (live `find`). So root
services lose nothing when their capability bounding set is emptied:
owner permissions already grant them their own files.

## CI

The deploy (`.github/workflows/ci.yml`, `REMOTE_DEPLOY` heredoc) runs as
root over SSH and calls vmctl's `/internal/vmctl/list`,
`/internal/vmctl/deploy-refresh` and `/internal/vmctl/refresh` with only
the header. Ad-hoc probes (`scripts/vmctl_*_probe.mjs`,
`scripts/m11_selfdev_episode_probe.mjs`, `scripts/lib/selfdev_approval.mjs`,
`frontend/tests/s0a-reality-probe.mjs`) do the same. The deploy key being
root is F13, out of scope here.

## What the repair must do

1. **A host-internal credential, not a location.** Internal routes
   (vmctl, gateway credential and admin routes, corpusd, maild, pprof)
   require a secret that only intended callers hold, in addition to the
   transport check. The secret lives in a root-only 0600 file and reaches
   services through systemd `LoadCredential=`, never through the
   environment (VMMs inherit vmctl's environment) and never in a command
   line. A rollout flag logs-but-allows first (observe), then refuses
   (enforce). Refusals log the route and the reason, never the secret.
2. **The secret never leaves the host.** Clients attach it only to
   loopback destinations; servers strip it from every request at ingress so
   no reverse proxy (vmctl's autoputer relay, the proxy) can forward it to
   a guest.
3. **Dolt authenticates.** An application user with a password and
   privileges on its one database; then root gets a password. Staged so no
   deploy flips clients and server at once.
4. **systemd hardening that is safe now:** fix the filter, empty the
   capability bounding set where no capability is used, take the JWT key
   and other secrets out of the mount view of services that do not read
   them, `ProtectHome`, tighten the inner state modes, and leave the
   user migration as the next, separately staged step.

## Pre-deploy baseline of the verifier

`scripts/verify-host-hardening.py` (read-only; run on Node B as root:
`ssh node-b python3 - --expect-internal observe --expect-dolt users <
scripts/verify-host-hardening.py`), at origin/main 980ffaf4e, about 21:00
UTC: 30 passed, 44 failed. Every internal probe answered 200 without a
credential (vmctl list and pprof, gateway breakers and search health,
corpusd control key and pprof, maild drafts for a made-up owner); root
services hold `CapBnd=000001fff7fcffff`; every `go-choir-*` filter still
allows `bpf chroot acct setuid capset`; proxy, vmctl and gateway see the
JWT private key at mode 600 in their mount namespaces; both Dolt servers
accept root with an empty password; `auth/` 755, `auth.db` 644,
`vm-state/`, `platform-dolt/`, `corpus-dolt/`, `platform-artifacts/` 755,
`corpus-dsn.env` and `sandbox-gateway-token.env` 644. After each rollout
step the same command, with the stage's expectations, is the acceptance
check.

## Residuals the repair does not close

- **F1 still wins.** The VMMs are root in vmctl's mount namespace. vmctl
  must hold the internal credential (it calls corpusd and the gateway), so
  a VMM escape can read `/run/credentials/go-choir-vmctl.service/`. The
  credential defends against every non-root process (SearXNG, goproxy,
  Qdrant, Caddy) and, after the user migration, against every service that
  is not a caller; against a VMM escape it waits for the jailer (J1).
- A root process can sniff loopback (`CAP_NET_RAW`). Once services drop
  their capability sets, only vmctl (`CAP_NET_ADMIN`) and the VMMs remain
  able to; a per-request HMAC would close it and is not needed until then.
- One shared secret, not one per caller: any credential holder can call
  any internal route. Per-caller secrets or `SO_PEERCRED` per route is
  the next step once services have distinct uids.
- **The proxy's object-graph route still trusts loopback** (found
  2026-10-21, code at 8f71f9c64). `requirePlatformObjectGraphCaller`
  accepts a loopback peer plus `X-Internal-Caller: true` without
  `hostauth.Authorize`, and `trustedInternalProxyTransport` also admits
  `192.0.2.*` in production (`internal/proxy/guest_authority.go:20-51`).
  The proxy then forwards to corpusd marked internal
  (`internal/proxy/handlers.go:1799-1820`), so any host process can borrow
  corpusd's internal authority on `/internal/platform/objects*`.
  Two more facts for the repair (FP3 group 3, SEC-11, 2026-10-21):
  `hostauth.MarkInternal` sends the credential only to ports 8083, 8084,
  8086 and 8087 (`internal/hostauth/hostauth.go` `defaultPorts`), never to
  the proxy's 8082, so a legitimate host caller of this route could not
  pass `Authorize` even if it wanted to; and the proxy unit already runs
  `CHOIR_INTERNAL_AUTH_MODE=enforce`, so calling `Authorize` here with no
  route-level mode would enforce on the first deploy, with no observe
  window to find callers. Caddy refuses `/internal/*` at the public edge
  and guests arrive from their tap addresses, so the loopback branch
  serves only host processes. The repair adds 8082 to the credential's
  ports, gates the loopback branch on `Authorize` behind a route mode
  (`PROXY_OBJECT_GRAPH_INTERNAL_AUTH`, shipped `observe`), and takes
  TEST-NET-1 from `hostauth.TestNetworkAllowed` (test binaries only).
