# Obscura runs untrusted page JavaScript as guest root, and only one button calls it

Date: 2026-10-21. Status: open; the fix is launch checklist L2-06 steps 1
to 5, station FP6
([choir-launch-fp6-network-2026-10-21.md](../definitions/choir-launch-fp6-network-2026-10-21.md)).
Found by: the L2-06 research pass of 2026-10-21 (launch checklist), which
this record checks against the code. Severity: P0 for strangers sharing a
host (SDC SEC-07, SEC-15). Mutation class of the fix: red (guest boundary,
egress, deployment image).

The record stays at the level the SDC already published
([SEC-07](../security-density-architecture-checklist-2026-10-21.md)). It
holds no payloads and no internal addresses.

## Problem

Web Lens has a backend mode. When it is on, a signed-in user's "Snapshot"
click makes the guest daemon run a headless browser binary, Obscura, against
a URL the user typed. Obscura executes the page's JavaScript in an embedded
V8 and makes its own network requests. Three facts make that a boundary
problem:

1. **It runs as guest root.** The guest runtime unit sets no `User=`
   (`nix/autoputer-vm.nix:928-941`, the `serviceConfig` block), so the
   daemon, and the Obscura child it starts, run as root in the guest. SDC
   SEC-15 already records "pdftotext and Obscura run unsandboxed".
2. **It has no address guard.** The navigate handler checks only that the
   scheme is `http` or `https` (`internal/browsercontrol/handler.go:79-93`,
   called at `:272`) and then passes the URL to the binary
   (`:1534`, `obscura fetch <url> --dump text|html|links`). The binary
   resolves and dials by itself, follows redirects by itself, and runs
   whatever script the page carries. The Go-side follow-ups are no better:
   the declared-alternate fetch pre-checks the first URL with
   `sourcefetch.ValidateURL` (`:748`, a literal-address and name check,
   `internal/sourcefetch/policy.go:82-107`) but then uses a plain
   `http.Client` that follows redirects unguarded (`:817`, `:920-940`).
   Every guarded path in the product goes through `sourcefetch.Client`
   (resolve, refuse non-public answers, dial the checked address, re-check
   each redirect); this one does not.
3. **It is a large, unmaintained surface for a small feature.** Obscura is
   built from a fork pinned to one commit, with a V8 archive pinned to
   rusty_v8 v137.3.0 (`flake.nix:277-297`), and nothing in this repository
   updates either on upstream security releases. The 2026-10-21 research
   measured it at about 508 MiB of guest image (launch checklist L2-06;
   not re-measured here).

## Who can reach it

Any authenticated owner, on their own computer, through
`POST /api/browser/sessions/{id}/navigate` (`internal/apihandler/routes.go:40-42`
registers the three `/api/browser/*` routes; the proxy forwards every
authenticated `/api/*` path to the guest, `internal/proxy/handlers.go:980-985`).
At launch that is every stranger who signs up. The only guard between a
user's chosen URL and a root-owned V8 is the host egress floor (SDC SEC-03),
which has not landed.

The CDP mode (`obscura serve`, `handler.go:1041`) is off unless
`CHOIR_OBSCURA_CDP_SCREENSHOTS` is set, and `nix/autoputer-vm.nix` does not
set it. The fetch mode is on wherever `CHOIR_OBSCURA_BIN` is set, which the
guest unit does (`nix/autoputer-vm.nix:918-920`).

## Who actually calls it

I grepped the whole tree (Go, Nix, scripts, frontend) for `obscura`,
`browsercontrol` and `/api/browser`:

- Go: only `internal/autoputer/run.go:22,220` (constructs the handler),
  `internal/apihandler/routes.go:7,18,40-42` (registers it), their tests,
  and the package itself. `internal/provideriface` carries two config
  fields for it.
- Frontend: only `frontend/src/lib/BrowserApp.svelte` (the Web Lens app)
  calls `/api/browser/capabilities`, `/sessions`, `/navigate`, `/control`
  and `/close`.
- No desk tool, prompt, source poller, research Read, or content import
  uses it: research Read, content import and the pollers use
  `sourcefetch`. This confirms the L2-06 research.

So the capability the product loses by deleting it is "a JavaScript-rendered
text snapshot of an arbitrary URL, made by the guest, on a button". The
source reader already shows durable reader snapshots made by the guarded
import path, and a top-level page in the owner's own browser is not subject
to frame blocking (L2-06).

## Found beside it (discovered, repaired by the same change)

The Web Lens iframe preview has no sandbox. `BrowserApp.svelte` carries the
attribute `autoputer="allow-scripts allow-same-origin ..."` where a
`sandbox` attribute was meant (the sandbox-to-autoputer rename reached an
HTML attribute), together with `allow="camera; microphone; ..."`. A browser
ignores an `autoputer` attribute, so the embedded third-party page runs
with the permissions the `allow` list names and no sandbox. The launcher
removes the iframe, so the defect is repaired by deletion.

## Fix

L2-06, as the checklist wrote it:

1. This record.
2. Stop using Obscura: remove `CHOIR_OBSCURA_BIN` and `OBSCURA_BIN` from the
   guest unit. Reversible by restoring two lines.
3. Web Lens becomes a launcher: a Source Viewer snapshot (the guarded import
   path) and "Open original" in the device's own browser. The `web_lens`
   open-surface value stays accepted in stored data and routes to the
   launcher.
4. Delete `internal/browsercontrol`, its three routes, its config fields,
   its callers and tests.
5. Remove the package from the image (separate commit; the image rebuild
   is the part that needs its own deploy).

## Heresy delta

Discovered: the unsandboxed Web Lens iframe. Introduced: none. Repaired:
both, when steps 3 and 4 land. Residual: the stored `choir.browser_session`
object kind stays registered (`internal/objectgraph/registry.go:95`,
`internal/vocabmigrate/families.go:145`) so any stored session objects keep
validating; nothing writes new ones.

## Proof (deployed, written as steps in the station goal file)

The routes answer 404 for an owner session; Web Lens opens the original in a
new top-level tab and shows a Source Viewer snapshot made by the guarded
import path; research Read of an arbitrary public URL and content import
still work; the paper's polls continue.
