import XCTest
@testable import AutoradioKit

// The failure list is at the top of AutoradioDesk.swift (numbers below refer to it). The contract is docs/reference/AUTORADIO-TURNS.md.
// A fake server stands in for the route (StubProtocol): no network.

final class AutoradioDeskTests: XCTestCase {
    var env: ChoirEnvironment!
    var log: EventLog!

    override func setUp() {
        let cfg = URLSessionConfiguration.ephemeral
        cfg.protocolClasses = [StubProtocol.self]
        log = EventLog(clock: LabClock())
        env = ChoirEnvironment.shared
        env.keys = InMemoryKeyStore([KeyAccount.apiKey: secretKey, KeyAccount.computerID: "comp-1"])
        env.log = log; env.session = URLSession(configuration: cfg)
        StubProtocol.seen = []
        StubProtocol.handler = { _ in .init() }
        TurnHistory.shared.clear()
    }
    override func tearDown() { env.session = .shared }

    static func sse(_ events: [(String, String)], comment: Bool = true) -> String {
        (comment ? ": autoradio\n\n" : "") + events.map { "event: \($0.0)\ndata: \($0.1)\n\n" }.joined()
    }
    static let doneJSON = #"{"turn_id":"x","status":"completed","sentences":2,"timings_ms":{"first_sentence":790,"done":1180}}"#
    func turn(_ words: String = "What is the top story?") -> DeskTurn {
        DeskTurn(turnID: "turn-abc12345", programmeID: "fixture-1", blockIndex: 2, listenerWords: words,
                 page: DeskPage(address: "pub:front@3#sha256:ab12", title: "Front page", text: "A strike ended in Nancy. Talks resume Monday."))
    }
    func collect(_ stage: AutoradioDeskStage, _ t: DeskTurn) async throws -> [DeskEvent] {
        var out: [DeskEvent] = []
        for try await e in stage.speech(t, heard: HeardLines()) { out.append(e) }
        return out
    }
    func turnRequests() -> [StubProtocol.Req] { StubProtocol.seen.filter { $0.path == "/api/autoradio/turns" } }

    // MARK: pieces

    func testTheParserReadsOneEventPerDataLineAndIgnoresCommentsAndUnknownNames() {
        var p = SSEParser()
        XCTAssertNil(p.push(": autoradio")); XCTAssertNil(p.push(""))
        XCTAssertNil(p.push("event: speech"))
        XCTAssertEqual(p.push(#"data: {"seq":0,"text":"Hi."}"#), SSEEvent(name: "speech", data: #"{"seq":0,"text":"Hi."}"#))
        XCTAssertEqual(p.push(#"data: {"x":1}"#), SSEEvent(name: "message", data: #"{"x":1}"#), "a data line without an event line is a message")
        XCTAssertNil(p.push("retry: 100"))
    }

    func testTheRequestBodyIsExactlyTheContractsFieldsInsideTheirBounds() {
        let t = DeskTurn(turnID: "t", programmeID: "fixture-1", blockIndex: 2, listenerWords: "What is the top story?", interrupted: true,
                         spokenText: String(repeating: "word ", count: 600), cutBlockIndex: 4,
                         page: DeskPage(address: "pub:front@3", title: "Front page", text: String(repeating: "é", count: 60_000)))
        let b = AutoradioDeskStage.body(t, id: "turn-1", history: [TurnHistory.Line(role: "listener", text: "Who won?"), TurnHistory.Line(role: "radio", text: "The union.")])
        XCTAssertEqual(Set(b.keys), ["turn_id", "words", "page", "heard", "programme", "history"])
        let page = b["page"] as! [String: Any]
        XCTAssertEqual(Set(page.keys), ["address", "title", "text"], "text_sha256 is omitted: the server hashes the text itself")
        XCTAssertLessThanOrEqual((page["text"] as! String).utf8.count, 48_000)
        XCTAssertFalse(((page["text"] as! String).unicodeScalars.contains("\u{FFFD}")), "cut on a character boundary")
        let h = b["heard"] as! [String: Any]
        XCTAssertEqual(h["block_index"] as? Int, 4)
        XCTAssertLessThanOrEqual((h["text"] as! String).count, 2000)
        XCTAssertEqual((b["history"] as! [[String: String]]).map { $0["role"]! }, ["listener", "radio"])
        XCTAssertEqual(b["words"] as? String, "What is the top story?")
    }

    func testNoPageNoHeardNoHistoryMeansNoSuchFields() {
        let b = AutoradioDeskStage.body(DeskTurn(turnID: "t", programmeID: "", blockIndex: 0, listenerWords: "hi"), id: "x", history: [])
        XCTAssertEqual(Set(b.keys), ["turn_id", "words"])
    }

    func testAWordsOverTheBoundIsKeptInside() {
        let b = AutoradioDeskStage.body(DeskTurn(turnID: "t", programmeID: "", blockIndex: 0, listenerWords: String(repeating: "ä", count: 3000)), id: "x", history: [])
        XCTAssertLessThanOrEqual((b["words"] as! String).utf8.count, 2000)
    }

    func testTurnIdsAreRouteSafe() {
        XCTAssertEqual(AutoradioDeskStage.routeID("turn-abc12345"), "turn-abc12345")
        let odd = AutoradioDeskStage.routeID(":bad id/with:colons" + String(repeating: "x", count: 100))
        XCTAssertTrue(odd.range(of: #"^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$"#, options: .regularExpression) != nil, odd)
    }

    func testHistoryKeepsTheLastEightFittingLinesAndNeverCutsOne() {
        let h = TurnHistory()
        for i in 0..<10 { h.add(listener: "q\(i)", radio: ["a\(i)"]) }
        h.add(listener: String(repeating: "x", count: 1200), radio: [])
        let s = h.snapshot()
        XCTAssertEqual(s.count, 8)
        XCTAssertTrue(s.allSatisfy { $0.text.utf8.count <= 1000 }, "a line over the bound is left out, not cut")
        XCTAssertEqual(s.last?.text, "a9")
    }

    // MARK: the stream (failure list 1 to 9)

    func testSpeechEventsArriveSentenceBySentenceThenDone() async throws {
        StubProtocol.handler = { _ in .init(raw: Self.sse([("speech", #"{"seq":0,"text":"The top story is a strike in Nancy."}"#),
                                                            ("handoff", #"{"x":1}"#),
                                                            ("speech", #"{"seq":1,"text":"Talks resume Monday."}"#), ("done", Self.doneJSON)])) }
        let ev = try await collect(AutoradioDeskStage(StageParams()), turn())
        XCTAssertEqual(ev, [.sentence(seq: 0, text: "The top story is a strike in Nancy."), .sentence(seq: 1, text: "Talks resume Monday."),
                            .done(status: "completed", sentences: 2, serverFirstSentenceMs: 790, serverDoneMs: 1180)], "the reserved handoff event is ignored")
        let r = try XCTUnwrap(turnRequests().first)
        XCTAssertEqual(r.method, "POST")
        XCTAssertEqual(r.headers["authorization"], "Bearer \(secretKey)")
        XCTAssertEqual(r.headers["x-choir-computer"], "comp-1")
        XCTAssertEqual(r.headers["accept"], "text/event-stream")
        XCTAssertEqual(r.body["turn_id"] as? String, "turn-abc12345")
        XCTAssertEqual((r.body["page"] as? [String: Any])?["title"] as? String, "Front page")
        XCTAssertNil((r.body["page"] as? [String: Any])?["text_sha256"])
    }

    func testAnErrorEventIsSpokenAndItsReasonIsLogged() async throws {
        StubProtocol.handler = { _ in .init(raw: Self.sse([("speech", #"{"seq":0,"text":"First."}"#),
                                                            ("error", #"{"spoken":"Sorry, I lost my place.","reason":"river refused"}"#),
                                                            ("done", #"{"status":"failed","sentences":1}"#)])) }
        let ev = try await collect(AutoradioDeskStage(StageParams()), turn())
        XCTAssertEqual(ev, [.sentence(seq: 0, text: "First."), .failed(spoken: "Sorry, I lost my place.", reason: "river refused"),
                            .done(status: "failed", sentences: 1, serverFirstSentenceMs: nil, serverDoneMs: nil)])
        XCTAssertTrue(log.events.contains { if case .error(let st, let m) = $0.event { return st == "desk" && m.contains("river refused") } else { return false } })
    }

    func testAStreamThatEndsAfterSentencesWithNoDoneIsInterruptedAndNotRetried() async throws {
        StubProtocol.handler = { _ in .init(raw: Self.sse([("speech", #"{"seq":0,"text":"First sentence."}"#)])) }
        let ev = try await collect(AutoradioDeskStage(StageParams()), turn())
        XCTAssertEqual(ev, [.sentence(seq: 0, text: "First sentence."), .done(status: "interrupted", sentences: 1, serverFirstSentenceMs: nil, serverDoneMs: nil)])
        XCTAssertEqual(turnRequests().count, 1, "the listener must not hear the first sentence twice")
    }

    func testADroppedStreamBeforeAnySentenceIsRetriedWithTheSameTurnIdAndWords() async throws {
        let n = Locked<Int>(0)
        StubProtocol.handler = { _ in
            let k = n.write { $0 += 1; return $0 }
            if k == 1 { return .init(raw: ": autoradio\n\n", dropAfter: true) }
            return .init(raw: Self.sse([("speech", #"{"seq":0,"text":"Here it is."}"#), ("done", #"{"status":"completed","sentences":1}"#)]))
        }
        let ev = try await collect(AutoradioDeskStage(StageParams()), turn())
        XCTAssertEqual(ev.first, .sentence(seq: 0, text: "Here it is."))
        let reqs = turnRequests()
        XCTAssertEqual(reqs.count, 2)
        XCTAssertEqual(reqs[0].body["turn_id"] as? String, reqs[1].body["turn_id"] as? String, "a retry of the same words keeps the id")
        XCTAssertEqual(reqs[0].body["words"] as? String, reqs[1].body["words"] as? String)
    }

    func testAStreamThatEndsCleanlyWithNoDoneAndNoSentencesFailsAfterThreeTries() async {
        StubProtocol.handler = { _ in .init(raw: ": autoradio\n\n") }
        do { _ = try await collect(AutoradioDeskStage(StageParams()), turn()); XCTFail("no answer, no done: an error") }
        catch { XCTAssertTrue("\(error)".contains("without a done"), "\(error)") }
        XCTAssertEqual(turnRequests().count, 3)
    }

    func test409OnTheFirstTryUsesAFreshTurnId() async throws {
        let n = Locked<Int>(0)
        StubProtocol.handler = { _ in
            let k = n.write { $0 += 1; return $0 }
            if k == 1 { return .init(status: 409, json: ["error": "turn id already used for different words"]) }
            return .init(raw: Self.sse([("speech", #"{"seq":0,"text":"Fine."}"#), ("done", #"{"status":"completed","sentences":1}"#)]))
        }
        let ev = try await collect(AutoradioDeskStage(StageParams()), turn())
        XCTAssertEqual(ev.first, .sentence(seq: 0, text: "Fine."))
        let reqs = turnRequests()
        XCTAssertEqual(reqs.count, 2)
        XCTAssertNotEqual(reqs[0].body["turn_id"] as? String, reqs[1].body["turn_id"] as? String)
    }

    func test409EveryTimeIsAnErrorAfterTheBackoffs() async {
        StubProtocol.handler = { _ in .init(status: 409, json: ["error": "being answered"]) }
        do { _ = try await collect(AutoradioDeskStage(StageParams()), turn()); XCTFail() }
        catch { XCTAssertEqual(error as? ChoirError, .server(status: 409, message: "being answered")) }
    }

    func testRefusalsBeforeAnyStreamAreTypedErrorsAndNothingIsSpoken() async {
        StubProtocol.handler = { _ in .init(status: 401, json: ["error": "no owner"]) }
        do { _ = try await collect(AutoradioDeskStage(StageParams()), turn()); XCTFail() }
        catch { XCTAssertEqual(error as? ChoirError, .unauthorized) }
        StubProtocol.handler = { _ in .init(status: 413, json: ["error": "page too large"]) }
        do { _ = try await collect(AutoradioDeskStage(StageParams()), turn()); XCTFail() }
        catch { XCTAssertEqual(error as? ChoirError, .server(status: 413, message: "page too large")) }
    }

    // A missing or unavailable route is a plain error (owner: computers are updated, the route is not planned to be missing): one short
    // honest line is spoken, the status is logged, and nothing else is asked (no polling desk, no retry).
    func testAMissingOrUnavailableRouteIsASpokenErrorAndNeverTheOtherDesk() async throws {
        for status in [404, 503] {
            let paths = Locked<[String]>([])
            StubProtocol.handler = { req in paths.write { $0.append(req.path) }; return .init(status: status, json: ["error": "nope"]) }
            let ev = try await collect(AutoradioDeskStage(StageParams()), turn())
            XCTAssertEqual(paths.value, ["/api/autoradio/turns"], "status \(status): one request, no other desk")
            XCTAssertTrue(ev.contains { if case .failed(let spoken, let reason) = $0 { return spoken == AutoradioDeskStage.unreachableLine && reason.contains("\(status)") } else { return false } }, "\(ev)")
            if case .done(let st, _, _, _)? = ev.last { XCTAssertEqual(st, "error") } else { XCTFail("done last: \(ev)") }
        }
    }

    func testNoTimerSourceSurvivesAroundModelCalls() throws {
        let dir = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().appendingPathComponent("Sources")
        let e = try XCTUnwrap(FileManager.default.enumerator(at: dir, includingPropertiesForKeys: nil))
        for case let u as URL in e where u.pathExtension == "swift" && !u.lastPathComponent.contains("generated") && u.lastPathComponent != "KeptTurns.swift" {
            let text = try String(contentsOf: u, encoding: .utf8)
            for word in ["system1Deadline", "stageDeadline", "holdTimeout", "timeoutSeconds"] {
                XCTAssertFalse(text.contains(word), "\(u.lastPathComponent) still names \(word)")
            }
        }
    }

    func testTheOnlyTransportTimeoutIsTheIdleOne() {
        XCTAssertEqual(ChoirClient.idleTimeout, 60)
        let c = env.client()
        XCTAssertEqual(c.timeout, ChoirClient.idleTimeout)
    }
}

// MARK: the engine speaks a streamed desk (failures: the desk's sentences wait for the local sentence in flight; none is lost or reordered)

final class ScriptedStreamingDesk: StreamingDesk, @unchecked Sendable {
    static let stageID = "scripted-stream-desk"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "scripted stream desk", params: [.number("delay", "d", 0.3, 0, 10), .number("gap", "g", 0.05, 0, 10)]) }
    nonisolated(unsafe) static var sentences: [String] = ["The desk says the strike ended.", "Talks resume Monday."]
    nonisolated(unsafe) static var requests: [DeskTurn] = []
    nonisolated(unsafe) static var heardBox: HeardLines?
    nonisolated(unsafe) static var hang = false
    let delay: Double, gap: Double
    init(_ p: StageParams) { delay = p.double("delay", 0.3); gap = p.double("gap", 0.05) }
    func submit(_ turn: DeskTurn) -> AsyncThrowingStream<Revision, Error> { AsyncThrowingStream { $0.finish() } }
    func speech(_ turn: DeskTurn, heard: HeardLines) -> AsyncThrowingStream<DeskEvent, Error> {
        Self.requests.append(turn); Self.heardBox = heard
        let (d, g, ss, hang) = (delay, gap, Self.sentences, Self.hang)
        return AsyncThrowingStream { c in
            let t = Task {
                try? await Task.sleep(nanoseconds: UInt64(d * 1e9))
                if hang { try? await Task.sleep(nanoseconds: 3_600_000_000_000); return }
                for (i, s) in ss.enumerated() {
                    if Task.isCancelled { return }
                    c.yield(.sentence(seq: i, text: s))
                    try? await Task.sleep(nanoseconds: UInt64(g * 1e9))
                }
                c.yield(.done(status: "completed", sentences: ss.count, serverFirstSentenceMs: 640, serverDoneMs: 900))
                c.finish()
            }
            c.onTermination = { _ in t.cancel() }
        }
    }
}

final class StreamedDeskEngineTests: XCTestCase {
    override func setUp() { AckHistory.shared.clear(); TurnHistory.shared.clear(); ScriptedStreamingDesk.sentences = ["The desk says the strike ended.", "Talks resume Monday."]; ScriptedStreamingDesk.hang = false; ScriptedStreamingDesk.requests = [] }
    override func tearDown() { LocalLanguage.model = nil }

    func make(model: LocalLanguageModel?, deskDelay: Double = 0.3, s1Delay: Double = 0.05, front: FrontPage? = FrontPage(address: "pub:front", title: "Front page", kind: "paper")) throws -> (TurnEngine, EventLog, Locked<TurnRecord?>, TurnEngine.Turn) {
        LocalLanguage.model = model
        let r = StageRegistry.builtIn()
        r.register(.system1, StubS1.self) { StubS1($0) }
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        r.register(.transcriber, StubTranscriber.self) { StubTranscriber($0) }
        r.register(.desk, ScriptedStreamingDesk.self) { ScriptedStreamingDesk($0) }
        var cfg = StageRegistry.defaultConfiguration()
        cfg.set(.system1, .init(id: "stub-s1", params: ["delay": .number(s1Delay), "asks": .string("yes")]))
        cfg.set(.voice, .init(id: "stub-voice")); cfg.set(.transcriber, .init(id: "stub-tr")); cfg.set(.holder, .init(id: "local-ack"))
        cfg.set(.desk, .init(id: "scripted-stream-desk", params: ["delay": .number(deskDelay)]))
        let log = EventLog(clock: LabClock())
        let got = Locked<TurnRecord?>(nil)
        let env = EngineEnvironment(registry: r, log: log, playback: SimulatedPlayback(position: ProgramPosition(programmeID: "p", blockIndex: 0, context: "ctx")),
                                    onRecord: { got.value = $0 }, frontPage: { front }, pageText: { _ in "A strike ended in Nancy." })
        let engine = TurnEngine(env: env, configuration: cfg)
        let t = TurnEngine.Turn(id: "t1", config: cfg, transcriber: try r.make(.transcriber, as: Transcriber.self, cfg),
                                yield: try r.make(.yieldPolicy, as: YieldPolicy.self, cfg), session: EmptyTranscription(), firstEventIndex: 0)
        log.emit(.turnEnded); log.emit(.finalTranscript("what is the top story"))
        return (engine, log, got, t)
    }

    func testTheDesksSentencesAreSpokenInOrderAfterTheLocalOneInFlightWithAHandover() async throws {
        let m = FakeLocalModel(pieces: ["Looking at the front page now. ", "The paper has several stories. ", "Checking which is first. ", "Still checking the order of them. ",
                                        "Looking at the second story. ", "Reading the third one now."], pieceDelay: 0.35)
        let (engine, log, got, t) = try make(model: m, deskDelay: 1.0)
        await engine.runPipeline(t, transcript: "what is the top story", endedAt: log.clock.now)
        let tl = try XCTUnwrap(TurnTimeline.compute(log.events).first)
        let desk = log.events.compactMap { e -> String? in if case .deskSentenceStarted(let t, _) = e.event { return t } else { return nil } }
        XCTAssertEqual(desk, ["The desk says the strike ended.", "Talks resume Monday."], "every desk sentence, in order, none lost")
        XCTAssertNotNil(tl.localCutMs, "the first desk sentence cut the local stream")
        XCTAssertEqual(tl.localCutReason, "the desk's speech was ready")
        XCTAssertNotNil(tl.transitionMs, "a hand-over sound between the phone's voice and the desk's")
        XCTAssertEqual(tl.deskStatus, "completed")
        XCTAssertEqual(tl.deskServerFirstSentenceMs, 640)
        XCTAssertNotNil(tl.deskFirstTokenMs); XCTAssertNotNil(tl.deskAudioStartMs)
        let ends = log.events.compactMap { e -> (String, Double)? in
            switch e.event { case .localSentenceStarted(_, let ms): return ("local", ms); case .transitionPlayed(let ms): return ("transition", ms); case .deskSentenceStarted(_, let ms): return ("desk", ms); default: return nil } }
        let order = ends.map(\.0)
        let ti = try XCTUnwrap(order.firstIndex(of: "transition"), "\(order)")
        do { XCTAssertTrue(order[..<ti].allSatisfy { $0 == "local" } && order[(ti + 1)...].allSatisfy { $0 == "desk" }, "\(order)") }
        XCTAssertNotNil(got.value)
    }

    func testTheRequestCarriesThePageAndItsText() async throws {
        let (engine, log, _, t) = try make(model: nil)
        await engine.runPipeline(t, transcript: "what is the top story", endedAt: log.clock.now)
        let r = try XCTUnwrap(ScriptedStreamingDesk.requests.first)
        XCTAssertEqual(r.page?.address, "pub:front"); XCTAssertEqual(r.page?.title, "Front page"); XCTAssertEqual(r.page?.text, "A strike ended in Nancy.")
        XCTAssertEqual(r.turnID, "t1")
    }

    func testWhatWasSaidAloudBecomesTheNextRequestsHistory() async throws {
        let m = FakeLocalModel(pieces: ["The front page has a strike story."])
        let (engine, log, _, t) = try make(model: m, deskDelay: 1.0)
        await engine.runPipeline(t, transcript: "what is the top story", endedAt: log.clock.now)
        let h = TurnHistory.shared.snapshot()
        XCTAssertEqual(h.first, TurnHistory.Line(role: "listener", text: "what is the top story"))
        XCTAssertTrue(h.contains(TurnHistory.Line(role: "radio", text: "The front page has a strike story.")))
        XCTAssertTrue(h.contains(TurnHistory.Line(role: "radio", text: "The desk says the strike ended.")))
    }

    func testADeskWithNothingToSayEndsTheTurnWithNothingSpoken() async throws {
        let (engine, log, _, t) = try make(model: nil)
        ScriptedStreamingDesk.sentences = []
        await engine.runPipeline(t, transcript: "what is the top story", endedAt: log.clock.now)
        let desk = log.events.filter { if case .deskSentenceStarted = $0.event { return true } else { return false } }
        XCTAssertTrue(desk.isEmpty, "nothing to say, nothing is spoken, and the turn still ends")
    }

    func testASystem1ThatNeverAnswersDoesNotHoldTheTurn() async throws {
        let (engine, log, got, t) = try make(model: nil, s1Delay: 3600)
        let t0 = Date()
        await engine.runPipeline(t, transcript: "what is the top story", endedAt: log.clock.now)
        XCTAssertLessThan(Date().timeIntervalSince(t0), 5, "the turn ended with the desk, not with System 1")
        XCTAssertNotNil(got.value)
        XCTAssertFalse(log.events.contains { if case .error(_, let m) = $0.event { return m.contains("no answer within") } else { return false } })
    }

    func testCancelStopsATurnWhoseDeskNeverAnswers() async throws {
        ScriptedStreamingDesk.hang = true
        let (engine, log, got, _) = try make(model: nil)
        let started = await engine.typedTurn("what is the top story")
        XCTAssertTrue(started)
        for _ in 0..<100 { let p = await engine.phase; if p == .holding || p == .acknowledging { break }; try await Task.sleep(nanoseconds: 50_000_000) }
        await engine.cancelTurn()
        await engine.waitUntilIdle()
        XCTAssertTrue(log.events.contains { if case .turnCancelled = $0.event { return true } else { return false } })
        XCTAssertNil(got.value, "a cancelled turn leaves no finished record")
        let phase = await engine.phase
        XCTAssertEqual(phase, .playing)
    }

    // The aim: a typed turn aimed at a page (the bar's `aim`) puts THAT page's address in the desk request, not the page the
    // environment calls the front one.
    func testATypedTurnAimedAtAPageSendsThatPagesAddressToTheDesk() async throws {
        let (engine, _, got, _) = try make(model: nil, front: FrontPage(address: "pub:front", title: "Front page", kind: "paper"))
        let started = await engine.typedTurn("what is this about", front: FrontPage(address: "pub:aimed", title: "Aimed story", kind: "story"))
        XCTAssertTrue(started)
        for _ in 0..<100 where got.value == nil { try await Task.sleep(nanoseconds: 100_000_000) }
        let r = try XCTUnwrap(ScriptedStreamingDesk.requests.first)
        XCTAssertEqual(r.page?.address, "pub:aimed"); XCTAssertEqual(r.page?.title, "Aimed story")
    }
}
