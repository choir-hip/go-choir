# The host fetch verb is open to every guest, 2026-10-22

Status: open. Found by FP0 on candidate lane/fp10-net01-host-fetch
(c8428350f) before panel review. Documented before the fix. Mutation class
of this record: green.

## The problem

Station FP10 shipped the host fetch route "off by default" by gating only
the guest: vmctl writes a grant on a listed computer's credential disk and
only that guest routes its fetches to the gateway. The gateway's
`POST /provider/v1/fetch` (`internal/gateway/host_fetch.go`, 2a73810d4)
checks the bearer and nothing else. Any guest with a valid gateway bearer,
listed or not, can call it. With `CHOIR_HOST_FETCH_COMPUTERS` empty, that is
a new host-side egress path open to every guest: neither off by default nor
fail-closed at the host. The guest's grant is a client-side choice, and an
attacker in a guest does not have to make that choice.

## A fact the fix must respect

The gateway knows a guest only by its credential identity, and vmctl issues
that credential under the **VM ID** (`vm-...`), not the ComputerID:
`issueGatewayToken(own.VMID)` (`internal/vmctl/ownership.go:1450, 2183`),
`issueGatewayTokenAt(r.gatewayURL, vmID)` (`internal/vmctl/platform_computer.go:365`).
The gateway cannot check a ComputerID list, nor a warmness class
(`class:test`), without reading vmctl's state.

## Constraints on the fix

- The gateway refuses the verb (403, typed error, one log line without the
  path) unless the bearer's identity is on the host's own list.
- The list comes from the host (environment), one source shared with vmctl;
  never from anything the guest sends.
- Empty or unset list: every call refused.

## Fix

Station FP10, follow-up commits on the same branch.
