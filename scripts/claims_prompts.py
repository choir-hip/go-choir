"""Frozen prompts of the claims E0 study. Their sha256 values are recorded
in docs/evidence/claims-e0/protocol.md before any labelled run; a change
here is a new prompt version, and results name the version they used."""

EXTRACT_V0 = """You extract a claims ledger from one captured news document.

A CLAIM is one attributed assertion: a SPEAKER said something checkable or
meaningful, anchored to a VERBATIM QUOTE from the document. Speakers are
people, organizations, institutions, or the outlet itself when it asserts in
its own voice. When an outlet reports what someone said ("X said Y",
"according to X, Y"), the claim's speaker is X and the carrier is the outlet.

Extract at most 12 claims, the most salient first: factual assertions about
the past or present (figures, events, findings), forecasts about the future,
commitments (a speaker promising its own future action), stated intents or
values, and attribution claims. Skip pure background, navigation text,
bylines, captions, and ads. An opinion or evaluation ("a disastrous policy")
may be included with modality "opinion" only when central.

For each claim give:
- speaker: {"surface": name as written, "type": person|organization|outlet|institution|anonymous|unknown}
- carrier: the outlet carrying it when it relays another speaker, else ""
- quote: the SHORTEST verbatim span from ONE block that grounds the claim,
  copied character for character (same language, same punctuation), at most
  300 characters, normally one sentence or less. Never paraphrase, translate
  or join text from two blocks.
- block: the id of the block the quote is in (e.g. "b7")
- stance: affirms|denies|hedges
- modality: assertion|forecast|commitment|intent|attribution|opinion
- readings: 1 to 4 interpretations {"gloss": English paraphrase of what is
  claimed, specific enough to check, "weight": share, "context_needed": what
  a reader would need to settle it, or ""}; weights sum to 1. Use more than
  one reading only when the statement genuinely admits several meanings.
- quantity: {"value","unit","measure","as_of","approximate"} when a number is
  asserted, else null
- deadline: ISO date or period the claim refers to for forecasts and
  commitments ("2026-10-31", "2027-Q1"), else ""
- expressed_confidence: an explicit probability ("p=0.7"), the hedge or
  estimative word used ("likely", "may", "expects", "could"), or "asserted"
- attribution_basis: verbatim_first_party (the speaker's own document),
  direct_quote (quoted words), paraphrase (reported speech), relay (the
  outlet cites another outlet), anonymous, reportedly, or own_voice (the
  outlet asserting itself)
- p_attr: your probability (0-1) that this speaker really said/asserted this
- p_extract: your probability (0-1) that span, speaker and modality are right
- uttered_at: when the speaker said it if the text tells (ISO date, may be
  approximate like "2026-10"), else ""

Also give:
- mentions: up to 15 named entities {"surface","type":
  person|organization|place|product|work|law|event, "wikidata_guess": a
  Wikidata QID only if you are confident, else ""}
- event: the main real-world occurrence the document reports, {"gloss": one
  sentence, "when", "where", "participants": [names], "topic": the larger
  ongoing subject it is part of}

Answer with JSON only:
{"claims":[...],"mentions":[...],"event":{...}}

DOCUMENT
"""

LABEL_V0 = """You are a careful annotator building a gold set for evaluating claim
extraction. Read the document blocks and list EVERY attributed claim in it.

A claim is one attributed assertion: a speaker (a person, organization,
institution, or the outlet in its own voice) asserts a fact, a figure, a
forecast, a commitment to its own future action, a stated intent or value,
or what someone else said. One sentence may hold more than one claim. Skip
navigation, bylines, captions, ads and boilerplate.

For each claim give:
- speaker: name as written (the outlet's name if it speaks in its own voice)
- quote: the shortest VERBATIM span (copied exactly, original language,
  at most 300 characters) from ONE block that grounds the claim
- block: the block id (e.g. "b3")
- modality: assertion|forecast|commitment|intent|attribution|opinion
- material: true if a careful editor building a ledger of who-said-what
  would keep this claim (central, checkable or consequential); false for
  minor or incidental claims

Answer with JSON only: {"claims":[{"speaker","quote","block","modality","material"}]}

DOCUMENT
"""

JUDGE_V0 = """You audit extracted claims against the source text. You do not know who
extracted them. For each numbered claim, using ONLY the source blocks shown,
answer:
- is_claim: true if the quote really expresses an assertion by the named
  speaker (not navigation, boilerplate, or a misreading)
- speaker_correct: true|false|"unclear" — is the named speaker the one who
  made this assertion (for "X said Y", X is correct; the outlet is correct
  only when it asserts in its own voice)?
- modality_correct: true|false; and correct_modality: one of
  assertion|forecast|commitment|intent|attribution|opinion
- readings_ok: true if the reading glosses are faithful to the quote and
  specific (not vacuous restatements like "something happened")
- specific: true if two careful readers would agree whether the claim turned
  out true / was kept, given its readings (the specificity check)
- settle_route: how it could be settled later —
  "primary_record" (an official dataset, filing, transcript, court record,
  or the actor's own act would settle it), "independent_sources" (needs
  independent reports to corroborate), "actor_action" (a commitment or
  intent settled by the speaker's later observable actions),
  "deadline_event" (a forecast settled by what happens by a date), or
  "not_settleable" (opinion, vague, or unobservable)

Answer with JSON only:
{"verdicts":[{"n":1,"is_claim":..,"speaker_correct":..,"modality_correct":..,
"correct_modality":"..","readings_ok":..,"specific":..,"settle_route":".."}]}

SOURCE BLOCKS
"""

ROUTE_V0 = """You group news items by real-world EVENT, using the structured event
description extracted for each item. Two items are the same event when they
report the same incident or its direct development (an arrest and the
charges that follow; a strike and its casualty count), in any language.
Items on the same TOPIC (same war, same company, same product line) that
report a DIFFERENT incident are different events: keep them apart, and name
the shared topic instead.

Answer with JSON only:
{"events":[{"items":[numbers],"gloss":"..."}],
 "topics":[{"events":[indexes into events],"gloss":"..."}]}
Every item number must appear in exactly one event (singletons allowed).

ITEMS
"""

ROUTE_TITLES_V0 = """You group news items by real-world EVENT. Two items are the same event
when they report the same incident or its direct development, in any
language. Items on the same topic that report a different incident are
different events.

Answer with JSON only: {"events":[{"items":[numbers],"gloss":"..."}]}
Every item number must appear in exactly one event (singletons allowed).

ITEMS
"""

EVENT_FROM_TITLE_V0 = """From this news headline (outlet and script given), describe the
real-world occurrence it reports. Answer with JSON only:
{"gloss": one sentence, "when": "", "where": "", "participants": [names],
 "topic": the larger ongoing subject it is part of,
 "kind": "incident"|"announcement"|"analysis"|"live_blog"|"case_study"|"other"}

HEADLINE
"""

SETTLE_V0 = """You settle a frozen claim against later evidence. You are not told who
made the claim. Use ONLY the evidence text below; do not use outside
knowledge. Answer for each reading of the claim.

Outcomes: "confirmed" (the evidence shows it held), "qualified" (held in
part, or only under a looser reading), "contradicted" (the evidence shows
it did not hold), "unresolved" (the evidence does not settle it).

Answer with JSON only:
{"readings":[{"r":1,"outcome":"...","p":probability the outcome is right,
"evidence_quote":"verbatim span of the evidence you relied on, or ''"}]}
"""

CLOSED_BOOK_V0 = """Answer from your own knowledge only; no evidence is given. Did the
following claim, made at the stated time, turn out to hold? Outcomes:
"confirmed", "qualified", "contradicted", "unknown" (you do not know).
Answer with JSON only: {"readings":[{"r":1,"outcome":"...","p":0.0}]}
"""

HYPOTHESIS_SETTLE_V0 = """You settle hypotheses that an editor wrote BEFORE a news story was
researched. You are not told the editor's confidence. Use ONLY the evidence
below: the research desk's reported findings and the published story.

For each numbered hypothesis answer:
- outcome: "confirmed" (the evidence supports it as stated), "qualified"
  (supported only in part or with a material correction), "contradicted"
  (the evidence shows it is wrong), "unresolved" (the evidence does not
  address it)
- p: your probability that your outcome is right

Answer with JSON only: {"verdicts":[{"n":1,"outcome":"...","p":0.0}]}
"""

COVERAGE_V0 = """A research desk was asked the numbered questions below and then
filed the report that follows, stating that its work was complete. For each
question answer whether the report answers it: "answered" (directly, with
specifics), "partial" (some of it, or says what could not be found),
"unanswered" (not addressed). Use only the texts given.

Answer with JSON only: {"verdicts":[{"n":1,"answer":"answered|partial|unanswered"}]}
"""

# A variant tried only on the historical slice after EXTRACT_V0's recall was
# read there (so its H0 numbers are optimistic, tuned on the same three
# releases): forward-looking statements first, one claim per guided metric.
EXTRACT_V1_FORWARD = EXTRACT_V0.replace(
    "Extract at most 12 claims, the most salient first:",
    "Extract at most 12 claims. FIRST include every forward-looking statement\n"
    "the document makes: each guidance figure or range (one claim per metric\n"
    "and period), target, forecast and commitment; only then add the most\n"
    "salient other claims up to the cap. Claim kinds:")
