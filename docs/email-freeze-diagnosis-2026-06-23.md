# Email Freeze Diagnosis - 2026-06-23

## Scope

This pass investigated the reported Email app freeze without changing product code. The evidence is staging browser proof against `https://choir.news` plus source inspection of the Email app, auth renewal helper, desktop state persistence, and desktop suspension paths.

Mutation class: yellow. No runtime behavior was changed.

Protected surfaces inspected:

- `frontend/src/lib/EmailApp.svelte`
- `frontend/src/lib/auth.js`
- `frontend/src/lib/stores/desktop.js`
- `frontend/src/lib/Desktop.svelte`
- `frontend/src/lib/apps/registry.ts`

## Staging Reproduction Attempts

Probe 1 used a temporary staging account:

```text
email-freeze-probe-1782194649400-pqti8r@example.com
```

Steps:

1. Register passkey user on `https://choir.news`.
2. Reload into authenticated desktop.
3. Open Email from the desktop icon.
4. Capture console/page errors plus `/auth/session`, `/api/desktop/state`, and `/api/email/*` responses.
5. Cycle folders: Sent, Drafts, Inbox, Quarantine.

Observed network:

```text
GET /api/email/aliases                      200 {"aliases":[]}
GET /api/email/aliases                      200 {"aliases":[]}
GET /api/email/messages?folder=inbox        200 {"messages":[]}
GET /api/email/messages?folder=inbox        200 {"messages":[]}
GET /api/email/messages?folder=sent         200 {"messages":[]}
GET /api/email/drafts                       200 {"drafts":[]}
GET /api/email/messages?folder=quarantine   200 {"messages":[]}
PUT /api/desktop/state                      200
```

Observed UI after open:

```text
Email / No address / Inbox / 0 messages / No messages / Select a message
```

No browser console errors, page errors, request failures, 401 renewal loop, or hard freeze were observed.

Probe 2 delayed the first `GET /api/email/messages?folder=inbox` response by 12 seconds while allowing the duplicate request to continue. The app did not wedge: the second request returned 200, `loading` cleared, and the UI showed `No messages` while the delayed first request was still pending.

## Findings

### Confirmed: duplicate initial Email bootstrap requests

`EmailApp.svelte` starts authenticated bootstrap work in two places:

- `onMount`: calls `loadAliases()` and `loadMessages(...)`.
- Reactive block: when `authenticated && !loadedOnce && !loading`, also calls `loadAliases()` and `loadMessages(...)`.

On staging, this produced duplicate `GET /api/email/aliases` and duplicate `GET /api/email/messages?folder=inbox` calls during initial open.

This is not by itself a reproduced hard freeze. The delayed-response probe showed one slow duplicate request is survivable. It is still a real hazard because there is no single bootstrap owner, no request generation token, and no abort/timeout. A stale or hung request can leave the component in an old state or keep work pending indefinitely.

### Weakened: auth renewal loop

`fetchWithRenewal` performs one request, one `renewSession()` attempt on 401, then one retry. It does not contain a loop. Staging probes observed `GET /auth/session` returning authenticated state and no `/api/email/*` 401 loop.

### Weakened: desktop suspension

The Email app registry entry has `heavy: false`. `suspendBackgroundHeavyWindows` and `suspendWindowBody` only suspend windows for which `isHeavyAppId(w.appId)` is true. That makes desktop heavy-window suspension unlikely as the direct cause of an Email freeze.

### Not reproduced: reported hard freeze

The hard-freeze condition was not reproduced on a fresh staging account. The missing discriminator is affected-account evidence: a session/account with real Email state, aliases, messages, drafts, or a specific restored `appContext` that triggers the freeze.

## Root-Cause Hypothesis

The strongest supported hypothesis is not a desktop or auth renewal loop. It is an Email bootstrap/load-state weakness:

```text
onMount bootstrap + reactive !loadedOnce bootstrap
  -> duplicate initial aliases/messages requests
  -> no request timeout or generation guard
  -> stale/hung request can keep or overwrite Email state
```

The state variables/functions involved are:

- `loadedOnce`
- `loading`
- `activeFolder`
- `selectedId`
- `detail`
- `loadAliases`
- `loadMessages`
- `loadDetail`

The function most directly responsible for recovery risk is `loadMessages`: it owns `loading`, `loadedOnce`, folder state, selected message state, and detail loading, but it does not know whether it is still the latest request by the time its awaited calls return.

## Proposed Fix Path

1. Replace the dual initial-load paths with one guarded bootstrap function, for example `bootstrapMailbox()`, controlled by an `initialLoadStarted` or `initialLoadKey` guard.
2. Add a monotonically increasing request generation for `loadMessages` and `loadDetail`; only the latest generation should mutate `messages`, `selectedId`, `detail`, `error`, and `loading`.
3. Add abort/timeout handling around Email fetches so a hung `/api/email/*` request can surface an error and release the loading state.
4. Keep `fetchWithRenewal` as a one-renewal helper; do not add a renewal loop.
5. Add a focused Playwright regression that asserts initial Email open performs one aliases request and one message-list request, and that a stale slow response cannot overwrite a newer folder selection.

## Evidence Boundary

This pass supports a fix plan for Email bootstrap hardening. It does not prove the reported hard freeze's exact affected-account root cause, and it does not prove a fix. The next implementation pass should either obtain affected-account staging reproduction evidence first or explicitly treat the bootstrap hardening as a preventive repair for the confirmed request-state hazard.

