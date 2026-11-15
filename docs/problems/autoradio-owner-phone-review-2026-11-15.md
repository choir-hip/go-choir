# Autoradio on the owner's phone: the first review (2026-11-15)

Recorded 2026-11-15, from the owner's message and six screenshots of the app
on their phone. This is the problem record; no fix is in it. Causes below are
marked as observed or as not yet known. Nothing here has been checked against
the app's code or a trace yet.

## What the owner said

Kept:

- The cards are liked.

Wanted:

1. **Every paper article is a card in a stack.** Swiping goes from article to
   article without a return to the front page.
2. **No horizontal scrolling inside a page.** It is unexpected and it breaks
   card swiping. The menus' design is not liked.
3. **Page turning may be the better idea than card swiping**, with its own
   animation. More animation in general.
4. **Audio does not work, and that is the big problem.** A single story cannot
   be listened to. The purpose is hands-free voice with a DJ that scripts
   stories and other media (podcasts) together; listening to one story is the
   base of that.
5. **The "Prompt Autoradio" bar is confusing.** It is not clear what it does.
6. **Search covers only what was already read.** It should search the whole
   corpus, for example to filter the paper for "Nvidia" or "Israel".
7. **Search and edition personalization may be one feature**: a temporary
   filter on the paper, and a saved or learned default, by the same
   technique. The edition comes from more than a BM25 search, so this needs
   thought before a design.
8. **The voice interview does not work at all.** The hands-free voice work
   from Mike is not in the app. Too many versions were attempted. Go all in on
   hands-free voice as it works in Mike.
9. **Transcription anywhere there is a text block.**
10. **Text to speech for any long text**: Textures from Autopaper or
    elsewhere, reader content (PDF, EPUB), and anything captured from a news
    article.
11. **A source opens in a web view inside the app**, not in Safari.

One sentence of the message ("Do you want to wear a bigger pant home?") reads
as a dictation error and is not interpreted here.

## What the screenshots show (observed)

- The programme is three segments: station break, bridge, station break. No
  story is in it. Known cause for part of this: the phone sends no candidates
  to the programme route (overnight letter, 2026-11-15).
- The interview screen (step 4 of 4) shows the radio's question "What do you
  want to know about every day?" twice under "So far", with no owner turn,
  while the screen says the microphone is open.
- The Mail page is drawn shifted left, with its title and first tab cut off:
  the page scrolls sideways.
- The Paper entry of the deck shows a card with only the word "Paper".
- The search field says "Search pages, files, mail".
- The source sheet has an "Open original" button.

## Belief and remaining error

Belief: items 4, 8, 9 and 10 are one piece of work (the voice loop and text to
speech from Mike, adopted whole), and it comes first because the rest of the
app's purpose rests on it. Items 1, 2, 3 and 11 are navigation work in the
app. Items 5, 6 and 7 are one design question about search, the prompt bar
and the edition.

Not known: why the interview hears nothing and repeats its question; whether
any text-to-speech path exists in the app today; what of Mike's voice work is
portable and what state it is in; which pages scroll sideways and why.

## Owner rulings, 2026-11-15 (after the investigation)

- **The other voice versions are deprecated, not deleted yet**, once Mike's
  loop is in. Deletion waits for the owner's word.
- **Models: a model under 100 MB may be bundled in the app. A larger one is
  downloaded during onboarding.** This replaces the earlier no-bundled-models
  ruling for the phone app.

The investigation is `docs/evidence/autoradio-phone-review-investigation-2026-11-15.md`.
- **The target is confirmed**: hands-free is the default everywhere a voice
  session is open, and one voice engine does conversation, dictation into any
  text field, and reading any long text aloud.
- **"Tap to talk" is removed.**
- **The app is pre-TestFlight and runs only on the owner's phone: hard
  cutover, no backwards compatibility.** No preference migration, no shim,
  no old path kept alive for a transition.
- **Hands-free voice works first; four existing pieces are then worked back
  in gradually**, because they have value: the local Qwen3 model that thinks
  aloud as a holding line; the second set of parts (Silero, Parakeet, Kokoro,
  Pocket TTS, LFM3 audio, the llama.cpp responder); the voice setup and
  configurator with its model store; and the unmerged voice branches (the
  responder ladder). They are kept for later, not merely deprecated.
- **Be practical; the happy path is the main thing** (same day, later). The
  configurator, the model store and the onboarding download are needed and
  stay in use. Fallback voices and more available models are good. The work
  is ordered by what makes hands-free voice work end to end soonest, not by
  removing the other implementations. A model or voice picker is a feature,
  not a switch.
