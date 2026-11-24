# App Store texts for Autoradio 1 (final, 2026-11-24)

Limits (research doc, read 2026-11-23): name 30 characters, subtitle 30, promotional text 170, description 4,000, keywords 100 bytes. No other platforms' names (2.3.10), no prices (2.3.7), no unverifiable claims, no placeholder text. Every sentence below is true of origin/main plus the lanes landing on 2026-11-24 (exact-rest-a, exact-editor, voice-models, demo-account, account-delete, dev-profile); where a sentence depends on a lane, the line "Depends on" under each block says which. The publisher is Choir Harmonic Intelligence Platform, Inc.

Cut from the earlier draft because it is not built or not true (2026-11-24): the line that the app asks agreement "before the first request" (the agreement is the create-account screen, there is no separate screen: owner, 2026-11-24); "There are no passwords and no codes" (a new account types an 8-digit code mailed to its address: `app-signin.ts`, `onboarding.contract`); "Voice can stay on for up to three minutes with the screen locked" (the microphone closes when the screen locks: `b53276cba`; only the idle-off of three minutes remains, and it is not worth a sentence); "report a problem with a story" (no report action exists); screen 2 stays only if the lead sees it on a device.

Open for the owner (all with a default; see submission-checklist.md): territories and price (default free, United States first); secondary category (default none); whether the model files move to our own host (the text names Hugging Face until then); the support page (section "URLs").

Rulings applied (owner, 2026-11-23 and 24): minimum age 16 (age rating 16+); one app; passkey accounts made and deleted inside the app; model files only after the person agrees in voice setup (one agreement, the total size shown); during the beta every account is free tier and providers may train on what is sent.

## Name

Autoradio 1 (11 characters). Already set on the app record (the record exists, see submission-checklist.md).

## Subtitle (chosen)

```
News read aloud, with sources
```

29 characters. It says what the app does and that sources are named. (Alternatives dropped: "Listen to AI-written news" is also true, but the AI line is in the first sentence of the description and the AI label inside the app.)

## Promotional text (170 characters or fewer)

```
Autoradio 1 reads the day's news aloud. Every story is written by AI from cited web sources and links to them. Listen with no account. Make one to shape your own edition.
```

170 characters. True of the build: the paper plays signed out; a personal edition needs an account.

## Description (under 4,000 characters)

```
Autoradio 1 is a radio that reads a news paper aloud.

The paper is called Autopaper. Its stories are written by AI agents from cited web sources. Every story shows its sources as links, so you can open the original and check it. Stories can contain mistakes. The app says plainly that they are written by AI.

Press Play and listen. You do not need an account. The radio reads the front page with the voice built into your iPhone until you add a better one, and it keeps playing with the screen locked, with controls on the lock screen.

With an account you get more:

A personal edition. Choose the topics you care about, by tapping or by speaking. Tap more of this or less of this on any story and the radio adjusts.

Talk to it. Turn on Voice and ask about the story you are hearing, or tell the radio to skip, pause or play something next. Your speech is turned into text on your phone. The text of a request is sent to your account's computer on our server and to outside AI models that write the answer. During the beta every account is on the free tier, and some of those companies may keep what is sent and use it to train their models. You agree to this when you create your account, and the screen says so above the button.

A reader. Read a story or a document, see its sources and its versions, and hear it read aloud.

Podcasts. Search for shows, subscribe, and listen. Episodes stream from the publisher.

Mail and files. Read and write mail, and keep files on your computer.

Notebook. Keep notes that your computer remembers.

Takes. Record a take of up to 60 seconds. It is shared with other listeners only if you press Publish, and you can delete it.

Your computer. Each account has its own computer on our server. It holds what you make and what you ask the radio to remember.

Sign in with a passkey. There are no passwords. A new account first checks your email address with a code we send, and you make the passkey on your phone. You can delete the account in the app, in Settings. Your access ends at once, and your account, your computer and your data are permanently removed after 72 hours. You must be 16 or older.

What to know before you use it:

Stories are written by AI, not by reporters. The sources are cited and linked. Please check them for anything that matters.

Voice and takes use the microphone only when you turn them on. The iPhone shows its microphone indicator while it is on.

When you set up voice, the app recommends the speech and language models that fit your phone, shows their total size, and downloads them from Hugging Face only after you agree. In Settings, Voice and models, you can try, switch or remove models.

The news can include war, crime and other serious subjects.

Privacy policy: https://choir.news/privacy
```

Depends on: lane account-delete (the deletion sentence, with its 72 hour hold), lane voice-models (first-run voice setup's recommendation and agreement, and the Voice and models page), origin/main (the create-account line above the button, `onboarding.contract`; the "Written by AI" label, `reader-parse.ts` DISCLOSURE_STORY; takes 3 to 60 seconds, `takes.ts`). Not proven on a phone, and the lead must see each on the build before upload: lock-screen controls and playing with the screen locked (docs/current-architecture.md says built, not run on a phone), the Voice and models page, and the deletion path. If the lead cannot see one, cut its sentence.

The support URL goes in its own field only (section "URLs"), so no placeholder is in the description.

## URLs

- Privacy policy URL: `https://choir.news/privacy` (typed into the app info localization). Blocker: that route renders `frontend/public/legal/privacy-policy.md`, effective 2026-10-31, which does not mention the Autoradio app, outside AI companies training on requests, 16+, in-app deletion or the 72 hours, and names `privacy@choir.news` as its contact. The new policy (`docs/ship/privacy-policy.md`, still waiting for the owner's read, with tags and DATE_OF_PUBLICATION) must replace it before submission; the create-account line in the app also links to it.
- Terms: `https://choir.news/terms` (linked from the create-account screen; the route exists in `frontend/src/App.svelte`). Same check: it is the older web terms.
- Support URL: checked 2026-11-24 with curl. Every path on choir.news answers 200 with the same single-page-app shell, including a path that cannot exist (`/zzz-nonexistent-qq`), so a 200 proves nothing. The app only routes `/privacy` and `/terms` (`legalDocumentKindFromPath`), so `/support`, `/help` and `/contact` show no support page. Nothing exists. Proposal: add a `support` kind to `LegalDocument.svelte` beside privacy and terms, served at `https://choir.news/support`, with `frontend/public/legal/support.md`: what the app is, "write to 000@choir.news", how to delete an account (Settings, then Delete account, or choir.news/account/delete) and the 72 hours. Orange (frontend route), the lead's change, with a deployed check that `/support` shows that text. Until it lands, the field is blocked; do not use `/privacy` as the support URL (it is not support, and 1.5 asks for a way to contact).
- Marketing URL: `https://choir.news`.

## Keywords (100 bytes or fewer, comma separated)

```
news,radio,audio news,podcast,headlines,briefing,listen,voice,sources,current events,reader
```

91 bytes (counted 2026-11-24, ASCII only). No name or subtitle words are repeated except "news" inside "audio news": drop "audio news" and add "daily" if App Store Connect warns. No competitor names, no other platforms.

## Copyright

`2026 Choir Harmonic Intelligence Platform, Inc.`

## Categories and ratings

- Primary: News. Secondary: none (owner may choose Reference). Not Music, Entertainment or Kids.
- Age rating: 16+. Answers, one by one, are in submission-checklist.md ("Age rating declaration"). The minimum age for an account is 16; the app asks for no birth date.
- Content rights: "Contains third-party content": yes. Stories cite web sources with links and podcasts stream from publishers' own feeds; the paper's stories are our own AI-written text. Owner confirms he holds or is licensed for what is shown, the one right the form asks him to affirm.
- Export compliance: the build sets ITSAppUsesNonExemptEncryption false (`mobile/scripts/check-store-ipa.py` refuses an upload without it); HTTPS and system encryption only.

## Screenshot plan (iPhone only, one large size)

Capture at 1320 x 2868 on a slim simulator named for the run; Apple scales to smaller phones. No placeholder text, no fixtures, real published stories pinned for the run. Show the app in use, not a title card. No prices, no other platforms. Captions are six words or fewer and sit above the screen on a plain ground.

| # | Screen | What it shows | Caption |
|---|---|---|---|
| 1 | Radio, playing | The Autoradio page with a story on air, the Play control and the up-next list. | The paper, read to you |
| 2 | Lock screen | The lock screen with the title, the progress and the controls. | Keeps playing with the screen locked |
| 3 | A story with sources | A story open with the citation chips and the Sources list visible. | Every story links to its sources |
| 4 | The AI label | The line that says the story is written by AI from cited sources, with a story below it. | Written by AI, from cited sources |
| 5 | Voice | Voice on, the words the radio heard, and the answer starting. | Ask about the story you hear |
| 6 | Edition topics | The topic tree with some topics chosen. | Choose the topics you follow |
| 7 | Podcasts | Search results and a subscribed show. | Podcasts stream from the publisher |
| 8 | Sign in | The in-app passkey screen over the signed-out radio. | Listen first. Sign in later. |
| 9 | Mail and files | The mail list and a file open on your computer. | Mail and files, on your computer |
| 10 | Takes and notebook | A take being recorded, or the notebook with a note. | Notes and takes, yours to keep |

Screens 4 and 8 exist in the build: the label is the "Written by AI" line on a story (`reader-parse.ts`), and the passkey screen is the create-account screen (`onboarding.contract`). The lead's job: capture them from a build, one at a time, on a slim simulator.

If voice, sound, the lock screen, mail, files or takes have not been used on the owner's phone by the time of capture, drop their screens. Show only what has been used. (Research risk 4.)

## Preview storyboard (20 seconds, captured footage only)

Apple allows 15 to 30 seconds, 886 x 1920 portrait, muted by default, captured from the app itself. Captions are allowed; narration is allowed. No people filmed with a device. No prices or dates. Say that sign-in is optional.

| Seconds | Captured footage | Caption |
|---|---|---|
| 0 to 3 | Cold launch. The front page loads on the radio screen. | Open the app |
| 3 to 7 | Tap Play. The first story starts and the passage highlights as it is read. | Press Play. No account needed. |
| 7 to 10 | Lock the screen. The lock screen shows the title and the controls. | Keeps playing when locked |
| 10 to 14 | Unlock, open the story, scroll to the sources and tap one. | Every story links to its sources |
| 14 to 18 | Turn Voice on, say a short question, show the heard words and the answer. | Ask about what you hear |
| 18 to 20 | The radio screen again, playing. | Autoradio 1 |

Encode: H.264 at 30 fps or lower, 10 to 12 Mbps, stereo AAC at 256 kbps. Narration, if any, is mixed in afterwards because the simulator's audio is not captured. Use en-US only for version 1.

## What's New (version 1.0.1 and later)

Plain user sentences from the change log. Not required for version 1.
