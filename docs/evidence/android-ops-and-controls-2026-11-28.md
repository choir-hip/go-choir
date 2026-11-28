# Android: every native op, who answers it, and every control that calls it

Standing: governing | kind: happened | settled-by: a reading of origin/main 46a18adac on 2026-11-28 | note: read from code, nothing run on a phone

The Kotlin module answers `NotSupportedError` for any name it does not
handle (`mobile/exact-package/android/AutoradioModule.kt`, the `else` of
`later`, line 97 at 46a18adac). Exact then gives the declared default.
So a control that calls such an op is drawn and does nothing.

Kotlin answers 7 of the 31 interface names. Swift answers all 31
(`handle("...")` in `mobile/exact-package/apple/AutoradioModule+*.swift`,
and two `case` lines in `AutoradioModule.swift:215-216`). Kotlin also answers
four `nativeModel*` ops that are in no interface file, so no shared code can
call them today.

## The table

"Kotlin" is the line in `AutoradioModule.kt` at origin/main 46a18adac. "Swift" is the
file that handles it. "Controls" are the contract places that call or read it.

| Op | Kind | Swift | Kotlin | Controls and pages that call it | Android before |
| --- | --- | --- | --- | --- | --- |
| `readerOpenExternal` | send | +Reader:94 | answered, :76 | Source Viewer "open in browser", mail links | works |
| `deckState` | source | +Deck:50 | answered, :84 | the bar's playing line, the Autoradio page, radio pump | works |
| `deckPlay` `deckEnqueue` `deckAct` `deckAck` `deckWipe` | send | +Deck:51-55 | answered, :85-93 | the play button, up next, skip, sign-out wipe | works |
| `choirTalkCancel` | send | AutoradioModule.swift:216 | no | bar "Cancel" while listening (bar.contract, `micCancel`) | only drawn while `heardNow`, which is never true on Android |
| `choirHandsFreeToggle` | send | +Spine:51 | no | the bar's microphone button (`micTap`, `micContinue`) | drawn; opened "Voice and models" while no hearing model, else did nothing |
| `choirAsk` | send | +Spine:63 | no | the bar's Send and Enter (`sendAsk`); mail "Summarise this thread" (`askMail`); Texture list "Make your edition" (`editAsk` prefill) | drawn; the text was cleared and nothing answered |
| `radioSpeed` | send | +Bar:71 | no | the speed row of the Autoradio page (nudge, slider, reset: `setSpeed`, `speedEnd`, `speedNudge`, `speedReset`) | drawn; did nothing |
| `choirPlayer` | source | AutoradioModule.swift:215 | no (default) | account.contract `pl`: the player's plain message | default is empty: correct |
| `barFacts` | source | +Bar:58 | no (default) | spine.contract `bf`: voice phase, transcript, hands-free, errors | default is a phone with no voice turn: correct |
| `radioView` | source | +Bar:63 | no (default) | members-bar.contract `ri`: yield level, speed value, mic level | default `play`, 1, 0: correct |
| `radioNote` | source | +Queue:11 | no (default) | spine.contract `rnote`: native asks the pump to run again | nothing to ask: correct |
| `pageRequest` | source | +Spine:37 | no (default) | spine.contract `preq`: a native request to open a page | none: correct |
| `settingsFacts` | source | +Settings:10 | no (default) | members-settings.contract `sfacts` | default; not read for this change |
| `retrievalAsk` | source | +Retrieval:12 | no (default) | members-retrieval.contract `rask`: the voice turn's ask | no voice turn: correct |
| `takesRecorder` | source | +Takes:68 | no (default) | members-takes.contract `trec`: the take recorder | Takes page is drawn; recording does nothing. Not changed here: not one of the three named |
| `conversationEntries` | source | +Talk:31 | no (default) | members-talk.contract `cvraw`: answer card entries | empty: correct |
| `answerAction` | send | +Talk:32 | no | members-talk.contract: the answer card's Try again and Cancel | the card never shows on Android now |
| `voiceProfile` | source | +Talk:26 | no (default) | members-talk.contract `voicep` | default |
| `voiceLearn` | send | +Talk:27 | no | no contract calls it | none |
| `voiceForget` | send | +Talk:28 | no | members-talk.contract: forget my voice | drawn on the voiceprint page; does nothing. Not changed here |
| `voiceSetupFacts` | source | +Onboarding:362 | no (default) | members-onboarding.contract `ovoice`; Settings "Voice and models" | page lists the declared default offers. Not changed here |
| `voiceCards` `voicePick` `voiceHear` `voiceStop` `voiceContinue` | source, sends | +Voice:103-109 | no | members-voice.contract: the voice cards and their Hear and Use | drawn; do nothing. Not changed here |
| `nativeModelFacts` `nativeModelEnsure` `nativeModelRemove` `nativeModelProof` | not in the interface | none | answered, :80-83 | none | unreachable from shared code |

This slice changes the controls of the four ops named in "What Android shows now": the speed row, the bar, mail and the Texture list. The rows marked "Not changed here" are the same
defect in the voice pages (voice cards, voiceprint, takes, Voice and models).
They are named residuals: they depend on the owner's choice between "Kotlin
answers them" and "do not offer them", and the second reading, applied to
them, would remove whole pages.

## The mechanism: one table in shared code

Exact has no fact that says "this host does not answer this op"
(`docs/contract-for-agents.md:4453-4459` lets a source list only `"web"` in
`substitute.unsupported`; a `NotSupportedError` from a native module becomes
the declared default with no mark). A native report would be a new interface
name, and the boundary gate's name ceiling only falls. So the fact is data:

- `mobile/AutoradioLab/native-answers.ts` has one row, `android`, equal to the
  `when` cases of `AutoradioModule.kt`. A platform with no row answers every
  op: iOS and macOS (every interface name has a Swift handler, tested), and web.
- The platform is `exactPage().platform` (`contract-for-agents.md:6545`: `ios`,
  `macos`, `android` and others on native hosts; `web` in a browser, even on an
  Android phone; `""` before the first answer, read as "all").
- `spine.contract` reads the table once (`nans`) and each control asks
  `opAnswered(nans.all, nans.ops, "<op>")`. No control tests the platform.
- `android-controls.test.ts` K2 fails when Kotlin gains a case the table lacks.
  Then the commit that adds the case adds the name, and the control appears
  with no contract change.
- The web host is unchanged: it has no row, so it still offers every control.

Before the first page answer the platform is `""`, which answers nothing native:
no guarded control is drawn and a typed question is kept. The controls appear
with the host's first answer, on every platform. A platform with no row,
including one that does not exist yet, answers everything.

## What Android shows now

| Control | Op | Android now |
| --- | --- | --- |
| Speed row, Autoradio page | `radioSpeed` | not drawn (nothing else depended on the row) |
| Microphone button, bar | `choirHandsFreeToggle` | not drawn |
| Cancel while listening | `choirTalkCancel` | not drawn (it was only reachable while listening) |
| Send, bar | `choirAsk`, or an address | drawn only when the field holds a web address; for a question it is not drawn |
| The field's placeholder | `choirAsk` | "Open a web address..." ("Web address..." on a narrow phone) |
| The line above the field while typing | `choirAsk` | "This phone does not answer questions yet. A web address opens in the app." |
| Enter in the field | `choirAsk` | an address opens in place (as on iPhone); a question does nothing: the text stays, nothing is sent, the sign-in sheet does not open for it |
| "Summarise this thread", mail | `choirAsk` | not drawn |
| "Make your edition" card, Texture list | `choirAsk` | not drawn (the card's only action is a prefilled question) |

The typed question, end to end, before this change on Android: Enter ran
`sendAsk` (`spine.contract`), which sent `choirAsk`, got `NotSupportedError`
and then cleared the field (`ask = ""`). The words were dropped and the bar
showed nothing, and the line above the field said "The radio answers aloud",
which was false. After: the text stays, and the line says what is true.

The microphone button used to open "Voice and models" when no hearing model
was on the phone. With the button not drawn, that route is gone from the bar;
Settings still lists it.
