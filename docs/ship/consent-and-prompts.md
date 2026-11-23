# Consent screen and prompt wording for Autoradio 1

Short, plain, no exclamation marks. Every text here is for the app as it will be at submission, so none of the screens exist in the code today; that is the single [WHEN-BUILT] for the whole file. Tag [CHECK] needs the owner or a lawyer. The exact strings go in LAB/strings/en.json (system permission strings) and in the page code (the rest).

Still open for the owner:

- The contact email (CONTACT_EMAIL, used in the error text of section 5).

Rulings applied (owner, 2026-11-23): during the beta every account is free tier and providers may train on what is sent; minimum age 16; passkey sign-in and account creation inside the app, no codes; deletion with a 72 hour hold; downloads only after the person agrees; the microphone asked for only when voice or a take is turned on; signed out is listening only.

Guideline behind the consent screen: 5.1.2(i), "clearly disclose where personal data will be shared with third parties, including with third-party AI, and obtain explicit permission before doing so."

## 1. The consent screen

Shown once per account, before the first request that reaches a model on a server. Two requests do this: the first spoken or typed request to the radio, and the first programme request of a signed-in listener (the radio writing the next stretch from your topics and history; see data-inventory.md F2). The screen therefore appears on the first signed-in start of the radio, not only on the first question.

Title:

> Before the radio answers

Body (the beta notice comes first and is not folded away):

> During the beta every account is on the free tier. What you say or type to the radio goes to outside companies that run AI models, with the page you have open, the recent conversation, your topics, and any mail or files you ask about.
>
> Some of them may keep it and use it to train their models, so do not put anything here that you would not want used that way.
>
> A paid tier with stronger protections is planned. It is not available yet.
>
> Your voice audio is not sent. It is turned into text on this phone.

Link line (opens the policy page inside the app, at the section on AI models):

> Read the privacy policy

Buttons:

> Allow
> Not now

If the listener taps Not now:

> The radio still plays. Listening works, and you can still say commands like skip and pause. Questions need your agreement. You can allow it in Settings.

Settings row, off after Not now, on after Allow:

> AI answers from outside models
> Allowed. Your requests are sent to AI models run by other companies, which may keep them and train on them. Turn off to stop.

Notes for the builder:

- Allow is recorded on the account, with the date, and shown in Settings. Withdrawing stops requests at once. This is consent, not a feature switch.
- Not now blocks the request before it is made, not after. That includes the programme request: after Not now the radio plays the public front page and does not send topics or history to a model. [CHECK: builder to confirm what the radio plays after Not now.]
- The provider names are not on this screen. They are in the policy (privacy-policy.md section 5), which this screen links to.
- Do not add the words "we ask providers not to train" anywhere. We do not promise it.
- Show the same screen again only if the categories of data change, such as a new kind of data being sent. [CHECK]

## 2. Permission strings (the system prompts)

Apple asks that a purpose string "clearly and completely describe" the use. Keys are in LAB/strings/en.json. These replace the current strings.

Microphone (purpose.microphone):

> Autoradio 1 listens when you turn Voice on or record a take, so you can ask about a story, tell the radio what to do, or make a take. Speech is turned into text on this phone. Audio is not uploaded unless you publish a take.

Speech recognition (purpose.speech):

> Autoradio 1 turns what you say into text on this phone so it can understand your request.

Microphone with the screen locked (purpose.recordingInBackground):

> Keeps Voice listening with the screen locked, until you turn Voice off or three minutes pass without you speaking.

Where each is asked: the microphone and speech prompts appear only when the listener first turns Voice on or first turns on a take. Never at launch, at Play or at sign-in. Before the system prompt, the app shows one line:

> To talk to the radio or record a take, Autoradio 1 needs your microphone and speech recognition.

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
- Speech recognition by default is Apple's on-device recogniser, which needs no download. The hearing file is optional.
- If the files move to our own host, change "Hugging Face" in the text.

## 4. What a signed-out listener sees when they try to ask

Signed out is listening only. Voice commands that need no server (skip, pause, play, clear the queue) still work on the phone. Asking a question, changing the edition or opening mail, files or notes opens a sheet.

Title:

> Sign in to ask the radio

Body:

> Listening needs no account. Asking, your edition, notes and mail need one, so the radio has a computer to keep them on.
>
> Sign in or make an account with a passkey. The phone's passkey screen opens. There is no password and no code.

Buttons:

> Sign in
> Make an account
> Not now

Sub-line under Make an account:

> You will use your email address and a passkey. You must be 16 or older. You can delete the account in Settings.

After Not now, nothing is asked again until the listener tries the same thing.

## 5. Account deletion confirmation

Reached from Settings, Account, Delete account (or from choir.news).

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

> The account was not deleted. Check your connection and try again, or write to CONTACT_EMAIL.

Do not offer "deactivate" as a substitute; Apple requires deletion, not deactivation.
