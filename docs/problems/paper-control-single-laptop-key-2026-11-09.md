# Paper control depends on one laptop's saved browser session (2026-11-09)

Class: problem record (green). Fix design:
[paper-control-cli-2026-11-09.md](../design/paper-control-cli-2026-11-09.md)
and [platform-operator-account-2026-11-06.md](../design/platform-operator-account-2026-11-06.md)
section 5.

## Evidence

- Ledger, 2026-11-09 (`docs/definitions/choir-autopaper-improvement-2026-11-09.ledger.md`):
  "Paper control: the only way to act on the paper is a 7-day key minted from
  a saved browser session on one laptop; no sign-in, no policy verbs, no
  validation, no kept revision, no record on the tape."
- [The runbook](../runbooks/paper-control.md) "The key": the paper's owner is
  a test account with no person behind it; its API key lives in
  `~/.config/choir-qa/test-computers/autopaper-paper.json`, expires after 7
  days, and is renewed by `scripts/qa_renew_key.mjs` replaying a saved
  Playwright passkey session on that machine. "If that session is lost, the
  paper has no owner who can act."
- Ledger 2026-11-05T18:35Z (modular architecture): the same key expired on
  2026-11-02 unnoticed.
- `CHOIR_PLATFORM_OPERATOR_USERS` (the O1 roster, carried by the deploy to
  corpusd) exists, but only the package yank uses it; no route lets the
  owner's own account act on a platform computer.

## Belief state

The defect is the authority, not the verbs. Policy verbs with validation and
kept revisions landed (79061b7a8) on the owner-key path, which still needs the
laptop key. Remaining error: the owner's own login cannot act on the paper;
nothing on the paper's tape says who changed its policy.

## Not fixed by this record

Fix lands as slices O2 and O3 of the 11-06 design. O4 (schedules) and O5
(restart, short-lived keys, retiring the saved session) remain.
