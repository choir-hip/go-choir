import XCTest
@testable import AutoradioKit

// The failure list is at the top of LocalAck.swift and LocalSpeaker.swift (numbers below refer to them); the trace's list is at the
// top of TurnTimeline.swift. No test here asserts a wall-clock cutoff: the code has none. Waits in tests are the tests' own.

/// A language model that is not one. It streams `pieces` (each after `pieceDelay` seconds), or, with none, completes `reply` after
/// `delay`. It records what it was asked (the stable prefix and the suffix separately), whether its stream was cancelled, how often it was
/// announced and ended, and what was read ahead (`prefill`): a generation whose prefix was read ahead reports a warm cache.
final class FakeLocalModel: LocalLanguageModel, @unchecked Sendable {
    let reply: String, delay: Double, loaded: Bool, fails: Bool, pieces: [String]?, pieceDelay: Double
    private let lock = NSLock()
    private(set) var asked: [(system: String, user: String, prefix: String, suffix: String)] = []
    private(set) var will = 0, ended = 0, streamsCancelled = 0, piecesSent = 0
    private(set) var prefilled: [(system: String, prefix: String)] = []
    private var report: GenerationReport?
    func willGenerate() { lock.lock(); will += 1; lock.unlock() }
    func generationEnded() { lock.lock(); ended += 1; lock.unlock() }
    init(pieces: [String]? = nil, pieceDelay: Double = 0, reply: String = "", delay: Double = 0, loaded: Bool = true, fails: Bool = false) {
        self.reply = reply; self.delay = delay; self.loaded = loaded; self.fails = fails; self.pieces = pieces; self.pieceDelay = pieceDelay
    }
    var isLoaded: Bool { loaded }
    func prefill(system: String, userPrefix: String) async { lock.lock(); prefilled.append((system, userPrefix)); lock.unlock() }
    func lastReport() -> GenerationReport? { lock.lock(); defer { lock.unlock() }; return report }
    func generate(system: String, user: String, temperature: Double) async throws -> String {
        lock.lock(); asked.append((system, user, "", user)); lock.unlock()
        if delay > 0 { try await Task.sleep(nanoseconds: UInt64(delay * 1e9)) }
        if fails { throw LabError.unavailable("fake failure") }
        return reply
    }
    func stream(system: String, userPrefix: String, userSuffix: String, temperature: Double) -> AsyncThrowingStream<String, Error> {
        lock.lock()
        asked.append((system, userPrefix + userSuffix, userPrefix, userSuffix))
        // Words are about 4 characters here; a token about 4 characters. The prefix counts as cached when it was read ahead.
        let prefixTokens = (system.count + userPrefix.count) / 4, total = prefixTokens + userSuffix.count / 4
        let warm = prefilled.contains { $0.system == system && $0.prefix == userPrefix }
        report = GenerationReport(promptTokens: total, prefixTokens: prefixTokens, reusedTokens: warm ? prefixTokens : 0, prefillMs: warm ? 8 : 900)
        lock.unlock()
        return AsyncThrowingStream { c in
            let t = Task { [self] in
                if delay > 0 { try? await Task.sleep(nanoseconds: UInt64(delay * 1e9)) }
                if fails { c.finish(throwing: LabError.unavailable("fake failure")); return }
                for p in pieces ?? [reply] {
                    if Task.isCancelled { return }
                    if pieceDelay > 0 { try? await Task.sleep(nanoseconds: UInt64(pieceDelay * 1e9)) }
                    if Task.isCancelled { return }
                    lock.lock(); piecesSent += 1; lock.unlock()
                    c.yield(p)
                }
                c.finish()
            }
            c.onTermination = { [self] reason in
                if case .cancelled = reason { lock.lock(); streamsCancelled += 1; lock.unlock() }
                t.cancel()
            }
        }
    }
}

/// Records what the engine hands the holder, and says one line.
final class CapturingHolder: Holder, @unchecked Sendable {
    static let stageID = "capturing"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "capturing") }
    nonisolated(unsafe) static var seen: [HolderInput] = []
    init(_ p: StageParams) {}
    func lines(for input: HolderInput) -> AsyncStream<String> {
        Self.seen.append(input)
        return AsyncStream { $0.yield("On it."); $0.finish() }
    }
}

/// A desk that answers after a delay and remembers what the phone had said aloud when it was asked and when it answered.
final class HeardDesk: Desk, @unchecked Sendable {
    static let stageID = "heard-desk"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "heard desk", params: [.number("delay", "d", 1.5, 0, 10)]) }
    nonisolated(unsafe) static var heardAtAnswer: [String] = []
    nonisolated(unsafe) static var box: HeardLines?
    let delay: Double
    init(_ p: StageParams) { delay = p.double("delay", 1.5) }
    func submit(_ t: DeskTurn) -> AsyncThrowingStream<Revision, Error> { submit(t, heard: HeardLines()) }
    func submit(_ t: DeskTurn, heard: HeardLines) -> AsyncThrowingStream<Revision, Error> {
        Self.box = heard
        let d = delay
        return AsyncThrowingStream { c in
            let task = Task {
                try? await Task.sleep(nanoseconds: UInt64(d * 1e9))
                Self.heardAtAnswer = heard.snapshot
                c.yield(Revision(id: "h", fromBlock: t.blockIndex + 1, blocks: [.speech(text: "The desk answers.")]))
                c.finish()
            }
            c.onTermination = { _ in task.cancel() }
        }
    }
}

final class LocalAckTests: XCTestCase {
    override func setUp() { AckHistory.shared.clear(); LocalLanguage.model = nil }
    override func tearDown() { AckHistory.shared.clear(); LocalLanguage.model = nil }

    func collect(_ s: AsyncStream<String>) async -> [String] { var out: [String] = []; for await l in s { out.append(l) }; return out }
    func holder(_ model: LocalLanguageModel?, history: AckHistory = AckHistory()) -> LocalAckHolder {
        LocalAckHolder(StageParams(), history: history, model: { model })
    }
    func input(_ words: String = "what did the council decide about the harbor park", title: String = "Harbor park opens", kind: String = "texture",
               pageText: (@Sendable () async -> String)? = nil) -> HolderInput {
        HolderInput(listenerWords: words, answers: [], pageTitle: title, pageKind: kind, pageText: pageText)
    }

    // MARK: the prompt builder: front-loaded, standalone sentences, truthful (failures 5, 8)

    func testSystemPromptSteersEverythingBecauseNothingFiltersTheReply() {
        let s = AckPrompt.system()
        XCTAssertTrue(s.contains("the radio's desk is now working on the answer"), "what the local voice is for")
        XCTAssertTrue(s.contains("so the listener is not left in silence"))
        XCTAssertTrue(s.contains("The most useful thing comes first"), "front-loaded")
        XCTAssertTrue(s.contains("interrupted after any sentence"))
        XCTAssertTrue(s.contains("show that you understood what they asked, in their own terms"))
        XCTAssertTrue(s.contains("page text"), "sentence one may use what the page says")
        XCTAssertTrue(s.contains("Each later sentence adds something less important"))
        XCTAssertTrue(s.contains("make sense on its own"), "any sentence may be the last one heard")
        XCTAssertTrue(s.contains("Say only what the page in front and the listener's words support"))
        XCTAssertTrue(s.contains("If the page has nothing about what they asked, say so plainly in one sentence instead of guessing or inventing"))
        XCTAssertTrue(s.contains("never present a guess as the answer"))
        XCTAssertTrue(s.contains("never claim the desk has found something"))
        XCTAssertTrue(s.contains("Do not repeat what you have just said"))
        XCTAssertTrue(s.contains("nothing more that is true and useful to add, stop"), "it ends by itself")
        XCTAssertTrue(s.contains("spoken aloud"), "words a voice can say")
        for rule in ["twelve", "12 words", "words at most", "Yes or No", "no more than"] { XCTAssertFalse(s.contains(rule), "no numeric or pattern rule in the prompt: \(rule)") }
    }

    func testRecentLinesAreCarriedAsPromptContextInTheSuffixNotThePrefix() {
        let suffix = AckPrompt.userSuffix(listener: "what next", previous: ["Checking on that harbor vote.", "Let me find that."])
        XCTAssertTrue(suffix.contains("Checking on that harbor vote."))
        XCTAssertTrue(suffix.contains("Let me find that."))
        XCTAssertTrue(suffix.contains("already said and should not say again"))
        XCTAssertFalse(AckPrompt.userSuffix(listener: "x", previous: []).contains("Lines you said recently"))
        XCTAssertFalse(AckPrompt.system().contains("Lines you said recently"), "history never enters the fixed instructions")
    }

    func testThePrefixIsByteStableAndTheSuffixIsWhatChanges() {
        let a = AckPrompt.userPrefix(pageTitle: "Harbor park opens", pageKind: "texture", pageText: "The park opened.\nIt is big.")
        let b = AckPrompt.userPrefix(pageTitle: "Harbor park opens", pageKind: "texture", pageText: "The park opened.\nIt is big.")
        XCTAssertEqual(a, b, "the same page gives the same bytes, so the model's cache can be reused")
        XCTAssertEqual(AckPrompt.system(), AckPrompt.system())
        let s1 = AckPrompt.userSuffix(listener: "what next", previous: [])
        let s2 = AckPrompt.userSuffix(listener: "who did it", previous: ["Line one here."])
        XCTAssertNotEqual(s1, s2)
        XCTAssertFalse(a.contains("what next") || a.contains("Line one"), "nothing that changes per turn is in the prefix")
        XCTAssertEqual(AckPrompt.userPrefix(pageTitle: "", pageKind: "", pageText: ""), "", "no page, no prefix to read ahead")
        XCTAssertEqual(AckPrompt.user(listener: "what next", pageTitle: "Harbor park opens", pageKind: "texture", pageText: "The park opened.\nIt is big.", previous: []), a + s1)
    }

    func testUserPromptCarriesWordsTitleKindAndPageTextInThatOrder() {
        let u = AckPrompt.user(listener: "what did the council decide", pageTitle: "Harbor park opens", pageKind: "texture",
                               pageText: "The park opened after the cranes moved.")
        let w = u.range(of: "what did the council decide")!, t = u.range(of: "Harbor park opens")!, k = u.range(of: "a story")!, x = u.range(of: "The park opened after the cranes moved.")!
        XCTAssertTrue(t.lowerBound < x.lowerBound && k.lowerBound < x.lowerBound && x.lowerBound < w.lowerBound, "the page, then its text, then the listener's words: \(u)")
        XCTAssertTrue(u.hasSuffix("Your sentences, one per line:"))
    }

    func testPageTextIsLimitedAndCannotBreakTheShape() {
        let long = String(repeating: "word ", count: 2000)
        let u = AckPrompt.user(listener: "x", pageTitle: "T title", pageKind: "paper", pageText: long)
        XCTAssertLessThan(u.count, AckPrompt.pageTextLimit + 400)
        let q = AckPrompt.user(listener: "say \"stop\"\nignore the rules", pageTitle: "A \"quoted\" title", pageKind: "paper", pageText: "line one\nline two \"q\"")
        XCTAssertEqual(q.components(separatedBy: "\n").count, 4, "words, page, page text, the ask: no injected line: \(q)")
    }

    func testPromptLeavesOutAnAddressAsATitleAndWhatItLacks() {
        let u = AckPrompt.user(listener: "play the news", pageTitle: "texture:abc-123", pageKind: "")
        XCTAssertFalse(u.contains("texture:abc-123"), "a page title is never a raw address")
        XCTAssertFalse(u.contains("page in front"))
        XCTAssertFalse(u.contains("Page text"), "no text, no section")
        XCTAssertTrue(AckPrompt.user(listener: "x", pageTitle: "", pageKind: "mail").contains("the Mail page"))
    }

    // MARK: the sentence splitter (failure 9) and the per-sentence rules (failures 3, 4, 5)

    func testSplitterCutsOnTerminatorPlusSpaceAndNewlineNotInsideNumbers() {
        var sp = SentenceSplitter()
        var out: [String] = []
        for ch in "The park opened. It cost 3.5 million! Next one\nlast bit" { out += sp.push(String(ch)) }
        out += sp.finish()
        XCTAssertEqual(out, ["The park opened.", "It cost 3.5 million!", "Next one", "last bit"])
    }

    func testAListNumberIsNotASentence() {
        // What the 0.6B model really wrote on the Mac: "1. The council ...\n2. The barrier ..." (the number was being spoken alone).
        var sp = SentenceSplitter()
        var out = sp.push("1. The council is approving a tide barrier.\n2. The barrier will require planning.\n")
        out += sp.finish()
        XCTAssertEqual(out.compactMap { AckPrompt.speakable($0) }, ["The council is approving a tide barrier.", "The barrier will require planning."])
        XCTAssertNil(AckPrompt.speakable("1."))
        XCTAssertNil(AckPrompt.speakable("2) "))
        XCTAssertNil(AckPrompt.speakable("..."))
        XCTAssertEqual(AckPrompt.firstSentence("1. The council is approving a tide barrier. More."), "The council is approving a tide barrier.")
    }

    func testSplitterDoesNotCutAtAnEndOfBufferTerminatorUntilWhitespaceArrives() {
        var sp = SentenceSplitter()
        XCTAssertEqual(sp.push("It is 3."), [])
        XCTAssertEqual(sp.push("5 percent. Done"), ["It is 3.5 percent."])
        XCTAssertEqual(sp.finish(), ["Done"])
    }

    func testSanitisingStripsMarkupAndNothingElse() {
        XCTAssertEqual(AckPrompt.speakable("The council approved a tide barrier across the east channel."), "The council approved a tide barrier across the east channel.")
        XCTAssertEqual(AckPrompt.speakable("- It will close when water rises."), "It will close when water rises.")
        XCTAssertEqual(AckPrompt.speakable("1. Construction starts next spring."), "Construction starts next spring.")
        XCTAssertEqual(AckPrompt.speakable("**Important:** the vote was 7 to 2."), "Important: the vote was 7 to 2.")
        XCTAssertEqual(AckPrompt.speakable("<think>\n\n</think>\n\nChecking the plan."), "Checking the plan.")
        XCTAssertEqual(AckPrompt.speakable("\"Quoted line.\""), "Quoted line.")
        XCTAssertNil(AckPrompt.speakable("   \n "))
        XCTAssertNil(AckPrompt.speakable("<think>\n\n</think>"))
    }

    func testNothingIsJudgedOnMeaning() {
        // The old filters are gone: these all pass through to the voice (a model that says them is steered by the prompt, or is the
        // wrong model; that is a finding, not a rule).
        for line in ["Okay", "Yes, the park opens on Monday.", "The answer is the harbor.", "The work starts in 2027 and costs 40 million.",
                     Array(repeating: "word", count: 60).joined(separator: " ") + "."] {
            XCTAssertEqual(AckPrompt.speakable(line), line)
        }
    }

    func testFirstSentenceOfAWholeCompletion() {
        XCTAssertEqual(AckPrompt.firstSentence("<think>\n\n</think>\n\nChecking the council's plan. Then more."), "Checking the council's plan.")
        XCTAssertNil(AckPrompt.firstSentence("<think>\n\n</think>"))
    }

    func testHistoryKeepsTheLastFew() {
        let h = AckHistory(keep: 3)
        for i in 1...5 { h.add("line \(i)") }
        XCTAssertEqual(h.recent, ["line 3", "line 4", "line 5"])
    }

    // MARK: the holder, with the model faked (failures 1, 2, 3, 6, 7)

    func testTheModelStreamsSentencesInOrderFromTheWordsThePageAndItsText() async {
        let m = FakeLocalModel(pieces: ["The council approved a tide ", "barrier on Tuesday. It will close ", "when water rises. Work starts next spring."])
        let h = holder(m)
        let lines = await collect(h.lines(for: input(pageText: { "The harbor council voted to approve a tide barrier." })))
        XCTAssertEqual(lines, ["The council approved a tide barrier on Tuesday.", "It will close when water rises.", "Work starts next spring."])
        XCTAssertEqual(m.asked.count, 1, "one call asks for the whole stream")
        XCTAssertTrue(m.asked[0].user.contains("harbor park"))
        XCTAssertTrue(m.asked[0].user.contains("Harbor park opens"))
        XCTAssertTrue(m.asked[0].user.contains("The harbor council voted to approve a tide barrier."), "the page text reached the model")
        XCTAssertEqual(m.will, 1); XCTAssertEqual(m.ended, 1)
        XCTAssertTrue(h.diagnostics().contains("3 sentences"), h.diagnostics())
        XCTAssertTrue(h.diagnostics().contains("nothing more"), "it ended by itself, and said so")
    }

    func testItStopsByItselfWhenTheModelEndsItsStream() async {
        let m = FakeLocalModel(pieces: ["Looking at the harbor vote now."])
        let lines = await collect(holder(m).lines(for: input()))
        XCTAssertEqual(lines.count, 1, "no padding after the model has nothing more")
    }

    func testRepeatsAreNotFilteredTheModelIsToldWhatWasSaid() async {
        let history = AckHistory()
        let m = FakeLocalModel(pieces: ["Looking into the harbor park vote. Looking into the harbor park vote. Checking who proposed it."])
        let first = await collect(holder(m, history: history).lines(for: input()))
        XCTAssertEqual(first.count, 3, "no filter: what the model wrote is what is spoken")
        let second = await collect(holder(m, history: history).lines(for: input()))
        XCTAssertEqual(second.count, 3)
        XCTAssertFalse(m.asked[0].user.contains("Lines you said recently"), "the first turn had no history")
        XCTAssertTrue(m.asked[1].user.contains("Looking into the harbor park vote."), "the second ask carries what was said before, in the prompt")
        XCTAssertEqual(m.asked[0].prefix, m.asked[1].prefix, "and the stable prefix did not change")
        XCTAssertTrue(m.asked[1].user.contains("Checking who proposed it."))
    }

    func testOnlyLinesSaidAboutTheSamePageGoIntoThePrompt() async {
        let history = AckHistory()
        let m = FakeLocalModel(pieces: ["The strike ended in Nancy today."])
        _ = await collect(holder(m, history: history).lines(for: HolderInput(listenerWords: "what happened", answers: [], pageTitle: "Front", pageKind: "paper", pageAddress: "pub:front")))
        let m2 = FakeLocalModel(pieces: ["The battery stores solar power."])
        _ = await collect(holder(m2, history: history).lines(for: HolderInput(listenerWords: "tell me more", answers: [], pageTitle: "Grid", pageKind: "texture", pageAddress: "texture:grid")))
        XCTAssertFalse(m2.asked[0].user.contains("The strike ended in Nancy today."), "another page's line is not context for this page")
        let m3 = FakeLocalModel(pieces: ["Something else about the front page."])
        _ = await collect(holder(m3, history: history).lines(for: HolderInput(listenerWords: "and then", answers: [], pageTitle: "Front", pageKind: "paper", pageAddress: "pub:front")))
        XCTAssertTrue(m3.asked[0].user.contains("The strike ended in Nancy today."), "the same page's earlier line is")
        XCTAssertFalse(m3.asked[0].user.contains("The battery stores solar power."))
    }

    func testNoModelOrNotLoadedMeansNoWordsNotAStockPhrase() async {
        let h1 = holder(nil)
        let a = await collect(h1.lines(for: input()))
        XCTAssertEqual(a, [])
        XCTAssertTrue(h1.diagnostics().contains("not loaded"))
        let m = FakeLocalModel(pieces: ["Looking into that."], loaded: false)
        let b = await collect(holder(m).lines(for: input()))
        XCTAssertEqual(b, [])
        XCTAssertEqual(m.asked.count, 0, "an unloaded model is never asked, and never waited for")
        XCTAssertEqual(m.will, 0)
    }

    func testASlowModelIsWaitedForNotCutOff() async {
        let m = FakeLocalModel(pieces: ["The first thing is ready now. ", "The second comes later."], pieceDelay: 0.4)
        let t0 = Date()
        let lines = await collect(holder(m).lines(for: input()))
        XCTAssertEqual(lines.count, 2)
        XCTAssertGreaterThan(Date().timeIntervalSince(t0), 0.7, "nothing cut the slow stream short")
        XCTAssertEqual(m.will, m.ended)
    }

    func testAFailingModelAndEmptyWordsSaySomethingReadable() async {
        let h = holder(FakeLocalModel(fails: true))
        let a = await collect(h.lines(for: input()))
        XCTAssertEqual(a, [])
        XCTAssertTrue(h.diagnostics().contains("failed"), h.diagnostics())
        let m = FakeLocalModel(pieces: ["Looking into that."])
        let b = await collect(holder(m).lines(for: input("   ")))
        XCTAssertEqual(b, [])
        XCTAssertEqual(m.asked.count, 0, "no words, no call")
    }

    func testCancellingStopsTheGenerationAndNothingMoreIsWritten() async {
        let m = FakeLocalModel(pieces: ["First sentence is here now. ", "Second sentence is here now. ", "Third sentence is here now."], pieceDelay: 0.3)
        let h = holder(m)
        let task = Task { await collect(h.lines(for: input())) }
        try? await Task.sleep(nanoseconds: 450_000_000)
        h.cancel()
        let lines = await task.value
        XCTAssertLessThan(lines.count, 3)
        for _ in 0..<40 where m.will != m.ended { try? await Task.sleep(nanoseconds: 50_000_000) }
        XCTAssertEqual(m.will, m.ended, "a cancelled generation is still ended")
        XCTAssertLessThan(m.piecesSent, 3, "the model stopped producing")
    }

    func testNoStockPhraseListSurvivesInTheSources() throws {
        let dir = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
            .appendingPathComponent("Sources")
        let e = try XCTUnwrap(FileManager.default.enumerator(at: dir, includingPropertiesForKeys: nil))
        for case let u as URL in e where u.pathExtension == "swift" && !u.lastPathComponent.contains("generated") {
            let text = try String(contentsOf: u, encoding: .utf8)
            for phrase in ["I am pulling that together", "Still working on that", "Thanks for waiting", "Let me look into that"] {
                XCTAssertFalse(text.contains(phrase), "\(u.lastPathComponent) still holds the stock phrase \"\(phrase)\"")
            }
        }
    }

    // MARK: the non-verbal sounds

    func testTheCuesAreShortSoftAndFinite() {
        for f in [NonVerbalCue.frames(), NonVerbalCue.handover()] {
            let samples = f.flatMap(\.samples)
            XCTAssertFalse(samples.isEmpty)
            XCTAssertLessThan(Double(samples.count) / (f.first?.sampleRate ?? 24_000), 0.4)
            XCTAssertTrue(samples.allSatisfy { $0.isFinite && abs($0) <= 0.2 })
            XCTAssertGreaterThan(samples.map(abs).max() ?? 0, 0.05, "audible")
        }
        XCTAssertNotEqual(NonVerbalCue.frames().flatMap(\.samples), NonVerbalCue.handover().flatMap(\.samples), "the hand-over sounds different from the cue")
    }

    // MARK: the engine (failures 1, 2 end to end; the page reaches the holder; the desk is told what was heard)

    final class Box: @unchecked Sendable { var value: TurnRecord?; func set(_ r: TurnRecord) { value = r } }

    func runPipeline(model: LocalLanguageModel?, holderID: String = "local-ack", front: FrontPage? = nil, envFront: FrontPage? = nil,
                     deskDelay: Double = 0.4, deskID: String = "stub-echo", pageText: String = "") async throws -> (TurnRecord, EventLog) {
        LocalLanguage.model = model
        let r = StageRegistry.builtIn()
        r.register(.system1, StubS1.self) { StubS1($0) }
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        r.register(.transcriber, StubTranscriber.self) { StubTranscriber($0) }
        r.register(.holder, CapturingHolder.self) { CapturingHolder($0) }
        r.register(.desk, HeardDesk.self) { HeardDesk($0) }
        var cfg = StageRegistry.defaultConfiguration()
        cfg.set(.system1, .init(id: "stub-s1", params: ["delay": .number(0.05), "asks": .string("yes")]))
        cfg.set(.voice, .init(id: "stub-voice")); cfg.set(.transcriber, .init(id: "stub-tr"))
        cfg.set(.holder, .init(id: holderID))
        cfg.set(.desk, .init(id: deskID, params: deskID == "stub-echo" ? ["delaySeconds": .number(deskDelay)] : ["delay": .number(deskDelay)]))
        let log = EventLog(clock: LabClock())
        let pb = SimulatedPlayback(position: ProgramPosition(programmeID: "p", blockIndex: 0, context: "ctx"))
        let got = Box()
        let env = EngineEnvironment(registry: r, log: log, playback: pb, onRecord: { got.set($0) }, frontPage: { envFront }, pageText: { _ in pageText })
        let engine = TurnEngine(env: env, configuration: cfg)
        let t = TurnEngine.Turn(id: "t1", config: cfg, transcriber: try r.make(.transcriber, as: Transcriber.self, cfg),
                                yield: try r.make(.yieldPolicy, as: YieldPolicy.self, cfg), session: EmptyTranscription(), firstEventIndex: 0)
        t.front = front
        log.emit(.turnEnded); log.emit(.finalTranscript("tell me about the harbor park"))
        await engine.runPipeline(t, transcript: "tell me about the harbor park", endedAt: log.clock.now)
        return (try XCTUnwrap(got.value), log)
    }

    func testEngineSpeaksTheStreamSentenceBySentenceAndLogsEachOne() async throws {
        let m = FakeLocalModel(pieces: ["The council approved a barrier. ", "It closes when water rises."])
        let (rec, log) = try await runPipeline(model: m, envFront: FrontPage(address: "texture:a", title: "Harbor park opens", kind: "texture"), deskDelay: 1.5, pageText: "The council approved a barrier.")
        XCTAssertEqual(rec.summary.holdingLines, ["The council approved a barrier.", "It closes when water rises."])
        XCTAssertTrue(m.asked[0].user.contains("The council approved a barrier."), "the page text reached the model through the engine")
        let tl = try XCTUnwrap(TurnTimeline.compute(log.events).first)
        XCTAssertEqual(tl.localSentences.map(\.text), ["The council approved a barrier.", "It closes when water rises."])
        XCTAssertNil(tl.localCutMs, "the model ended its stream by itself; nothing was cut")
        XCTAssertNotNil(tl.transitionMs, "the hand-over sound played before the desk's answer")
        XCTAssertTrue(tl.cueUsed)
        XCTAssertEqual(tl.withinLocalBudget, true)
        XCTAssertNotNil(rec.summary.deskFirstTokenMs)
    }

    func testTheCuePlaysAtOnceWhileTheModelIsStillThinking() async throws {
        let (rec, _) = try await runPipeline(model: FakeLocalModel(pieces: ["The first sentence is ready."], delay: 0.6), deskDelay: 2.0)
        XCTAssertLessThan(rec.summary.turnEndToAckMs ?? 9999, 100, "the cue came at once, before the model wrote anything")
        XCTAssertEqual(rec.summary.holdingLines, ["The first sentence is ready."], "and the slow model was not cut off")
        XCTAssertGreaterThan(rec.summary.localReplyReadyMs ?? 0, 500)
    }

    func testDeskSpeechCutsTheLocalStreamAtASentenceBoundaryAndCancelsGeneration() async throws {
        let m = FakeLocalModel(pieces: ["Sentence one is about the harbor. ", "Sentence two adds some context. ", "Sentence three adds more context. ",
                                        "Sentence four adds even more. ", "Sentence five goes on and on. ", "Sentence six keeps going."], pieceDelay: 0.35)
        let (rec, log) = try await runPipeline(model: m, deskDelay: 0.9)
        let tl = try XCTUnwrap(TurnTimeline.compute(log.events).first)
        XCTAssertNotNil(tl.localCutMs, "the desk's speech cut the stream")
        XCTAssertEqual(tl.localCutReason, "the desk's speech was ready")
        XCTAssertGreaterThanOrEqual(tl.localSentences.count, 1)
        XCTAssertLessThan(tl.localSentences.count, 6)
        XCTAssertEqual(tl.localCutAfterSentences, tl.localSentences.count)
        // No sentence starts to be heard after the cut.
        let cutIdx = try XCTUnwrap(log.events.firstIndex { if case .localCut = $0.event { return true } else { return false } })
        XCTAssertFalse(log.events[cutIdx...].contains { if case .localSentenceStarted = $0.event { return true } else { return false } })
        XCTAssertNotNil(tl.transitionMs)
        for _ in 0..<60 where m.will != m.ended { try await Task.sleep(nanoseconds: 50_000_000) }
        XCTAssertEqual(m.will, m.ended)
        XCTAssertLessThan(m.piecesSent, 6, "generation was cancelled, it did not run to the end")
        XCTAssertLessThan(rec.summary.holdingLines.count, 6)
    }

    func testTheDeskIsToldWhatWasActuallySaidAloud() async throws {
        HeardDesk.heardAtAnswer = []; HeardDesk.box = nil
        let m = FakeLocalModel(pieces: ["The council approved a barrier. ", "It closes when water rises."])
        _ = try await runPipeline(model: m, deskDelay: 1.5, deskID: "heard-desk")
        XCTAssertNotNil(HeardDesk.box, "the engine handed the desk the heard-lines box")
        XCTAssertEqual(HeardDesk.heardAtAnswer, ["The council approved a barrier.", "It closes when water rises."])
    }

    func testEnginePlaysOnlyTheCueWhenTheModelIsNotLoaded() async throws {
        let (rec, log) = try await runPipeline(model: nil)
        XCTAssertEqual(rec.summary.holdingLines, [], "no stock phrase stands in")
        XCTAssertNotNil(rec.summary.localAckSkipped)
        let tl = try XCTUnwrap(TurnTimeline.compute(log.events).first)
        XCTAssertTrue(tl.cueUsed)
        XCTAssertTrue(tl.localSentences.isEmpty)
        XCTAssertNil(tl.transitionMs, "nothing was said, so no hand-over")
        XCTAssertEqual(tl.withinLocalBudget, true)
    }

    func testThePageInFrontReachesTheHolder() async throws {
        CapturingHolder.seen = []
        _ = try await runPipeline(model: nil, holderID: "capturing", envFront: FrontPage(address: "paper:front", title: "Today's paper", kind: "paper"))
        XCTAssertEqual(CapturingHolder.seen.last?.pageTitle, "Today's paper")
        XCTAssertEqual(CapturingHolder.seen.last?.pageKind, "paper")
        CapturingHolder.seen = []
        _ = try await runPipeline(model: nil, holderID: "capturing", front: FrontPage(address: "texture:a", title: "Harbor park opens", kind: "texture"),
                                  envFront: FrontPage(address: "paper:front", title: "Today's paper", kind: "paper"))
        XCTAssertEqual(CapturingHolder.seen.last?.pageTitle, "Harbor park opens", "the page the bar named wins over the one in front now")
    }

    func testAWarmCacheIsLoggedInTheTimelineAndAColdOneToo() async throws {
        let prefix = AckPrompt.userPrefix(pageTitle: "Harbor park opens", pageKind: "texture", pageText: "The harbor council voted to approve a tide barrier.")
        let warmModel = FakeLocalModel(pieces: ["The council approved a barrier."])
        await warmModel.prefill(system: AckPrompt.system(), userPrefix: prefix)          // the page was read ahead
        let (_, warmLog) = try await runPipeline(model: warmModel, front: FrontPage(address: "texture:a", title: "Harbor park opens", kind: "texture"),
                                                 pageText: "The harbor council voted to approve a tide barrier.")
        let warm = try XCTUnwrap(TurnTimeline.compute(warmLog.events).first)
        XCTAssertEqual(warm.cacheWarm, true)
        XCTAssertEqual(warm.prefillMs, 8)
        XCTAssertEqual(warm.reusedTokens, warm.promptTokens.map { _ in warmModel.lastReport()!.prefixTokens })
        let (_, coldLog) = try await runPipeline(model: FakeLocalModel(pieces: ["The council approved a barrier."]),
                                                 front: FrontPage(address: "texture:a", title: "Harbor park opens", kind: "texture"), pageText: "The harbor council voted to approve a tide barrier.")
        let cold = try XCTUnwrap(TurnTimeline.compute(coldLog.events).first)
        XCTAssertEqual(cold.cacheWarm, false)
        XCTAssertEqual(cold.reusedTokens, 0)
        XCTAssertTrue(warm.lines.contains { $0.contains("WARM page cache") }); XCTAssertTrue(cold.lines.contains { $0.contains("cold page cache") })
    }

    func testTheTurnsPrefixIsExactlyWhatWasReadAhead() async throws {
        let m = FakeLocalModel(pieces: ["The council approved a barrier."])
        let page = "The harbor council voted to approve a tide barrier."
        let readAhead = AckPrompt.userPrefix(pageTitle: "Harbor park opens", pageKind: "texture", pageText: page)
        _ = try await runPipeline(model: m, front: FrontPage(address: "texture:a", title: "Harbor park opens", kind: "texture"), pageText: page)
        XCTAssertEqual(m.asked.first?.prefix, readAhead, "the prefix the turn asks with is byte-for-byte what warmResponder reads ahead")
    }

    // MARK: the timeline from events (trace failures 1 to 4)

    func ev(_ t: Double, _ e: LabEvent) -> TimedEvent { TimedEvent(t: t, event: e) }

    func testTimelineSplitsTurnsAndReadsTheStreamTheCutAndTheTransition() {
        let events = [
            ev(1.0, .speechEnded(seconds: 1.2)), ev(1.3, .turnEnded), ev(1.31, .finalTranscript("first question")),
            ev(1.32, .cueStarted(msSinceTurnEnd: 20)), ev(1.32, .ackStarted(msSinceTurnEnd: 20)),
            ev(1.5, .localAckReady(text: "Looking.", msSinceTurnEnd: 200)), ev(1.55, .localSentenceStarted(text: "Looking at it.", msSinceTurnEnd: 250)),
            ev(1.9, .localSentenceStarted(text: "The page says the park opened.", msSinceTurnEnd: 600)),
            ev(2.0, .deskFirstToken(msSinceTurnEnd: 700)), ev(2.1, .revisionArrived(id: "r", fromBlock: 1, blocks: 1, summary: "s")),
            ev(2.4, .localCut(afterSentences: 2, reason: "the desk's speech was ready", msSinceTurnEnd: 1100)), ev(2.45, .transitionPlayed(msSinceTurnEnd: 1150)),
            ev(2.9, .programmeAudioStart(block: 1)),
            ev(5.0, .turnEnded), ev(5.01, .finalTranscript("typed question")),
            ev(5.3, .localAckSkipped(reason: "the local model is not loaded", msSinceTurnEnd: 290)), ev(5.3, .cueStarted(msSinceTurnEnd: 290)), ev(5.3, .ackStarted(msSinceTurnEnd: 290)),
        ]
        let t = TurnTimeline.compute(events)
        XCTAssertEqual(t.count, 2)
        XCTAssertEqual(t[0].transcript, "first question")
        XCTAssertEqual(t[0].speechEndedAt, 1.0)
        XCTAssertEqual(t[0].localAudioStartMs, 20)
        XCTAssertEqual(t[0].localSentences.map(\.ms), [250, 600])
        XCTAssertEqual(t[0].localCutMs, 1100); XCTAssertEqual(t[0].localCutAfterSentences, 2)
        XCTAssertEqual(t[0].transitionMs, 1150)
        XCTAssertEqual(t[0].deskFirstTokenMs, 700)
        XCTAssertEqual(try XCTUnwrap(t[0].deskAudioStartMs), 1600, accuracy: 0.5, "2.9 - 1.3 seconds, after the revision")
        XCTAssertEqual(t[1].transcript, "typed question")
        XCTAssertNil(t[1].speechEndedAt); XCTAssertNil(t[1].localCutMs); XCTAssertNil(t[1].deskFirstTokenMs, "a step that did not happen is nil, not 0")
        XCTAssertTrue(t[1].cueUsed)
        XCTAssertTrue(t[0].lines.contains { $0.contains("local sentence 2 heard") })
        XCTAssertTrue(t[0].lines.contains { $0.contains("local stream cut") && $0.contains("after 2 sentences") })
        XCTAssertTrue(t[1].lines.contains { $0.contains("local sentences: none") })
    }

    func testAudioBeforeAnyRevisionIsNotDeskAudio() {
        let t = TurnTimeline.compute([ev(1, .turnEnded), ev(1.2, .programmeAudioStart(block: 0))])
        XCTAssertNil(t[0].deskAudioStartMs)
    }

    func testTheExportRoundTripsAndCarriesTheTimeline() throws {
        let events = [ev(10, .turnEnded), ev(10.01, .finalTranscript("hello there")), ev(10.2, .localAckReady(text: "On it.", msSinceTurnEnd: 200)),
                      ev(10.25, .localSentenceStarted(text: "On it.", msSinceTurnEnd: 250)), ev(10.3, .ackStarted(msSinceTurnEnd: 300))]
        let rec = TurnRecord.fold(id: "t9", configuration: StageRegistry.defaultConfiguration(), device: DeviceInfo(model: "m", os: "o", appBuild: "b"), events: events)
        let x = TraceExport.make(records: [rec], log: events, device: DeviceInfo(model: "m", os: "o", appBuild: "b"), describe: LabRuntime.describe)
        let back = try JSONDecoder().decode(TraceExport.self, from: x.encoded())
        XCTAssertEqual(back.turns.first?.id, "t9")
        XCTAssertEqual(back.turns.first?.timeline?.localSentences.first?.text, "On it.")
        XCTAssertEqual(back.events.count, 5)
        XCTAssertTrue(back.events[2].text.contains("local reply ready"))
    }
}
