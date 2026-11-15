import XCTest
@testable import AutoradioKit

// Player controls (owner QA round 1, 2026-11-15): speed to 3x, next and previous story, autoplay, the DJ's candidates.
// The failure list is in mobile/README.md ("Player controls"); the numbers in the test names (P1 and so on) are its numbers.

// MARK: Speed

final class PlaybackSpeedTests: XCTestCase {
    func testP1_RangeIsHalfToThreeAndGarbageIsOne() {
        XCTAssertEqual(PlaybackSpeed.clamp(9), 3); XCTAssertEqual(PlaybackSpeed.clamp(0.01), 0.5); XCTAssertEqual(PlaybackSpeed.clamp(2.5), 2.5)
        XCTAssertEqual(PlaybackSpeed.clamp(.nan), 1); XCTAssertEqual(PlaybackSpeed.clamp(.infinity), 1)
    }
    func testP2_ASpeedOffTheStepSnapsToTheStep() {
        XCTAssertEqual(PlaybackSpeed.snap(1.2345), 1.25); XCTAssertEqual(PlaybackSpeed.snap(1.02), 1); XCTAssertEqual(PlaybackSpeed.snap(2.99), 3)
        XCTAssertEqual(PlaybackSpeed.snap(0.7), 0.7); XCTAssertEqual(PlaybackSpeed.snap(1.15), 1.15)
        XCTAssertEqual(PlaybackSpeed.name(1), "1"); XCTAssertEqual(PlaybackSpeed.name(1.25), "1.25"); XCTAssertEqual(PlaybackSpeed.name(2.5), "2.5")
        XCTAssertEqual(PlaybackSpeed.name(3), "3"); XCTAssertEqual(PlaybackSpeed.name(1.05), "1.05"); XCTAssertEqual(PlaybackSpeed.name(7), "3")
    }
    func suite() -> UserDefaults { let n = "speed-\(UUID().uuidString)"; let d = UserDefaults(suiteName: n)!; d.removePersistentDomain(forName: n); return d }
    func testP5_TheChosenSpeedIsKeptAndAGarbageOneIsIgnored() {
        let d = suite(); let s = SpeedStore(defaults: d)
        XCTAssertNil(s.load(), "nothing saved")
        s.save(2.75); XCTAssertEqual(SpeedStore(defaults: d).load(), 2.75, "a new store on the same defaults reads it")
        s.save(9); XCTAssertEqual(s.load(), 3, "saved clamped")
        for bad in [0.0, -1, 99, Double.nan] as [Double] { d.set(bad, forKey: "autoradio.playbackSpeed"); XCTAssertNil(s.load(), "\(bad)") }
        d.set("fast", forKey: "autoradio.playbackSpeed"); XCTAssertNil(s.load())
    }
    func testP1_TheTrackerAndThePlayerAndTheAudioAgreeOnTheCeiling() {
        let tr = BarTracker(); let got = Locked<[Double]>([])
        tr.speedSink = { v in got.write { $0.append(v) } }
        tr.setSpeed(3); tr.setSpeed(9); tr.setSpeed(1.2345)
        XCTAssertEqual(got.read { $0 }, [3, 3, 1.25])
        let a = virtualAudio(); a.setProgrammeRate(3)
        XCTAssertEqual(a.programmeRate, 3); XCTAssertEqual(a.timePitch.rate, 3, "the time-pitch unit takes it (its range is 1/32 to 32)")
        a.setProgrammeRate(.nan); XCTAssertEqual(a.programmeRate, 1)
        var info = RadioInfo(); info.speedValue = 3
        XCTAssertEqual(info.speedValue, 3)
        var snap = LabSnapshot(); snap.running = true; snap.blockCount = 1
        let r = tr.radioInfo(player: ChoirPlayerView(), snapshot: snap, document: ProgrammeDocument(id: "d", title: "T", blocks: [.speech(text: "One.")]), route: "r")
        XCTAssertEqual(r.speedValue, 1.25); XCTAssertEqual(r.speed, "1.25")
    }
    func testP4_ThreeTimesIsThreeTimesFasterForAnyVoice() async {
        let a = virtualAudio(); a.programmePlay()
        a.setProgrammeRate(3)
        let t0 = Date(); await a.playProgramme([Float](repeating: 0, count: 48_000), rate: 16_000, tag: 1)       // 3 s of source audio
        let took = Date().timeIntervalSince(t0)
        XCTAssertLessThan(took, 1.5); XCTAssertGreaterThan(took, 0.8, "3 s at 3x is about 1 s")
    }
    func testP3_ChangingTheSpeedMidSentenceNeverMovesThePositionBack() async {
        let a = virtualAudio(); a.programmePlay(); a.setProgrammeRate(1)
        let task = Task { await a.playProgramme([Float](repeating: 0, count: 48_000), rate: 16_000, tag: 2) }
        try? await Task.sleep(nanoseconds: 400_000_000)
        let before = a.programmePosition(tag: 2)?.seconds ?? -1
        a.setProgrammeRate(3)
        try? await Task.sleep(nanoseconds: 100_000_000)
        let after = a.programmePosition(tag: 2)?.seconds ?? -1
        XCTAssertGreaterThanOrEqual(after, before, "the position holds across the change")
        XCTAssertGreaterThan(before, 0.2)
        let t0 = Date(); await task.value
        XCTAssertLessThan(Date().timeIntervalSince(t0), 1.0, "the rest of the 3 s block goes at 3x, not at 1x")
    }
}

// MARK: The list, previous and the autoplay decision

final class StoryCursorTests: XCTestCase {
    let items = ["a", "b", "c"].map { StoryRef(route: $0, title: "T\($0)") }
    func testP7_NextAtTheLastAndPreviousAtTheFirstGoNowhere() {
        XCTAssertEqual(StoryCursor.neighbour(of: "a", in: items, step: 1)?.route, "b")
        XCTAssertEqual(StoryCursor.neighbour(of: "b", in: items, step: -1)?.route, "a")
        XCTAssertNil(StoryCursor.neighbour(of: "c", in: items, step: 1), "no wrap")
        XCTAssertNil(StoryCursor.neighbour(of: "a", in: items, step: -1))
    }
    func testP9_AStoryNotInTheListHasNoNeighbour() {
        XCTAssertNil(StoryCursor.neighbour(of: "z", in: items, step: 1)); XCTAssertNil(StoryCursor.neighbour(of: "a", in: [], step: 1))
    }
    func testP8_PreviousRestartsAfterThreeSecondsOrPastTheFirstBlock() {
        XCTAssertFalse(StoryCursor.previousRestartsCurrent(blockIndex: 0, secondsIntoBlock: 0))
        XCTAssertFalse(StoryCursor.previousRestartsCurrent(blockIndex: 0, secondsIntoBlock: 2.9))
        XCTAssertTrue(StoryCursor.previousRestartsCurrent(blockIndex: 0, secondsIntoBlock: 3))
        XCTAssertTrue(StoryCursor.previousRestartsCurrent(blockIndex: 1, secondsIntoBlock: 0))
    }
    func testP10_TheReadersTextDocumentNamesItsRoute() {
        XCTAssertEqual(StoryCursor.route(ofTextDocument: "text:pub:harbour"), "harbour")
        XCTAssertNil(StoryCursor.route(ofTextDocument: "text:mail-1")); XCTAssertNil(StoryCursor.route(ofTextDocument: "text:pub:")); XCTAssertNil(StoryCursor.route(ofTextDocument: "choir:x"))
    }
    func testP10_FrontOrderKeepsTheStackAsComposed() {
        let o = FrontOrder(); o.set(routes: ["x", "y"], titles: ["X"])
        XCTAssertEqual(o.stories, [StoryRef(route: "x", title: "X"), StoryRef(route: "y", title: "")])
    }
    func testP12to16_TheAutoplayDecision() {
        func d(_ paused: Bool, _ turn: Bool, _ played: Bool, _ next: Bool) -> AutoplayDecision { Autoplay.decide(userPaused: paused, turnOpen: turn, anyPlayed: played, hasNext: next) }
        XCTAssertEqual(d(false, false, true, true), .advance, "P12: simply on")
        XCTAssertEqual(d(true, false, true, true), .stop, "P13: the reader paused")
        XCTAssertEqual(d(false, true, true, true), .waitForTurn, "P13: the reader is talking")
        XCTAssertEqual(d(false, false, false, true), .stop, "P15: nothing was heard")
        XCTAssertEqual(d(false, false, true, false), .stop, "P16: the list is over")
        XCTAssertEqual(d(false, true, true, false), .stop, "P16: over, even with a turn open")
        XCTAssertEqual(d(true, true, true, true), .stop)
    }
}

// MARK: The DJ's candidates

final class StretchCandidatesTests: XCTestCase {
    func row(_ route: String, title: String = "Title", dek: String = "A dek.", importance: Int = 5, extra: [String: Any] = [:]) -> [String: Any] {
        var o: [String: Any] = ["title": title, "dek": dek, "route": ["bare": route], "importance": importance, "latest_revision": 3, "version_count": 9]
        for (k, v) in extra { o[k] = v }
        return o
    }
    func testP19_OrderedByTheWeightAndStableAndTheVersionIsTheLatestRevision() {
        let c = StretchCandidates.fromFront(["stories": [row("low", importance: 2), row("high", importance: 9), row("mid-a", importance: 5), row("mid-b", importance: 5)]])
        XCTAssertEqual(c.map(\.route), ["high", "mid-a", "mid-b", "low"])
        XCTAssertEqual(c.first?.version, 3); XCTAssertEqual(c.first?.ref, "pub:high@v3")
    }
    func testP20_AnythingTheServerWouldRefuseIsShapedOrDropped() {
        let long = String(repeating: "é", count: 400)                                   // 800 bytes
        let rows = [row("ok", title: long, dek: String(repeating: "d", count: 2000)), row("nodek", dek: "  "), row("a b"), row("a@b"), row("a#b"), row(""),
                    row("dup"), row("dup"), row("two\nlines", title: "One\nline here"), row("notitle", title: " ")]
        let c = StretchCandidates.fromFront(["stories": rows])
        XCTAssertEqual(c.map(\.route), ["ok", "dup"])
        XCTAssertLessThanOrEqual(c[0].title.utf8.count, 300); XCTAssertLessThanOrEqual(c[0].dek.utf8.count, 1000)
        XCTAssertTrue(c[0].title.allSatisfy { $0 == "é" }, "cut on a character, not a byte")
        let many = StretchCandidates.fromFront(["stories": (0..<55).map { row("r\($0)") }])
        XCTAssertEqual(many.count, 40, "at most 40")
    }
    func testP20_TextOnlyOnTheFirstTenAndNeverOver8KB() {
        let items = StretchCandidates.fromFront(["stories": (0..<14).map { row("r\($0)", importance: 10 - ($0 / 2)) }])
        var texts: [String: String] = [:]
        for c in items { texts[c.route] = String(repeating: "word ", count: 5000) + "\n\nSecond paragraph." }
        let body = StretchCandidates.body(items, texts: texts)
        XCTAssertEqual(body.count, 14)
        XCTAssertEqual(body.prefix(10).filter { $0["text"] != nil }.count, 10)
        XCTAssertEqual(body.suffix(4).filter { $0["text"] != nil }.count, 0)
        for b in body.prefix(10) { XCTAssertLessThanOrEqual((b["text"] as? String ?? "").utf8.count, 8 * 1024) }
        // a short story keeps its paragraphs and loses its markup
        let one = StretchCandidates.body([items[0]], texts: [items[0].route: "# Heading\n\nFirst **bold** [link](http://x).\n\nSecond."])
        XCTAssertEqual(one[0]["text"] as? String, "Heading\n\nFirst bold link.\n\nSecond.")
        // a candidate with no text still goes
        XCTAssertNil(StretchCandidates.body([items[0]], texts: [:])[0]["text"])
    }
    func testP23_TheRoutesAlreadyToldCameFromTheCites() {
        var d = segmentedDocument(["One.", "Two."])
        d.segments[0].cites = [ProgrammeCite(ref: "pub:a@v2", label: "A"), ProgrammeCite(ref: "pub:b/c@v1"), ProgrammeCite(ref: "edition:P1")]
        XCTAssertEqual(StretchCandidates.routes(citedIn: d), ["a", "b/c"])
    }
}

// MARK: The request and the app

final class PlayerAppTests: XCTestCase {
    var env: ChoirEnvironment!
    let failing = Locked<Set<String>>([])
    var programmeCalls: [StubProtocol.Req] { StubProtocol.seen.filter { $0.path == "/api/autoradio/programme" } }

    override func setUp() {
        env = ChoirEnvironment.shared
        let cfg = URLSessionConfiguration.ephemeral
        cfg.protocolClasses = [StubProtocol.self]
        env.keys = InMemoryKeyStore([KeyAccount.apiKey: secretKey, KeyAccount.computerID: "comp-1"])
        env.log = EventLog(clock: LabClock()); env.session = URLSession(configuration: cfg)
        StubProtocol.seen = []
        failing.write { $0 = [] }
        StubProtocol.handler = { [self] req in front(req) }
    }
    override func tearDown() { env.session = .shared; LabEnvironment.virtualAudio = false; LabEnvironment.promptsAllowed = true }

    func front(_ req: StubProtocol.Req) -> StubProtocol.Resp {
        switch req.path {
        case "/api/platform/publications/front":
            return .init(json: ["stories": ["a", "b", "c"].enumerated().map { i, r in
                ["title": "Story \(r)", "dek": "Dek \(r).", "route": ["bare": r], "section": "world", "words": 40, "importance": 9 - i] as [String: Any] }])
        case "/api/platform/publications/resolve":
            let r = req.query["route"] ?? ""
            if failing.read({ $0.contains(r) }) { return .init(status: 404, json: ["error": "gone"]) }
            return .init(json: ["publication": ["title": "Story \(r)"], "version": ["id": "pv-\(r)", "manifest_hash": "mh"], "payload": ["text": "Text of \(r)."]])
        default: return .init(status: 404, json: ["error": "nope"])
        }
    }

    final class EmptyVoice: Voice, @unchecked Sendable {
        static let stageID = "empty-voice"
        static var descriptor: StageDescriptor { StageDescriptor(displayName: "empty voice") }
        init(_ p: StageParams) {}
        func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error> { AsyncThrowingStream { $0.finish() } }
    }

    func makeApp(voice: String = "stub-voice") async -> (ChoirApp, LabRuntime) {
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("pc-\(UUID().uuidString)")
        let rt = LabRuntime(supportDir: dir, session: NullSessionOwner())
        rt.registry.register(.voice, StubVoice.self) { StubVoice($0) }
        rt.registry.register(.voice, LongVoice.self) { LongVoice($0) }
        rt.registry.register(.voice, EmptyVoice.self) { EmptyVoice($0) }
        rt.setStage(.voice, id: voice)
        await rt.start(virtual: true)
        rt.player.pause()
        let app = ChoirApp(runtime: rt, keys: InMemoryKeyStore([KeyAccount.apiKey: secretKey, KeyAccount.computerID: "comp-1"]), openURL: nil)
        await app.loadStories()
        return (app, rt)
    }
    func resolved() -> [String] { StubProtocol.seen.filter { $0.path == "/api/platform/publications/resolve" }.compactMap { $0.query["route"] } }

    // P12, P16: a story ends and the next starts by itself; at the last it stops.
    func testP12_P16_AutoplayRunsTheListAndStopsAtTheEnd() async {
        let (app, rt) = await makeApp()
        await app.play(route: "a")
        let b = await poll(8) { rt.player.document.id == "choir:pv-b" }; XCTAssertTrue(b, "the second story started by itself")
        let c = await poll(8) { rt.player.document.id == "choir:pv-c" }; XCTAssertTrue(c)
        let over = await poll(5) { !rt.player.loopIsActive }; XCTAssertTrue(over)
        try? await Task.sleep(nanoseconds: 600_000_000)
        XCTAssertEqual(rt.player.document.id, "choir:pv-c", "it stopped at the end of the list")
        XCTAssertEqual(resolved().filter { $0 == "a" || $0 == "b" || $0 == "c" }, ["a", "b", "c"], "each once, in the list's order")
        rt.player.pause()
    }

    // P13: the reader paused.
    func testP13_ThePausedReaderIsNotInterrupted() async {
        let (app, rt) = await makeApp()
        await app.play(route: "a")
        rt.player.pause()
        await app.ranDry(documentID: rt.player.document.id, anyPlayed: true)
        XCTAssertEqual(rt.player.document.id, "choir:pv-a")
        try? await Task.sleep(nanoseconds: 500_000_000)
        XCTAssertEqual(rt.player.document.id, "choir:pv-a", "nothing started")
        XCTAssertEqual(resolved(), ["a"])
    }

    // P13: the reader is talking to the radio: the next story waits for the end of the turn, then starts once.
    func testP13_ATurnInProgressHoldsTheNextStoryUntilItEnds() async {
        let (app, rt) = await makeApp()
        rt.log.emit(.phase(from: "playing", to: "candidate"))
        let open = await poll { rt.snapshot().phase == "candidate" }; XCTAssertTrue(open)
        await app.play(route: "a")
        let dry = await poll(5) { !rt.player.loopIsActive }; XCTAssertTrue(dry, "story a ran out of blocks")
        try? await Task.sleep(nanoseconds: 600_000_000)
        XCTAssertEqual(rt.player.document.id, "choir:pv-a", "not over the reader's words")
        rt.log.emit(.phase(from: "candidate", to: "playing"))
        let b = await poll(8) { rt.player.document.id == "choir:pv-b" || rt.player.document.id == "choir:pv-c" }
        XCTAssertTrue(b, "the turn ended: it carries on")
        rt.player.pause()
    }

    // P14: a story that fails to load is skipped once, and the reader is told.
    func testP14_AFailedLoadIsSkippedOnceAndSaid() async {
        let (app, rt) = await makeApp()
        failing.write { $0 = ["b"] }
        await app.play(route: "a")
        let c = await poll(8) { rt.player.document.id == "choir:pv-c" }; XCTAssertTrue(c, "b was skipped")
        XCTAssertTrue(app.playerView().message.contains("Story b"), "the reader is told which: \(app.playerView().message)")
        XCTAssertTrue(app.playerView().message.contains("skipped"))
        rt.player.pause()
    }
    func testP14_TwoFailuresInARowStop() async {
        let (app, rt) = await makeApp()
        failing.write { $0 = ["b", "c"] }
        await app.play(route: "a")
        let said = await poll(8) { app.playerView().message.contains("Stopping") }
        XCTAssertTrue(said, app.playerView().message)
        XCTAssertEqual(rt.player.document.id, "choir:pv-a")
        XCTAssertEqual(resolved().filter { $0 == "b" || $0 == "c" }, ["b", "c"], "one try each, no more")
        rt.player.pause()
    }

    // P15: a voice that makes no sound for any block is not a finished story.
    func testP15_ASilentVoiceDoesNotRunThroughTheList() async {
        let (app, rt) = await makeApp(voice: "empty-voice")
        await app.play(route: "a")
        let dry = await poll(5) { !rt.player.loopIsActive }; XCTAssertTrue(dry)
        try? await Task.sleep(nanoseconds: 500_000_000)
        XCTAssertEqual(rt.player.document.id, "choir:pv-a")
        XCTAssertEqual(resolved(), ["a"], "no other story was fetched")
        rt.player.pause()
    }

    // P7, P8: next and previous.
    func testP7_P8_NextAndPreviousFollowTheList() async {
        let (app, rt) = await makeApp(voice: "long-voice")                   // 3 s a block: the story stays put
        await app.play(route: "a")
        await app.moveStory(.previous)
        XCTAssertEqual(rt.player.document.id, "choir:pv-a", "previous at the first story restarts it")
        await app.moveStory(.next)
        XCTAssertEqual(rt.player.document.id, "choir:pv-b")
        await app.moveStory(.next)
        XCTAssertEqual(rt.player.document.id, "choir:pv-c")
        await app.moveStory(.next)
        XCTAssertEqual(rt.player.document.id, "choir:pv-c", "next at the last story does nothing")
        await app.moveStory(.previous)                                         // just begun: the story before
        XCTAssertEqual(rt.player.document.id, "choir:pv-b")
        rt.player.start(at: 1)                                                 // well into it
        await app.moveStory(.previous)
        XCTAssertEqual(rt.player.document.id, "choir:pv-b", "past the first seconds previous restarts this one")
        XCTAssertEqual(rt.player.blockIndex, 0)
        XCTAssertFalse(rt.player.userPaused)
        rt.player.pause()
    }

    // P10: the paper's stack is the list when the host knows it.
    func testP10_TheStackOrderWinsOverTheFrontList() async {
        let (app, rt) = await makeApp(voice: "long-voice")
        app.storyOrder = { ["c", "a", "b"].map { StoryRef(route: $0, title: "T") } }
        await app.play(route: "c")
        await app.moveStory(.next)
        XCTAssertEqual(rt.player.document.id, "choir:pv-a", "the stack's order, not the front list's")
        rt.player.pause()
    }

    // P9: text that is not in any list (mail's Read it to me): the buttons do not guess, they skip a passage.
    func testP9_ATextOutsideTheListsIsNeverGuessed() async {
        let (app, rt) = await makeApp(voice: "long-voice")
        XCTAssertTrue(rt.player.speakText(address: "mail-1", text: "One.\n\nTwo.\n\nThree.", startNow: true))
        await app.moveStory(.next)
        XCTAssertEqual(rt.player.document.id, "text:mail-1")
        XCTAssertEqual(rt.player.blockIndex, 1, "a passage, as the skip button does")
        await app.ranDry(documentID: "text:mail-1", anyPlayed: true)
        XCTAssertEqual(rt.player.document.id, "text:mail-1", "and nothing starts when it ends")
        XCTAssertEqual(resolved(), [])
        rt.player.pause()
    }

    // P10: the reader's Listen is a story of the list too.
    func testP10_AListenedStoryHasItsListToo() async {
        let (app, rt) = await makeApp(voice: "long-voice")
        app.noteListening(address: "pub:b")
        XCTAssertTrue(rt.player.speakText(address: "pub:b", text: "Text of b.", title: "Story b", startNow: true))
        XCTAssertEqual(rt.player.document.id, "text:pub:b")
        await app.moveStory(.next)
        XCTAssertEqual(rt.player.document.id, "choir:pv-c")
        rt.player.pause()
    }

    // P10: in a programme next and previous are segments.
    func testP10_InAProgrammeTheStoryButtonsSkipSegments() async {
        let (app, rt) = await makeApp(voice: "long-voice")
        rt.player.setDocument(segmentedDocument(["One.", "Two.", "Three."]))
        rt.player.start(at: 0); rt.player.play()
        await app.moveStory(.next)
        XCTAssertEqual(rt.player.currentSegmentIndex, 1)
        await app.moveStory(.next)
        await app.moveStory(.next)
        XCTAssertEqual(rt.player.currentSegmentIndex, 2, "no wrap")
        await app.moveStory(.previous)
        XCTAssertEqual(rt.player.currentSegmentIndex, 1, "just begun: the segment before")
        rt.player.pause()
    }

    // P11: the lock screen's buttons are the same moves.
    func testP11_TheLockScreenButtonsTakeTheStoryMoves() async {
        let (app, rt) = await makeApp(voice: "long-voice")
        await app.play(route: "a")
        XCTAssertEqual(rt.player.onNextTrack?(), true)
        let b = await poll { rt.player.document.id == "choir:pv-b" }; XCTAssertTrue(b)
        rt.player.setDocument(ProgrammeDocument(id: "text:mail-9", title: "M", blocks: [.speech(text: "x")]))
        XCTAssertEqual(rt.player.onNextTrack?(), false, "no list speaks for it: the player's own skip")
        rt.player.setDocument(segmentedDocument(["One.", "Two."]))
        XCTAssertEqual(rt.player.onNextTrack?(), false, "a programme: the player's own segment skip")
        _ = app
        rt.player.pause()
    }

    // P23: a stretch ends: one more request, leaving out what was told; a stretch that adds nothing is not asked for again.
    final class Told: StretchSource, @unchecked Sendable {
        let lock = NSLock(); var calls: [(cause: String, skip: Set<String>)] = []; var script: [[IncomingSegment]]
        init(_ s: [[IncomingSegment]]) { script = s }
        func events(cause: String) async throws -> AsyncThrowingStream<StretchEvent, Error> { try await events(cause: cause, excludingRoutes: []) }
        func events(cause: String, excludingRoutes: Set<String>) async throws -> AsyncThrowingStream<StretchEvent, Error> {
            lock.lock(); let n = calls.count; calls.append((cause, excludingRoutes)); lock.unlock()
            let segs = n < script.count ? script[n] : []
            return AsyncThrowingStream { c in for s in segs { c.yield(.segment(s)) }; c.yield(.done); c.finish() }
        }
    }
    func testP23_AnEndedStretchAsksOnceMoreAndStopsWhenNothingIsAdded() async {
        func seg(_ n: Int, cite: String) -> IncomingSegment {
            IncomingSegment(stretchID: "s\(n)", segmentID: "s\(n).s1", kind: "story", title: "S\(n)", text: "Words \(n).", cites: [ProgrammeCite(ref: "pub:\(cite)@v1", role: "subject", label: cite)], blockCount: nil)
        }
        let src = Told([[seg(1, cite: "a")], [seg(2, cite: "b")], []])
        let (app, rt) = await makeApp()
        let article = { @Sendable () async throws -> ProgrammeDocument in ProgrammeDocument(id: "choir:x", title: "X", blocks: [.speech(text: "x")]) }
        rt.registry.register(.programme, RadioProgramme.self) { _ in RadioProgramme(source: src, fallback: article, firstSegmentDeadline: 5) }
        await app.loadProgramme()
        rt.player.play()
        let two = await poll(8) { rt.player.document.segments.count == 2 }; XCTAssertTrue(two, "the second stretch was appended and played on")
        let three = await poll(8) { src.calls.count == 3 }; XCTAssertTrue(three)
        try? await Task.sleep(nanoseconds: 800_000_000)
        XCTAssertEqual(src.calls.map(\.cause), ["open", "extend", "extend"], "and no more once a stretch adds nothing")
        XCTAssertEqual(src.calls[1].skip, ["a"]); XCTAssertEqual(src.calls[2].skip, ["a", "b"])
        rt.player.pause()
    }
}
