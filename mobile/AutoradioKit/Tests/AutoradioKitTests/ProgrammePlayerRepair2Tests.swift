import XCTest
@testable import AutoradioKit

// Station P1, review round 2 (REPAIR): written before the repair. B1 resumeAt outlives its document, S1 the closed marker swallows a real
// replay, S2 sign-out leaves `loading` set, S3 format-character stripping is too broad, and the late programme (the first segment after
// the deadline is handed over at a block boundary instead of lost). They fail on d2bf58f61.

final class Repair2TrackerTests: XCTestCase {
    final class Sink: @unchecked Sendable { var events: [PlaybackEvent] = []; var kinds: [PlaybackKind] { events.map(\.kind) } }
    func make() -> (SegmentTracker, Sink) { let t = SegmentTracker(); let s = Sink(); t.observe { s.events.append($0) }; return (t, s) }
    func twoSegments() -> ProgrammeDocument { segmentedDocument(["A one.\n\nB two.\n\nC three.", "Next."]) }       // units 0,1,2 | 3

    // S1: the last segment finished and the programme is dry; previous, a rewind within it, a jump onto it are real plays.
    func testS1_ReplayOfAClosedLastSegmentIsReported() {
        let d = segmentedDocument(["A one.\n\nB two.\n\nC three."])
        for seek in [0, 1, 2] {                                            // previous (the start), a rewind inside, the last block again
            let (t, s) = make()
            for u in 0..<3 { t.unitBegan(u, in: d); t.unitEnded(u, played: true, in: d) }
            XCTAssertEqual(s.kinds.last, .finished)
            let n = s.events.count
            t.unitBegan(seek, in: d)
            XCTAssertEqual(Array(s.kinds[n...]), [.segmentBegan, .blockBegan], "a seek to unit \(seek) is a play: it opens the segment again")
            t.unitEnded(2, played: true, in: d)                            // and it can finish again
            if seek == 2 { XCTAssertEqual(s.kinds.last, .finished) }
        }
    }

    // S1: jump onto the closed last segment from a segment before it.
    func testS1_JumpOntoAClosedSegmentFromAnotherOne() {
        let d = twoSegments()
        let (t, s) = make()
        for u in 0..<4 { t.unitBegan(u, in: d); t.unitEnded(u, played: true, in: d) }     // both finished, dry
        let n = s.events.count
        t.unitBegan(3, in: d)                                              // jump onto the last segment again
        XCTAssertEqual(Array(s.kinds[n...]), [.segmentBegan, .blockBegan])
        let m = s.events.count
        t.unitBegan(0, in: d)                                              // and back to the first: the open one is rewound
        XCTAssertEqual(s.kinds[m], .rewound)
    }

    // Round 1 still holds: after a failed block the later blocks of that segment (after it) report nothing.
    func testS1_FailedStillSwallowsLaterBlocksButNotASeekBack() {
        let d = segmentedDocument(["A one.\n\nB two.\n\nC three."])
        let (t, s) = make()
        t.unitBegan(0, in: d); t.unitEnded(0, played: false, in: d)
        t.unitBegan(1, in: d); t.unitEnded(1, played: true, in: d); t.unitBegan(2, in: d); t.unitEnded(2, played: true, in: d)
        XCTAssertEqual(s.kinds, [.segmentBegan, .blockBegan, .failed])
        t.unitBegan(0, in: d)                                              // a seek to the failed block's start or before: a new play
        XCTAssertEqual(s.kinds.suffix(2), [.segmentBegan, .blockBegan])
    }
}

final class Repair2PlayerTests: XCTestCase {
    func makePlayer(voice: String = "long-voice", doc: ProgrammeDocument) -> ProgrammePlayer {
        let r = StageRegistry.builtIn()
        r.register(.voice, LongVoice.self) { LongVoice($0) }
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        var cfg = StageRegistry.defaultConfiguration(); cfg.set(.voice, .init(id: voice))
        return ProgrammePlayer(audio: virtualAudio(), registry: r, log: EventLog(clock: LabClock()), configuration: cfg, document: doc)
    }
    let article = ProgrammeDocument(id: "choir:a", title: "A", blocks: [.speech(text: "One."), .speech(text: "Two."), .speech(text: "Three.")])
    let eight = (1...8).map { "Paragraph \($0) of the message." }.joined(separator: "\n\n")

    func runDry(_ p: ProgrammePlayer) async {
        p.play()
        let ended = await poll(10) { !p.loopIsActive }
        XCTAssertTrue(ended, "the article ran dry")
    }

    // B1: resumeAt (3) must not outlive its document.
    func testB1_ATextReadAfterTheProgrammeRanDryStartsAtItsFirstParagraph() async {
        let p = makePlayer(voice: "stub-voice", doc: article)
        await runDry(p)
        XCTAssertTrue(p.speakText(address: "m", text: eight))
        XCTAssertEqual(p.document.id, "text:m"); XCTAssertEqual(p.blockIndex, 0, "paragraphs 0 to 2 are not skipped")
        p.pause()
    }
    func testB1_PlayFalseThenPlayLaterStartsAtTheFirstParagraph() async {
        let p = makePlayer(voice: "stub-voice", doc: article)
        await runDry(p)
        p.pause()
        XCTAssertTrue(p.speakText(address: "m", text: eight, startNow: true, play: false))
        p.play()
        XCTAssertEqual(p.blockIndex, 0)
        p.pause()
    }
    func testB1_ANewDocumentSetAfterDryStartsAtItsFirstBlock() async {
        let p = makePlayer(voice: "stub-voice", doc: article)
        await runDry(p)
        p.pause()
        p.setDocument(ProgrammeDocument(id: "choir:b", title: "B", blocks: (0..<8).map { .speech(text: "Block \($0).") }))
        p.play()
        XCTAssertEqual(p.blockIndex, 0)
        p.pause()
    }

    // S1 at the player: play at the end with nothing new, and previous, are reported as plays.
    func testS1_PreviousAfterTheLastSegmentFinishedIsReported() async {
        let p = makePlayer(voice: "stub-voice", doc: segmentedDocument(["One."]))
        let seen = Locked<[String]>([])
        p.playback.observe { e in seen.write { $0.append(e.kind.rawValue) } }
        p.play()
        let a = await poll(10) { seen.read { $0.contains("finished") } }; XCTAssertTrue(a)
        let b = await poll(5) { !p.loopIsActive }; XCTAssertTrue(b)
        p.previous()
        let c = await poll(10) { seen.read { $0.filter { $0 == "finished" }.count == 2 } }
        XCTAssertTrue(c, "the replay was reported: \(seen.read { $0 })")
        XCTAssertEqual(seen.read { $0.filter { $0 == "segmentBegan" }.count }, 2)
    }

    // The late programme: handed over at the next block boundary while an article plays.
    func testHandOverReplacesAnArticleAtTheNextBlockBoundary() async {
        let p = makePlayer(voice: "stub-voice", doc: article)
        let seen = Locked<[String]>([])
        p.playback.observe { e in seen.write { $0.append("\(e.kind.rawValue):\(e.segmentID)") } }
        p.play()
        let started = await poll { p.loopIsActive }; XCTAssertTrue(started)
        var prog = ProgrammeDocument(id: "radio:late", title: "Autoradio", blocks: [])
        prog.append(SegmentPrep.prepare(IncomingSegment(stretchID: "late", segmentID: "late/0", kind: "story", title: "Late", text: "Late words.", cites: [], blockCount: nil))!)
        p.handOver(prog)
        let ok = await poll(10) { seen.read { $0.contains("finished:late/0") } }
        XCTAssertTrue(ok, "the programme played: \(seen.read { $0 })")
        XCTAssertEqual(p.document.id, "radio:late")
    }
    func testHandOverWhilePausedReplacesAtOnceAndMakesNoSound() async {
        let p = makePlayer(voice: "stub-voice", doc: article)          // never played: paused
        var prog = ProgrammeDocument(id: "radio:late", title: "Autoradio", blocks: [])
        prog.append(SegmentPrep.prepare(IncomingSegment(stretchID: "late", segmentID: "late/0", kind: "story", title: "Late", text: "Late words.", cites: [], blockCount: nil))!)
        p.handOver(prog)
        XCTAssertEqual(p.document.id, "radio:late"); XCTAssertTrue(p.userPaused); XCTAssertFalse(p.loopIsActive)
    }
    func testASegmentForAHandedOverDocumentWaitsWithIt() async {
        let p = makePlayer(voice: "long-voice", doc: article)
        p.play()
        let started = await poll { p.loopIsActive }; XCTAssertTrue(started)
        var prog = ProgrammeDocument(id: "radio:late", title: "Autoradio", blocks: [])
        prog.append(SegmentPrep.prepare(IncomingSegment(stretchID: "late", segmentID: "late/0", kind: "story", title: "Late", text: "Late words.", cites: [], blockCount: nil))!)
        p.handOver(prog)
        let more = SegmentPrep.prepare(IncomingSegment(stretchID: "late", segmentID: "late/1", kind: "story", title: "More", text: "More words.", cites: [], blockCount: nil))!
        XCTAssertTrue(p.appendSegment(more, toDocument: "radio:late"), "appended to the document waiting to take over")
        p.pause()
    }
}

final class Repair2StageTests: XCTestCase {
    var env: ChoirEnvironment!
    override func setUp() {
        env = ChoirEnvironment.shared
        env.keys = InMemoryKeyStore([KeyAccount.apiKey: secretKey, KeyAccount.computerID: "comp-1"])
        env.log = EventLog(clock: LabClock())
    }
    final class Manual: StretchSource, @unchecked Sendable {
        var cont: AsyncThrowingStream<StretchEvent, Error>.Continuation!
        func events(cause: String) async throws -> AsyncThrowingStream<StretchEvent, Error> { AsyncThrowingStream { self.cont = $0 } }
    }
    let article = { @Sendable () async throws -> ProgrammeDocument in ProgrammeDocument(id: "choir:a", title: "A", blocks: [.speech(text: "Article.")]) }
    func seg(_ n: Int) -> StretchEvent { .segment(IncomingSegment(stretchID: "m", segmentID: "m/\(n)", kind: "story", title: "S\(n)", text: "Words \(n).", cites: [], blockCount: nil)) }
    func player(_ doc: ProgrammeDocument) -> ProgrammePlayer {
        let r = StageRegistry.builtIn(); var cfg = StageRegistry.defaultConfiguration(); cfg.set(.voice, .init(id: "stub-voice"))
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        return ProgrammePlayer(audio: virtualAudio(), registry: r, log: EventLog(clock: LabClock()), configuration: cfg, document: doc)
    }

    // The deadline no longer loses a late programme: the article plays, the stretch is followed, and it takes over at a block boundary.
    func testALateFirstSegmentIsHandedOverAfterTheDeadline() async throws {
        let src = Manual()
        let stage = RadioProgramme(source: src, fallback: article, firstSegmentDeadline: 0.2)
        let o = try await stage.open()
        XCTAssertTrue(o.usedFallback); XCTAssertNotNil(o.follow, "it keeps following the stretch")
        let p = player(o.document)
        let follower = Task { await o.follow?(p) }
        try? await Task.sleep(nanoseconds: 100_000_000)
        src.cont.yield(seg(0)); src.cont.yield(seg(1))
        let ok = await poll { p.document.id == "radio:m" && p.document.segments.count == 2 }
        XCTAssertTrue(ok, "the programme took over with both segments")
        src.cont.finish()
        _ = await follower.value
    }
    func testACancelledFollowerOfALateStretchHandsNothingOver() async throws {
        let src = Manual()
        let stage = RadioProgramme(source: src, fallback: article, firstSegmentDeadline: 0.2)
        let o = try await stage.open()
        let p = player(o.document)
        let follower = Task { await o.follow?(p) }
        try? await Task.sleep(nanoseconds: 100_000_000)
        follower.cancel(); _ = await follower.value                      // a reload
        src.cont.yield(seg(0))
        try? await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertEqual(p.document.id, "choir:a")
    }

    // S2: sign-out clears `loading` even when a load is in flight.
    func testS2_SignOutClearsLoading() async {
        let cfg = URLSessionConfiguration.ephemeral
        cfg.protocolClasses = [StubProtocol.self]
        env.session = URLSession(configuration: cfg)
        StubProtocol.handler = { _ in .init(status: 404, json: ["error": "x"]) }
        defer { env.session = .shared }
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("p1-\(UUID().uuidString)")
        let rt = LabRuntime(supportDir: dir, session: NullSessionOwner())
        let slow = Manual()
        rt.registry.register(.programme, RadioProgramme.self) { _ in RadioProgramme(source: slow, fallback: self.article, firstSegmentDeadline: 60) }
        let app = ChoirApp(runtime: rt, keys: InMemoryKeyStore([KeyAccount.apiKey: secretKey, KeyAccount.computerID: "comp-1"]), openURL: nil)
        let first = Task { await app.loadProgramme() }
        let waiting = await poll { slow.cont != nil }; XCTAssertTrue(waiting)
        XCTAssertTrue(app.playerView().loading)
        await app.signOut()
        XCTAssertFalse(app.playerView().loading, "sign-out cleared it")
        slow.cont.yield(seg(0))
        await first.value
        XCTAssertFalse(app.playerView().loading, "and the stale load did not set it again")
        slow.cont.finish()
    }

    // S3: only bidi controls, the zero-width space and the byte-order mark are removed; joiners are part of the text.
    func testS3_JoinersAreKept() {
        let family = "\u{1F468}\u{200D}\u{1F469}\u{200D}\u{1F467}"
        XCTAssertEqual(SegmentPrep.title("Family \(family) day", kind: "story"), "Family \(family) day", "a ZWJ emoji sequence stays whole")
        let persian = "می\u{200C}خواهم"
        XCTAssertEqual(SegmentPrep.title(persian, kind: "story"), persian, "a ZWNJ inside a word stays")
        let hindi = "क्\u{200D}ष"
        XCTAssertEqual(SegmentPrep.title(hindi, kind: "story"), hindi)
        for bad in ["\u{202A}", "\u{202B}", "\u{202C}", "\u{202D}", "\u{202E}", "\u{2066}", "\u{2067}", "\u{2068}", "\u{2069}", "\u{200E}", "\u{200F}", "\u{061C}", "\u{200B}", "\u{FEFF}"] {
            XCTAssertEqual(SegmentPrep.title("Ha\(bad)rbour", kind: "story"), "Harbour", "U+\(String(bad.unicodeScalars.first!.value, radix: 16)) is removed")
        }
    }
}
