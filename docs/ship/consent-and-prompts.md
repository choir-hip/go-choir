# Consent screen and prompt wording for Autoradio 1

Short, plain, no exclamation marks. Every text here is for the app as it will be at submission, so none of the screens exist in the code today; that is the single [WHEN-BUILT] for the whole file. Tag [CHECK] needs the owner or a lawyer. The exact strings go in LAB/strings/en.json (system permission strings) and in the page code (the rest).

Still open for the owner:


Rulings applied (owner, 2026-11-23): during the beta every account is free tier and providers may train on what is sent; minimum age 16; passkey sign-in and account creation inside the app, no codes; deletion with a 72 hour hold; downloads only after the person agrees; the microphone asked for only when voice or a take is turned on; signed out is listening only.

Guideline behind the consent screen: 5.1.2(i), "clearly disclose where personal data will be shared with third parties, including with third-party AI, and obtain explicit permission before doing so."

## 1. The agreement at account creation

There is no separate consent screen (owner, 2026-11-24: "we dont need such a before the radio answers consent screen. thats not industry standard"). A person agrees once, by creating an account. Asking the radio needs an account, so everyone who can send personal content has seen this line. It sits above the button on the create-account screen, in ordinary text size:

> By creating an account you confirm you are 16 or older and agree to the Privacy Policy and Terms. What you ask the radio is sent to outside AI companies, which during the beta may use it to train their models.

"Privacy Policy" opens choir.news/privacy and "Terms" opens choir.news/terms. Tapping Create account is the agreement; there is no checkbox. Someone signing in to an existing account sees nothing extra. The second sentence stays in some form: Apple's review rule 5.1.2(i) asks an app to disclose sharing personal data with outside AI services and to get permission first.

## 2. Permission strings (the system prompts)

Apple asks that a purpose string "clearly and completely describe" the use. Keys are in LAB/strings/en.json. These replace the current strings.

Microphone (purpose.microphone):

> Autoradio 1 listens when you turn Voice on or record a take, so you can ask about a story, tell the radio what to do, or make a take. Speech is turned into text on this phone. Audio is not uploaded unless you publish a take.

Speech recognition (purpose.speech):

> Autoradio 1 turns what you say into text on this phone so it can understand your request.

There is no background microphone purpose string: the background microphone grant was removed on main (`b53276cba`), and the app gives the microphone up when it goes to the background.

Where each is asked: the microphone and speech prompts appear only when the listener first turns Voice on or first turns on a take. Never at launch, at Play or at sign-in. Before the system prompt, the app shows one line:

> To talk to the radio, record a take or learn your voice, Autoradio 1 needs your microphone and speech recognition.

Buttons: Continue, Not now. If the listener declines in the system prompt:

> Microphone is off for Autoradio 1. You can turn it on in Settings.

One button: Open Settings. Do not ask again until the listener turns Voice or a take on again.

## 3. Model download prompt

Nothing downloads until the listener taps Download. This includes the embedding model, which today downloads without a prompt (data-inventory.md F8). In onboarding the prompt comes after the consent screen and after the listener has chosen to set up voice. Sizes come from the setup plan (KIT/VoiceSetup.swift:109-123, :179-182) and the pinned files (KIT/KnownModels.swift).

Title:

> Add a better voice and talking to the radio

Body (a phone that can run the larger set):

> This downloads about 1.8 GB of files to your phone. They are data files, not apps.
>
> Hearing you, 0.5 GB. The voice, 0.4 GB. A small language model that answers while the computer works, 0.6 GB. Search on this phone, 0.3 GB.
>
> The files come from Hugging Face. That site sees your phone's address and which file you ask for.
>
> The radio keeps working with the iPhone voice if you skip this.

Body (a phone with less memory, where the plan takes the smaller listener and the lighter voice):

> This downloads about 1.5 GB of files to your phone. They are data files, not apps.

Buttons:

> Download on Wi-Fi
> Not now

If the phone is on mobile data, replace the first button and add a line:

> You are on mobile data. This will use about 1.8 GB of your plan.
>
> Download now
> Wait for Wi-Fi

Progress text, always real: "Downloading 214 MB of 1.8 GB." Resumable. If it stalls: "This is taking longer than expected. Try again, or use the iPhone voice."

Notes for the builder:

- The plan's first-choice sizes are 480 MB (Parakeet, the larger listener), 350 MB (Kokoro voice), 639 MB (Qwen3-0.6B) and 2 MB (Silero), about 1.47 GB; the smaller set is 250 MB, 300 MB, 639 MB and 2 MB, about 1.2 GB. The embedding model is 310 MB, so the prompt totals are about 1.8 GB and 1.5 GB. Read the sizes from the plan at run time; do not hard-code the text.
- The hearing and speaking models are needed, not optional (owner, 2026-11-24): Apple's built-in recogniser and voice are not good enough, and the models are replaced as better ones appear. Names and sizes in the prompt come from the app's model list, not from this text.
- If the files move to our own host, change "Hugging Face" in the text.

## 4. What a signed-out listener sees when they try to ask

Signed out is listening only. Voice commands that need no server (skip, pause, play, clear the queue) still work on the phone. Asking a question, changing the edition or opening mail, files or notes opens a sheet.

Title:

> Sign in to ask the radio

Body:

> Listening needs no account. Asking, your edition, notes and mail need one, so the radio has a computer to keep them on.
>
> Sign in or make an account with a passkey. The phone's passkey screen opens. There is no password. A new account first checks your email address with a code we send.

Buttons:

> Sign in
> Make an account
> Not now

Sub-line under Make an account:

> You will use your email address and a passkey. You must be 16 or older. You can delete the account in Settings.

After Not now, nothing is asked again until the listener tries the same thing.

## 5. Account deletion confirmation

**Built 2026-11-24 (not deployed), with the owner's rulings of 2026-11-23 settling the decisions below:** the words in the app (`mobile/AutoradioLab/settings.contract`, `DeleteScreen`) and on the web (`frontend/src/lib/DeleteAccountRoute.svelte`) are what the build says, and they differ from this draft: published stories stay with the author detached; access ends at once and the removal takes 72 hours, which signing in cancels; a receipt and server logs stay. The sentence about the final screen is `SCHEDULED` in `account-delete.ts`, which says "scheduled for deletion", not "deleted". Edit this section to match the built words before the store texts quote it.

Screen title:

> Delete your account

Body:

> This removes your account, your computer and everything on it: your edition, notes, documents, files, mail, listening history and your takes.
>
> Stories the paper already published stay public, with your name taken off.
>
> Your access ends now, on every device. Everything is permanently removed after 72 hours. If you sign in before then, the deletion is cancelled.
>
> Outside AI companies may keep what you sent them. We cannot delete it for you.

Buttons:

> Delete my account
> Keep my account

Confirmation step: the app asks for the passkey. Then:

> Your account is set to be deleted. You are signed out. Sign in within 72 hours to cancel.

If it cannot complete (offline, server error):

> The account was not deleted. Check your connection and try again, or write to 000@choir.news.

Do not offer "deactivate" as a substitute; Apple requires deletion, not deactivation.
