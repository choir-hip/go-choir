---
definition_version: 4
readiness: drafted
review:
  reviewer: none
  frozen_ref: none
  verdict: none
  evidence_ref: none

start:
  captured_at: 2026-11-09T03:10:00Z
  source:
    canonical_ref: origin/main 92f49ad05
    deploy_identity: staging daa7fcd10 (d@e); the owner's computer is unknown (it cannot be read from here)
  worktrees:
    - path: /Users/wiz/Dev/go-choir
      status: unknown
      class: unknown
      owner: owner
      touch: forbidden
      recovery: leave in place
  observed:
    - "Owner, 2026-11-09 (iPhone, own computer): 'What's latest in ai?' reached Texture v1 in about a minute and v2 'after quite some time'; v2 cited 4 sources and v3 7, though Keenable returned about 40."
    - "A note-like prompt, 'Gotta fix the prompt bar', got a v1 acknowledgement only, while management made 498 model calls (165.6M input tokens) on crouton-alpha plus 177 on gpt-6-luna, and engineering made 30."
    - "Model trails walk the whole default pool, including members already marked known exhausted, before reaching crouton-alpha."
    - "Prompt bar UI on mobile: typed text overlaps the desk chip and renders above the bar; the green dot is a status dot, not a submit button; long text does not wrap into a multiline input."
    - "Texture toolbar: Sources is disabled while revising."
    - "d@e: a management run on recursive/flint-v1.0:free looped 60+ ListDir turns (ledger 2026-11-09); one Texture turn was a 165 s free-model call (docs/problems/activation-wait-times-2026-11-09.md)."

finish:
  deliver: >
    A prompt typed into the prompt bar (UI or CLI) reaches Texture, which
    calls research, engineering or both as the prompt needs; versions arrive
    quickly and keep improving; the document shows its sources, models and
    search providers; and an engineering prompt can fetch data from the
    internet, analyse it, and deliver an artifact (a PDF) that opens in
    Reader, with media embedded in the Texture document.
  artifact: the deployed prompt-bar -> Texture -> research/engineering path on staging
  acceptance:
    - action: "d@e and b@c, UI and CLI: 'What's latest in ai?'"
      proves: "v1 within the latency budget (benchmarked on Synthetic DeepSeek, low/medium/high reasoning, no max); research dispatched; later versions cite a justified share of retrieved sources; no pool walk through known-dead members"
      evidence_class: deployed proof (run ids, event timings)
    - action: "a note-like prompt ('Gotta fix the prompt bar')"
      proves: "bounded work: no runaway management loop; the round closes"
      evidence_class: deployed proof (call and token counts per desk)
    - action: "an engineering prompt: fetch public economic data not held locally (for example FRED or BLS series), analyse it, produce a PDF with a chart"
      proves: "Texture -> engineering -> artifact; the PDF opens in Reader; the chart is embedded in the Texture document"
      evidence_class: deployed proof plus screenshot
    - action: "a research prompt citing a YouTube video or podcast"
      proves: "Texture embeds the video/audio, and images and charts, as media blocks"
      evidence_class: deployed proof plus screenshot
    - action: "Playwright at 390x844 and desktop"
      proves: "the prompt bar has no overlap, an always-visible submit button, multiline growth; Sources opens while revising; the Sources panel lists search providers beside models"
      evidence_class: local spec plus deployed screenshot
  rollback: git revert of each landed commit; no data migration
  landing: commit -> push origin main -> CI -> staging deploy -> d@e identity -> deployed acceptance above

value:
  better_means: "minimize the time and model spend from prompt to a good cited version, and the share of ordinary prompts that stall, loop or under-use their evidence, while preserving desk authority boundaries and the no-switches rule"
  goodharting_would_be: "a fast v1 that says nothing; citing every retrieved source regardless of quality; capping management's turns instead of fixing why it loops; a demo prompt tuned to pass"

homotopy:
  realism_axis: "d@e CLI prompt -> d@e UI prompt -> b@c (a second ordinary computer) -> the owner's own computer"

boundaries:
  authority: "owner, 2026-11-09 (preview of the next mission; start tonight)"
  test_computers: "d@e and b@c only; the owner's computer and the paper are read-only"
  models: "crouton-alpha first for almost everything; Synthetic DeepSeek used seldom, as the timing benchmark (owner, 2026-11-09)"
  mutation_classes: "UI fixes orange; pool routing red (provider routing); research selection orange; Texture media blocks red (Texture canonical writes)"
  invariants: [no switches, the restart rules, no secrets or owner addresses in the repo]

now:
  status: working
  slice: "investigate and fix, in parallel: availability-aware pool routing; prompt-bar and toolbar UI plus search providers in Sources; research source selection, the runaway management loop and per-hop latency (docs/problems/ordinary-prompt-path-2026-11-09.md, pending)"
  conjecture:
    claim: "most of the slowness and the runaway are one substrate: the pool walks dead members and a looping thread never fails over, so desks spend minutes and millions of tokens before reaching a working model"
    test: "after availability-aware routing, re-run the acceptance prompts on d@e and compare per-hop timings and calls with the baseline"
    edge: "missing_oracle: the owner's computer cannot be read; d@e may differ in pool state"
    delta_o: "per-desk call and token counts per round, recorded with each deployed proof"
    scope: "d@e and b@c"
  believed_state: "Texture and research work but slowly; research under-uses sources; the pool walks dead members; management can loop without bound"
  main_uncertainty: "why research passes few sources (budget, prompt, capture failure or model choice), and why management looped on a note"
  next_observation: "the investigation doc's per-hop counts and timings on d@e"
  next_action: "land the routing and UI builds through review; then the engineering-to-PDF acceptance prompt on d@e"
---

# The ordinary prompt path

The owner's ask, 2026-11-09: "make sure the whole basic functionality is
working, and it's working as quickly as possible ... we should try to make
some basic research prompts as well as self-development prompts go through
the prompt bar and work correctly ... something which requires engineering
... getting some data from the internet and then doing some analysis ...
turning that into some kind of artifact ... a PDF and then open that PDF in
the Reader app ... And we also should have multimedia embedded in textures."

Follow-on notes from the same evening:
- Fix the prompt-bar alignment. The green dot must be a submit button that is always shown, and long text must expand into a multiline input.
- Sources must stay pressable while Texture revises.
- The Sources panel shows search providers as well as models.
- Research passes on a minority of the sources it retrieves. It should be selective, but it is not choosing well.
- The pool should route by availability, so a turn goes straight to the member that works.

The Autopaper improvement mission
(choir-autopaper-improvement-2026-11-09.md) follows this one and shares its
Texture media work.
