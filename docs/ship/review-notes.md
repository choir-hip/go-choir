# App Review notes for Autoradio 1 (the App Store version)

Paste the block under "Notes" into App Review Information, Notes (limit 4,000 bytes; this one is checked below). The sign-in fields, contact fields and attachments are separate. Everything marked OWNER is for the owner to fill in the form; no password or personal contact is in this repo.

## Fields beside the notes

- Sign-in required: yes (the review account is provided; the radio itself plays with no account).
- User name: REVIEW_USER_PLACEHOLDER (OWNER: the name given to `demo-account credential --user`, for example "reviewer"). Password: REVIEW_PASSWORD_PLACEHOLDER (OWNER: printed once by `demo-account credential`; runbook `docs/ship/demo-account.md` on lane/demo-account; the credential is turned on by the owner on Node B before Apple opens the app, and `demo-account off` turns it off after review).
- Where the reviewer types them: on the app's sign-in screen, the small "Reviewer sign-in" link at the bottom opens a page with a user name and a password. No passkey, email address or code is involved.
- Contact: first name and last name OWNER; email 000@choir.news; phone OWNER (a number that is answered).
- Attachment: none needed.

## Notes (paste)

```
WHAT THE APP IS
Autoradio 1 is a radio that reads a news paper aloud. The paper, Autopaper (choir.news), is written by AI agents from cited web sources. Every story is labelled "Written by AI" and lists its sources as links. Without an account the app is listening only. With an account the listener gets a personal edition and can talk to the radio by voice. Mail, files, a reader, podcasts, takes and a notebook sit on a computer that belongs to the account. Minimum age is 16.

SIGN IN WITH THE REVIEW ACCOUNT
On the sign-in screen tap "Reviewer sign-in" at the bottom. Enter the user name and password in the sign-in fields of this form. This account works even when public sign-up is closed (sign-up has a capacity limit). It cannot send mail or delete itself.

SEE IT WORK, NO ACCOUNT
1. Open the app. The front page loads. No permission prompt appears.
2. Tap Play. The first story is read aloud with the system voice. Lock the screen: the lock screen shows the title and controls.
3. Open a story. Its sources are listed with links.
Asking the radio a question while signed out opens a sheet asking you to sign in or make an account.

WHERE EVERYTHING IS (signed in)
Radio and Paper: the first page. Voice: the microphone control on the radio. Edition topics: Settings or the topics page. Reader, Podcasts, Mail, Files, Takes and Notebook: the launcher (the grid button). Settings: Account (sign out, Delete account), Voice and models (try, switch or remove models), Appearance, Computer, About.

CREATE AN ACCOUNT AND DELETE IT
Tap Make an account. Enter any email address; we mail an 8-digit code, type it, then the phone's passkey screen makes the passkey. There is no password. The line above the Create account button says the person is 16 or older, agrees to the Privacy Policy and Terms, and that what is asked of the radio is sent to outside AI companies that may use it to train their models during the beta. To delete: Settings, Account, Delete account, confirm with the passkey. Access ends at once and everything is removed after 72 hours; signing in within 72 hours and choosing Cancel deletion stops it. The review account cannot be deleted; its delete page says so. To see deletion, make a new account.

MICROPHONE AND SPEECH
Used only for Voice and for recording a take. The prompts appear when the listener first turns Voice or a take on, never at launch. Speech is turned into text on the phone; audio is not sent to our servers, except a take the listener presses Publish on. Voice is off at launch and turns itself off after three minutes without speech. The microphone closes when the screen locks.

AI AND DATA
The text of a request, the page open, recent conversation, topics and any mail or files asked about go from the account's computer to outside AI model companies. During the beta every account is free tier and some providers may keep this and train on it. The create-account screen says so; asking needs an account.

BACKGROUND AUDIO
UIBackgroundModes audio keeps spoken news playing with the screen locked, with lock-screen controls. Nothing else runs in the background.

MODEL FILES
After install the app can download speech and language model data files from huggingface.co. Voice setup recommends the models that fit the phone, shows their total size, and downloads only after the listener agrees. Settings, Voice and models, lets the listener try, switch or remove models. They are data read by code already in the app; no executable code and no feature arrives with them. The app plays with the system voice without them.

OTHER
Built with a native cross-platform framework; the interface is native, not a web page. No purchases, no advertising, no tracking. Podcast search uses Apple's podcast search; episodes stream from publishers. A take is a voice clip of 3 to 60 seconds that is published only when the listener presses Publish and can be deleted by the listener. Publisher: Choir Harmonic Intelligence Platform, Inc. Contact 000@choir.news.
```

(The block is 3923 bytes, under the 4,000 limit; recount after any edit.)

## Facts behind the notes (each checked against the code, 2026-11-24)

| Sentence | Source | State |
|---|---|---|
| "Written by AI" label on stories | `reader-parse.ts` DISCLOSURE_STORY (main) | built; lead sees it on the build |
| Reviewer sign-in link; the review account cannot send mail or delete | lane demo-account: `internal/auth/demo_account.go`, runbook: key scopes exclude `owner:acts`, mail send is refused to every key; mail send is refused to every key. A take is NOT withheld (the runbook says so), so the note does not claim it | built on the lane; not run on Node B (runbook residual) |
| The review account's delete page says it cannot be deleted | runbook residual: the delete route must refuse `IsDemoAccount` and the page must read `whoami reviewer`, "one line each", to land with lane account-delete | **verify on the build before upload; if absent, cut that sentence** |
| Sign-up has a capacity limit | `docs/changes.md` "Sign-up is bounded by the host's room" (main) | built |
| 8-digit code then passkey, no password | `app-signin.ts`, `onboarding.contract` (main) | built; seen on a simulator drive only |
| Settings: Account, Delete account; 72 hours; Cancel deletion | lane account-delete `settings.contract` ("sign in with your passkey in that time and choose Cancel deletion") | built on the lane |
| First-run voice setup (recommended models for the phone, one agreement with the total size; `offers.ts`) and the Voice and models page | lane voice-models `settings.contract` (nav row "Voice and models"), `consent.ts` per-model agreement | built on the lane |
| Mic closes when the screen locks; audio background mode only | `b53276cba`; `mobile/AutoradioLab/app.json` backgroundModes audio | built; not proven on a phone |
| Voice off at launch, off after three minutes without speech | `HandsFreeSwitch.swift:25` (180 s) | built |
| Podcast search is Apple's | `podcast-model.ts:222` | built |
| Lock screen shows title and controls | `LiveAudio.swift`, `RemoteAudioPlayer.swift` use the now-playing center | built; not run on a phone |

Risks the reviewer may raise, and the answer to hold ready:

- Guideline 1.2 (user-generated content): takes are user voice clips offered to other listeners. The build has no report or block action for a take (grep of `takes.ts` and `members-takes.contract`, 2026-11-24). Either cut takes from the build, or add a report action, or accept the risk with the owner's say. This file does not claim moderation.
- Guideline 5.1.1(v): deletion must be reachable by the reviewer. If public sign-up is closed, the reviewer can use the review account but cannot reach the deletion flow with it. Keep sign-up open during review, or say so here.
- 4.2 minimum functionality: no; the app has many pages. 2.5.2 downloaded code: the note says data files only.
