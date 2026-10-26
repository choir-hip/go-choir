# PD3 S1 acceptance on b@c, 2026-10-25

The Texture root desk and master document
([PD3](../definitions/choir-pd3-texture-root-desk-2026-10-11.md), slice S1),
deployed in 7f769cc34 (CI 40188211719 success, deployed 11:26:11 UTC, vmctl
NRestarts 0). b@c (computer-6c50d6e566143ae32cdb660faa60e24d) booted onto it
at 11:29. Run by FP0 with `scripts/proof-pd3-root-desk` (run 1, stopped at
its on phase) and the same checks by hand after it. Scratch evidence:
`/private/tmp/claude-501/-Users-wiz-Dev-go-choir/99fb2dd5-a4f7-4cc5-a0f5-6ee541f24543/scratchpad/pd3run1/`.

## Results

| Check | Result |
|---|---|
| Off is inert | pass: texture.root_desk off at 11:30:13; no "Master document" before or after six owner turns (11:30-11:32) |
| Master appears | pass: switch on 11:32:12; the first digest created the master ef0e2ef8 at 11:34:13 (v0 owner-authored seed, generation 1) |
| Writer | pass: the only agent with runs on the master's trajectory is texture:ef0e2ef8 (trace export, runs layer); master revisions after v0 are appagent |
| Coverage | pass: every live document's open work items (3 across the computer) appear by 8-hex short id in the master's refs; every document with a trajectory is listed |
| Standing item | pass: the master's trajectory is live with one open item (77d20dad) |
| Isolation | pass: A's Texture run 9587225f cancelled mid-turn at 11:35:34; document C revised six times; the master's next revision (11:37:37) reflects A's new request and C |
| Rate | pass: two root runs (11:34:13, 11:37:33), 3m20s apart, one master revision each |
| Latency, owner revision to Texture commit | off: 10, 11, 11, 21, 21, 21 s (p50 16 s); on: 10, 10, 10 s (A only) |
| Refusals | store test receipt (TestTextureRootRefusalsComeFromTheChannelBinding, CI 40188211719); no product-API path lets a caller choose the writing agent, so no deployed negative probe exists |
| Crash | runtime test receipt (TestPreBootTextureRootDigest*, actorruntime, CI 40188211719); no no-SSH crash act exists for a test computer, a named residual |

## What the master says

The root's first revision covered the 50 most recently revised documents:
for each, what the owner last asked, open work by desk, what waits on the
owner, and a refs line. It named the one genuinely open item on the
computer, work 4c55862e on the Reader document (trace review R6-6, an item
left open by a refused cast).

## Findings

- F1 (proof): document B's trajectory settled after its third owner turn
  (all its work items completed), and its next owner revision was refused
  with 409 "durable lifecycle state is unavailable or terminal". This is the
  known settled-document limit (the metamission's blocker_or_risk), not a
  PD3 mechanism; it stopped the script's on phase. The script now creates
  fresh documents for each phase.
- F2 (quality): the root summarized a rejection as the owner's request for
  the Reader document ("making each wrapped PDF line its own paragraph",
  which was the complaint). "What the owner last asked" reads the newest
  owner revision, and a rejection reads like a request. Watch in S2.
- F3 (latency on): the on-phase sample is one document and three turns,
  smaller than the design's ten per document; the p50 is lower than off,
  not higher, so the bound holds on this sample.

## S2 paired test, 2026-10-25 12:29-12:33 (fails)

S2 deployed in 3a1a702ff (CI 40192322501 success, vmctl 12:24:22, NRestarts
0); b@c booted onto it at 12:28. Document A ("Team offsite plan", doc
5a090ab3) states an open plan: a full-day team offsite at the lake house on
Saturday, November 7th. The root rendered it into the master at 12:31:14,
whose head now begins with "Waits on the owner".

- On (switch on, master current): document 6da12ab4 asks to book the lake
  house for a family reunion that day. Its run's prompt carries the
  labelled excerpt (label present), but the excerpt holds the master's
  first 11 document sections, oldest first, and is cut before A. The answer
  does not mention the offsite.
- Off (switch off at 12:32:15): document 94ea5d70, the same request. No
  label in the prompt, no mention of the offsite. Correct.
- A first attempt at the off half (document 2a6f8671, "Family reunion") is
  void: the setting call was malformed and the switch stayed on; that run
  got an excerpt from the 11:37 master, before A existed.

Finding F4 (design gap, S2): the root writes the master's document sections
in a stable order with new documents appended at the end (22 sections;
the newest last), and the excerpt is cut from the end at 4 KiB. So the
excerpt carries the oldest documents and drops the newest, which are the
ones most likely to bear on a new request. "Cut from the end" was only
right if the end was the least relevant part. The fix belongs in the
excerpt builder, deterministically: order the other documents' sections by
their documents' last revision time, newest first, before cutting; and ask
the root to write the master newest first as well, for the owner's own
reading. The switch was turned back on at 12:33:18 so S1 keeps the master
current.

## S2 paired test, rerun after F4, 2026-10-25 13:09-13:11 (fails again)

The F4 fix deployed in a4ef55f5d (CI 40195331859 success, vmctl 13:04:44,
NRestarts 0); b@c booted onto it at 13:08. On sample e89959c8: label
present, offsite absent from the excerpt, absent from the answer. Off
sample f26d775a: no label, no mention. The excerpt was still in the
master's own order.

Finding F5 (format mismatch): the root writes each document's refs at the
end of its paragraph ("... is listed. refs: doc=<id> work=none"), and the
excerpt builder recognizes "refs: doc=" only at the start of a line. Every
section therefore counted as unknown and kept the root's order, and the
run's own section was not dropped either. The unit tests passed because
they built masters in the format the parser expects rather than the one
the root writes. Fix: find "refs: doc=<uuid>" anywhere in a section, and
test against a master in the root's real inline format (taken from b@c's
master ef0e2ef8).

## S2 paired test, third run after F5, 2026-10-25 23:51-23:52 (mechanism passes, behavior fails)

The F5 fix (f82a750cf, then b6a9b1f37 for review 251's LOW: a section's
last refs name it, headings are not read) deployed 13:33:04 (CI
40197428136 success, vmctl 13:32:07, NRestarts 0); b@c booted onto it
(source projection upstreamed, booted base b6a9b1f37). The run waited ten
hours: FP0's deploy watcher polled a wrong observability path
(/api/computers/<id>/observability, 404; the route is
/api/runtime/observability) and never ended.

- On (doc 1c9632e6): label present; the excerpt carries "### Team offsite
  plan" (the full-day lake-house offsite on Saturday, November 7) near the
  top, newest first. The answer drafts the reunion booking and does not
  mention the offsite.
- Off (doc 34ffea0a): no label, no mention. Correct.

Finding F6 (behavior, hypothesis): the context reaches the run and the run
does not use it. The label says to read it as context, never act on it
alone, and the request does not ask about conflicts; a sample of one. Not
tuned yet (understand before tuning): next, read the run's turn to see
whether the model saw the excerpt and judged it irrelevant, then decide
whether the label or the prompt core should name cross-document conflicts
(a date or a resource another document already holds) as something to
surface to the owner.

F6, cause found (23:5x, from the run's prompt, not yet from its reasoning):
the excerpt's label, as S2 shipped it, ends "Never act on it alone: do not
write, cast, ask or note about another document because of a line here."
Naming document A in document B's answer is a note about another document,
so the label forbids exactly what the paired test requires ("on, it names
A"). The contradiction is between the S2 label and the PD3 acceptance
text, not a model lapse. Fix (lane pd3-s2d): keep "never write, cast or
note on another document", and add: if the owner's request conflicts with
an open request listed (the same date, place or resource), say so in your
own document, name that document's title, and let the owner decide. The
run still writes only its own document.
