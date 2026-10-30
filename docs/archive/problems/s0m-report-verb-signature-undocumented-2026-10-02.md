# S0m: research desk cannot call choir.Report/ReportPacket — signature undocumented

**Found:** 2026-10-02 during RN3b deployed acceptance (trajectory
`aaaf9a83-8c63-5862-aa2d-6da5774814d7`, research run
`ee6bc98c-d30c-4f16-83f4-8f5261f55441`, autoputer d4d10e27).

## Evidence

Thirteen consecutive `desk_go_eval` cells failed at compile time guessing
the report verb signature:

- `choir.ReportPacket(map[string]any{...})` → `cannot use type map[string]any
  as type string` (toDesk bound the packet map)
- `choir.ReportPacket(p)` → `not enough arguments` (missing resolverID)
- `choir.Message(m)` → `not enough arguments`
- `choir.Report("research:543b…", p)` → reducer rejected:
  `report tray-1 targets unknown desk` (the desk reported to itself)

The run completed with the answer narrated in the final text instead of a
committed report — exactly the kind of silent delivery loss RN3 exists to
kill, produced one layer up at the verb signature.

## Cause

`internal/runtimeprompts/overlays/rlm_research_runtime.yaml` (and the
management overlay) name `choir.Report`, `choir.ReportPacket`, `choir.Message`
as coordination/reporting verbs but never state their Go signatures:

- `choir.Report(toDesk string, claim string, evidenceRefs []string, resolverID string)`
- `choir.ReportPacket(toDesk string, packet any, resolverID string)`
- `choir.Message(toDesk, body string)` — and lifecycle-bound desks reject
  Message outright (record-native), so it must be de-documented for
  research/texture anyway.

The companion rule "choir verbs take Go-native values" plus no signatures
leaves the model to invent them; it invents wrong ones 100% of the time
observed here.

## Fix class

Green (prompt/default text): add the exact signatures — and the correct
addressee (`texture` — the requesting desk's `requested_by_agent_id`, never
`research:*` itself) — to the research overlay's coordination section, and
the same one-line signatures in the management overlay's coordination list.
RN4 prompt update subsumes; this is the minimum repair for RN3b acceptance.
