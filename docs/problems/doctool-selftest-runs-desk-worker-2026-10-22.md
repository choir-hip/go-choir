# The isolation self-test runs the document-tool probe (and the seccomp leg) with the desk-worker binary, which serves only desk-session (2026-10-22)

Status: fixed on lane/fp2-selftest-binary (test c6f76c31c, fix 7dad68b50),
not yet deployed; staging step 1 and the seccomp leg re-run after it lands.
Found by FP2 second batch staging step 1 on deploy P.
Mutation class of the eventual fix: orange (a diagnostic self-test path; no
product behavior).

## Evidence

Deploy P (main 966d3f519, live on staging since 12:35 UTC). Staging step 1
of "Second batch, staging steps"
(`docs/definitions/choir-launch-fp2-desk-boundary-2026-10-21.md`) on the FP2
test computer computer-9648031f92e131adc1fb65ec455c5903, guest build
966d3f519 (built 20261022121952, deployed 2026-10-22T12:34:49Z). Receipt
`desk_cell_confinement_staging_receipt`, verdict failed (FP0's
`~/runs/fp2-p-step1.json`). The server's answer:

    document_tool: no report (warnings ['/nix/store/k76cfgkykh75ldynhvqmhd4ldwmm70hk-autoputer-0.1.0/bin/desk-worker_failed: desk-session: session socket unavailable (fd=-1 err=<nil>)'], failures ['the sandboxed probe did not run cleanly: ...same...'])

Everything else in the self-test answered: `runtime_capabilities` observe,
`cap_bnd` 000001ffffffffff, 25 extra, none missing; `document_tool.observe`
all zeros (no document had been read yet).

## Cause (confirmed in code)

- `internal/agentcore/isolation_selftest.go:148` calls
  `doctool.SelfTest(ctx, deskWorkerBinary(), ...)`.
- `doctool.SelfTest` (`internal/doctool/probe_linux.go:142`) uses that one
  binary as both the sandbox helper (`runOptions.helperBin`, run as
  `<bin> doc-tool-sandbox <tool> ...`) and the tool (`<bin> doc-tool-probe`).
- Only `cmd/autoputer/main.go` dispatches `doctool.HelperArg`
  (`doc-tool-sandbox`) and `doctool.ProbeArg` (`doc-tool-probe`).
- `deskWorkerBinary()` (`internal/agentcore/tools_desk.go`) returns
  `deskWorkerBinaryFor(os.Executable())`: the dedicated `desk-worker` beside
  the daemon when that is an executable regular file, else the daemon
  itself. RES-17 (c97ad44f8) added the dedicated binary; the image ships it
  beside autoputer (`flake.nix`, `extraBins = [ "desk-worker" ]`; the path
  in the warning).
- `cmd/desk-worker/main.go` is `os.Exit(deskworker.Run())` and dispatches
  no argument: it ran as a desk-session worker with no session socket and
  printed the error above (`internal/deskworker/deskworker.go`). The helper
  exited 1, not with `helperSetupExit`, so the run reads as a tool failure
  (`desk-worker_failed`), not a setup failure.

The same class, by code reading (not yet seen on staging): the capsule
seccomp leg (`?leg=seccomp`, `isolation_selftest.go:213`) calls
`capsule.SeccompSelfTest(ctx, deskWorkerBinary())`, which runs
`<bin> capsule-seccomp-selftest <filter>`
(`internal/capsule/seccomp_selftest_linux.go:200`). Only
`cmd/autoputer/main.go` dispatches `capsule.SeccompSelfTestArg`, so on an
image with desk-worker every filter's child would run desk-session and
fail. No staging receipt of that leg on such an image is in the repo; this
is a hypothesis until a receipt shows it. The other callers of
`deskWorkerBinary()` start desk-session workers
(`tools_desk.go:445`, `isolation_selftest.go:251`), which is what
desk-worker serves; they are correct.

## Why each lane's tests passed alone

- The FP2 second batch (doctool, capfloor) was written when
  `deskWorkerBinary()` returned `os.Executable()`. Its doctool tests run
  the test binary itself as helper and tool (`TestMain` dispatches both
  args); nothing in them involves agentcore's binary choice.
- RES-17's tests check that a dedicated desk-worker is preferred for
  desk-session, which is right for desk cells. They do not run the
  self-test's document-tool or seccomp legs.
- In agentcore's tests `os.Executable()` is the test binary and no
  `desk-worker` sits beside it, so `deskWorkerBinary()` returns the test
  binary and the self-test never sees desk-worker.
- The guest image is the first place both meet: deploy P carried batch 2
  after RES-17, and step 1 was the first self-test there.

## Production impact

None found. Production extraction (`internal/content/extract.go:216`) calls
`doctool.RunOnTempFile`, which leaves `runOptions.helperBin` empty, and
`runSandboxed` then uses `os.Executable()`, the autoputer daemon, which
dispatches `HelperArg` (`internal/doctool/doctool_linux.go`). No production
caller passes `deskWorkerBinary()` to doctool or capsule; the only
`doctool.SelfTest` caller is the isolation self-test. FP0 is checking this
on staging with step 2's observe counters after real PDF reads.

## What it blocks

Until fixed, the self-test cannot prove the document-tool sandbox: step 1's
`document_tool` judgement (uid 61010, NoNewPrivs, Seccomp 2, CapEff 0, every
probe denied) has no report to judge. The switch criterion "required is
ready" for `CHOIR_DOCUMENT_TOOL_SANDBOX` therefore cannot be read, whatever
step 2's counters show. The seccomp leg, if the hypothesis holds, cannot
prove the capsule filters either.

## Fix direction

Pass the daemon's own executable (`os.Executable()`, with a test override as
`deskWorkerBinOverride` has) to `doctool.SelfTest` and
`capsule.SeccompSelfTest`, not `deskWorkerBinary()`. The alternative, making
`cmd/desk-worker` dispatch the doctool and seccomp args, would link the
doctool sandbox and capsule seccomp code into the binary RES-17 keeps
minimal, and would make desk-worker a second entry point for those
privileged-setup paths. A test pins that the self-test's binaries are the
daemon's own even when a desk-worker sits beside it.
