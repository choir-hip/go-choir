# The approval page's return to the phone's sign-in sheet (server and page)

Part of [EXACT.md](../EXACT.md). This page holds the part that is on main: the
server and the `/device` page. **The app part (opening the system sign-in sheet
with Exact's `openAuthSession`, the callback grant, the attempt state machine)
is not landed**; it is on `lane/signin-sheet`, and that lane's version of this
page replaces this one when it lands. Nothing here has been seen on a phone.

- **Server.** `POST /auth/device/approve` takes an optional `return` and
  `state`. `return` is an app id from a fixed list in server code
  (`DeviceReturnSchemes`, the ids of `mobile/AutoradioLab/app.json`: the default
  and variant `one`), never a URL. `state` is 16 to 128 of `A-Za-z0-9_-`. Only
  an **approval** answers `return_to`, and only after the decision is recorded:
  `<app id>:/signed-in?state=<state>`. A denial answers none (the app learns of
  it by its poll). A bad or missing hint is not an error: the decision is
  recorded and `return_to` is left out. A hint never refuses an approval and
  answers nothing about a code (an unknown code is the same 404 with or without
  it). A body over `readJSON`'s 1 KiB cap is a 400 before the code is looked up
  (no decision, no miss counted); the page never sends such a hint.
- **The callback is a hint.** It carries the app's own nonce and nothing else: no
  key, no device code, no user code, no account id. The key arrives only by the
  device poll to `/auth/device/token`, which needs the device code and an
  approval by a passkey session.
- **Page.** `frontend/src/lib/device-return.js`. The link the app opens carries
  `return` and `state`. The page sends them with the decision only if they have
  the server's shape, and after an **approval** follows `return_to` only if it
  is exactly `<the scheme it sent>:/signed-in?state=<the state it sent>`.
  Without a hint the page and the response are as they were.
- **The allowlist is ahead of the app.** `DeviceReturnSchemes` holds the second
  id (`news.choir.autoradio.lab.one`) before `app.json` declares it on main; the
  Go test is one-way (every id `app.json` declares is in the list) until the app
  part lands with its strict equality test.
- **Proof.** Go tests in `internal/auth/device_return_test.go`, node tests in
  `frontend/tests/device-return.test.mjs`, and
  `scripts/proofs/device-approve-return-deployed.sh` (a stranger's checks only).
  The approval answering `return_to` on staging is not yet seen.
