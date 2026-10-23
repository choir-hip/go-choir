# Two Deployed-Proof Steps Need A View From Inside A Guest, And There Is No Route

Date: 2026-10-23. Source: the launch first-pass ledger
([docs/definitions/choir-launch-first-pass-metamission-2026-10-21.ledger.md](../definitions/choir-launch-first-pass-metamission-2026-10-21.ledger.md),
08:25 UTC entry: "Step 11 (oversized bodies from a guest shell as root), FP7
step 5 and SEC-10 step 3 need a guest root shell") and the SEC-10 section of
`docs/plans/security-reds-readiness-2026-10-23.md` on
`origin/lane/fp0-security-readiness`. Mutation class of the fix: **orange**
(additive guest runtime diagnostic routes, host-peer only). Problem first; no
code in the commit that wrote this record. The owner consented to FP0 running
these two steps.

## The problem in one sentence

Two deployed-proof steps are written as "from a guest shell", the guest has no
shell (the image has no sshd, no getty and no serial login, and the Management
Console's shell was removed), so the steps cannot be run and two launch
residuals stay open.

## The two steps

1. **SEC-10 step 3, guest DNS through the host resolver.** Owner-shell
   version, three reads on test computer T: `cat /etc/resolv.conf` (expect
   `nameserver 127.0.0.53`); `resolvectl status` (the link's DNS servers are
   exactly the tap host address, no fallback in use); `cat
   /run/systemd/network/10-vm-runtime.network` (ends with `DNS=<tap host
   ip>`). The unit is written by `go-choir-configure-network`
   (`nix/autoputer-vm.nix:407-421`), which appends `DNS=` only when the kernel
   command line carries a well-formed `choir.dns=`. Steps 2, 4 and 6 are
   recorded PASS in the ledger (08:08 to 08:12 on D12); step 3 and the
   closing of public UDP 53 wait on this one.
2. **FP7 step 11, gateway body caps from a guest with the computer's own
   token** (`docs/definitions/choir-launch-fp7-gateway-identity-2026-10-21.md`
   lines 767-782). Shell version: read the token from
   `/run/choir-bootstrap/gateway-token`, then (a) stream 100 MiB at
   `POST /provider/v1/search` and expect 413 (or a connection closed
   mid-upload), (b) declare a 700 MiB `Content-Length` at
   `POST /provider/v1/inference`, over the observe ceiling of 576 MiB, and
   expect 413 before a byte is read, (c) one ordinary prompt still answers.
   Part (c) needs no guest view (a product prompt). Step 10 (observe mode,
   limits in `/health`) is already PASS from Node B.

## Why no route exists today

`GET /internal/diag/tcp-dial` (`internal/autoputer/boot_timeline.go:585`,
registered `internal/autoputer/run.go:639`) is the only host-peer guest probe.
It dials IP literals and, with `mode=http`, sends an allowlisted forged-identity
request. It cannot read guest files, cannot do a name lookup, cannot speak UDP,
and may not carry an `Authorization` header (the allowlist is
`X-Internal-Caller` and `X-Authenticated-User`, by design: "arbitrary headers
would turn the oracle into a credential relay"). So neither step is expressible
through it, and widening it would break the property that made it safe to leave
deployed.

## The two probes

Both are `GET` (the body probe also accepts `POST`) on the guest autoputer's
port 8085, answered only to the host peer (`server.HostPeerCaller`, the strict
form: the guest's default gateway on the tap link; loopback and in-guest
callers get 403). Both take no caller-supplied name, address, size or path.

### GET /internal/diag/dns

Read-only. Returns, as `kind: guest_dns_probe`:

- `resolv_conf`: path, the `nameserver` lines of `/etc/resolv.conf`;
- `network_unit`: path, the `DNS=` values of
  `/run/systemd/network/10-vm-runtime.network`;
- `resolved`: the `nameserver` lines of `/run/systemd/resolve/resolv.conf`
  (resolved's own list, which shows a fallback server if resolved fell back),
  plus, when `resolvectl` is in the image, the bounded output of the fixed
  command `resolvectl status` and the DNS and fallback lines parsed from it;
- `system_lookup`: one lookup of the fixed name `example.com` through the
  guest's system resolver, with addresses and latency;
- `direct_udp53`: one UDP DNS query for `example.com` to the fixed public
  resolver `1.1.1.1:53` with a 2 s timeout, `reachable` true or false;
- `caller_ip`: the host-peer address that asked (the tap host address).

Fixed targets mean the probe is not an open resolver oracle. After SEC-10 closes
public 53, `direct_udp53.reachable` flips to false; today it should be true
for an unselected computer and (until the close) also for T, which is the
point of reading it before the close.

### GET|POST /internal/diag/gateway-body-cap

Sends two requests to `RUNTIME_GATEWAY_URL` with the guest's own token
(`provideriface.GatewayToken()`, the same resolution the guest's search client
uses) and reports each leg's status, response headers, a body prefix and the
latency. The token is never in the response (the response is also scrubbed of
the token's value as a second guard).

- `search_streamed`: `POST /provider/v1/search`, a chunked body of 1 MiB +
  4 KiB (fixed), against the search cap of 1 MiB. Expect 413. This is step
  11(a) at the smallest size that crosses the cap.
- `model_declared`: `POST /provider/v1/inference` with `Content-Length:
  734003200` (700 MiB, step 11(b)) and **no body bytes sent**, over a raw TCP
  connection with a 5 s deadline. Expect 413 before a byte is read.

The probe is serialized (one at a time, 429 if busy) and spaced (10 s between
runs, 429 with `Retry-After`), so it cannot be turned into a flood.

## Provider-safety argument (no provider is called, no tokens spent)

The caps are in `observe` mode on the model routes, where a model body over
160 MiB and under the 576 MiB ceiling is served. The probe is built so that
mode does not matter. Everything below is at origin/main `b1abdfb0b`.

1. **The search leg cannot become a provider call in any mode.**
   `HandleSearch` (`internal/gateway/handlers.go:904`) reaches a search
   provider only after `h.decodeBody(...)` succeeds and `req.Query != ""`
   (`:940` decode, `:949-950` query check, provider call at `:970` `StreamFor`). The search cap
   enforces in every mode (`body_caps.go:20-23`: "Every other route enforces
   in every mode"). The body is `{"query":"` followed by `a` bytes and is
   never closed: an unterminated JSON string. Under the cap the decoder hits
   EOF inside the string and returns an error, and the handler answers 400
   "invalid request body" without reaching the query check. Over the cap
   `MaxBytesReader` fails the decode and the answer is 413
   (`body_caps.go:368-371`). No cap setting, mode, or override of
   `GATEWAY_BODY_LIMITS` makes this body decode, so `req.Query` is never set.
2. **The model leg sends zero body bytes.** A model provider call needs a
   successfully decoded `ProviderRequest` (`handlers.go:442` decodes first,
   and returns on any error). A body whose `Content-Length` is
   700 MiB and of which none is sent either is refused unread because the
   declared length is over the read cap (`body_caps.go:318`: `r.ContentLength
   > readCap`, 413, ceiling 576 MiB in observe, 160 MiB in enforce), or, if the
   gateway had been configured with a ceiling over 700 MiB, waits for bytes
   that never come and is cut by the probe's 5 s deadline, whereupon the
   decode sees a truncated or empty body and fails. A decode of nothing never
   yields a request, so no provider is reachable.
3. **Authentication precedes both**, and the identity is the computer's own, so
   the only side effects on the gateway are one search-window slot and one
   model-window slot of that computer (a rate-limit entry), plus the
   `body cap refused` log lines and `refused.*` counters that step 11 reads.
4. **A test pins all of it.** The yellow tests include a gateway-package test
   that feeds the probe's exact bytes (from the shared payload package) to the
   real `Handler` with a counting search provider and a counting model
   provider, in observe and enforce mode, with the cap limits at default and
   at a raised value, and asserts both counts stay zero.

If a later change to the gateway makes any of these false (a route that
decodes before its cap, a tolerant partial-JSON decoder), the pinned test
fails.

## What the probes do not show

The 100 MiB streamed body of the shell version is not reproduced (4 KiB over
the cap crosses the same line, with the same code path, at 1/100 of the
bytes). Gateway memory staying flat under a 100 MiB stream is a separate
property that step 11 also checks; it remains the owner's or FP0's from the
host side, or can be read from step 11's `MemoryPeak` check after this probe,
which only proves the refusal path, not memory under a long stream.

## Rollback

Both routes are additive and inert unless asked: revert the commit, or leave
them deployed (host-peer only, fixed targets, serialized). A computer started
before the deploy simply answers 404 and the driver reports "route missing".
No flag, no state, no schema, no event on the tape.

## Heresy delta

- discovered: the proof steps were written for a shell the image does not have;
  `tcp-dial` is deliberately too narrow to carry them.
- introduced: none (two read-only host-peer routes).
- repaired: the steps are now expressible; closed only when FP0 runs them on a
  deployed test computer.
