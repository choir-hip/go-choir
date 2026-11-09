---
definition_version: 4
readiness: executable
review:
  reviewer: none (owner promotes by pasting the /goal string, 2026-11-09)
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
  correction_2026_11_09_evening: >
    Added at promotion; the observations above stand. d@e is the only
    computer on the layered guest image, so the Claude binary layer reaches
    no other computer, and d@e is not on the delegate grant: its delegate
    status reads "the claude binary is missing from the guest image". b@c's
    delegate is granted (Sonnet, until 2026-11-17, 50M tokens a day).
    Investigation receipts: docs/problems/ordinary-prompt-path-2026-11-09.md
    (where research drops sources, the note runaway, the per-hop time
    budget). Prompt to v2 on d@e took 202 s, over 90% of it model latency;
    research held its first citable source from +18 s to +126 s; it read 12
    results and discarded 3 Read errors unseen; the tool loop has no
    iteration cap and no repeated-call detector (toolloop.go:841). The
    owner's 498-call runaway did not reproduce on d@e.

finish:
  deliver: >
    A prompt typed into the prompt bar (UI or CLI) reaches Texture, which
    calls research, engineering or both as the prompt needs; versions arrive
    quickly and keep improving; the document shows its sources, models and
    search providers; an engineering prompt can fetch data from the
    internet, analyse it, and deliver an artifact (a PDF) that opens in
    Reader; Texture embeds media (images, charts, PDF and file artifacts,
    video and audio). Every computer runs the layered guest image, and the
    Claude delegate works under the new sandbox on d@e, b@c and the owner's
    computer. This is the groundwork the Autopaper improvement mission
    (choir-autopaper-improvement-2026-11-09.md) builds on: its images and
    charts land in the same Texture media blocks.
  artifact: the deployed prompt-bar -> Texture -> research/engineering path on staging, and the fleet on the layered image
  acceptance:
    - action: "d@e and b@c, UI and CLI: 'What's latest in ai?'"
      proves: "v1 within the latency budget (benchmarked on Synthetic DeepSeek, low/medium/high reasoning, no max); research dispatched; an interim packet reaches Texture within about 30 s of research's first citable read; later versions cite a justified share of retrieved sources, with every dropped read visible as a reason; no pool walk through known-dead members"
      evidence_class: deployed proof (run ids, event timings, model trail)
    - action: "a note-like prompt ('Gotta fix the prompt bar')"
      proves: "bounded work: no runaway management loop; a repeated identical cell is stopped with a visible fate; the round closes"
      evidence_class: deployed proof (call and token counts per desk)
    - action: "an engineering prompt: fetch public economic data not held locally (for example FRED or BLS series), analyse it, produce a PDF with a chart"
      proves: "Texture -> engineering -> artifact; the PDF opens in Reader; the chart is embedded in the Texture document"
      evidence_class: deployed proof plus screenshot
    - action: "a research prompt citing a YouTube video or podcast, and one with an image and a chart"
      proves: "Texture embeds the video/audio, images and charts as media blocks"
      evidence_class: deployed proof plus screenshot
    - action: "Playwright at 390x844 and desktop"
      proves: "the prompt bar has no overlap, an always-visible submit button, multiline growth; Sources opens while revising; the Sources panel lists search providers beside models"
      evidence_class: local spec plus deployed screenshot
    - action: "the fleet: vmctl launch records after the ring walk reaches the paper"
      proves: "every computer cold booted the layered release (base drive attached); none fell back to single except with a logged cause; the paper refreshed last, planned and marker-backed"
      evidence_class: Node B metadata and logs (launch lines, ring walk state)
    - action: "claude delegate status on d@e, b@c (and read-only on the owner's computer); one self-development round on b@c whose engineering edits the Claude delegate served"
      proves: "the delegate is granted and runs under the new worker sandbox; the trace shows the delegate as the editor of kept code"
      evidence_class: deployed proof (delegate status, trajectory trace)
    - action: "self-development rounds on b@c, delegate inside: themes OLED Black, Vector Heart (the Designers Republic aesthetic), Neo Vector Heart, Metal Heart, Utopian Scholastic, and hybrids with Frutiger Aero and Liquid Metal; then the desktop background split"
      proves: "each theme applies through self-development and is kept; the background is the owner's image or graphic from Files or an upload, not part of the theme"
      evidence_class: deployed proof plus screenshots
  rollback: git revert of each landed commit; the layered ring returns to d@e alone by restoring the R0 list; no data migration
  landing: commit -> push origin main -> CI -> staging deploy -> d@e identity -> deployed acceptance above

value:
  better_means: "minimize the time and model spend from prompt to a good cited version, and the share of ordinary prompts that stall, loop or under-use their evidence, while preserving desk authority boundaries, the restart rules and the no-switches rule"
  goodharting_would_be: "a fast v1 that says nothing; citing every retrieved source regardless of quality; capping management's turns instead of fixing why it loops; a demo prompt tuned to pass; a theme that only passes a screenshot"

homotopy:
  realism_axis: "d@e CLI prompt -> d@e UI prompt -> b@c (a second ordinary computer) -> the owner's own computer (read-only proof: the owner runs the prompt)"

boundaries:
  authority: "owner, 2026-11-09 (this mission; start tonight); owner 2026-11-08 for the layered fleet and the delegate grant to d@e, b@c and the owner's computer"
  test_computers: "d@e and b@c only; the owner's computer and the paper are read-only"
  models: "crouton-alpha first for almost everything; Synthetic DeepSeek used seldom, as the timing benchmark (owner, 2026-11-09)"
  delegate: "the Sonnet delegate writes code we keep (themes, the background split), not tests; it shares the owner's Claude Max subscription; 50M tokens a day per computer for now"
  mutation_classes: "UI fixes orange; pool routing, vmctl ring and the delegate grant red; research selection orange; the tool-loop stop rule red; Texture media blocks red (Texture canonical writes)"
  tempo: >
    Aggressive, expedite mode (owner, 2026-11-09: "we're going to be on
    expedite mode for everything until we do the auto paper improvements
    mission. And then ... the initial auto radio mission ... once we get
    auto radio really solid, only then do we really start slowing down with
    the progressive delivery, because that's the point where we're going to
    be launching to real users"). Every landing commit carries
    Ring-Expedite; the expedite keeps planned marker-backed refreshes, the
    live-capsule, apply and skew refusals, the health guards and automatic
    rollback, and holds R0 and the paper until each is seen healthy.
  invariants: [no switches, the restart rules, no secrets or owner addresses in the repo, the paper never asks for human approval]
  not_in_scope: "the wait-time redesign (docs/problems/activation-wait-times-2026-11-09.md options 1-5 await the owner's choice); the Autopaper source audit and images (the next mission)"

now:
  status: complete
  slice: >
    Stations in order (each lands through the landing loop; parallel where
    independent):
    L1 pool routing by availability — landed ead8873b6 (Devin ACCEPT), deploy in flight;
    L2 prompt bar, Sources while revising, search providers in Sources — built 1adebd1e2 (epoch 115), Devin review running, then push;
    L3 layered image on every computer, every tool layer attached, delegate granted to d@e and the owner's computer — built e6efe56c5, tier 2 review (Devin and a Sonnet seat) running;
    L3.5 tools load and unload live (owner, 2026-11-09: "a computer could be running without a particular tool. Then you load that tool, and only then it goes into memory, and then you use it, and then you could even offload that tool and clear it from memory without restarting the computer ... keep it secure as well"). Load: the slot boots on its empty placeholder; a request from the guest names a tool from its fixed profile; vmctl (the authority, host side) verifies the layer against the release manifest and swaps the slot's backing file through Firecracker's drive PATCH; the guest mounts it read-only at the tool's own mount point, outside the store overlay whose lowers cannot change live. Unload: the guest stops the tool's processes, unmounts (its page cache is freed with the mount), and vmctl swaps the placeholder back. Hibernate binds the drive set as it stands. Security: the guest can only ask; vmctl decides from the computer's profile; layers are read-only, digest-checked erofs from the signed release; the delegate's spend stays the gateway grant. The design generalizes to desks and Library packages (owner, 2026-11-09: "a video editing desk, or something for Blender or CAD ... How could this work with packages from the library as well?"): a tool is itself a package (a host-built, signed, content-addressed erofs layer); a desk's manifest names the tools it needs; installing the desk passes the tools through install review; the desk takes a tool by lease and the lease's end unloads it; slots are a fixed set of generic drives assigned at load, not one drive per tool; the computer's tool profile comes from its installed packages, replacing the node-b.nix list. This build delivers the first-party mechanism; tool packages from the Library are its next station. Design first, then build, proved on d@e: a cold boot reads no tool bytes; claude and the browser load within a bounded wait, run, unload, and guest memory returns to its pre-load reading; a request for a tool outside the profile is refused; hibernate and wake keep the loaded set;
    L4 Synthetic DeepSeek benchmark at low, medium, high on d@e (the latency budget);
    L5 research: interim packets enforced, a visible read ledger, a pack floor (problem doc 1.5 items 1-3, 3.2 item 1);
    L6 tool loop stop rule: identical-cell detector plus a visible stop fate (problem doc 2.4 item 1);
    L7 engineering prompt -> PDF -> Reader on d@e;
    L8 Texture media blocks (images, charts, PDF and file artifacts, YouTube and audio);
    L9 delegate proof on b@c (OLED Black round, trajectory 9a29063b), then the remaining theme rounds and the background slot (a platform self-development round on b@c, carried to main through GitHub, then backgrounds as packages).
  conjecture:
    claim: "most of the slowness and the runaway are one substrate: the pool walks dead members and a looping thread never stops, and research reports late; fix routing, the stop rule and interim packets, and an ordinary prompt reaches a good cited v2 in about a minute"
    test: "after L1, L5 and L6, re-run the acceptance prompts on d@e and compare per-hop timings, calls and cited/read ratios with the 202 s baseline"
    edge: "missing_oracle: the owner's computer cannot be read; d@e may differ in pool state"
    delta_o: "per-desk call and token counts and per-hop timings, recorded with each deployed proof"
    scope: "d@e and b@c"
  believed_state: "Stations L1-L9 landed and deployed (L3.5 phase 1 eviction; tool packages are its next station). The conjecture test passed on d@e at eaf66a1aa: \"What's latest in ai?\" reached a cited v2 in 78 s (baseline 202 s) and its last revision in 126 s (L4 crouton 296 s), 24 cite marks. The owner's added features landed and were proved on staging: appearance flow; account switcher (deployed proof passed); restart to update (b@c current, owner-act refusal, d@e refreshed while busy at 15:45Z); sandbox pause (drill passed on d@e at 1f2493123, 16:14-16:17Z: the owner update answered 409 pausing, then 201, and capsule-aadf2f61 resumed after the platform update with its upper of 25 files, 104040 bytes, and kept working)."
  main_uncertainty: "whether the speedup holds across prompt kinds (one run, one prompt); pause is proved for one capsule on one release change"
  next_observation: "none for this mission; the Autopaper improvement mission is next"
  next_action: "mission complete; the live station is docs/definitions/choir-autopaper-improvement-2026-11-09.md, then Autoradio"
  ledger: docs/definitions/choir-ordinary-prompt-path-2026-11-09.ledger.md
---

# The ordinary prompt path

Goal string: `/goal docs/definitions/choir-ordinary-prompt-path-2026-11-09.md`

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
- The pool should route by availability, so a turn goes straight to the member that works ("Crouton Alpha for almost everything ... Synthetic DeepSeek ... seldomly ... as a benchmark").
- Benchmark Synthetic DeepSeek at low, medium and high reasoning (no max).

Owner, 2026-11-08: "d@e is the only computer on the new layered guest
image .. We need to get every computer updated. And yeah, we should let D at
E and B at C and my computer all have access to the Claude delegate." And
on tool layers: a tool "should be something which is not loaded when it's
not needed". An attached layer is a read-only block device that costs no
page cache until a process reads it, so every computer gets every layer and
the delegate's authority stays the gateway relay's grant list.

Self-development acceptance prompts (owner, 2026-11-09): new themes, built
by self-development with the Sonnet (Claude) delegate inside the round:
- **Vector Heart.** The Designers Republic-derived aesthetic Evan Collins named. It means the visual style, not literal vectors and hearts. The existing "vectorheart" preset with hearts stays as is, the "cute" one.
- **Neo Vector Heart.** The Designers Republic take on it.
- **Metal Heart.** The related aesthetic.
- **Utopian Scholastic.**
- **OLED Black.** As black as possible while staying readable and usable, to minimise power on OLED screens.
- **Hybrids.** Mixes of these with Frutiger Aero and Liquid Metal, "in a cool way".

Themes must be separate from the desktop background. The background is an
image or a graphic design that the owner uploads or picks from Files; a
theme does not set it. Checked 2026-11-09: it is not separate today. Presets
are hard-coded in frontend/src/lib/theme.ts and the desktop reads the
background from the theme (Desktop.svelte, var(--choir-bg)). The split needs
one platform change, a background slot, which is itself a test of how
modular the shell is: a platform self-development round on b@c proposes it,
the reviewed diff is carried to main through GitHub, and backgrounds then
ship as packages.

## Why this comes before the Autopaper mission

The Autopaper improvement mission (choir-autopaper-improvement-2026-11-09.md,
drafted for the owner's morning review, decisions D1 to D13) audits the
paper's existing sources and then adds images: charts and maps from cited
data, free-licensed subject photos, and a source photo as a link card. Its
images render through the same Texture media blocks this mission builds
(L8), its source selection shares the research read ledger (L5), and its
desks run on the routing (L1) and stop rule (L6) landed here. The paper
itself is read-only in this mission.
