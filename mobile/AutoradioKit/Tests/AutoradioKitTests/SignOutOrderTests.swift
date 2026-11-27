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
    private var gate = false
    struct AtDelete: Sendable { var switchOn: Bool; var gateAllowed: Bool; var lastClaim: AudioState?; var deskLive: Bool; var voiceLive: Bool }
    private(set) var atDelete: AtDelete?
    private var lastClaimValue: AudioState?
    private var deskLiveValue = false, voiceLiveValue = false
    func reset() { l.lock(); n = 0; entries = []; gate = false; atDelete = nil; lastClaimValue = nil; deskLiveValue = false; voiceLiveValue = false; l.unlock() }
    @discardableResult func note(_ s: String) -> Int { l.lock(); defer { l.unlock() }; n += 1; entries.append((n, s)); return n }
    func seq(_ s: String) -> Int? { l.lock(); defer { l.unlock() }; return entries.first { $0.1 == s }?.0 }
    func count(_ s: String) -> Int { l.lock(); defer { l.unlock() }; return entries.filter { $0.1 == s }.count }
    var all: [String] { l.lock(); defer { l.unlock() }; return entries.map { "\($0.0):\($0.1)" } }
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
    init(_ initial: [String: String]) { v = initial }
    func get(_ a: String) -> String? { l.lock(); defer { l.unlock() }; return v[a] }
    func set(_ value: String, account: String) throws { l.lock(); v[account] = value; l.unlock() }
    func delete(_ a: String) { if a == KeyAccount.apiKey { SignOutProbe.shared.keyDeleted() }; l.lock(); v[a] = nil; l.unlock() }
}

final class OrderOwner: AudioSessionOwner, @unchecked Sendable {
    func claim(_ state: AudioState, preferBuiltInMic: Bool, reason: String) async throws { SignOutProbe.shared.claimed(state) }
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

    override func setUp() async throws {
        probe.reset()
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
        rt = LabRuntime(supportDir: dir, session: OrderOwner(), registry: registry, emptyProgramme: true)
        for (k, id) in [(StageKind.system1, "stub-s1"), (.voice, "stub-voice"), (.transcriber, "stub-tr"), (.holder, "local-ack"), (.desk, "order-desk")] { rt.setStage(k, id: id) }
        app = ChoirApp(runtime: rt, keys: keys)
        TurnGate.shared.keyPresent = { [keys] in keys!.get(KeyAccount.apiKey) != nil }     // what the module wires at launch
        // What today's module registers (`OnboardingController.reset`, run inside the wipe, after the key): the microphone closed. It stands for the OLD order; the fix puts the close first.
        app.accountWipe.register("onboarding") { [rt] in rt!.setHandsFree(false) }
    }

    override func tearDown() async throws {
        probe.gateOpen = true
        try? keys.set("test-key-2", account: KeyAccount.apiKey)      // a new sign-in completes: the gate must open again
        _ = TurnGate.shared.allowed
        TurnGate.shared.keyPresent = nil
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
        let started = await rt.engine.typedTurn("a question after sign-out began")
        XCTAssertFalse(started, "the engine refuses a new turn during the wait")
        XCTAssertEqual(probe.count("desk-request-live"), 1, "no second request was made: \(probe.all)")
        await done.value
        XCTAssertNil(keys.get(KeyAccount.apiKey))
        XCTAssertEqual(probe.count("keydelete"), 1)
    }

    // F6, F7: no new turn from the start of sign-out until a new sign-in completes
    func testNoNewTurnStartsFromTheStartOfSignOutUntilANewSignInCompletes() async throws {
        ackAfter(100)
        await app.signOut()
        XCTAssertFalse(TurnGate.shared.allowed, "signed out")
        rt.setHandsFree(true); XCTAssertFalse(HandsFreeSwitch.shared.isOn, "the microphone cannot be turned on")
        let started = await rt.engine.typedTurn("hello")
        XCTAssertFalse(started, "a typed turn is refused")
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
