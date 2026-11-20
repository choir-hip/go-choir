import XCTest
@testable import AutoradioKit

// Lane autoradio-talk. Failure list (written before the code):
//  index:        a question the corpus cannot answer returns nothing; stop words are not relevance; a re-indexed document does not double;
//                a huge line is cut; plural and singular match; non-ASCII is kept.
//  conversation: the answer is kept; a failure is kept with a retry; a new question closes the old one; late sentences for a closed
//                question are ignored; nothing says "working" forever; a thank-you leaves no card.
//  first answer: nothing relevant -> the model is not asked and the line is an honest holding line; the model says NO_ANSWER -> same;
//                model missing, unloaded or failing -> same, no error; at most three sentences; the source story is named; the desk's
//                answer waits for the first answer to finish (arrives before it ends) and both are kept as one entry.
//  engine:       no network (the desk throws) -> a plain line is spoken and kept with a retry; the server answer never arrives -> Cancel
//                leaves a cancelled entry, not a failure; two questions in quick succession -> the first is cancelled, the second answered.

final class RetrievalIndexPortedTests: XCTestCase {      // the old PassageIndexTests, on the one index
    func makeIndex() -> RetrievalIndex {
        let i = RetrievalIndex()
        i.upsert(RetrievalDoc(id: "a", kind: "story", title: "Harbor strike ends after nine days",
              text: "Dock workers at the harbor voted on Tuesday to end a nine day strike over pay and shift lengths.\nThe union said talks with the port authority resume Monday.", address: "pub:a"))
        i.upsert(RetrievalDoc(id: "b", kind: "story", title: "Central bank holds interest rates",
              text: "The central bank left interest rates unchanged on Wednesday, citing steady inflation and a cooling labour market.", address: "pub:b"))
        return i
    }
    func search(_ i: RetrievalIndex, _ q: String, page: String? = nil) -> [RetrievalItem] { i.retrieve(.init(words: q, pageID: page), record: false).items }
    func testFindsTheRightStory() { XCTAssertEqual(search(makeIndex(), "what happened with the harbor strike").first?.id, "a") }
    func testPluralAndSingularMatch() { XCTAssertEqual(search(makeIndex(), "what did the bank do about interest rate").first?.id, "b") }
    func testAQuestionTheCorpusCannotAnswerReturnsNothing() {
        XCTAssertTrue(search(makeIndex(), "what is the weather on Mars today").isEmpty)
        XCTAssertTrue(search(makeIndex(), "who won the football match").isEmpty)
    }
    func testOneSharedCommonWordIsNotRelevance() { XCTAssertTrue(search(makeIndex(), "lightning strike survivors in Peru").isEmpty) }
    func testStopWordsOnlyAndEmpty() {
        XCTAssertTrue(search(makeIndex(), "what about it").isEmpty)
        XCTAssertTrue(search(makeIndex(), "").isEmpty)
        XCTAssertTrue(search(RetrievalIndex(), "harbor strike").isEmpty)
    }
    func testIndexingADocumentAgainReplacesIt() {
        let i = makeIndex(); let n = i.count
        i.upsert(RetrievalDoc(id: "a", kind: "story", title: "Harbor strike ends after nine days", text: "Dock workers ended the harbor strike on Tuesday after nine days of talks and pay.", address: "pub:a"))
        XCTAssertLessThanOrEqual(i.count, n)
        XCTAssertEqual(search(i, "harbor strike").filter { $0.id == "a" }.count, 1)
    }
    func testAHugeLineIsCutIntoBoundedPassages() {
        let line = (0..<400).map { "word\($0) harbor" }.joined(separator: " ")
        let parts = RetrievalText.passages(line)
        XCTAssertGreaterThan(parts.count, 1)
        XCTAssertTrue(parts.allSatisfy { $0.count <= 2000 })
    }
    func testNonASCIITextIsIndexed() {
        let i = RetrievalIndex()
        i.upsert(RetrievalDoc(id: "c", kind: "story", title: "Café reopens", text: "The café on rue Saint-Honoré reopens after the renovation, with a new espresso menu.", address: "pub:c"))
        XCTAssertEqual(search(i, "café renovation reopens").first?.id, "c")
    }
    func testThePageInFrontIsPreferred() {
        let i = RetrievalIndex()
        i.upsert(RetrievalDoc(id: "x", kind: "story", title: "Rates", text: "Interest rates stayed unchanged at the central bank this week.", address: "pub:x"))
        i.upsert(RetrievalDoc(id: "page:y", kind: "page", title: "Rates", text: "Interest rates stayed unchanged at the central bank this week.", address: "pub:y"))
        XCTAssertEqual(search(i, "central bank interest rates", page: "page:y").first?.id, "page:y")
    }
}

final class ConversationTests: XCTestCase {
    func testTheAnswerIsKeptAsItIsSpoken() {
        let c = Conversation()
        c.begin(id: "t1", question: "What changed?")
        XCTAssertEqual(c.entries().last?.status, "asking")
        c.addAnswer(id: "t1", sentence: "The strike ended.")
        c.addAnswer(id: "t1", sentence: "Talks resume Monday.")
        c.finishAnswer(id: "t1", status: "completed")
        let e = c.entry("t1")
        XCTAssertEqual(e?.answer, "The strike ended. Talks resume Monday.")
        XCTAssertEqual(e?.status, "answered")
    }
    func testAFailureIsKeptWithARetry() {
        let c = Conversation()
        c.begin(id: "t1", question: "What changed?")
        c.fail(id: "t1", note: "I can't reach the paper right now.")
        let e = c.entries().last
        XCTAssertEqual(e?.status, "failed"); XCTAssertEqual(e?.note, "I can't reach the paper right now.")
        XCTAssertEqual(e?.question, "What changed?")   // the card and its retry flag are bar.ts (bar.test.ts B8)
    }
    func testANewQuestionClosesTheOldOneAndLateSentencesAreIgnored() {
        let c = Conversation()
        c.begin(id: "t1", question: "first?"); c.begin(id: "t2", question: "second?")
        c.addAnswer(id: "t1", sentence: "late words for the first")
        c.addAnswer(id: "t2", sentence: "answer two")
        XCTAssertEqual(c.entry("t1")?.status, "cancelled"); XCTAssertEqual(c.entry("t1")?.answer, "")
        XCTAssertEqual(c.entry("t2")?.answer, "answer two")
        XCTAssertEqual(c.entries().last?.id, "t2")
    }
    func testNothingSaysWorkingForever() {
        let c = Conversation()
        c.begin(id: "t1", question: "q"); c.settle(id: "t1")
        XCTAssertEqual(c.entry("t1")?.status, "failed")
        c.begin(id: "t2", question: "q"); c.setFirst(id: "t2", text: "saved words", source: "From the paper's story on X"); c.settle(id: "t2")
        XCTAssertEqual(c.entry("t2")?.status, "answered"); XCTAssertFalse(c.entry("t2")?.note.isEmpty ?? true, "says the full answer did not arrive")
    }
    func testAThankYouLeavesNoCardAndCancelLeavesNoFailure() {
        let c = Conversation()
        c.begin(id: "t1", question: "thanks"); c.cancelOpen()
        XCTAssertEqual(c.entries().last?.status, "cancelled")   // no card for it: bar.ts
    }
    func testFirstAnswerThenFullAnswerIsOneEntry() {
        let c = Conversation()
        c.begin(id: "t1", question: "q")
        c.setFirst(id: "t1", text: "The strike ended.", source: "From the paper's story on Harbor")
        XCTAssertEqual(c.entries().last?.first, "The strike ended."); XCTAssertEqual(c.entries().last?.firstSource, "From the paper's story on Harbor")
        c.addAnswer(id: "t1", sentence: "Full: talks resume Monday."); c.finishAnswer(id: "t1", status: "completed")
        XCTAssertEqual(c.entries().last?.answer, "Full: talks resume Monday.")
        XCTAssertEqual(c.entries().count, 1)
    }
    func testDismissHidesTheCardUntilTheNextQuestion() {
        let c = Conversation()
        c.begin(id: "t1", question: "q"); c.addAnswer(id: "t1", sentence: "a"); c.finishAnswer(id: "t1", status: "completed")
        c.dismiss(); XCTAssertEqual(c.dismissedID(), "t1")
        c.begin(id: "t2", question: "q2"); XCTAssertEqual(c.dismissedID(), "", "a new question shows its card again")
        c.dismiss(id: "t1"); XCTAssertEqual(c.dismissedID(), "t1", "the card names the entry it shows (bar.ts answerId)")
    }
}

final class SavedAnswerTests: XCTestCase {
    override func setUp() { Conversation.shared.clear(); AckHistory.shared.clear(); TurnHistory.shared.clear() }
    override func tearDown() { LocalLanguage.model = nil }

    func corpus() -> SavedCorpus {
        let c = SavedCorpus(heldPages: nil)
        c.setSource("t") { [SavedCorpus.Document(id: "pub:a", title: "Harbor strike ends after nine days", address: "pub:a",
            text: "Dock workers at the harbor voted on Tuesday to end a nine day strike over pay and shift lengths.\nThe union said talks with the port authority resume Monday.")] }
        return c
    }
    func holder(_ m: LocalLanguageModel?, _ c: SavedCorpus) -> SavedAnswerHolder { SavedAnswerHolder(StageParams(), corpus: c, model: { m }) }
    func collect(_ s: AsyncStream<String>) async -> [String] { var out: [String] = []; for await l in s { out.append(l) }; return out }

    func testAnswersFromTheSavedStoryAndNamesIt() async {
        let m = FakeLocalModel(pieces: ["The dock workers voted to end the strike. ", "Talks with the port authority resume Monday. "])
        let h = holder(m, corpus())
        Conversation.shared.begin(id: "t1", question: "what happened with the harbor strike")
        let lines = await collect(h.lines(for: HolderInput(listenerWords: "what happened with the harbor strike", answers: [])))
        XCTAssertEqual(lines.count, 2)
        XCTAssertTrue(lines[0].hasPrefix("From the paper's story on Harbor strike ends after nine days: The dock workers"), lines[0])
        XCTAssertTrue(h.keepsSpeakingForDesk)
        XCTAssertEqual(Conversation.shared.entry("t1")?.first, "The dock workers voted to end the strike. Talks with the port authority resume Monday.")
        XCTAssertTrue(m.asked.first?.prefix.contains("Passage 1") ?? false)
        XCTAssertTrue(m.asked.first?.system.contains("NO_ANSWER") ?? false, "told to refuse rather than use its own knowledge")
    }
    func testAtMostThreeSentences() async {
        let m = FakeLocalModel(pieces: ["One is here. Two is here. Three is here. Four is here. Five is here."])
        let lines = await collect(holder(m, corpus()).lines(for: HolderInput(listenerWords: "harbor strike pay", answers: [])))
        XCTAssertEqual(lines.count, 3)
    }
    func testAQuestionTheCorpusCannotAnswerNeverAsksTheModel() async {
        let m = FakeLocalModel(pieces: ["An invented answer about Mars."])
        let h = holder(m, corpus())
        let lines = await collect(h.lines(for: HolderInput(listenerWords: "what is the weather on Mars today", answers: [])))
        XCTAssertEqual(lines.count, 1)
        XCTAssertEqual(lines[0], "I have nothing on that in the paper.", "it names what it searched")
        XCTAssertTrue(m.asked.isEmpty, "the small model must not answer from its own knowledge")
        XCTAssertFalse(h.keepsSpeakingForDesk, "a holding line is cut by the desk's answer")
    }
    func testNoAnswerFromTheModelIsAHoldingLineNotAnAnswer() async {
        let m = FakeLocalModel(pieces: ["NO_ANSWER"])
        let h = holder(m, corpus())
        let lines = await collect(h.lines(for: HolderInput(listenerWords: "how many dock workers voted at the harbor", answers: [])))
        XCTAssertEqual(lines.count, 1); XCTAssertTrue(lines[0].hasPrefix("Let me look that up"))
        XCTAssertFalse(h.keepsSpeakingForDesk)
    }
    func testModelMissingUnloadedOrFailingSaysOnlyTheHoldingLine() async {
        for m in [nil, FakeLocalModel(loaded: false), FakeLocalModel(fails: true)] as [LocalLanguageModel?] {
            let lines = await collect(holder(m, corpus()).lines(for: HolderInput(listenerWords: "what happened with the harbor strike", answers: [])))
            XCTAssertEqual(lines.count, 1, "\(String(describing: m))"); XCTAssertTrue(lines[0].hasPrefix("Let me look that up"))
        }
    }
    func testTheHoldingLineSaysWhatIsLookedUp() {
        XCTAssertEqual(SavedAnswerPrompt.holdingLine("What is the weather on Mars today?"), "Let me look that up: What is the weather on Mars today.")
        XCTAssertEqual(SavedAnswerPrompt.holdingLine("  "), "Let me look that up.")
    }
    func testCancelledWhileComposingSpeaksNothingMore() async {
        let m = FakeLocalModel(pieces: ["First sentence here. ", "Second sentence here. "], pieceDelay: 0.3)
        let h = holder(m, corpus())
        let s = h.lines(for: HolderInput(listenerWords: "harbor strike pay", answers: []))
        var got: [String] = []
        for await l in s { got.append(l); h.cancel() }
        XCTAssertLessThanOrEqual(got.count, 1)
    }

    // MARK: engine

    func engine(desk: String, model: LocalLanguageModel?, corpus c: SavedCorpus, deskParams: [String: JSONValue] = [:]) throws -> (TurnEngine, EventLog, Locked<TurnRecord?>) {
        LocalLanguage.model = model
        let r = StageRegistry.builtIn()
        r.register(.system1, StubS1.self) { StubS1($0) }
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        r.register(.transcriber, StubTranscriber.self) { StubTranscriber($0) }
        r.register(.desk, ScriptedStreamingDesk.self) { ScriptedStreamingDesk($0) }
        r.register(.desk, FailingStreamingDesk.self) { FailingStreamingDesk($0) }
        r.register(.holder, SavedAnswerHolder.self) { SavedAnswerHolder($0, corpus: c, model: { LocalLanguage.model }) }
        var cfg = StageRegistry.defaultConfiguration()
        cfg.set(.system1, .init(id: "stub-s1", params: ["delay": .number(0.02), "asks": .string("yes")]))
        cfg.set(.voice, .init(id: "stub-voice")); cfg.set(.transcriber, .init(id: "stub-tr")); cfg.set(.holder, .init(id: "saved-answer"))
        cfg.set(.desk, .init(id: desk, params: deskParams))
        let log = EventLog(clock: LabClock())
        let got = Locked<TurnRecord?>(nil)
        let env = EngineEnvironment(registry: r, log: log, playback: SimulatedPlayback(position: ProgramPosition(programmeID: "p", blockIndex: 0, context: "ctx")),
                                    onRecord: { got.value = $0 })
        return (TurnEngine(env: env, configuration: cfg), log, got)
    }
    func spoken(_ log: EventLog) -> [String] {
        log.events.compactMap { e -> String? in
            switch e.event { case .localSentenceStarted(let t, _): return "local:" + t; case .deskSentenceStarted(let t, _): return "desk:" + t; default: return nil } }
    }
    func waitDone(_ got: Locked<TurnRecord?>) async throws { for _ in 0..<200 where got.value == nil { try await Task.sleep(nanoseconds: 50_000_000) } }

    func testTheServerAnswerArrivingBeforeTheFirstAnswerEndsWaitsForIt() async throws {
        ScriptedStreamingDesk.sentences = ["The desk says the strike ended Tuesday.", "Talks resume Monday."]; ScriptedStreamingDesk.hang = false
        let m = FakeLocalModel(pieces: ["The dock workers voted to end the strike. ", "Talks resume Monday with the port authority. ", "The vote followed nine days. "], pieceDelay: 0.2)
        let (e, log, got) = try engine(desk: "scripted-stream-desk", model: m, corpus: corpus(), deskParams: ["delay": .number(0.5)])
        let started = await e.typedTurn("what happened with the harbor strike")
        XCTAssertTrue(started)
        try await waitDone(got)
        let order = spoken(log)
        XCTAssertEqual(order.filter { $0.hasPrefix("local:") }.count, 3, "the first answer was heard to its end: \(order)")
        let firstDesk = try XCTUnwrap(order.firstIndex { $0.hasPrefix("desk:") })
        XCTAssertTrue(order[..<firstDesk].allSatisfy { $0.hasPrefix("local:") } && order[firstDesk...].allSatisfy { $0.hasPrefix("desk:") }, "\(order)")
        let entries = Conversation.shared.entries()
        XCTAssertEqual(entries.count, 1, "one conversation, not two")
        XCTAssertEqual(entries[0].status, "answered")
        XCTAssertFalse(entries[0].first.isEmpty); XCTAssertEqual(entries[0].answer, "The desk says the strike ended Tuesday. Talks resume Monday.")
    }

    func testNoNetworkIsSaidInPlainWordsAndKeptWithARetry() async throws {
        let (e, log, got) = try engine(desk: "failing-stream-desk", model: nil, corpus: corpus())
        _ = await e.typedTurn("what is the top story")
        try await waitDone(got)
        let entry = try XCTUnwrap(Conversation.shared.entries().last)
        XCTAssertEqual(entry.status, "failed")
        XCTAssertFalse(entry.note.isEmpty)
        XCTAssertTrue(spoken(log).contains("desk:" + entry.note), "the failure is spoken, never a silent turn: \(spoken(log))")
    }

    func testTheServerAnswerNeverArrivingThenCancelLeavesNoFailureCard() async throws {
        ScriptedStreamingDesk.hang = true
        let (e, _, _) = try engine(desk: "scripted-stream-desk", model: nil, corpus: corpus())
        _ = await e.typedTurn("what is the top story")
        for _ in 0..<100 { let p = await e.phase; if p == .holding || p == .acknowledging { break }; try await Task.sleep(nanoseconds: 50_000_000) }
        XCTAssertEqual(Conversation.shared.entries().last?.status, "asking", "the question is on screen while the answer is awaited")
        await e.cancelTurn()
        XCTAssertEqual(Conversation.shared.entries().last?.status, "cancelled", "no failure card: a cancelled entry shows none (bar.ts)")
        ScriptedStreamingDesk.hang = false
    }

    func testTwoQuestionsInQuickSuccession() async throws {
        ScriptedStreamingDesk.sentences = ["Answer."]; ScriptedStreamingDesk.hang = false
        let (e, _, got) = try engine(desk: "scripted-stream-desk", model: nil, corpus: corpus(), deskParams: ["delay": .number(0.3)])
        _ = await e.typedTurn("first question about the harbor")
        _ = await e.typedTurn("second question about the bank")
        for _ in 0..<200 where Conversation.shared.entries().last?.status != "answered" { try await Task.sleep(nanoseconds: 50_000_000) }
        _ = got
        let es = Conversation.shared.entries()
        XCTAssertEqual(es.count, 2)
        XCTAssertEqual(es[0].status, "cancelled"); XCTAssertEqual(es[1].status, "answered"); XCTAssertEqual(es[1].answer, "Answer.")
        XCTAssertEqual(es[1].question, "second question about the bank")
    }
}

// Playback order by voice. Failure list (written first): a question holding a command word is not a command; case, punctuation and
// politeness do not matter; "this" with no page in front is not a command; a command never reaches the desk; with no queue hook the
// words go to the desk as before; the request carries the queue and candidates; a queue_ops event is handed on unread and not spoken.
final class QueueVoiceTests: XCTestCase {
    override func setUp() { Conversation.shared.clear(); QueueVoice.hooks = QueueVoice.Hooks(); RecordingDesk.turns = [] }
    override func tearDown() { QueueVoice.hooks = QueueVoice.Hooks() }

    func engine(desk: String = "rec-desk") throws -> (TurnEngine, EventLog) {
        let r = StageRegistry.builtIn()
        r.register(.system1, StubS1.self) { StubS1($0) }
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        r.register(.transcriber, StubTranscriber.self) { StubTranscriber($0) }
        r.register(.desk, RecordingDesk.self) { RecordingDesk($0) }
        r.register(.desk, QueueOpsDesk.self) { QueueOpsDesk($0) }
        var cfg = StageRegistry.defaultConfiguration()
        cfg.set(.system1, .init(id: "stub-s1", params: ["delay": .number(0), "asks": .string("yes")]))
        cfg.set(.voice, .init(id: "stub-voice")); cfg.set(.transcriber, .init(id: "stub-tr")); cfg.set(.holder, .init(id: "local-ack"))
        cfg.set(.desk, .init(id: desk))
        let log = EventLog(clock: LabClock())
        let env = EngineEnvironment(registry: r, log: log, playback: SimulatedPlayback(position: ProgramPosition(programmeID: "p", blockIndex: 0, context: "ctx")),
                                    frontPage: { FrontPage(address: "pub:a", title: "A story", kind: "story") })
        return (TurnEngine(env: env, configuration: cfg), log)
    }

    func testEveryTurnGoesToTheDeskWithItsWords() async throws {
        let (e, _) = try engine()
        _ = await e.typedTurn("skip")
        for _ in 0..<60 where RecordingDesk.turns.isEmpty { try await Task.sleep(nanoseconds: 50_000_000) }
        XCTAssertEqual(RecordingDesk.turns.first?.listenerWords, "skip", "no phrase is handled on the phone: the desk answers and acts through the run sheet's ops")
    }

    func testTheRequestBodyCarriesTheQueueAndCandidates() throws {
        var t = DeskTurn(turnID: "t1", programmeID: "", blockIndex: 0, listenerWords: "play the long read next")
        t.queue = Data(#"{"version":7,"current":null,"upcoming":[{"id":"q13","item":{"kind":"story","route":"/a","title":"A"},"added_by":"listener"}]}"#.utf8)
        t.candidates = [DeskCandidate(route: "/r1", title: "One"), DeskCandidate(route: "", title: "dropped")]
        let b = AutoradioDeskStage.body(t, id: "t1", history: [])
        XCTAssertEqual((b["queue"] as? [String: Any])?["version"] as? Int, 7)
        XCTAssertEqual((b["candidates"] as? [[String: String]])?.count, 1)
        let plain = AutoradioDeskStage.body(DeskTurn(turnID: "t2", programmeID: "", blockIndex: 0, listenerWords: "hi"), id: "t2", history: [])
        XCTAssertNil(plain["queue"]); XCTAssertNil(plain["candidates"])
    }

    func testAQueueOpsEventIsHandedOnUnreadAndNothingIsSpokenForIt() async throws {
        let got = Locked<[Data]>([])
        var h = QueueVoice.Hooks(); h.applyServerOps = { d in got.write { $0.append(d) } }
        QueueVoice.hooks = h
        let (e, log) = try engine(desk: "queue-ops-desk")
        _ = await e.typedTurn("play the long read next")
        for _ in 0..<80 where got.value.isEmpty { try await Task.sleep(nanoseconds: 50_000_000) }
        XCTAssertEqual(String(decoding: got.value.first ?? Data(), as: UTF8.self), QueueOpsDesk.json)
        await e.waitUntilIdle()
        let said = log.events.compactMap { ev -> String? in if case .deskSentenceStarted(let t, _) = ev.event { return t } else { return nil } }
        XCTAssertEqual(said, ["Okay, the long read is next."], "only the server's own line is spoken")
    }
    func testASseQueueOpsEventBecomesADeskEvent() async throws {
        var parser = SSEParser()
        XCTAssertNil(parser.push("event: queue_ops"))
        let ev = parser.push("data: " + QueueOpsDesk.json)
        XCTAssertEqual(ev?.name, "queue_ops")
    }
}

final class QueueOpsDesk: StreamingDesk, @unchecked Sendable {
    static let stageID = "queue-ops-desk"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "queue ops desk") }
    static let json = #"{"turn_id":"t","base_version":7,"replayed":false,"ops":[{"op":"play_next","item":{"kind":"story","route":"/a","title":"A"},"base_version":7,"reason":"you asked"}]}"#
    init(_ p: StageParams) {}
    func submit(_ turn: DeskTurn) -> AsyncThrowingStream<Revision, Error> { AsyncThrowingStream { $0.finish() } }
    func speech(_ turn: DeskTurn, heard: HeardLines) -> AsyncThrowingStream<DeskEvent, Error> {
        AsyncThrowingStream { c in
            c.yield(.sentence(seq: 0, text: "Okay, the long read is next."))
            c.yield(.queueOps(json: Data(Self.json.utf8)))
            c.yield(.done(status: "completed", sentences: 1, serverFirstSentenceMs: nil, serverDoneMs: nil))
            c.finish()
        }
    }
}
