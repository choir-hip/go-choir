# A deploy was marked incomplete after one health read timed out, and the paper took two planned restarts in eleven minutes (2026-11-22)

Status: open. Nothing was changed by hand. No rollback ran.

## Problem

CI run 45043963968 (a docs push, `22192b13b`, the first run to pass the disk
preflight since 10:17Z) installed the release and then failed its deploy job.
The platform serves `22192b13b` and the paper runs it, but the deploy's
evidence file says `incomplete`, so there is no clean receipt for the release
that carries F0 (`34bc283ee`) and the spend test fix (`aefdfbbc4`).

## Evidence

- Deploy job log, 16:58:16Z to 16:58:21Z: the health reads of proxy, gateway,
  corpusd and two computers answer; one read ends "curl: (28) Connection
  timed out after 5001 milliseconds"; then "Recorded incomplete deployment
  evidence at /var/lib/go-choir/deploy-failures/45043963968-1.json" and exit 1.
- That file: `status: incomplete`, `observed_artifacts.autoputer: installed`
  at the target commit, a rollback path named and not taken.
- Node B journal: "vmctl: deploy refresh caller=retry-sweep … vm-1669020…" at
  16:57:35Z, "refreshed VM vm-1669020…" at 16:58:36Z, and "deploy refresh
  caller=deploy" for the same VM at 16:58:36Z.
- The paper's console: "host requested a planned restart (platform update
  22192b13b…; allow_busy=true, interrupting 11 runs)" at 16:47:25Z, and again
  at 16:58:46Z "interrupting 0 runs". Both boots say "restart kind=planned".
- The first boot did what the day needed: "compact rebase: rebased onto the
  compact copy at sequence 13617852: 55367938048 -> 51220189184 bytes, 44520
  events to replay". Node B went from 65 GiB free at 16:45Z to 88 GiB at
  16:58Z.

## Belief

The health read that timed out was of a computer being refreshed at that
second, most likely the paper during its second restart. Two callers
(the deploy and the retry sweep) each refreshed the paper for the same
target. Neither is confirmed from code: which computer the timed-out read
addressed is not in the log line, and why a computer already on the target
was refreshed again was not read.

## Remaining error

Whether the second restart interrupted work that the first had resumed;
whether the deploy's health step should wait out a refresh it started; why
the compact rebase freed 23 GiB on the host and not the 48 GiB of the staged
copy (the guest's trim may not have run yet). The next landing's deploy is
the next observation.
