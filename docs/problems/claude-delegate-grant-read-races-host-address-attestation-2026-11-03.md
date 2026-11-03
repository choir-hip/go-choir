# The Claude delegate's grant read races the host's address attestation after a restart (2026-11-03)

Date: 2026-11-03. Follows problems 25 and 26 of the data system cutover
ledger. Mutation class of the fix: red (gateway, provider credential
relay), tier 2 review. Problem first; no fix in this commit.

## The problem in one sentence

When a computer with the Claude delegate grant reboots onto a new guest
address, its first grant read reaches the gateway before vmctl re-attests
the new address, the gateway answers 429, the guest caches that refusal for
30 seconds and the engineering spawn cancels the round with "claude delegate
unavailable: the gateway refused the grant read (status 429)".

## Evidence

Node B journal, 2026-11-03, claude arm
(`computer-a875023a6379c41f6ddb438f02e76a02`, `vm-8bee33d5…`). The VM was
rebooted by deploy refreshes four times between 21:39 and 22:21Z, each on a
new address (vmmanager `booted VM` lines):

| boot (vmctl)  | new address  | relay refusal (gateway)                     | pin moved by vmctl ensure |
|---------------|--------------|---------------------------------------------|---------------------------|
| 21:39:07      | 10.202.199.2 | none logged                                 | 21:39:07                  |
| 21:47:36      | 10.202.205.2 | 21:47:36 refused, pinned to 10.202.199.2    | 21:47:36                  |
| 22:05:52      | 10.202.211.2 | 22:05:52 refused, pinned to 10.202.205.2    | 22:05:52                  |
| 22:20:52      | 10.202.218.2 | none logged                                 | 22:20:52                  |

Each refusal is in the same second as the boot and as the gateway's
`Jev bearer of autoputer … moved to its current guest address` line, and
no refusal follows the move. Gateway restarts (21:36:52, 21:47:02,
21:58:36, 22:10:36) did not reset or freeze the pin. The relay refusal log
is bounded to one line a minute per caller address, so more refusals in
the same minute are not visible.

The addresses are VM addresses, not capsule addresses: vmmanager logs each
as the VM's host (`host=http://10.202.205.2:8085`). A claude-mode capsule's
relay calls leave through the capsule's egress proxy in the guest, which
swaps the capsule's relay secret for the computer's bearer, so the gateway
sees the VM's address. The pin is per VM identity, not per capsule.

Ordinary inference from the same VM did not fail: its 429 carries
`Retry-After: 2`, and `internal/gatewayruntime/provider.go` waits it out.

## Cause, from code

- The pin is `AutoputerIdentity.PeerIP`. vmctl moves it to the VM's current
  address through `POST /provider/v1/credentials/ensure` with `peer_ip`
  (`ensureExistingGatewayCredential`, `internal/vmctl/ownership.go`), after
  a boot returns. That is the host's attestation and the single source of
  truth; the relay only reads it (`relayPeerCheck`,
  `internal/gateway/anthropic_relay.go`).
- A boot returns once the guest answers, so a guest that resumes work at
  boot (a planned, marker-backed refresh of an always-busy computer) can
  call the gateway before the ensure lands.
- The relay's mismatch answer is `authFailure`'s 429 `rate_limit_error`
  "credential is not attested for this address; retry shortly", shared with
  inference.
- The guest's grant read (`ClaudeDelegateGateway.read`,
  `internal/capsule/claude_delegate.go`) treats any non-200 as unavailable,
  does not retry, reports only "status 429", and caches the result for
  `claudeDelegateGrantTTL` (30 s).
- `Executor.Spawn` refuses a claude-mode capsule on that status, and the
  assignment runtime cancels the round (`spawn assigned capsule after
  durable open: …`).

## Effect

A round whose capsule spawn lands in the first second after a restart is
cancelled, with a reason ("status 429") that reads like a rate limit or a
quota. Management on the claude arm read it as "claude-mode capsules cannot
work on this computer" and rewrote the round to drop the delegate.

## The coordinator's hypotheses, checked

- "The pin keeps the first IP and refreshes only at a gateway restart":
  refuted; vmctl's ensure moves the pin in the boot's own second, and
  gateway restarts did not move it.
- "The addresses are per-capsule addresses": refuted; they are the VM's
  boot addresses, and the VM did reboot at each.

## Fix direction

- Keep the host's ensure as the only writer of the pin; the relay still
  refuses another VM's bearer and any address the host has not attested.
- The grant read gets a distinct answer for an address that is not yet
  attested (status 409, type `address_not_attested`, Retry-After), and the
  guest waits it out for a bounded time, as inference does, without caching
  a transient refusal for the full TTL.
- When the wait runs out, the reason says in plain words that the gateway
  still holds the address pin from before this computer restarted, that this
  is usually transient, and names the options: retry the round, or re-run it
  without the delegate.
