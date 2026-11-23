# Consent screen and prompt wording for Autoradio 1

Short, plain, no exclamation marks. Every text here is for the app as it will be at submission, so none of the screens exist today. Tags: [DECISION] depends on an owner decision, [CHECK] needs the owner or a lawyer. The exact strings go in LAB/strings/en.json (system permission strings) and in the page code (the rest).

Guideline behind the consent screen: 5.1.2(i), "clearly disclose where personal data will be shared with third parties, including with third-party AI, and obtain explicit permission before doing so."

## 1. The consent screen

Shown once per account, before the first request that reaches a model on a server. Two requests do this: the first spoken or typed request to the radio, and the first programme request of a signed-in listener (the radio writing the next stretch from your topics and history; see data-inventory.md F2). The screen must therefore appear on the first signed-in start of the radio, not only on the first question. [DECISION: or stop sending topics and history until the listener has agreed, and show the screen at the first question.]

Title:

> Before the radio answers

Body:

> To answer you, your computer on our server sends your request to AI models run by other companies.
>
> What is sent: the words you said or typed, the page you have open, the last few lines of the conversation, and your topics and recent listening.
>
> Who receives it: companies that run AI models and web search for us. They process it to write the answer. They may keep it under their own terms.
>
> Your voice audio is not sent. It is turned into text on this phone.
>
> You can change this later in Settings.

Link line (opens the policy page inside the app):

> Read the privacy policy

Buttons:

> Allow
> Not now

If the listener taps Not now:

> The radio still plays, and you can still say commands like skip and pause. Questions need your agreement. You can allow it in Settings.

Settings row, off after Not now, on after Allow:

> AI answers from outside models
> Allowed. Your requests are sent to AI models run by other companies. Turn off to stop.

Notes for the builder:

- Allow is recorded on the account, with the date, and shown in Settings. Withdrawing stops requests at once. This is consent, not a feature switch.
- Not now blocks the request before it is made, not after.
- The provider names are not on this screen. They are in the policy (privacy-policy.md section 5), which this screen links to. The screen says "companies that run AI models and web search for us", which is the category.
- Show the same screen again only if the categories change, such as a new kind of data being sent. [CHECK]

## 2. Permission strings (the system prompts)

Apple asks that a purpose string "clearly and completely describe" the use. Keys are in LAB/strings/en.json.

Microphone (purpose.microphone):

> Autoradio 1 listens when you turn Voice on, so you can ask about a story or tell the radio what to do. Speech is turned into text on this phone and the audio is not uploaded.

[DECISION: if takes ship, add "except a take you choose to publish".]

Speech recognition (purpose.speech):

> Autoradio 1 turns what you say into text on this phone so it can understand your request.

Microphone with the screen locked (purpose.recordingInBackground):

> Keeps Voice listening with the screen locked, until you turn Voice off or three minutes pass without you speaking.

Where each is asked: the microphone and speech prompts appear only when the listener first turns Voice on. Before the system prompt, the app shows one line:

> To talk to the radio, Autoradio 1 needs your microphone and speech recognition.

Buttons: Continue, Not now. If the listener declines in the system prompt:

> Microphone is off for Autoradio 1. You can turn it on in Settings.

One button: Open Settings. Do not ask again until the listener turns Voice on again.

## 3. Model download prompt

Shown before any model file downloads, including the embedding model that now downloads without a prompt (data-inventory.md F8). Sizes come from the setup plan (KIT/VoiceSetup.swift:109-123, :179-182) and the pinned files (KIT/KnownModels.swift).

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
- Speech recognition by default is Apple's on-device recogniser, which needs no download. The hearing file is optional. [DECISION: drop it from the prompt and cut about 0.5 GB.]
- [DECISION] Host the files ourselves before the App Store release and change "Hugging Face" in the text.

## 4. What a logged-out listener sees when they try to ask

Signed out is listening only. Voice commands that need no server (skip, pause, play, clear the queue) still work on the phone. Asking a question, changing the edition or opening mail, files or notes opens a sheet.

Title:

> Sign in to ask the radio

Body:

> Listening needs no account. Asking, your edition, notes and mail need one, so the radio has a computer to keep them on.
>
> Sign in or make an account with a passkey. There is no password.

Buttons:

> Sign in
> Make an account
> Not now

Sub-line under Make an account:

> You will use your email address and a passkey. You can delete the account in Settings.

[WHEN-BUILT: today the app only signs in to an existing account.]

After Not now, nothing is asked again until the listener tries the same thing.

## 5. Account deletion confirmation

Reached from Settings, Account, Delete account. [WHEN-BUILT] Wording follows the design the owner has ordered: deletion started in the app removes the account, its computer and its data. [DECISION: this text changes if deletion does less.]

Screen title:

> Delete your account

Body:

> This removes your account, your computer and everything on it: your edition, notes, documents, files, mail, listening history and any takes you published. [DECISION: add "and the stories you made" if published stories are removed, or "Stories the paper published stay public" if not.]
>
> It also ends your sign-in on every device. You cannot undo this.
>
> Outside AI providers may keep copies of requests you made. We will ask them to delete them where their terms allow.
>
> [DECISION: This takes effect now. / This takes up to DELETION_TIME.]

Buttons:

> Delete my account
> Keep my account

Confirmation step: the app asks for the passkey. Then:

> Your account is deleted. The app is signed out.

If it cannot complete (offline, server error):

> The account was not deleted. Check your connection and try again, or write to CONTACT_EMAIL.

Do not offer "deactivate" as a substitute; Apple requires deletion, not deactivation.
