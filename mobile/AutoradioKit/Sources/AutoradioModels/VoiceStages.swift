import Foundation
import FluidAudio
import AutoradioKit

/// Split a line into sentences so the first one can be spoken while the rest is still being synthesised.
func sentences(_ s: String) -> [String] {
    var out: [String] = [], cur = ""
    for ch in s {
        cur.append(ch)
        if ".!?;:".contains(ch) { out.append(cur.trimmingCharacters(in: .whitespaces)); cur = "" }
    }
    if !cur.trimmingCharacters(in: .whitespaces).isEmpty { out.append(cur.trimmingCharacters(in: .whitespaces)) }
    return out.filter { !$0.isEmpty }
}

/// Yield `samples` as 100 ms frames so the player's reference signal and level changes act on short pieces.
func frames24k(_ samples: [Float], rate: Double, into c: AsyncThrowingStream<AudioFrame, Error>.Continuation) {
    let step = Int(rate / 10)
    var i = 0
    while i < samples.count {
        let j = min(samples.count, i + step)
        c.yield(AudioFrame(samples: Array(samples[i..<j]), sampleRate: rate, t: ProcessInfo.processInfo.systemUptime))
        i = j
    }
}

// MARK: Kokoro 82M (ANE v3)

/// Kokoro 82M through FluidAudio's ANE pipeline. Synthesis is per sentence: the first audio is the first sentence's
/// synthesis time, not the whole line's. Voice ids are Kokoro's (af_heart, af_bella, am_adam, ...).
public final class KokoroVoice: Voice, @unchecked Sendable {
    public static let stageID = "kokoro"
    public static var descriptor: StageDescriptor {
        StageDescriptor(
            displayName: "Kokoro 82M (ANE, per sentence)",
            summary: "On-device neural voice (Core ML via FluidAudio). First audio after the first sentence is synthesised.",
            params: [.text("voice", "Voice", "af_heart"), .number("speed", "Speed", 1.0, 0.6, 1.5)],
            needsModels: ["kokoro-82m-ane"])
    }
    static let slot = ModelSlot<KokoroAneManager>("kokoro-82m-ane")
    let voice: String, speed: Float
    public init(_ p: StageParams) { voice = p.string("voice", "af_heart"); speed = Float(p.double("speed", 1.0)) }

    public func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error> {
        let (voice, speed) = (voice, speed)
        return AsyncThrowingStream { c in
            let task = Task {
                do {
                    let m = try await Self.slot.get {
                        let k = KokoroAneManager(version: .v3)
                        try await k.initialize(preloadVoices: [voice])
                        return (k, { await k.cleanup() })
                    }
                    for await line in text {
                        for s in sentences(line) {
                            if Task.isCancelled { break }
                            let r = try await m.synthesizeDetailed(text: s, voice: voice, speed: speed)
                            frames24k(r.samples, rate: Double(r.sampleRate), into: c)
                        }
                    }
                    c.finish()
                } catch { c.finish(throwing: error) }
            }
            c.onTermination = { _ in task.cancel() }
        }
    }
    /// One synthesis with its timing, for the proof.
    public static func synthesizeOnce(_ text: String, voice: String = "af_heart") async throws -> (samples: [Float], rate: Double, ms: Double) {
        let m = try await slot.get { let k = KokoroAneManager(version: .v3); try await k.initialize(preloadVoices: [voice]); return (k, { await k.cleanup() }) }
        let t0 = nowMs()
        let r = try await m.synthesizeDetailed(text: text, voice: voice, speed: 1.0)
        return (r.samples, Double(r.sampleRate), nowMs() - t0)
    }
}

// MARK: PocketTTS (Kyutai), streams frames

/// Kyutai PocketTTS 100M through FluidAudio: autoregressive, so audio frames stream out as they are generated
/// (80 ms frames), rather than a sentence at a time.
public final class PocketTTSVoice: Voice, @unchecked Sendable {
    public static let stageID = "pocket-tts"
    public static var descriptor: StageDescriptor {
        StageDescriptor(
            displayName: "PocketTTS 100M (streaming frames)",
            summary: "On-device neural voice that streams frames while it generates (Core ML via FluidAudio).",
            params: [.text("voice", "Voice", "alba")],
            needsModels: ["pocket-tts"])
    }
    static let slot = ModelSlot<PocketTtsManager>("pocket-tts")
    let voice: String
    public init(_ p: StageParams) { voice = p.string("voice", "alba") }

    public func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error> {
        let voice = voice
        return AsyncThrowingStream { c in
            let task = Task {
                do {
                    let m = try await Self.slot.get {
                        let p = PocketTtsManager(defaultVoice: voice)
                        try await p.initialize()
                        return (p, { await p.cleanup() })
                    }
                    for await line in text {
                        if Task.isCancelled { break }
                        let stream = try await m.synthesizeStreaming(text: line, voice: voice)
                        for try await f in stream {
                            c.yield(AudioFrame(samples: f.samples, sampleRate: Double(PocketTtsConstants.audioSampleRate), t: ProcessInfo.processInfo.systemUptime))
                        }
                    }
                    c.finish()
                } catch { c.finish(throwing: error) }
            }
            c.onTermination = { _ in task.cancel() }
        }
    }
    public static func firstFrameMs(_ text: String, voice: String = "alba") async throws -> (firstMs: Double, totalMs: Double, seconds: Double) {
        let m = try await slot.get { let p = PocketTtsManager(defaultVoice: voice); try await p.initialize(); return (p, { await p.cleanup() }) }
        let t0 = nowMs()
        var first = 0.0, n = 0
        for try await f in try await m.synthesizeStreaming(text: text, voice: voice) {
            if first == 0 { first = nowMs() - t0 }
            n += f.samples.count
        }
        return (first, nowMs() - t0, Double(n) / Double(PocketTtsConstants.audioSampleRate))
    }
}
