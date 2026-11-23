# App Privacy label answers for Autoradio 1

Still open for the owner:

- Whether a provider's training on request text counts as a further purpose on the label beyond App Functionality (see the note under the table). The owner picks the conservative answer.
- The contact email and publisher details printed in the policy the label links to (CONTACT_EMAIL).

Rulings applied (owner, 2026-11-23): one app, so mail and takes ship and their rows are Yes; during the beta every account is free tier and providers may train on what is sent; minimum age 16; account created with an email address and a passkey inside the app. The publisher is Choir Harmonic Intelligence Platform, Inc.

For App Store Connect, App Privacy. Apple's test (read 2026-11-23, in docs/research/app-store-automation-2026-11-23.md): data is "collected" when it leaves the device and you or a partner can reach it for longer than needed to serve the request in real time. Data processed only on the device is not collected. Our servers store what a listener's computer holds, and outside model providers receive request text, so those types are collected. Evidence rows refer to data-inventory.md and carry the same file and line.

Tracking: No. No data is used to track people across other companies' apps or sites. There is no advertising or analytics SDK (KIT Package.swift:12-23), no AppTrackingTransparency, and the privacy manifest says tracking false (LAB/PrivacyInfo.xcprivacy:5). Do not add a tracking domain list.

Third-party partners: the AI model and search providers in the policy process data for us as service providers. Apple asks that data collected by third-party partners in your app be declared as collected. The types below already include it.

## Answers, type by type

"Linked to the user" = yes for every type collected, because it lives on the account's computer or is tied to the account. "Used for tracking" = no for every type.

| Apple category and type | Collected? | Linked | Tracking | Purposes | Reason and evidence |
|---|---|---|---|---|---|
| Contact Info: Email Address | Yes | Yes | No | App Functionality | The account is made in the app with an email address and a passkey, and requires the email. internal/auth/store.go:29-33. The research doc's "only if the account keeps one" is wrong; the code wins. |
| Contact Info: Name, Phone Number, Physical Address, Other | No | | | | Not asked. |
| Health and Fitness | No | | | | None. |
| Financial Info | No | | | | None. No purchases in version 1. |
| Location: Precise or Coarse | No | | | | The app asks for none. The programme request sends a time zone name and local time (internal/agentcore/api_autoradio_programme.go:181-182). Apple does not list a time zone as location. [CHECK: owner to accept this reading.] |
| Sensitive Info | No | | | | Not asked for. The listener may write anything in a note or say anything to the radio. We do not label or use it as sensitive info. [CHECK] |
| Contacts | No | | | | Not read. |
| User Content: Emails or Text Messages | Yes | Yes | No | App Functionality | The Mail feature holds the listener's messages on their computer (LAB/mail.ts:148-157). Mail ships in the one app. Whether the phone also keeps a copy does not change the answer: the messages are on our server. |
| User Content: Photos or Videos | No | | | | None. |
| User Content: Audio Data | Yes | Yes | No | App Functionality | A take uploads recorded voice (LAB/takes.ts:323). Microphone audio that is recognised and not uploaded is not collected. Apple's rule says on-device-only data is not collected (KIT/AppleStages.swift:60). The phone also keeps each turn's audio in its own storage (finding F6); that is on the device only. |
| User Content: Gameplay Content, Customer Support | No | | | | |
| User Content: Other User Content | Yes | Yes | No | App Functionality | The words of a spoken or typed request (stored on the tape, internal/agentcore/api_autoradio.go:472), notebook entries, documents, imported PDF and EPUB files, files. Also the request text and page text sent to model providers. |
| Browsing History | No | | | | The app does not send which web pages the reader opens to us (inferred; the reader fetches pages from the phone, LAB/app.ts:59). [CHECK] |
| Search History | Yes | Yes | No | App Functionality | Searches of the paper by the listener's words reach our server (docs/reference/AUTORADIO-TURNS.md, "The paper is searched"; LAB/search.ts:867). Podcast searches go straight to Apple and not through us (LAB/podcast-model.ts:221-222), so they are not ours to declare; we still name Apple in the policy. [CHECK] |
| Identifiers: User ID | Yes | Yes | No | App Functionality | The account id and the device's sign-in key id. internal/auth/store.go:29, :94-105. |
| Identifiers: Device ID | No | | | | No IDFA, IDFV or hardware id is read. The sign-in device label is a fixed text (KIT/Choir/ChoirSignIn.swift:28). |
| Purchases | No | | | | None. |
| Usage Data: Product Interaction | Yes | Yes | No | App Functionality; Personalization | What played, finished, skipped, removed, with times; "more" and "less" taps; edition topics; podcast subscriptions and now playing. LAB/runsheet-store.ts:7; LAB/steer.ts:1-15; LAB/podcast.ts:257; internal/agentcore/api_autoradio_programme.go:182-185. |
| Usage Data: Advertising Data | No | | | | No ads. |
| Usage Data: Other Usage Data | No | | | | |
| Diagnostics: Crash Data | No | | | | No crash reporter in our code. Apple's own TestFlight and App Store crash reports reach us through Apple and are Apple's collection. |
| Diagnostics: Performance Data | No | | | | None sent. Phone logs stay on the phone and carry counts only (KIT/DeskContext.swift:38-40). |
| Diagnostics: Other Diagnostic Data | Yes (conservative) | Yes | No | App Functionality | Server log lines carry a truncated IP (first two parts), the first eight characters of the user id and a session hash (internal/auth/access_log.go:12-39). The gateway logs the computer id with each search (internal/gateway/handlers.go:922). This is operational logging, not app diagnostics sent by the app; declare it if the owner wants no argument with the reviewer. [CHECK] |
| Other Data | No | | | | |

Summary for the form: collected and linked, not used for tracking: Email Address, User ID, Emails or Text Messages, Audio Data (takes), Other User Content, Search History, Product Interaction and Other Diagnostic Data.

Third-party AI providers and training: during the beta every account is on the free tier and some providers may keep request text and train on it. Apple's form asks the purposes for which data is used, including by partners. The rows above say App Functionality. [CHECK: whether provider training needs a further purpose on the label. If in doubt, declare it. Nothing here is used for tracking or advertising.]

Age: the minimum age is 16 and the age-rating answers give 16+. The app has no age gate and asks for no birth date (data-inventory.md section 1).

## Things the form does not capture, and the policy must still say

- The IP address of the phone is seen by Hugging Face when a model file downloads, by Apple when a podcast search runs, by publishers when a podcast loads, and by our server on every request. Apple has no data type for an IP address in transit. Say it in the policy (done in privacy-policy.md section 5).
- Free-tier model members in the computer's default pool, and that some providers may keep and train on request text (finding F3, settled by the owner's ruling: the policy and the consent screen say so).

## What changes when the consent screen and in-app deletion exist

| Event | Label change |
|---|---|
| The consent screen ships | None. Consent does not change whether the data is collected. It satisfies guideline 5.1.2(i), which asks for explicit permission before data is shared with third-party AI. |
| In-app deletion ships | None to the label. It satisfies 5.1.1(v). The policy's deletion section stops needing [WHEN-BUILT]. |
| The kept-audio file (finding F6) is no longer written | Nothing on the label; the policy paragraph in section 3 changes. |
| A crash reporter or analytics tool is added | Add Crash Data, Performance Data or Usage Data and update the manifest. |
| Page text stops being sent to models unless the request refers to it | Other User Content stays; the policy sentence in section 2 narrows. |
| The embedding or voice models move to our own host | Hugging Face leaves the policy table. No label change. |

## Privacy manifest

LAB/PrivacyInfo.xcprivacy declares tracking false and four required-reason API categories. It declares no collected data types. Add NSPrivacyCollectedDataTypes to match the table above so Xcode's privacy report agrees with the label. The research doc marks this as not enforced for an app's own manifest and worth doing for honesty.
