import XCTest
@testable import AutoradioKit

// The Test button on a model's row in Voice and models (ModelTrial). The failure list, written first:
//  T1 a hearing model (final or live) is tested on a recording the person makes on the spot: it says "say something", records, transcribes THAT model's way, and shows the words and the time; silence says so.
//  T2 knowing when you speak: records, finds where speech starts and stops, and shows both; no speech says so.
//  T3 a voice speaks a sample sentence aloud and shows the time to the first sound.
//  T4 the answering and quick-decision models answer the fixed prompt; the answer and the time are shown; an empty answer is a failure, not a pass.
//  T5 knowing your voice: records and shows the backend's verdict.
//  T6 search runs a fixed query on the phone's own stories and shows what it found.
//  T7 every failure (no microphone, a model that throws, a model not on the phone) is a plain line and a finished state; a test never leaves "testing" behind.
//  T8 the states reported are in order, ending in done or failed (a cancelled run ends silently), and a second run of the same row starts fresh.
final class ModelTrialTests: XCTestCase {
    final class Fake: TrialBackend, @unchecked Sendable {
        private let lock = NSLock()
        var calls: [String] = []
        var audio: [Float]? = Array(repeating: 0.1, count: 16_000 * 5)
        var text = "hello there radio", span: (Double?, Double?) = (0.8, 3.1), answerText = "Paris.", speakMs = 240.0, voiceLine = "That is your voice."
        var fail: String?
        func log(_ s: String) { lock.lock(); calls.append(s); lock.unlock() }
        func record(seconds: Double, gate: TrialGate) async -> [Float]? { log("record \(Int(seconds))"); return audio }
        func transcribe(_ id: String, _ audio: [Float], gate: TrialGate) async throws -> String { log("transcribe \(id)"); if let f = fail { throw NSError(domain: f, code: 1) }; return text }
        func speechSpan(_ id: String, _ audio: [Float], gate: TrialGate) async throws -> (start: Double?, end: Double?) { log("span \(id)"); return span }
        func speak(_ id: String, _ sentence: String, gate: TrialGate) async throws -> Double { log("speak \(id)"); if let f = fail { throw NSError(domain: f, code: 1) }; return speakMs }
        func answer(_ id: String, job: String, prompt: String, gate: TrialGate) async throws -> String { log("answer \(job) \(id) \(prompt)"); if let f = fail { throw NSError(domain: f, code: 1) }; return answerText }
        func voiceprint(_ audio: [Float], gate: TrialGate) async throws -> String { log("voiceprint"); return voiceLine }
    }
    final class Sink: @unchecked Sendable {
        private let lock = NSLock(); var seen: [(String, String, Double)] = []
        func put(_ s: String, _ l: String, _ ms: Double) { lock.lock(); seen.append((s, l, ms)); lock.unlock() }
        var states: [String] { lock.lock(); defer { lock.unlock() }; return seen.map { $0.0 } }
        var last: (String, String, Double)? { lock.lock(); defer { lock.unlock() }; return seen.last }
        var count: Int { lock.lock(); defer { lock.unlock() }; return seen.count }
    }
    func run(_ job: String, _ id: String, _ b: Fake) async -> Sink {
        let k = Sink(); await ModelTrial.run(job: job, id: id, backend: b, gate: TrialGate(), report: { k.put($0, $1, $2) }); return k
    }

    func testT1AHearingModelIsTestedOnARecordingMadeOnTheSpot() async {
        let b = Fake(); let k = await run("transcriber", "fluid-speech-mini", b)
        XCTAssertEqual(b.calls, ["record 5", "transcribe fluid-speech-mini"])
        XCTAssertEqual(k.states, ["recording", "working", "done"]); XCTAssertEqual(k.seen[0].1, "Say something now. Recording for 5 seconds.")
        XCTAssertEqual(k.last?.1, "Heard: \"hello there radio\""); XCTAssertGreaterThan(k.last?.2 ?? 0, -1)
    }
    func testT1SilenceSaysSoAndIsNotAPassOfTheModel() async {
        let b = Fake(); b.text = "  "; let k = await run("transcriber", "parakeet-eou-120m", b)
        XCTAssertEqual(k.states.last, "done"); XCTAssertTrue(k.last!.1.contains("heard nothing"), k.last!.1)
    }
    func testT2SpeechBoundsShowStartAndStop() async {
        let b = Fake(); var k = await run("bounds", "silero-vad-v6.2.1", b)
        XCTAssertEqual(b.calls, ["record 5", "span silero-vad-v6.2.1"]); XCTAssertEqual(k.last?.1, "It heard you start at 0.8 s and stop at 3.1 s.")
        b.span = (nil, nil); k = await run("bounds", "silero-vad-v6.2.1", b); XCTAssertTrue(k.last!.1.contains("no speech"), k.last!.1)
    }
    func testT3AVoiceSpeaksAndSaysHowLongItTookToFirstSound() async {
        let b = Fake(); let k = await run("voice", "kokoro-82m-ane", b)
        XCTAssertEqual(b.calls, ["speak kokoro-82m-ane"]); XCTAssertEqual(k.states, ["working", "done"]); XCTAssertEqual(k.last?.1, "It spoke a sentence. First sound after 240 ms.")
    }
    func testT4TheAnsweringAndTheDecisionModelAnswerTheFixedPrompt() async {
        let b = Fake(); var k = await run("responder", "lfm3-omni-2b-q8_0", b)
        XCTAssertEqual(b.calls.count, 1); XCTAssertTrue(b.calls[0].hasPrefix("answer responder lfm3-omni-2b-q8_0 "))
        XCTAssertTrue(k.last!.1.contains("Paris.") && k.last!.1.contains(ModelTrial.prompt), k.last!.1)
        k = await run("decision", "lfm2.5-350m-q4_k_m", b); XCTAssertEqual(k.states.last, "done"); XCTAssertTrue(b.calls.last!.hasPrefix("answer decision "))
        b.answerText = "   "; k = await run("responder", "x", b); XCTAssertEqual(k.states.last, "failed"); XCTAssertTrue(k.last!.1.contains("nothing"), k.last!.1)
    }
    func testT5KnowingYourVoiceRecordsAndShowsTheVerdict() async {
        let b = Fake(); let k = await run("speaker", "wespeaker-resnet34", b)
        XCTAssertEqual(b.calls, ["record 4", "voiceprint"]); XCTAssertEqual(k.last?.1, "That is your voice.")
    }
    func testT7EveryFailureIsAPlainLineAndAFinishedState() async {
        let b = Fake(); b.audio = nil
        for j in ["transcriber", "bounds", "speaker"] { let k = await run(j, "m", b); XCTAssertEqual(k.states.last, "failed", j); XCTAssertTrue(k.last!.1.contains("microphone"), k.last!.1) }
        let c = Fake(); c.fail = "boom"
        for j in ["transcriber", "voice", "responder"] { let k = await run(j, "m", c); XCTAssertEqual(k.states.last, "failed", j); XCTAssertTrue(k.last!.1.contains("did not work"), k.last!.1) }
        let d = Fake(); let k = await run("nonsense", "m", d); XCTAssertEqual(k.states, ["failed"])
    }
    func testT8EachRunStartsFreshAndEndsFinished() async {
        let b = Fake(); let a = await run("voice", "kokoro-82m-ane", b); let c = await run("voice", "kokoro-82m-ane", b)
        XCTAssertEqual(a.states, c.states); XCTAssertFalse(["recording", "working", "testing"].contains(a.states.last!))
    }
    // THE PROOF (docs/problems/model-test-cancellation-2026-11-24.md): a Test is an obligation owned by one account generation. A scripted backend suspends at every call of every job; the account leaves (the generation moves, or the task is
    // cancelled) while call k is suspended; everything is then released. Nothing happens after the sign-out: no further backend call (the microphone, a model, the speaker) and no report.
    final class Scripted: TrialBackend, @unchecked Sendable {
        private let lock = NSLock(); private var started: [String] = []; private var latches: [CheckedContinuation<Void, Never>] = []; private var open = false
        var calls: [String] { lock.lock(); defer { lock.unlock() }; return started }
        func step(_ name: String) async {
            lock.lock(); started.append(name); let released = open; lock.unlock()
            if released { return }
            await withCheckedContinuation { (c: CheckedContinuation<Void, Never>) in
                lock.lock(); if open { lock.unlock(); c.resume() } else { latches.append(c); lock.unlock() }
            }
        }
        /// After a step was released, the next one suspends again.
        func relatch() { lock.lock(); open = false; lock.unlock() }
        func releaseAll() { lock.lock(); open = true; let l = latches; latches = []; lock.unlock(); for c in l { c.resume() } }
        func record(seconds: Double, gate: TrialGate) async -> [Float]? { await step("record"); return [0.1] }
        func transcribe(_ id: String, _ audio: [Float], gate: TrialGate) async throws -> String { await step("transcribe"); return "words" }
        func speechSpan(_ id: String, _ audio: [Float], gate: TrialGate) async throws -> (start: Double?, end: Double?) { await step("span"); return (0, 1) }
        func speak(_ id: String, _ sentence: String, gate: TrialGate) async throws -> Double { await step("speak"); return 1 }
        func answer(_ id: String, job: String, prompt: String, gate: TrialGate) async throws -> String { await step("answer"); return "a" }
        func voiceprint(_ audio: [Float], gate: TrialGate) async throws -> String { await step("voiceprint"); return "v" }
    }
    func waitFor(_ s: Scripted, count: Int) async { for _ in 0..<400 { if s.calls.count >= count { return }; try? await Task.sleep(nanoseconds: 5_000_000) } }
    static let steps: [String: Int] = ["transcriber": 2, "live": 2, "bounds": 2, "voice": 1, "responder": 1, "decision": 1, "speaker": 2]

    func testT9AfterTheAccountLeavesNothingHappensAtAnyStepOfAnyJob() async {
        for (job, n) in Self.steps.sorted(by: { $0.key < $1.key }) {
            for k in 1...n {                                                         // the account leaves while backend call number k is suspended
                let b = Scripted(), k0 = Sink(); let gate = TrialGate()
                let t = Task { await ModelTrial.run(job: job, id: "m", backend: b, gate: gate, report: { k0.put($0, $1, $2) }) }
                for i in 1..<k { await waitFor(b, count: i); b.releaseAll(); b.relatch() }      // calls before k run to completion, one at a time
                await waitFor(b, count: k)
                let callsAtLeave = b.calls.count, reportsAtLeave = k0.count
                AccountWipe.bumpGeneration()                                                // sign-out: the generation moves first
                b.releaseAll(); await t.value
                XCTAssertEqual(b.calls.count, callsAtLeave, "\(job) left at call \(k): a backend call began after the sign-out: \(b.calls)")
                XCTAssertEqual(k0.count, reportsAtLeave, "\(job) left at call \(k): a line was written after the sign-out")
            }
        }
    }
    func testT9bTheSameForACancelledTask() async {
        for (job, n) in Self.steps.sorted(by: { $0.key < $1.key }) {
            for k in 1...n {
                let b = Scripted(), k0 = Sink(); let gate = TrialGate()
                let t = Task { await ModelTrial.run(job: job, id: "m", backend: b, gate: gate, report: { k0.put($0, $1, $2) }) }
                for i in 1..<k { await waitFor(b, count: i); b.releaseAll(); b.relatch() }
                await waitFor(b, count: k)
                let callsAtLeave = b.calls.count, reportsAtLeave = k0.count
                t.cancel(); b.releaseAll(); await t.value
                XCTAssertEqual(b.calls.count, callsAtLeave, "\(job) cancelled at call \(k): \(b.calls)")
                XCTAssertEqual(k0.count, reportsAtLeave, "\(job) cancelled at call \(k)")
            }
        }
    }
    func testT9cALeaveBeforeTheFirstStepDoesNothing() async {
        for job in Self.steps.keys {
            let b = Scripted(), k0 = Sink(); let gate = TrialGate()
            AccountWipe.bumpGeneration()                                               // the account left between the press and the run
            await ModelTrial.run(job: job, id: "m", backend: b, gate: gate, report: { k0.put($0, $1, $2) })
            XCTAssertEqual(b.calls, [], job); XCTAssertEqual(k0.count, 0, job)
        }
    }
}
