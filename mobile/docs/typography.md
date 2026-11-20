# Autoradio typography (owner, 2026-11-16)

Owner: "We need to have more typography and use typography to signify what's the difference between static text and a button and a headline. Typography is too same same, sans serif."

The face tells the reader what a piece of text is. Serif is for things you read; sans is for things that are about the page or that you press. The weight then separates the two sans roles. Faces are the iOS system's: New York (`ui-serif`), the system sans (`system-ui`, or `ui-rounded` for a theme that asks for it), `ui-monospace`. No font files. Contracts name a role token (`th.fontHead` and the rest), never a face; the roles are fixed and a theme no longer chooses the serif or the sans (it may still choose the rounded sans). `tests: mobile/AutoradioLab/contract-typography-rules.test.ts`.

Sizes are system text styles (`font-size="-exact-footnote"`), which follow Dynamic Type. A size that comes from data (a story's headline size, the theme scale) goes through `dyn(px)` (rem, which iOS scales from the body size; at the default setting it is the same number of points).

| Role | Token | Face | Weight | Size | Where | Why |
| --- | --- | --- | --- | --- | --- | --- |
| Headline | `th.fontHead` | New York | 700 at title2 and up; 600 for a row title (an object's name in a list) | large-title to subheadline | page titles, the masthead, a story's title, a now-playing title, file, mail and show names | a name is set like a newspaper headline; the row title is the same face a step down, so a list of names reads as titles, not as buttons |
| Reading text | `th.fontText` | New York | 400 (600 only to show state: unread, the sentence being read) | body 17, line height 1.4 to 1.65 | story paragraphs, documents, deks, the passage being spoken, answers, text fields | what you read at length is set for reading |
| Spoken line | `th.fontText` + italic | New York italic | 400 | as Reading text | the words of the conversation: what you are saying (live transcript), what was heard, the radio's holding line | its own role, because these words are in the air and not on the page; italic says so and costs no new face. The passage being read aloud from a document stays Reading text |
| Label | `th.fontLabel` | system sans | 400 | footnote, caption1, subheadline | metadata ("18 sources · 35 min ago"), notes, empty states, errors, captions | small, regular, muted: it describes, it never invites a press |
| Label, caps | `th.fontLabel` | system sans | 600, uppercase, letter-spacing 0.8 | caption1 or caption2 | section names, "UP NEXT", "NOW PLAYING" | tracking goes with capitals and only with them |
| Action | `th.fontAction` | system sans | 600 always | the text's own size, usually subheadline | any tappable word: Play, Queue, Sections, Cancel, Sign in, menu items, choice words, tabs, citation marks | semibold sans is the one treatment of a press. It is the only semibold sans; a Label is regular, a Headline is serif, so in grey the button is still the heavy sans among serifs and light sans. Colour (accent, muted for secondary, danger for destructive) is a second cue |
| Figure | Label + `font-variant-numeric="tabular-nums"` | system sans | as Label | as Label | elapsed, remaining, speed, positions, counts, list numbers | digits that change in place do not jitter. Monospace is not used for figures: tabular figures give the alignment without the typewriter look |
| Code | `th.fontMono` | system monospaced | as set | footnote | the sign-in code, addresses, trace and lab pages, code blocks | the only monospace; it earns it where characters must be told apart |

Rules the tests keep: every `font-family` is a role token; every `text` has one on it or above it; no bare point sizes on text; an Action is always weight 600 and never uppercase, tracked or italic; a Label is regular unless it is the caps label; uppercase and tracking only on the caps label (a masthead may be tracked); italic only on serif; an Action is inside a button, presses, or is the face component of one; a short word inside a button is an Action; the named figures are tabular.

Ternary roles exist on purpose: the breadcrumb (the current crumb is a Label, the others Actions), the player status line (the listener's words are a Spoken line, the radio's status a Label), a settings value (Code for a path, Label otherwise) and a Reader block (Headline for a heading, Reading text otherwise).

Inline links in a document stay Reading text in the accent colour (a link in a paragraph is read, then pressed). Citation marks in a paragraph are Actions.

## A tappable name (owner, 2026-11-16: type must tell a button from a headline)

One rule for every tappable name of a thing:

- **Navigation with no content behind the word** (the seven key pages: Paper, Textures, Files, Mail, Reader, Podcasts, Settings) is an **Action**: semibold sans in the accent colour.
- **A tappable title of an object** (a deck row, a search hit, a document, a mail row, a queue item, a source, a now-playing title) stays a **Headline**, and its button carries an action-role affordance: a forward chevron, a "›", an Action word, or a selection mark. The serif alone never says "tap".
- Named exceptions, in the test with reasons: the paper's story headlines (Play and Queue Actions sit beside them), a book's reading blocks, file tiles, a topic row.
- A short word inside a button is always an Action. The content row of a document or web page has no word "Close": the X on its line closes it.
- The ask field shows its placeholder as a Label (sans, muted) and typed words as Reading text: its face is `(ask == "" ? th.fontLabel : th.fontText)`, because a placeholder takes colour only.
