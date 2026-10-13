# Deploy fails when every active computer is deferred (2026-10-13)

Status: documented; fix follows in a separate commit.

## Evidence

CI run 38441988844 (40b196f4), job "Deploy to Staging (Node B)":

- The NixOS switch failed once on the known dbus-broker reload stall
  (docs/problems/deploy-switch-dbus-reload-and-vmctl-health-window-2026-10-09.md),
  retried, and completed at 18:00:33.
- The new active-computer refresh (bcc556de) deferred both active computers
  at 18:00:40: the owner's always-on computer (candidate-fleet-e15cb89f…)
  and the Autopaper paper (vm-1669020a…), "busy (guest predates
  planned-restart; /health reports work or did not answer)". Deferral is
  the designed outcome.
- The job then exited 1 with no diagnostics and without printing its
  "Deploy refresh summary" line; the deploy was recorded incomplete
  (/var/lib/go-choir/deploy-failures/38441988844-1.json) and
  `deployed_commit` on /health stayed d75cb193 although every host service
  runs 40b196f4.

## Cause

The deploy script runs under `set -eo pipefail`.
`refreshed_count=$(grep -lx 'refreshed' "$dir"/* | wc -l | tr -d ' ')`
fails when no outcome file says `refreshed`: grep exits 1, pipefail
carries it, and `set -e` ends the script at the assignment. The same holds
for `current_count`. Before bcc556de every selected computer was either
refreshed or failed the deploy, so a zero count never occurred.

## Consequences

- Any deploy whose active computers are all busy fails, with no release
  manifest or receipt, until they go idle.
- Both active guests predate the planned-restart route, so the deploy
  cannot move them by marker; they stay on their builds (owner 761d94c5,
  paper 761d94c5) until a refresh that is a crash boot.
