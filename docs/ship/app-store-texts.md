# App Store texts for Autoradio 1

Limits are from the research doc (read 2026-11-23): name 30 characters, subtitle 30, promotional text 170, description 4,000, keywords 100 bytes. No other platforms' names (2.3.10), no prices (2.3.7), no unverifiable claims, no placeholder text. The description is written for the version-1 default scope in the shipping mission (radio, paper, reader, voice, podcasts). [WHEN-BUILT] marks a sentence that is not true of today's code. Remove tags before pasting. Mail, files and takes are left out of the description; add them only after each has been used on the owner's phone.

## Name

Autoradio 1 (11 characters). Check that the name is free when the app record is made.

## Subtitle candidates (30 characters or fewer)

1. News read aloud, with sources (29)
2. A radio that reads the news (27)
3. Spoken news from cited sources (30)
4. Listen to AI-written news (25)
5. The paper, read to you (22)

Recommended: 1 or 3. Both say what it does and name the sources. 4 says AI in the first line, which helps the review.

## Promotional text (170 characters or fewer)

```
Autoradio 1 reads the day's news aloud. Every story is written by AI from cited web sources and links to them. Listen with no account. Make one to shape your own edition.
```

(Exactly 170 characters. Description is about 2,250 characters. Keywords are 91 bytes.)

## Description (under 4,000 characters)

```
Autoradio 1 is a radio that reads a news paper aloud.

The paper is called Autopaper. Its stories are written by AI agents from cited web sources. Every story shows its sources as links, so you can open the original and check it. Stories can contain mistakes. The app says plainly that they are written by AI.

Press Play and listen. You do not need an account. The radio reads the front page with the voice built into your iPhone, and it keeps playing with the screen locked, with controls on the lock screen.

With an account you get more:

A personal edition. Choose the topics you care about, by tapping or by speaking. Tap more of this or less of this on any story and the radio adjusts.

Talk to it. Turn on Voice and ask about the story you are hearing, or tell the radio to skip, pause or play something next. Your speech is turned into text on your phone. The text of a request is sent to your account's computer on our server and to outside AI models that write the answer. The app asks for your agreement before the first request that does this. [WHEN-BUILT]

A reader. Read a story or a document, see its sources and its versions, and hear it read aloud.

Podcasts. Search for shows, subscribe, and listen. Episodes stream from the publisher.

Notes. Keep notes that your computer remembers.

Your computer. Each account has its own computer on our server. It holds what you make and what you ask the radio to remember.

Sign in with a passkey. There are no passwords. You can make an account in the app, and you can delete it in the app. Deleting removes your account, your computer and your data. [WHEN-BUILT]

What to know before you use it:

Stories are written by AI, not by reporters. The sources are cited and linked. Please check them for anything that matters.

Voice uses the microphone only when you turn it on. With the screen locked, Voice can stay on for up to three minutes after you last speak. The iPhone shows its microphone indicator while it is on.

The app can download speech and language model files to your phone. They are large, so the app tells you the size and asks first. [WHEN-BUILT]

The news can include war, crime and other serious subjects.

Privacy policy: PRIVACY_POLICY_URL
Support: SUPPORT_URL
```

## Keywords (100 bytes or fewer, comma separated)

```
news,radio,audio news,podcast,headlines,briefing,listen,voice,sources,current events,reader
```

(Count the bytes in App Store Connect; the field counts commas. Do not repeat words from the name or subtitle. Do not use competitor names, "AI" as a rank trick, or other platforms.)

## Categories

- Primary: News.
- Secondary: leave empty, or Reference. Do not choose Music or Entertainment. Do not choose Kids. [DECISION]
- Age rating: answer honestly. The research recommends 16+ (frequent mature themes in news, no unrestricted web access). [DECISION]
- Content rights: answer yes to "contains third-party content" and state that stories cite sources with links and podcasts stream from publishers' own feeds. [CHECK]

## Screenshot plan (iPhone only, one large size)

Capture at 1320 x 2868 on a slim simulator named for the run; Apple scales to smaller phones. No placeholder text, no fixtures, real published stories pinned for the run. Show the app in use, not a title card. No prices, no other platforms. Captions are six words or fewer and sit above the screen on a plain ground.

| # | Screen | What it shows | Caption |
|---|---|---|---|
| 1 | Radio, playing | The Autoradio page with a story on air, the Play control and the up-next list. | The paper, read to you |
| 2 | Lock screen | The lock screen with the title, the progress and the controls. | Keeps playing with the screen locked |
| 3 | A story with sources | A story open with the citation chips and the Sources list visible. | Every story links to its sources |
| 4 | The AI label | The line that says the story is written by AI from cited sources, with a story below it. | Written by AI, from cited sources |
| 5 | Voice | Voice on, the words the radio heard, and the answer starting. | Ask about the story you hear |
| 6 | Edition topics | The topic tree with some topics chosen. | Choose the topics you follow |
| 7 | Podcasts | Search results and a subscribed show. | Podcasts stream from the publisher |
| 8 | Sign in | The sign-in sheet with a passkey, signed out state behind it. | Listen first. Sign in later. |

Screens 4 and 8 need the AI label and the signed-out sign-in sheet to exist in the build. [WHEN-BUILT]

If voice, sound or the lock screen have not been used on the owner's phone by the time of capture, drop their screens. Show only what has been used. (Research risk 4.)

## Preview storyboard (20 seconds, captured footage only)

Apple allows 15 to 30 seconds, 886 x 1920 portrait, muted by default, captured from the app itself. Captions are allowed; narration is allowed. No people filmed with a device. No prices or dates. Say that sign-in is optional.

| Seconds | Captured footage | Caption |
|---|---|---|
| 0 to 3 | Cold launch. The front page loads on the radio screen. | Open the app |
| 3 to 7 | Tap Play. The first story starts and the passage highlights as it is read. | Press Play. No account needed. |
| 7 to 10 | Lock the screen. The lock screen shows the title and the controls. | Keeps playing when locked |
| 10 to 14 | Unlock, open the story, scroll to the sources and tap one. | Every story links to its sources |
| 14 to 18 | Turn Voice on, say a short question, show the heard words and the answer. | Ask about what you hear |
| 18 to 20 | The radio screen again, playing. | Autoradio 1 |

Encode: H.264 at 30 fps or lower, 10 to 12 Mbps, stereo AAC at 256 kbps. Narration, if any, is mixed in afterwards because the simulator's audio is not captured. Use en-US only for version 1.

## What's New (version 1.0.1 and later)

Plain user sentences from the change log. Not required for version 1.
