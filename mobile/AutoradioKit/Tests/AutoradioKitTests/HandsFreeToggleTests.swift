import XCTest
@testable import AutoradioKit

// Hands-free as one on/off state (HandsFreeSwitch.swift: T1 to T7) and when the listening state shows. The engine tests push frames as
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

    // ---- T1, T2, T3, T8, T9: when it is on (owner, 2026-11-15) ----
    final class TestClock: @unchecked Sendable { var t = Date(timeIntervalSince1970: 1_000_000) }
    func make(_ c: TestClock) -> HandsFreeSwitch { HandsFreeSwitch(now: { c.t }) }
    func testFreshLaunchIsUndecidedAndOff() {
        let s = HandsFreeSwitch(); XCTAssertFalse(s.decided); XCTAssertFalse(s.isOn)
    }
    func testAppOpenWithMicrophoneTurnsItOnAndDeclinedLeavesItOff() {
        let c = TestClock(); let a = make(c), b = make(c)
        a.appOpened(micGranted: true); b.appOpened(micGranted: false)
        XCTAssertTrue(a.isOn); XCTAssertFalse(b.isOn)
        b.decideFirstRun(micGranted: true)
        XCTAssertFalse(b.isOn, "a later grant turns nothing on until the app is opened again")
    }
    func testNothingIsKeptAcrossLaunches() {
        let c = TestClock(); let a = make(c); a.appOpened(micGranted: true); a.set(false)
        XCTAssertFalse(make(c).isOn, "a new launch starts undecided, whatever the last one did")
    }
    func testThreeMinutesWithNoTurnTurnsItOffSilently() {
        let c = TestClock(); let s = make(c); s.appOpened(micGranted: true)
        let told = Locked<Int>(0)
        s.onChange = { told.write { $0 += 1 } }
        c.t.addTimeInterval(179); XCTAssertFalse(s.expireIfIdle()); XCTAssertTrue(s.isOn)
        c.t.addTimeInterval(2); XCTAssertTrue(s.expireIfIdle()); XCTAssertFalse(s.isOn)
        XCTAssertEqual(told.value, 1, "the bar re-reads; nothing is said")
        XCTAssertFalse(s.expireIfIdle())
    }
    func testATurnResetsTheTimerAndPlayingOrPausingDoesNot() {
        let c = TestClock(); let s = make(c); s.appOpened(micGranted: true)
        c.t.addTimeInterval(170); s.readerSpoke()
        c.t.addTimeInterval(170); XCTAssertFalse(s.expireIfIdle(), "170 s since the turn")
        // Playing or pausing never reaches the switch: there is no call for it, so only a turn can reset it.
        c.t.addTimeInterval(20); XCTAssertTrue(s.expireIfIdle())
    }
    func testTurnedOffByHandStaysOffUntilTheNextAppOpen() {
        let c = TestClock(); let s = make(c); s.appOpened(micGranted: true)
        s.set(false); c.t.addTimeInterval(10); XCTAssertFalse(s.isOn)
        s.decideFirstRun(micGranted: true); XCTAssertFalse(s.isOn)
        s.appOpened(micGranted: true); XCTAssertTrue(s.isOn, "a new foreground session")
    }
    func testTheMicButtonTurnsItOnAtAnyTimeAndRestartsTheTimer() {
        let c = TestClock(); let s = make(c); s.appOpened(micGranted: false)
        c.t.addTimeInterval(1000); s.set(true); c.t.addTimeInterval(100)
        XCTAssertFalse(s.expireIfIdle()); XCTAssertTrue(s.isOn)
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
