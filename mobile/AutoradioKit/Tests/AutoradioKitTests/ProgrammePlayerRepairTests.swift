import XCTest
@testable import AutoradioKit

// Station P1, review round 1 (REPAIR): the failure cases the reviewer found, written before the repair. Each is numbered R1.. in the
// repair list at the end of the P1 section of mobile/README.md. They fail on the tip 681ae12d9 and pass after the repair.

final class RepairPlayerTests: XCTestCase {
    func makePlayer(voice: String = "stub-voice", doc: ProgrammeDocument) -> (ProgrammePlayer, PlayerWatch) {
        let r = StageRegistry.builtIn()
        r.register(.voice, LongVoice.self) { LongVoice($0) }
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        var cfg = StageRegistry.defaultConfiguration(); cfg.set(.voice, .init(id: voice))
        let log = EventLog(clock: LabClock())
        let p = ProgrammePlayer(audio: virtualAudio(), registry: r, log: log, configuration: cfg, document: doc)
        return (p, PlayerWatch(log, p))
    }
    func prepared(_ n: Int, _ text: String = "More words.") -> PreparedSegment {
        SegmentPrep.prepare(IncomingSegment(stretchID: "st1", segmentID: "st1/\(n)", kind: "story", title: "T\(n)", text: text, cites: [], blockCount: nil))!
    }

    // R2: ran dry, the listener pauses, a segment arrives, the listener plays: the new segment, not the old last block again.
    func testR2_DryPauseAppendPlayStartsAtTheFirstUnplayedBlock() async {
        let (p, w) = makePlayer(doc: segmentedDocument(["One."]))
        p.play()
        let a = await poll(10) { w.ranDry.count == 1 }; XCTAssertTrue(a)
        let b = await poll(5) { !p.loopIsActive }; XCTAssertTrue(b, "the loop ended dry")
        p.pause()
        XCTAssertTrue(p.appendSegment(prepared(1), toDocument: "radio:t"))
        try? await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertEqual(w.air, [0], "paused: nothing plays"); XCTAssertEqual(w.ranDry.count, 1)
        p.play()
        let c = await poll(10) { w.ranDry.count == 2 }; XCTAssertTrue(c)
        XCTAssertEqual(w.air, [0, 1], "the old segment is not replayed or reported twice")
        XCTAssertEqual(p.blockIndex, 1)
    }

    // R2: with nothing appended, play at the end of a programme still plays the last block again, as it always did.
    func testR2_PlayAtTheEndWithNothingNewKeepsTheOldBehaviour() async {
        let p = makePlayer(voice: "long-voice", doc: ProgrammeDocument(id: "choir:a", title: "A", blocks: [.speech(text: "One.")])).0
        p.setRate(2)
        p.play()
        let a = await poll(10) { !p.loopIsActive }; XCTAssertTrue(a, "the article ended")
        p.pause(); p.play()
        let b = await poll(3) { p.loopIsActive }; XCTAssertTrue(b, "it plays the last block again")
        XCTAssertEqual(p.blockIndex, 0)
        p.pause()
    }

    // R3: a segment appended between the loop's last read and the write that ends it is played (a hook at the tail, no sleep).
    func testR3_AnAppendInTheTailWindowIsPlayed() async {
        let (p, w) = makePlayer(doc: segmentedDocument(["One."]))
        let seg = prepared(1)
        let once = Locked(true)
        p.beforeLoopTail = { [unowned p] in
            if once.write({ o -> Bool in let was = o; o = false; return was }) { _ = p.appendSegment(seg, toDocument: "radio:t") }
        }
        p.play()
        let ok = await poll(10) { !w.ranDry.isEmpty }
        XCTAssertTrue(ok); XCTAssertEqual(w.air, [0, 1], "the segment that arrived in the window was played: \(w.air)")
        XCTAssertEqual(w.ranDry.count, 1, "the loop carried on and ended dry once, after it")
    }
}

final class RepairStretchTests: XCTestCase {
    func segs(_ e: [StretchEvent]) -> [IncomingSegment] { e.compactMap { if case .segment(let s) = $0 { return s } else { return nil } } }

    // S3: a bare `event: done` and `event: error` with no data line.
    func testBareEventLinesAreDispatched() {
        var d = StretchDecoder()
        XCTAssertEqual(d.feed("event: done\n"), [], "not yet: a data line may follow")
        XCTAssertEqual(d.feed("\n"), [.done], "a blank line ends it")
        var e = StretchDecoder()
        XCTAssertEqual(e.feed("event: error\nevent: done\n"), [.error("the stretch failed")], "the next event line ends the error")
        XCTAssertEqual(e.finish(), [.done], "the end of the stream ends the last")
        var f = StretchDecoder()
        XCTAssertEqual(f.feed("event: error\ndata: {\"reason\":\"x\"}\n"), [.error("x")], "with data: as before")
        XCTAssertEqual(f.finish(), [])
    }

    // S4: zero-width and bidi format characters never reach a title.
    func testFormatCharactersAreStrippedFromTitles() {
        let t = SegmentPrep.title("Ha\u{200B}rbour \u{202E}news\u{2066}", kind: "story")
        XCTAssertEqual(t, "Harbour news")
        XCTAssertEqual(SegmentPrep.title("\u{200B}\u{202E}", kind: "story"), "Story")
        XCTAssertEqual(SegmentPrep.title("Caf\u{00E9} \u{FEFF}open", kind: "story"), "Café open")
    }

    // S5: SplitBlocks matches Go's (the outputs in blocks_unicode.json are what calibration.SplitBlocks returned).
    func testSplitBlocksMatchesGoOnUnicodeWhitespace() throws {
        let f = try LearningFixtures.json("blocks_unicode.json")
        let cases = try XCTUnwrap(f["cases"] as? [[String: Any]])
        XCTAssertEqual(cases.count, 12)
        for c in cases {
            let name = c["name"] as? String ?? "?"
            XCTAssertEqual(SplitBlocks.split(c["text"] as? String ?? ""), c["blocks"] as? [String], name)
        }
    }

    // nit: block_count is an integer, not a bool and not a fraction.
    func testBlockCountIsOnlyAnInteger() {
        func count(_ raw: String) -> Int? {
            var d = StretchDecoder()
            return segs(d.feed("{\"kind\":\"story\",\"title\":\"T\",\"text\":\"Words.\",\"block_count\":\(raw)}\n")).first?.blockCount
        }
        XCTAssertEqual(count("3"), 3); XCTAssertEqual(count("3.0"), 3)
        XCTAssertNil(count("true")); XCTAssertNil(count("false")); XCTAssertNil(count("2.5")); XCTAssertNil(count("\"3\""))
    }
}
