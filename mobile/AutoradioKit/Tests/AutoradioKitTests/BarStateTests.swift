import XCTest
@testable import AutoradioKit

// Written from the failure list at the top of BarState.swift.
final class BarStateTests: XCTestCase {
    private func snap(_ phase: String = "playing", running: Bool = true, paused: Bool = false, level: String = "play", idx: Int = 0, count: Int = 3, text: String = "") -> LabSnapshot {
        var s = LabSnapshot(); s.running = running; s.paused = paused; s.phase = phase; s.level = level
        s.blockIndex = idx; s.blockCount = count; s.transcript = text; return s
    }
    private func player(playing: Bool, title: String = "T", message: String = "") -> ChoirPlayerView {
        var p = ChoirPlayerView(); p.signedIn = true; p.playing = playing; p.title = title; p.message = message; return p
    }
    private let t0 = Date(timeIntervalSince1970: 1000)

    // (The bar's states, words and windows are bar.ts: bar.test.ts. What is native is what the engine told it, kept raw.)
    func testEventsAreKeptRawWithTheirTime() {
        let tr = BarTracker()
        tr.observe(.holdingSaid("Looking at the third paragraph…"), now: t0)
        XCTAssertEqual(tr.raw().holding, "Looking at the third paragraph…")
        tr.observe(.phase(from: "playing", to: "candidate"), now: t0)
        XCTAssertEqual(tr.raw().holding, "", "a new turn clears the old line")
        tr.observe(.revisionArrived(id: "r", fromBlock: 1, blocks: 2, summary: "s"), now: t0)
        XCTAssertEqual(tr.raw().revisedAt, 1000)
        tr.observe(.error(stage: "desk", message: "boom"), now: t0.addingTimeInterval(10))
        XCTAssertEqual(tr.raw().errorStage, "desk"); XCTAssertEqual(tr.raw().errorMessage, "boom"); XCTAssertEqual(tr.raw().errorAt, 1010)
        tr.observe(.error(stage: "desk", message: "boom"), now: t0.addingTimeInterval(13))
        XCTAssertEqual(tr.raw().errorAt, 1013, "a repeat moves the time, so the window restarts")
        tr.observe(.error(stage: "other", message: "x"), now: t0.addingTimeInterval(20))
        XCTAssertEqual(tr.raw().errorAt, 1013, "only the engine's own stages are errors for the bar")
        XCTAssertEqual(tr.expiry(now: t0.addingTimeInterval(5))!, 1.0, accuracy: 0.01)
        XCTAssertNil(tr.expiry(now: t0.addingTimeInterval(30)))
        tr.wipe()
        XCTAssertEqual(tr.raw().errorAt, 0); XCTAssertEqual(tr.raw().holding, ""); XCTAssertEqual(tr.raw().revisedAt, 0)
    }

    // Failure list for what is left of the radio view (the sentences, progress and follow-along are radio-follow.test.ts):
    //  V1 the block on air is clamped into the programme: a block past the end (after a revision shortens it) is the last, a negative one the first, no blocks is nothing; a clip speaks its summary, a speech block its words.
    //  V2 the title is the document's, or the segment's when the block is in one; nothing is "N of M".
    //  V3 the block's place is the audio's own, for THIS block only: another block's position, or no position, is 0, and a place at the very end stays below 1.
    func testBlockOnAirClampsIntoTheProgramme() {
        let tr = BarTracker()
        let blocks = (0..<100).map { ProgrammeBlock.speech(text: "Block \($0). More words.") }
        let d = ProgrammeDocument(id: "d", title: "Doc", blocks: blocks)
        let last = tr.blockOnAir(document: d, index: 99)
        XCTAssertEqual(last.index, 99); XCTAssertEqual(last.count, 100); XCTAssertEqual(last.spoken, "Block 99. More words."); XCTAssertEqual(last.document, "d")
        XCTAssertEqual(tr.blockOnAir(document: d, index: 500).index, 99)
        XCTAssertEqual(tr.blockOnAir(document: d, index: -4).index, 0)
        let none = tr.blockOnAir(document: ProgrammeDocument(id: "n", title: "", blocks: []), index: 3)
        XCTAssertEqual(none.count, 0); XCTAssertEqual(none.spoken, ""); XCTAssertEqual(none.index, 0); XCTAssertEqual(none.fraction, 0)
    }

    func testAClipSpeaksItsSummary() {
        let clip = ProgrammeBlock.clip(url: "https://x.test/a.mp3", start: 0, end: nil, source: "Show")
        let d = ProgrammeDocument(id: "pub:x", title: "Doc", blocks: [.speech(text: "Words here."), clip, .speech(text: "Other words.")])
        let tr = BarTracker()
        XCTAssertEqual(tr.blockOnAir(document: d, index: 1).spoken, clip.summary)
        XCTAssertEqual(tr.blockOnAir(document: d, index: 0).spoken, "Words here.")
    }

    // V3/V4: the answer is one moment. The place is the audio's for the block whose words are returned, read in the same call.
    func testTheFractionIsForTheBlockWhoseWordsAreReturned() {
        let tr = BarTracker()
        let d = ProgrammeDocument(id: "d", title: "T", blocks: (0..<3).map { .speech(text: "Block \($0).") })
        tr.positionSource = { RenderPosition(index: 2, fraction: 0.4, seconds: 1, total: 2) }
        let clamped = tr.blockOnAir(document: d, index: 9)               // a revision shortened the programme under the index
        XCTAssertEqual(clamped.index, 2); XCTAssertEqual(clamped.fraction, 0.4, accuracy: 1e-9, "the clamped block is the one the audio plays")
        XCTAssertEqual(tr.blockOnAir(document: d, index: 1).fraction, 0, "the audio plays block 2: block 1 never gets its fraction")
        tr.positionSource = { RenderPosition(index: 2, fraction: 1, seconds: 1, total: 2) }
        XCTAssertEqual(tr.blockOnAir(document: d, index: 2).fraction, 0.99, accuracy: 1e-9)
        tr.positionSource = nil
        XCTAssertEqual(tr.blockOnAir(document: d, index: 2).fraction, 0)
    }

    func testTheDocumentSwappedBetweenReadsNeverGivesAnotherBlocksPlace() {
        // The audio moves to the second document's block while the answer is being made (the position is read inside the call): the words are the document the caller passed, the place is 0.
        let tr = BarTracker()
        let first = ProgrammeDocument(id: "pub:first", title: "A", blocks: [.speech(text: "First one."), .speech(text: "First two.")])
        let second = ProgrammeDocument(id: "pub:second", title: "B", blocks: [.speech(text: "Second one."), .speech(text: "Second two.")])
        let current = Locked(first)
        tr.positionSource = { current.write { $0 = second }; return RenderPosition(index: 0, fraction: 0.7, seconds: 1, total: 2) }
        let a = tr.blockOnAir(document: first, index: 1)
        XCTAssertEqual(a.document, "pub:first"); XCTAssertEqual(a.spoken, "First two."); XCTAssertEqual(a.index, 1)
        XCTAssertEqual(a.fraction, 0, "the position is for block 0, the words are block 1's")
        let b = tr.blockOnAir(document: current.read { $0 }, index: 0)
        XCTAssertEqual(b.document, "pub:second"); XCTAssertEqual(b.spoken, "Second one."); XCTAssertEqual(b.fraction, 0.7, accuracy: 1e-9)
    }

    func testRadioInfoCarriesTheTitleAndFlags() {
        let tr = BarTracker()
        let d = ProgrammeDocument(id: "d", title: "Doc", blocks: [.speech(text: "One."), .speech(text: "Two.")])
        let r = tr.radioInfo(player: player(playing: true), snapshot: snap(level: "dip", idx: 1, count: 2), document: d)
        XCTAssertEqual(r.title, "Doc"); XCTAssertTrue(r.playing); XCTAssertEqual(r.level, "dip"); XCTAssertEqual(r.speedValue, 1)
        let empty = tr.radioInfo(player: player(playing: false, title: ""), snapshot: snap(count: 0), document: ProgrammeDocument(id: "n", title: "", blocks: []))
        XCTAssertEqual(empty.title, ""); XCTAssertFalse(empty.playing)
    }
}
