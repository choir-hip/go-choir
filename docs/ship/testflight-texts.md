# TestFlight texts for Autoradio 1

Written for the app as it will be at submission. [WHEN-BUILT] marks a sentence that is not true of today's code (in-app account creation, in-app deletion, the consent screen, a review sign-in, the report action). Today's app signs in to an existing account through the device flow and has no in-app creation or deletion. The Lab and Trace pages, fixtures and stage names must be out of the release build before any of this is sent (research risk 4). Remove the tags before pasting into App Store Connect. Contact fields use CONTACT_EMAIL.

## 1. Beta app description (what testers see)

Autoradio 1 is a radio that reads a news paper aloud. The paper is called Autopaper. Its stories are written by AI agents from cited web sources, and every story links to its sources.

You can press Play and listen with no account. If you make an account, you get a personal edition, you can talk to the radio, and you get a reader, podcasts and notes. Each account has its own computer on our server.

This is a test build. Some pages are new and some things will break. Please tell us what you hear, what you cannot do, and what is confusing. Use the Send Beta Feedback button in TestFlight.

(Count: about 112 words. Limit: 4,000 characters.)

## 2. What to Test, first build

Plain sentences, no symbols, no emoji, no angle brackets. About 850 characters. Per build and per locale.

```
This is the first build of Autoradio 1. Please try these things.

Open the app and press Play without signing in. Check that a story is read aloud within about a minute and that the lock screen shows the title and the controls.

Open a story and tap its sources. Check that each source opens.

Make an account in the app and sign in with a passkey. Then listen for a few minutes and tap more of this or less of this on a story.

Turn on Voice and ask the radio a question about the story you are hearing. The app asks for your agreement the first time. Tell us if the answer was wrong or slow.

Lock the screen with Voice on and say something. Tell us what happened.

When you have finished, delete your account in Settings and tell us if anything was unclear.

Please report crashes, silence, wrong stories and anything you could not understand.
```

[WHEN-BUILT] on the sentences about making an account, the first-time agreement and deleting the account.

## 3. Beta review notes (for Apple's reviewer)

Limit: 4,000 bytes in App Review Information. This draft is about 3,300 bytes before the tags are removed. Put the review sign-in in the username and password fields and repeat it here if Apple asks.

```
WHAT THE APP IS
Autoradio 1 is a radio that reads a news paper aloud. The paper, Autopaper (choir.news), is written by AI agents from cited web sources, and every story shows its sources as links. Without an account the app is listening only. With an account the listener gets a personal edition, can talk to the radio by voice, and has a reader, podcasts and notes on a computer that belongs to the account.

SEE IT WORK IN ONE MINUTE, NO ACCOUNT
1. Open the app. The public front page loads. No permission prompt appears.
2. Tap Play. The first story is read aloud with the system voice. The lock screen shows the title and the controls.
3. Open any story. The sources are listed with links.
No model download is needed for this. The app plays with the system voice if no model file is present.

CREATE AN ACCOUNT AND DELETE IT [WHEN-BUILT]
Tap Sign in or Sign up on any page that needs an account. The app opens the system sign-in sheet, where you make an account with an email address and a passkey. There is no password. To delete the account, open Settings, Account, Delete account, and confirm with the passkey. Deletion removes the account, its computer and its data.

REVIEW ACCOUNT [WHEN-BUILT] [DECISION]
If you cannot use a passkey, tap "I have a review code" on the sign-in page and enter the code below. It opens a prepared account that has sample content. The code does not expire.
Review code: REVIEW_CODE_PLACEHOLDER

MICROPHONE AND SPEECH
The microphone is used only for Voice, which is off when the app opens and starts only when the listener taps the Voice control. The app asks for microphone and speech permission at that moment, not before. Speech is recognised on the device using Apple's on-device recogniser, and audio is not sent to our servers. The text of the request is sent to the listener's account computer and is processed by outside AI models. The app asks for explicit agreement before the first request that does this [WHEN-BUILT], and the choice can be withdrawn in Settings [WHEN-BUILT]. If the listener says no, the radio, the paper, the reader and podcasts still work. Voice can stay on with the screen locked for up to three minutes after the listener last speaks, so the radio can be asked a question from the lock screen. The iPhone shows its microphone indicator while it is on.

BACKGROUND AUDIO
The app uses the audio background mode so that spoken news continues with the screen locked, with lock-screen controls.

AI-WRITTEN CONTENT
Stories are written by AI from the cited sources and are labelled as such. Each story lists its sources. A listener can report a problem with a story from the story page [WHEN-BUILT].

MODEL FILES
The app can download speech and language model files after install. They are data files that the app's own code reads. No executable code is downloaded and no feature is added by them. The app shows the size and asks before it downloads [WHEN-BUILT]. The files come from huggingface.co [DECISION: or our own host].

OTHER NOTES
The app is built with a native cross-platform framework. It does not display a web page as its interface. There are no purchases and no advertising. Podcast search uses Apple's podcast search service and podcasts stream from publishers.

CONTACT
CONTACT_EMAIL, and a phone number that is answered: PHONE_NUMBER_PLACEHOLDER.
```

## 4. Feedback email and other TestFlight fields

- Feedback email: CONTACT_EMAIL.
- Marketing URL: https://choir.news.
- Privacy policy URL: the published policy page (required for external testing). [DECISION: final URL]
- Beta app review contact: first name, last name, phone and email of the person who answers Apple. Leave as placeholders here: CONTACT_NAME_PLACEHOLDER, PHONE_NUMBER_PLACEHOLDER, CONTACT_EMAIL.
- Sign-in required: leave unticked while the signed-out radio is the core of the build. [CHECK: tick it only if every reviewed feature needs an account.]
- Export compliance: the app uses encryption built into the operating system (HTTPS, Keychain, AES-GCM from CryptoKit for the local mail copy). Answer as exempt. The research doc suggests setting ITSAppUsesNonExemptEncryption to NO in the build; whether Exact accepts that key is unconfirmed.

## 5. Facts behind the notes, and what to fix first

| Sentence in the notes | True today? | Evidence |
|---|---|---|
| Open the app, public front page loads, no prompt | Built for the simulator, not seen on a phone | docs/current-architecture.md:124 |
| Tap Play reads with the system voice | Built; the default speaker uses the system voice while the Kokoro model is missing | KIT/Choir/ChoirApp.swift:12-13; docs/current-architecture.md:156 |
| Speech recognised on device | Yes, by default | KIT/AppleStages.swift:43, :60 |
| Microphone asked only when Voice is turned on | Yes in code; the README's first-run list says it is asked at Play, which is stale | KIT/LabRuntime.swift:121, :174; mobile/README.md:178 |
| Voice off at launch | Yes | KIT/HandsFreeSwitch.swift:25, :28 |
| Voice can stay on with the screen locked up to three minutes | Yes in code (not proven on a phone) | app.ts:48; HandsFreeSwitch.swift:25 |
| Account creation and deletion in the app | No | route grep, no deletion route in internal/auth |
| Consent screen | No | none in LAB or KIT |
| Model files are data | Yes | KIT/KnownModels.swift; the app runs them with code already in the binary |
| Size and prompt before download | Partly: voice setup states the size and asks on mobile data; the embedding model downloads on Wi-Fi with no prompt | KIT/VoiceSetup.swift:179-182; LAB/retrieval-run.ts:233-243 |
| Report a story | No | none found |
