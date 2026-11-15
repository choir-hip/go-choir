# The podcast clipping work of November 10 was never written down

**Problem.** On 2026-11-15 the owner asked where podcast clipping stood and
the lead answered "almost nowhere", after searching the repo. That was wrong.
A day of research, two paid Exa runs, a hit-rate experiment and a
transcription benchmark were done on 2026-11-10, and reported only in chat.
The lead said then "I'll write this into the podcast design note once the
Autoradio lanes are moving" and did not. No design note exists.

**Evidence** (the session transcript of 2026-11-10, 18:17Z to 21:27Z, and the
build VM).

- Owner's framing, 18:19Z: "we should look for audio content that sources the
  autopaper articles". The reading: an episode is a source; its transcript is
  the capture; a citation to a transcript block that carries start and end
  times is the clip. The citation format needs no change.
- Repo gaps found then: the feed poller drops the enclosure URL, duration and
  transcript tag; nothing fetches audio or transcribes; capture search does
  not search bodies; episodes are not stored.
- Outside facts found then: none of eight news feeds carried a transcript
  tag; all six enclosure URLs tested answered a byte-range request, so a clip
  can play from the publisher's file; ad insertion shifts timings.
- Exa, 18:45Z, $1.09 spent: podcast search is Exa's Agent API with the
  Particle source. Run 1 returned 31 passages from about 25 shows in 3.4
  minutes for $0.78; only 2 had times and none an audio link. Run 2, with
  times required, returned nothing in 64 seconds for $0.31. Exa finds
  episodes and quotes; it does not clip.
- The hybrid proposed then: Exa names the episodes and quotes; we download
  and transcribe only those; the quote is found in our transcript, which
  gives times in the file we hold.
- Hit rate, same message: with six episodes from six shows, 8 of 25 of that
  day's paper stories had a strong matching passage.
- Transcription benchmark, 21:27Z, on the build VM: Parakeet 110M through
  parakeet.cpp (8-bit) ran about 25 times real time on four cores in 0.7 GB
  with word timestamps, 5.3% word error against the largest Parakeet on one
  news episode. Seven other models were measured. The files are still on the
  build VM in the agent's home under `asr-bench` (5.9 GB, with a README).
- Owner rulings that day: playing and clipping podcasts is allowed and the
  rights section is dropped; Exa is used for podcast search only.

**Belief.** The design is mostly decided and the measurements exist; what is
missing is the note and the build.

**Remaining error.** The benchmark's table lives in a chat message and a VM
directory that nothing protects. The figures here are copied from the chat,
not re-measured. The design note is still unwritten.
