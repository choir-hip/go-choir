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
into `/var/lib/go-choir/unit-env` (0700 root). Every long-running host
unit, and every oneshot that declares hidden paths, hides the directory
(`hostSecretPaths.unitEnv`), not each file. The thirteen oneshots listed
under residuals hide nothing (corrected after round 2, F7). Why the
directory:

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
place (no unit reads it) and fails the verifier. (Superseded by round 2,
below: such a file is quarantined and the activation fails.) A link or file planted at
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
  docs/archive/runbooks/platform-dolt-authority-split-2026-09-18.md is updated.
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

## Round 2 review findings (bunch 85, on 368ffec70), before any fix

Reviews: Sonnet 5.5 accept with edits
(`_runs/bunch85/sonnet.out`); flash (omp-gemini38) accept; DeepSeek still
running. Sonnet reproduced F2 and F3 on temp trees with the evaluated
snippet. The others come from reading the code, not from observation.

- **F1 (medium): no unit hides the old paths now.** Several things can
  put a regular file at an old path:
  - an operator's `sed -i`, `install` or editor, which replaces the link by
    rename;
  - a restored backup from before the change;
  - a reverted deploy program's scrub.

  Every root unit can read that file (a 0600 root file is readable by its
  owner), and no unit reads it as its environment. The snippet logs it and
  returns 0, and only the verifier flags it, after units have started. For
  that path this is a confidentiality regression against the base, which
  hid the file when it existed at a unit's start.
- **F2 (medium): the snippet returns 0 while a unit loses its env file.**
  The state is a regular file at an old path, no file of that name in
  unit-env, and the marker present. This lane's own rollback text produced
  it: it moved the files back and left `.imported`, so a later landing
  imports nothing. For corpus-dsn.env, corpusd and checkpointd then start
  without CORPUSD_CORPUS_DOLT_DSN and put corpus tables on Store A
  (internal/platform/config.go:68; nix/node-b.nix's comment on that
  EnvironmentFile). The snippet also returns 0 when both paths hold the
  name, leaving it unclear which copy wins.
- **F3 (low-medium): a link at an old path to a file elsewhere is repointed
  without importing its target.** The operator's file stays where it is,
  unread, and unit-env lacks the name.
- **F4 (low): the old name is briefly absent.** Between a file's move and
  its link, the old name does not exist. A unit still on its old definition
  that restarts in that window starts without the file. An abort in the
  middle of the loop leaves the old names absent until the next successful
  run.
- **F5 (low): units keep their old view until they restart in the landing
  deploy.** In that view unit-env is not masked. This covers:
  - auth, maild, corpusd and the proxy, which the deploy program restarts
    after the switch;
  - the gateway, which restarts last, after a drain of up to 11 minutes;
  - vmctl;
  - every Firecracker VMM started before vmctl's restart, until its guest
    next boots.

  The three moved files stay masked in those views, because a rename moves
  the dentry that carries the mask. Anything new in unit-env during that
  window, a kill switch for example, is visible to them. The verifier sees
  these units but does not check VMMs.
- **F6 (low): the classifier.** Because `scripts/off-host-backup.sh` changed,
  this landing also rebuilds the guest image, deploys the frontend and
  refreshes every active computer. That refresh also relaunches VMMs under
  vmctl's new namespace. This is an owner decision; FP0 takes it to the
  owner, and the classifier is not changed here.
- **F7 (low): "every unit hides the directory" overclaims.** Thirteen root
  oneshots hide nothing: btrfs-headroom, both dolt-users units, digest,
  disk-gc, nix-gc, the three offhost-backup units,
  platform-dolt-history-audit, platform-dolt-squash and store-compaction.
  They can see and write unit-env.
- **F8 (low): the verifier now requires unit-env/deploy.env.** The base never
  mode-checked deploy.env. A host without the file (a fresh rebuild, a
  restore without it) fails the modes section. Node B has the file.
- **F9 (info): the kill switch through the link has two traps.**
  - `echo X=off > /var/lib/go-choir/corpusd-retrieval.env` through the
    dangling link creates the file with the shell's umask (0644). The
    verifier's modes check fails until the next activation sets 0600; use
    `install -m 0600` into unit-env instead.
  - Two older records still name the old path for the opinion and retrieval
    switch: corpus-retrieval-windowed-unindexed-untimed-2026-10-14.md:342
    and oped-not-connected-2026-10-20.md:157. There, `rm` leaves the switch
    on.
- **F10 (info): what the design assumes.**
  - The first run trusts whatever is at the old paths, and an existing
    unit-env with a marker. A unit that can write `/var/lib/go-choir`
    before the landing could pre-plant either. This is no worse than the
    base, but it should be checked before the deploy.
  - systemd follows a link named in InaccessiblePaths= and EnvironmentFile=.
    That comes from the source, not from observation, and matters only on
    the revert path.
  - CI does not run the nix test.

Dispositions planned on this lane, tests first:

- F1: quarantine. A regular file found at an old path after the import is
  moved into unit-env as `<name>.stray.<epoch>` (0600). The old path
  becomes the link, and the activation fails non-zero with a journal line
  naming the path.
- F2: the same failure applies to a file at an old path with no file of
  that name in unit-env. The failure is sticky: every activation fails
  while a stray exists, so the deploy's switch retry does not pass it
  silently.
  - A file identical to the one in unit-env is only relinked, with no
    failure; that is what a reverted deploy's scrub leaves.
  - Before the first import, the old path wins and an unexpected file
    already in unit-env becomes the stray.
  - The rollback text is corrected (below).
- F3: before the first import, a link at an old path to a regular file
  elsewhere is imported by copy. After it, the target's content becomes a
  stray and the activation fails; a dangling link is only repointed.
- F4: each file is moved and linked in adjacent steps rather than after
  all four moves, and a link is replaced through a temporary name and a
  rename. A hard link would keep the old name present throughout, but it
  would detach the old-view masks during the landing (F5). Moving the
  dentry keeps them, so the window stays two system calls wide.
- F7: the comments are corrected (this commit corrects the record).
- F5, F8, F9 and F10 are recorded. FP0's proof gains a check before the
  deploy (F10) and runs the verifier after the gateway's restart (F5).
- F6 stays with FP0 and the owner.

Between activations, F1 stays open in one respect: a regular file that
appears at an old path between two activations is readable by every root
unit until the next activation (boot or switch) quarantines it. The
verifier's legacy-path check flags it on any run. A systemd path unit
watching `/var/lib/go-choir` could close that window; it is a residual,
not built here.

### Rollback (corrected)

Revert the fix and redeploy. Move no files: the links keep the old paths
working for the reverted units, and a later landing then relinks without
importing anything. If the files must go back to their old paths for good,
run `mv -f /var/lib/go-choir/unit-env/<name> /var/lib/go-choir/<name>` for
each name present. Also run `rm /var/lib/go-choir/unit-env/.imported`, so
that the next landing imports them again as on its first run. The red
commit's message (844e0026b) gives the older text, which left the marker
behind; this text supersedes it.

### Round 2 dispositions (lane fp3-containment-gaps)

Findings ac8dd5caa (green), tests 202a5996e (yellow, failing on the round
1 snippet: 6 failures and 1 error of 22 in the nix test, 2 errors of 14 in
the verifier test), fix c756a14eb (red), proof steps in the station file
(this commit).

- F1 and F2, fixed. After the import, a regular file at an old path (or a
  link to one elsewhere) is never adopted. It is moved, or its target
  copied, into unit-env as `<name>.stray.<epoch>.<pid>` (0600), and the
  old path is relinked. Each case logs a line to stderr and to the
  journal (`journalctl -t go-choir-unit-env`) naming the path. The
  activation fails while any stray waits, so the deploy's one switch retry
  fails too. The deploy stops at the switch, before it restarts the proxy,
  corpusd and the other units marked restartIfChanged = false.
  - A file with the same content as `unit-env/<name>` (a reverted deploy's
    scrub) is only relinked.
  - Before the import, the old path wins over an unexpected file in
    unit-env, which becomes the stray.
  - The verifier fails while a stray waits ("no quarantined file waits in
    /var/lib/go-choir/unit-env").
  - Rollback text corrected (above).
  - What the switch already did is not undone. A unit that the switch
    restarted, or a timer-started oneshot such as checkpointd, runs
    without a file whose name is missing from unit-env until the operator
    resolves the stray. For corpus-dsn.env that is the Store A fallback,
    reachable only by moving a file back and leaving the marker (the old
    rollback text).
- F3, fixed. On the first run, a link's target is imported by copy and
  named; after the import, it becomes a stray.
- F4, fixed as far as it goes without losing F5's masks. Links are
  replaced through a temporary name and a rename, and a link to a
  directory is removed first. A first import's move and link are
  adjacent: two system calls.
- F5, recorded. Proof step 2 runs after the gateway's restart; VMMs are
  not checked, and the landing's refresh relaunches them.
- F6: FP0 and the owner; the classifier is unchanged.
- F7, fixed in the comments and in this record.
- F8, recorded, unchanged. Node B has deploy.env; a fresh host would fail
  the modes section until it has one.
- F9, recorded. The station file's operator note says to use
  `install -m 0600` into unit-env. The two older records are left as
  history; this record and node-b.nix carry the current paths.
- F10, recorded. Proof step 0 checks before the deploy that unit-env does
  not exist and what is at the old paths. The other assumptions stand as
  stated.

## Round 2b findings (DeepSeek, bunch 85, on 368ffec70), before any fix

DeepSeek v4.1 flash: ACCEPT (conditional), `_runs/bunch85/out/omp-deepseek-v41-flash.out`.
Its third finding is F2 (fixed in c756a14eb) and its fourth is F6 (the
classifier, with FP0 and the owner). The rest of its findings round 2 does
not cover:

- **DS1 (medium): the directory is hidden fail-open.**
  `hostSecretPaths.unitEnv` is `"-/var/lib/go-choir/unit-env"`, and the
  "-" makes systemd skip the mask when the directory is missing.
  switch-to-configuration runs the activation, records a failure and still
  starts and restarts units. So if the snippet's `mkdir` or `chmod` fails
  (ENOSPC, a read-only filesystem), every unit runs with the real
  directory unmasked as soon as it appears. Only the verifier would notice,
  after the fact.
- **DS2 (medium): the first import is not failure-atomic.** Each file
  moves separately under `set -e`, and the marker is written after all
  four. An ENOSPC or IO failure after some moves aborts the activation, but
  units still start, and their EnvironmentFile paths point into a
  directory that lacks the unmoved names. The proxy loses its overrides;
  corpusd and checkpointd fall back to Store A for corpus tables. The
  deploy retries the switch once and its activation may finish the import,
  but units started in between keep the partial view: checkpointd on its
  minute timer, and any unit that restarts.
- **DS5 (low): the verifier requires deploy.env, but the proxy does not.**
  The verifier newly requires `unit-env/deploy.env`, while the proxy's
  EnvironmentFile is optional ("-"). The base verifier never checked
  deploy.env (this is F8).
- **DS6 (low): the corrected DSN runbook still creates the file 0644.** It
  writes corpus-dsn.env with `cat >`, which creates the file at the
  operator's umask (0644) inside the 0700 directory. The modes check fails
  until the next activation sets 0600.
- **DS7 (low): one test does not test what it says.** `test_N1_keeps_are_real`
  says it rejects a keep that the unit hides anyway, but never checks the
  unit's hidden list. The gateway's keep of the JWT key is one such
  keep: the gateway hides the whole auth-signing directory. The test file
  also has no case for both paths holding the same content before the
  import, and none for a partial import.

Planned dispositions, tests first:

- DS1: the entry loses its "-", so a unit whose directory is missing
  refuses to start. The directory exists before any unit starts: the
  activation creates it at boot before systemd starts units and in a
  switch before units restart, and tmpfiles creates it as well.
- DS2: the partial state heals itself and is loud and safe. The marker is
  still written last, so the next activation (a boot, or the deploy's
  switch retry) continues the import of what is left. The three units that
  read an env file from the directory (go-choir-proxy, go-choir-corpusd,
  go-choir-checkpointd) get `AssertPathExists=` on the marker, so they
  refuse to start, loudly, until the import is complete.

  I did not take the staged, atomic directory swap. It needs either a
  hard link per file, which detaches the old-view masks during the landing
  (F5), or a directory exchange (`mv --exchange`). The assert keeps the
  per-file rename that carries the masks, and it guarantees that no reader
  starts on a partial import.
- DS5: deploy.env becomes optional in the verifier, matching the unit.
  corpus-dsn.env and sandbox-gateway-token.env stay required, as in the
  base verifier; Node B has both, and a missing corpus-dsn.env is a real
  regression there. The marker is added as required.
- DS6: the runbook writes the file with `install -m 0600 /dev/stdin`.
- DS7:
  - The test asserts that no kept path is hidden after all, whether by
    the exact entry or by a hidden parent directory. A path counts as
    hidden when a parent directory is, so the gateway's JWT-key keep
    goes.
  - New cases: both paths holding the same content before the import,
    and a failure after the second move (injected with a `mv` shim on the
    snippet's PATH).

### Round 2b dispositions (lane fp3-containment-gaps)

Findings 8b6b45c02 (green); tests be8a09abb (yellow). On the round 2 code
the nix test had 3 failures of 26 and the verifier test 1 of 14. The
partial-import and both-alike cases already passed there, because the
snippet heals and relinks; DS2's safety comes from the assert. Fix
580c98f28 (red); this commit carries the docs.

- **DS1, fixed.** The directory is hidden without "-", so a unit whose
  directory is missing refuses to start (226/NAMESPACE). The activation (at
  boot and in a switch, before units start or restart) and tmpfiles both
  make it. An activation that cannot make it takes the host's units down,
  loudly. The rendered proxy unit has
  `InaccessiblePaths=/var/lib/go-choir/unit-env`.
- **DS2, fixed.** It heals itself and is loud and safe; it is not atomic.
  - proxy, corpusd and checkpointd assert `unit-env/.imported`, so they
    do not start while the first import is incomplete. The nix test pins
    them as the only readers.
  - The next activation finishes the import. The temp-tree test injects
    a mv failure on the third file: the first two are in unit-env, the
    third is still at its old path, there is no marker, and the run exits
    non-zero; the next run completes it.
  - An atomic swap was not taken: hard links would detach the masks units
    still on their old view rely on during the landing (F5), and an
    exchange needs `mv --exchange`.
  - Never remove the marker except as the rollback text says.
- **DS5, fixed.** This also changes F8. The verifier checks deploy.env as
  optional, matching the proxy's EnvironmentFile, and requires the marker.
  corpus-dsn.env and sandbox-gateway-token.env stay required, as in the
  base verifier.
- **DS6, fixed.** The runbook writes corpus-dsn.env with
  `install -m 0600 -o root -g root /dev/stdin`.
- **DS7, fixed.** A path counts as hidden when a hidden parent directory
  covers it. A keep the unit hides anyway fails the test, so the gateway's
  JWT-key keep is gone. Both paths with the same content before the import
  relink without a stray. The partial import has its test (DS2).
- DS3 is F2 (fixed in round 2). DS4 is F6, which stays with FP0 and the
  owner.
- FP0's proof gains two checks: that the directory entry carries no "-",
  and the asserts. It also says what the two intended loud states mean and
  how to clear them.

## Round 2c findings (DeepSeek, bunch 98, on 8b14bc9cc), before any fix

DeepSeek v4.1 flash: ACCEPT (conditional), `_runs/bunch98/out/omp-deepseek-v41-flash.out`.
DS1 is closed. DS2 is not, as written:

- **DS8 (medium, confirmed): the marker is written by a failing
  activation, and it certifies no file.** The snippet writes
  `unit-env/.imported` whenever the run started without it, before the
  `fail != 0` exit. The "neither a file nor a link" branch sets `fail=1`
  and leaves `unit-env/<name>` missing. Reproduced on a temp tree with the
  evaluated snippet:
  - first run with a directory at the old path of corpus-dsn.env: exit 1,
    the FAIL line, the marker present, `unit-env/corpus-dsn.env` absent;
  - the recovery the station file gives (remove the obstruction, run
    `/run/current-system/activate`): exit 0, nothing to import, the marker
    still there. corpusd and checkpointd pass `AssertPathExists`, and their
    corpus-dsn.env EnvironmentFile is optional ("-"), so they start on
    Store A for corpus tables: F2's harm, which DS2 claimed to close.
  - A required file removed from unit-env after a good import gives the
    same state with exit 0.
  switch-to-configuration records an activation failure and still restarts
  units, so the assert is the only thing that keeps the readers down.
- **DS9 (low): `chmod` on a stray can follow a link.** A stray is a link
  when `mv "$new" "$stray"` moved a link planted in unit-env; `chmod 0600`
  then changes its target.
- **DS10 (low): the marker can be a link.** `[ -e ]` is false for a
  dangling link, and `: >` then writes through it. The marker is never
  chmod'ed and has no tmpfiles `z` line.
- **DS11 (low): the nix test does not run in CI.** It is the only pin for
  the directory entry without "-" and the asserts. This is F10; not
  introduced here.
- **DS12 (info): overclaims.** node-b.nix's comment on `unitEnvImported`
  ("once the first import has moved every file in") and this record's
  "refuse to start ... until the import is complete" are false in DS8's
  states.

What else the node needs to know: the authority-split runbook's rollback
removes `unit-env/corpus-dsn.env` on purpose to send corpusd back to Store
A, and corpusd treats an empty `CORPUSD_CORPUS_DOLT_DSN` as Store A
(internal/platform/config.go). An assert on the file's existence must keep
that rollback possible.

Planned dispositions, tests first:

- DS8: the marker is written only after the `fail != 0` exit, so a failing
  activation never writes it. corpusd and checkpointd also assert
  `unit-env/corpus-dsn.env`, so a missing DSN file keeps them down, loudly,
  whatever the marker says. The runbook's rollback writes a file without a
  DSN (an explicit Store A) instead of removing it. The marker is not gated
  on the files' presence as well: the proxy asserts the marker and reads no
  required file, so that gate would take the site down for a corpus DSN gap
  that the per-file assert already covers, and sandbox-gateway-token.env
  has no unit reader.
- DS9: a link stray is not chmod'ed.
- DS10: the marker is written as a fresh regular file (any link there
  removed first), 0600, and tmpfiles carries its `z` line.
- DS11: recorded, with F10.
- DS12: the comments and this record are corrected with the fix.

### Round 2c dispositions (lane fp3-containment-gaps)

Findings cd2084a75 (green); tests 952be102f (yellow), 6 of 31 failing on
8b14bc9cc. Fix 3369f64b5 (red); this commit carries the docs. Local: nix
test 31/31, verifier tests 14/14, `nix eval` of the go-choir-b toplevel
drvPath passes, and the rendered corpusd unit carries both asserts.

- **DS8, fixed.**
  - The marker is written after the `fail != 0` exit, so only a first run
    that passed writes it: no obstruction, no stray, every move done. The
    obstruction-then-recovery case has its test. After the obstruction is
    gone, the rerun is still a first import and adopts a file put back at
    the old path.
  - corpusd and checkpointd assert `unit-env/corpus-dsn.env` as well as
    the marker. A required file removed after a good import keeps them
    down, loudly. The "-" stays on the EnvironmentFile, so the assert is
    what refuses, with its own message.
  - The authority-split runbook's rollback writes a file that sets no DSN
    (an empty `CORPUSD_CORPUS_DOLT_DSN` is Store A) instead of removing
    it. Its preconditions say so.
  - Declined in part: the marker is not also gated on the required files'
    presence. The proxy asserts the marker and reads only the optional
    deploy.env. That gate would take the site down for a missing corpus
    DSN, which the per-file assert already covers on the units that read
    it. sandbox-gateway-token.env has no unit reader; the verifier still
    requires it.
  - New consequence: a stray found during the first import (a file in
    unit-env before it existed on Node B) now keeps the marker absent, so
    proxy, corpusd and checkpointd stay down until the operator resolves
    it. Proof step 0 checks that unit-env does not exist before the
    landing, so on Node B this needs tampering before the deploy.
- **DS9, fixed.** A stray that is a link is chown'ed with `-h` and not
  chmod'ed.
- **DS10, fixed.** Whatever sits at the marker's path is removed first,
  and the marker is written as a regular file and set to 0600. tmpfiles
  carries `z unit-env/.imported 0600`.
- **DS11, recorded** with F10: the nix test still runs only by hand
  (`python3 -I scripts/test_host_containment_nix.py`, needs nix).
- **DS12, fixed.** The comments on `unitEnvImported` and on the snippet
  now say what the marker certifies (a passed first run, not any file).
  The round 2b text above ("refuse to start ... until the import is
  complete") is kept as history; this section supersedes it.
