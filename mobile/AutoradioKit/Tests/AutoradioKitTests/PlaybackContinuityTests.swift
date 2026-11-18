import XCTest
@testable import AutoradioKit

// Background audio, interruption, route loss and Play after the background. FAILURE LIST: see PlaybackContinuity.swift (B1 to B8).
// What these cannot show: that the system keeps the process alive, that Exact's owner activates a speech claim in the background, that
// AVSpeechSynthesizer and the neural voice synthesize there, that AVPlayer resumes. Those are on the phone (B8).

final class FakeOwner: AudioSessionOwner, @unchecked Sendable {
    let lock = NSLock()
    private(set) var claims: [AudioState] = []
    private(set) var releases = 0
    var failNext = false
    private var handler: (@Sendable (String, String) -> Void)?
    func claim(_ state: AudioState, preferBuiltInMic: Bool, reason: String) async throws {
        lock.lock(); defer { lock.unlock() }
        if failNext { failNext = false; throw LabError.unavailable("refused") }
        claims.append(state)
    }
    func release() { lock.lock(); releases += 1; lock.unlock() }
    func describe() -> String { "fake" }
    func observe(_ h: @escaping @Sendable (String, String) -> Void) { handler = h }
    func say(_ kind: String) { handler?(kind, "") }
}

final class PlaybackContinuityTests: XCTestCase {
    func make(voice: Locked<Bool>) -> (SessionCoordinator, FakeOwner, EventLog) {
        let owner = FakeOwner(), log = EventLog(clock: LabClock())
        let c = SessionCoordinator(inner: owner, log: log, audio: LiveAudio(), policy: { .alwaysTalking }, voiceOn: { voice.value })
        return (c, owner, log)
    }

    // B-pure
    func testNoticesReadTheSystemsRawValues() {
        XCTAssertTrue(SessionNotices.shouldResume(optionsRaw: 1))
        XCTAssertFalse(SessionNotices.shouldResume(optionsRaw: 0)); XCTAssertFalse(SessionNotices.shouldResume(optionsRaw: nil))
        XCTAssertTrue(SessionNotices.routeLost(reasonRaw: 2)); XCTAssertFalse(SessionNotices.routeLost(reasonRaw: 3))
    }
    func testMemoryResumesOnlyWithTheRecommendationAndOnlyOnce() {   // B2, B3, B7
        var m = InterruptionMemory()
        m.began(programmePlaying: true, remotePlaying: false)
        m.began(programmePlaying: false, remotePlaying: false)     // a second began sees the pause, keeps the first memory
        var r = m.ended(shouldResume: true)
        XCTAssertEqual(r.programme, true); XCTAssertEqual(r.remote, false)
        r = m.ended(shouldResume: true)
        XCTAssertEqual(r.programme, false, "consumed once")
        m.began(programmePlaying: true, remotePlaying: true)
        r = m.ended(shouldResume: false)
        XCTAssertEqual(r.programme, false); XCTAssertEqual(r.remote, false)
        m.began(programmePlaying: true, remotePlaying: false); m.forget()
        XCTAssertEqual(m.ended(shouldResume: true).programme, false, "the reader's tap clears it")
        m.remoteSystemPaused()
        XCTAssertEqual(m.ended(shouldResume: true).remote, true)
    }

    // The session is playback-only while voice is off, a capture session while it is on, and follows the switch.
    func testVoiceOffIsAPlaybackSessionAndTheSwitchMovesIt() async throws {
        let voice = Locked(false)
        let (c, owner, _) = make(voice: voice)
        try await c.claim(.talking, preferBuiltInMic: true, reason: "start")
        XCTAssertEqual(owner.claims, [.playing]); XCTAssertEqual(c.current, .playing)
        voice.write { $0 = true }; await c.reapply()
        XCTAssertEqual(owner.claims, [.playing, .talking])
        voice.write { $0 = false }; await c.reapply()
        XCTAssertEqual(owner.claims.last, .playing, "voice off again drops the microphone")
        try await c.claim(.listening, preferBuiltInMic: false, reason: "turn over")
        XCTAssertEqual(owner.claims.last, .playing, "a typed question's claims stay playback while voice is off")
    }

    // B4
    func testPlayDoesNotTrustWhatItRemembers() async throws {
        let voice = Locked(false)
        let (c, owner, _) = make(voice: voice)
        try await c.claim(.talking, preferBuiltInMic: false, reason: "start")
        var ok = await c.ensureReady(reason: "play")
        XCTAssertTrue(ok); XCTAssertEqual(owner.claims.count, 1, "healthy: nothing is taken again")
        c.markSuspect("app in the background")
        XCTAssertTrue(c.suspect)
        ok = await c.ensureReady(reason: "play")
        XCTAssertTrue(ok); XCTAssertEqual(owner.claims.count, 2, "doubted: the claim is released and taken again"); XCTAssertEqual(owner.releases, 1)
        XCTAssertFalse(c.suspect)
        c.markSuspect("again"); c.confirmHealthy()
        _ = await c.ensureReady(reason: "play")
        XCTAssertEqual(owner.claims.count, 2, "heard to be fine")
    }
    func testAFailedClaimIsAnEventAndPlayReportsIt() async throws {
        let (c, owner, log) = make(voice: Locked(false))
        try await c.claim(.talking, preferBuiltInMic: false, reason: "start")
        c.markSuspect("x"); owner.failNext = true
        let ok = await c.ensureReady(reason: "play")
        XCTAssertFalse(ok); XCTAssertNil(c.current)
        XCTAssertTrue(log.events.contains { if case .error(let st, let m) = $0.event { return st == "audio-session" && m.contains("refused") } else { return false } })
        let again = await c.ensureReady(reason: "play")
        XCTAssertTrue(again, "the next Play takes the claim")
    }

    // B1, B2, B3
    func testInterruptionEndsWithAndWithoutTheRecommendation() async throws {
        let (c, owner, _) = make(voice: Locked(false))
        try await c.claim(.talking, preferBuiltInMic: false, reason: "start")
        let seen = Locked<[String]>([])
        c.onInterruption = { began, go in seen.write { $0.append("\(began)/\(go)") } }
        owner.say("interrupted")
        XCTAssertEqual(seen.value, ["true/false"]); XCTAssertTrue(c.suspect)
        c.noteRecommendation(shouldResume: true)
        let e1 = expectation(description: "end 1")
        c.onInterruption = { began, go in seen.write { $0.append("\(began)/\(go)") }; if !began { e1.fulfill() } }
        owner.say("resumed")
        await fulfillment(of: [e1], timeout: 3)
        XCTAssertEqual(seen.value.last, "false/true"); XCTAssertEqual(owner.claims.count, 2, "recommended: the session is taken again")
        owner.say("interrupted")
        let e2 = expectation(description: "end 2")
        c.onInterruption = { began, go in seen.write { $0.append("\(began)/\(go)") }; if !began { e2.fulfill() } }
        owner.say("resumed")                                       // no recommendation heard
        await fulfillment(of: [e2], timeout: 3)
        XCTAssertEqual(seen.value.last, "false/false"); XCTAssertEqual(owner.claims.count, 2, "not recommended: nothing is taken by itself")
        let ok = await c.ensureReady(reason: "play")
        XCTAssertTrue(ok); XCTAssertEqual(owner.claims.count, 3, "the next Play re-establishes it")
    }
}
