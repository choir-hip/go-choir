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

    /// Load the model with every Kokoro voice of the catalog ready, so the voice cards switch voices without another fetch. If one of them
    /// will not load, the model loads with the wanted voice alone: one missing voice must not take the voice away.
    static func load(preferring voice: String) async throws -> (KokoroAneManager, @Sendable () async -> Void) {
        try LibraryScope.require("kokoro-82m-ane")           // a library is initialised only for a model that is ready by manifest, or being fetched or checked: initialising it for missing files would be a download
        // Every Kokoro voice of the catalog (the generated English rows: 28) is loaded with the model, inside the one fetch the person agreed to: a voice pack fetched only when its card is tapped would
        // be a library fetch outside the download door's window. Each voice pack is a 2.7 MB file the pack lists (`VoiceRow.bytes`), 75 MB in all, which the offered size of this model counts (`KnownModels.kokoroBytes`).
        let all = Set([voice] + VoiceCatalog.options.filter { $0.stageID == stageID }.map(\.voice))
        let k = KokoroAneManager(version: .v3)
        do { try await k.initialize(preloadVoices: all) }
        catch {
            await k.cleanup()
            let one = KokoroAneManager(version: .v3)
            try await one.initialize(preloadVoices: [voice])
            return (one, { await one.cleanup() })
        }
        return (k, { await k.cleanup() })
    }

    public func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error> {
        let (voice, speed) = (voice, speed)
        return AsyncThrowingStream { c in
            let task = Task {
                do {
                    let m = try await Self.slot.get { try await Self.load(preferring: voice) }
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
        let m = try await slot.get { try await load(preferring: voice) }
        let t0 = nowMs()
        let r = try await m.synthesizeDetailed(text: text, voice: voice, speed: 1.0)
        return (r.samples, Double(r.sampleRate), nowMs() - t0)
    }
}

// MARK: Phonon 31M (Gradium), streams frames

/// Gradium's Phonon 31M English voice through FluidAudio's `PocketTtsSmallManager(model: .phonon31m)` (FluidAudio 0.18.0 and later; the
/// 0.18.3 we link has it). Autoregressive like Pocket, so audio frames (80 ms, 24 kHz) stream out while it generates. The pack is
/// `FluidInference/pocket-tts-coreml`, folder `phonon/phonon-31m-en` (Apache-2.0), fetched by FluidAudio itself into `KnownModels.fluidRoot`.
/// Voice ids are the pack's own (`en_us_Easton`, `en_us_Zoey`, `en_gb_Maeve`, `default`, ...). NOT the Hugging Face repository
/// `FluidInference/phonon-2-coreml`: that is Fermion Research's Phonon-2 speech recogniser, a different model with the same word in its name.
public final class PhononVoice: Voice, @unchecked Sendable {
    public static let stageID = "phonon"
    public static var descriptor: StageDescriptor {
        StageDescriptor(
            displayName: "Phonon 31M (streaming frames)",
            summary: "On-device neural voice from Gradium that streams frames while it generates (Core ML via FluidAudio). English.",
            params: [.text("voice", "Voice", "en_us_Easton")],
            needsModels: ["phonon-31m"])
    }
    static let slot = ModelSlot<PocketTtsSmallManager>("phonon-31m")
    let voice: String
    public init(_ p: StageParams) { voice = p.string("voice", "en_us_Easton") }

    /// FluidAudio puts a small-model pack at `<directory>/Models/pocket-tts-coreml/phonon/phonon-31m-en`; with the directory one above
    /// `fluidRoot` (`.../FluidAudio/Models`) that is the folder the catalog's `cacheFolders` names, the one the progress bar watches.
    static func load(preferring voice: String) async throws -> (PocketTtsSmallManager, @Sendable () async -> Void) {
        try LibraryScope.require("phonon-31m")
        let m = PocketTtsSmallManager(model: .phonon31m, defaultVoice: voice)
        try await m.initialize(directory: KnownModels.fluidRoot.deletingLastPathComponent())
        return (m, {})                       // the manager has no cleanup: its Core ML models go when the last reference does
    }

    public func speak(_ text: AsyncStream<String>) -> AsyncThrowingStream<AudioFrame, Error> {
        let voice = voice
        return AsyncThrowingStream { c in
            let task = Task {
                do {
                    let m = try await Self.slot.get { try await Self.load(preferring: voice) }
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
    public static func firstFrameMs(_ text: String, voice: String = "en_us_Easton") async throws -> (firstMs: Double, totalMs: Double, seconds: Double) {
        let m = try await slot.get { try await load(preferring: voice) }
        let t0 = nowMs()
        var first = 0.0, n = 0
        for try await f in try await m.synthesizeStreaming(text: text, voice: voice) {
            if first == 0 { first = nowMs() - t0 }
            n += f.samples.count
        }
        return (first, nowMs() - t0, Double(n) / Double(PocketTtsConstants.audioSampleRate))
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
                        try LibraryScope.require("pocket-tts")
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
        let m = try await slot.get { try LibraryScope.require("pocket-tts"); let p = PocketTtsManager(defaultVoice: voice); try await p.initialize(); return (p, { await p.cleanup() }) }
        let t0 = nowMs()
        var first = 0.0, n = 0
        for try await f in try await m.synthesizeStreaming(text: text, voice: voice) {
            if first == 0 { first = nowMs() - t0 }
            n += f.samples.count
        }
        return (first, nowMs() - t0, Double(n) / Double(PocketTtsConstants.audioSampleRate))
    }
}
