# Account switcher: after adding an account there is no way back to the first

Date: 2026-11-09. Status: open, not fixed. Reported by the owner on staging.

## What the owner saw

Signed in as one account, the owner used "Add another account" in the
launcher and signed in as a second account. The browser then ran as the
second account, and the launcher offered no row to switch back to the first.

## Evidence (code, read-only)

- `internal/auth/accounts.go`, `addToAccountSet`: a sign-in adds only the
  account that just signed in to the browser's set. Its one caller is the
  sign-in handler (`internal/auth/handlers.go:473`).
- A session that began before the switcher shipped has no `choir_accounts`
  cookie and no member row. A desktop-redeemed session has none either
  (design doc, test list item 16).
- So when such a browser adds a second account, the set is created holding
  the second account alone. `GET /auth/accounts` then lists one account, and
  the launcher has nothing to switch to.

## Belief state

Hypothesis, strongly supported by the code and not yet confirmed from the
auth log: the owner's first session predated the switcher (or came from the
desktop handoff), so it was never a member of a set. The log line to read is
`operation=account_set_add` for the second sign-in, and whether the request
carried a set cookie.

A second cause is not ruled out: the first account's refresh session may
have been revoked or rotated out during the add flow.

## Workaround

Sign out everywhere, sign in to the first account, then add the second.
Both sign-ins then join one set.

## Fix direction (not built)

When a sign-in arrives with no set cookie but with a valid refresh session
for a different user, add that session to the new set as a non-active
member. This is auth and session code: red, two review seats, proved on d@e
with two test accounts before it lands.

## Also reported: b@c "on old code"

At 22:40 UTC b@c's computer reported build `cb149a273`, the deployed
release, with no divergence. What the owner saw as old is not yet
identified; candidates are a cached browser bundle, or the launcher package
the page loaded before the switch. Needs the owner's observation (which
screen, what was missing) before any change.
