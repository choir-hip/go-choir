# Why Texture Background

Date: 2026-06-15

Status: historical and explanatory background. This is the current document
that may discuss the retired VText name directly while explaining the rename to
Texture. It is evidence and context, not the operational protocol.

## Why This Exists

Texture did not begin as a naming exercise. It emerged from repeated failure of
turn-thread surfaces to carry multi-agent work, long-running work,
evidence-bearing work, and safe self-development.

The name change matters because the old name, VText, sounded like an internal
data structure. It was technical, fragile, and transcription-hostile. It did
not help a human or an agent feel the category. Texture is stronger because it
suggests surface, weave, grain, continuity, tactility, text, and artifact-ness
without requiring a glossary.

Texture is not poetic branding. It is an ontology shift.

## The Multi-Agent Transcript Lesson

The first Choir prototype was a multi-agent assistant. A user entered a prompt,
then the system ran a sequence of models. A fast model wrote an initial answer.
Another model searched a knowledge base. Another performed web research. A
critic inspected the research. Another model wrote a response. A final model
rewrote the answer to a style guide.

The output could be high quality. Model diversity helped. Staged context growth
helped. For some questions it was even interesting to see the intermediate
responses.

But the surface was wrong. A five-paragraph answer already asks for attention.
Six or seven intermediate responses can become thirty paragraphs. Most users do
not want the entire cognitive supply chain printed into the conversation for
every prompt.

The chain also failed to scale with difficulty. A weather question should not
activate a full research, critique, rewrite, and style pipeline. Hard questions
needed more cognition, simple questions needed less, and the fixed transcript
shape flattened both.

Showing every intermediate response overloaded the human. Hiding every
intermediate response created invisible state. That fork is the root problem.

## The Ghostwriter Lesson

The later ghostwriter prototype kept multi-agent generation but published only
the final article. Intermediate work stopped burdening the reader. The pipeline
could produce usable prose.

That clarified a second failure. The bottleneck was not just output rendering.
The bottleneck was idea development. The system could write articles, but it
did not give the human a durable surface where thinking, evidence, revision,
style, and direction could evolve together.

The lesson was:

```text
intermediate work should become evidence for an evolving artifact,
not prose the owner must read linearly
```

## The Web Desktop Deduction

Coding agents revealed the stronger pattern. A coding agent does not merely
print a better answer. It mutates a durable context through targeted edits. The
codebase is the substrate, and the diff is the reviewable mutation.

That makes coding agents a CLI for AI.

The corresponding GUI is not a larger conversation window. The GUI for AI is a
GUI: a persistent computer with apps, files, artifacts, state, workers,
evidence, and candidate worlds.

On-device AI helps but cannot be the whole answer. Device owners control the
platform. Local agents can corrupt local state. Laptops and phones sleep, run
on batteries, and cannot reliably host 24/7 background work. Remote desktop and
VNC expose the wrong interface for mobile and touch. A virtual phone inside a
phone is not the right abstraction. The web desktop follows from these
constraints: a persistent cloud computer reachable through the web, designed
for agentic apps and human supervision.

The first web desktop made an assistant-thread agent the main control plane. It
could control apps and build a new app that appeared on the desktop. That demo
proved the web desktop mattered. It also revived the same transcript problem:
conversation surfaces couple model context, user-visible history, and control
plane state too tightly.

## Versioned Documents As Superset

The deeper move was to treat versioned documents as a superset of conversation.

Each user prompt, direct edit, agent revision, research incorporation, source
packet, or execution result can become a new version of an artifact. Once the
artifact is versioned, Choir gains properties that transcript surfaces lack:

- diffs instead of repeated full responses;
- inline editing instead of prompt-only steering;
- canonical current state instead of transcript archaeology;
- exportable artifacts without conversational debris;
- reusable context that can be cited, embedded, forked, or transcluded;
- reviewable state transitions rather than a pile of messages.

This was the original force behind VText. The old name pointed at versioned
text, but the abstraction kept growing beyond text. The object was also a
document, version tree, transclusion surface, publication object, live
supervision narrative, style carrier, and multi-agent artifact control plane.

Texture names that larger object better.

## Transclusion And Live References

Texture is not only the latest rendered prose. It is a living object with a
version tree. The default reading surface shows the latest version. The owner
can step backward and forward through history, compare versions, restore a
historical version, publish a particular version, and fork from a point in
history.

Each version can carry different transclusions. A version may cite source
material, another Texture, a code diff, a screenshot, a video, a generated
artifact, or a verifier receipt. Later versions may inherit or change those
transclusions. The previous version's evidence remains intact.

Live references matter too. A published Texture should not be only a frozen
snapshot unless explicitly pinned as one. A published Texture is the whole
living object, with its revision history and current head visible according to
publication policy. When another Texture transcludes a version, it should pin
the cited version by default while showing that newer versions exist.

## Automatic Computer, Paper, And Radio

Texture also explains why the web desktop expands into automatic paper and
automatic radio.

Automatic paper is not a traditional newspaper with faster article production.
An article is a Texture over an event, issue, claim-space, or continuing story.
As new evidence arrives, the article revises. Traditional newspapers often
treat correction as reputational cost. Automatic paper treats revision as the
normal shape of learning.

Automatic radio is a later projection from the same substrate. Audio,
human voices, generated voice, clips, archival material, and user recordings
can become projections from provenance-bearing Texture. A listener can consume
the work as radio, but the source of truth remains the revisioned,
transclusive, evidence-bearing artifact.

The important claim is not that every output becomes text. The claim is that
every serious output needs a substrate that can preserve provenance, revision,
style, source, uncertainty, and learning.

## Safe Self-Development

The major milestone for Choir is safe self-development: Choir as a
self-improving mainframe where users can request custom apps, inspect evidence,
approve promotion, and roll back if needed.

That requires more than a coding agent. It requires a human-facing artifact that
can explain the current purpose, evidence, architecture, candidate work,
verifier findings, risks, promotion claim, and rollback path while work is
still happening.

If the report appears only after a long run completes, the human cannot steer
the run while the important decisions remain open. If every action appears in a
log, the human cannot keep up. Texture is the impedance-matching layer between
fast autonomous intelligence and bounded human judgment.

It does not govern autonomous work as a passive supervisor. It directs results
with autonomy and facilitates learnings. It lets the owner correct the idea,
not only the next action.

## Why Rename

The old VText name was useful during construction because it pointed at
versioned text. It has now become too narrow.

Texture is better because it names the substrate:

- text, but not only text;
- surface, but not only UI;
- weave, because evidence and revisions interlace;
- grain, because provenance and style remain visible;
- continuity, because the object persists across revisions and projections;
- artifact-ness, because the work lives somewhere owned and addressable.

The hard cutover should be real. Old ontology should not survive as current
runtime names, prompts, UI labels, API concepts, docs, tests, or agent
vocabulary. Historical mission docs can remain as git history or explicitly
historical evidence. The live system should teach agents and humans the current
name.

## Short Form

Texture is the single-writer-among-agents, append-only, transclusive,
present-tense artifact layer that turns autonomous activity into directed
results and compounding learning.

The prompt surface is where you ask. Texture is where the work lives and
learns.
