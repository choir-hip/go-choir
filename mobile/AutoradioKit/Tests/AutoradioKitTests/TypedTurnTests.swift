import XCTest
@testable import AutoradioKit

// The bar's typed question runs the spoken turn's pipeline. Failure list (written first):
//  1. empty or blank text: no turn, no event.
//  2. a typed turn reaches the desk with exactly the typed words (not a transcript of silence) and ends as a record.
//  3. the engine is left idle afterwards (never a stuck phase), so the radio and a second ask both still work.
final class TypedTurnTests: XCTestCase {
    final class Box: @unchecked Sendable { var records: [TurnRecord] = []; func add(_ r: TurnRecord) { records.append(r) } }

    func makeEngine() throws -> (TurnEngine, EventLog, Box) {
        let r = StageRegistry.builtIn()
        r.register(.system1, StubS1.self) { StubS1($0) }
        r.register(.voice, StubVoice.self) { StubVoice($0) }
        r.register(.transcriber, StubTranscriber.self) { StubTranscriber($0) }
        var cfg = StageRegistry.defaultConfiguration()
        cfg.set(.system1, .init(id: "stub-s1", params: ["delay": .number(0.05), "asks": .string("yes")]))
        cfg.set(.voice, .init(id: "stub-voice"))
        cfg.set(.transcriber, .init(id: "stub-tr"))
        cfg.set(.holder, .init(id: "fixed-lines", params: ["maxLines": .number(0)]))
        cfg.set(.desk, .init(id: "stub-echo", params: ["delaySeconds": .number(0.1)]))
        let log = EventLog(clock: LabClock())
        let pb = SimulatedPlayback(position: ProgramPosition(programmeID: "p", blockIndex: 0, context: "ctx"))
        let got = Box()
        let env = EngineEnvironment(registry: r, log: log, playback: pb, onRecord: { got.add($0) })
        return (TurnEngine(env: env, configuration: cfg), log, got)
    }

    func testBlankTextStartsNothing() async throws {
        let (engine, log, got) = try makeEngine()
        let started = await engine.typedTurn("  \n ")
        XCTAssertFalse(started)
        XCTAssertTrue(got.records.isEmpty)
        XCTAssertFalse(log.events.contains { if case .finalTranscript = $0.event { return true } else { return false } })
    }

    func testTypedWordsReachTheDeskAndTheEngineIsIdleAfter() async throws {
        let (engine, log, got) = try makeEngine()
        let started = await engine.typedTurn("  What changed in the harbor story?  ")
        XCTAssertTrue(started)
        for _ in 0..<100 where got.records.isEmpty { try await Task.sleep(nanoseconds: 100_000_000) }
        let rec = try XCTUnwrap(got.records.first, "the turn must end with a record")
        XCTAssertEqual(rec.summary.transcript, "What changed in the harbor story?")
        XCTAssertTrue(log.events.contains { if case .finalTranscript(let t) = $0.event { return t == "What changed in the harbor story?" } else { return false } })
    }
}
