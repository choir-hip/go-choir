# Self-development drill on staging, end to end: PASS (2026-10-24)

Run by an FP0 worker, 03:02 to 03:39 UTC, on staging `0cbc52edc` (D32:
the jailer and host-only guest DNS for class:test). The question: does a
whole self-development cycle work on a fresh test computer today? A change
is made in a capsule, frozen, verified, approved, host-built, applied,
checkpointed and restored. The answer is yes, on the first run, with
19 of 19 checks passing. One round failed along the way, and the desk
recovered from it on its own (below).

## Command and identifiers

```
node scripts/reeng_tracer.mjs --scenario engineering --change go-served \
  --label fp0-selfdev-1024a --expect-sha 0cbc52edc --fence-min 60 \
  --timeout-min 60 --apply-min 30 --restore-min 12 --keep-computer
```

- Exit 0. `acceptance.pass = true`, `reached = restore`, `failed = []`.
  One named gap: no product route serves the projection head after a
  restore (a standing gap, by design).
- Computer: `computer-2ff1f63709dd385ebe92c100905e0e3b` (freshly minted,
  `warmness_class: test`). Trajectory `1f4638a0-9d32-580d-a7dd-20ff94e462cd`.
- Operation `selfdev-7925c7c1aaae1b3e671ef08ec03c7f23`. Frozen bundle
  `1c230ebf…`, verified bundle `dd1839d1…`, verifier ref `b66ab4cc…`.
- Pre-apply checkpoint `0a2eb4a2…` (effective head `d4324bad…`).
  Materialization receipt `7a0f9633…`, checkpoint ref
  `checkpoint:sha256:2ff6a5f3…`, route certificate `a505c006…`
  (generation 1).
- Receipt and bundle (outside the repo):
  `~/.config/choir-qa/traces/fp0-selfdev-1024a/receipt.json` and
  `bundle-a1.json` (sha256 `a1388a64…`).
- Identity fence: the tracer waited from 03:02:27 until staging served
  `0cbc52edc` with no CI busy (03:07:41). No contamination.

## Expected outcome, written before the run

The last full pass was go-served-1920 (2026-10-15, 18 of 18). The last
go-served run, s6-0-1817 (10-20), applied an empty patch (failure 15).
Since then: an empty patch refused at Freeze, carried patches (option A),
the D14 and D20 replay fixes. Modules have come from the host goproxy since
10-14. The new risk tonight was D32: the capsule's Go module fetch or the
gateway might be refused under host-only guest DNS. My prediction was a
mint in 1 to 3 minutes and a Freeze after one or two rounds in 15 to 40
minutes. If anything failed, I expected the Go build in the capsule to
fail first (on network, or on time); otherwise a pass.

What happened: a pass. The network was not a problem: modules downloaded
in the capsule in both rounds. Round 1 did fail, but because the desk gave
up early, not because of the network.

## Phases

| Phase | Result | Time (UTC) | Duration |
|---|---|---|---|
| Mint (fresh test computer) | PASS | 03:07:42 to 03:07:57 | 16.2 s |
| Mode propose_only | PASS | 03:07:57 | <1 s |
| Probe absent before apply (404) | PASS | 03:07:58 | |
| Texture casts to management | PASS | 03:08:07 | 9 s after the story |
| Management casts engineering, round 1 | PASS | 03:08:13 | 6 s |
| Engineering round 1 | FAIL (blocked; recovered) | 03:08:13 to 03:10:14 | 2 min |
| Texture re-casts, management casts round 2 | PASS | 03:10:34, 03:10:41 | 27 s |
| Engineering round 2: build, test, vet, Freeze | PASS | 03:10:44 to 03:19:50 | 9 min |
| Final report completed, names the bundle | PASS | 03:20:14 | |
| Verify, awaiting_approval | PASS | 03:20:29 | 15 s |
| Pre-apply checkpoint | PASS | 03:20:47 | |
| Owner approval (first try) | PASS | 03:20:47 | |
| Host build, then materializing | PASS | 03:20:47 to 03:37:57 | 17 min 10 s |
| One planned restart (`self_development_apply`, target = the operation, no crash lines) | PASS | boot 03:38:00 | |
| Applied, route receipt | PASS | 03:38:14 | 17 s after the materialize |
| Probe served from the layered host-built release (build `e1d3cd73`) | PASS | 03:38:48 | |
| Restore to the pre-apply checkpoint (`tape_reconstruct`, 930 events, witness matched, frontend restaged) | PASS | 03:39:07 | 1 try |
| Restart into the pre-apply release; probe gone (404, build `0cbc52edc`) | PASS | 03:39:13 | 6 s |
| Release witness joined at frozen, accepted, applied and restored | PASS | 03:39:14 | |

Wall clock was about 31 minutes from mint to restore. The host build took
more than half of it (17 minutes; go-served-1920 took 13). The capsule's
cold `go build ./cmd/autoputer` took about 7 minutes (job start 03:11:09;
the test job started 03:18:24).

## Round 1: the first thing that failed

Source: the run events of `run:assignment-2b214c9c8f28c015a7b61836`, read
from the product route `GET /api/runs/{id}/events` and kept in the bundle.

- Engineering wrote the package, its test and the route. The first write
  had a Go syntax error in the cell; the second worked. The frontend build
  job passed.
- At 03:08:53 it started `go build -o /var/lib/artifact/release/bin/autoputer
  ./cmd/autoputer` as a job with a 20-minute budget.
- It then called `choir.Poll` on that job 30 times in 73 seconds
  (03:08:53 to 03:10:06). It also read `choir.Help("Poll")`, which says
  the call "never waits: end your turn and you are woken when the job
  ends". Each poll showed the job still running, with `go: downloading …`
  lines in the tail, so modules were arriving through the host goproxy.
- At 03:10:02 it tried `choir.Complete("blocked", …)`, and the reducer
  refused: "job … is still running; Complete waits until it ends … or
  choir.Stop it". Engineering then stopped the build and completed as
  blocked ("the required bin/autoputer build job … remained running across
  repeated polls with dependency downloads and no completed receipt").
- The round took 51 model turns and 1.68 M input tokens, about all of it
  polling.

Classification: a desk behaviour problem. The model did not wait on the
job; it polled the job in a loop. It was not a network problem: the build
was progressing when it was stopped. The substrate held up. The reducer
refused a Complete while the job was running. The failed round's diff was
carried: round 2 read `/selfdev/carried/source.patch` (2135 bytes) and
resumed from it, so failure 15 option A now has a proof on a Go change.

Round 2 polled as well: 57 polls over a build of about 7 minutes, in 71
model turns and 1.54 M input tokens. This time it waited for the job to
end. Its first `choir.Complete` was refused with "executor receipt
unavailable": it cited the receipt with one character dropped
(`…a3a3cdb4…` for `…a3a3c3db4…`). It corrected the receipt and completed
on the next cell, 6 s later.

## Observations, not failures

- D32: the computer is class test, so the jailer and host-only DNS select
  it at launch. Product routes cannot show whether it really ran jailed,
  and I did not look at the host. In the capsule, the goproxy and the model
  gateway both worked across the mint, the apply restart and the restore
  restart.
- At 03:39:07, right after the restore answered, the tracer read the
  operation back as `awaiting_approval`. That is its state at the restored
  sequence 930. The `restored` witness at 03:39:14, taken after the
  restart, reads `applied`. One of the two reads was taken before the
  restart and one after, and the cause has not been looked into: it is
  worth one read of the operation projection after a restore.
- The computer hibernated for idleness at about 03:39:27, 13 s after the
  drill's last request (`stopped_by: idle`). After that,
  `/api/computers/{id}/self-development/operations` answers 502 "computer
  authority unavailable", which is expected for a hibernated computer. It
  was kept, as `--keep-computer` asks.

## What to fix first

The busy poll. In both rounds, polling took most of the model turns and
tokens, and in round 1 it made the desk give up on a healthy build. Either
`choir.Poll` on a running job should end the turn and wake the desk when
the job ends, or engineering's overlay should forbid polling in a loop.
Second: the 17-minute host build, the longest stage of the cycle, which a
warm build cache would shorten.
