import Foundation

/// A failure the sample reports after the audio has played what there was, and what it measured.
final class SampleOutcome: @unchecked Sendable {
    private let lock = NSLock()
    private var error: Error?
    private var frames = 0
    private var first: Double?
    func firstSound(_ ms: Double) { lock.lock(); if first == nil { first = ms }; lock.unlock() }
    var firstMs: Double { lock.lock(); defer { lock.unlock() }; return first ?? 0 }
    func fail(_ e: Error) { lock.lock(); error = e; lock.unlock() }
    func got(_ n: Int) { lock.lock(); frames += n; lock.unlock() }
    func result() -> (Error?, Int) { lock.lock(); defer { lock.unlock() }; return (error, frames) }
}

extension LabRuntime {
    /// Say the option's sample in THAT voice (the Test on a voice row, Use, the hearing check). There is no fall back here: a card for Heart that played the phone's voice would be a lie.
    /// `gate` is REQUIRED (docs/problems/model-test-cancellation-2026-11-24.md) and read at every effect: before the engine starts, after it, after the voice is made (its load), before the model runs, for every frame, and
    /// by `playVoice` before the node starts and before every buffer. `start` is the engine start (the radio's own `start(virtual: false)` by default). Returns the milliseconds to the first sound.
    @discardableResult
    public func playSample(_ option: VoiceOption, gate: TrialGate, start: (@Sendable () async -> Void)? = nil) async throws -> Double {
        let t0 = DispatchTime.now()
        guard gate.live else { throw CancellationError() }               // an account that left starts nothing, not even the engine
        if let start { await start() } else { await self.start(virtual: false) }            // does nothing when the radio is already running
        guard gate.live else { throw CancellationError() }
        var cfg = configuration
        cfg.set(.voice, option.setting)
        let voice = try registry.make(.voice, as: Voice.self, cfg)
        guard gate.live else { throw CancellationError() }               // the voice was made (its model loaded): a model run starts only for an account that is still here
        let outcome = SampleOutcome()
        let frames = AsyncThrowingStream<AudioFrame, Error> { c in
            let t = Task {
                do {
                    for try await f in voice.speak(AsyncStream<String> { $0.yield(option.sample); $0.finish() }) {
                        guard gate.live else { break }
                        outcome.firstSound(Double(DispatchTime.now().uptimeNanoseconds - t0.uptimeNanoseconds) / 1e6); outcome.got(f.samples.count); c.yield(f)
                    }
                } catch { outcome.fail(error) }
                c.finish()
            }
            c.onTermination = { _ in t.cancel() }
        }
        await audio.playVoice(frames, gate: gate)
        let (error, n) = outcome.result()
        if let error { throw error }
        guard gate.live else { throw CancellationError() }
        if n == 0 { throw LabError.unavailable("the voice \(option.id) made no sound") }
        return outcome.firstMs                                    // milliseconds from the start to the first sound (the Test button shows it)
    }
}
