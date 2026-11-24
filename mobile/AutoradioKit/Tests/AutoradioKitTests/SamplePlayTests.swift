import XCTest
@testable import AutoradioKit

// A voice sample (the Test on a voice row, Use, the hearing check) is an effect like any other: it passes the one gate at every step (docs/problems/model-test-cancellation-2026-11-24.md; seat 2 round 6): before the engine starts, before
// the voice is made, before the model runs, per frame, and before the node plays. There is no nil gate: `playVoice` and `playSample` take one, and a path with no Test owner says why with `.alwaysLive(because:)`.
//  P1 a gate that is dead before the call starts nothing: not the engine, not the voice, not the model.
//  P2 the account leaves while the engine is starting: the voice is not made and its model does not run.
//  P3 the account leaves during the model's load (the first frame is late): the frames that come are not played (no further pull).
//  P4 a live gate plays the sample, as before.
//  P5 an always-live gate keeps playing after a sign-out: the radio's own line is not a Test, and says so.
final class SamplePlayTests: XCTestCase {
    final class Probe: @unchecked Sendable {
        static let shared = Probe()
        private let l = NSLock(); private var started = 0, made = 0, spoke = 0, pulled = 0; private var latch: CheckedContinuation<Void, Never>?; private var openLoad = true
        func reset(hold: Bool) { l.lock(); started = 0; made = 0; spoke = 0; pulled = 0; latch = nil; openLoad = !hold; l.unlock() }
        func bump(_ k: ReferenceWritableKeyPath<Probe, Int>) { l.lock(); self[keyPath: k] += 1; l.unlock() }
        var counts: (started: Int, made: Int, spoke: Int, pulled: Int) { l.lock(); defer { l.unlock() }; return (started, made, spoke, pulled) }
        func load() async { l.lock(); let go = openLoad; l.unlock(); if go { return }; await withCheckedContinuation { (c: CheckedContinuation<Void, Never>) in l.lock(); if openLoad { l.unlock(); c.resume() } else { latch = c; l.unlock() } } }
        func release() { l.lock(); openLoad = true; let c = latch; latch = nil; l.unlock(); c?.resume() }
        func note(_ w: String) { l.lock(); if w == "started" { started += 1 } else if w == "made" { made += 1 } else if w == "spoke" { spoke += 1 } else { pulled += 1 }; l.unlock() }
    }
    final class SampleVoice: Voice, @unchecked Sendable {
        static let stageID = "sample-voice"
        static var descriptor: StageDescriptor { StageDescriptor(displayName: "sample voice") }
        init(_ p: StageParams) { Probe.shared.note("made") }
        func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error> {
            Probe.shared.note("spoke")
            return AsyncThrowingStream { c in
                let t = Task { await Probe.shared.load(); for i in 0..<6 { Probe.shared.note("pulled"); c.yield(AudioFrame(samples: [Float](repeating: 0.1, count: 480), sampleRate: 24_000, t: Double(i))); try? await Task.sleep(nanoseconds: 3_000_000) }; c.finish() }
                c.onTermination = { _ in t.cancel() }
            }
        }
    }
    func runtime() -> LabRuntime {
        let rt = LabRuntime(supportDir: FileManager.default.temporaryDirectory.appendingPathComponent("sample-\(UUID().uuidString)"), session: NullSessionOwner())
        rt.registry.register(.voice, SampleVoice.self) { SampleVoice($0) }
        return rt
    }
    let option = VoiceOption(id: "sample:x", stageID: "sample-voice", voice: "x", name: "X", blurb: "b", sample: "Hello.", modelID: "")

    func testP1ADeadGateStartsNothing() async {
        Probe.shared.reset(hold: false); let rt = runtime(); let gate = TrialGate()
        AccountWipe.bumpGeneration()
        do { _ = try await rt.playSample(option, gate: gate, start: { Probe.shared.note("started") }); XCTFail("played") } catch {}
        let c = Probe.shared.counts
        XCTAssertEqual(c.started, 0); XCTAssertEqual(c.made, 0); XCTAssertEqual(c.spoke, 0)
    }
    func testP2TheAccountLeavingWhileTheEngineStartsMakesNoVoiceAndRunsNoModel() async {
        Probe.shared.reset(hold: false); let rt = runtime(); let gate = TrialGate()
        do { _ = try await rt.playSample(option, gate: gate, start: { Probe.shared.note("started"); AccountWipe.bumpGeneration() }); XCTFail("played") } catch {}
        let c = Probe.shared.counts
        XCTAssertEqual(c.started, 1); XCTAssertEqual(c.made, 0, "the voice was not made after the leave"); XCTAssertEqual(c.spoke, 0)
    }
    func testP2bTheAccountLeavingJustAfterTheVoiceIsMadeRunsNoModel() async {
        Probe.shared.reset(hold: false); let rt = runtime(); let gate = TrialGate()
        rt.registry.register(.voice, SampleVoice.self) { p in let v = SampleVoice(p); AccountWipe.bumpGeneration(); return v }      // the account leaves as the voice is made (its load)
        do { _ = try await rt.playSample(option, gate: gate, start: {}); XCTFail("played") } catch {}
        XCTAssertEqual(Probe.shared.counts.spoke, 0, "the model did not run after the voice was made for an account that left")
    }
    func testP3TheAccountLeavingDuringTheLoadPlaysNothingFurther() async {
        Probe.shared.reset(hold: true); let rt = runtime(); let gate = TrialGate()
        let t = Task { () -> Double? in try? await rt.playSample(option, gate: gate, start: {}) }
        for _ in 0..<400 where Probe.shared.counts.spoke == 0 { try? await Task.sleep(nanoseconds: 5_000_000) }
        XCTAssertEqual(Probe.shared.counts.spoke, 1, "the model was running (its load) when the account left")
        AccountWipe.bumpGeneration(); Probe.shared.release()
        let result = await t.value
        XCTAssertNil(result, "the sample did not count as played: the frames that came after the leave were not forwarded to the player")
    }
    func testP4ALiveGatePlaysTheSample() async throws {
        Probe.shared.reset(hold: false); let rt = runtime()
        _ = try await rt.playSample(option, gate: TrialGate(), start: {})
        XCTAssertEqual(Probe.shared.counts.pulled, 6)
    }
    func testP5AnAlwaysLiveGateKeepsPlayingAfterASignOut() async throws {
        Probe.shared.reset(hold: false); let rt = runtime()
        let g = TrialGate.alwaysLive(because: "the radio's own line")
        AccountWipe.bumpGeneration()
        XCTAssertTrue(g.live); XCTAssertFalse(g.why.isEmpty)
        _ = try await rt.playSample(option, gate: g, start: {})
        XCTAssertEqual(Probe.shared.counts.pulled, 6)
    }
}
