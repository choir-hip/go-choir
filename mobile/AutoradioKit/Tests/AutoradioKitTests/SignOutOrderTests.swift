import XCTest
@testable import AutoradioKit

// Sign-out closes the microphone and the turn BEFORE the key goes (docs/problems/phone-sign-out-leaves-the-microphone-open-past-the-key-2026-11-27.md, item 1).
// Failure list, written before the code (each line: if X is wrong, test Y fails):
//  F1 the microphone is still open when the key is deleted (the switch is on, or the session is not the playback one) -> testIdle..., testHandsFreeListening..., testMicrophoneIsClosedBeforeTheWait...
//  F2 hands-free listening in the engine is still on after sign-out -> testHandsFreeListening...
//  F3 a turn in flight is not cancelled before the key is deleted (its desk request is still live at that moment) -> testATurnInFlight...
//  F4 a request the turn attempts after sign-out began is sent (it was not cancelled) -> testATurnInFlight... (the gate is opened after sign-out and nothing may be sent)
//  F5 the first spoken answer is still playing when the key is deleted -> testTheFirstSpokenAnswer...
//  F6 the gate still allows a turn at the key's deletion, or from the start of sign-out until a new sign-in -> testIdle..., testNoNewTurnStartsFromTheStart...
//  F7 the gate stays shut after a new sign-in completes -> testNoNewTurnStartsFromTheStart...
//  F8 the old key is deleted before shared code posts that it left the air (the wait is skipped or the key goes first) -> testMicrophoneIsClosedBeforeTheWait... (the key is still held while the wait runs)
//  F9 two sign-outs at once send `stopAll` twice (the second resets the first's wait) or delete the key before the microphone is closed -> testSignOutTwiceAtOnce
//  F10 the microphone is closed only after the 5 s wait elapses -> testMicrophoneIsClosedBeforeTheWait... (checked 1 s into a wait nobody answers)
// Every test observes ORDER: a shared sequence number is taken by the key store's delete, the session owner's claim, the desk's cancellation and the voice's end.

final class SignOutProbe: @unchecked Sendable {
    static let shared = SignOutProbe()
    private let l = NSLock()
    private var n = 0
    private var entries: [(Int, String)] = []
    private var gate = false, hold = false
    struct AtDelete: Sendable { var switchOn: Bool; var gateAllowed: Bool; var lastClaim: AudioState?; var deskLive: Bool; var voiceLive: Bool }
    private(set) var atDelete: AtDelete?
    private var lastClaimValue: AudioState?
    private var deskLiveValue = false, voiceLiveValue = false
    func reset() { l.lock(); n = 0; entries = []; gate = false; hold = false; atDelete = nil; lastClaimValue = nil; deskLiveValue = false; voiceLiveValue = false; l.unlock() }
    @discardableResult func note(_ s: String) -> Int { l.lock(); defer { l.unlock() }; n += 1; entries.append((n, s)); return n }
    func seq(_ s: String) -> Int? { l.lock(); defer { l.unlock() }; return entries.first { $0.1 == s }?.0 }
    func count(_ s: String) -> Int { l.lock(); defer { l.unlock() }; return entries.filter { $0.1 == s }.count }
    var all: [String] { l.lock(); defer { l.unlock() }; return entries.map { "\($0.0):\($0.1)" } }
    var holdOpen: Bool { get { l.lock(); defer { l.unlock() }; return hold } set { l.lock(); hold = newValue; l.unlock() } }
    var gateOpen: Bool { get { l.lock(); defer { l.unlock() }; return gate } set { l.lock(); gate = newValue; l.unlock() } }
    func claimed(_ s: AudioState) { l.lock(); lastClaimValue = s; l.unlock(); note("claim:\(s.rawValue)") }
    var lastClaim: AudioState? { l.lock(); defer { l.unlock() }; return lastClaimValue }
    func setDeskLive(_ v: Bool) { l.lock(); deskLiveValue = v; l.unlock() }
    func setVoiceLive(_ v: Bool) { l.lock(); voiceLiveValue = v; l.unlock() }
    var deskLive: Bool { l.lock(); defer { l.unlock() }; return deskLiveValue }
    var voiceLive: Bool { l.lock(); defer { l.unlock() }; return voiceLiveValue }
    func keyDeleted() {
        let snap = AtDelete(switchOn: HandsFreeSwitch.shared.isOn, gateAllowed: TurnGate.shared.allowed, lastClaim: lastClaim, deskLive: deskLive, voiceLive: voiceLive)
        l.lock(); if atDelete == nil { atDelete = snap }; l.unlock()
        note("keydelete")
    }
}

final class OrderKeys: KeyStore, @unchecked Sendable {
    private let l = NSLock(); private var v: [String: String]
    var failDelete = false      // the Keychain refuses: the key stays
    init(_ initial: [String: String]) { v = initial }
    func get(_ a: String) -> String? { l.lock(); defer { l.unlock() }; return v[a] }
    func set(_ value: String, account: String) throws { l.lock(); v[account] = value; l.unlock() }
    func delete(_ a: String) { if a == KeyAccount.apiKey { SignOutProbe.shared.keyDeleted() }; if failDelete { return }; l.lock(); v[a] = nil; l.unlock() }
}

final class OrderOwner: AudioSessionOwner, @unchecked Sendable {
    enum Mode { case normal, fails, hangs }
    nonisolated(unsafe) var mode = Mode.normal      // what moving OFF the microphone does: nothing wrong, throws, or never answers
    func claim(_ state: AudioState, preferBuiltInMic: Bool, reason: String) async throws {
        if state == .playing, mode == .fails { throw LabError.unavailable("the session refused") }
        if state == .playing, mode == .hangs { try? await Task.sleep(nanoseconds: 30_000_000_000); throw LabError.unavailable("late") }
        SignOutProbe.shared.claimed(state)
    }
    func release() { SignOutProbe.shared.note("release") }
}

/// What the module's TakeGate does (AutoradioModule+Takes.swift:12-26), which the Kit cannot link: the give-back turns hands-free on again, through the switch itself.
@MainActor final class OrderTakeGate: TakeRadioGate {
    let rt: LabRuntime; var handsFree = false; let waits: Bool
    init(_ rt: LabRuntime, waits: Bool = false) { self.rt = rt; self.waits = waits }
    func yieldForTake() async -> String? {
        handsFree = HandsFreeSwitch.shared.isOn
        if handsFree { HandsFreeSwitch.shared.set(false) }
        await rt.player.setLevel(.pause)
        if waits { SignOutProbe.shared.note("yield-waiting"); while !SignOutProbe.shared.gateOpen { try? await Task.sleep(nanoseconds: 20_000_000) } }
        do { try await rt.coordinator.setRecordingTake(true) } catch { giveBack(); return "the microphone could not be taken (\(error))" }
        return nil
    }
    func giveBack() {
        let hf = handsFree; handsFree = false
        Task { @MainActor in
            try? await rt.coordinator.setRecordingTake(false)
            await rt.player.setLevel(.play)
            if hf { HandsFreeSwitch.shared.set(true) }
        }
    }
}

/// A recogniser whose session does not answer `finish` until the test opens the gate: a commit suspended in transcription.
final class SlowTranscriber: Transcriber, @unchecked Sendable {
    static let stageID = "slow-tr"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "slow tr") }
    init(_ p: StageParams) {}
    func begin() async throws -> TranscriptionSession { Session() }
    final class Session: TranscriptionSession, @unchecked Sendable {
        let partials = AsyncStream<String> { $0.finish() }
        func append(_ frame: AudioFrame) {}
        func finish() async throws -> String { SignOutProbe.shared.note("finish-waiting"); while !SignOutProbe.shared.gateOpen { try? await Task.sleep(nanoseconds: 20_000_000) }; return "ask the harbour" }
        func cancel() {}
    }
}
/// A player whose `spoken()` (the first wait of a typed turn) does not answer until the gate opens, and that counts what it was asked to say.
final class SlowSpokenPlayback: PlaybackControl, @unchecked Sendable {
    let waits: Bool
    init(waits: Bool) { self.waits = waits }
    func position() async -> ProgramPosition { ProgramPosition(programmeID: "p", blockIndex: 1, context: "ctx") }
    func setLevel(_ level: PlayLevel) async {}
    func speak(_ frames: AsyncThrowingStream<AudioFrame, Error>) async { SignOutProbe.shared.note("SPOKEN"); do { for try await _ in frames {} } catch {} }
    func apply(_ revision: Revision) async { SignOutProbe.shared.note("SPOKEN-revision") }
    func resume() async {}
    func spoken() async -> SpokenCut? { if !waits { return nil }; SignOutProbe.shared.note("spoken-waiting"); while !SignOutProbe.shared.gateOpen { try? await Task.sleep(nanoseconds: 20_000_000) }; return nil }
}
/// A streaming desk whose answer arrives only when the gate opens, and that does not look at cancellation itself.
final class OrderStreamingDesk: StreamingDesk, @unchecked Sendable {
    static let stageID = "order-stream-desk"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "order stream desk") }
    init(_ p: StageParams) {}
    func submit(_ turn: DeskTurn) -> AsyncThrowingStream<Revision, Error> { AsyncThrowingStream { $0.finish() } }
    func speech(_ turn: DeskTurn, heard: HeardLines) -> AsyncThrowingStream<DeskEvent, Error> {
        AsyncThrowingStream { c in
            SignOutProbe.shared.note("stream-request")
            let t = Task { while !SignOutProbe.shared.gateOpen { try? await Task.sleep(nanoseconds: 20_000_000) }; SignOutProbe.shared.note("stream-ANSWER"); c.yield(.sentence(seq: 0, text: "The harbour is open.")); c.finish() }
            c.onTermination = { _ in t.cancel() }
        }
    }
}

/// A desk whose request is live until the test opens the gate; a cancelled turn must never get past it.
final class OrderDesk: Desk, @unchecked Sendable {
    static let stageID = "order-desk"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "order desk") }
    init(_ p: StageParams) {}
    func submit(_ turn: DeskTurn) -> AsyncThrowingStream<Revision, Error> {
        AsyncThrowingStream { c in
            let p = SignOutProbe.shared
            p.setDeskLive(true); p.note("desk-request-live")
            let t = Task {
                while !p.gateOpen { try? await Task.sleep(nanoseconds: 20_000_000); if Task.isCancelled { return } }
                if Task.isCancelled { return }
                p.note("desk-SENT-after-gate")           // what a turn that was not cancelled does once the server would answer
                c.finish()
            }
            c.onTermination = { _ in p.setDeskLive(false); p.note("desk-ended"); t.cancel() }
        }
    }
}

/// A voice that is saying something until it is cut: the first spoken answer being played.
final class OrderVoice: Voice, @unchecked Sendable {
    static let stageID = "order-voice"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "order voice") }
    init(_ p: StageParams) {}
    func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error> {
        AsyncThrowingStream { c in
            let p = SignOutProbe.shared
            let t = Task {
                for await _ in text {
                    p.setVoiceLive(true); p.note("voice-speaking")
                    var k = 0
                    while !Task.isCancelled { c.yield(AudioFrame(samples: [Float](repeating: 0.1, count: 800), sampleRate: 16000, t: Double(k) * 0.05)); k += 1; try? await Task.sleep(nanoseconds: 50_000_000) }
                }
                c.finish()
            }
            c.onTermination = { _ in p.setVoiceLive(false); p.note("voice-ended"); t.cancel() }
        }
    }
}

final class SignOutOrderTests: XCTestCase {
    let probe = SignOutProbe.shared
    var keys: OrderKeys!
    var rt: LabRuntime!
    var app: ChoirApp!
    var owner: OrderOwner!

    override func setUp() async throws {
        probe.reset(); TurnGate.shared.resetForTests()      // RESET
        _ = RadioBridge.shared.drain()
        RadioBridge.shared.changed = { for m in RadioBridge.shared.drain() where m.contains("stopAll") { SignOutProbe.shared.note("stopAll") } }
        keys = OrderKeys([KeyAccount.apiKey: "test-key", KeyAccount.keyID: "test-key-id", KeyAccount.computerID: "comp-1"])
        let dir = FileManager.default.temporaryDirectory.appendingPathComponent("so-\(UUID().uuidString)")
        let registry = StageRegistry.builtIn()
        registry.register(.system1, StubS1.self) { StubS1($0) }
        registry.register(.voice, StubVoice.self) { StubVoice($0) }
        registry.register(.voice, OrderVoice.self) { OrderVoice($0) }
        registry.register(.transcriber, StubTranscriber.self) { StubTranscriber($0) }
        registry.register(.desk, OrderDesk.self) { OrderDesk($0) }
        owner = OrderOwner()
        rt = LabRuntime(supportDir: dir, session: owner, registry: registry, emptyProgramme: true)
        for (k, id) in [(StageKind.system1, "stub-s1"), (.voice, "stub-voice"), (.transcriber, "stub-tr"), (.holder, "local-ack"), (.desk, "order-desk")] { rt.setStage(k, id: id) }
        app = ChoirApp(runtime: rt, keys: keys)
        TurnGate.shared.keyPresent = { [keys] in keys!.get(KeyAccount.apiKey) != nil }      // what the module wires at launch (main's own line)
        // What today's module registers (`OnboardingController.reset`, run inside the wipe, after the key): the microphone closed. It stands for the OLD order; the fix puts the close first.
        app.accountWipe.register("onboarding") { [rt] in rt!.setHandsFree(false) }
    }

    override func tearDown() async throws {
        await rt.engine.endForSignOut()      // a test that left a turn running must not leak its request into the next test
        _ = await poll(2) { !probe.deskLive && !probe.voiceLive }; try? await Task.sleep(nanoseconds: 400_000_000)      // and what it cancelled has finished (the probe is shared)
        probe.gateOpen = true
        try? keys.set("test-key-2", account: KeyAccount.apiKey)      // a new sign-in completes: the gate must open again
        TurnGate.shared.resetForTests()      // RESET
        HandsFreeSwitch.shared.set(false)
        LocalLanguage.model = nil
        RadioBridge.shared.changed = {}
        _ = RadioBridge.shared.drain()
    }

    /// Shared code answers `signedOut` a little after `stopAll` (it does so within 3 s in the app).
    func ackAfter(_ ms: UInt64) { Task { try? await Task.sleep(nanoseconds: ms * 1_000_000); RadioBridge.shared.markSignedOut() } }

    func startListening() async throws {
        rt.setHandsFree(true)
        XCTAssertTrue(HandsFreeSwitch.shared.isOn, "precondition: with a key the switch turns on")
        try await rt.coordinator.claim(.talking, preferBuiltInMic: false, reason: "test")
        XCTAssertEqual(probe.lastClaim, .talking, "precondition: the microphone's session is claimed")
    }

    func startTurn(words: String = "What changed in the harbour story?") async {
        let started = await rt.engine.typedTurn(words)
        XCTAssertTrue(started, "precondition: the turn started")
        let live = await poll(5) { probe.deskLive }
        XCTAssertTrue(live, "precondition: its request to the server is live")
    }

    func assertClosedAtKeyDelete(file: StaticString = #filePath, line: UInt = #line) throws {
        let d = try XCTUnwrap(probe.atDelete, "the key was deleted", file: file, line: line)
        XCTAssertFalse(d.switchOn, "hands-free is off when the key goes", file: file, line: line)
        XCTAssertFalse(d.gateAllowed, "the gate refuses when the key goes", file: file, line: line)
        XCTAssertNotEqual(d.lastClaim, .talking, "the microphone's session is not the capture one when the key goes", file: file, line: line)
        XCTAssertNotEqual(d.lastClaim, .listening, "the microphone's session is not the capture one when the key goes", file: file, line: line)
        XCTAssertFalse(d.deskLive, "the turn's request is cancelled when the key goes", file: file, line: line)
        XCTAssertFalse(d.voiceLive, "nothing is being spoken when the key goes", file: file, line: line)
    }

    // idle: nothing to cancel, but the gate and the switch are still in order
    func testIdleSignOutRefusesTurnsAndKeepsTheOrder() async throws {
        ackAfter(200)
        await app.signOut()
        let d = try XCTUnwrap(probe.atDelete)
        XCTAssertFalse(d.gateAllowed, "the gate refuses when the key goes")
        XCTAssertFalse(d.switchOn)
        XCTAssertNil(keys.get(KeyAccount.apiKey))
    }

    // F1, F2: hands-free was listening
    func testHandsFreeListeningIsClosedBeforeTheKeyGoes() async throws {
        try await startListening()
        ackAfter(200)
        await app.signOut()
        let d = try XCTUnwrap(probe.atDelete)
        XCTAssertFalse(d.switchOn, "hands-free is off when the key goes")
        XCTAssertEqual(d.lastClaim, .playing, "the microphone's session is the playback one (no microphone) when the key goes")
        let claimPlaying = try XCTUnwrap(probe.seq("claim:playing"), "the session was moved off the microphone")
        XCTAssertLessThan(claimPlaying, try XCTUnwrap(probe.seq("keydelete")), "the microphone closed before the key was deleted")
        let hfOn = await rt.engine.hf.enabled
        XCTAssertFalse(hfOn, "the engine hears nothing after sign-out")
    }

    // F3, F4: a turn is in flight (speech captured, the server has not answered)
    func testATurnInFlightIsCancelledBeforeTheKeyGoesAndSendsNothingMore() async throws {
        try await startListening()
        await startTurn()
        ackAfter(200)
        await app.signOut()
        try assertClosedAtKeyDelete()
        XCTAssertLessThan(try XCTUnwrap(probe.seq("desk-ended")), try XCTUnwrap(probe.seq("keydelete")), "the turn's request ended before the key was deleted")
        probe.gateOpen = true                                       // the server would answer now
        try? await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertEqual(probe.count("desk-SENT-after-gate"), 0, "a request attempted after sign-out began is never sent: \(probe.all)")
        let phase = await rt.engine.phase
        XCTAssertEqual(phase, .playing, "the engine is idle")
    }

    // F5: the first spoken answer is playing
    func testTheFirstSpokenAnswerIsCutBeforeTheKeyGoes() async throws {
        rt.setStage(.voice, id: "order-voice")
        LocalLanguage.model = FakeLocalModel(pieces: ["Looking into that now."], pieceDelay: 0.01)
        await startTurn()
        let speaking = await poll(5) { probe.voiceLive }
        XCTAssertTrue(speaking, "precondition: the first answer is being spoken: \(probe.all)")
        ackAfter(200)
        await app.signOut()
        try assertClosedAtKeyDelete()
        XCTAssertLessThan(try XCTUnwrap(probe.seq("voice-ended")), try XCTUnwrap(probe.seq("keydelete")), "the voice stopped before the key was deleted")
    }

    // F8, F10: shared code never answers. The microphone and the turn are closed at once; the key is held for the whole wait; then it goes.
    func testMicrophoneIsClosedBeforeTheWaitAndTheOldKeyIsHeldDuringIt() async throws {
        try await startListening()
        await startTurn()
        let done = Task { await app.signOut() }
        try? await Task.sleep(nanoseconds: 1_000_000_000)           // one second into a wait nobody answers
        XCTAssertEqual(keys.get(KeyAccount.apiKey), "test-key", "the old key is held while shared code is given the chance to post that it left the air")
        XCTAssertNil(probe.seq("keydelete"))
        XCTAssertFalse(HandsFreeSwitch.shared.isOn, "the microphone is already closed, before the wait elapses")
        XCTAssertEqual(probe.lastClaim, .playing, "the session already gave the microphone up")
        XCTAssertFalse(probe.deskLive, "the turn is already cancelled")
        XCTAssertFalse(TurnGate.shared.allowed, "no new turn can start during the wait")
        let requestsBefore = probe.count("desk-request-live")
        let started = await rt.engine.typedTurn("a question after sign-out began")
        XCTAssertFalse(started, "the engine refuses a new turn during the wait")
        try? await Task.sleep(nanoseconds: 200_000_000)
        XCTAssertEqual(probe.count("desk-request-live"), requestsBefore, "no new request was made: \(probe.all)")
        await done.value
        XCTAssertNil(keys.get(KeyAccount.apiKey))
        XCTAssertEqual(probe.count("keydelete"), 1)
    }

    // F6, F7: no new turn from the start of sign-out until a new sign-in completes
    func testNoNewTurnStartsFromTheStartOfSignOutUntilANewSignInCompletes() async throws {
        XCTAssertTrue(TurnGate.shared.allowed, "signed in: the gate (wired as the module wires it) allows a turn")
        ackAfter(100)
        await app.signOut()
        XCTAssertFalse(TurnGate.shared.allowed, "signed out")
        rt.setHandsFree(true); XCTAssertFalse(HandsFreeSwitch.shared.isOn, "the microphone cannot be turned on")
        try keys.set("test-key-2", account: KeyAccount.apiKey)     // the next sign-in completes
        XCTAssertTrue(TurnGate.shared.allowed, "after a new sign-in the gate opens again")
        rt.setHandsFree(true); XCTAssertTrue(HandsFreeSwitch.shared.isOn)
    }

    // F9: two sign-outs at once
    func testSignOutTwiceAtOnce() async throws {
        try await startListening()
        await startTurn()
        ackAfter(300)
        async let a: Void = app.signOut()
        async let b: Void = app.signOut()
        _ = await (a, b)
        try assertClosedAtKeyDelete()
        let stops = probe.count("stopAll")            // the wipe empties the inbox, so each send is read as it is made
        XCTAssertEqual(stops, 1, "one sign-out ran: the second joined the first, so it did not reset the first's wait")
        XCTAssertEqual(probe.count("keydelete"), 1)
        XCTAssertNil(keys.get(KeyAccount.apiKey))
    }
}


// ---- Round 2 (two reviewers). Failure list, written before the code:
//  R1 a take's give-back turns hands-free on after sign-out (switch on, session on the microphone at or after the key's deletion)        -> testATakesGiveBackCannotReopenTheMicrophone
//  R2 a turn suspended before it commits or runs (transcription, `spoken()`) resumes after sign-out began and sends or says something     -> testACommitSuspendedInTranscription..., testATypedTurnSuspendedBefore...
//  R3 an answer that arrives after sign-out began is spoken or written to the conversation                                                  -> testAnAnswerThatArrivesAfterSignOutBeganIsNotSpokenOrWritten
//  R4 sign-in during the wait: its new key is deleted, or the gate stays shut                                                               -> testANewSignInDuringTheWaitKeepsItsKeyAndOpensTheGate
//  R5 the key could not be deleted: the gate reopens for the account that is leaving                                                        -> testAKeyThatCannotBeDeletedLeavesTheGateShut
//  R6 a stale write that restores the old key after its deletion reopens the gate                                                           -> testAStaleWriteOfTheOldKeyDoesNotReopenTheGate
//  R7 moving the session off the microphone fails or hangs: the microphone stays open, or sign-out hangs                                    -> testASessionMoveThatFails..., testASessionMoveThatNeverAnswers...
//  R8 a typed turn is refused during a screen's `hold` (the microphone check), which worked before                                          -> testATypedTurnStillWorksDuringAScreensHold
//  R9 the gate is shut only after a hop: a turn begun on another task right after `signOut()` is called starts                             -> testATurnAttemptedTheMomentSignOutIsCalledIsRefused

extension SignOutOrderTests {
    func testATakesGiveBackCannotReopenTheMicrophone() async throws {
        try await startListening()
        let take = await MainActor.run { () -> TakeSession in
            let s = TakeSession(recorder: FakeRecorder(), transcriber: FakeTranscriber(), gate: OrderTakeGate(rt), preview: FakePreview(), micAllowed: { true }, responds: { nil },
                                dir: FileManager.default.temporaryDirectory.appendingPathComponent("so-take-\(UUID().uuidString)"))
            TakeSession.live = s; return s
        }
        await take.start(TakeLimits(minSeconds: 3, maxSeconds: 59.5, maxBytes: 512 * 1024))
        let state = await MainActor.run { take.state }
        XCTAssertEqual(state, .recording, "precondition: a take is being recorded while hands-free was on")
        ackAfter(200)
        await app.signOut()
        try assertClosedAtKeyDelete()
        for _ in 0..<10 { try? await Task.sleep(nanoseconds: 50_000_000); await MainActor.run {} }      // let the main actor drain: the give-back runs now
        XCTAssertFalse(HandsFreeSwitch.shared.isOn, "the give-back did not turn hands-free back on")
        let after = probe.all.drop { !$0.hasSuffix(":keydelete") }
        XCTAssertFalse(after.contains { $0.hasSuffix("claim:talking") || $0.hasSuffix("claim:listening") }, "no capture session after the key went: \(Array(after))")
        await MainActor.run { TakeSession.live = nil }
    }

    func testACommitSuspendedInTranscriptionSendsNothingAfterSignOutBegan() async throws {
        let (engine, pb) = hfEngine()
        await engine.handsFreeSet(true)
        let feed = Task { for _ in 0..<12 { await engine.ingestHandsFree(frame(true)) }; for _ in 0..<45 { await engine.ingestHandsFree(frame(false)) } }      // the commit runs inside the frame that ends the turn
        let waiting = await poll(8) { probe.count("finish-waiting") > 0 }
        XCTAssertTrue(waiting, "precondition: the commit is suspended in transcription")
        TurnGate.shared.closeForSignOut(); await engine.endForSignOut()
        probe.gateOpen = true
        await feed.value
        try? await Task.sleep(nanoseconds: 600_000_000)
        XCTAssertEqual(RecordingDesk.turns.count, 0, "no request reached the server stage")
        XCTAssertEqual(probe.count("SPOKEN"), 0, "nothing was spoken: \(probe.all)")
        _ = pb
        TurnGate.shared.finishSignOut()      // RESET
    }

    func testATypedTurnSuspendedBeforeItRunsSendsNothingAfterSignOutBegan() async throws {
        let (engine, _) = hfEngine(slowSpoken: true)
        let turn = Task { await engine.typedTurn("What changed in the harbour story?") }
        let waiting = await poll(5) { probe.count("spoken-waiting") > 0 }
        XCTAssertTrue(waiting, "precondition: the typed turn is suspended before runTurn")
        TurnGate.shared.closeForSignOut(); await engine.endForSignOut()
        probe.gateOpen = true
        let started = await turn.value
        try? await Task.sleep(nanoseconds: 400_000_000)
        XCTAssertFalse(started, "the turn did not start")
        XCTAssertEqual(RecordingDesk.turns.count, 0, "no request reached the server stage")
        XCTAssertEqual(probe.count("SPOKEN"), 0)
        TurnGate.shared.finishSignOut()      // RESET
    }

    func testAnAnswerThatArrivesAfterSignOutBeganIsNotSpokenOrWritten() async throws {
        let (engine, _) = hfEngine(streaming: true)
        let started = await engine.typedTurn("What changed in the harbour story?")
        XCTAssertTrue(started)
        let live = await poll(5) { probe.count("stream-request") > 0 }
        XCTAssertTrue(live, "precondition: the request is out, the answer not yet")
        let begun = probe.note("signout-begins")
        TurnGate.shared.closeForSignOut(); await engine.endForSignOut()
        probe.gateOpen = true                                          // the server answers now
        try? await Task.sleep(nanoseconds: 600_000_000)
        let after = probe.all.filter { (Int($0.split(separator: ":")[0]) ?? 0) > begun }
        XCTAssertFalse(after.contains { $0.hasSuffix("SPOKEN") || $0.hasSuffix("SPOKEN-revision") }, "nothing was spoken after sign-out began (the holding line before it is not this): \(after)")
        XCTAssertFalse(Conversation.shared.entries().contains { $0.answer.contains("harbour is open") }, "nothing was written to the conversation")
        TurnGate.shared.finishSignOut()      // RESET
    }

    // Main's own race, kept exactly: a sign-in that lands while sign-out runs is destroyed with the old one (the person signs in again). Not fixed here (problem record).
    func testASignInThatLandsDuringSignOutIsDestroyedAsOnMain() async throws {
        let done = Task { await app.signOut() }
        try? await Task.sleep(nanoseconds: 500_000_000)
        try keys.set("test-key-new", account: KeyAccount.apiKey); try keys.set("test-key-new-id", account: KeyAccount.keyID)
        RadioBridge.shared.markSignedOut()
        await done.value
        XCTAssertNil(keys.get(KeyAccount.apiKey), "the stored key is deleted, whichever it is, as on main")
        XCTAssertEqual(probe.count("keydelete"), 1)
    }

    // A key that cannot be deleted is a sign-out that did not finish (shared code says "try again"): the person is still signed in, and the gate does not stay shut or stuck.
    func testAKeyThatCannotBeDeletedLeavesTheSequenceEndedAndTheGateNotStuck() async throws {
        keys.failDelete = true
        ackAfter(200)
        await app.signOut()
        XCTAssertNotNil(keys.get(KeyAccount.apiKey), "precondition: the key is still stored")
        XCTAssertFalse(TurnGate.shared.signedOutShut, "the sequence ended")
        XCTAssertTrue(TurnGate.shared.allowed, "still signed in: as on main, a turn is allowed")
        keys.failDelete = false
    }

    // A sign-in with the SAME key value after a completed sign-out is a sign-in (there is no identity to compare, and none is kept).
    func testASignInWithTheSameKeyValueAfterSignOutIsAllowed() async throws {
        ackAfter(200)
        await app.signOut()
        XCTAssertFalse(TurnGate.shared.allowed)
        try keys.set("test-key", account: KeyAccount.apiKey); try keys.set("test-key-id", account: KeyAccount.keyID)
        XCTAssertTrue(TurnGate.shared.allowed)
    }

    func testASessionMoveThatFailsStillClosesTheMicrophone() async throws {
        try await startListening()
        owner.mode = .fails
        ackAfter(200)
        await app.signOut()
        XCTAssertGreaterThan(probe.count("release"), 0, "the session was released the direct way")
        XCTAssertFalse(HandsFreeSwitch.shared.isOn)
        XCTAssertNil(keys.get(KeyAccount.apiKey), "sign-out went on")
    }

    func testASessionMoveThatNeverAnswersDoesNotHangSignOut() async throws {
        try await startListening()
        owner.mode = .hangs
        ackAfter(200)
        let done = Task { await app.signOut() }
        let finished = await poll(9) { keys.get(KeyAccount.apiKey) == nil }
        XCTAssertTrue(finished, "sign-out reached the key within the bound")
        XCTAssertGreaterThan(probe.count("release"), 0, "the session was released the direct way, not waited for")
        XCTAssertFalse(HandsFreeSwitch.shared.isOn)
        _ = done      // not awaited: on the old code it never returns
    }

    func testATypedTurnStillWorksDuringAScreensHold() async throws {
        TurnGate.shared.hold(); defer { TurnGate.shared.release() }
        let started = await rt.engine.typedTurn("What changed in the harbour story?")
        XCTAssertTrue(started, "a screen's hold is not a reason to refuse a typed question")
    }

    func testATurnAttemptedTheMomentSignOutIsCalledIsRefused() async throws {
        let done = Task { await app.signOut() }
        await Task.yield()
        let started = await rt.engine.typedTurn("a question right after sign-out was called")
        XCTAssertFalse(started)
        XCTAssertEqual(probe.count("desk-request-live"), 0)
        RadioBridge.shared.markSignedOut()
        await done.value
    }

    // R10: the one place a native request reads the key refuses while the account signs out (the check that makes this class impossible whichever stage forgot its own)
    func testNoNativeRequestReadsTheKeyWhileTheAccountSignsOut() async throws {
        let cfg = URLSessionConfiguration.ephemeral; cfg.protocolClasses = [StubProtocol.self]
        StubProtocol.seen = []; StubProtocol.handler = { _ in .init() }
        let client = ChoirClient(baseURL: "https://example.test", keys: InMemoryKeyStore([KeyAccount.apiKey: "test-key", KeyAccount.computerID: "comp-1"]), session: URLSession(configuration: cfg), log: nil, trustedOrigin: nil)
        TurnGate.shared.closeForSignOut()
        defer { TurnGate.shared.finishSignOut() }      // RESET
        do { _ = try await client.send("GET", "/api/x", route: "/api/x"); XCTFail("a request was made while the account signs out") } catch ChoirError.notSignedIn {} catch { XCTFail("wrong error \(error)") }
        XCTAssertEqual(StubProtocol.seen.count, 0, "nothing reached the server")
    }

    // helpers
    func frame(_ loud: Bool) -> AudioFrame { AudioFrame(samples: [Float](repeating: loud ? 0.5 : 0, count: 512), sampleRate: 16000, t: 0) }
    func hfEngine(slowSpoken: Bool = false, streaming: Bool = false) -> (TurnEngine, PlaybackControl) {
        let r = StageRegistry.builtIn()
        r.register(.system1, StubS1.self) { StubS1($0) }; r.register(.voice, StubVoice.self) { StubVoice($0) }
        r.register(.speechBounds, LoudBounds.self) { LoudBounds($0) }; r.register(.transcriber, SlowTranscriber.self) { SlowTranscriber($0) }
        r.register(.desk, RecordingDesk.self) { RecordingDesk($0) }; r.register(.desk, OrderStreamingDesk.self) { OrderStreamingDesk($0) }
        r.register(.transcriber, StubTranscriber.self) { StubTranscriber($0) }
        var cfg = StageRegistry.defaultConfiguration()
        cfg.set(.addressing, .init(id: "open")); cfg.set(.speechBounds, .init(id: "loud-bounds")); cfg.set(.transcriber, .init(id: slowSpoken || streaming ? "stub-tr" : "slow-tr"))
        cfg.set(.system1, .init(id: "stub-s1", params: ["delay": .number(0), "asks": .string("yes")])); cfg.set(.voice, .init(id: "stub-voice")); cfg.set(.holder, .init(id: "local-ack"))
        cfg.set(.desk, .init(id: streaming ? "order-stream-desk" : "rec-desk"))
        RecordingDesk.turns = []
        let pb: PlaybackControl = slowSpoken || streaming ? SlowSpokenPlayback(waits: slowSpoken) : StoppablePlayback()
        return (TurnEngine(env: EngineEnvironment(registry: r, log: EventLog(clock: LabClock()), playback: pb), configuration: cfg), pb)
    }
}


// ---- Final round. Failure list, written before the code:
//  T1 a take started during the wait, or suspended in the permission prompt / the radio's yield across sign-out, opens the microphone under the old account -> testATakeStartedDuring..., testATakeSuspendedIn...
//  T3 a turn suspended in `addressing.decide` gets its pipeline created after sign-out ran `interruptPipeline` (nothing to cancel): only the guard at the start of the pipeline body stops it      -> testATurnSuspendedInAddressing...
//  T4 a turn suspended in `holder.retrieve` goes on to the desk request: the re-check before it and the client's check both stop it                                                                   -> testATurnSuspendedInRetrieval...
//  T5 an answer element that passed its check and is suspended in its apply step writes to the conversation / hands to speech after sign-out began                                                    -> testAnAnswerSuspendedInItsApplyStep...
//  T6 a second sign-out called from the first's last callback finds the finished task and nobody clears the gate                                                                                      -> testASecondSignOutFromTheLastCallbackDoesNotStickTheGate
//  T7 a stale `handsFreeSet(true)` after sign-out began leaves the engine listening                                                                                                                  -> testAStaleHandsFreeSetDuringSignOutIsRefused
//  T8 the enrolment tap is set again after it was cleared                                                                                                                                            -> testTheEnrolmentTapCannotBeSetDuringSignOut
//  T9 the gate is shut synchronously by the first call; a turn started right after is refused                                                                                                        -> testAGateShutSynchronouslyRefusesTheNextTurn
//  T10 after the direct close (`stop`) nothing starts the engine again after a new sign-in                                                                                                           -> testTheEngineStartsAgainAfterTheDirectClose
//  T11 an old turn's request, made after sign-out AND a new sign-in, carries the new key (request not bound to its turn's epoch)                                                                     -> testARequestIsBoundToTheEpochOfTheTurnThatMadeIt  (NEWAPI)

final class SlowAddressing: Addressing, @unchecked Sendable {
    static let stageID = "slow-addr"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "slow addr") }
    init(_ p: StageParams) {}
    var holdsTurnOpen: Bool { false }
    var noSpeechTimeout: Double { 6 }
    func decide(_ input: AddressInput, now: Double) async -> AddressDecision {
        if input == .turnFinished { SignOutProbe.shared.note("decide-waiting"); while !SignOutProbe.shared.gateOpen { try? await Task.sleep(nanoseconds: 20_000_000) } }
        return .none
    }
}
final class SlowHolder: Holder, @unchecked Sendable {
    static let stageID = "slow-holder"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "slow holder") }
    init(_ p: StageParams) {}
    func lines(for input: HolderInput) -> AsyncStream<String> { AsyncStream { $0.finish() } }
    func lines(for input: HolderInput, later: @escaping @Sendable () async -> [System1Answer]) -> AsyncStream<String> { AsyncStream { $0.finish() } }
    func cancel() {}
    func diagnostics() -> String { "" }
    func report() -> GenerationReport? { nil }
    func retrieve(words: String, pageTitle: String, pageAddress: String) async -> RetrievalContext? {
        SignOutProbe.shared.note("retrieve-waiting")
        while !SignOutProbe.shared.gateOpen { try? await Task.sleep(nanoseconds: 20_000_000) }
        return nil
    }
}
/// A desk that makes its request through the REAL client (against a stand-in server), as the app's desks do.
final class OrderClientDesk: Desk, @unchecked Sendable {
    static let stageID = "order-client-desk"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "order client desk") }
    init(_ p: StageParams) {}
    func submit(_ turn: DeskTurn) -> AsyncThrowingStream<Revision, Error> {
        AsyncThrowingStream { c in
            SignOutProbe.shared.note("client-desk-submit")
            let cfg = URLSessionConfiguration.ephemeral; cfg.protocolClasses = [StubProtocol.self]
            let client = ChoirClient(baseURL: "https://example.test", keys: InMemoryKeyStore([KeyAccount.apiKey: "test-key-new", KeyAccount.computerID: "comp-1"]), session: URLSession(configuration: cfg), log: nil, trustedOrigin: nil)
            let t = Task { do { _ = try await client.send("GET", "/x", route: "/x"); SignOutProbe.shared.note("client-SENT") } catch { SignOutProbe.shared.note("client-refused") }; c.finish() }
            c.onTermination = { _ in t.cancel() }
        }
    }
}
/// Blocks the engine's actor (a synchronous method) until the test lets go: whatever needs the actor waits behind it.
extension TurnEngine {
    func holdForTest() { SignOutProbe.shared.note("hold-running"); while !SignOutProbe.shared.holdOpen { usleep(10_000) } }
}

extension SignOutOrderTests {
    func takeSession(waitsInMic: Bool = false, waitsInYield: Bool = false) async -> (TakeSession, FakeRecorder) {
        await MainActor.run {
            let rec = FakeRecorder()
            let s = TakeSession(recorder: rec, transcriber: FakeTranscriber(), gate: OrderTakeGate(rt, waits: waitsInYield), preview: FakePreview(),
                                micAllowed: { if waitsInMic { SignOutProbe.shared.note("mic-waiting"); while !SignOutProbe.shared.gateOpen { try? await Task.sleep(nanoseconds: 20_000_000) } }; return true },
                                responds: { nil }, dir: FileManager.default.temporaryDirectory.appendingPathComponent("so-take-\(UUID().uuidString)"))
            TakeSession.live = s; return (s, rec)
        }
    }
    func takeLimits() -> TakeLimits { TakeLimits(minSeconds: 3, maxSeconds: 59.5, maxBytes: 512 * 1024) }
    func assertNoTakeIsOpen(_ take: TakeSession, _ rec: FakeRecorder, file: StaticString = #filePath, line: UInt = #line) async {
        let (state, started) = await MainActor.run { (take.state, rec.started.count) }
        XCTAssertEqual(state, .idle, "no take is recording", file: file, line: line)
        XCTAssertEqual(started, 0, "the recorder never started", file: file, line: line)
        XCTAssertFalse(HandsFreeSwitch.shared.isOn, file: file, line: line)
        XCTAssertNotEqual(probe.lastClaim, .listening, "the session is not the capture one", file: file, line: line)
        XCTAssertNotEqual(probe.lastClaim, .talking, file: file, line: line)
    }

    // T1, scenario 1: Record is tapped during the wait, after the voice was closed
    func testATakeStartedDuringTheWaitOpensNoMicrophone() async throws {
        try await startListening()
        let (take, rec) = await takeSession()
        let done = Task { await app.signOut() }
        try? await Task.sleep(nanoseconds: 700_000_000)
        await take.start(takeLimits())
        await assertNoTakeIsOpen(take, rec)
        RadioBridge.shared.markSignedOut(); await done.value
        try assertClosedAtKeyDelete()
        await assertNoTakeIsOpen(take, rec)
        await MainActor.run { TakeSession.live = nil }
    }
    // T1, scenario 2: start() is suspended in the permission prompt when sign-out begins
    func testATakeSuspendedInThePermissionPromptIsRefusedWhenItResumes() async throws {
        try await startListening()
        let (take, rec) = await takeSession(waitsInMic: true)
        let started = Task { await take.start(takeLimits()) }
        let waiting = await poll(5) { probe.count("mic-waiting") > 0 }; XCTAssertTrue(waiting)
        ackAfter(200); await app.signOut()
        probe.gateOpen = true; await started.value
        try assertClosedAtKeyDelete()
        await assertNoTakeIsOpen(take, rec)
        await MainActor.run { TakeSession.live = nil }
    }
    // T1, scenario 2b: start() is suspended in the radio's yield; the sign-out ends before it resumes. Its microphone is given back at once (the brief opening is what remains).
    func testATakeSuspendedInTheRadiosYieldEndsWithTheMicrophoneClosed() async throws {
        try await startListening()
        let (take, rec) = await takeSession(waitsInYield: true)
        let started = Task { await take.start(takeLimits()) }
        let waiting = await poll(5) { probe.count("yield-waiting") > 0 }; XCTAssertTrue(waiting)
        ackAfter(200); await app.signOut()
        probe.gateOpen = true; await started.value
        for _ in 0..<10 { try? await Task.sleep(nanoseconds: 50_000_000); await MainActor.run {} }
        await assertNoTakeIsOpen(take, rec)
        XCTAssertEqual(probe.lastClaim, .playing, "the capture session was given back")
        await MainActor.run { TakeSession.live = nil }
    }

    // T3: only the guard at the start of the pipeline body stops this one
    func testATurnSuspendedInAddressingStartsNoPipelineAfterSignOut() async throws {
        rt.registry.register(.addressing, SlowAddressing.self) { SlowAddressing($0) }; rt.registry.register(.holder, SlowHolder.self) { SlowHolder($0) }
        rt.setStage(.addressing, id: "slow-addr"); rt.setStage(.holder, id: "slow-holder")      // the quick holder: the default one waits for shared code's retrieval
        let applied = await poll(5) { let c = await rt.engine.configuration; return c.setting(.addressing)?.id == "slow-addr" && c.setting(.holder)?.id == "slow-holder" }; XCTAssertTrue(applied, "precondition: the engine has the stage")
        let before = probe.count("desk-request-live")
        let t = Task { await rt.engine.typedTurn("What changed in the harbour story?") }
        let waiting = await poll(5) { probe.count("decide-waiting") > 0 }; XCTAssertTrue(waiting, "precondition: runTurn is suspended in addressing.decide")
        ackAfter(200); await app.signOut()
        probe.gateOpen = true; _ = await t.value
        try? await Task.sleep(nanoseconds: 500_000_000)
        XCTAssertEqual(probe.count("desk-request-live"), before, "no request reached the desk: \(probe.all)")
        XCTAssertEqual(probe.count("SPOKEN"), 0)
    }

    // T4: suspended in holder.retrieve; the desk's request goes through the real client against a stand-in server
    func testATurnSuspendedInRetrievalMakesNoRequestAfterSignOut() async throws {
        rt.registry.register(.holder, SlowHolder.self) { SlowHolder($0) }; rt.registry.register(.desk, OrderClientDesk.self) { OrderClientDesk($0) }
        rt.setStage(.holder, id: "slow-holder"); rt.setStage(.desk, id: "order-client-desk")
        let applied = await poll(5) { let c = await rt.engine.configuration; return c.setting(.holder)?.id == "slow-holder" && c.setting(.desk)?.id == "order-client-desk" }; XCTAssertTrue(applied, "precondition: the engine has the stages")
        StubProtocol.seen = []; StubProtocol.handler = { _ in .init() }
        let started = await rt.engine.typedTurn("What changed in the harbour story?"); XCTAssertTrue(started)
        let waiting = await poll(10) { probe.count("retrieve-waiting") > 0 }; XCTAssertTrue(waiting, "precondition: the pipeline is suspended in holder.retrieve: \(probe.all)")
        ackAfter(200); await app.signOut()
        try keys.set("test-key-new", account: KeyAccount.apiKey)       // a new sign-in has completed
        probe.gateOpen = true
        try? await Task.sleep(nanoseconds: 600_000_000)
        XCTAssertEqual(StubProtocol.seen.count, 0, "no request reached the server: \(probe.all)")
        XCTAssertEqual(probe.count("client-SENT"), 0)
    }

    // T5: the element passed its check; its apply step (the hop to the engine) is suspended across sign-out beginning
    func testAnAnswerSuspendedInItsApplyStepIsNotSpokenOrWritten() async throws {
        let (engine, _) = hfEngine(streaming: true)
        let started = await engine.typedTurn("What changed in the harbour story?"); XCTAssertTrue(started)
        let live = await poll(5) { probe.count("stream-request") > 0 }; XCTAssertTrue(live)
        let hold = Task { await engine.holdForTest() }
        let holding = await poll(5) { probe.count("hold-running") > 0 }; XCTAssertTrue(holding, "precondition: the engine is busy")
        probe.gateOpen = true                                          // the answer arrives, passes its check, and waits for the engine
        try? await Task.sleep(nanoseconds: 300_000_000)
        let begun = probe.note("signout-begins")
        TurnGate.shared.closeForSignOut()
        probe.holdOpen = true; await hold.value
        try? await Task.sleep(nanoseconds: 500_000_000)               // the apply step runs now, with the flag up and the turn not yet cancelled: only its own check stops it
        let after = probe.all.filter { (Int($0.split(separator: ":")[0]) ?? 0) > begun }
        XCTAssertFalse(Conversation.shared.entries().contains { $0.answer.contains("harbour is open") }, "nothing was written to the conversation: \(after)")
        XCTAssertFalse(after.contains { $0.hasSuffix("SPOKEN") }, "nothing was handed to speech: \(after)")
        await engine.endForSignOut()
        TurnGate.shared.finishSignOut()
    }

    // T6: the first sign-out's last callback calls sign-out again
    func testASecondSignOutFromTheLastCallbackDoesNotStickTheGate() async throws {
        let again = Locked<Task<Void, Never>?>(nil)
        app.onChange = { [app] in if again.value == nil { again.value = Task { await app!.signOut() } } }
        ackAfter(200)
        await app.signOut()
        await again.value?.value
        XCTAssertFalse(TurnGate.shared.signedOutShut, "the gate is not stuck shut")
        app.onChange = nil
    }

    // T7, T8
    func testAStaleHandsFreeSetDuringSignOutIsRefused() async throws {
        await rt.engine.handsFreeSet(false)      // the engine alone starts with it on
        TurnGate.shared.closeForSignOut()
        await rt.engine.handsFreeSet(true)
        let on = await rt.engine.hf.enabled
        XCTAssertFalse(on, "the engine is not left listening")
        TurnGate.shared.finishSignOut()
    }
    func testTheEnrolmentTapCannotBeSetDuringSignOut() async throws {
        TurnGate.shared.closeForSignOut()
        await rt.setEnrolmentTap { _ in }
        let tap = await rt.engine.enrolmentTap
        XCTAssertNil(tap, "a tap on the microphone is not set while the account signs out")
        TurnGate.shared.finishSignOut()
    }

    // T9: the gate is shut by one synchronous call (what the module makes where the native op arrives)
    func testAGateShutSynchronouslyRefusesTheNextTurn() async throws {
        TurnGate.shared.closeForSignOut()
        let started = await rt.engine.typedTurn("hello")
        XCTAssertFalse(started)
        TurnGate.shared.finishSignOut()
    }

    // T10: after the 2 s fallback closed the engine, the next play or ask starts it again (ensureEngine / the host's startRequest)
    func testTheEngineStartsAgainAfterTheDirectClose() async throws {
        defer { LabEnvironment.virtualAudio = false; LabEnvironment.promptsAllowed = true }
        rt.startRequest = { [rt] in await rt!.start(virtual: true) }
        _ = await app.ensureEngine()
        XCTAssertTrue(rt.snapshot().running, "precondition: the engine runs")
        try await startListening()
        owner.mode = .fails
        ackAfter(200); await app.signOut()
        XCTAssertFalse(rt.snapshot().running, "the direct close stopped the engine")
        owner.mode = .normal
        try keys.set("test-key-2", account: KeyAccount.apiKey)           // a new sign-in
        _ = await app.ensureEngine()
        XCTAssertTrue(rt.snapshot().running, "the next play starts it again")
    }

    // NEWAPI-BEGIN
    // T11
    func testARequestIsBoundToTheEpochOfTheTurnThatMadeIt() async throws {
        let client = ChoirClient(baseURL: "https://example.test", keys: InMemoryKeyStore([KeyAccount.apiKey: "test-key-new", KeyAccount.computerID: "comp-1"]), session: .shared, log: nil, trustedOrigin: nil)
        let oldEpoch = TurnGate.shared.epoch
        ackAfter(100); await app.signOut()                              // fast sign-out, then a new sign-in completes: the gate is open again
        try keys.set("test-key-new", account: KeyAccount.apiKey)
        XCTAssertTrue(TurnGate.shared.allowed)
        await TurnGate.$turnEpoch.withValue(oldEpoch) {
            do { _ = try await client.request("GET", "/x"); XCTFail("the old turn's request was built with the new account's key") } catch ChoirError.notSignedIn {} catch { XCTFail("\(error)") }
        }
        let fresh = try await TurnGate.$turnEpoch.withValue(TurnGate.shared.epoch) { try await client.request("GET", "/x") }
        XCTAssertNotNil(fresh.value(forHTTPHeaderField: "Authorization"), "a request of the current epoch is built")
    }
    func testTheTurnsEpochTravelsToItsDesk() async throws {
        rt.registry.register(.desk, OrderEpochDesk.self) { OrderEpochDesk($0) }; rt.setStage(.desk, id: "order-epoch-desk")
        let applied = await poll(5) { await rt.engine.configuration.setting(.desk)?.id == "order-epoch-desk" }; XCTAssertTrue(applied)
        let e = TurnGate.shared.epoch
        let started = await rt.engine.typedTurn("What changed in the harbour story?"); XCTAssertTrue(started)
        let seen = await poll(5) { probe.count("epoch-\(e)") > 0 }
        XCTAssertTrue(seen, "the desk ran under the turn's epoch: \(probe.all)")
    }
    // NEWAPI-END
}
// NEWAPI-BEGIN
final class OrderEpochDesk: Desk, @unchecked Sendable {
    static let stageID = "order-epoch-desk"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "order epoch desk") }
    init(_ p: StageParams) {}
    func submit(_ turn: DeskTurn) -> AsyncThrowingStream<Revision, Error> { SignOutProbe.shared.note("epoch-\(TurnGate.turnEpoch ?? -1)"); return AsyncThrowingStream { $0.finish() } }
}
// NEWAPI-END
