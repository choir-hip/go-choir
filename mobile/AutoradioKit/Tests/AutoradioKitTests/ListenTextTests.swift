import XCTest
@testable import AutoradioKit

// Listen (owner review 2026-11-15, items 4 and 10): the failure list, written before the code.
//
// L1  Nothing outside paper Play and mail's "Read it to me" reaches speakText: an open article, a Texture, an EPUB chapter and a
//     text file have no Listen control. (The contract and module wiring is proven by the Exact build; the text is tested here.)
// L2  An article read aloud would start with its body and never say its headline. ListenText.compose puts the headline first,
//     as its own paragraph.
//       L2a headline then body, in that order, paragraph by paragraph
//       L2b a body that already opens with the headline does not say it twice
//       L2c a headline with no body is not a listening: nothing to say (no engine start, no empty programme)
//       L2d no headline: the body alone
// L3  Markdown spoken as written: an EPUB chapter and a .md file would read "hash hash", "asterisk" and link addresses.
//       L3a headings, emphasis, bullets, quotes, code marks and fences lose their marks; link text stays, the address goes
//       L3b an image is dropped (its alt text is not prose)
// L4  A Listen on a text while a story plays queued behind it (speakText's rule for mail). A person pressing Listen on what is
//     in front of them hears it now: the player gets startNow, and the document is the text, starting at the headline.
//       L4a with a story playing, startNow replaces it and block 0 is the headline
// L5  Control characters and runs of blank lines in captured text must not make empty paragraphs or a title with a newline.
final class ListenTextTests: XCTestCase {
    func testL2a_HeadlineThenBody() {
        let t = ListenText.compose(title: "Chips rally", body: "First para.\n\nSecond para.")
        XCTAssertEqual(ChoirText.paragraphs(t), ["Chips rally", "First para.", "Second para."])
    }
    func testL2b_NoRepeatedHeadline() {
        let t = ListenText.compose(title: "Chips rally", body: "Chips rally\n\nFirst para.")
        XCTAssertEqual(ChoirText.paragraphs(t), ["Chips rally", "First para."])
    }
    func testL2c_NoBodyIsNothingToSay() {
        XCTAssertEqual(ListenText.compose(title: "Chips rally", body: "  \n\n "), "")
        XCTAssertEqual(ListenText.compose(title: "", body: ""), "")
    }
    func testL2d_NoHeadlineStillReadsTheBody() {
        XCTAssertEqual(ChoirText.paragraphs(ListenText.compose(title: " ", body: "Only body.")), ["Only body."])
    }
    func testL3a_MarkdownLosesItsMarks() {
        let md = "# Chapter One\n\nIt was a *very* **dark** night, see [the map](https://x.example/map).\n\n- first\n- second\n\n> quoted\n\n```\ncode\n```\n\nUse `ls`."
        let t = ListenText.compose(title: "", body: md)
        XCTAssertEqual(ChoirText.paragraphs(t), ["Chapter One", "It was a very dark night, see the map.", "first second", "quoted", "code", "Use ls."])
        XCTAssertFalse(t.contains("http")); XCTAssertFalse(t.contains("*")); XCTAssertFalse(t.contains("#"))
    }
    func testL3b_ImagesAreDropped() {
        XCTAssertEqual(ListenText.compose(title: "", body: "Before ![a chart](img.png) after."), "Before after.")
    }
    func testL5_ControlCharactersAndBlankRuns() {
        let t = ListenText.compose(title: "Head\nline\u{0007}", body: "A.\n\n\n\n\u{0000}\n\nB.")
        XCTAssertEqual(ChoirText.paragraphs(t), ["Head line", "A.", "B."])
    }

    // L4a: the player takes the text now, from its headline, even with a story playing.
    func testL4a_ListenReplacesAPlayingStoryAtTheHeadline() {
        let r = StageRegistry.builtIn()
        var cfg = StageRegistry.defaultConfiguration(); cfg.set(.voice, .init(id: "stub-voice"))
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        let doc = ProgrammeDocument(id: "choir:a", title: "A", blocks: (0..<4).map { .speech(text: "Block \($0).") })
        let p = ProgrammePlayer(audio: virtualAudio(), registry: r, log: EventLog(clock: LabClock()), configuration: cfg, document: doc)
        let text = ListenText.compose(title: "Chips rally", body: "First.\n\nSecond.")
        XCTAssertTrue(p.speakText(address: "pub:chips", text: text, title: "Chips rally", startNow: true, play: false))
        XCTAssertEqual(p.document.id, "text:pub:chips")
        XCTAssertEqual(p.document.title, "Chips rally")
        XCTAssertEqual(p.blockIndex, 0)
        XCTAssertEqual(p.document.blocks.compactMap(\.spokenText), ["Chips rally", "First.", "Second."])
    }
}
