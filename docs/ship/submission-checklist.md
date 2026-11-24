# App Store submission checklist for Autoradio 1 (2026-11-24)

Win line: apply to App Review by about 2026-11-25T02:30Z. Uploading a build and pressing Submit are the owner's acts. States: **ready** (text final, can be typed or sent), **owner** (the owner's own act or choice), **lead** (the lead's build or deploy), **blocked on X**.

## What App Store Connect holds now (read-only GET, 2026-11-24, key 52R8S94U54; no write was made)

| Resource | Finding |
|---|---|
| App | exists: id `6838500877`, name "Autoradio 1", bundle id `news.choir.autoradio`, sku `autoradio-ios`, primary locale en-US, not for kids, `contentRightsDeclaration` null |
| Bundle id | `R5WUBUAAUR`, platform UNIVERSAL, team `S47GERGJZ3`, capabilities: ASSOCIATED_DOMAINS and IN_APP_PURCHASE (the latter is unused; harmless, no purchases in version 1) |
| appInfo | id `4320e1c8-da3e-4bb7-b6ce-b4d9b4285c1f`, state PREPARE_FOR_SUBMISSION; primary and secondary category both empty; age rating declaration exists (same id) with every answer null |
| appInfoLocalization en-US | id `0a1e49b1-6373-4bcc-8cfb-5eb973e48819`: name "Autoradio 1" set; subtitle, privacyPolicyUrl empty |
| appStoreVersion | id `09b1b53d-3928-4a0b-97ee-20f9cf5de377`, iOS, "1.0", PREPARE_FOR_SUBMISSION, release type AFTER_APPROVAL, created 2026-11-23; copyright empty |
| appStoreVersionLocalization en-US | id `ae936d2e-08a0-490d-a31f-394453e5e832`: description, keywords, promotional text, support URL, marketing URL all empty |
| appStoreReviewDetail | none yet (GET returns null) |
| betaAppReviewDetail | exists, every field empty. betaAppLocalizations: none. betaGroups: none |
| Builds, preReleaseVersions | none (no build uploaded; the TestFlight first upload has not happened) |
| appPriceSchedule | resource exists, no price set; appAvailabilityV2: 404 (no territories chosen) |
| App Privacy | not in the API (404 on `/v1/apps/{id}/appPrivacy`); the owner types it in the web form from `app-privacy-label.md` |
| Screenshots and previews | none read (set not created) |

So the record and a 1.0 version already exist; nothing needs creating except the review detail, price and availability.

## Fields

| Field | Value or where it is | State |
|---|---|---|
| Name | Autoradio 1 | set |
| Subtitle | "News read aloud, with sources", `app-store-texts.md` | ready (send) |
| Promotional text, Description, Keywords | `app-store-texts.md` | ready (send) |
| What's New | not required for 1.0 | n/a |
| Support URL | no page exists; proposal in `app-store-texts.md` "URLs" | **blocked on the lead adding `/support`** |
| Marketing URL | https://choir.news | ready |
| Privacy policy URL | https://choir.news/privacy; the page serves the old policy | **blocked on the owner's read of `privacy-policy.md` and publishing it** |
| Copyright | 2026 Choir Harmonic Intelligence Platform, Inc. | ready |
| Primary category | News (`NEWS`); secondary none | ready |
| Age rating | 16+ target; the answers below | ready, owner confirms |
| Content rights | uses third-party content (yes) | owner confirms the affirmation |
| App Privacy | `app-privacy-label.md` "The exact answers to type" | owner (web form, no API) |
| Price and availability | free; United States first (lead's default, owner confirms) | owner |
| Review notes | `review-notes.md` | ready except the placeholders |
| Review contact | first and last name, phone: owner. email 000@choir.news | owner |
| Review sign-in | user name and password | owner (credential command on Node B, lane demo-account) |
| Export compliance | build sets ITSAppUsesNonExemptEncryption false | lead (the store build) |
| Build | `mobile/scripts/archive-store.sh` then upload | lead builds, owner uploads |
| Screenshots | below | lead |
| App preview video | optional; storyboard in `app-store-texts.md` | skip for 1.0 unless the lead has time |
| Version release | after approval (already set) | set |
| Digital Services Act trader status | a business publisher must declare trader or not trader (EU) | owner |
| Agreements, tax and banking | free app: Paid Apps agreement not needed | owner checks the Account Holder has accepted the current Developer Program agreement |

## Age rating declaration (answers)

Re-GET `/v1/appInfos/{id}/ageRatingDeclaration` before the PATCH: the attribute list was cut off in the read, and Apple's 2025 questionnaire renamed several. Values are `NONE`, `INFREQUENT_OR_MILD`, `FREQUENT_OR_INTENSE` for the graded ones and booleans for the rest.

| Question | Answer | Why |
|---|---|---|
| Cartoon or fantasy violence | None | |
| Realistic violence | Infrequent or mild | news about war and crime; the paper is not graphic by policy [owner confirms, CHECK] |
| Prolonged graphic or sadistic realistic violence | None | |
| Sexual content or nudity; graphic | None | |
| Profanity or crude humor | Infrequent or mild | quoted speech in stories |
| Mature or suggestive themes | Infrequent or mild | |
| Horror or fear themes | None | |
| Alcohol, tobacco, drug references | None | |
| Medical or treatment information; health or wellness topics | None | |
| Gambling (real or simulated); contests; loot boxes | None / No | |
| Unrestricted web access | No [CHECK] | the reader opens a story's cited sources as extracted text inside the app; it is not a browser with an address bar. Owner confirms: a Yes would raise the rating |
| User-generated content | Yes | takes are voice clips offered to other listeners. See the 1.2 risk in `review-notes.md` |
| Messaging and chat | No | mail is the listener's own mail client |
| Social media; advertising; parental controls; age assurance | No | |

Expected result: 16+ (the owner's ruling). If App Store Connect computes a higher rating, tell the owner before submitting; do not lower answers to reach 16+.

## Screenshots (the lead's job: simulator and device builds are the lead's)

- Required: iPhone, **6.9" display**: 1320 x 2868 (or 1290 x 2796) portrait, PNG or JPEG without alpha, at least 1 and at most 10; they scale to every smaller iPhone, so no other size is needed. iPad: not asked (deviceFamily is iPhone only). Locale en-US only.
- Take 6 for version 1.0, in this order, from the plan in `app-store-texts.md`: (1) radio playing with a story on air and the up-next list; (3) a story with its sources and the "Written by AI" line (merges the plan's 3 and 4); (5) Voice on with the heard words and an answer starting; (6) the edition topics; (7) podcasts; (8) the create-account screen over the signed-out radio, with the line above the button. Add (2) the lock screen only if the lead sees it on a device; add (9) mail and files, and (10) takes and notebook, only if used on the owner's phone. Real published stories, no fixtures, no prices, no other platforms, six-word captions on a plain ground.
- Capture on a slim simulator named for the run (`AR_SIM_NAME=ship mobile/scripts/sim-device.sh`), signed in with the review account for 5, 6, 7; one native build at a time under the lock.
- Upload by API after capture: `POST /v1/appScreenshotSets` for the localization `ae936d2e-...` with `screenshotDisplayType` `APP_IPHONE_69` (use `APP_IPHONE_67` if the API rejects the first), then for each file `POST /v1/appScreenshots` (reserve), PUT the bytes to the returned upload operations, `PATCH /v1/appScreenshots/{id}` with `uploaded: true` and the md5 `sourceFileChecksum`. The lead's calls.

## Calls to fill the metadata (not run; for the lead)

Auth: `Authorization: Bearer <ES256 JWT>`, kid 52R8S94U54, aud `appstoreconnect-v1`, 20 minutes, issuer from `ASC_ISSUER_ID` in `~/.config/choir-ship/contact.env`; key at `~/.appstoreconnect/private_keys/AuthKey_52R8S94U54.p8`. A scratch signer is at `/private/tmp/claude-501/-Users-wiz-Dev-go-choir/569bd20c-a6f4-4961-bb8d-a40886e2fbec/scratchpad/asc.py` (GET only; reads the issuer from the environment, prints nothing secret). Base `https://api.appstoreconnect.apple.com`. Content-Type `application/json`. Texts come from the fenced blocks of the named files; send them with a script that reads the block, not by retyping.

1. Subtitle and privacy URL: `PATCH /v1/appInfoLocalizations/0a1e49b1-6373-4bcc-8cfb-5eb973e48819` with `{"data":{"type":"appInfoLocalizations","id":"0a1e49b1-6373-4bcc-8cfb-5eb973e48819","attributes":{"subtitle":"News read aloud, with sources","privacyPolicyUrl":"https://choir.news/privacy"}}}`. Send the URL only after the policy is published.
2. Version text: `PATCH /v1/appStoreVersionLocalizations/ae936d2e-08a0-490d-a31f-394453e5e832` with attributes `description`, `keywords`, `promotionalText` (the three fenced blocks), `marketingUrl` "https://choir.news", and `supportUrl` once the page exists.
3. Copyright: `PATCH /v1/appStoreVersions/09b1b53d-3928-4a0b-97ee-20f9cf5de377` with attributes `{"copyright":"2026 Choir Harmonic Intelligence Platform, Inc."}`.
4. Category: `PATCH /v1/appInfos/4320e1c8-da3e-4bb7-b6ce-b4d9b4285c1f` with `{"data":{"type":"appInfos","id":"4320e1c8-da3e-4bb7-b6ce-b4d9b4285c1f","relationships":{"primaryCategory":{"data":{"type":"appCategories","id":"NEWS"}}}}}`.
5. Age rating: `PATCH /v1/ageRatingDeclarations/4320e1c8-da3e-4bb7-b6ce-b4d9b4285c1f` with the answers of the table above (attribute names as the re-GET shows).
6. Content rights: `PATCH /v1/apps/6838500877` with `{"data":{"type":"apps","id":"6838500877","attributes":{"contentRightsDeclaration":"USES_THIRD_PARTY_CONTENT"}}}`.
7. Review detail: `POST /v1/appStoreReviewDetails` with attributes `contactEmail` "000@choir.news", `notes` (the fenced block of `review-notes.md`), `demoAccountRequired` true, and leave `contactFirstName`, `contactLastName`, `contactPhone`, `demoAccountName`, `demoAccountPassword` for the owner to type in the web page; relationship `appStoreVersion` -> `{"type":"appStoreVersions","id":"09b1b53d-3928-4a0b-97ee-20f9cf5de377"}`. (Apple may refuse an empty required field at submission, not at save.)
8. Price and availability, only if the owner confirms the default: `POST /v1/appPriceSchedules` (free price point in the US base territory, no manual prices) and `POST /v1/appAvailabilities` with `availableInNewTerritories` false and the US territory. Owner's call; not part of the text draft.
9. Then GET each resource again and compare; nothing is submitted: no `reviewSubmissions` call, no build.

## Open fields and whose

- Owner: reviewer user name and password; review contact name and phone; the read and publication of the new privacy policy; the App Privacy web form; price and availability; DSA trader status; content-rights affirmation; the unrestricted-web-access and violence answers; the 1.2 decision on takes (report action, cut, or accept); upload the build; press Submit.
- Lead: `/support` page (frontend, orange); the store build with the final lanes; every screenshot; seeing on the build the "Written by AI" label, lock-screen play, Voice and models, and the deletion path; confirming the review account's delete page; keeping sign-up open or telling Apple during review; Node B steps of the demo-account runbook.
- Blocked: support URL (the page); privacy policy URL (publication); any build field (no build).
