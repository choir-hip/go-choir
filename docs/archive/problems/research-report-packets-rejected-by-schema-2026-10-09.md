# Research report packets are rejected by the packet schema; Texture gets no sources (2026-10-09)

Found by the Texture acceptance suite, fifth run on staging (search-plane
fix 97970cda deployed; receipt
`evidence/texture-acceptance-2026-10-09T19-50-30-737Z.json`), fresh
disposable account on VM `vm-6965f6ce…`, research document `b3c347df…`.
Problem first; no fix in this commit. Mutation class of a fix: orange
(cell verb behaviour) plus yellow (desk overlay text).

## What the owner sees

T1–T5 pass (first draft and revise 26 s each; cancel clears in 1 s). T6
asks for current reporting with two cited web sources. No research
revision lands in 6 minutes, and none by 20:00Z.

## Trace

- Search works now: every gateway search from this computer between
  19:52:53 and 19:58:45 succeeded (Brave, Parallel, SerpAPI).
- Texture turn reasons (trajectory `69814e50…`), in order: the desk
  received "a research blocker with zero sources"; "two candidate URLs but
  zero durable source records (packet.sources empty; its own import call
  failed)"; two topic-fit imports but "packet.sources is empty"; "imported a
  current source but its packet was schema-rejected"; "Six worker packets
  have now returned zero durable source records; each failure is a
  packet-schema field-name mismatch". Texture refused to cite without a
  source record each time. That is the right call (no fabricated
  citations).
- The research runs' own events (internal run events, six research runs)
  carry the rejections: `not a coagent source packet: json: unknown field
  "claim"` (5), `"title"` (2), `"date"` (2), and `cannot unmarshal string
  into ... CoagentPacketSource.sources.selectors of type
  types.CoagentPacketSourceSelector`.

## Cause (code reading)

- The research overlay (`runtimeprompts/overlays/rlm_research_runtime.yaml`)
  says ReportPacket's packet is "a map literal shaped like
  coagent_source_packet.v1 (kind "evidence_update", summary, claims,
  sources, questions)". It names no field inside a claim or a source.
- The schema (`types.CoagentSourcePacketPayload`) wants `claims[].text`,
  `sources[].kind` (`web_source`, ...), `sources[].target.uri` and
  `.title`, `sources[].selectors[]` as objects with a `kind`
  (`text_quote`, ...). The model's natural guesses (`claim`, `title` and
  `date` on the source, selectors as strings) are all rejected:
  `commitAddressedPacketIntent` (`agentcore/rlm_reduce.go`) decodes with
  `DisallowUnknownFields`.
- `choir.ReportPacket` (`yaegikernel/choir.go`) only JSON-encodes the
  value; validation happens at reduce time, after the cell. So the desk
  learns of the rejection late, and it spends a research round per
  attempt.

This is the same shape as O24 (closure depends on a field the model is
not told about), at the research-to-Texture boundary: the contract is
strict and the desk is never shown it.

## Fix directions

1. Validate at the call: `choir.ReportPacket` strict-decodes the packet
   when the cell calls it and returns the error at once, naming the
   expected fields.
2. Show the contract: the research overlay gives one exact minimal packet
   (claim `text` with `source_ids`, source `kind`/`target.uri`/`title`,
   a `text_quote` selector).
3. Not chosen: loosening the decoder to accept aliases. The strict schema
   is what keeps a packet's sources typed and citable.
