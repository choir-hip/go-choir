# Privacy Policy for Autoradio 1 and choir.news

Effective date: DATE_OF_PUBLICATION
Published by: PUBLISHER_NAME ("we", "us")
Contact: CONTACT_EMAIL

Tags in this draft: [CHECK] marks a sentence a lawyer or the owner should verify. [DECISION] marks a sentence that depends on an open owner decision. [WHEN-BUILT] marks a sentence that is true only after a feature that does not exist today is built. Remove every tag before publishing. Sources for each statement are in data-inventory.md.

## 1. What this covers

This policy covers the Autoradio 1 app for iPhone and the website at choir.news, including the sign-in pages and the paper. It does not cover the websites of the sources that stories cite, the podcast publishers you listen to, or Apple's services. Those have their own policies.

Autoradio 1 is a radio that reads a news paper aloud. The paper is called Autopaper. Its stories are written by AI agents from cited web sources. You can listen without an account. With an account you get a personal edition, you can talk to it, and you get mail, files, a reader, podcasts and notes on a computer that is yours.

## 2. What we collect and why

### Listening without an account

The app asks our server for the public front page and the stories. Our server receives your IP address and ordinary request details such as the app version. We do not ask for your name or email. No spoken or typed request leaves your phone for a model when you are signed out. [CHECK: the code path was read in the README and the client rules, not traced line by line.]

### Creating an account

We collect your email address and a passkey. The passkey gives us a public key and some details about it, such as when it was made. The private key stays on your device. We never see a password and we do not store one. We use the email address to identify your account, to send sign-in codes, and for recovery. [CHECK]

### Your computer

Each account has its own computer on our server. It is a virtual machine with its own disk. It holds what the app does for you: your edition topics, your play history, the notes and documents you make, files you import, your mail, your podcast subscriptions and your taps of "more of this" and "less of this". We use this to run the features you asked for and to personalise your edition.

### Listening history and your edition

While you are signed in, the app tells your computer what played, what you skipped and what you finished, with times. Your computer uses this so the radio does not repeat what you heard and so it can choose what comes next. Your edition topics and this history are also sent to an outside AI model each time the radio writes your next stretch. See section 5.

### Talking to the radio

If you turn on Voice and speak, the app turns your speech into text on your phone. The text of your request goes to your computer. It goes with the title and text of the page you have open (which can be a story, a document or a mail message), the last few lines of the conversation, your play queue, and a list of candidate stories. Your computer sends this to an outside AI model to write the answer. Section 7 says exactly what is recorded and where.

### Podcasts

Podcast search and the charts go from your phone to Apple's podcast services, not through us. Podcast feeds and episodes are fetched from each publisher's own server, which can see your phone's IP address. We store the feeds you subscribe to and the episode you are playing so your computer can keep your library in step. [CHECK]

### Takes [DECISION: include this section only if takes ship in version 1]

A take is a short recording of your voice, up to 60 seconds. It is uploaded only when you press Publish. After that it is offered to other Autoradio listeners. You can delete it at any time. A take that contains music or another person's recording will be removed.

### Logs

Our servers keep logs so we can run the service and find faults. A sign-in service log line has the route, the result, the first two parts of your IP address, the first eight characters of your account id and a short hash of your session. It has no email, token or cookie. Our gateway logs searches that your computer's agents make, with the search text and your computer's id. Our model gateway counts calls per computer and does not log the prompt text. [CHECK: confirm no other service logs request text.]

### What we do not collect

We do not use advertising, tracking or analytics software in the app or on the website. We do not collect your location. We do not read your contacts or photos. The app asks Apple for no advertising identifier. We do not run a crash reporter. [CHECK]

## 3. What stays on your phone

Speech recognition runs on your phone with Apple's on-device recogniser. The app is set to refuse to recognise speech if on-device recognition is not available, so your audio is not sent to Apple or to us for recognition.

Your sign-in key is in the iPhone Keychain, on this device only. Your mail copy on the phone is encrypted with a key in the Keychain. The paper's cached front page, your edition topics, your taps, your podcast library and your settings are kept in the app's own files.

Each voice turn is saved on your phone as an audio file with a record of the turn, in the app's storage. It is not uploaded. It is deleted when you sign out or delete the app. [CHECK: if the build stops writing this file, replace this paragraph with "We do not keep your voice audio."]

## 4. What is sent to our servers

When you are signed in, the app sends the following to your computer on our server over HTTPS:

- the text of your spoken or typed requests, with the context listed in section 2;
- what you played, skipped and finished;
- your edition topics, and the time zone and local time of your phone;
- your taps of "more of this", "less of this" and mute;
- your podcast subscriptions and the episode you are playing;
- documents and books you import, files and notes you make, and mail you read or write;
- a published take, if you publish one. [DECISION]

We do not send your microphone audio to our servers, except in a take you publish.

## 5. Outside services that process data

We use outside services to run the product. They process data on our behalf. The list changes as we change providers. The current list is at PROCESSORS_URL [DECISION: create this page; today the list is section 3.1 of the web policy, which is out of date]. We update it before a new provider receives data.

These are the categories, and the providers found in our code on DATE_OF_PUBLICATION. [CHECK: re-read the model catalog before publishing.]

| Category | What they get | Providers today |
|---|---|---|
| AI models that write answers and stretches of radio | Your spoken request text and the context in section 2, your edition topics, time zone, local time and a list of stories you recently heard. For work on your computer, such as documents, your edition and research, the instructions and content of that work. | River (DeepSeek models); Amazon Web Services Bedrock (Anthropic Claude models); OpenRouter; OpenAI (ChatGPT models); Devin; OpenCode; Synthetic |
| AI model for quick decisions | The text of your request and a line saying what is playing | OpenRouter |
| Web search | Search queries written by agents on your computer, which can include your topics | Keenable, SearXNG, Brave, Serper, Parallel, Tavily, SerpAPI, ChatGPT search |
| Email delivery | Your email address and the body of messages we send for sign-in, recovery and the Mail feature | Resend |
| Model file hosting | Your phone's IP address and which model file you asked for | Hugging Face |
| Podcast search and charts | The search text you type | Apple |
| Hosting | Everything stored on our server | OVH (bare-metal server) [CHECK: country] |

Each provider has its own privacy terms. We ask providers not to train on your data where they offer that choice, but we cannot promise every provider honours it, and some free tiers may keep data under weaker terms. [CHECK: this sentence is the honest reading of the free-tier members of the default model pool. The owner may prefer to stop sending personal content to free tiers; if so, change the sentence.] Treat anything you say or write to the radio as something outside providers may see.

Before your first request reaches an AI model on a server, the app asks you to agree. If you choose Not now, voice still works for commands on your phone, such as skip and pause. [WHEN-BUILT] You can withdraw agreement in Settings. [WHEN-BUILT]

We do not sell your data. We do not share it for advertising. We share it only with the providers above, to run the product, and where the law requires it. [CHECK]

## 6. AI-written content and its sources

The stories in Autopaper are written by AI agents. They fetch public web pages and feeds, then write a story that cites those sources. Each story shows its sources as links. Stories can be wrong. Check the sources before you rely on a story. The agents can make mistakes in facts, names and summaries. You can report a problem with a story from the story page. [WHEN-BUILT: the report action does not exist today.]

Stories are public. They are not personal data about you unless a story is about you.

## 7. Voice

Plainly:

- Voice is off when the app opens. It turns on only when you tap the Voice control.
- When Voice is on, the microphone is open. With the screen locked, the microphone can stay open and the iPhone shows its microphone indicator. Voice turns itself off after three minutes of silence from you. You can turn it off at any time.
- Your speech is turned into text on your phone. The audio is not sent to us or to Apple for recognition.
- The text of your request goes to your computer and to the AI models in section 5, as described in section 2.
- Your computer writes the text of your request and the answer on its event log. The log is kept for the life of the computer (see section 8).
- Each voice turn is saved on your phone as audio, as described in section 3. [CHECK]
- The app asks for the microphone and speech recognition permission only when you turn Voice on. If you say no, the radio, paper, reader and podcasts still work.

The app downloads voice and language model files to your phone. They are data files, not programs. The app asks before it downloads and shows the size. The files come from Hugging Face today. [DECISION: or from our own host]

## 8. How long we keep data

| Data | How long |
|---|---|
| Email address, passkey public key | Until you delete your account |
| Sign-in sessions | Until they expire (up to 30 days for the refresh session) or you sign out |
| Email sign-in and recovery attempts | Codes expire in 10 minutes for sign-in and 15 minutes for recovery. Attempt records are removed after 24 hours. |
| Your computer's working data (documents, files, mail, notes, play history) | Until you delete your account |
| The event log of your computer, including the text of your requests and answers | For the life of the computer. It is sealed under a key that belongs to the computer. We do not edit it. It is removed when your computer is destroyed, as in section 9. |
| Server logs | Our code sets no fixed period. [CHECK: the existing web policy says 90 days for authentication logs; no job enforcing that was found. Set a period and enforce it, or remove the number.] |
| Voice audio kept by the app on your phone | Until you sign out or delete the app [CHECK] |
| Data held by outside providers | Set by each provider |

## 9. Deleting your account

You can delete your account in the app: Settings, Account, Delete account. [WHEN-BUILT] The app asks you to confirm with your passkey. [WHEN-BUILT]

Deleting your account starts these steps. [DECISION: this describes the design the owner has ordered]

- It removes your account, your email address and your passkeys. [WHEN-BUILT]
- It destroys your computer and the key that seals its event log, which makes the log unreadable. It removes your documents, files, mail, notes, play history and takes. [WHEN-BUILT] [DECISION: confirm that destroying the computer is the meaning of deletion]
- It signs you out on every device. [WHEN-BUILT]
- Stories that Autopaper published are public. [DECISION: we remove them / we keep them unless a story contains your personal data. The existing web policy keeps them.]
- Deletion cannot be undone. [WHEN-BUILT]
- It does not delete data already sent to outside AI model providers, which follow their own terms. We will ask a provider to delete where its terms allow.
- It happens at once or within DELETION_TIME [DECISION: immediate, or a stated delay].

Until in-app deletion exists, write to CONTACT_EMAIL and we will delete your account by hand. [CHECK: only say this if it is true at the time the policy is published; the existing web policy says the log is not rewritten and whole-computer deletion is planned.]

You can also sign out at any time. Signing out removes the key and your local data from the phone.

## 10. Your choices and rights

You can see, change and delete much of your data in the app. You can ask us for a copy of the data we hold about you, to correct it, or to delete it, by writing to CONTACT_EMAIL. You can withdraw your agreement to AI processing in Settings [WHEN-BUILT], and you can turn the microphone off in iPhone Settings.

If you live in the European Economic Area, the United Kingdom, California or another place whose law gives you rights over your personal data, you have the rights that law gives you. These usually include access, correction, deletion, a portable copy, objection to some processing, and a complaint to your data protection authority. We do not use your data for advertising and we do not sell it. We reply within the time the law sets. [CHECK: lawyer to confirm the lawful bases, the transfer terms for providers outside your country, and whether a representative is needed. We make no claim of certification.]

## 11. Children

Autoradio 1 is not directed to children under 13, and we do not knowingly collect personal data from them. [DECISION: the existing web policy says under 16; choose one age and use it here and there.] The App Store age rating is 16+ because the news covers war, crime and other mature subjects. [DECISION: confirm the rating.] If you believe a child has given us personal data, write to CONTACT_EMAIL and we will delete it.

## 12. Security

- Sign-in uses passkeys. There are no passwords to steal or reset.
- The app talks to our server over HTTPS.
- Your sign-in key on the phone is in the iPhone Keychain, on this device only.
- Mail kept on the phone is encrypted with a key in the Keychain.
- The event log of your computer is sealed under a key that belongs to the computer. [CHECK]
- Each account's computer is a separate virtual machine with its own disk. Provider keys are held by our gateway and are not given to computers. We operate the server, so people who run the service can reach the data it holds. [CHECK: owner to state who has operator access.]
- We do not claim any security certification. No system is perfectly secure. [CHECK]

Cookies at choir.news: the sign-in service sets cookies for sign-in only. They are choir_access (5 minutes), choir_refresh (up to 30 days), choir_accounts (up to 30 days, for switching accounts) and choir_signin_attempt (10 minutes, for email sign-in). They are secure, HttpOnly and SameSite=Lax. We set no advertising or tracking cookies. The app signs in through the iPhone's sign-in sheet, which shares the system browser's passkeys and cookies. [CHECK]

## 13. Changes to this policy

We will put the new date at the top and, for a material change, show a notice in the app and on choir.news before it takes effect. The list of providers in section 5 changes more often than the rest. Past versions are kept at POLICY_ARCHIVE_URL. [DECISION: where to keep them]

## 14. Contact

Write to CONTACT_EMAIL. PUBLISHER_NAME publishes the app. [CHECK: add the postal address if Apple's EU trader rules or the law require it.]
