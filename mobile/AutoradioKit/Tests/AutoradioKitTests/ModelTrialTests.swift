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
//  T8 the states reported are in order, ending in done or failed, and a second run of the same row starts fresh.
final class ModelTrialTests: XCTestCase {
    final class Fake: TrialBackend, @unchecked Sendable {
        private let lock = NSLock()
        var calls: [String] = []
        var audio: [Float]? = Array(repeating: 0.1, count: 16_000 * 5)
        var text = "hello there radio", span: (Double?, Double?) = (0.8, 3.1), answerText = "Paris.", speakMs = 240.0, voiceLine = "That is your voice.", searchLine = "Found 3 of your stories."
        var fail: String?
        func log(_ s: String) { lock.lock(); calls.append(s); lock.unlock() }
        func record(seconds: Double) async -> [Float]? { log("record \(Int(seconds))"); return audio }
        func transcribe(_ id: String, _ audio: [Float]) async throws -> String { log("transcribe \(id)"); if let f = fail { throw NSError(domain: f, code: 1) }; return text }
        func speechSpan(_ id: String, _ audio: [Float]) async throws -> (start: Double?, end: Double?) { log("span \(id)"); return span }
        func speak(_ id: String, _ sentence: String) async throws -> Double { log("speak \(id)"); if let f = fail { throw NSError(domain: f, code: 1) }; return speakMs }
        func answer(_ id: String, job: String, prompt: String) async throws -> String { log("answer \(job) \(id) \(prompt)"); if let f = fail { throw NSError(domain: f, code: 1) }; return answerText }
        func voiceprint(_ audio: [Float]) async throws -> String { log("voiceprint"); return voiceLine }
        func search(_ query: String) async throws -> String { log("search \(query)"); return searchLine }
    }
    final class Sink: @unchecked Sendable {
        private let lock = NSLock(); var seen: [(String, String, Double)] = []
        func put(_ s: String, _ l: String, _ ms: Double) { lock.lock(); seen.append((s, l, ms)); lock.unlock() }
        var states: [String] { seen.map { $0.0 } }
        var last: (String, String, Double)? { seen.last }
    }
    func run(_ job: String, _ id: String, _ b: Fake) async -> Sink {
        let k = Sink(); await ModelTrial.run(job: job, id: id, backend: b, report: { k.put($0, $1, $2) }); return k
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
    func testT6SearchRunsAFixedQuery() async {
        let b = Fake(); let k = await run("search", "embeddinggemma", b)
        XCTAssertEqual(b.calls, ["search \(ModelTrial.searchQuery)"]); XCTAssertEqual(k.last?.1, "Found 3 of your stories.")
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
    // Review round 3: a Test that was cancelled (the account left) says nothing more: no "done" line, no sound after the cancel.
    func testT9ACancelledTestReportsNothingFurther() async {
        final class Slow: TrialBackend, @unchecked Sendable {
            func record(seconds: Double) async -> [Float]? { try? await Task.sleep(nanoseconds: 300_000_000); return [0.1] }
            func transcribe(_ id: String, _ audio: [Float]) async throws -> String { "words" }
            func speechSpan(_ id: String, _ audio: [Float]) async throws -> (start: Double?, end: Double?) { (0, 1) }
            func speak(_ id: String, _ sentence: String) async throws -> Double { 1 }
            func answer(_ id: String, job: String, prompt: String) async throws -> String { "a" }
            func voiceprint(_ audio: [Float]) async throws -> String { "v" }
            func search(_ query: String) async throws -> String { "s" }
        }
        let k = Sink()
        let t = Task { await ModelTrial.run(job: "transcriber", id: "m", backend: Slow(), report: { k.put($0, $1, $2) }) }
        try? await Task.sleep(nanoseconds: 50_000_000); t.cancel(); await t.value
        XCTAssertEqual(k.states, ["recording"], "nothing after the cancel: \(k.states)")
    }
}
