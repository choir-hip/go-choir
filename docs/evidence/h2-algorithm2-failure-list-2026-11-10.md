# H2 algorithm2: the build's failure list (2026-11-10)

Lane H2, pass 1. Written before any code or test. Mutation class: green (docs).
Rollback: revert. Serves handoff section 6 item 8 ("a user sees their behavior
ledger, and an algorithm2 proposal that came from it").

Where the spec is silent (decisions, to be repeated in the report):

- The behavior ledger has no field-level definition (contracts 5.5). Pass 1
  derives it on read from what the computer already holds (changelog events
  and the revisions they point at); it adds no state, so there is no second
  authority to drift.
- A proposal is a local object (a file on the owner's computer), shaped like a
  published version, served to the revise call through the one
  `PublicationVersionReader` seam. The revise call is not widened. Decision D9
  (published versions only) is the open owner residual; the seam is the one
  place that changes either way.
- Whether an edit is auto-accepted is decided by code from a line diff of the
  proposed text against the algorithm document, never from what the model says
  its edits are.

Ways it can fail, each with the test that must be written first:

1. The ledger leaves the computer: it is served only on owner routes, an
   in-process desk caller is refused, and nothing in it enters a publish or
   changelog path.
2. The ledger is unbounded: the outline is capped at a constant entry count and
   says how many were left out.
3. Auto-accept trusts the model: a proposal that calls its edit "an addition"
   but removes a line must be asked, because the diff decides.
4. Rule precedence: "ask before" beats "auto-accept" when both match; no rules,
   an unparsable rule or an unknown section means ask.
5. A stale proposal is applied: the algorithm document's head moved since the
   proposal's base; the proposal is marked stale and nothing is written.
6. Accept twice or a retry: the revise call's client request id is derived from
   the proposal id, so the second accept replays and writes one revision.
7. Cadence: a second round inside the cadence makes no model call; a failed
   model call consumes the slot; a pending proposal on the same base makes no
   call; an empty ledger makes no call.
8. A crash boot runs a round: a process within the startup grace, or a markerless
   boot within one cadence, runs none.
9. Bad model output: malformed, oversized, identical to the current text, or
   emptying the document, yields no proposal and no write.
10. Prompt injection through ledger text (titles, author labels): ledger text is
    data in the prompt; the accept decision never reads model prose.
11. A forged or foreign proposal id used as a revise source: unknown ids,
    another document's proposal and an already-decided proposal are refused at
    the reader.
12. Owner edits algorithm2 between the round and the decision: rules are read at
    decision time from the current head.
13. A proposal id escaping the proposals directory: ids are fixed-width hex.
