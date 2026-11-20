# The gateway hands its port to the new process before it drains (2026-11-20)

Class of this file: green (a design; nothing changes until the landings
below). Problem: [the gateway refuses every guest while it drains on a
deploy](../problems/gateway-refuses-guests-while-it-drains-on-deploy-2026-11-19.md).
Code read at `origin/main` 7f5ad41e1. Every claim about code cites the file
and line it was read at. A number marked *(not measured)* is not a fact.

## 1. The decision in five lines

1. Two instances of one gateway unit, `go-choir-gateway@a` and `@b`, both
   bind port 8084 with `SO_REUSEPORT`. The deploy starts the idle one on the
   new binary, waits for it to say it is ready, then stops the old one
   without waiting.
2. The old one closes its listener and drains its in-flight calls (up to
   10 min 30 s, as now). New connections reach the new one from the first
   moment. A guest never sees `connection refused` on a deploy.
3. The kernel moves connections still queued on the closing listener to the
   live one (`net.ipv4.tcp_migrate_req = 1`), so none is reset.
4. Because two processes run at once for up to eleven minutes, the four
   files the gateway rewrites (two spend files, the identity store, the
   ChatGPT auth file) become safe for two writers first.
5. Five landings, all red but the last: the spend files, the identity store
   and auth refresh, the listener, the units and the deploy, then the
   removal of the old unit.

## 2. What the thing is

The gateway is one long-lived HTTP server on port 8084 that every guest,
the paper included, uses for model calls, judgments and searches. It is a
host service with no obligations of its own: it finishes requests and
keeps a few files.

### What the code does today

- The server binds its port in `Start`: `net.Listen("tcp", s.addr)` unless a
  listener source is set (`internal/server/server.go:328-332`). On SIGTERM it
  calls `http.Server.Shutdown` with `SERVER_SHUTDOWN_TIMEOUT`
  (`server.go:353-364`). `Shutdown` closes the listener first and then waits
  for active requests. So the port is closed from the first second of the
  drain.
- The unit gives it 10 min 30 s to drain and 11 min to stop
  (`nix/node-b.nix:2502`, `:2581`). It is one unit, `go-choir-gateway`
  (`nix/node-b.nix:2485`), so systemd starts the new process only after the
  old one exits.
- The deploy restarts it last and alone with `systemctl restart
  go-choir-gateway` (`.github/scripts/node-b-ci-deploy:469-479`), and the
  pointer-only path restarts it in a loop with the others (`:449-451`).
  `nix/deploy-provider-creds.sh:85` and `:219` restart it the same way after
  a credential change.
- Two hooks exist and are unused by the gateway: `SetListenerSource`
  (`server.go:225`), which lets `Start` take a listener made elsewhere (only
  the autoputer's store gate uses it, `internal/autoputer/store_split_gate.go:224`),
  and `SetOnListen` (`server.go:219`), which runs right after the bind. These
  are the replacement's two attachment points; nothing new is needed in
  `internal/server`.
- No unit in `nix/` uses systemd socket activation, and nothing in the
  repo reads `LISTEN_FDS` or `NOTIFY_SOCKET` (grep of `*.go` and `*.nix`).
- The runtime directory `/run/go-choir-gateway` holds the Claude delegate
  token copy and is removed when the unit stops
  (`nix/node-b.nix:2506-2508`, path at `:575`).

### Files the gateway rewrites, and what two writers would do to them

| File | How it is written | Two processes at once |
|---|---|---|
| `paid-spend.json` | the whole day's totals from memory, by rename of a fixed `.tmp` (`internal/gateway/paid_budget.go:221-235`) | each overwrites the other's calls: the owner's spending cap under-counts; the shared `.tmp` name can race |
| `anthropic-relay-spend.json` | the same pattern (`internal/gateway/anthropic_relay.go:296-306`), loaded once at start (`:279`) | the same: the per-computer daily token cap under-counts |
| `gateway-identities.json` | the whole registry from memory, loaded once (`internal/gateway/config.go:247-290`); written on issue, ensure, peer pin and revoke (`:325`, `:373-395`, `:551`, `:652`) | the old process's last write erases credentials the new one issued since its start: a computer's token stops working at the next restart |
| `codex-auth.json` | read from disk on every call (`internal/provider/provider.go:3575`), refreshed under an in-process mutex and written by rename (`:3738-3768`) | safe to read; two processes refreshing at once can spend one refresh token twice |

## 3. Physical bounds

| Quantity | Value | Source |
|---|---|---|
| Gap per deploy, 2026-11-19 | 4 s to 9 min, about 25 min in six deploys | record |
| Gap per deploy, 2026-11-20 | 1 min 47 s, 44 s, 1 min 51 s, 2 min 1 s (journal "received terminated" to "server stopped", 03:16Z to 06:17Z) | read on Node B for this design |
| Drain allowed | 10 min 30 s, stop killed at 11 min | `nix/node-b.nix:2581`, `:2502` |
| Gateway memory now | 39 MiB current, 44 MiB peak | `systemctl show`, Node B, 06:2xZ |
| Host memory | 31 GiB, 22 GiB available | `free -g`, Node B |
| Listen backlog | 4096 | `ss -ltn` on 8084 |
| Kernel | 6.18.57; `tcp_migrate_req` is 0 today (needs 5.14 or later) | read on Node B |

So the overlap costs about 40 MiB of memory for at most eleven minutes.
What must be consistent across the two processes: the spend totals (an
owner-set cap reads them), the identity registry (every guest's bearer
check reads it) and the auth file. Nothing else the gateway holds outlives
a request.

## 4. Options

| Option | What a guest sees during the drain | Cost | Verdict |
|---|---|---|---|
| A. Today: one unit, restart | `connection refused` for the whole drain | none | the defect |
| B. One unit with a systemd socket (`.socket`, `Accept=no`) | the connection is accepted by the kernel but not served until the old process exits: the same minutes, as hangs | a socket unit | rejected: restart is still stop-then-start, so the gap stays, only hidden |
| C. Self-exec with fd passing (tableflip style), `ExecReload` | nothing | a new dependency; the new process inherits the old one's environment, so a change to the unit's `Environment=` (frequent: computer lists, relay grants) would need a full restart and bring the gap back; `/proc/self/exe` names the old binary, not the pointer | rejected |
| D. Two instances behind `SO_REUSEPORT`, deploy starts new then stops old without waiting | nothing | template unit, a ready signal, a sysctl, a deploy step, two-writer-safe files | **chosen** |
| E. Two instances sharing one systemd socket (`Sockets=`) | nothing | the socket must trigger a service when it is readable, so traffic while the live instance is the other one starts the wrong instance; needs a dummy service to hold the trigger | rejected: D does the same with less |
| F. Guests retry refused dials | waits of minutes, then success | caller code everywhere | rejected as the fix; the event layer's unretried captures (`internal/agentcore/event_layer.go:374`, per the record) are their own defect |

D is chosen because each start of an instance reads the current unit file
(so environment changes ship without a gap), a failed new binary never
takes the port away from the old one, and the two attachment points it
needs already exist in `internal/server`.

### What in-flight guest requests see

- A request already being served by the old process runs to its end, up to
  10 min 30 s, exactly as today.
- A new connection reaches the new process. During the moment both are
  listening, the kernel spreads new connections over both; both serve.
- A connection that finished its handshake but was not yet accepted by the
  old process moves to the new one when the old listener closes
  (`tcp_migrate_req`). Without the sysctl it would be reset.
- An idle keep-alive connection to the old process is closed by `Shutdown`.
  A POST that races that close can fail with EOF. This race exists today
  too; it is not widened.

## 5. Landings, in order

The order puts the two-writer safety before any overlap can happen.

### G1. The spend files are safe for two writers (red; delivered, 418a9bf0c and its follow-up)

What was built (`internal/gateway/paid_budget.go`, `anthropic_relay.go`,
`spend_file_lock.go`, `cmd/gateway/main.go`):

- Each ledger (the paid ledger and the relay's per-computer day spend) keeps
  an unsaved delta by UTC day next to its in-memory record. `Record` and
  `charge` add to both under the ledger's mutex and kick a flusher
  goroutine; a guest's call never waits for the file lock or the disk.
- The flusher (`spendSaver`) saves: it takes an exclusive `flock` on
  `<file>.lock` (5 s deadline, then the delta is kept and the save retried
  each second), reads the file, copies the delta under the mutex, adds the
  copy to the day each call was made on (a finished day's calls land in
  `prev`; a day the file has not reached rolls it first), writes a uniquely
  named, fsynced temp file and renames it, then takes the merged record into
  memory under the mutex, subtracting the saved copy so calls made during
  the save are neither lost nor doubled. The file format is unchanged.
- A file that is absent is empty (memory is written whole). A file that is
  present but cannot be read is never moved or written over; the save errors
  and keeps the delta. A file that is read but corrupt (truncated, empty,
  wrong type, no day) is set aside under the lock to
  `<file>.bad-<UTC time>` (mode 0600, bytes kept, never deleted by code),
  at load and at save, logged loudly and counted in the status; the ledger
  continues from its own memory and writes a fresh file. Two processes that
  see the same bad file: the lock makes one mover; the second finds the fresh
  file and merges into it. The loss is one visible event (the spend only the
  old file held), not every restart.
- The day rolls forward only (`day > recorded day`); a file or clock behind
  never moves today's spend into `prev`. The relay keeps its own memory when
  its clock is behind the file and counts the charges it drops (a finished
  day: nothing reads one) in the status and the log.
- `Refusal` and the relay's cap read memory, which holds this process's own
  spend plus what the last merge brought in. A process therefore sees the
  other's spend when its own next save lands (right after each of its own
  calls). Worst case over an owner-set cap during the overlap: the calls the
  draining process finishes after the new one's last save; for the relay up
  to 8 admitted calls per computer instead of 4. Accounting is exact; only
  admission overlaps.
- `GET /provider/v1/spend` carries `paid_spend.persistence` and
  `relay_persistence` (unsaved calls, last error and time, last save, error
  lines logged, bad files moved and the last name, dropped charges), and the
  hourly summary line adds the same when something is wrong.
- `cmd/gateway` flushes both files in a shutdown hook after the server has
  drained. A crash loses the calls not yet saved: those made during one
  healthy save (milliseconds); while saves fail, every call since the last
  good save (the status counts them). `writeSpendFile` fsyncs the file, not
  the directory, so a power loss can revert the last rename.
- Rules for operators: do not delete `<file>.lock` while the gateway runs
  (exclusion depends on its inode); do not restore a spend file from a
  backup while the gateway runs (the process keeps its memory and would not
  rewrite it); to repair, write at least the totals `persistence` and the
  hourly line show plus what the old file held.

Failure list the tests answer (`internal/gateway/spend_two_writers_test.go`):
two ledgers on one file sum their calls, per computer for the relay; the
other process's day turn; a killed writer's temp and lock files; a late
starter loads the merged total; `Refusal` and the relay cap see the merged
total; a held lock keeps the delta and loses nothing; a bad file of each kind
is moved aside at load and at save, once for two processes, and a restart
moves nothing; an unreadable file is not moved; the day never rolls backwards
(`b.Snapshot()` and `b.Refusal()` after the other writer turned the day);
calls during a save are not lost or doubled; goroutines writing two ledgers
at once keep every call; the lock wait is never on the call path; an
unopenable lock file is surfaced; dropped relay charges are counted.

Protected surfaces: the owner's spending caps (paid budget, relay daily
tokens), gateway and provider calls.

Deployed proof on d@e: there is no overlap before G4, so G1's deployed
proof is that nothing regressed: read `paid_spend.providers.<p>.calls` and
`paid_spend.persistence` from `GET /provider/v1/spend`, make one priced d@e
call, and within a minute `calls` is one higher, `unsaved_calls` is 0,
`last_error` is absent and the journal has no `spend file not saved` or
`SPEND FILE CORRUPT` line. The two-writer property is proven in G4's drill.

Rollback: revert G1. The file format is unchanged; a leftover `.lock` or
`.bad-*` file is harmless.

A crash in the middle: `flock` is released by the kernel when the process
dies; the rename is atomic, so the file is the old total or the new one.

Claim for the ledger: "In G4's first deploy with a call draining on the old
instance, the paid spend file's day `calls` after the drain equals the
count of priced `inference succeeded` lines in both instances' journals
since 00:00Z." Probability 0.8. Settle at that deploy.

### G2. The identity store and the auth refresh are safe for two writers (red)

Scope: `internal/gateway/config.go`, `persistLocked` (`:270`): under an
exclusive `flock` on `gateway-identities.json.lock`, read the file, apply
only the entries this call changed (upsert for issue, ensure and peer pin;
delete for revoke), write by unique temp name and rename, and replace the
in-memory map with the merged one. `internal/provider/provider.go`,
`AccessToken` and `ForceRefresh` (`:3571`, `:3589`): take an exclusive
`flock` on `codex-auth.json.lock` around read, refresh and write, and read
the file again after the lock, so a second process uses the first one's
fresh token instead of refreshing again.

Protected surfaces: gateway bearer credentials (auth), the ChatGPT
provider's OAuth refresh token.

Failure list: two registries on one file issue credentials in turn and
both survive; a revoke in one is not undone by a later save in the other;
a peer pin in one survives the other's save; two processes asking for a
token past its expiry refresh it once; a reader never sees a half file.

Deployed proof on d@e: after the deploy, d@e's next model call is answered
(its console shows no `401` from the gateway), and after a deploy-time
refresh of d@e its bearer is still accepted (vmctl's ensure path).

Rollback: revert G2.

A crash in the middle: as G1; the lock dies with the process and the file
is whole.

Claim: "In the 14 days after G4 lands, no computer on Node B logs a gateway
bearer refusal for a token issued by the gateway since its last start."
Probability 0.85. Settle by the gateway journals' refusal lines.

### G3. The listener shares its port and says when it is ready (red)

Scope: `cmd/gateway/main.go` (server made at `:19`, started at `:134`):
`SetListenerSource` with a `net.ListenConfig` whose `Control` sets
`SO_REUSEPORT` on the socket, and `SetOnListen` that sends `READY=1` to
`$NOTIFY_SOCKET` (a datagram; about fifteen lines, no dependency) and logs
`gateway: listening on <addr> (shared port), ready`. `nix/node-b.nix`: the
gateway unit takes `Type = "notify"`; `boot.kernel.sysctl."net.ipv4.tcp_migrate_req" = 1`.
The deploy is unchanged, so this deploy still has the gap one last time.

Protected surfaces: the gateway's listener (every guest's provider calls),
deploy readiness.

Failure list: two gateway processes in a test both bind one port; a
connection queued on a listener that closes is served by the other (on a
kernel with the sysctl); `READY=1` is sent only after the bind; with no
`NOTIFY_SOCKET` the server starts as before; the unit reaches active only
after the ready signal.

Deployed proof: on Node B, `sysctl net.ipv4.tcp_migrate_req` reads 1,
`systemctl show -p Type go-choir-gateway` reads `notify`, the journal has the
`(shared port), ready` line, and d@e's next model call is answered.

Rollback: revert G3 (the unit returns to `simple`; the sysctl returns to 0).

A crash in the middle: as today; a crash restart of the one unit leaves the
3 s `RestartSec` gap it has now.

Claim: none; G3 alone changes nothing a guest sees. Its effect is measured
in G4.

### G4. Two instances and a handoff in the deploy (red)

Scope:

- `nix/node-b.nix`: the unit becomes a template `go-choir-gateway@.service`
  with the same settings, `restartIfChanged = false`, `Type = "notify"`,
  `RuntimeDirectory = "go-choir-gateway-%i"`, and the token path in
  `GATEWAY_ANTHROPIC_OAUTH_TOKEN_FILE` and the `ExecStartPre` stage taking
  `%i`. Otherwise the instance stopping would delete the live one's token
  copy. Instance `a` is wanted by `multi-user.target`. The old
  `go-choir-gateway.service` stays defined for this one release, with no
  `wantedBy`, so a host boot does not start it.
- A handoff command, `go-choir-gateway-handoff`, built into the closure
  (`pkgs.writeShellApplication` in `nix/node-b.nix`), so the CI deploy and
  the credential script run the same thing on Node B:
  1. Find the live set among `go-choir-gateway.service`, `@a`, `@b`.
  2. None live: start `@a`; done.
  3. Pick `next` as the instance not live (`a` if only the old unit is
     live). If both instances are live (a deploy died midway), stop the one
     that started first with `--no-block`, and take the other as live.
  4. `systemctl start go-choir-gateway@next`. It returns when the instance
     has sent `READY=1`. If `next` is still draining from the last deploy,
     the start waits for that drain; the live instance serves meanwhile.
  5. If the start failed: stop nothing, exit non-zero. The old binary keeps
     the port and the deploy's rollback runs.
  6. `systemctl stop --no-block` every other live gateway unit. Exit 0.
- `.github/scripts/node-b-ci-deploy`: `:469` resets failed state on the
  instances; `:479` runs `go-choir-gateway-handoff` instead of `systemctl
  restart go-choir-gateway`; the pointer-only loop (`:449-451`) runs the
  handoff when the service is the gateway. The identity check (`:925-960`)
  is unchanged: once the old listener is closed, only the new instance
  answers `/health` on 8084, and the check already retries 15 times.
- `nix/deploy-provider-creds.sh:85`, `:219`: the handoff instead of the
  restart.
- The journal readers move from `-u go-choir-gateway` to
  `-u 'go-choir-gateway@*'`: `scripts/status-hourly:71`,
  `scripts/status-paper-usage:375-383`, `scripts/status-corpus-reuse:520`,
  `scripts/spend-report:47`, `scripts/proofs/jev-call-stats.sh:29`,
  `scripts/proofs/fp7-gateway.sh:110`, `scripts/proofs/lib.sh:40`,
  `scripts/paper-liveness-probe.py:95`, `scripts/host-fetch-proof.py:182`,
  `scripts/cpu_host_sample.py:23`. `scripts/verify-host-hardening.py:74-241`
  checks the instances. `scripts/land:350` evaluates the template's
  environment.
- `docs/current-architecture.md` gains the row.

The first deploy of G4 hands off from the old unit to `@a` with no gap, as
step 3 allows. NixOS's switch may already have started `@a` (it is newly
wanted); then the handoff finds `@a` and the old unit both live and stops
the old one.

Protected surfaces: deploy routing, the gateway's listener, every guest's
provider calls, the deploy's rollback.

Failure list:

1. Handoff with only the old unit live: `@a` starts, the old unit is
   stopped without waiting, the port never closes.
2. Handoff with `@a` live: `@b` starts, `@a` stops.
3. The new instance fails to start: the old one is not stopped and the
   handoff exits non-zero.
4. Both instances live: the older is stopped and the handoff proceeds.
5. `next` still draining: the start waits; the live instance keeps
   serving; no unit is left stopped.
6. Stopping one instance does not remove the other's runtime directory or
   token copy.
7. A host boot starts `@a` only.
8. The journal readers find lines from both instances.

Deployed proof on d@e, with observables the author cannot write:

- Before the deploy, start a d@e run that makes model calls through the
  deploy window.
- In the gateway journal: the new instance's `(shared port), ready` line
  at T1, the old instance's `received terminated` at T2 > T1 and its
  `server stopped` at T3. A real drain needs T3 - T2 of 30 s or more; if it
  is shorter, the proof waits for the next deploy.
- In d@e's console between T2 and T3: zero `connection refused` lines for
  the gateway's address, and at least one gateway call answered (the new
  instance's `inference succeeded for autoputer <d@e>` between T2 and T3).
- G1's claim: the paid spend file's `calls` against both journals.

Rollback: revert G4. The revert restores the single unit; its first deploy
has the gap once (stop the instances, start the unit).

A crash in the middle:

- The deploy dies after step 4: both instances serve the same port. That is
  harmless; the next handoff stops the older one (step 3).
- The host crashes during a drain: the draining calls are lost, as in any
  crash. Boot starts `@a` from the current pointer. The gateway resumes
  nothing; it has no obligations, and the guests' callers see a failed
  call as they would today.

Claim for the ledger: "At the first deploy after G4 in which the old
instance drains for 30 s or more, d@e's console shows zero `connection
refused` lines to the gateway between the old instance's `received
terminated` and `server stopped`, and the new instance answers at least
one call in that window." Probability 0.8. Settle at that deploy.

### G5. Remove the old unit (orange)

Scope: delete `systemd.services.go-choir-gateway` from `nix/node-b.nix`
and the old-unit branch of the handoff. By then the unit is stopped, so the
switch has nothing to stop.

Failure list: nix eval of both systems; the handoff with only instances.

Deployed proof: a deploy after G5 hands off between `@a` and `@b` with the
same journal order as G4's proof.

Rollback: revert G5.

Claim: none worth entering.

## 6. What is not measured

- What the other callers (research, the pilot's units, mail) did during
  the 2026-11-19 gaps, and whether they retried. Not needed for the design;
  the handoff removes the gap for all of them.
- Whether a drain is usually one long call or many. The 2026-11-20 drains
  (44 s to 2 min) suggest a few long calls *(not measured)*.
- The overlap's effect on owner-set per-computer bounds held in memory: the
  Anthropic relay's 4 calls in flight per computer
  (`GATEWAY_ANTHROPIC_RELAY_MAX_INFLIGHT`, `nix/node-b.nix`) could reach 4 on
  the old instance plus 4 on the new for the length of the drain. The old
  instance only finishes calls it had already admitted.

## 7. Questions only the owner can answer

- During a handoff, may two gateway processes hold calls at once for up to
  eleven minutes, so an in-memory per-computer bound (the relay's 4 in
  flight) can briefly be exceeded by calls the old process had already
  admitted? Default taken: yes. Spending caps stay exact through G1; only
  the in-flight count overlaps.
- The rollback of G4 brings the gap back for one deploy. Default taken:
  accept it, since it happens only on a revert.
