# Swift page logic to move to Exact (read-only survey, 2026-11-20)

Smallest first. "Test counts" are `func test` counts per Swift file; the test bodies were not read.

| # | Page | Swift page logic | Size | Already in Exact? |
|---|---|---|---|---|
| 1 | edition picks | `AutoradioModule+Paper.swift` 8-17; `EditionSetup.swift` 375-380 (`load`) | ~20 | Mostly: `edition-model.ts` (380 lines) ports `EditionSetup.swift`; `edition.test.ts` |
| 2 | conversation info | `Conversation.swift` 135-155 (`ConversationInfo.make`); `AutoradioModule+Talk.swift` 30 | ~25 (+120-line entry store the voice turn writes) | No |
| 3 | texture edit info | `TextureEditor.swift` 39-150; `AutoradioModule+TextureEdit.swift` 15-39 | ~200 | No |
| 4 | themes | `ThemeLoader.swift` 159-330, 357-545; `AutoradioModule+Spine.swift` 33-38, 142-145; `+Settings.swift` 69-106 | ~450 (+239 generated) | No (Android uses the default theme) |
| 5 | Textures place | `TextureReaderSource.swift` 234-292; `AutoradioModule+Reader.swift` 111-117 | ~70 | No; `reader.ts` covers only `pub:` |
| 6 | takes | `TakesPage.swift` 1-70; `AutoradioModule+Takes.swift` 68-108; data half of `Choir/Takes.swift` (437), `TakeSession.swift` (295) | ~800 | Mostly: `takes.ts` (383), not yet wired in `app.ts` |
| 7 | books shelf | `ReaderShelf.swift` (507), `Epub.swift` (461), `AutoradioModule+Books.swift` | ~1000 | Barely |

## 1. Edition picks
Reads `edition-topics.json`, returns topics joined by newlines.
- Native-only: nothing in principle (a file read), BUT the file is written by Swift topic screens and read by `Notebook.swift:130,343`, the saved corpus (`SavedCorpus` "edition-topics") and `AccountWipe.swift:88`. Moving the read alone removes no Swift.
- Exact: `picks()` in `edition-model.ts`; grant on `app:/data/edition-topics.json`; the path moves from Application Support to `app:/data`, so the writer must move too.
- Tests: E7, E9 of `EditionSetupTests` (28), mostly ported in `edition.test.ts`.

## 2. Conversation info
Reverses entries, drops cancelled ones, sets `canRetry` on failed / noted-answer rows.
- Native-only: none for the formatter; entries are written by the voice turn engine, so the store is shared state written through a one-way native call.
- Exact: `conversation.ts` with `info(entries)`, an `app:/data` store (limit 30); contract `conversationInfo` source plus a `conversationNote` send.
- Tests: `TalkTests` (31), the entry-state ones.

## 3. Texture edit info
Per-document save state (idle, dirty, saving, saved, offline, failed); debounced edit, `head` and `post` to `/api/texture/...`, retry from a pending file.
- Native-only: background-flush notifications (`Platform.flushNotes`).
- Exact: `texture-edit.ts` on the `api.ts` authed client and `app:/data` pending storage; shares markdown body code from `TextureBody.swift` (381), `TextureDoc.swift` (758, partly). Contract `textureEditInfo`, `textureEdit`, `textureFlush`, plus a flush-on-background host event.
- Tests: `TextureEditorTests` (7), `TextureBodyTests` (6), `TextureDocTests` (14).

## 4. Themes
Theme JSON (colours, type, radii, motion, mark). `ThemeStore` reads bundled + user themes, validates, resolves per scheme with a contrast check, lists; choice in `UserDefaults`.
- Native-only: system colour scheme and Reduce Motion (host facts); user theme directory (Exact: `app:/data/themes`).
- Exact: `themes.ts` (validate, resolve, contrast, list); bundled JSON generated from `mobile/themes/*.json` into a TS module, replacing `BundledThemes.generated.swift`; choice in an `app:/data` file; sources `theme`, `themeList`, `themeExtras`, `themeCards`; sends `setTheme`, `setMode`, `installFromTexture`, `remove` (Texture theme fetch through the API client).
- Tests: `ThemeDefaultsTests` (12), `SettingsDataTests` (29, 23 use `ThemeStore`), theme checks in `DesignRulesTests`, `TypographyRulesTests`.

## 5. Textures place
Lists `/api/texture/documents?limit=100` as rows, pins the edition first (doc with `metadata.role == "edition"`, a title starting "My edition", or the id from `GET /api/texture/edition`).
- Native-only: nothing. Needs the authed client and the signed-out words from `account-state.ts`. Swift says "Sign in" when the key is only locked (`TextureReaderSource.swift:286`).
- Exact: `textures.ts` (`parse`, `applying`, `load`), a `texturesList(at)` source in `app.ts`; rows become `texture:<id>` pages; `reader.ts` must resolve `texture:`.
- Tests: list parts of `TextureDocTests`, `RealShapeTests`.

## 6. Takes (moved 2026-11-20, lane exact-takes; see the problem record's last update)
Model already shared. Left: wire `takesPage` and `takesAct` in `app.ts`, split the recorder state machine out of `TakeSession.swift`.
- Native-only (voice, stays): microphone and the take gate (`TakeGate`), `AVTakeRecorder`, `FileTranscriber`, `AVTakePreview`; fetching another listener's take for the run sheet (writes a file the player deletes, `+Takes.swift` 53-60).
- Exact: `takes.ts` plus a session reducer (`take-session.ts`); contract `takesPage(recorder)` taking native recorder state, `takesAct` calling native `record`, `stop`, `preview`, `discard`, then publishing through `client`.
- Tests: `takes.test.ts` (T1-T9) covered; port `TakesTests` (22), `TakeSessionTests` (23).

## 7. Books shelf
EPUB and PDF books (list, import, remove), cut into sections and blocks, inverted-index search, reading position, follow cursor, `ShelfPlayback` hands blocks to the run sheet.
- Native-only: PDF page view (file copy into `app:/cache/files`); read-aloud and enqueue (`RunSheetPlayer`, audio).
- Exact: `shelf.ts`: EPUB reader (zip + XML, `xml.ts` exists), `DocExtract`; PDF text extraction is the hard part (native op or show PDFs by `src` only); storage `app:/data/books`; sources `shelfList`, `shelfDoc`, `shelfChapter`, `shelfFind`, `shelfFollow`; sends `shelfImport`, `shelfRemove`, `shelfPlace`, `shelfReadAloud`, `shelfEnqueue` (the last two keep calling native `runSheet` ops).
- Tests: `ReaderShelfTests` (23), `EpubTests` (6), `LibraryCorpus` (shared with search retrieval; `SavedCorpus.setSource("library", ...)` needs care).

Order: 1 and 5 cheapest (but see the note on 1), then 2; themes (4) is large with no native dependency beyond two host facts; the shelf (7) last.
