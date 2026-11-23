import XCTest
@testable import AutoradioKit

// Listen (owner review 2026-11-15): a Listen on a text while a story plays queued behind it (speakText's rule for mail). A person
// pressing Listen on what is in front of them hears it now: the player gets startNow, and the document is the text, starting at the
// headline. (Composing the headline and the body is shared code now; the Swift ListenText is gone.)
final class ListenTextTests: XCTestCase {

    // L4a: the player takes the text now, from its headline, even with a story playing.
    func testL4a_ListenReplacesAPlayingStoryAtTheHeadline() {
        let r = StageRegistry.builtIn()
        var cfg = StageRegistry.defaultConfiguration(); cfg.set(.voice, .init(id: "stub-voice"))
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        let doc = ProgrammeDocument(id: "choir:a", title: "A", blocks: (0..<4).map { .speech(text: "Block \($0).") })
        let p = ProgrammePlayer(audio: virtualAudio(), registry: r, log: EventLog(clock: LabClock()), configuration: cfg, document: doc)
        let text = "Chips rally\n\nFirst.\n\nSecond."
        XCTAssertTrue(p.speakText(address: "pub:chips", text: text, title: "Chips rally", startNow: true, play: false))
        XCTAssertEqual(p.document.id, "text:pub:chips")
        XCTAssertEqual(p.document.title, "Chips rally")
        XCTAssertEqual(p.blockIndex, 0)
        XCTAssertEqual(p.document.blocks.compactMap(\.spokenText), ["Chips rally", "First.", "Second."])
    }
}
