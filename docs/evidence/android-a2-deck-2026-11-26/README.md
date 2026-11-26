# The Android deck plays, survives the screen, and obeys the media session (2026-11-26)

Device: Motorola `ZY22H5K8QR`, moto g stylus 5G (2023), Android 14 (API 34),
SM6450. Package `news.choir.autoradio.lab.one`. APK SHA-256
`935f285d2c1e91e1451cc883525b30bb80c879abc9536f82c700c61f3a40ae76`, signed
with the lab-one debug certificate
(`dfe9767c642a2f864b297049bde97c0836cb58d5ce516ffe7a7da11f6fec9ea7`).

**What was run.** `scripts/proofs/android-a2-deck.sh` force-stops the app, cold
launches `news.choir.autoradio.lab.one`, and drives the deck through the
system's own media session (no credentials, no test switch in the app):

1. the station starts by itself signed out: `PLAYING`, “Opening”;
2. Home (the phone's own screen): ten seconds later still `PLAYING`, position
   2,901 → 14,938 ms (the position of a spoken story restarts with each player item, so the check accepts a later position or a later item and reports both);
3. `cmd media_session dispatch pause`: `PAUSED`, position held for two seconds;
4. `cmd media_session dispatch play`: `PLAYING` again;
5. `cmd media_session dispatch next`: the entry on air changes from “Opening”
   to the story “ICJ rules Myanmar committed genocide against Rohingya…” (a
   front-page story, read on the phone);
6. force-stop and a second cold launch: the station plays again (`PLAYING`,
   position 0 of its opening).

Raw output is in this directory: `launch.txt`, `playing.txt`, `background.txt`,
`paused.txt`, `played.txt`, `next.txt`, `relaunch.txt`, `relaunched.txt`,
`media-session.txt`, `apksigner.txt`.

## What this proves

The Audio Authority's Android half is a native deck: Media3 renders audio,
on-device speech is synthesised to files and played through the same player,
and a `MediaSessionService` keeps it in the foreground when the activity is
gone. The proof shows real rendered progress (positions), not a prepare/play
return: the background position advanced only while the activity was hidden,
and pausing froze it.

Fixes this proof forced, each found on the phone:

- ExoPlayer no longer requests audio focus itself (`setAudioAttributes(..., false)`);
  the process's `AudioFocus` claim owns it. Two requests from one process made
  the deck hear `AUDIOFOCUS_LOSS` and pause itself at start.
- The media session wraps the deck's forwarding player, so pause/play/next from
  the notification or a headset reach the deck's own rules instead of
  ExoPlayer.
- A media-session `next` is recorded for shared code and started on its next
  pulse; starting the next entry from the native callback was a call made
  outside a data answer and its reply was let go.
- The spoken story is synthesised one chunk ahead while the first plays:
  `TextToSpeech.voices` took 83 s on this phone, and synthesising the whole
  story before the first sound took 42 s; the first sentence now starts in
  about half a second.

## What is not proved

Audio route changes (unplugging headphones, a Bluetooth hand-off), a phone
call's transient focus loss, the notification's own controls by touch, a locked
screen, and long-run battery/thermal behaviour. The story pages themselves are
shared code and are not exercised here beyond the station's own text. iOS and
the web are unchanged.
