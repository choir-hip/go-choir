# Automating App Store and TestFlight submission for Autoradio (research, 2026-11-23)

Read-only research. Nothing was built, run or edited. Every web source was read on 2026-11-23; keys in square brackets point to the list at the end. No project detail was put in any search or request. Marks: **[guess]** is my inference; **[memory]** is from training data and not re-read today; **[unconfirmed]** means I looked and could not settle it.

Repo facts used (checkout `lead-docs2`): the app is an Exact app, id `news.choir.autoradio.lab`, name "Autoradio", iPhone only, iOS 18.0 minimum, `backgroundModes: ["audio"]`, `deploy.store.ios: "0"` (`mobile/AutoradioLab/app.json`). A variant "Autoradio 1" has id `news.choir.autoradio.lab.one`. Signing so far is a free personal team with 7-day profiles; Exact never creates profiles and signs with `EXACT_IDENTITY` and `EXACT_PROFILE` (`docs/reference/exact/build-and-ship.md`). An archive is `bun exact.mjs ios --device --archive out.ipa`; a TestFlight upload has never been tried, and whether Exact's `.ipa` passes Apple's processing is an open question in that file. Usage strings are in `strings/en.json`. A privacy manifest exists (`PrivacyInfo.xcprivacy`, four required-reason categories, tracking false). Models download from huggingface.co after install (`KnownModels.swift`). Signed out, the paper reads and the radio reads the public front page aloud (`docs/current-architecture.md` section 6, built, not yet seen on a phone). The auth service has passkey registration and no account-deletion route that I could find (`internal/auth`, by route grep only).

## 1. First submission checklist

A = an agent can do it; H = a human in the browser; D = owner decision.

| # | Item | Who | How |
|---|---|---|---|
| 1 | Agreements, tax, banking | H | App Store Connect > Business. The Account Holder is the only role that signs agreements [R]. The free-apps agreement is enough for a free app; the paid-apps agreement needs tax and bank forms. A private-API attempt to accept agreements returned "fresh 2FA required" in the CLI's own notes [N]; do not automate. |
| 2 | EU trader status (DSA) | H, D | Required to distribute in the EU since 2025-02-17 [U]. A trader's address and phone are shown publicly. Needs identity verification [N]. |
| 3 | Production bundle id | D then A | A bundle id cannot change after the first upload **[memory]**. `...autoradio.lab` says "lab" forever. Recommended: `news.choir.autoradio` as `app.id`, with the lab build as a variant suffix. Register: `asc bundle-ids create ...` or `POST /v1/bundleIds` (Provisioning API, team key only [K]). Exact flags of the CLI command: **[unconfirmed]**, run `asc bundle-ids --help`. |
| 4 | App record (name, SKU, primary language, bundle id) | H | The public API cannot create an app; the CLI does it only through Apple's private web session (`asc web apps create`), which its own docs call "experimental, unofficial, and discouraged" and a risk to the account [W][N]. fastlane `produce` does not work with an API key [F]. Five minutes in the browser. The name must be 30 characters or fewer and unique (2.3.7) [G]. |
| 5 | API key | H | Users and Access > Integrations. Team key, role App Manager (enough for metadata, TestFlight, submission; **[memory]** for the exact role matrix, Apple's role page did not list it [R]). Provisioning calls need a team key, not an individual key [K]. The private key downloads once [K]. Store it outside the repo (section 7). |
| 6 | Distribution certificate and App Store profile | A, once H | Simplest: the owner signs in to Xcode once and lets it create an "Apple Distribution" certificate; the private key stays in the login keychain. Then an agent creates the profile by API (`asc profiles ...` or fastlane `sigh` with the API key [F]) and passes its path as `EXACT_PROFILE` and the certificate's SHA-1 as `EXACT_IDENTITY`. Agents never see the private key: they name it by hash. No `match` repo is needed for one Mac. |
| 7 | Capabilities | A | None beyond the default today: background audio is an Info.plist key, the sign-in callback is a private scheme with no entitlement (`docs/reference/exact/auth-session.md`). An in-app passkey sheet later needs Associated Domains, which is a new capability and a new profile. |
| 8 | Version and build number in the bundle | A | `CFBundleShortVersionString` 1.0.0 and a rising `CFBundleVersion`. How Exact sets them on iOS: **[unconfirmed]**; read Exact's reference before the first archive. `asc builds next-build-number` gives the next number [A]. |
| 9 | Export compliance | A | The app uses HTTPS through the OS, the Keychain, and AES-GCM from CryptoKit for the local mail copy (`MailSeal.swift`). All of it is the operating system's encryption. Apple: "the use of encryption that's built into the operating system ... is exempt from export documentation upload requirements" [E]. Set `ITSAppUsesNonExemptEncryption` to `NO` in Info.plist so no question is asked per build [E][X]. Whether Exact's iOS manifest accepts an extra plist key: **[unconfirmed]**; otherwise answer per build (`asc encryption ...`, or the build's `usesNonExemptEncryption` attribute **[memory]**). No documents to file. Apple also says an exempt app "might" owe a year-end self-classification report to the US government [E]; whether it does here is a legal reading I cannot settle. France asks for a declaration only for non-OS encryption [X]. |
| 10 | Privacy manifest | A | Exists. Exact's build scans every binary in the bundle, including the statically linked FluidAudio and llama.cpp, and stops on an undeclared required-reason API (`docs/reference/EXACT.md`). Neither library, nor Exact, is on Apple's list of SDKs that must ship their own manifest and signature [T]; that list has had no ML library on it. Add `NSPrivacyCollectedDataTypes` to match the nutrition label **[guess: not enforced for an app's own manifest, but it keeps Xcode's privacy report honest]**. Proof: upload a build; Apple emails ITMS-91053 if a reason is missing **[memory]**. |
| 11 | Privacy nutrition label | H or risky A, D | Not in the public API. The CLI can pull, plan and apply it through the web session [W]; I would do it by hand once. Content below. |
| 12 | Privacy policy URL, support URL | D, A | Both required. The policy must also be reachable inside the app (5.1.1(i)) [G]. The support URL "must lead to actual contact information" [P]. `asc app-setup info set --app ID --locale en-US --privacy-policy-url URL` [S]. |
| 13 | Age rating | D then A | New questionnaire since 2026-01-31 [U][AR]. `asc age-rating ...` (public API) [C]. Recommended answers in section 2, risk 12. |
| 14 | Content rights | D, H | App Store Connect asks whether the app contains, shows or accesses third-party content and whether you have the rights. Answer yes. Basis: stories are original text written from cited sources with links; podcasts play from publishers' own feeds. The field is in the API as `contentRightsDeclaration` **[memory]**. |
| 15 | Category, price, territories | D, A | News primary, free. `asc categories`, `asc pricing` [C]. |
| 16 | Metadata as files | A | `asc metadata init`, `asc metadata apply --dry-run`, then apply [A]; or fastlane `deliver` folders. Limits: description 4000 characters, keywords 100 bytes, promotional text 170 [P]. |
| 17 | Screenshots | A | Section 5. |
| 18 | Review information | A | Contact name, email, phone with country code; notes up to 4000 bytes; demo sign-in "must not expire" [P]. `asc review ...` manages details and attachments [C]. Text in section 4. |
| 19 | Archive | A (lead only) | In the build tree, through the build lock: `EXACT_IDENTITY=<sha1> EXACT_PROFILE=<path> bun exact.mjs ios --device --archive out.ipa` [doc, unrun]. `asc ipa-info out.ipa` inspects it offline [C]. |
| 20 | Upload | A | `asc builds upload --app ID --ipa out.ipa` [C], or `fastlane pilot upload --ipa out.ipa --api_key_path <json>` [F], or `xcrun altool --upload-app -f out.ipa -t ios` with API-key flags (**[memory]** for the flags; Apple's page shows only the username form [B]). Apps must be built with Xcode 26 or later [U]. |
| 21 | Submit | A | `asc validate --app ID --version 1.0.0 --strict`, then `asc publish appstore --app ID --ipa out.ipa --version 1.0.0 --submit --confirm`, then `asc status --app ID --until review-done --timeout 2h` (exit 0 approved, 1 rejected, 7 still pending) [A][S]. Choose manual release for version 1 [P]. |

**The nutrition label, exactly.** Apple's test: "collect" means sending data off the device so that you or a partner can reach it "for a period longer than what is necessary to service the transmitted request in real time"; "data that is processed only on device is not collected" [L]. The listener's "computer" is a server the developer operates, so what it stores is collected by the developer. Declare, all "linked to the user", none used for tracking:

| Data type (Apple's name) | Why | Purpose |
|---|---|---|
| User ID | the account and the device key | App Functionality |
| Email Address | only if the account keeps one (the auth service has email verification routes) | App Functionality |
| Other User Content | the transcribed words of a spoken request, documents, imported PDF and EPUB files, notebook entries | App Functionality |
| Emails or Text Messages | the Mail feature's messages held on the computer | App Functionality |
| Audio Data | **only** when Takes ships (a published voice recording). Microphone audio that is transcribed and discarded on the phone is not declared. | App Functionality |
| Product Interaction | what was played, finished, skipped; "more of this" taps (`/api/autoradio/steer`) | App Functionality, Personalization |
| Search History | podcast and paper searches sent to the server | App Functionality |
| Crash Data, Diagnostics | none today; add if a crash reporter or uploaded traces are added | n/a |

Do not declare: on-device speech audio, free disk space, boot time. Web views count unless the user is browsing the open web [L]. Requests for model files to a third-party host expose the IP address to that host; this is not a listed data type, but name the host in the privacy policy **[guess]**.

## 2. Rejection risks for this app, ranked

TF = also applies to TestFlight external beta review ("should comply with the App Review Guidelines", 2.2 [G]). In practice beta review is lighter **[memory]**, but sign-in, crashes and permissions are checked. AS = App Store review only.

| # | Risk | Guideline, current text | Why here | Mitigation | Scope |
|---|---|---|---|---|---|
| 1 | Reviewer cannot get in | 2.1(a): "include demo account info (and turn on your back-end service!) if your app includes a login." 5.1.1(v): "If your app doesn't include significant account-based features, let people use it without a login." | Sign-in is a passkey in a browser sheet plus device approval. A reviewer has no passkey for a demo account, and passkeys cannot be typed into the username and password fields. | Signed-out listening as the default first run (already built), plus a reviewer path that needs no passkey (section 4). | TF, AS |
| 2 | Account deletion missing | 5.1.1(v): "If your app supports account creation, you must also offer account deletion within the app." Apple's page: it applies even when creation happens in a browser; deactivation is not enough; a link "directly to the page" that completes it is allowed [D]. | Registration exists on the web; no deletion route found. | Settings > Account > Delete account: confirm with the passkey, delete the account, its computer and its published objects, state the time it takes. A server change (red class: auth). | AS; TF likely |
| 3 | Spoken words sent to third-party AI without consent | 5.1.2(i): "You must clearly disclose where personal data will be shared with third parties, including with third-party AI, and obtain explicit permission before doing so." | A spoken request is transcribed on the phone and the text goes to the server, which calls outside model providers. | One consent screen before the first server turn, naming the kinds of provider and what is sent; the same in the privacy policy; a Settings switch to withdraw (this is consent, not a feature switch). Without consent, voice still does on-device commands. | TF, AS |
| 4 | Completeness: crashes, placeholders, developer surfaces | 2.1(a): "final versions ... placeholder text, empty websites, and other temporary content should be scrubbed"; "over 40% of unresolved issues are related to guideline 2.1" [V]. 2.3.1(a): hidden or undocumented features. | Hands-free voice "has never been spoken to on a device"; sound, clips and podcasts "never heard on a device"; the carried build has a developer menu (the README names a Lab tab with Trace; I could not find it in this checkout's pages, so its state on main is **[unconfirmed]**); fixture text "written only to test the radio" exists in fallbacks. | Remove Lab, Trace, stage names and fixtures from the release build at compile time (the demo-mode fix is the pattern: `docs/problems/owner-phone-stuck-in-demo-mode-2026-11-20.md`). Ship only pages the owner has used on a phone; cut the rest from version 1 rather than hide them. | TF, AS |
| 5 | Model downloads | 4.2.3(ii): "If your app needs to download additional resources in order to function on initial launch, disclose the size of the download and prompt users before doing so." 2.5.2: apps may not "download, install, or execute code which introduces or changes features or functionality". | 0.64 GB language model, 0.31 GB embedder, voices, from a third-party host. A FluidAudio stall once looked like a 20-minute hang (`mobile/README.md`). | Weights are data, not executable code, and are run by code already in the binary; say so in the notes. State sizes and ask before downloading; show progress; the radio must play with the system voice with no model present. Host the files yourself or as Apple-hosted Background Assets (downloads managed by the system; On-Demand Resources is deprecated [BA]) so review does not depend on a third party. Pin by hash (already done for the GGUF). | TF, AS |
| 6 | AI-written news: accuracy and sources | 1.1.6: "False information and features" (objectionable content). 5.2.1: no protected third-party material without permission. 5.2.2: displaying "content from a third-party service" needs to be "specifically permitted". 4.2.2: apps should not be primarily "content aggregators, or a collection of links". No guideline names AI-written news; the only AI mentions are 1.2, 4.7 and 5.1.2(i) [G]. | Stories are machine-written from web sources. | Label every story as written by AI from cited sources, on the page and in the description; show sources as tappable links (already designed: source chips); never reproduce a source at length; a "Report a problem with this story" action; a corrections policy page. Do not claim accuracy in metadata (2.3.7: no "unverifiable product claims"). The CLI's notes mention a web-only "AI-generated content" compliance declaration [N]; I could not confirm what it is or where it applies **[unconfirmed]** (likely one storefront). | AS mainly |
| 7 | User-generated content | 1.2: apps where users "create, generate, upload, share" content, including "AI content creation apps", must include filtering, "a mechanism to report offensive content and timely responses", "the ability to block abusive users", and "published contact information". | Takes are published and heard by other listeners; documents can be published to the shared graph **[guess: I did not trace a publish action on the phone]**. | Ship version 1 without publishing from the phone, or build report, block, a filter at publish, and a contact page first. The server already has an operator yank. | TF, AS |
| 8 | Microphone with the screen locked | 2.5.14: "explicit user consent and ... a clear visual and/or audible indication when recording". 5.1.1(iv): do not "force people to consent to unnecessary data access". 2.5.4: background services "only ... for their intended purposes". | `ExactCapturesInBackground = true`; a third usage string says "Keeps the radio able to hear you with the screen locked." | Voice off until the listener turns it on (already the rule); an audible cue and on-screen state when listening starts and stops; never ask for the microphone at Play (the README's first-run step 5 does: change it); the radio fully usable when the microphone is declined. Say in the notes that audio is not recorded or uploaded. | TF, AS |
| 9 | Sign-in leaves the app | 4 (design), and Apple's deletion page: linking out for sign-in "provides a poor user experience" [D]. 4.8 does not apply: sign-in is the developer's own system. | Device flow with a code and approval page. | The system sign-in sheet over the app (built 2026-11-23); never show a raw code unless the sheet fails. | AS |
| 10 | Minimum functionality, web wrapper | 4.2: "features, content, and UI that elevate it beyond a repackaged website". | Cross-platform framework; a reader with a web view. | Exact draws native views, not a web page; lead the screenshots and notes with lock-screen playback, voice and on-device models. Low risk. | AS |
| 11 | Over-the-air updates | 2.5.2 (above). The licence agreement's interpreted-code clause allows downloaded interpreted code that does not change the app's primary purpose **[memory; the page was too long to reach the clause today]**. | Exact can update signed plans and a TypeScript module with `deploy.store "A"`. | Keep `"0"` for version 1 (it is today). | AS |
| 12 | Age rating wrong | 2.3.6: "Answer the age rating questions ... honestly". | News covers war and crime: "Mature or Suggestive Themes" includes "real-world crimes ... war or political strife"; frequent is 16+, infrequent 9+. "Unrestricted Web Access" is 16+. User-generated content alone stays 4+ [AR]. | Declare frequent mature themes: 16+ **[guess on "frequent"; honest for a daily news paper]**. Keep the reader's web view to pages the paper links, with no address bar, and answer no to unrestricted web access. Not in the Kids category (1.3). | AS |
| 13 | Podcasts and clips | 5.2.3: do not "save, convert, or download media from third-party sources ... without explicit authorization"; streaming "may also violate Terms of Use". | Podcast search, feeds, streaming, and cut clips ("Hear it") played from the publisher's file. | Streaming from the publisher's own RSS enclosure is the normal podcast-app pattern. Clips must play ranges of the publisher's file and never be re-hosted; no download-to-file export in version 1. | AS |
| 14 | Mail and Files | 5.1.1(iii) data minimisation; 5.2.2. | Mail is the listener's own mailbox on their computer; Files uses the system picker. | Uses the out-of-process picker (good). If Mail ever connects to an outside mail provider, that provider's sign-in rules apply. Low risk. | AS |
| 15 | Other platforms in metadata | 2.3.10: no "names, icons, or imagery of other mobile platforms". | A shared codebase with Android. | No Android word in the app, screenshots, notes or What's New. | TF, AS |
| 16 | Payments | 3.1.1: unlocking features needs in-app purchase. 3.1.3(f): a free companion to a paid web tool is allowed only for "VoIP, Cloud Storage, Email Services, or Web Hosting" with no purchase call to action. | A paid computer tier may come. | Version 1 free, no prices or upgrade links anywhere. A paid tier later means in-app purchase or a careful reading of the US-storefront link rules. | AS |
| 17 | Model licences | 5.2.1. | One optional model family is licensed by revenue; another has its own terms (`docs/reference/ON-DEVICE-MODELS.md`). | Ship only models whose licences allow it; list them in About with attributions. | AS |
| 18 | Second app | 4.3 spam (duplicate apps) **[memory for the number]**. | "Autoradio 1". | Never upload the variant to the same category as a second public app; it stays a development install. | AS |

## 3. TestFlight first

| | Internal | External |
|---|---|---|
| Who | up to 100 team members with an App Store Connect role [TF] | up to 10,000, by email or public link [TF] |
| Review | none | the first build of an app is reviewed; "subsequent builds may not need a full review" [TO] |
| Needs | the app record, a processed build, export-compliance answer | also a beta description, a feedback email and beta review information [TF][TI] |
| Expiry | 90 days per build [TO] | the same |

Fastest safe path: (1) owner does checklist items 1, 3, 4, 5 and the Xcode sign-in (about 40 minutes); (2) the lead archives and uploads; (3) an internal group with automatic distribution delivers every processed build to the owner's phone with no review. The owner must be invited once and install the TestFlight app. This also replaces the 7-day free profile: a paid team's development profile lasts a year **[memory]**, and TestFlight builds last 90 days.

Commands [TG][A]:
```
asc testflight groups create --app ID --name "Owner" --internal
asc builds upload --app ID --ipa out.ipa
asc builds add-groups --build-id BUILD --group GROUP
asc testflight groups create --app ID --name "Friends"          # external; triggers beta review on first build
asc testflight testers add --app ID --email <address> --group "Friends"
asc testflight feedback view --submission-id ID ; asc testflight crashes log --submission-id ID
```
"What to Test" is a per-build, per-locale text: `asc build-localizations ...` [C] or `fastlane pilot upload --changelog "..."` [F]. Apple rejects `<`, emoji and many symbols in that text [N]; generate it from the change log as plain sentences. `asc publish testflight` wraps upload and distribution [A].

Fix before even an internal build: the production bundle id decision (an upload fixes it); the version and build number mechanism; `ITSAppUsesNonExemptEncryption`; the release build must not contain the Lab or Trace pages; proof that Exact's archive passes processing (the first upload is that proof; expect a round or two). The `.one` variant needs no App Store record: keep it a development-signed install, or give it its own record only if the owner wants two TestFlight apps side by side (then it is a separate app with its own review).

Before external testers: account deletion, the third-party AI consent, the model download prompt, and a reviewer path (section 4), because beta review signs in.

## 4. The reviewer's experience and onboarding

Assume: no account, a fresh phone, a slow network, a reviewer who may tap "Don't Allow" on every prompt, and three minutes of attention.

**First run must do this.**
1. Launch opens the radio, signed out, with the public front page loaded. One large Play. No prompt of any kind.
2. Play speaks the first story with the system voice at once. No model download is needed for this. The lock screen shows title and controls. This is the whole "works within a minute" proof and it is already the designed signed-out behaviour; it needs to be seen on a phone.
3. A quiet row offers "Better voice and talking to the radio: 1.0 GB download". Tapping it shows the sizes and asks (4.2.3(ii)), then downloads with real progress, resumable, on Wi-Fi by default.
4. Only when the listener turns Voice on: a one-screen explanation, then the microphone prompt, then the speech prompt. Then the third-party AI consent the first time words would leave the phone.
5. Voice onboarding ("what are you interested in?") runs after step 4 and always has a typed and tappable alternative (the topic tree already has tap and typed edits). Signed out, the picks stay on the phone.
6. Sign-in is offered where an account is needed (mail, files, your own computer), never as a gate to listening.

**Declined microphone.** The radio, paper, reader and podcasts work fully. The Voice control shows one line, "Microphone is off for Autoradio", with a button to Settings. Never re-prompt in a loop (5.1.1(iv)).

**Usage strings.** Current: "Hears you when you talk to the radio." and "Turns what you say into text on this phone." They are clear but short. Proposed: microphone, "Autoradio listens when you turn Voice on, so you can ask about a story or steer the radio. Audio is processed on this phone and is not uploaded." Speech, "Autoradio turns what you say into text on this phone to understand your request." Apple asks that purpose strings "clearly and completely describe your use" (5.1.1(ii)). The background string must match what the code does.

**How a reviewer signs in, when sign-in is a passkey.** Options, best first:
- **A. No sign-in needed for the core.** Tick "sign-in required" off only if every reviewed feature works signed out. Not true once mail and files are in the build.
- **B. A review account with a code sign-in.** The review notes carry a user name and a long one-time-style secret; the app's sign-in page has an "I have a review code" link; the server exchanges it for a device key bound to one seeded review computer with sample mail, files and a notebook. It does not expire (Apple: the demo account "must not expire" [P]), is rate-bounded, cannot spend money, and is wiped nightly. This is a second sign-in method for one account, so it is an auth change (red class) and must not become a general password path.
- **C. A built-in demo mode.** Allowed only "with prior approval by Apple" and it must show "full features" (2.1(a)). The repo deleted demo mode on devices after it trapped the owner's phone; do not bring it back.
- **D. A video only.** Apple accepts a demo video where the environment is "hard to replicate" [V], but for sign-in a video alone usually draws a rejection **[memory]**.

Recommended: A for the listening core plus B for account features. Put the B credentials in the username and password fields and repeat them in the notes.

**Review notes (about 2,500 bytes of the 4,000).** In this order: what the app is in two sentences; "no account is needed: tap Play"; the review sign-in; that stories are written by AI from cited sources and where the sources show; that speech recognition, the voice and a small language model run on the device, that model files are data downloaded after an explicit prompt and no executable code is downloaded; why the audio background mode is used (spoken news continues with the screen locked, with lock-screen controls); why the microphone may stay available with the screen locked and how the listener turns it off; that the app is built with a native cross-platform framework, not a web view; where account deletion is; where reporting is; a contact phone that will be answered.

**The video.** There is an attachment field in App Review Information ("attach the files in the Attachment section" [V]); the API has review attachments and the CLI manages them [C]. Limits on type and size were not on the pages I read **[unconfirmed]**, so also host the video at an unlisted URL and put the link in the notes. Ninety seconds: cold launch, Play, lock screen, Voice on with the prompts, one spoken question, the sources on a story, delete account.

## 5. Screenshots and app previews

**Screenshots** [SS]. One to ten per device size, `.png` or `.jpg`, no alpha. For an iPhone-only app one set is required: the medium Dynamic Island size, 1179 x 2556 or 1206 x 2622 portrait ("Yes (at least one screenshot)"). The large sizes (1290 x 2796, 1320 x 2868, or 1260 x 2736) are scaled down to smaller phones, so capture large and let Apple scale: one capture run covers everything. The page's fallback wording for the large sizes is circular; uploading both a large and a medium set removes doubt. iPad sets (2064 x 2752) are needed only if the app runs on iPad; `deviceFamily` is iPhone only, so none. A new foldable size becomes required in April 2027 for apps on the iOS 27.1 SDK. Content: "the app in use, and not merely the title art, login page, or splash screen" (2.3.3); 4+ appropriate whatever the rating (2.3.8); no other platforms (2.3.10); no prices (2.3.7).

**Previews** [PV][PC]. Optional, up to three per size and language, 15 to 30 seconds, 500 MB, H.264 (`.mov`, `.m4v`, `.mp4`, up to 30 fps, 10 to 12 Mbps) or ProRes 422 HQ, stereo AAC 256 kbps at 44.1 or 48 kHz. iPhone resolution for every Face ID and Dynamic Island size: 886 x 1920 portrait. (The CLI's guide says "resolution matching the device screenshot size" [SG]; that conflicts with Apple's table, and Apple's table wins.) Content: "may only use video screen captures of the app itself" (2.3.4); "Don't film people interacting with a device"; overlays, touch hotspots, narration and text are allowed; they play muted by default, so use captions; no prices or dates; if the app "requires login, you must disclose this"; only content you have rights to show. For an audio product, a preview with captions of what is being spoken is worth more than a fifth screenshot.

**Pipeline for this app.** Lead only for capture, because it needs a native build and a simulator; slim simulators only, each named, through `mobile/scripts/sim-device.sh` (several may now run at once if slim; the native build still takes the one build lock).
1. Seed: a pinned set of real published stories served by a fixture mode of the proof harness, so the screens are stable and contain no placeholder text. Real stories, real sources.
2. Drive: the existing simulator drive (`mobile/scripts/proof-real.sh` style, Exact's agent driver) walks to each screen. Exact has no Xcode UI-test target, so fastlane `snapshot` (which needs one [FS]) does not fit.
3. Clean the bar: `xcrun simctl status_bar <name> override --time 9:41 --batteryState charged --batteryLevel 100 --cellularBars 4` **[memory]**.
4. Capture: `xcrun simctl io <name> screenshot --type=png shots/en-US/iphone/01-radio.png` on an iPhone Pro Max class simulator (1320 x 2868) **[memory for the pixel size of a given simulator; check with `asc screenshots sizes --all`]**. Video: `xcrun simctl io <name> recordVideo --codec h264 raw.mov`, stop with interrupt.
5. Caption and frame: a small script (ImageMagick or a headless HTML render) writes the caption above the screenshot on a plain ground. Device frames are optional and Apple does not require them. `asc screenshots frame` and fastlane `frameit` exist; the first is marked experimental [SG], the second needs ImageMagick and a `Framefile.json` [FS].
6. Preview encode: `ffmpeg -i raw.mov -vf "scale=886:1920,fps=30" -c:v libx264 -profile:v high -level 4.0 -b:v 11M -f lavfi -i anullsrc=r=48000:cl=stereo -shortest -c:a aac -b:a 256k preview.mp4` **[memory; reorder inputs as ffmpeg requires]**. A silent stereo track avoids rejections of mono or missing audio **[memory]**. The simulator's own audio is not captured by `recordVideo` **[memory]**, so narration is mixed in afterwards.
7. Localise: captions come from a strings file per locale; the drive runs once per locale. Version 1: en-US only.
8. Upload: `asc screenshots upload --app ID --version 1.0.0 --path ./shots --device-type <type>` with a `shots/<locale>/iphone/` layout, and `asc video-previews upload --version-localization LOC --path preview.mp4 --device-type <type>` [SG]. Or `fastlane deliver --skip_binary_upload --skip_metadata` [FS].

## 6. Ongoing updates

An agent can run all of this; the owner appears only where marked.

1. **Version and build.** Marketing version from the change log's release heading; build number `asc builds next-build-number` or a UTC timestamp. Never reuse a build number.
2. **What's New.** Generated from `docs/changes.md` entries tagged for the phone, rewritten as plain user sentences, kept as `mobile/store/en-US/release_notes.txt`. Required from version 1.0.1 on, 4000 characters [P]. No other platforms, no prices.
3. **Metadata as files in the repo**: description, keywords, subtitle, promotional text, URLs, review notes (without the review secret, which is injected from the keychain at apply time), age-rating answers, screenshots' captions. `asc metadata apply --dry-run` in CI shows drift [A]; `asc migrate` converts to and from fastlane's layout [C].
4. **Build and upload**: lead, build lock, archive, `asc builds upload`.
5. **TestFlight**: internal group automatic; external group after the owner's go.
6. **Submit**: `asc validate --strict`; `asc release stage --app ID --version V --build BUILD --copy-metadata-from PREV --confirm`; submit; `asc status --until review-done` [A][S].
7. **Release**: manual release then phased release ("over a 7-day period" to automatic updaters [P]). The phased-release resource is in the public API **[memory]**; the CLI's page did not show the command **[unconfirmed]**, `asc api` can send it. Pause it on a crash spike.
8. **Expedite**: Apple grants it for "a critical bug" or a dated event [V]. It is a web form **[memory]**; I found no API. Owner or lead in the browser.
9. **Rejection**: `asc status` exits 1. The reviewer's message lives in the Resolution Center, which has "no public OpenAPI surface" [N]; the CLI reads it only through the web session (`asc web review list|show`) [W]. So: the agent detects the rejection, a human reads and replies in the browser, the agent prepares the fix and resubmits. One appeal per rejection [V]. Record each rejection as a problem record first.
10. **Monitoring**: customer reviews (`asc reviews`), TestFlight feedback and crash logs (`asc testflight feedback`, `crashes`), performance and diagnostics (`asc performance`), webhooks for status changes (`asc webhooks`) [C][TG]. All public API. A daily read into a problem record when something new appears.

**Over-the-air updates under the guidelines.** 2.5.2 forbids downloading code "which introduces or changes features or functionality". Exact's store `"A"` can replace signed plans, assets and one TypeScript module; never native code or grants. May change without a new build: the paper's content, the programme, fixes to logic that keep the reviewed behaviour, copy, layout corrections. May not: a new page or feature, a new data collection or permission use, anything that changes what the reviewer saw, anything touching payments. My recommendation: stay on `"0"` until the app has been approved twice, then treat `"A"` as bug-fix delivery only, with every over-the-air change also in the next binary and described in its notes. This is conservative; other frameworks' over-the-air services operate under the same reading **[memory]**.

## 7. Tool recommendation

| Tool | State today | Fit |
|---|---|---|
| App Store Connect CLI (`asc`) | Go, MIT, one binary, about 8.5k stars and 13,800 commits; version 7.3.1 released today; ten releases in the last 17 days [A][RL] | Best fit for agents: JSON by default when piped, `--dry-run`, `--confirm`, exit codes, API-key auth, raw `asc api`, covers TestFlight, metadata, screenshots, signing, review, feedback. Risks: unofficial; very fast-moving with major versions weeks apart; telemetry on by default (`ASC_TELEMETRY_DISABLED=1`); its `web` commands use Apple's private session. Pin a version by hash; never use `asc web` unattended. |
| fastlane | Maintained: 2.242.0 on 16 November, security fixes in October [FR]. `pilot`, `deliver`, `match`, `sigh`, `cert`, `precheck` work with an API key; `produce` and `pem` do not [F] | Mature fallback for upload and metadata. Needs Ruby (3.1 or later). `gym` and `snapshot` assume an Xcode project and UI tests, which Exact does not have. `match` solves multi-machine signing we do not have. |
| Raw API | Version 4.6.1 [RN]; JWT from a team key | The ground truth; use through `asc api` for anything the CLI lacks. |
| Xcode Cloud | Works from an Xcode project and workflow | Does not fit: Exact builds with its own toolchain (Bun, Rust, `swiftc`), no Xcode project. |
| altool, Transporter | Both still listed by Apple for uploads [B]; altool no longer notarises [U] | A dependency-free upload fallback. |

**Recommendation:** `asc` (pinned) for everything on the public API, `xcrun altool` as the upload fallback, fastlane not installed unless `asc` fails us, the browser for the app record, agreements, the nutrition label, the Resolution Center and expedite requests. I would refuse to automate: anything through the private web session (it needs the owner's Apple ID password and two-factor codes, and Apple does not sanction it); agreement acceptance; tax and banking; replies to App Review; the release button for version 1; removal from sale.

**Credentials.** One team API key, role App Manager. The key file lives at `~/.appstoreconnect/private_keys/AuthKey_<id>.p8` (the conventional place altool reads **[memory]**), mode 600, on the lead's Mac only; or in the macOS keychain through `asc auth login` (its default store [A]). The key id and issuer id go in the gitignored `.envrc.local`. Never `asc auth login --local`, which writes `./.asc/config.json` in the repo. Add `.asc/` and `*.p8` to `.gitignore` and to the secret scan. The distribution certificate's private key stays in the login keychain; agents pass only its SHA-1. Revoke and reissue the key if a transcript ever shows it [K].

## 8. Mission outline and owner decisions

| Station | Proves | Owner needed | Owner minutes |
|---|---|---|---|
| S0 Account | agreements active; team id known | Sign the free-apps agreement (and paid, with tax and bank forms, only if a price is planned); two-factor prompts; EU trader details. Enrolment facts: an individual enrols under their legal name, which is shown as the seller; an organisation needs a D-U-N-S number, a legal entity, a website on its domain and binding authority [EN] | 15 to 45 |
| S1 Identity | production bundle id registered; app record exists; API key stored; Xcode signed in | choose id and name; create the record; create the key | 20 |
| S2 First upload | Exact's archive passes processing; export answer set; privacy manifest accepted | none | 0 |
| S3 Internal TestFlight | the owner installs from TestFlight; builds arrive automatically | install TestFlight, accept the invite | 5 |
| S4 Release build is clean | no Lab or Trace; no fixtures; signed-out radio plays within a minute on a fresh phone, microphone declined | listen once on the phone | 15 |
| S5 Compliance features | account deletion; third-party AI consent; download prompt with sizes; story report action; AI label and sources | approve the consent and policy wording | 20 |
| S6 Reviewer path | review sign-in works against a seeded review computer; notes and video done | watch the video | 10 |
| S7 External beta | beta review passed; first outside testers | choose testers | 10 |
| S8 Store assets | screenshots and one preview generated by script and uploaded | approve the look | 15 |
| S9 Legal pages | privacy policy, support page, corrections policy live; nutrition label entered | read and approve; enter the label in the browser | 30 |
| S10 Submission | approved, or a recorded rejection and a fix | answer App Review if asked; press release | 10 to 30 |
| S11 Update loop | version 1.0.1 shipped by an agent end to end with a phased release | none | 0 |

Total owner time: roughly two and a half to four hours, most of it in S0, S1 and S9.

**Owner decisions, with my default.**
1. Enrolment type: individual (seller shown as a personal name) or organisation. Default: whatever the renewed account already is; do not convert before the first submission.
2. Production bundle id and name. Default: `news.choir.autoradio`, "Autoradio" (check the name is free when creating the record).
3. Price. Default: free, no purchase anywhere in version 1.
4. Territories. Default: United States first; add the EU after trader status is done.
5. What ships in version 1. Default: radio, paper, reader, voice, podcasts by streaming; mail, files and takes publishing only if each has been used on the owner's phone and its risks above are closed.
6. Reviewer access. Default: signed-out listening plus a review-code sign-in to a seeded computer.
7. Account deletion scope and delay. Default: immediate, deletes the account, the computer and published objects.
8. Third-party AI consent wording and whether voice turns may leave the phone by default. Default: ask once, on first use.
9. Model hosting. Default: our own host now, Apple-hosted Background Assets later.
10. Age rating. Default: 16+ (frequent mature themes in news), no unrestricted web access.
11. Over-the-air updates. Default: off.
12. Privacy policy, support contact and a phone number App Review can call. Default: a page on the product domain and a monitored address.
13. Tools. Default: `asc` pinned, no private web-session automation.
14. Release. Default: manual release, then a 7-day phased rollout from 1.0.1.

## Sources (all read 2026-11-23)

- [G] App Review Guidelines: https://developer.apple.com/app-store/review/guidelines/ (no date shown on the page)
- [V] App Review overview: https://developer.apple.com/distribute/app-review/
- [D] Account deletion: https://developer.apple.com/support/offering-account-deletion-in-your-app/
- [L] App privacy details: https://developer.apple.com/app-store/app-privacy-details/
- [T] Third-party SDK list: https://developer.apple.com/support/third-party-SDK-requirements/
- [U] Upcoming requirements: https://developer.apple.com/news/upcoming-requirements/
- [E] Encryption export: https://developer.apple.com/documentation/security/complying-with-encryption-export-regulations
- [X] Export compliance overview: https://developer.apple.com/help/app-store-connect/manage-app-information/overview-of-export-compliance
- [AR] Age ratings: https://developer.apple.com/help/app-store-connect/reference/age-ratings-values-and-definitions
- [SS] Screenshot specifications: https://developer.apple.com/help/app-store-connect/reference/screenshot-specifications
- [PV] Preview specifications: https://developer.apple.com/help/app-store-connect/reference/app-preview-specifications
- [PC] Preview content: https://developer.apple.com/app-store/app-previews/
- [P] Platform version and review information: https://developer.apple.com/help/app-store-connect/reference/platform-version-information and .../reference/app-review-information
- [TO] TestFlight overview: https://developer.apple.com/help/app-store-connect/test-a-beta-version/testflight-overview
- [TF] TestFlight: https://developer.apple.com/testflight/
- [TI] Test information: https://developer.apple.com/help/app-store-connect/test-a-beta-version/provide-test-information
- [B] Upload builds: https://developer.apple.com/help/app-store-connect/manage-builds/upload-builds
- [K] API keys: https://developer.apple.com/documentation/appstoreconnectapi/creating-api-keys-for-app-store-connect-api
- [RN] API release notes index: https://developer.apple.com/documentation/appstoreconnectapi/app-store-connect-api-release-notes
- [R] Roles: https://developer.apple.com/help/app-store-connect/reference/role-permissions
- [BA] Background Assets: https://developer.apple.com/documentation/backgroundassets
- [EN] Enrolment: https://developer.apple.com/programs/enroll/
- [F] fastlane API key: https://docs.fastlane.tools/app-store-connect-api/ ; [FS] screenshots: https://docs.fastlane.tools/getting-started/ios/screenshots/ ; [FR] releases: https://github.com/fastlane/fastlane/releases
- [A] CLI README: https://github.com/rorkai/App-Store-Connect-CLI ; [C] docs/COMMANDS.md ; [N] docs/API_NOTES.md ; [RL] releases page ; [W] https://docs.asccli.sh/commands/web.md ; [TG] .../guides/testflight.md ; [S] .../guides/app-store-submission.md ; [SG] .../guides/screenshots.md

Not confirmed today: the licence agreement's interpreted-code clause (page too long to reach); attachment size and type limits; the exact API role matrix; the phased-release and expedite surfaces; beta review information fields beyond Apple's one-line statement; CLI flags for commands whose pages I did not open (`bundle-ids`, `profiles`, `age-rating`, `review`, `build-localizations`); the CLI's first release date and open-issue count.
