import Foundation

/// What the phone can do for a model's Test button. Each call needs the model itself (loading it, listening, speaking, answering, searching), so the live one is native (the app module); the words and the order of a test are `ModelTrial`'s,
/// and are tested here with a fake.
public protocol TrialBackend: Sendable {
    /// The microphone for `seconds` (16 kHz mono), or nil when it is not available or the Test's account left. The implementation reads `gate` at every effect (permission, engine, tap, frames).
    func record(seconds: Double, gate: TrialGate) async -> [Float]?
    /// This recogniser's words for this audio.
    func transcribe(_ id: String, _ audio: [Float], gate: TrialGate) async throws -> String
    /// Where this speech detector finds speech start and stop (seconds into the audio).
    func speechSpan(_ id: String, _ audio: [Float], gate: TrialGate) async throws -> (start: Double?, end: Double?)
    /// Speak the sentence aloud in this voice's model; the milliseconds to the first sound. The implementation starts no sound once `gate` is dead.
    func speak(_ id: String, _ sentence: String, gate: TrialGate) async throws -> Double
    /// This language model's answer to the prompt (`job` is "responder" or "decision").
    func answer(_ id: String, job: String, prompt: String, gate: TrialGate) async throws -> String
    /// The voice-print model's verdict on this audio, in plain words.
    func voiceprint(_ audio: [Float], gate: TrialGate) async throws -> String
}

/// The Test button of Settings > Voice and models: one run of one model, reported as it goes (`report(state, line, ms)`; state is recording | working | done | failed). Every path ends in done or failed, except a CANCELLED run (the account left): it
/// stops at once and says nothing, and touches neither the microphone nor the speaker nor a model after the cancel (every backend call is behind `live`).
public enum ModelTrial {
    public static let prompt = "What is the capital of France? Answer in one short sentence."
        public static let sentence = "Good morning. This is your radio, speaking from this phone."
    public typealias Report = @Sendable (_ state: String, _ line: String, _ ms: Double) -> Void

    /// Every backend call is behind `gate.live`, read before the call and after it returns; a line is written only while it is live. A Test whose account left says nothing and does nothing.
    public static func run(job: String, id: String, backend b: TrialBackend, gate: TrialGate, report rawReport: Report) async {
        var live: Bool { gate.live }
        func report(_ state: String, _ line: String, _ ms: Double) { if live { rawReport(state, line, ms) } }
        func ms(_ since: DispatchTime) -> Double { Double(DispatchTime.now().uptimeNanoseconds - since.uptimeNanoseconds) / 1e6 }
        func s1(_ x: Double) -> String { String(format: "%.1f", x) }
        func failed(_ why: String) { report("failed", why, 0) }
        func broke(_ e: Error) { failed("It did not work on this phone: \((e as? LabError).map { "\($0)" } ?? e.localizedDescription)") }
        let micOff = "The microphone is not available. Allow it in Settings and try again."
        guard live else { return }
        switch job {
        case "transcriber", "live":
            report("recording", "Say something now. Recording for 5 seconds.", 0)
            guard live, let audio = await b.record(seconds: 5, gate: gate) else { return failed(micOff) }
            guard live else { return }
            report("working", "Listening to what you said.", 0)
            let t0 = DispatchTime.now()
            do {
                let text = try await b.transcribe(id, audio, gate: gate).trimmingCharacters(in: .whitespacesAndNewlines)
                report("done", text.isEmpty ? "It heard nothing. Try again, closer to the phone." : "Heard: \"\(text)\"", ms(t0))
            } catch { broke(error) }
        case "bounds":
            report("recording", "Say a sentence, then stop. Recording for 5 seconds.", 0)
            guard live, let audio = await b.record(seconds: 5, gate: gate) else { return failed(micOff) }
            guard live else { return }
            report("working", "Looking for where you spoke.", 0)
            let t0 = DispatchTime.now()
            do {
                let span = try await b.speechSpan(id, audio, gate: gate)
                if let a = span.start, let z = span.end { report("done", "It heard you start at \(s1(a)) s and stop at \(s1(z)) s.", ms(t0)) }
                else if let a = span.start { report("done", "It heard you start at \(s1(a)) s and did not see you stop.", ms(t0)) }
                else { report("done", "It found no speech in the recording. Try again, closer to the phone.", ms(t0)) }
            } catch { broke(error) }
        case "voice":
            report("working", "Speaking.", 0)
            guard live else { return }
            do { let first = try await b.speak(id, sentence, gate: gate); report("done", "It spoke a sentence. First sound after \(Int(first)) ms.", first) } catch { broke(error) }
        case "responder", "decision":
            report("working", "Asking: \(prompt)", 0)
            guard live else { return }
            let t0 = DispatchTime.now()
            do {
                let text = try await b.answer(id, job: job, prompt: prompt, gate: gate).trimmingCharacters(in: .whitespacesAndNewlines)
                if text.isEmpty { failed("It wrote nothing for: \(prompt)") }
                else { report("done", "Asked: \(prompt) Answered: \(text)", ms(t0)) }
            } catch { broke(error) }
        case "speaker":
            report("recording", "Say a sentence. Recording for 4 seconds.", 0)
            guard live, let audio = await b.record(seconds: 4, gate: gate) else { return failed(micOff) }
            guard live else { return }
            report("working", "Checking your voice.", 0)
            let t0 = DispatchTime.now()
            do { report("done", try await b.voiceprint(audio, gate: gate), ms(t0)) } catch { broke(error) }
        default:
            failed("There is no test for this model.")
        }
    }
}
