# Autopaper captures were built per computer, against the Store B taxonomy

Found 2026-10-11 10:30Z by the owner, after W1b landed on staging
(c1d21d5e). Evidence class: design and goal-file text, landed code, and
the owner's correction.

## What happened

W1b gave research a `choir.PollSource` verb that writes each polled item as
a `choir.web_capture` in the polling computer's own embedded object graph.
Its bodies go into a capture segment in that computer's file root. The
owner: "autopaper shouldnt run on embedded dolt, but global dolt ... the
point is to make it a resource for everyone. then individual users/vms
(and eventually, different hosts) can have overlays to filter, customize,
combine private data."

The repository already said so. docs/computer-ontology.md ("Dolt Store
Taxonomy") names Store B, the `corpus` database behind corpusd, as the
world-wire store. It holds the platform object graph and the source and
ingestion tables. The VM-local embedded store indexes one computer's own
event chain.

This is a recurrence. Standing question 2's receipt is the same failure in
July: an Autopaper activation definition "hard-coded embedded-Dolt VM
topology two days after D-WIRE settled sql-server mode — twelve attempts
... faithfully rebuilt the rejected architecture."

## Cause

The 2026-10-09 design doc (docs/autopaper-sw-design-2026-10-09.md, §2
"Storage that cannot repeat the 104 G problem" and §8 "Storage") placed
the paper's working state, captures included, in "the paper's computer
(embedded Dolt plus content storage)". It kept only published Textures
global. Those lines answered the 104 G problem (unbounded ingest into a
versioned shared store), but they moved the shared resource itself onto
one computer. W1 was authored from the design doc without diffing it
against the store taxonomy (standing question 2), and C1 (the guest's
single embedded connection) was then treated as Autopaper's first
substrate problem. With captures in Store B, ingest never touches a
guest's embedded connection.

## What is true now

- Captures are a shared resource in Store B, written through corpusd.
  Their bodies live in host content storage, never in Dolt rows (the
  104 G lesson stands).
- Each computer keeps its own working state, private documents and
  overlays (filters, customizations, private data combined with shared
  objects by id and version) in its embedded store.
- Published Textures stay global, as before.
- The Dolt upgrade (W1a) stays: Texture and every guest still run
  embedded Dolt.

## Test

No Autopaper capture is written to a guest's embedded graph. A poll from
the paper's computer lands its captures in Store B and its bodies on host
storage, and any bound computer can read them.
