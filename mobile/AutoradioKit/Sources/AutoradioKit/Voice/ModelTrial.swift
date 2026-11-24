import Foundation

/// What the phone can do for a model's Test button. Each call needs the model itself (loading it, listening, speaking, answering, searching), so the live one is native (the app module); the words and the order of a test are `ModelTrial`'s,
/// and are tested here with a fake.
public protocol TrialBackend: Sendable {
    /// The microphone for `seconds` (16 kHz mono), or nil when it is not available.
    func record(seconds: Double) async -> [Float]?
    /// This recogniser's words for this audio.
    func transcribe(_ id: String, _ audio: [Float]) async throws -> String
    /// Where this speech detector finds speech start and stop (seconds into the audio).
    func speechSpan(_ id: String, _ audio: [Float]) async throws -> (start: Double?, end: Double?)
    /// Speak the sentence aloud in this voice's model; the milliseconds to the first sound.
    func speak(_ id: String, _ sentence: String) async throws -> Double
    /// This language model's answer to the prompt (`job` is "responder" or "decision").
    func answer(_ id: String, job: String, prompt: String) async throws -> String
    /// The voice-print model's verdict on this audio, in plain words.
    func voiceprint(_ audio: [Float]) async throws -> String
    /// A search over the phone's own stories, in plain words.
    func search(_ query: String) async throws -> String
}

/// The Test button of Settings > Voice and models: one run of one model, reported as it goes (`report(state, line, ms)`; state is recording | working | done | failed). Every path ends in done or failed.
public enum ModelTrial {
    public static let prompt = "What is the capital of France? Answer in one short sentence."
    public static let searchQuery = "what is in my stories today"
    public static let sentence = "Good morning. This is your radio, speaking from this phone."
    public typealias Report = @Sendable (_ state: String, _ line: String, _ ms: Double) -> Void

    public static func run(job: String, id: String, backend b: TrialBackend, report: Report) async {
        func ms(_ since: DispatchTime) -> Double { Double(DispatchTime.now().uptimeNanoseconds - since.uptimeNanoseconds) / 1e6 }
        func s1(_ x: Double) -> String { String(format: "%.1f", x) }
        func failed(_ why: String) { report("failed", why, 0) }
        func broke(_ e: Error) { failed("It did not work on this phone: \((e as? LabError).map { "\($0)" } ?? e.localizedDescription)") }
        switch job {
        case "transcriber", "live":
            report("recording", "Say something now. Recording for 5 seconds.", 0)
            guard let audio = await b.record(seconds: 5) else { return failed("The microphone is not available. Allow it in Settings and try again.") }
            report("working", "Listening to what you said.", 0)
            let t0 = DispatchTime.now()
            do {
                let text = try await b.transcribe(id, audio).trimmingCharacters(in: .whitespacesAndNewlines)
                report("done", text.isEmpty ? "It heard nothing. Try again, closer to the phone." : "Heard: \"\(text)\"", ms(t0))
            } catch { broke(error) }
        case "bounds":
            report("recording", "Say a sentence, then stop. Recording for 5 seconds.", 0)
            guard let audio = await b.record(seconds: 5) else { return failed("The microphone is not available. Allow it in Settings and try again.") }
            report("working", "Looking for where you spoke.", 0)
            let t0 = DispatchTime.now()
            do {
                let span = try await b.speechSpan(id, audio)
                if let a = span.start, let z = span.end { report("done", "It heard you start at \(s1(a)) s and stop at \(s1(z)) s.", ms(t0)) }
                else if let a = span.start { report("done", "It heard you start at \(s1(a)) s and did not see you stop.", ms(t0)) }
                else { report("done", "It found no speech in the recording. Try again, closer to the phone.", ms(t0)) }
            } catch { broke(error) }
        case "voice":
            report("working", "Speaking.", 0)
            do { let first = try await b.speak(id, sentence); report("done", "It spoke a sentence. First sound after \(Int(first)) ms.", first) } catch { broke(error) }
        case "responder", "decision":
            report("working", "Asking: \(prompt)", 0)
            let t0 = DispatchTime.now()
            do {
                let text = try await b.answer(id, job: job, prompt: prompt).trimmingCharacters(in: .whitespacesAndNewlines)
                if text.isEmpty { failed("It wrote nothing for: \(prompt)") }
                else { report("done", "Asked: \(prompt) Answered: \(text)", ms(t0)) }
            } catch { broke(error) }
        case "speaker":
            report("recording", "Say a sentence. Recording for 4 seconds.", 0)
            guard let audio = await b.record(seconds: 4) else { return failed("The microphone is not available. Allow it in Settings and try again.") }
            report("working", "Checking your voice.", 0)
            let t0 = DispatchTime.now()
            do { report("done", try await b.voiceprint(audio), ms(t0)) } catch { broke(error) }
        case "search":
            report("working", "Searching your stories on this phone.", 0)
            let t0 = DispatchTime.now()
            do { report("done", try await b.search(searchQuery), ms(t0)) } catch { broke(error) }
        default:
            failed("There is no test for this model.")
        }
    }
}
