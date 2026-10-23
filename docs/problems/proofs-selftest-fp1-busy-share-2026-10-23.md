# The proofs self-test fails "fp1 step 1 computes a busy share" under jq 1.6 (2026-10-23)

Found 03:10 UTC running `bash scripts/proofs/selftest.sh` on origin/main
(a33a3dfec): `157 passed, 1 failed`, the failure being
`FAIL fp1 step 1 computes a busy share`. Class: yellow (a proof script and its
self-test; no runtime behavior).

## Evidence

- The self-test's stub fixture is sound. Running it with a newer jq passes:
  `PATH=/opt/homebrew/bin:$PATH bash scripts/proofs/selftest.sh` gives
  `158 passed, 0 failed` (jq 1.8.1). With the operator shell's own PATH, the
  first `jq` is `/opt/anaconda3/bin/jq`, version jq-1.6, and the case fails.
- Under jq 1.6 the step prints `could not read two engine_mutex samples`. That
  message hides the cause, because `fp1-store.sh` step 1 sends the jq filter's
  stderr to /dev/null. With the redirect removed, jq 1.6 says:
  `syntax error, unexpected label, expecting IDENT or __loc__` at
  `{label: $label, window_s: ...}` (fp1-store.sh line 85).
- `label` is a jq keyword (`label $name | ...`). jq 1.7 and later accept a
  keyword as an unquoted object key; jq 1.6 does not.
- The filter has had that line since the scripts landed in 50d5c3773
  (2026-10-22, `git log -S'{label: $label'`). No lane renamed an observability
  field: the stub's `engine_mutex` shape (`calls`, `wait_ns`, `hold_ns`,
  `max_hold_ns`) matches what the filter reads. So this is not a regression
  from a recent change; it never passed under jq 1.6.
- Not a stub-fixture bug and not a parsing bug against the real shape: it is a
  jq-version portability bug in the proof script's own filter. The same filter
  on staging would fail the same way from any machine whose `jq` is 1.6, and
  the message would blame the guest.
- No other unquoted keyword key exists in `scripts/proofs/*.sh` (grepped for
  label, and, or, not, if, then, else, end, as, def, reduce, foreach, try,
  catch, import, include).

## CI

The self-test is not run in CI. Nothing under `.github/` references
`scripts/proofs/selftest.sh`; it is run by hand. Not wired in here (out of
scope).

## Plan

Quote the key (`"label": $label`), which every jq version accepts and which
changes nothing the step measures. A self-test case pins the rule so a future
unquoted keyword key fails the self-test under any jq.
