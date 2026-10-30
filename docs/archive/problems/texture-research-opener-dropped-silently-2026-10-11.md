# Texture's third research request is dropped silently; the document promises a revision that never comes (2026-10-11)

Mission: [research fan-out](../definitions/choir-research-fanout-2026-10-10.md),
slice S1. Problem first; no fix here.

## Evidence

S1 demo on staging 0b7250ef, document ee7e8c2b, trajectory ef649f2f.

- 00:02:16: Texture's first turn opens two research desks (work 74d7d1bd
  and 7553b91b).
- 00:05:54: revision 3 carries two items and ends "Still open: ... The
  next revision should replace this partial frame as those sources
  arrive."
- 00:06:32: Texture run f48ffa3d commits `decide` / `wait_for_evidence`
  with one `open_researcher` control for the uncovered threads (other
  labs, funding, chips, policy). The cell printed `decide: tray-1 <nil>`.
  The committed reason ends
  `[runtime: research budget for this owner request is spent (2 of 2); 1 research opener(s) not opened]`.
  No `work_opened` or `control_queued` follows.
- Nothing else runs. The document stays at revision 3, promising a next
  revision.

## Cause

`textureResearchBudgetPerOwnerRequest = 2`
(`internal/textureowner/texture_turn_runtime.go`), the bound added for
`texture-research-loop-never-idles-2026-10-09.md`. Past the budget an
opener is dropped at commit, by design "not failed: the turn still
commits". The drop note goes into the committed reason, which the Texture
run does not see: its cell already returned. No later turn wakes, so
Texture never learns that its research did not open.

## Effect

Two research desks per owner request, whatever the question. For a
week-of-news request, the threads Texture named as open stay open, and the
owner reads a document that says more is coming when nothing is.

## Open decision (not taken here)

The 2026-10-09 bound exists so a research report cannot wake Texture into
opening research forever. Choices recorded for after the S1 rerun:

- Keep the count, and make the refusal visible to the Texture run (an
  error or notice in the same cell), so its last revision says what
  remains uncovered instead of promising a next revision. This is the
  minimum whatever else is chosen.
- Gate further rounds on progress instead of a fixed count: another
  research round only after the previous round's evidence was applied to
  the document, under a high ceiling. A fruitless round ends the loop; a
  productive one continues until saturation.
- Leave width to the research desk: with `choir.Research`, one desk can
  fan out as wide as the question needs, so two desks may be enough once
  fan-out works. The S1 rerun with the 2026-10-11 fixes tests this.

## Decision (owner, 2026-10-11 ~01:10Z)

"We shouldn't have multiple research desks for a task ... The one research
desk should fan out if needed, but Texture shouldn't be managing multiple
research desks." And on fan-out: "we really shouldn't be encouraging
recursive fan out. We don't really have the language model or search
limit ... models can and should do a round of search and then report back
and then do another round of search ... because we want to get the data to
the texture agent as soon as possible."

What made Texture manage several desks, besides the budget of two:

- `lifecycleTargetWorkRequestedByTexture` (store) accepts a follow-up only
  from the Texture run that opened the work. Texture is one agent per
  document but a new run every turn, so every later turn could not reach
  the open desk and had to open another.
- Texture's run prompt says "Use parallel research desks when branches are
  distinct", "Open additional research entries when branches are
  distinct", and "Work an earlier Texture run opened ... cannot carry this
  run's control; open a new research entry".

The change:

1. One research desk per owner request. A follow-up from any turn of this
   document's Texture reaches it (authority stays with this document's
   Texture agent; the run match goes). A second opener in the same request
   becomes a follow-up to that desk instead of being dropped.
2. The 2026-10-09 loop bound becomes a bound on research rounds per owner
   request (the opener plus follow-ups), not on desks.
3. The research desk works in rounds: search, read, report what landed,
   then the next round on what is still uncovered, all in one run.
   choir.Research stays on the desk for a question that splits into
   separate parallel lines; children cannot call it (no recursion).
4. Texture's prompt says one desk per request, follow-ups to it.
