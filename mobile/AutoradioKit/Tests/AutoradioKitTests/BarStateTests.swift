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
    //  V4 the block's key changes with the document, the block or its words; the fraction answers for the clamped block.
    //  V3 the block's place is the audio's own, for THIS block only: another block's position, or no position, is 0, and a place at the very end stays below 1.
    func testBlockOnAirClampsIntoTheProgramme() {
        let blocks = (0..<100).map { ProgrammeBlock.speech(text: "Block \($0). More words.") }
        let d = ProgrammeDocument(id: "d", title: "Doc", blocks: blocks)
        let last = BarTracker.blockOnAir(snap(idx: 99, count: 100), document: d)
        XCTAssertEqual(last.index, 99); XCTAssertEqual(last.count, 100); XCTAssertEqual(last.spoken, "Block 99. More words.")
        XCTAssertEqual(BarTracker.blockOnAir(snap(idx: 500, count: 100), document: d).index, 99)
        XCTAssertEqual(BarTracker.blockOnAir(snap(idx: -4, count: 100), document: d).index, 0)
        let none = BarTracker.blockOnAir(snap(count: 0), document: ProgrammeDocument(id: "n", title: "", blocks: []))
        XCTAssertEqual(none.count, 0); XCTAssertEqual(none.spoken, ""); XCTAssertEqual(none.index, 0)
    }

    func testAClipSpeaksItsSummaryAndTheKeyFollowsTheBlockAndItsWords() {
        let clip = ProgrammeBlock.clip(url: "https://x.test/a.mp3", start: 0, end: nil, source: "Show")
        let d = ProgrammeDocument(id: "pub:x", title: "Doc", blocks: [.speech(text: "Words here."), clip, .speech(text: "Other words.")])
        let c = BarTracker.blockOnAir(snap(idx: 1, count: 3), document: d)
        XCTAssertEqual(c.spoken, clip.summary); XCTAssertEqual(c.document, "pub:x")
        let a = BarTracker.blockOnAir(snap(idx: 0, count: 3), document: d), b = BarTracker.blockOnAir(snap(idx: 2, count: 3), document: d)
        XCTAssertEqual(Set([a.key, c.key, b.key]).count, 3, "another block is another key")
        XCTAssertEqual(a.key, BarTracker.blockOnAir(snap(idx: 0, count: 3), document: d).key, "the same block, the same key")
        let edited = ProgrammeDocument(id: "pub:x", title: "Doc", blocks: [.speech(text: "Words there."), clip, .speech(text: "Other words.")])
        XCTAssertNotEqual(a.key, BarTracker.blockOnAir(snap(idx: 0, count: 3), document: edited).key, "other words in the same block, another key")
        XCTAssertEqual(BarTracker.blockOnAir(snap(count: 0), document: ProgrammeDocument(id: "n", title: "", blocks: [])).key, "")
    }

    func testTheFractionAnswersForTheBlockThatWasClamped() {
        let tr = BarTracker()
        let d = ProgrammeDocument(id: "d", title: "T", blocks: (0..<3).map { .speech(text: "Block \($0).") })
        let s = snap(idx: 9, count: 3)                                   // a revision shortened the programme under the index
        tr.positionSource = { RenderPosition(index: 2, fraction: 0.4, seconds: 1, total: 2) }
        let b = BarTracker.blockOnAir(s, document: d)
        XCTAssertEqual(b.index, 2)
        XCTAssertEqual(tr.blockFraction(s), 0, "the raw index names no block the audio plays")
        XCTAssertEqual(tr.blockFraction(s, index: b.index), 0.4, accuracy: 1e-9, "the clamped block is the one the facts describe, so its place is the audio's")
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
