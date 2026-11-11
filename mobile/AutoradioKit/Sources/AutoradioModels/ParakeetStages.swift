import Foundation
import AVFoundation
import FluidAudio
import AutoradioKit

// MARK: Parakeet EOU 120M, streaming

/// NVIDIA Parakeet EOU 120M, cache-aware streaming encoder, through FluidAudio. English. Partials arrive while the
/// listener speaks. Chosen first for 6 GB: about 120M parameters, so it is the small streaming model; the 0.6B
/// Nemotron and Unified streaming variants are the next sizes up.
public final class ParakeetEOUTranscriber: Transcriber, @unchecked Sendable {
    public static let stageID = "parakeet-eou"
    public static var descriptor: StageDescriptor {
        StageDescriptor(
            displayName: "Parakeet EOU 120M (streaming)",
            summary: "Streaming English transcription on device (Core ML via FluidAudio). Partials while the listener speaks.",
            params: [.choice("chunkMs", "Chunk size (ms)", "320", ["160", "320", "1280"])],
            needsModels: ["parakeet-eou-120m"])
    }
    let variant: StreamingModelVariant
    // One manager per chunk size, shared by every instance asking for it.
    static let slots: [String: ModelSlot<any StreamingAsrManager>] = [
        "160": ModelSlot("parakeet-eou-120m"), "320": ModelSlot("parakeet-eou-120m"), "1280": ModelSlot("parakeet-eou-120m")]
    let chunk: String
    public init(_ p: StageParams) {
        chunk = ["160", "320", "1280"].contains(p.string("chunkMs", "320")) ? p.string("chunkMs", "320") : "320"
        variant = chunk == "160" ? .parakeetEou160ms : chunk == "1280" ? .parakeetEou1280ms : .parakeetEou320ms
    }

    public func begin() async throws -> TranscriptionSession {
        let v = variant
        let mgr = try await Self.slots[chunk]!.get {
            let m = v.createManager()
            try await m.loadModels()
            return (m, { await m.cleanup() })
        }
        try await mgr.reset()
        return EOUSession(mgr)
    }

    final class EOUSession: TranscriptionSession, @unchecked Sendable {
        let mgr: any StreamingAsrManager
        let partials: AsyncStream<String>
        let cont: AsyncStream<String>.Continuation
        let input: AsyncStream<AVAudioPCMBuffer>
        let inputCont: AsyncStream<AVAudioPCMBuffer>.Continuation
        var worker: Task<Void, Never>?
        init(_ m: any StreamingAsrManager) {
            mgr = m
            (partials, cont) = AsyncStream<String>.makeStream()
            (input, inputCont) = AsyncStream<AVAudioPCMBuffer>.makeStream()
            let cont = cont, mgr = m, input = input
            Task { await mgr.setPartialTranscriptCallback { cont.yield($0) } }
            worker = Task {
                for await b in input {
                    do { try await mgr.appendAudio(b); try await mgr.processBufferedAudio() } catch { break }
                }
            }
        }
        func append(_ frame: AudioFrame) {
            if let b = pcm(samples16k(frame), rate: 16000) { inputCont.yield(b) }
        }
        func finish() async throws -> String {
            inputCont.finish()
            await worker?.value
            let text = try await mgr.finish()
            cont.finish()
            return text.trimmingCharacters(in: .whitespacesAndNewlines)
        }
        func cancel() { inputCont.finish(); worker?.cancel(); cont.finish() }
    }
}

// MARK: Parakeet TDT 0.6B, batch at the end of the turn

/// Parakeet TDT 0.6B v2 (English) as an offline pass over the turn's audio when it ends. More accurate than the
/// streaming 120M, about 480 MB, and no partials. v2 is the English-only build; `model` selects v3 (25 languages),
/// ultra or redux (smaller, iOS 18+).
public final class ParakeetTDTTranscriber: Transcriber, @unchecked Sendable {
    public static let stageID = "parakeet-tdt"
    public static var descriptor: StageDescriptor {
        StageDescriptor(
            displayName: "Parakeet TDT 0.6B (batch)",
            summary: "Offline English transcription when the turn ends (Core ML via FluidAudio). No partials.",
            params: [.choice("model", "Model", "v2", ["v2", "v3", "ultra", "redux"])],
            needsModels: ["parakeet-tdt-0.6b-v2"])
    }
    static let slot = ModelSlot<AsrManager>("parakeet-tdt-0.6b-v2")
    let version: AsrModelVersion
    public init(_ p: StageParams) {
        switch p.string("model", "v2") { case "v3": version = .v3; case "ultra": version = .ultra; case "redux": version = .redux; default: version = .v2 }
    }
    public func begin() async throws -> TranscriptionSession {
        let v = version
        let m = try await Self.slot.get {
            let models = try await AsrModels.downloadAndLoad(version: v)
            let mgr = AsrManager(config: .default)
            try await mgr.loadModels(models)
            return (mgr, { await mgr.cleanup() })
        }
        return TDTSession(m)
    }
    final class TDTSession: TranscriptionSession, @unchecked Sendable {
        let mgr: AsrManager
        let partials: AsyncStream<String>
        let cont: AsyncStream<String>.Continuation
        let lock = NSLock()
        var samples: [Float] = []
        init(_ m: AsrManager) { mgr = m; (partials, cont) = AsyncStream<String>.makeStream() }
        func append(_ frame: AudioFrame) { let s = samples16k(frame); lock.lock(); samples.append(contentsOf: s); lock.unlock() }
        func finish() async throws -> String {
            lock.lock(); let s = samples; lock.unlock()
            cont.finish()
            guard s.count > 1600 else { return "" }
            var state = TdtDecoderState.make(decoderLayers: await mgr.decoderLayerCount)
            let r = try await mgr.transcribe(s, decoderState: &state)
            return r.text.trimmingCharacters(in: .whitespacesAndNewlines)
        }
        func cancel() { cont.finish() }
    }
}
