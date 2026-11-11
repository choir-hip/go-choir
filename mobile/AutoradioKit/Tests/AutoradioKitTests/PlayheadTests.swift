import XCTest
@testable import AutoradioKit

// Engine round 2: playback rate and a real position, `DeskTurn.interrupted`, `speakText`.
// Written from the failure lists at the top of Playhead.swift (position, cut text), and these for the rest:
//
// Rate:
//  R1. The speed button changes only a number and the audio stays at 1x: the virtual engine must take less wall time at 2x
//      and more at 0.5x, and the unit that does the work is the time-pitch node (real hardware path, phone only).
//  R2. A rate set mid-block must apply to the rest of the block, and the position (source seconds) must not jump.
//  R3. The tracker (the bar's progress, the sentence) must read the audio position when one exists, at any rate, and must
//      not use wall time; a block that has no audio yet is at 0, not at the old block's position.
//  R4. The speed picked in the player must reach the audio (speedSink), clamped to 0.5...2.
//
// Interrupted:
//  I1. The engine decides "interrupted" late (after the yield has paused the programme): it must ask at the moment the turn
//      begins, before anything yields. Test: the answer changes after the button goes down; the turn keeps the first.
//  I2. Nothing was speaking (paused, clip, nothing): interrupted is false and the spoken text empty.
//  I3. The interview reads the engine's answer, not a timing guess: with no information it is not interrupted.
//  I4. The cut line keeps the words heard, and the saved document shows only what was heard.
//
// speakText:
//  S1. Empty or markup-only text speaks nothing and changes nothing.
//  S2. A story is playing: the text waits behind it, in order, once per address, and plays when the story ends.
//  S3. Nothing is playing: it plays at once, as a programme of its own.
//  S4. The engine is not running: it becomes the loaded programme and does not play (it would be skipped).
//  S5. Skipping past the last block of a story goes to the queued text.
//  S6. Mail's request reaches the handler, and with no handler nothing breaks.

// MARK: Stubs

/// A voice that produces `seconds` of silence per line in one frame.
final class LongVoice: Voice, @unchecked Sendable {
    static let stageID = "long-voice"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "long voice") }
    init(_ p: StageParams) {}
    func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error> {
        AsyncThrowingStream { c in
            let t = Task {
                for await _ in text { c.yield(AudioFrame(samples: [Float](repeating: 0, count: 16000 * 3), sampleRate: 16000, t: 0)) }
                c.finish()
            }
            c.onTermination = { _ in t.cancel() }
        }
    }
}
final class TextTranscriber: Transcriber, @unchecked Sendable {
    static let stageID = "text-tr"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "text tr") }
    init(_ p: StageParams) {}
    func begin() async throws -> TranscriptionSession { Session() }
    final class Session: TranscriptionSession, @unchecked Sendable {
        let partials = AsyncStream<String> { $0.finish() }
        func append(_ frame: AudioFrame) {}
        func finish() async throws -> String { "actually, tech" }
        func cancel() {}
    }
}
final class RecordingDesk: Desk, @unchecked Sendable {
    static let stageID = "rec-desk"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "rec desk") }
    nonisolated(unsafe) static var turns: [DeskTurn] = []
    init(_ p: StageParams) {}
    func submit(_ turn: DeskTurn) -> AsyncThrowingStream<Revision, Error> {
        Self.turns.append(turn)
        return AsyncThrowingStream { $0.finish() }
    }
}
final class CutPlayback: PlaybackControl, @unchecked Sendable {
    var cut: SpokenCut?
    func position() async -> ProgramPosition { ProgramPosition(programmeID: "p", blockIndex: 2, context: "ctx") }
    func setLevel(_ level: PlayLevel) async { cut = nil }          // the programme yields: nothing is being said any more
    func speak(_ frames: AsyncThrowingStream<AudioFrame, Error>) async { do { for try await _ in frames {} } catch {} }
    func apply(_ revision: Revision) async {}
    func resume() async {}
    func spoken() async -> SpokenCut? { cut }
}

func virtualAudio() -> LiveAudio {
    let a = LiveAudio()
    try? a.start(bus: AudioBus(), clock: LabClock(), wantMic: false, virtual: true)
    return a
}
func poll(_ seconds: Double = 6, _ ok: () async -> Bool) async -> Bool {
    let end = Date().addingTimeInterval(seconds)
    while Date() < end { if await ok() { return true }; try? await Task.sleep(nanoseconds: 25_000_000) }
    return await ok()
}

// MARK: Pure pieces

final class PlayheadTests: XCTestCase {
    func testCutTextIsWholeWordsOnly() {
        let t = "The harbor council voted on Tuesday to extend ferry service"
        XCTAssertEqual(SpeechCut.text(of: t, upTo: 0), "")
        XCTAssertEqual(SpeechCut.text(of: t, upTo: 1), t)
        XCTAssertEqual(SpeechCut.text(of: t, upTo: 7), t, "over one clamps")
        XCTAssertEqual(SpeechCut.text(of: t, upTo: -3), "")
        XCTAssertEqual(SpeechCut.text(of: t, upTo: .nan), "")
        XCTAssertEqual(SpeechCut.text(of: t, upTo: 0.5), "The harbor council voted on", "Tuesday was only started")
        XCTAssertEqual(SpeechCut.text(of: t, upTo: 0.47), "The harbor council voted on", "cut exactly at a space")
        XCTAssertEqual(SpeechCut.text(of: t, upTo: 0.05), "", "inside the first word nothing was heard")
        for k in 0...100 {
            let s = SpeechCut.text(of: t, upTo: Double(k) / 100)
            XCTAssertTrue(t.hasPrefix(s))
            if !s.isEmpty, s != t { XCTAssertTrue(t.dropFirst(s.count).first == " ", "never mid-word at \(k): '\(s)'") }
            XCTAssertEqual(s, s.trimmingCharacters(in: .whitespaces))
        }
        XCTAssertEqual(SpeechCut.text(of: "", upTo: 0.5), "")
        XCTAssertEqual(SpeechCut.text(of: "Supercalifragilistic", upTo: 0.6), "")
        XCTAssertEqual(SpeechCut.text(of: "café 👍 déjà vu again", upTo: 0.5), "café 👍")
    }

    func testPlayheadIsRelativeToTheScheduleNotTheNodeClock() {
        var h = Playhead(tag: 1, totalFrames: 88_200)           // 2 s
        h.reschedule(from: 0, base: 441_000)                     // the node already stood at 10 s (earlier blocks)
        XCTAssertEqual(h.seconds(rendered: 441_000), 0, accuracy: 1e-9)
        XCTAssertEqual(h.seconds(rendered: 441_000 + 22_050), 0.5, accuracy: 1e-9)
        XCTAssertEqual(h.fraction(rendered: 441_000 + 44_100), 0.5, accuracy: 1e-9)
        XCTAssertEqual(h.seconds(rendered: 441_000 + 22_050), h.seconds(rendered: 441_000 + 22_050), "a paused node reports the same value")
        XCTAssertEqual(h.frames(rendered: 100), 0, "a render value before the base clamps at the start")
        XCTAssertEqual(h.fraction(rendered: 441_000 + 900_000), 1, accuracy: 1e-9, "never past the block")
        XCTAssertEqual(Playhead(tag: 0, totalFrames: 0).fraction(rendered: 5), 0)
    }

    func testPlayheadSurvivesAnEngineRestart() {
        var h = Playhead(tag: 1, totalFrames: 88_200)
        h.reschedule(from: 0, base: 0)
        h.freeze(rendered: 30_000)                               // the engine stops 30000 frames in
        XCTAssertEqual(h.frames(rendered: 0), 30_000, "the node forgot its time; the position did not")
        XCTAssertEqual(h.frames(rendered: 77), 30_000)
        h.freeze(rendered: 0)                                    // a second stop while down changes nothing
        XCTAssertEqual(h.frames(rendered: 0), 30_000)
        h.reschedule(from: 30_000, base: 0)                      // restarted: new node clock from 0
        XCTAssertEqual(h.frames(rendered: 10_000), 40_000)
    }
}

// MARK: Rate and render position, on the virtual engine

final class RateAndPositionTests: XCTestCase {
    func testRateChangesHowLongABlockTakesAndPitchIsNotTheEnginesBusiness() async {
        let a = virtualAudio(); a.programmePlay()
        let one = [Float](repeating: 0, count: 16_000)           // 1 s of audio
        a.setProgrammeRate(1)
        var t0 = Date(); await a.playProgramme(one, rate: 16_000, tag: 3)
        let normal = Date().timeIntervalSince(t0)
        XCTAssertGreaterThan(normal, 0.95)
        a.setProgrammeRate(2)
        t0 = Date(); await a.playProgramme(one, rate: 16_000, tag: 4)
        let fast = Date().timeIntervalSince(t0)
        XCTAssertLessThan(fast, 0.8); XCTAssertGreaterThan(fast, 0.4)
        a.setProgrammeRate(0.5)
        t0 = Date(); await a.playProgramme([Float](repeating: 0, count: 8_000), rate: 16_000, tag: 5)   // 0.5 s of audio
        XCTAssertGreaterThan(Date().timeIntervalSince(t0), 0.9, "half speed takes twice as long")
        let p = a.programmePosition(tag: 5)
        XCTAssertEqual(p?.fraction ?? 0, 1, accuracy: 0.001)
        XCTAssertEqual(p?.total ?? 0, 0.5, accuracy: 0.01, "the block's length is source seconds, whatever the rate")
        XCTAssertNil(a.programmePosition(tag: 3), "another block's position never answers")
    }

    func testRateIsClamped() {
        let a = virtualAudio()
        a.setProgrammeRate(9); XCTAssertEqual(a.programmeRate, 2)
        a.setProgrammeRate(0.01); XCTAssertEqual(a.programmeRate, 0.5)
        a.setProgrammeRate(.nan); XCTAssertEqual(a.programmeRate, 1)
    }

    func testPositionHoldsWhilePausedAndFollowsARateChange() async {
        let a = virtualAudio(); a.programmePlay(); a.setProgrammeRate(1)
        let two = [Float](repeating: 0, count: 32_000)           // 2 s
        let t0 = Date()
        let task = Task { await a.playProgramme(two, rate: 16_000, tag: 7) }
        try? await Task.sleep(nanoseconds: 500_000_000)
        let early = a.programmePosition(tag: 7)?.seconds ?? -1
        XCTAssertEqual(early, 0.5, accuracy: 0.25)
        a.programmePause()
        try? await Task.sleep(nanoseconds: 150_000_000)
        let p1 = a.programmePosition(tag: 7)?.seconds ?? -1
        try? await Task.sleep(nanoseconds: 400_000_000)
        let p2 = a.programmePosition(tag: 7)?.seconds ?? -2
        XCTAssertEqual(p1, p2, accuracy: 0.001, "paused: the position does not creep")
        a.setProgrammeRate(2); a.programmePlay()                 // the rest at double speed
        await task.value
        let total = Date().timeIntervalSince(t0)
        XCTAssertLessThan(total, 2.25, "0.5 s + 0.55 s paused + about 0.75 s at double speed; at 1x it would be 2.55 s or more")
        XCTAssertEqual(a.programmePosition(tag: 7)?.fraction ?? 0, 1, accuracy: 0.001)
    }

    func testTrackerReadsTheAudioPositionAtAnyRate() {
        let tr = BarTracker()
        var snap = LabSnapshot(); snap.running = true; snap.paused = false; snap.phase = "playing"; snap.level = "play"; snap.blockIndex = 1; snap.blockCount = 2
        let doc = ProgrammeDocument(id: "d", title: "T", blocks: [.speech(text: "One."), .speech(text: "First sentence here. Second one follows it. Third closes.")])
        var player = ChoirPlayerView(); player.playing = true
        let box = PosBox(); box.value = RenderPosition(index: 1, fraction: 0.5, seconds: 5, total: 10)
        tr.positionSource = { box.value }
        let now = Date(timeIntervalSince1970: 5000)
        tr.setSpeed(1.5)
        var info = tr.radioInfo(player: player, snapshot: snap, document: doc, route: "r", now: now)
        XCTAssertEqual(info.blockProgress, 0.5, accuracy: 1e-9, "from the audio, not from wall time")
        XCTAssertEqual(info.sentenceIndex, 1)
        let later = tr.radioInfo(player: player, snapshot: snap, document: doc, route: "r", now: now.addingTimeInterval(300))
        XCTAssertEqual(later.blockProgress, 0.5, accuracy: 1e-9, "300 s of wall time moved nothing: the audio did not move")
        box.value = RenderPosition(index: 1, fraction: 0.95, seconds: 9.5, total: 10)
        info = tr.radioInfo(player: player, snapshot: snap, document: doc, route: "r", now: now)
        XCTAssertEqual(info.sentenceIndex, 2)
        box.value = RenderPosition(index: 0, fraction: 0.9, seconds: 1, total: 1)
        info = tr.radioInfo(player: player, snapshot: snap, document: doc, route: "r", now: now)
        XCTAssertEqual(info.blockProgress, 0, "the old block's position does not answer for the new one")
        box.value = nil
        XCTAssertEqual(tr.radioInfo(player: player, snapshot: snap, document: doc, route: "r", now: now).blockProgress, 0)
        XCTAssertEqual(tr.barInfo(player: player, snapshot: snap, now: now).progress, 0.5, accuracy: 1e-9, "block 1 of 2 begun, nothing of it played")
    }
    final class PosBox: @unchecked Sendable { var value: RenderPosition? }

    func testSpeedReachesTheAudio() {
        let tr = BarTracker()
        let got = PosBox2()
        tr.speedSink = { got.values.append($0) }
        tr.setSpeed(1.25); tr.setSpeed(9); tr.setSpeed(0.1)
        XCTAssertEqual(got.values, [1.25, 2, 0.5])
        XCTAssertEqual(tr.speed, 0.5)
    }
    final class PosBox2: @unchecked Sendable { var values: [Double] = [] }
}

// MARK: The player: position of speech, the cut, speakText

final class PlayerEngineTests: XCTestCase {
    func makePlayer(voice: String = "long-voice", blocks: [String] = ["One two three four five six seven eight nine ten.", "Second block."]) -> ProgrammePlayer {
        let r = StageRegistry.builtIn()
        r.register(.voice, LongVoice.self) { LongVoice($0) }
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        var cfg = StageRegistry.defaultConfiguration()
        cfg.set(.voice, .init(id: voice))
        return ProgrammePlayer(audio: virtualAudio(), registry: r, log: EventLog(clock: LabClock()), configuration: cfg,
                               document: ProgrammeDocument(id: "story", title: "Story", blocks: blocks.map { .speech(text: $0) }))
    }

    func testPlayerReportsWhatWasHeardOfTheLineBeingSpoken() async {
        let p = makePlayer(blocks: ["One two three four five six seven eight nine ten."])
        let none = await p.spoken()
        XCTAssertNil(none, "I2: nothing is playing")
        p.play()
        let got = await poll { await p.spoken() != nil }
        XCTAssertTrue(got)
        let cut = await p.spoken()
        XCTAssertEqual(cut?.full, "One two three four five six seven eight nine ten.")
        XCTAssertEqual(cut?.blockIndex, 0)
        XCTAssertTrue(cut!.full.hasPrefix(cut!.spoken))
        XCTAssertLessThan(cut!.fraction, 1)
        let pos = p.renderPosition()
        XCTAssertEqual(pos?.index, 0)
        p.setRate(2)
        XCTAssertEqual(p.rate, 2); XCTAssertEqual(p.audio.programmeRate, 2)
        await p.setLevel(.pause)
        let paused = await p.spoken()
        XCTAssertNil(paused, "I2: a paused programme was not cut into")
        await p.setLevel(.play)
        p.pause()
        let userPaused = await p.spoken()
        XCTAssertNil(userPaused)
    }

    func testSpokenFractionAdvancesAndEndsWithTheLine() async {
        let p = makePlayer(blocks: ["a b c d e f g h"])
        p.setRate(2)                                              // 3 s of voice takes 1.5 s
        p.play()
        _ = await poll { await p.spoken() != nil }
        let first = (await p.spoken())?.fraction ?? 1
        try? await Task.sleep(nanoseconds: 500_000_000)
        let later = (await p.spoken())?.fraction ?? 1
        XCTAssertGreaterThan(later, first, "the position moves with the audio")
        XCTAssertGreaterThan(later - first, 0.2, "at 2x half a second is a third of a 3 s line")
        let ended = await poll(4) { await p.spoken() == nil }
        XCTAssertTrue(ended, "once the line has finished there is nothing to cut into")
    }

    // S1
    func testEmptyTextSpeaksNothing() {
        let p = makePlayer()
        XCTAssertFalse(p.speakText(address: "mmsg:1", text: "   \n\n  "))
        XCTAssertEqual(p.document.id, "story"); XCTAssertTrue(p.userPaused); XCTAssertTrue(p.queuedTexts.isEmpty)
    }

    // S3
    func testIdlePlayerSpeaksTextAtOnceAsAProgrammeOfItsOwn() async {
        let p = makePlayer(blocks: [])
        XCTAssertTrue(p.speakText(address: "mmsg:7", text: "Hello from the harbor office.\n\nThe second paragraph."))
        XCTAssertEqual(p.document.id, "text:mmsg:7")
        XCTAssertEqual(p.document.blocks.count, 2)
        XCTAssertEqual(p.document.title, "Hello from the harbor office.")
        XCTAssertFalse(p.userPaused)
        let reached = await poll { p.blockIndex == 1 }
        XCTAssertTrue(reached, "it plays through its paragraphs")
        let cut = await poll { await p.spoken()?.full == "The second paragraph." }
        XCTAssertTrue(cut, "I1: and can be cut into like any speech")
    }

    // S2
    func testTextQueuesBehindTheStoryInOrderOncePerAddress() async {
        let p = makePlayer(voice: "stub-voice", blocks: ["Story one.", "Story two."])
        p.setRate(2)
        p.play()
        XCTAssertTrue(p.speakText(address: "a", text: "First message."))
        XCTAssertTrue(p.speakText(address: "b", text: "Second message."))
        XCTAssertTrue(p.speakText(address: "a", text: "First message again."))
        XCTAssertEqual(p.queuedTexts, ["text:a", "text:b"], "asked twice: once; in order")
        XCTAssertEqual(p.document.id, "story", "the story is not interrupted")
        let a = await poll(10) { p.document.id == "text:a" }
        XCTAssertTrue(a, "when the story ends, the first message plays")
        XCTAssertEqual(p.document.blocks.first?.spokenText, "First message again.")
        let b = await poll(10) { p.document.id == "text:b" }
        XCTAssertTrue(b)
        XCTAssertTrue(p.queuedTexts.isEmpty)
    }

    // S4
    func testNotRunningMakesItTheLoadedProgrammeWithoutPlaying() {
        let p = makePlayer()
        XCTAssertTrue(p.speakText(address: "m", text: "Read me.", startNow: true, play: false))
        XCTAssertEqual(p.document.id, "text:m"); XCTAssertTrue(p.userPaused); XCTAssertEqual(p.blockIndex, 0)
    }

    // S5
    func testSkippingPastTheLastBlockGoesToTheQueuedText() async {
        let p = makePlayer(blocks: ["Only block."])
        p.play()
        _ = p.speakText(address: "n", text: "Queued.")
        XCTAssertEqual(p.queuedTexts, ["text:n"])
        p.next()
        XCTAssertEqual(p.document.id, "text:n")
        XCTAssertTrue(p.queuedTexts.isEmpty)
    }

    // S6
    func testMailRequestReachesTheHandlerAndIsSafeWithoutOne() {
        let m = MailReading()
        m.request(address: "x", text: "no handler")
        XCTAssertEqual(m.last?.text, "no handler")
        let box = PosBox3()
        m.handler = { a, t in box.got = (a, t) }
        m.request(address: "mmsg:2", text: "body")
        XCTAssertEqual(box.got?.0, "mmsg:2"); XCTAssertEqual(box.got?.1, "body")
    }
    final class PosBox3: @unchecked Sendable { var got: (String, String)? }
}

// MARK: The turn: what the listener cut into

final class InterruptedTurnTests: XCTestCase {
    func run(cut: SpokenCut?) async throws -> DeskTurn {
        let r = StageRegistry.builtIn()
        r.register(.system1, StubS1.self) { StubS1($0) }
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        r.register(.transcriber, TextTranscriber.self) { TextTranscriber($0) }
        r.register(.desk, RecordingDesk.self) { RecordingDesk($0) }
        var cfg = StageRegistry.defaultConfiguration()
        cfg.set(.addressing, .init(id: "button"))
        cfg.set(.system1, .init(id: "stub-s1", params: ["delay": .number(0), "asks": .string("yes")]))
        cfg.set(.voice, .init(id: "stub-voice")); cfg.set(.transcriber, .init(id: "text-tr"))
        cfg.set(.holder, .init(id: "fixed-lines", params: ["maxLines": .number(0)]))
        cfg.set(.desk, .init(id: "rec-desk"))
        RecordingDesk.turns = []
        let pb = CutPlayback(); pb.cut = cut
        let engine = TurnEngine(env: EngineEnvironment(registry: r, log: EventLog(clock: LabClock()), playback: pb), configuration: cfg)
        await engine.buttonDown()
        pb.cut = nil                                              // the programme yields after the button: too late to ask
        await engine.buttonUp(held: 1.0)
        await engine.waitUntilIdle()
        return try XCTUnwrap(RecordingDesk.turns.last, "the desk got the turn")
    }

    // I1
    func testTurnCarriesWhatTheListenerCutInto() async throws {
        let t = try await run(cut: SpokenCut(blockIndex: 2, full: "Today's paper has stories on tech, sports and local news.", spoken: "Today's paper has stories on tech,", fraction: 0.6))
        XCTAssertTrue(t.interrupted)
        XCTAssertEqual(t.spokenText, "Today's paper has stories on tech,")
        XCTAssertEqual(t.cutBlockIndex, 2)
        XCTAssertEqual(t.listenerWords, "actually, tech")
    }
    // I2
    func testTurnOverSilenceIsNotInterrupted() async throws {
        let t = try await run(cut: nil)
        XCTAssertFalse(t.interrupted); XCTAssertEqual(t.spokenText, ""); XCTAssertNil(t.cutBlockIndex)
    }

    func testDeskTurnDecodesAnOldRecordWithoutTheNewFields() throws {
        let old = #"{"turnID":"t","programmeID":"p","blockIndex":1,"listenerWords":"hi"}"#
        let t = try JSONDecoder().decode(DeskTurn.self, from: Data(old.utf8))
        XCTAssertFalse(t.interrupted); XCTAssertEqual(t.spokenText, "")
        let round = try JSONDecoder().decode(DeskTurn.self, from: JSONEncoder().encode(DeskTurn(turnID: "t", programmeID: "p", blockIndex: 1, listenerWords: "x", interrupted: true, spokenText: "ab", cutBlockIndex: 3)))
        XCTAssertTrue(round.interrupted); XCTAssertEqual(round.cutBlockIndex, 3)
    }

    func testChoirDeskPromptTellsWhatWasHeard() {
        let playing = PlayingProgramme(programmeID: "p", title: "Harbor", route: "r", publicationVersionID: "", manifestHash: "", blocks: ["Para one.", "Para two is long."])
        let plain = ChoirDeskStage.prompt(DeskTurn(turnID: "t", programmeID: "p", blockIndex: 1, listenerWords: "more"), playing: playing, include: true)
        XCTAssertFalse(plain.contains("cut in"))
        let cut = ChoirDeskStage.prompt(DeskTurn(turnID: "t", programmeID: "p", blockIndex: 1, listenerWords: "more", interrupted: true, spokenText: "Para two"), playing: playing, include: true)
        XCTAssertTrue(cut.contains("cut in on the radio") && cut.contains("heard only: \"Para two\""), cut)
    }

    // I3, I4
    func testInterviewUsesTheEnginesAnswerNotATimingGuess() async throws {
        let session = InterviewSession(interview: Interview(), file: nil)
        _ = session.start()
        _ = await session.hear("the news")
        let l = await session.hear("actually, tech")             // no information: not interrupted, however soon it came
        XCTAssertFalse(session.snapshot.state.turns.contains { $0.interrupted })
        XCTAssertFalse(l.isEmpty)

        let s2 = InterviewSession(interview: Interview(), file: nil)
        _ = s2.start()
        _ = await s2.hear("the news")
        _ = await s2.hear("actually, tech", interrupted: true, heard: "Today's paper has")
        let cutTurn = s2.snapshot.state.turns.last { $0.who == "radio" && $0.interrupted }
        XCTAssertEqual(cutTurn?.heard, "Today's paper has")
        let md = s2.snapshot.documentMarkdown(title: "t")
        XCTAssertTrue(md.contains("Today's paper has …"), "the saved document shows what was heard")
        XCTAssertTrue(md.contains("(cut off)"))
    }
}
