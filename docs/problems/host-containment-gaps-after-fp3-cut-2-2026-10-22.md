# Host containment gaps after FP3 cut 2: a renamed deploy.env reappears, and four units have no hidden paths (2026-10-22)

Date: 2026-10-22, 17:05 UTC. Found by: FP0, in deploy T's deployed proof
(FP3 cut 2, staging f0b1ee313). Status: open, problem record before any
fix. Mutation class of the fix: red (host unit sandboxing, the deploy
program).

## Evidence

`scripts/verify-host-hardening.py --expect-internal enforce --expect-dolt
clients --expect-guest-floor --allow-reboot-pending`, run from deploy T's
tree as root on Node B:

- Before T (on S, ddd20de0e): 105 PASS, 61 FAIL (59 containment, 2
  credential). Evidence `_runs/fp3cut2/before-S.txt`.
- After T (f0b1ee313, 17:00): 138 PASS, 28 FAIL, all containment. The two
  credential checks and 31 containment checks now pass. Evidence
  `_runs/fp3cut2/after-T.txt`.

The 28 remaining failures are two different problems.

### 1. deploy.env reappears in every running unit after each deploy

Twelve failures are `/var/lib/go-choir/deploy.env`, readable (mode 644)
inside the namespace of the eight go-choir units (auth, proxy, gateway,
corpusd, maild, vmctl, platform-dolt, corpus-dolt) and the four units of
problem 2. `nix/node-b.nix` lists it in `hostSecretPaths` (`deployEnv`),
and every go-choir unit hides it through `InaccessiblePaths`.

Cause, confirmed by timestamps and code: the deploy program rewrites the
file by rename after the switch has restarted the units.
`.github/workflows/ci.yml:2057-2060`:

```
grep -v -E '^CHOIR_DEPLOYED_(AT|COMMIT)=' /var/lib/go-choir/deploy.env > /var/lib/go-choir/deploy.env.tmp || true
mv /var/lib/go-choir/deploy.env.tmp /var/lib/go-choir/deploy.env
```

On deploy T, go-choir-auth and go-choir-proxy started at 16:56:41 and
go-choir-vmctl at 16:57:48; deploy.env was replaced at 16:59:59 (new inode
908113653). `InaccessiblePaths` on a file bind-mounts over that file when
the unit's namespace is made. A later rename puts a new file at the path,
and running units see it. So after every deploy, every running unit can
read deploy.env until its next restart, and a unit that can write
`/var/lib/go-choir` can rewrite it for another unit's next start. That is
the case the record's own comment says the hiding prevents.

### 2. caddy, go-choir-goproxy, qdrant and searx hide nothing

The other sixteen failures are those four units, each reaching all five
checked paths: `/run/systemd/private`, `/run/dbus/system_bus_socket`,
`deploy.env`, `corpus-dsn.env` and `sandbox-gateway-token.env`.
`systemctl show -p InaccessiblePaths` is empty for all four. Cut 2's quick
containment covers the go-choir units only. The verifier checks every host
unit, so either the verifier's scope is wider than cut 2 meant, or these
four were missed. PID 1's private socket is the serious one: a root unit
that reaches it can ask systemd to run a command outside its sandbox.

## What still holds

The rest of T's proof passes. Dolt: corpusd, checkpointd (a one-minute
timer, last result success) and vmctl connect as the app user, with no
"Access denied" or dbcred line since 16:55. The issuer credential is held
by corpusd and vmctl, and no "issuer: would refuse" line followed the
paper's, vm-48bc0981's and the candidate fleet's boots on T. SEC-11 in
observe: a call to 127.0.0.1:8082/internal/platform/objects with the old
marker alone answers 200 and logs "hostauth: observe: would refuse ...
missing credential", and with the marker plus the credential answers 200
and logs nothing. FP3's runbook step 2 omits the marker header; with the
credential alone the route answers 403 "internal caller required".

## Fix options (not chosen here)

1. Problem 1: write deploy.env in place (same inode), or move the env files
   into a directory and hide the directory (a rename inside a hidden
   directory stays hidden), or rewrite deploy.env before the switch
   restarts units (still leaves units that do not restart exposed).
2. Problem 2: give caddy, goproxy, qdrant and searx the same
   `InaccessiblePaths` (and the other quick-containment settings that fit
   each), or narrow the verifier to the units cut 2 covers and record the
   four as a residual with a reason.
3. Correct FP3's runbook step 2 to send the marker with the credential.

## Addendum: more of the same class, found while fixing 1 and 2 (2026-10-22, before any fix)

Found by the lane worker on lane/fp3-containment-gaps (base 273d139c9),
reading code and the evaluated host configuration
(`nix eval .#nixosConfigurations.go-choir-b.config.systemd.services`). None
of these was observed on Node B; each is confirmed in code and stays a
hypothesis about the live host until FP0's probe.

### 3. The gateway replaces two hidden files by rename itself

`hostSecretPaths` hides `gateway-identities.json` and `codex-auth.json` as
files from every unit but the gateway. The gateway rewrites both by rename:

- `IdentityRegistry.persistLocked` (internal/gateway/config.go:248-266)
  writes `gateway-identities.json.tmp` and renames it over the store on
  every credential issue, rotation and revocation, so on most computer
  boots. The file holds SHA-256 hashes of bearer tokens and their pinned
  addresses.
- `ChatGPTAuth.write` (internal/provider/provider.go:3003-3033) creates a
  temp file beside `codex-auth.json` and renames it over the file on every
  ChatGPT token refresh. The file holds the owner's ChatGPT OAuth tokens,
  refresh token included.

By problem 1's mechanism, after the gateway's first such write following a
unit's start, that unit sees the new file until its next restart. Every
go-choir unit runs as uid 0 without capabilities, and a 0600 root file is
readable by its owner, so the proxy, auth, maild, corpusd, vmctl and both
Dolt servers could read the owner's ChatGPT tokens. vmctl (which can write
`/var/lib/go-choir`) could also rewrite the identity store. The verifier
does not check either path, so deploy T's proof could not see it.

Probe (FP0, Node B, root; prints inode and mode, never content):

```
for f in codex-auth.json gateway-identities.json; do
  echo "host $(stat -c '%i %a' /var/lib/go-choir/$f)"
  for u in go-choir-proxy go-choir-vmctl go-choir-corpusd; do
    pid=$(systemctl show -p MainPID --value $u)
    echo "$u $(nsenter -t $pid -m -- stat -c '%i %a' /var/lib/go-choir/$f 2>&1)"
  done
done
```

A unit line showing the host's inode means that unit can read the file.

### 4. corpusd-retrieval.env can be planted for corpusd's next start

corpusd reads `/var/lib/go-choir/corpusd-retrieval.env` through
`EnvironmentFile=` (the retrieval and opinion kill switch). The file is
normally absent and is not hidden from any unit. go-choir-gateway and
go-choir-vmctl have `/var/lib/go-choir` writable (`ReadWritePaths`) as
uid 0, so either can create it, and corpusd takes it at its next start. An
EnvironmentFile overrides `Environment=`, so the file can set any corpusd
variable, `CHOIR_OWNER_ACT_MODE` and `CORPUSD_DOLT_DSN` among them. The same
holds for every hidden file that is absent when a unit starts: the "-"
prefix skips an absent path, so nothing is mounted over it, and a file
created later is visible to the unit (and, if the unit can write
`/var/lib/go-choir`, creatable by it).

### 5. Two long-running host units are outside the verifier and hide nothing

go-choir-pkgmirror (NET-02; guests reach it on their tap at 8089) and
unbound (the guests' resolver from FP3 cut 1; guests reach it on 53) have
no `InaccessiblePaths` and are not in the verifier's `UNITS`. The oneshot
searx-init (it renders searx's settings) has none either. These are the
host services most exposed to guests.

### Exposure of problem 2's units

caddy runs as the user caddy; go-choir-goproxy, go-choir-pkgmirror, qdrant
and searx as dynamic users; unbound as unbound. None is root. PID 1 refuses
a connection on its private socket from a peer that is not root, polkit is
off on Node B, and `/var/lib/go-choir` is 0750 root, so these units could
not use what the verifier saw (it stats as root inside the namespace). The
system bus is still reachable to them. Hiding stays the floor for every
host unit, but problem 2 is a defense-in-depth gap, not an open path.
