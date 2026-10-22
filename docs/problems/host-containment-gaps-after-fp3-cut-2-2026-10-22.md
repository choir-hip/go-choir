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

## Fix (lane fp3-containment-gaps, 2026-10-22)

Commits on lane/fp3-containment-gaps (base 273d139c9): 49b2f1790 (the
addendum above), 545d573fd (tests, yellow), 844e0026b (fix, red), and the
docs commit that adds this section.

**Problem 1: the directory, not an in-place rewrite.** deploy.env,
corpus-dsn.env, corpusd-retrieval.env and sandbox-gateway-token.env move
into `/var/lib/go-choir/unit-env` (0700 root). Every unit hides the
directory (`hostSecretPaths.unitEnv`), not each file. Why the directory:

- Writing deploy.env in place would fix only the one CI writer. Any other
  rename (sed -i, an editor, a future script) still reappears in running
  units, and every future writer would have to remember to keep the inode.
- A file absent when a unit starts is never hidden: "-" skips it. A later
  create is visible to the unit, and a unit that can write
  `/var/lib/go-choir` can plant it (problem 4). Inside a directory that
  always exists and is always hidden, neither can happen. The unit's view
  of the directory is systemd's read-only empty node, and the mountpoint
  cannot be renamed from inside.
- An in-place rewrite can be read half-written by a unit that starts during
  it (systemd reads an EnvironmentFile at start).
- `secrets/` already works this way.

The deploy's scrub is deleted, not moved. It stripped
CHOIR_DEPLOYED_AT/COMMIT, which nothing has written since the receipt
(f2d1d330c, 2026-07-10), and buildinfo ignores them. Its only effect was
a new inode after every deploy's restarts. The deploy contract test now
rejects any `deploy.env` in the workflow and the Node B programs.

The move is an activation snippet (`go-choir-unit-env`). Activation runs at
boot before units start and in a switch before units restart, so every
unit's new namespace is made with the directory present. The first run
renames the files in, which keeps inode and contents on one filesystem,
and writes a marker. Later runs never adopt a file found at an old path:
a unit that can write `/var/lib/go-choir` could have planted it. Each old
path keeps a link to `unit-env/<name>`. A rollback past the fix reads
through it, because systemd follows the link for EnvironmentFile= and for
InaccessiblePaths=. An old runbook's `>` writes through it, but `rm` of an
old path removes only the link. A regular file at an old path is left in
place (no unit reads it) and fails the verifier. A link or file planted at
the directory's own path is moved aside, never followed.

**Problems 2 and 5.** caddy, qdrant, unbound, go-choir-goproxy and
go-choir-pkgmirror hide the whole set. searx and searx-init hide all of it
except their own searxng.env, per FP0's ruling; hiding it would gain
nothing, because the process already holds SEARXNG_SECRET in its
environment and in /run/searx/settings.yml. None of these units uses D-Bus
or PID 1's socket:

- caddy reloads through its own admin API;
- name lookups go through nscd;
- searx reads its settings from /run/searx.

pkgmirror and unbound join the verifier's units. Nothing in the verifier
was narrowed.

**Problem 4** is closed by the directory, because corpusd-retrieval.env
moved into it.

**Problem 3 stays open.** A proposed fix: a gateway-only directory hidden
from every other unit, with GATEWAY_IDENTITY_STORE_PATH pointing into it,
and codex-auth.json moved there too. Moving codex-auth.json needs either
CHATGPT_AUTH_PATH changed in the owner's gateway-provider.env, written in
place, or a gateway writer that resolves a link before it renames. Then
the verifier checks both paths in every other unit. FP0's probe above
comes first.

### Operator paths

- corpus-dsn.env (Store B DSN) is now
  `/var/lib/go-choir/unit-env/corpus-dsn.env`; the runbook
  docs/runbooks/platform-dolt-authority-split-2026-09-18.md is updated.
- The retrieval and opinion kill switch is now
  `/var/lib/go-choir/unit-env/corpusd-retrieval.env`.
- The proxy's environment is now `/var/lib/go-choir/unit-env/deploy.env`.

Older notes that say `echo ... > /var/lib/go-choir/corpusd-retrieval.env`
still work through the link. Turning a switch off must remove the file in
unit-env.

### Residuals

- Problem 3, above.
- The other file entries (gateway-provider.env, maild.env, searxng.env,
  codex-auth.json, gateway-identities.json, the JWT and platform keys) are
  still hidden one by one. An operator's rename of one exposes it to
  running units until they restart.
- The oneshot units without hidden paths: disk-gc, nix-gc,
  btrfs-headroom, store-compaction, digest, platform-dolt-squash,
  platform-dolt-history-audit, the two dolt-users units and the three
  offhost-backup units. Several need what they see (the backup reads every
  secret; dolt-users reads the Dolt root password; store-compaction runs
  systemctl).
- The deploy classifier puts `scripts/off-host-backup.sh` under "unknown
  deployed path" (host OS, plus guest image and active-computer refresh),
  although nix/node-b.nix is its only reader. This landing therefore
  refreshes active computers unless a classifier case is added.
- CI does not run scripts/test_host_containment_nix.py; nor does it run
  snapshot-vault-nix-eval-test.sh. It needs nix and evaluates the host
  closure.
- The first deploy restarts every unit whose unit file changes, as cut 2's
  did. At the switch, that is both Dolt servers, caddy, qdrant, searx,
  unbound, goproxy and pkgmirror; the deploy program restarts the rest.
  caddy's restart drops open edge connections, and clients reopen them.
