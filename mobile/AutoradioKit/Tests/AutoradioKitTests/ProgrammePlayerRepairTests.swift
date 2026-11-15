import XCTest
@testable import AutoradioKit

// Station P1, review round 1 (REPAIR): the failure cases the reviewer found, written before the repair. Each is numbered R1.. in the
// repair list at the end of the P1 section of mobile/README.md. They fail on the tip 681ae12d9 and pass after the repair.

final class RepairTrackerTests: XCTestCase {
    final class Sink: @unchecked Sendable { var events: [PlaybackEvent] = []; var kinds: [PlaybackKind] { events.map(\.kind) } }
    func make() -> (SegmentTracker, Sink) { let t = SegmentTracker(); let s = Sink(); t.observe { s.events.append($0) }; return (t, s) }
    func threeBlocks(_ more: [String] = []) -> ProgrammeDocument { segmentedDocument(["A one.\n\nB two.\n\nC three."] + more) }

    // R1: a multi-block segment whose first block got no voice: failed once, and nothing after it.
    func testR1_FailedFirstBlockOfThreeIsNotReopenedByTheNextBlocks() {
        let d = threeBlocks()
        XCTAssertEqual(d.segments[0].blockCount, 3)
        let (t, s) = make()
        t.unitBegan(0, in: d); t.unitEnded(0, played: false, in: d)
        t.unitBegan(1, in: d); t.unitEnded(1, played: true, in: d)
        t.unitBegan(2, in: d); t.unitEnded(2, played: true, in: d)
        XCTAssertEqual(s.kinds, [.segmentBegan, .blockBegan, .failed], "failed, never began again, never finished")
    }

    // R1: a finished segment whose last block is begun again (a replay) reports nothing, until another segment has begun.
    func testR1_AFinishedSegmentStaysClosedUntilAnotherSegmentBegins() {
        let d = threeBlocks(["Next."])
        let (t, s) = make()
        for u in 0..<3 { t.unitBegan(u, in: d); t.unitEnded(u, played: true, in: d) }
        XCTAssertEqual(s.kinds.last, .finished)
        // Round 2 (S1): begun again at or before the unit it closed on, the last block is a seek, which is a play: it is reported again.
        let n = s.events.count
        t.unitBegan(2, in: d); t.unitEnded(2, played: true, in: d)
        XCTAssertEqual(Array(s.kinds[n...]), [.segmentBegan, .blockBegan, .finished])
        t.unitBegan(3, in: d)                                              // another segment begins
        XCTAssertEqual(s.kinds.suffix(2), [.segmentBegan, .blockBegan])
        let m = s.events.count
        t.unitBegan(0, in: d)                                              // and a return to the first is a real replay
        XCTAssertEqual(s.kinds[m], .rewound); XCTAssertEqual(s.kinds[m + 1], .segmentBegan)
    }

    func testR1_AFailedSegmentReopensAfterTheDocumentIsReplaced() {
        let d = threeBlocks()
        let (t, s) = make()
        t.unitBegan(0, in: d); t.unitEnded(0, played: false, in: d)
        t.documentReplaced()
        t.unitBegan(1, in: d)
        XCTAssertEqual(s.kinds, [.segmentBegan, .blockBegan, .failed, .segmentBegan, .blockBegan])
    }
}

final class RepairPlayerTests: XCTestCase {
    final class Sink: @unchecked Sendable {
        private let lock = NSLock(); private var e: [PlaybackEvent] = []
        func add(_ x: PlaybackEvent) { lock.lock(); e.append(x); lock.unlock() }
        var kinds: [String] { lock.lock(); defer { lock.unlock() }; return e.map { "\($0.kind.rawValue):\($0.segmentID.split(separator: "/").last ?? "")" } }
    }
    func makePlayer(voice: String = "stub-voice", doc: ProgrammeDocument) -> (ProgrammePlayer, Sink) {
        let r = StageRegistry.builtIn()
        r.register(.voice, LongVoice.self) { LongVoice($0) }
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        var cfg = StageRegistry.defaultConfiguration(); cfg.set(.voice, .init(id: voice))
        let p = ProgrammePlayer(audio: virtualAudio(), registry: r, log: EventLog(clock: LabClock()), configuration: cfg, document: doc)
        let s = Sink(); p.playback.observe { s.add($0) }
        return (p, s)
    }
    func prepared(_ n: Int, _ text: String = "More words.") -> PreparedSegment {
        SegmentPrep.prepare(IncomingSegment(stretchID: "st1", segmentID: "st1/\(n)", kind: "story", title: "T\(n)", text: text, cites: [], blockCount: nil))!
    }

    // R2: ran dry, the listener pauses, a segment arrives, the listener plays: the new segment, not the old last block again.
    func testR2_DryPauseAppendPlayStartsAtTheFirstUnplayedBlock() async {
        let (p, s) = makePlayer(doc: segmentedDocument(["One."]))
        p.play()
        let a = await poll(10) { s.kinds.contains("finished:0") }; XCTAssertTrue(a)
        let b = await poll(5) { !p.loopIsActive }; XCTAssertTrue(b, "the loop ended dry")
        p.pause()
        XCTAssertTrue(p.appendSegment(prepared(1), toDocument: "radio:t"))
        try? await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertEqual(s.kinds, ["segmentBegan:0", "blockBegan:0", "finished:0"], "paused: nothing plays")
        p.play()
        let c = await poll(10) { s.kinds.contains("finished:1") }; XCTAssertTrue(c)
        XCTAssertEqual(s.kinds, ["segmentBegan:0", "blockBegan:0", "finished:0", "segmentBegan:1", "blockBegan:1", "finished:1"],
                       "the old segment is not replayed or reported twice")
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
        let (p, s) = makePlayer(doc: segmentedDocument(["One."]))
        let seg = prepared(1)
        let once = Locked(true)
        p.beforeLoopTail = { [unowned p] in
            if once.write({ o -> Bool in let was = o; o = false; return was }) { _ = p.appendSegment(seg, toDocument: "radio:t") }
        }
        p.play()
        let ok = await poll(10) { s.kinds.contains("finished:1") }
        XCTAssertTrue(ok, "the segment that arrived in the window was played: \(s.kinds)")
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

final class RepairStageTests: XCTestCase {
    var env: ChoirEnvironment!
    override func setUp() {
        let cfg = URLSessionConfiguration.ephemeral
        cfg.protocolClasses = [StubProtocol.self]
        env = ChoirEnvironment.shared
        env.keys = InMemoryKeyStore([KeyAccount.apiKey: secretKey, KeyAccount.computerID: "comp-1"])
        env.log = EventLog(clock: LabClock()); env.session = URLSession(configuration: cfg)
        StubProtocol.seen = []
    }
    override func tearDown() { env.session = .shared }

    final class Manual: StretchSource, @unchecked Sendable {
        var cont: AsyncThrowingStream<StretchEvent, Error>.Continuation!
        func events(cause: String) async throws -> AsyncThrowingStream<StretchEvent, Error> { AsyncThrowingStream { self.cont = $0 } }
    }
    let article = { @Sendable () async throws -> ProgrammeDocument in ProgrammeDocument(id: "choir:a", title: "A", blocks: [.speech(text: "Article.")]) }

    // The first-segment deadline: the design names none (it asks for sound within two seconds, 2.1, which is the cold-start ladder, not
    // this); 8 seconds is the default, and the stage takes it as a parameter so the test does not wait for it.
    func testFirstSegmentDeadlineFallsBackToTheArticle() async throws {
        XCTAssertEqual(RadioProgramme.defaultFirstSegmentDeadline, 8)
        let src = Manual()
        let stage = RadioProgramme(source: src, fallback: article, firstSegmentDeadline: 0.3)
        let t0 = Date()
        let o = try await stage.open()
        XCTAssertTrue(o.usedFallback); XCTAssertEqual(o.document.id, "choir:a"); XCTAssertNil(o.follow)
        XCTAssertLessThan(Date().timeIntervalSince(t0), 3)
    }

    func testASegmentBeforeTheDeadlineIsNotAFallback() async throws {
        let src = Manual()
        let stage = RadioProgramme(source: src, fallback: article, firstSegmentDeadline: 5)
        let opening = Task { try await stage.open() }
        let ready = await poll { src.cont != nil }; XCTAssertTrue(ready)
        src.cont.yield(.segment(IncomingSegment(stretchID: "m", segmentID: "m/0", kind: "story", title: "S", text: "Words.", cites: [], blockCount: nil)))
        let o = try await opening.value
        XCTAssertFalse(o.usedFallback)
        src.cont.finish()
    }

    // S2: a cancelled open is cancelled, not a fallback fetch.
    func testACancelledOpenDoesNotFetchTheArticle() async {
        let src = Manual()
        let fetched = Locked(false)
        let stage = RadioProgramme(source: src, fallback: { fetched.write { $0 = true }; return ProgrammeDocument(id: "choir:a", title: "A", blocks: []) }, firstSegmentDeadline: 30)
        let opening = Task { try await stage.open() }
        let ready = await poll { src.cont != nil }; XCTAssertTrue(ready)
        opening.cancel()
        let r = await opening.result
        if case .success = r { XCTFail("a cancelled open returned a programme") }
        XCTAssertFalse(fetched.read { $0 }, "the article was fetched for a cancelled open")
    }

    // S1: a stale open never lands over a newer load.
    struct NoRoute: StretchSource { func events(cause: String) async throws -> AsyncThrowingStream<StretchEvent, Error> { throw ChoirError.empty("no route") } }
    func testAStaleLoadDoesNotOverwriteTheNewerOne() async {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("p1-\(UUID().uuidString)")
        let rt = LabRuntime(supportDir: dir, session: NullSessionOwner())
        rt.registry.register(.voice, StubVoice.self) { StubVoice($0) }
        rt.setStage(.voice, id: "stub-voice")
        let slow = Manual()
        let made = Locked(0)
        let newer = { @Sendable () async throws -> ProgrammeDocument in ProgrammeDocument(id: "choir:newer", title: "N", blocks: [.speech(text: "Newer.")]) }
        rt.registry.register(.programme, RadioProgramme.self) { _ in
            made.write { $0 += 1; return $0 } == 1 ? RadioProgramme(source: slow, fallback: newer, firstSegmentDeadline: 60)
                                                   : RadioProgramme(source: NoRoute(), fallback: newer, firstSegmentDeadline: 60)
        }
        let app = ChoirApp(runtime: rt, keys: InMemoryKeyStore([KeyAccount.apiKey: secretKey, KeyAccount.computerID: "comp-1"]), openURL: nil)
        let first = Task { await app.loadProgramme() }
        let waiting = await poll { slow.cont != nil }; XCTAssertTrue(waiting, "the first load is waiting for its first segment")
        await app.loadProgramme()                                     // the second load: no route, the article
        XCTAssertEqual(rt.player.document.id, "choir:newer")
        slow.cont.yield(.segment(IncomingSegment(stretchID: "old", segmentID: "old/0", kind: "story", title: "Old", text: "Old words.", cites: [], blockCount: nil)))
        await first.value
        XCTAssertEqual(rt.player.document.id, "choir:newer", "the stale open did not replace the newer programme")
        XCTAssertEqual(rt.player.document.segments, [])
        slow.cont.finish()
        rt.player.pause()
    }

    // nit: the player view counts segments in a programme, as the bar does.
    func testPlayerViewPositionCountsSegments() {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("p1-\(UUID().uuidString)")
        let rt = LabRuntime(supportDir: dir, session: NullSessionOwner())
        let app = ChoirApp(runtime: rt, keys: InMemoryKeyStore([:]), openURL: nil)
        rt.player.setDocument(segmentedDocument(["One.\n\nTwo.", "Three."]))
        XCTAssertEqual(app.playerView().position, "1 of 2")
        rt.player.setDocument(ProgrammeDocument(id: "choir:x", title: "A", blocks: [.speech(text: "One."), .speech(text: "Two.")]))
        XCTAssertEqual(app.playerView().position, "1 of 2")
    }
}
