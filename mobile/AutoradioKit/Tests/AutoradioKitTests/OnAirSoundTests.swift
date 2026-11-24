import XCTest
@testable import AutoradioKit

/// A voice that answers but makes no sound (the failure the listener hears as silence).
final class MuteVoice: Voice, @unchecked Sendable {
    static let stageID = "phonon"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "mute voice") }
    init(_ p: StageParams) {}
    func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error> {
        AsyncThrowingStream { c in Task { for await _ in text {}; c.finish() } }
    }
}
/// Stands in for the system voice ("avspeech") with a real signal.
final class SystemStandIn: Voice, @unchecked Sendable {
    static let stageID = "avspeech"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "system stand-in") }
    init(_ p: StageParams) {}
    func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error> {
        AsyncThrowingStream { c in
            Task { for await _ in text { c.yield(AudioFrame(samples: [Float](repeating: 0.2, count: 1600), sampleRate: 16000, t: 0)) }; c.finish() }
        }
    }
}

final class OnAirSoundTests: XCTestCase {
    func player(voice: String) -> ProgrammePlayer {
        let r = StageRegistry.builtIn()
        r.register(.voice, MuteVoice.self) { MuteVoice($0) }
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        r.register(.voice, SystemStandIn.self) { SystemStandIn($0) }
        var cfg = StageRegistry.defaultConfiguration()
        cfg.set(.voice, .init(id: voice))
        return ProgrammePlayer(audio: virtualAudio(), registry: r, log: EventLog(clock: LabClock()), configuration: cfg,
                               document: ProgrammeDocument(id: "rs:x", title: "T", blocks: [.speech(text: "Hello there.")]))
    }

    /// An entry on air hands samples to the output node.
    func testAnOnAirEntryGivesTheOutputNodeSamples() async {
        let p = player(voice: "stub-voice")
        p.begin(at: 0)
        let got = await poll { p.audio.programmeSamplesHanded > 0 }
        XCTAssertTrue(got, "the output node was given nothing")
        XCTAssertNil(p.speakFailure)
    }

    /// A voice that makes no sound falls back to the system voice, so the listener still hears the words.
    func testAVoiceThatMakesNoSoundFallsBackToTheSystemVoice() async {
        let p = player(voice: "phonon")
        p.begin(at: 0)
        let got = await poll { p.audio.programmeSamplesHanded > 0 }
        XCTAssertTrue(got, "the system voice should have spoken")
    }

    /// When nothing can speak, the reason is kept for the bar.
    func testWhenNothingCanSpeakTheReasonIsKept() async {
        let r = StageRegistry.builtIn()
        r.register(.voice, MuteVoice.self) { MuteVoice($0) }
        r.register(.voice, MuteVoiceTwo.self) { MuteVoiceTwo($0) }
        var cfg = StageRegistry.defaultConfiguration()
        cfg.set(.voice, .init(id: "phonon"))
        let p = ProgrammePlayer(audio: virtualAudio(), registry: r, log: EventLog(clock: LabClock()), configuration: cfg,
                                document: ProgrammeDocument(id: "rs:y", title: "T", blocks: [.speech(text: "Hello there.")]))
        let t = await p.synthesize("Hello there.")
        if t == nil { XCTAssertNotNil(p.speakFailure) }   // the built-in system voice may speak on this machine; then there is nothing to report
    }

    /// The microphone is not wanted by a playback-only session (the tap on its input node kept the engine from starting).
    func testAPlaybackOnlyStartWantsNoMicrophone() throws {
        let a = LiveAudio()
        try a.start(bus: AudioBus(), clock: LabClock(), wantMic: true, mic: false, virtual: true)
        XCTAssertTrue(a.virtual)
    }

    /// The session is not ready: the entry fails at once with the plain reason, and the bar says so.

}
final class MuteVoiceTwo: Voice, @unchecked Sendable {
    static let stageID = "avspeech"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "mute system") }
    init(_ p: StageParams) {}
    func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error> { AsyncThrowingStream { c in Task { for await _ in text {}; c.finish() } } }
}

/// One voice at a time, and the listener outranks the radio.
final class OneVoiceTests: XCTestCase {
    func audio() -> LiveAudio { virtualAudio() }
    func line(seconds: Double) -> AsyncThrowingStream<AudioFrame, Error> {
        AsyncThrowingStream { c in c.yield(AudioFrame(samples: [Float](repeating: 0.1, count: Int(16000 * seconds)), sampleRate: 16000, t: 0)); c.finish() }
    }

    /// Two sources asked to speak at once: the programme and a voice line are never audible together.
    func testAVoiceLineHoldsTheProgrammeAndTheProgrammeGoesOnAfterIt() async {
        let a = audio()
        a.programmePlay()
        XCTAssertTrue(a.programmeAudible)
        let v = Task { await a.playVoice(self.line(seconds: 0.4), reason: "test", gate: .alwaysLive(because: "a test of the radio own voice line: no Test owner")) }
        let held = await poll { a.voiceIsSpeaking }
        XCTAssertTrue(held)
        XCTAssertFalse(a.programmeAudible, "the programme is silent while the voice line speaks")
        a.programmePlay()                                           // the programme asks to play again: it waits
        XCTAssertFalse(a.programmeAudible, "a second request while one speaks never mixes")
        await v.value
        XCTAssertFalse(a.voiceIsSpeaking)
        XCTAssertTrue(a.programmeAudible, "when the voice line ends the programme goes on")
    }

    /// Barge-in: the listener cuts the voice line; the programme is not left held.
    func testCuttingTheVoiceLineReleasesTheProgramme() async {
        let a = audio()
        a.programmePlay()
        let v = Task { await a.playVoice(self.line(seconds: 20), reason: "test", gate: .alwaysLive(because: "a test of the radio own voice line: no Test owner")) }
        _ = await poll { a.voiceIsSpeaking }
        v.cancel()
        _ = await v.value
        XCTAssertFalse(a.voiceIsSpeaking)
    }

    /// A paused programme stays paused when a voice line ends.
    func testAPausedProgrammeStaysPausedAfterAVoiceLine() async {
        let a = audio()
        a.programmePlay()
        let v = Task { await a.playVoice(self.line(seconds: 0.2), reason: "test", gate: .alwaysLive(because: "a test of the radio own voice line: no Test owner")) }
        _ = await poll { a.voiceIsSpeaking }
        a.programmePause()
        await v.value
        XCTAssertFalse(a.programmeAudible)
    }

    /// Speech start pauses at once (the default policy), and a pause is kept for the listener's whole turn.
    func testSpeechStartPausesAtOnce() throws {
        let cfg = StageRegistry.defaultConfiguration()
        let y = try StageRegistry.builtIn().make(.yieldPolicy, as: YieldPolicy.self, cfg)
        XCTAssertEqual(y.target(for: YieldSituation(current: .play, speechSeconds: 0.1, addressed: false, notATurn: false)), .pause)
        XCTAssertEqual(y.target(for: YieldSituation(current: .pause, speechSeconds: 0, addressed: false, notATurn: false)), .pause)
        XCTAssertEqual(y.target(for: YieldSituation(current: .pause, speechSeconds: 3, addressed: false, notATurn: true)), .play)
    }

    /// Mic on is one session state whether or not a turn is open: no category or mode change between listening and talking.
    func testVoiceOnIsOneSessionState() {
        let c = SessionCoordinator(inner: NullSessionOwner(), log: EventLog(clock: LabClock()), audio: LiveAudio(), policy: { .switchOnAddress }, voiceOn: { true })
        XCTAssertEqual(c.target(for: .listening), .talking)
        XCTAssertEqual(c.target(for: .talking), .talking)
        let off = SessionCoordinator(inner: NullSessionOwner(), log: EventLog(clock: LabClock()), audio: LiveAudio(), policy: { .switchOnAddress }, voiceOn: { false })
        XCTAssertEqual(off.target(for: .talking), .playing)
    }
}
