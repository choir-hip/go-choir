# An app package cannot declare its agent's network (NET-10)

Date: 2026-10-22. Class: design gap, red when built. Checklist item: NET-10
in the [security, density and architecture checklist](../security-density-architecture-checklist-2026-10-21.md).
Station: [FP10 NET-10](../definitions/choir-launch-fp10-net10-2026-10-22.md).

## Observed (code at main 51cb3a8cd)

- `internal/appdesk.Manifest.Egress` is a list of bare strings
  (`egress_verbs`), and `appdesk.Validate` refuses any non-empty list with
  `egress_not_granted` (`internal/appdesk/appdesk.go`, "owner grants are
  NET-10's"). There is no typed egress entry a package could fill in.
- There is no ingress field at all. The only app-agent ingress is mail
  through maild on the host (`internal/maild/webhook.go`, `ingest.go`); no
  model says how a package desk names a host service, an address or a
  record shape.
- `deskmanifest.appDeskVerbs` has no egress verb; `deskmanifest.Desk` has
  no egress field (ARCH-02's "egress verbs" field is unbuilt).
- `internal/egressgrant` (NET-05) has the app-desk declaration form
  (`HolderAppDesk`, `verb`, `effect_class`, `operation_contract`,
  `data_ceiling`, `credential`), but nothing turns a package manifest into
  those declarations, and nothing decides whether an upgraded manifest
  widens the approved one.
- The host fetch verb (NET-01, `POST /provider/v1/fetch`, on
  `lane/fp10-net01-host-fetch`, not on main) admits a computer by VM ID and
  carries no grant id, so it cannot tell an app desk's allowlisted call
  from research's open read.
- The NET-10 design ([app-agent-network-policy-2026-10-21.md](../design/app-agent-network-policy-2026-10-21.md))
  was written before `internal/deskmanifest`, `internal/appdesk`,
  `internal/egressgrant` and the host fetch verb existed; it says
  "`internal/deskmanifest` does not exist yet", which is no longer true.

## Why it matters

1. **Agency.** Capability 7 of "Networking must enable, not starve" (app
   agents' declared ingress and egress) has no path: a Mail, Calendar or
   podcast app agent that needs one API cannot get it, even with the owner's
   approval.
2. **The safe default has no successor.** Refusing all egress is fail
   closed, but the first app that needs the network will be tempted to route
   around it (research relaying text, a system desk verb) unless the
   declared path exists.
3. **Widening on upgrade is undetectable.** Without a typed entry and a
   pure diff, an upgrade cannot be held for re-approval entry by entry.

## Not the problem

- General guest egress (NET-01, NET-04): a desk worker already runs in an
  empty network namespace (SEC-01), so an app desk's cells have no sockets;
  the gap is the declared path, not a leak.
- The grant record itself (NET-05, on main) and install as a tape fact
  (ARCH-03, `internal/appdesk.Derive`, on main).
