import XCTest
@testable import AutoradioKit

// Hands-free as one on/off state (HandsFreeSwitch.swift: T4 to T7) and when the listening state shows. The engine tests push frames as
// HandsFreeTests does: loud frames are speech, quiet frames silence; the transcriber is scripted (LoudBounds, ScriptedTranscriber).
final class HandsFreeToggleTests: XCTestCase {
    // ---- T7: when the listening state shows (pure) ----
    func testVoiceActivityAloneIsNotVisible() {
        XCTAssertFalse(ListeningVisibility.visible(handsFree: true, speechMs: 20_000, words: 0, addressed: false, notATurn: false))
        XCTAssertFalse(ListeningVisibility.visible(handsFree: true, speechMs: 300, words: 0, addressed: false, notATurn: false))
    }
    func testSustainedSpeechWithWordsIsVisible() {
        XCTAssertTrue(ListeningVisibility.visible(handsFree: true, speechMs: 800, words: 2, addressed: false, notATurn: false))
        XCTAssertFalse(ListeningVisibility.visible(handsFree: true, speechMs: 800, words: 1, addressed: false, notATurn: false), "one word is a cough or a name")
        XCTAssertFalse(ListeningVisibility.visible(handsFree: true, speechMs: 400, words: 3, addressed: false, notATurn: false), "too short to count")
    }
    func testAnAddressedTurnIsVisible() {
        XCTAssertTrue(ListeningVisibility.visible(handsFree: true, speechMs: 0, words: 0, addressed: true, notATurn: false))
    }
    func testNotATurnIsHiddenAgain() {
        XCTAssertFalse(ListeningVisibility.visible(handsFree: true, speechMs: 5000, words: 9, addressed: false, notATurn: true))
        XCTAssertFalse(ListeningVisibility.visible(handsFree: true, speechMs: 5000, words: 9, addressed: true, notATurn: true))
    }
    func testOffIsNeverVisible() {
        XCTAssertFalse(ListeningVisibility.visible(handsFree: false, speechMs: 5000, words: 9, addressed: true, notATurn: false))
    }

    // ---- voice is off unless the listener turns it on (owner, 2026-11-16) ----
    // Failure list, written first: V1 launch turns it on; V2 returning to the foreground turns it on; V3 playback start turns it on (the
    // runtime's start, with the microphone granted, is the launch path); V4 a stored "on" turns it on at the next launch; V5 the explicit
    // tap does not turn it on; V6 the quiet timeout does not turn it off; V7 the listener speaking does not restart the timer;
    // V8 the explicit tap off does not turn it off; V9 a granted microphone answer turns it on.
    final class TestClock: @unchecked Sendable { var t = Date(timeIntervalSince1970: 1_000_000) }
    func make(_ c: TestClock) -> HandsFreeSwitch { HandsFreeSwitch(now: { c.t }) }
    func testFreshSwitchIsOff() { XCTAssertFalse(HandsFreeSwitch().isOn) }
    func testNothingButTheTapTurnsItOn() {
        let c = TestClock(); let s = make(c)
        s.readerSpoke(); c.t.addTimeInterval(5); XCTAssertFalse(s.expireIfIdle()); XCTAssertFalse(s.isOn, "a turn or the clock never turns it on")
        s.set(true); XCTAssertTrue(s.isOn)
    }
    func testOnIsNotKeptAcrossLaunches() {
        let suite = UserDefaults(suiteName: "hf-\(UUID().uuidString)")!
        let a = HandsFreeSwitch(defaults: suite); a.set(true)
        XCTAssertFalse(HandsFreeSwitch(defaults: suite).isOn, "a new launch is off whatever the last one did")
        XCTAssertFalse(HandsFreeSwitch().isOn)
    }
    func testLaunchStartingTheRuntimeLeavesVoiceOff() async {
        HandsFreeSwitch.shared.set(false)
        let rt = LabRuntime(supportDir: FileManager.default.temporaryDirectory.appendingPathComponent("hf-\(UUID().uuidString)"), session: NullSessionOwner())
        await rt.start(virtual: true)            // virtual: the microphone counts as granted, the case that used to turn it on
        XCTAssertFalse(rt.handsFreeIsOn); XCTAssertFalse(HandsFreeSwitch.shared.isOn)
        let closed = await poll(3) { await rt.engine.hf.enabled == false }
        XCTAssertTrue(closed, "the engine looks at no frame")
        rt.player.pause()
    }
    func testExplicitTapTurnsItOnAndATapTurnsItOff() async {
        HandsFreeSwitch.shared.set(false)
        let rt = LabRuntime(supportDir: FileManager.default.temporaryDirectory.appendingPathComponent("hf-\(UUID().uuidString)"), session: NullSessionOwner())
        await rt.start(virtual: true)
        rt.setHandsFree(true)
        XCTAssertTrue(rt.handsFreeIsOn)
        rt.setHandsFree(false)
        XCTAssertFalse(rt.handsFreeIsOn)
        let heard = await poll(3) { await rt.engine.hf.enabled == false }
        XCTAssertTrue(heard)
        rt.player.pause()
    }
    /// The decision at the call site (review S5): turning voice on with the microphone not open says an error only for a FINAL state. A pending retry that then opens says nothing; one that fails says it once.
    func listenErrors(_ rt: LabRuntime) -> [String] { rt.log.events.compactMap { if case .error(let stage, let m) = $0.event, stage == "listen" { return m } else { return nil } } }
    func testAPendingMicrophoneThatOpensSaysNoError() async {
        HandsFreeSwitch.shared.set(false)
        let rt = LabRuntime(supportDir: FileManager.default.temporaryDirectory.appendingPathComponent("hf-\(UUID().uuidString)"), session: NullSessionOwner())
        rt.micSettleNanos = 10_000_000; rt.micPollNanos = 10_000_000
        let reads = Locked<Int>(0)
        rt.micStateNow = { reads.write { $0 += 1 }; return reads.value < 6 ? .pending : .open }
        rt.setHandsFree(true, persist: false)
        _ = await poll(3) { reads.value >= 6 }
        try? await Task.sleep(nanoseconds: 100_000_000)
        XCTAssertEqual(listenErrors(rt), [], "a retry that opens the microphone left no error line")
        rt.setHandsFree(false, persist: false)
    }
    func testAFailedMicrophoneSaysItOnce() async {
        HandsFreeSwitch.shared.set(false)
        let rt = LabRuntime(supportDir: FileManager.default.temporaryDirectory.appendingPathComponent("hf-\(UUID().uuidString)"), session: NullSessionOwner())
        rt.micSettleNanos = 10_000_000; rt.micPollNanos = 10_000_000
        let reads = Locked<Int>(0)
        rt.micStateNow = { reads.write { $0 += 1 }; return reads.value < 4 ? .pending : .failed }
        rt.setHandsFree(true, persist: false)
        _ = await poll(3) { !self.listenErrors(rt).isEmpty }
        XCTAssertEqual(listenErrors(rt).count, 1)
        rt.setHandsFree(false, persist: false)
    }
    func testOnStaysOnForTheQuietTimeoutThenGoesOffSilently() {
        let c = TestClock(); let s = make(c); s.set(true)
        let told = Locked<Int>(0)
        s.onChange = { told.write { $0 += 1 } }
        XCTAssertGreaterThanOrEqual(HandsFreeSwitch.idleLimit, 120)
        c.t.addTimeInterval(179); XCTAssertFalse(s.expireIfIdle()); XCTAssertTrue(s.isOn)
        c.t.addTimeInterval(2); XCTAssertTrue(s.expireIfIdle()); XCTAssertFalse(s.isOn)
        XCTAssertEqual(told.value, 1, "the bar re-reads; nothing is said")
        XCTAssertFalse(s.expireIfIdle())
    }
    func testTheListenerSpeakingRestartsTheTimerEachTime() {
        let c = TestClock(); let s = make(c); s.set(true)
        c.t.addTimeInterval(170); s.readerSpoke()
        c.t.addTimeInterval(170); XCTAssertFalse(s.expireIfIdle(), "170 s since the turn")
        s.readerSpoke()
        c.t.addTimeInterval(170); XCTAssertFalse(s.expireIfIdle(), "restarted again")
        c.t.addTimeInterval(20); XCTAssertTrue(s.expireIfIdle())
    }
    func testTurnedOffByHandStaysOffAndTurningItOnAgainRestartsTheTimer() {
        let c = TestClock(); let s = make(c); s.set(true)
        s.set(false); c.t.addTimeInterval(1000); XCTAssertFalse(s.isOn)
        s.set(true); c.t.addTimeInterval(100); XCTAssertFalse(s.expireIfIdle()); XCTAssertTrue(s.isOn)
    }

    // ---- no press-and-hold talk (owner, 2026-11-16: "I don't like press and hold"): the bar's Voice control is the one way to talk ----
    // Failure list, written first: H1 a talkDown/talkUp action or message is in a contract; H2 it is in the Exact interface (generator
    // and exact.json); H3 a button there is bound to pointerdown/pointerup; H4 the module or runtime still handles one.
    func testNoPressAndHoldTalkAnywhereInTheApp() throws {
        let root = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent()
        var hits: [String] = []
        let fm = FileManager.default
        for dir in ["AutoradioLab", "exact-package", "AutoradioKit/Sources"] {
            guard let en = fm.enumerator(at: root.appendingPathComponent(dir), includingPropertiesForKeys: nil) else { continue }
            for case let u as URL in en {
                // Build output is not the app's source: Exact's web loop writes generated page-style.json under target/ (seen 2026-11-18).
                guard u.path.contains("/Kit/") == false, u.path.contains("/target/") == false, u.path.contains("/node_modules/") == false,
                      ["contract", "json", "swift"].contains(u.pathExtension),
                      let text = try? String(contentsOf: u, encoding: .utf8) else { continue }
                for (i, line) in text.split(separator: "\n", omittingEmptySubsequences: false).enumerated() {
                    let l = String(line)
                    if l.contains("TalkDown") || l.contains("TalkUp") || l.contains("talkDown") || l.contains("talkUp") || l.contains("pointerdown") || l.contains("lab-talk") {
                        hits.append("\(u.lastPathComponent):\(i + 1)")
                    }
                }
            }
        }
        XCTAssertEqual(hits, [], "press-and-hold talk is still in the app")
    }

    // ---- the engine ----
    func make() throws -> (TurnEngine, StoppablePlayback) {
        let r = StageRegistry.builtIn()
        r.register(.system1, StubS1.self) { StubS1($0) }
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        r.register(.speechBounds, LoudBounds.self) { LoudBounds($0) }
        r.register(.transcriber, ScriptedTranscriber.self) { ScriptedTranscriber($0) }
        r.register(.desk, RecordingDesk.self) { RecordingDesk($0) }
        var cfg = StageRegistry.defaultConfiguration()
        cfg.set(.addressing, .init(id: "open"))
        cfg.set(.speechBounds, .init(id: "loud-bounds"))
        cfg.set(.transcriber, .init(id: "scripted-tr"))
        cfg.set(.system1, .init(id: "stub-s1", params: ["delay": .number(0), "asks": .string("yes")]))
        cfg.set(.voice, .init(id: "stub-voice"))
        cfg.set(.holder, .init(id: "local-ack"))
        cfg.set(.desk, .init(id: "rec-desk"))
        RecordingDesk.turns = []
        ScriptedTranscriber.finals = []; ScriptedTranscriber.sessions = []
        let pb = StoppablePlayback()
        return (TurnEngine(env: EngineEnvironment(registry: r, log: EventLog(clock: LabClock()), playback: pb), configuration: cfg), pb)
    }
    private var t = 0.0
    func frame(loud: Bool) -> AudioFrame {
        defer { t += 0.032 }
        return AudioFrame(samples: [Float](repeating: loud ? 0.5 : 0, count: 512), sampleRate: 16000, t: t)
    }
    func push(_ e: TurnEngine, loud: Bool, _ n: Int) async { for _ in 0..<n { await e.ingestHandsFree(frame(loud: loud)) } }
    func phase(_ e: TurnEngine) async -> TurnPhase { await e.phase }

    // A TV: speech with no words yet shows nothing. Words and enough speech show the state.
    func testListeningShowsOnlyWithWords() async throws {
        let (e, _) = try make()
        await push(e, loud: true, 40)                              // 1.3 s of voice, no words recognised
        var p = await phase(e)
        XCTAssertNotEqual(p, .capturing)
        ScriptedTranscriber.sessions.last?.say("tell me")
        let shown = await poll(5) { await self.push(e, loud: true, 1); return await e.phase == .capturing }
        XCTAssertTrue(shown)
        // the segment ends with no words that count: the state goes away
        await e.handsFreeSet(false); await e.handsFreeSet(true)
        p = await phase(e)
        XCTAssertNotEqual(p, .capturing)
    }

    // T5: Cancel while the speech goes on: the turn is dropped, hands-free stays on, no new turn opens until the speech pauses.
    func testCancelWhileSpeechContinuesDropsTheTurnAndStaysOn() async throws {
        let (e, _) = try make()
        await push(e, loud: true, 5)
        ScriptedTranscriber.sessions.last?.say("tell me about the harbor")
        await push(e, loud: true, 30)
        let shown = await poll(5) { await self.push(e, loud: true, 1); return await e.phase == .capturing }
        XCTAssertTrue(shown)
        let before = ScriptedTranscriber.sessions.count
        await e.cancelTurn()
        var p = await phase(e)
        XCTAssertNotEqual(p, .capturing)
        await push(e, loud: true, 60)                               // the TV talks on
        XCTAssertEqual(ScriptedTranscriber.sessions.count, before, "no new turn while the same speech runs")
        p = await phase(e)
        XCTAssertNotEqual(p, .capturing)
        await push(e, loud: false, 10)                              // a pause
        await push(e, loud: true, 5)                                // new speech: a turn opens again (still on)
        XCTAssertEqual(ScriptedTranscriber.sessions.count, before + 1)
        XCTAssertTrue(RecordingDesk.turns.isEmpty, "nothing was sent")
    }

    // T4: off in the middle of a turn: the turn is dropped at once and nothing more is heard; on again hears.
    func testOffInTheMiddleOfATurnDropsItAndHearsNothing() async throws {
        let (e, _) = try make()
        LocalLanguage.model = FakeLocalModel(pieces: ["One moment."]); defer { LocalLanguage.model = nil }
        ScriptedTranscriber.finals = ["what is on today"]
        await push(e, loud: true, 5)
        ScriptedTranscriber.sessions.last?.say("what is on")
        await push(e, loud: true, 30)
        await e.handsFreeSet(false)
        let p = await phase(e)
        XCTAssertNotEqual(p, .capturing)
        let n = ScriptedTranscriber.sessions.count
        await push(e, loud: true, 20); await push(e, loud: false, 60)
        XCTAssertEqual(ScriptedTranscriber.sessions.count, n, "the microphone is closed: nothing opens")
        try await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertTrue(RecordingDesk.turns.isEmpty)
        await e.handsFreeSet(true)
        await push(e, loud: true, 5)
        XCTAssertEqual(ScriptedTranscriber.sessions.count, n + 1)
    }

    // T6: speech that never goes quiet: the turn is closed by its deadline (30 s) and sent as it stands; not before.
    func testTurnThatNeverGoesQuietCloses() async throws {
        let (e, _) = try make()
        LocalLanguage.model = FakeLocalModel(pieces: ["One moment."]); defer { LocalLanguage.model = nil }
        ScriptedTranscriber.finals = ["the tv went on and on"]
        await push(e, loud: true, 5)
        ScriptedTranscriber.sessions.last?.say("the tv went on")
        await push(e, loud: true, 650)                              // about 21 s
        XCTAssertTrue(RecordingDesk.turns.isEmpty, "no deadline before 30 s")
        await push(e, loud: true, 400)                              // past 30 s
        let sent = await poll(8) { !RecordingDesk.turns.isEmpty }
        XCTAssertTrue(sent, "the deadline sends what it has")
        XCTAssertEqual(RecordingDesk.turns.first?.listenerWords, "the tv went on and on")
    }
}
