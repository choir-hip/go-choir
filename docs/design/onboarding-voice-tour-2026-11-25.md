# Autoradio first run: a radio tour while the models download

Owner direction, 2026-11-25:

> “we should start by playing autoradio, then slide the onboarding interview in
> … as next radio segments”

Status: **deferred design**. The submitted build includes EmbeddingGemma 2
in the first-run download set and starts its fetch when Download is pressed.
The radio-first tour is not in that build. The first-run experience still
walks through screens for setup, microphone, hearing and voice choice,
before letting the listener choose topics.

## The tour

1. **Play radio first.** Use the bundled Tiny Voca voice, without waiting for
   the other models. Once learned, an explicit pause or the listener's saved
   place takes priority on later launches.
2. **Put the tour in the programme.** Use short segments that explain what the
   radio does, check that the listener can hear it, offer the same sentence
   in F2 and M2, and ask what topics the listener cares about. Previous and
   Next let the listener revisit or skip a segment without losing progress.
3. **One download action.** Pressing **Download** is consent for the stated set
   of required models, including EmbeddingGemma 2. Use the shared background
   download queue with resumable transfers and accurate progress, Wi-Fi by
   default. Ask before using mobile data. Model names and detailed progress
   are available on demand rather than the focus of the tour. The radio keeps
   playing if the listener defers the download.
4. **Voice questions when listening is ready.** Ask spoken questions after
   the microphone has been granted and the listening models are ready. Until
   then, keep the audio tour useful and offer equivalent tap responses. The
   tour never waits on a spinner.

## Implementation boundary

Exact owns the tour, segments, questions, queue and saved progress. Native
code provides audio, speech, microphone and on-device model execution.
Reuse the shared download subsystem; do not add a second downloader to the
tour. This replaces the presentation of `mobile/AutoradioLab/onboarding.contract`
and the step machine in `onboarding.ts`, without adding a tour selector.
