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
