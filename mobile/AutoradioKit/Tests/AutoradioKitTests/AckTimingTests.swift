import XCTest
@testable import AutoradioKit

/// Stub stages for the acknowledgement timing: System 1 takes `s1Delay` and answers `asks`; the voice makes audio
/// for each line at once; the desk answers after `deskDelay`. No network, no audio hardware.
final class StubS1: System1, @unchecked Sendable {
    static let stageID = "stub-s1"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "stub", params: [.number("delay", "d", 0.6, 0, 5), .text("asks", "a", "yes")]) }
    let delay: Double, asks: String
    init(_ p: StageParams) { delay = p.double("delay", 0.6); asks = p.string("asks", "yes") }
    func answer(_ i: System1Input) async throws -> [System1Answer] {
        try? await Task.sleep(nanoseconds: UInt64(delay * 1e9))
        return [System1Answer(questionID: "asks_for_something", option: asks, score: 1, ms: delay * 1000)]
    }
}
final class StubVoice: Voice, @unchecked Sendable {
    static let stageID = "stub-voice"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "stub voice") }
    init(_ p: StageParams) {}
    func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error> {
        AsyncThrowingStream { c in
            let t = Task {
                for await _ in text {
                    // a line of 0.2 s of audio, delivered in four frames; a line in flight is always finished
                    for k in 0..<4 {
                        c.yield(AudioFrame(samples: [Float](repeating: 0.1, count: 800), sampleRate: 16000, t: Double(k) * 0.05))
                        try? await Task.sleep(nanoseconds: 50_000_000)
                    }
                }
                c.finish()
            }
            c.onTermination = { _ in t.cancel() }
        }
    }
}
final class StubTranscriber: Transcriber, @unchecked Sendable {
    static let stageID = "stub-tr"
    static var descriptor: StageDescriptor { StageDescriptor(displayName: "stub tr") }
    init(_ p: StageParams) {}
    func begin() async throws -> TranscriptionSession { EmptyTranscription() }
}

final class AckTimingTests: XCTestCase {
    struct Run { let record: TurnRecord; let events: [TimedEvent]; let log: EventLog }

    func run(ack: AckTiming, asks: String, s1Delay: Double = 0.6, deskDelay: Double = 1.5) async throws -> Run {
        let r = StageRegistry.builtIn()
        r.register(.system1, StubS1.self) { StubS1($0) }
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        r.register(.transcriber, StubTranscriber.self) { StubTranscriber($0) }
        var cfg = StageRegistry.defaultConfiguration()
        cfg.ackTiming = ack
        cfg.set(.system1, .init(id: "stub-s1", params: ["delay": .number(s1Delay), "asks": .string(asks)]))
        cfg.set(.voice, .init(id: "stub-voice"))
        cfg.set(.transcriber, .init(id: "stub-tr"))
        cfg.set(.holder, .init(id: "fixed-lines", params: ["maxLines": .number(0)]))
        cfg.set(.desk, .init(id: "stub-echo", params: ["delaySeconds": .number(deskDelay)]))
        let log = EventLog(clock: LabClock())
        let pb = SimulatedPlayback(position: ProgramPosition(programmeID: "p", blockIndex: 0, context: "ctx"))
        let got = Box()
        let env = EngineEnvironment(registry: r, log: log, playback: pb, onRecord: { got.set($0) })
        let engine = TurnEngine(env: env, configuration: cfg)
        let t = TurnEngine.Turn(id: "t1", config: cfg, transcriber: try r.make(.transcriber, as: Transcriber.self, cfg),
                                yield: try r.make(.yieldPolicy, as: YieldPolicy.self, cfg), session: EmptyTranscription(), firstEventIndex: 0)
        log.emit(.turnEnded)
        await engine.runPipeline(t, transcript: "tell me more", endedAt: log.clock.now)
        let rec = try XCTUnwrap(got.value)
        return Run(record: rec, events: log.events, log: log)
    }
    final class Box: @unchecked Sendable { var value: TurnRecord?; func set(_ r: TurnRecord) { value = r } }

    func end(_ r: Run, _ stage: String) -> Double? { r.record.summary.timings.first { $0.stage == stage }?.endedAt }

    func testImmediateAckStartsBeforeSystem1Ends() async throws {
        let r = try await run(ack: .immediate, asks: "yes")
        let first = try XCTUnwrap(end(r, "voice.firstAudio")), s1 = try XCTUnwrap(end(r, "system1"))
        XCTAssertLessThan(first, s1, "the acknowledgement must start before System 1 has answered")
        XCTAssertLessThan(r.record.summary.turnEndToAckMs ?? 9999, 300)
        XCTAssertEqual(r.record.summary.holdingLines, ["Got it.", "Let me look into that."], "the answers select the follow-up")
        XCTAssertNil(r.record.summary.ackCancelled)
    }

    func testAfterSystem1WaitsForTheAnswers() async throws {
        let r = try await run(ack: .afterSystem1, asks: "yes")
        let first = try XCTUnwrap(end(r, "voice.firstAudio")), s1 = try XCTUnwrap(end(r, "system1"))
        XCTAssertGreaterThan(first, s1)
        XCTAssertEqual(r.record.summary.holdingLines, ["Got it. Let me look into that."])
    }

    func testCarryOnAnswerCancelsTheAckBeforeAnyAudio() async throws {
        // System 1 answers before the voice can start (a fast answer, a voice that has not produced a frame).
        let r = try await run(ack: .immediate, asks: "no", s1Delay: 0.0)
        // The ack is either cancelled outright or, if its first frame was already out, its line finished; either way no
        // second line, no desk revision, and the record says so.
        XCTAssertNotNil(r.record.summary.ackCancelled, "the record must say the acknowledgement was stopped")
        XCTAssertEqual(r.record.summary.holdingLines.filter { $0 != "Got it." }, [])
        XCTAssertTrue(r.record.summary.revisions.isEmpty, "the desk was not engaged")
    }

    func testCarryOnAfterAudibleAckFinishesTheLineAndStarts_NoOther() async throws {
        let r = try await run(ack: .immediate, asks: "no", s1Delay: 0.5)
        let s = r.record.summary
        XCTAssertTrue(s.ackCancelled?.contains("already speaking") == true, "the ack was audible when System 1 answered: \(s.ackCancelled ?? "nil")")
        XCTAssertEqual(s.holdingLines, ["Got it."])
        XCTAssertTrue(s.revisions.isEmpty)
        XCTAssertNotNil(s.turnEndToAckMs)
    }
}
