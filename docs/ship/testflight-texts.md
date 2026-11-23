# TestFlight texts for Autoradio 1

Written for the app as it will be at submission. [WHEN-BUILT] marks the one thing that is not true of today's code: the in-app passkey account creation and deletion, the consent screen and the report action are all built overnight and must be checked on the build before anything is pasted. Today's app signs in to an existing account through the device flow. The Lab and Trace pages, fixtures and stage names must be out of the release build before any of this is sent (research risk 4). Remove the tags before pasting into App Store Connect. Contact fields use CONTACT_EMAIL.

Still open for the owner:

- The contact email, the contact name and the phone number Apple may call (placeholders below).
- The final privacy policy URL.
- Whether to give Apple a demo account if sign-up is turned off at the host limit (see the review notes).
- Whether model files move to our own host (the review notes name huggingface.co until then).

Rulings applied (owner, 2026-11-23): one app with every page; passkey account creation and sign-in inside the app, no codes; minimum age 16; model downloads only after agreement, with sizes; the microphone asked for only when voice or a take is turned on; the publisher is Choir Harmonic Intelligence Platform, Inc.

## 1. Beta app description (what testers see)

Autoradio 1 is a radio that reads a news paper aloud. The paper is called Autopaper. Its stories are written by AI agents from cited web sources, and every story links to its sources.

You can press Play and listen with no account. If you make an account with an email address and a passkey, you get a personal edition, you can talk to the radio, and you get mail, files, a reader, podcasts, takes and a notebook. Each account has its own computer on our server. You must be 16 or older.

This is a test build. During the beta every account is on the free tier. What you say or type to the radio goes to outside companies that run AI models, and some of them may keep it and train on it. Do not put anything in the app that you would not want used that way. Some pages are new and some things will break. Please tell us what you hear, what you cannot do, and what is confusing. Use the Send Beta Feedback button in TestFlight.

(Count: about 160 words, about 960 characters. Limit: 4,000 characters.)

## 2. What to Test, first build

Plain sentences, no symbols, no emoji, no angle brackets. Per build and per locale.

```
This is the first build of Autoradio 1. Please try these things.

Open the app and press Play without signing in. Check that a story is read aloud within about a minute and that the lock screen shows the title and the controls.

Open a story and tap its sources. Check that each source opens.

Ask the radio a question while signed out. Check that the app asks you to sign in or make an account.

Make an account with your email address and a passkey. Then listen for a few minutes and tap more of this or less of this on a story.

Turn on Voice and ask the radio a question about the story you are hearing. The app asks for your agreement the first time, and asks for the microphone only now. Tell us if the answer was wrong or slow.

Lock the screen with Voice on and say something. Tell us what happened.

Try mail, files, the reader, podcasts, a take and the notebook. Tell us what you could not do.

When you have finished, delete your account in Settings, Account. Your access ends at once and everything is removed after 72 hours. Tell us if anything was unclear.

Please report crashes, silence, wrong stories and anything you could not understand.
```

[WHEN-BUILT] on the sentences about making an account, the first-time agreement and deleting the account.

## 3. Beta review notes (for Apple's reviewer)

Limit: 4,000 bytes in App Review Information. The block below is about 3,600 bytes with the tags in it. Repeat the contact fields in the form fields Apple provides.

```
WHAT THE APP IS
Autoradio 1 is a radio that reads a news paper aloud. The paper, Autopaper (choir.news), is written by AI agents from cited web sources, and every story shows its sources as links. Without an account the app is listening only. With an account the listener gets a personal edition, can talk to the radio by voice, and has mail, files, a reader, podcasts, takes and a notebook on a computer that belongs to the account. Minimum age is 16.

SEE IT WORK IN ONE MINUTE, NO ACCOUNT
1. Open the app. The public front page loads. No permission prompt appears.
2. Tap Play. The first story is read aloud with the system voice. The lock screen shows the title and the controls.
3. Open any story. The sources are listed with links.
No model download is needed for this. The app plays with the system voice if no model file is present. If you ask the radio something while signed out, the app asks you to sign in or make an account.

CREATE AN ACCOUNT AND DELETE IT [WHEN-BUILT]
No review account or code is needed. Tap Make an account on any prompt that asks for one. The app shows the phone's own passkey screen, where you make an account with any email address and a passkey. There is no password and no code. To delete the account, open Settings, Account, Delete account, and confirm with the passkey. Access ends at once and everything is permanently removed after 72 hours; signing in again within 72 hours cancels it.

MICROPHONE AND SPEECH
The microphone is used only for Voice and takes. Voice is off when the app opens and starts only when the listener taps the Voice control. The app asks for microphone and speech permission at the moment Voice or a take is first turned on, not before. Speech is recognised on the device using Apple's on-device recogniser, and audio is not sent to our servers, except in a take the listener publishes. Voice can stay on with the screen locked for up to three minutes after the listener last speaks. The iPhone shows its microphone indicator while it is on.

AI AND CONSENT
The text of a request, with the page open, recent conversation, topics and mail or files the listener asks about, is sent from the account's computer to outside AI model companies. During the beta every account is on the free tier and some providers may keep this and train on it. Before the first such request the app shows this in plain words and asks for explicit agreement [WHEN-BUILT]. The listener can say Not now and listening still works. The agreement can be withdrawn in Settings [WHEN-BUILT]. A paid tier with stronger protections is planned.

BACKGROUND AUDIO
The app uses the audio background mode so that spoken news continues with the screen locked, with lock-screen controls.

AI-WRITTEN CONTENT
Stories are written by AI from the cited sources and are labelled as such. Each story lists its sources. A listener can report a problem with a story from the story page [WHEN-BUILT].

MODEL FILES
The app can download speech and language model files after install, only after the listener agrees, and it shows the sizes first. They are data files that the app's own code reads. No executable code is downloaded and no feature is added by them. The files come from huggingface.co.

OTHER NOTES
The app is built with a native cross-platform framework. It does not display a web page as its interface. There are no purchases and no advertising. Podcast search uses Apple's podcast search service and podcasts stream from publishers. Publisher: Choir Harmonic Intelligence Platform, Inc.

CONTACT
CONTACT_EMAIL, and a phone number that is answered: PHONE_NUMBER_PLACEHOLDER.
```

[CHECK: if sign-up is turned off at the host limit while Apple reviews, a reviewer cannot make an account. Then give Apple a demo account in the username and password fields. Leave the notes as written while sign-up is open.]

## 4. Feedback email and other TestFlight fields

- Feedback email: CONTACT_EMAIL.
- Marketing URL: https://choir.news.
- Privacy policy URL: the published policy page (required for external testing). Final URL still open.
- Beta app review contact: first name, last name, phone and email of the person who answers Apple. Leave as placeholders here: CONTACT_NAME_PLACEHOLDER, PHONE_NUMBER_PLACEHOLDER, CONTACT_EMAIL.
- Sign-in required: leave unticked while the signed-out radio is the core of the build. [CHECK: tick it only if every reviewed feature needs an account.]
- Export compliance: the app uses encryption built into the operating system (HTTPS, Keychain, AES-GCM from CryptoKit if a local mail copy is kept). Answer as exempt. The research doc suggests setting ITSAppUsesNonExemptEncryption to NO in the build; whether Exact accepts that key is unconfirmed.

## 5. Facts behind the notes, and what to fix first

| Sentence in the notes | True today? | Evidence |
|---|---|---|
| Open the app, public front page loads, no prompt | Built for the simulator, not seen on a phone | docs/current-architecture.md:124 |
| Tap Play reads with the system voice | Built; the default speaker uses the system voice while the Kokoro model is missing | KIT/Choir/ChoirApp.swift:12-13; docs/current-architecture.md:156 |
| Speech recognised on device | Yes, by default | KIT/AppleStages.swift:43, :60 |
| Microphone asked only when Voice is turned on | Yes in code; the README's first-run list says it is asked at Play, which is stale | KIT/LabRuntime.swift:121, :174; mobile/README.md:178 |
| Voice off at launch | Yes | KIT/HandsFreeSwitch.swift:25, :28 |
| Voice can stay on with the screen locked up to three minutes | Yes in code (not proven on a phone) | app.ts:48; HandsFreeSwitch.swift:25 |
| Account creation and deletion in the app | Not yet; ordered built overnight (2026-11-23) | route grep, no deletion route in internal/auth; docs/design/account-create-delete-2026-11-23.md |
| Consent screen | Not yet; ordered built overnight | none in LAB or KIT |
| Model files are data | Yes | KIT/KnownModels.swift; the app runs them with code already in the binary |
| Size and prompt before download | Partly: voice setup states the size and asks on mobile data; the embedding model downloads on Wi-Fi with no prompt (the ruling is: only after the person agrees) | KIT/VoiceSetup.swift:179-182; LAB/retrieval-run.ts:233-243 |
| Report a story | No | none found |
