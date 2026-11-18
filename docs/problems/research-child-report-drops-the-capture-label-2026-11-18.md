# A research child's report reaches its parent without the capture label (2026-11-18)

## Problem

A run that reads a captured page with `ReadCapture` is marked
`owner_private:capture`. That mark is the floor publish admission checks
before a story built from captures is published without asking
(`internal/agentcore/publish_auto_admission.go`).

When a research run delegates to a web child with `choir.Research`, the
child's report comes back to the parent labelled as public-web text only,
whatever the child read. A child that read a capture and quoted it hands the
parent that text without the floor.

## Evidence

- `internal/agentcore/run_labels.go`, the `ActionResearch` case of
  `egressReadLabels` (line 198 on main at d21930b76): a child whose role is
  not the Texture role gets `worldReadLabels`; nothing looks at what the
  child's own run was marked with.
- Found by review seat 2 on 2026-11-18 while reviewing the full-page search
  of captures (`lane/capture-fulltext-2`), which closes the same gap for
  search results that carry a page passage. Seat 2 called it "pre-existing;
  same class".

## Belief

The parent should inherit the union of the labels its children's runs
acquired, as it inherits the Texture label from a Texture child. Not yet
traced: whether a child's run labels reach the parent or the publishing run
by another path (the run lineage, the delegated task's records). If they do,
this is a gap only in the label computed from the report; if they do not, a
story can be composed from captured text and published with no capture floor
on the record.

## Remaining error

- No trace of a real run was read; this is from code.
- Whether any computer holds a capture that is private to its owner is not
  known. On the newspaper's computer captures are public web pages and
  publish admission admits the capture label for its own stories, so the
  newspaper's behaviour would not change with the fix.
- Not fixed. It is outside the full-page search change and needs its own
  red change with two review seats.
