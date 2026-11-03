# A new account signed in by `choir login` has no computer (2026-11-03)

Status: open. Found by the deployed acceptance of CLI onboarding
(`scripts/e2e-cli-login-staging.sh`, staging 4036f0c37, run 03:13Z-03:19Z).

## Evidence

- L0, A1 and L1 pass: a fresh passkey account approved the device login and
  `choir login` wrote the credentials file (mode 0600).
- C1 fails: `GET /api/compute/status` with the new key listed no computer for
  five minutes. vmctl's journal for 03:13:30-03:20:00 shows no admission or
  start for a new ownership (the only admissions are existing computers).
- The spec registers at the site root and then opens the approve link; it
  never loads the desktop. A computer is created when the desktop bootstraps
  (`/api/shell/bootstrap` resolves the primary desktop), so a person or agent
  who signs up and signs in only through the terminal flow has a key and no
  computer: every computer-scoped command fails.
- The run exited at C1, before R1 (logout), so the run's key was not revoked;
  the credentials file was deleted by the script's exit trap. The key belongs
  to a throwaway example.com test account.

## Belief

FINDING: no computer exists for an account until its desktop first loads.
HYPOTHESIS: the right fix is that approving a device login (or the first
owner-wide key use) resolves the account's primary computer, the same call
bootstrap makes, so the terminal path is complete on its own; and the
acceptance script revokes its key on every exit path.

## Fix (branch cli-login-computer)

The first owner-wide API-key read of `/api/compute/status` on an account whose
primary desktop has no computer starts, in the background, the same vmctl
resolve the bootstrap makes (`internal/proxy/api_key_first_computer.go`). One
flight per user; a failure holds off the next attempt 30 s; cookie sessions and
bound keys are unchanged. The acceptance script revokes its key on every exit
path and waits 10 minutes for the listing and for the first self-dev answer.
