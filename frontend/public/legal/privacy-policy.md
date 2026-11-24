# Privacy Policy for Autoradio 1 and choir.news

Effective date: 2026-11-24
Published by: Choir Harmonic Intelligence Platform, Inc. ("we", "us")
Contact: 000@choir.news

## 1. What this covers

This policy covers the Autoradio 1 app for iPhone and the website at choir.news, including the sign-in pages and the paper. It does not cover the websites of the sources that stories cite, the podcast publishers you listen to, or Apple's services. Those have their own policies.

Autoradio 1 is a radio that reads a news paper aloud. The paper is called Autopaper. Its stories are written by AI agents from cited web sources. You can listen without an account. With an account you get a personal edition, you can talk to it, and you get mail, files, a reader, podcasts, takes and a notebook on a computer that is yours.

Read this first. During the beta, every account is on the free tier. What you say or type to the radio goes to outside companies that run AI models, and some of them may keep it and use it to train their models. Section 5 says what is sent and what to do about it.

You must be 16 or older to use an account (section 11).

## 2. What we collect and why

### Listening without an account

Signed out, the app is listening only. It asks our server for the public front page and the stories. Our server receives your IP address and ordinary request details such as the app version. We do not ask for your name or email. A spoken or typed request cannot reach a model when you are signed out. If you ask the radio something, the app asks you to sign in or make an account.

### Creating an account

You make an account inside the app, with the phone's own passkey screen. We collect your email address and a passkey. The passkey gives us a public key and some details about it, such as when it was made. The private key stays on your device. No person's account has a password, so we never see or store one. The one exception is an account we keep for Apple's App Review, which signs in with a user name and password; we store that password only as a hash. The app asks for no codes. We use the email address to identify your account and for recovery. It is the same account on choir.news.

### Your computer

Every account has its own computer on our server. It is a virtual machine with its own disk. It holds what the app does for you: your edition topics, your play history, the notes and documents you make, files you import, your mail, your podcast subscriptions and your taps of "more of this" and "less of this". We use this to run the features you asked for and to personalise your edition.

### Listening history and your edition

While you are signed in, the app tells your computer what played, what you skipped and what you finished, with times. Your computer uses this so the radio does not repeat what you heard and so it can choose what comes next. Your edition topics and this history are also sent to an outside AI model each time the radio writes your next stretch. See section 5.

### Talking to the radio

If you turn on Voice and speak, the app turns your speech into text on your phone. The text of your request goes to your computer. It goes with the title and text of the page you have open (which can be a story, a document or a mail message), the last few lines of the conversation, your play queue, and a list of candidate stories. Your computer sends this to an outside AI model to write the answer. When you ask about your mail or files, that content goes too. Section 7 says exactly what is recorded and where.

### Podcasts

Podcast search and the charts go from your phone to Apple's podcast services, not through us. Podcast feeds and episodes are fetched from each publisher's own server, which can see your phone's IP address. We store the feeds you subscribe to and the episode you are playing so your computer can keep your library in step.

### Takes

A take is a short recording of your voice, up to 60 seconds. It is uploaded only when you press Publish. After that it is offered to other Autoradio listeners. You can delete it at any time. A take that contains music or another person's recording will be removed. The microphone is asked for only when you turn on a take.

### Logs

Our servers keep logs so we can run the service and find faults. A sign-in service log line has the route, the result, the first two parts of your IP address, the first eight characters of your account id and a short hash of your session. It has no email, token or cookie. Our web server's access log is different: it holds the full IP address, the path and the user agent of every request. Our gateway logs searches that your computer's agents make, with the search text and your computer's id. The log line for a voice turn carries counts and sizes, not the text. The text of your requests and the answers is kept in your computer's event log (section 8), not in these server logs.

### What we do not collect

We do not use advertising, tracking or analytics software in the app or on the website. We do not collect your location. We do not read your contacts or photos. The app asks Apple for no advertising identifier. We do not run a crash reporter. If you use TestFlight or the App Store, Apple's own crash and feedback reports reach us through Apple, under your iPhone's settings.

## 3. What stays on your phone

Speech recognition runs on your phone, with speech models the app downloads. Your audio is not sent to us, to Apple or to anyone else to be recognised.

Your sign-in key is in the iPhone Keychain, on this device only. If the app keeps a copy of your mail on the phone, it is encrypted with a key in the Keychain. The paper's cached front page, your edition topics, your taps, your podcast library and your settings are kept in the app's own files.

The app keeps a short record of your recent voice turns on your phone, in its own storage: the words it heard, what it did and when. It keeps the newest 20 turns and none older than 30 days. It does not save the sound of your voice. The record is not uploaded. It is deleted when you sign out or delete the app.

## 4. What is sent to our servers

When you are signed in, the app sends the following to your computer on our server over HTTPS:

- the text of your spoken or typed requests, with the context listed in section 2;
- what you played, skipped and finished;
- your edition topics, and the time zone and local time of your phone;
- your taps of "more of this", "less of this" and mute;
- your podcast subscriptions and the episode you are playing;
- documents and books you import, files and notes you make, and mail you read or write;
- a published take, if you publish one.

We do not send your microphone audio to our servers, except in a take you publish.

## 5. Outside services that process data

### AI models, and what they may do with your data

During the beta, every account is on the free tier. What you say or type to the radio, and the content sent with it, goes to outside companies that run AI models. That content is the page you have open, the last few lines of the conversation, your topics and what you recently heard, and your mail or files when you ask about them. Some of these companies may keep it and use it to train their models. Do not put anything into the app that you would not want used that way. A paid tier with stronger protections is planned. It is not available yet.

Deleting your account does not delete what these companies already hold, and we cannot promise that they will delete it. Each of them follows its own terms.

### The list

We use outside services to run the product. They process data on our behalf. The list changes as we change providers. We update this table before a new provider receives data.

These are the categories, and the providers in our code on 2026-11-24.

- AI models that write answers and stretches of radio. They get your spoken request text and the context above, your edition topics, time zone, local time and a list of stories you recently heard. For work on your computer, such as documents, your edition and research, they get the instructions and content of that work. Providers today: River (DeepSeek models); Amazon Web Services Bedrock (Anthropic Claude models); OpenRouter; OpenAI (ChatGPT models); Devin; OpenCode; Synthetic.
- AI model for quick decisions. It gets the text of your request and a line saying what is playing. Provider today: OpenRouter.
- Web search. It gets search queries written by agents on your computer, which can include your topics. Providers today: Keenable, SearXNG, Brave, Serper, Parallel, Tavily, SerpAPI, ChatGPT search.
- Email delivery. It gets your email address and the body of messages we send for recovery, web sign-in and the Mail feature. Provider today: Resend.
- Model file hosting. It gets your phone's IP address and which model file you asked for. Provider today: Hugging Face.
- Podcast search and charts. They get the search text you type. Provider today: Apple.
- Hosting. It holds everything stored on our server. Provider today: OVH (bare-metal server, in the United States).

You agree to this when you create an account: the create-account screen says it above the button, with a link to this policy. Listening to the paper needs no account and sends none of your content to a model. If you no longer agree, delete your account (section 9).

We do not sell your data. We do not share it for advertising. We share it only with the providers above, to run the product, and where the law requires it.

## 6. AI-written content and its sources

The stories in Autopaper are written by AI agents. They fetch public web pages and feeds, then write a story that cites those sources. Each story shows its sources as links. Stories can be wrong. Check the sources before you rely on a story. The agents can make mistakes in facts, names and summaries. To report a problem with a story, write to 000@choir.news.

Stories are public. They are not personal data about you unless a story is about you.

## 7. Voice

Plainly:

- Voice is off when the app opens. It turns on only when you tap the Voice control.
- When Voice is on, the microphone is open. With the screen locked, the microphone can stay open and the iPhone shows its microphone indicator. Voice turns itself off after three minutes of silence from you. You can turn it off at any time.
- Your speech is turned into text on your phone. The audio is not sent to us or to Apple for recognition.
- The text of your request goes to your computer and to the AI models in section 5, as described in section 2.
- Your computer writes the text of your request and the answer on its event log. The log is kept for the life of the computer (see section 8).
- The app keeps a short record of your recent voice turns on your phone, without the sound, as described in section 3.
- The app asks for the microphone and speech recognition permission only when you turn on Voice or a take. If you say no, the radio, paper, reader and podcasts still work.

The app downloads voice and language model files to your phone. They are data files, not programs. The app downloads them only after you agree, and it shows the size first. The files come from Hugging Face today.

## 8. How long we keep data

- Email address and passkey public key: until your account is deleted (section 9).
- Sign-in sessions: until they expire (up to 30 days for the refresh session) or you sign out.
- Email sign-in and recovery attempts on choir.news: codes expire in 10 minutes for sign-in and 15 minutes for recovery. Attempt records are removed after 24 hours.
- Your computer's working data (documents, files, mail, notes, play history): until your account is deleted.
- The event log of your computer, including the text of your requests and answers: for the life of the computer. It is sealed under a key that belongs to the computer. We do not edit it. It is removed when your computer is destroyed, as in section 9.
- Server logs: we do not yet delete them on a fixed schedule, so we make no promise of a deletion time for them. We are building a limit of at most 30 days for our host's request, service and login logs, and we will change this line to say so when it is in force. A person's own computer and the logs on it follow the computer (above).
- Abuse logs after you delete your account: see section 9.
- Deletion receipt: kept. It holds a job number, counts and times, and no content.
- The record of your recent voice turns on your phone: the newest 20 turns, none older than 30 days, and all of it removed when you sign out or delete the app.
- Data held by outside providers: set by each provider. We cannot promise they delete it.

## 9. Deleting your account

You can delete your account in the app (Settings, Account, Delete account) or at choir.news/account/delete. You confirm with your passkey.

What happens:

- Access is revoked at once. Every sign-in and key for your account stops working, and you are signed out on every device.
- Nothing is erased for 72 hours. If you sign in during those 72 hours, the deletion is cancelled.
- After 72 hours everything below is permanently removed from our servers: your account, your email address and your passkeys; your computer with its disk, its stored history (including the log of your requests and answers) and the key that seals it, and everything on it (documents, files, mail, notes and play history); your mail and its address; your synced documents; and your takes, their words and their audio.
- Stories that Autopaper already published stay public. Your name is detached from them.
- We keep a deletion receipt (a job number, counts and times, no content) and our server logs for as long as section 8 says. No archive of your computer is kept. Mail you sent or received stays with the people on the other end and with our mail provider.
- Our encrypted off-site backups still hold a copy of your account for up to three weeks, and it is deleted with them. If a backup is ever restored, the deletion is carried out again.
- After the 72 hours, deletion cannot be undone.
- Deletion does not remove data already sent to outside AI model providers. Section 5 says why.

You can also sign out at any time. Signing out removes the key and your local data from the phone.

## 10. Your choices and rights

You can see, change and delete much of your data in the app. You can ask us for a copy of the data we hold about you, to correct it, or to delete it, by writing to 000@choir.news. You can withdraw your agreement to AI processing by deleting your account (section 9), and you can turn the microphone off in iPhone Settings.

If you live in the European Economic Area, the United Kingdom, California or another place whose law gives you rights over your personal data, you have the rights that law gives you. These usually include access, correction, deletion, a portable copy, objection to some processing, and a complaint to your data protection authority. We do not use your data for advertising and we do not sell it. We reply within the time the law sets. We make no claim of certification.

## 11. Children

Autoradio 1 is for people 16 and older. Do not make an account if you are under 16. We do not knowingly collect personal data from anyone under 16. The App Store age rating is 16+, because the news covers war, crime and other mature subjects. If you believe a child has given us personal data, write to 000@choir.news and we will delete it.

## 12. Security

- Sign-in uses passkeys. No person's account has a password to steal or reset. One account kept for App Review has a password, stored on our server only as a hash.
- The app talks to our server over HTTPS.
- Your sign-in key on the phone is in the iPhone Keychain, on this device only.
- The event log of your computer is sealed under a key that belongs to the computer.
- Each account's computer is a separate virtual machine with its own disk. Provider keys are held by our gateway and are not given to computers. We operate the server, so the operator can reach the data it holds. Today one person operates it: the founder of Choir Harmonic Intelligence Platform, Inc.
- We do not claim any security certification. No system is perfectly secure.

Cookies at choir.news: the sign-in service sets cookies for sign-in only. They are choir_access (5 minutes), choir_refresh (up to 30 days), choir_accounts (up to 30 days, for switching accounts) and choir_signin_attempt (10 minutes, for email sign-in). They are secure, HttpOnly and SameSite=Lax. We set no advertising or tracking cookies.

## 13. Changes to this policy

We will put the new date at the top and, for a material change, show a notice in the app and on choir.news before it takes effect. The list of providers in section 5 changes more often than the rest. Earlier versions are kept in the public history of https://github.com/choir-hip/go-choir (the file frontend/public/legal/privacy-policy.md).

## 14. Contact

Write to 000@choir.news, or by post to:

Choir Harmonic Intelligence Platform, Inc., c/o Yusef Mosiah Nathanson, 1414 Andover St, Tewksbury, MA 01876.
