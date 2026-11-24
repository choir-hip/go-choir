import Foundation
import FluidAudio
import AutoradioKit

// Fluid Speech Mini by FluidVoice (CC BY 4.0), the recogniser the owner chose and built in Mike (owner, 2026-11-24: "we should be using
// fluid speech mini"). English only, batch (no partials), 16 kHz mono, 15 s windows (FluidAudio chunks longer audio). A distilled
// Parakeet TDT 0.6B v3 derivative as Core ML; FluidAudio loads it as `.ultra`. The port of Mike's `FluidSpeechMini.swift`
// (ios/Sources/MikeSpeechPlugin): it keeps the three things that file settled.
//   1. The model is read from a folder on the phone and NEVER downloaded here (`AsrModels.loadLocal`): the download is the model store's,
//      after the person agreed (`ModelStore.ensureBundle`, called by voice setup). A stage asked to begin without the files throws `unavailable`.
//   2. The model is warm: Core ML specialises a model for the Neural Engine on its first prediction, not at load, and that first prediction
//      is the slow part, so the load runs the model once on a second of silence before anyone is waiting for a transcript.
//   3. One pass over the speech-bounded audio gives the final transcript: the session keeps the turn's samples and `finish()` runs the model once.
//      Audio shorter than the model's one-second minimum is padded with silence.
//
// WAYS THIS CAN FAIL (each has a test in FluidSpeechMiniTests unless it says "device"):
//  1. The files are not on the phone: `begin()` throws `unavailable` at once (no download, no hang); the composite stage then speaks with what it has.
//  2. A file is missing or the wrong size (an interrupted download): the folder is not "present" (ModelStore.bundleDirectoryIfPresent); same as 1.
//  3. The joint network is under the repository's name only: FluidAudio looks for `JointDecisionv3.mlmodelc`; the store makes the copy, and
//     presence includes it.
//  4. A cough, a door: audio with no words returns "" and the engine discards the turn (HandsFree H2).
//  5. Two turns at once load two copies of 250 MB: one `ModelSlot` holds the one loaded manager; the second caller waits for the first load.
//  6. The first transcript is slow: the slot's load runs the warm-up (device: measured on the phone).
//  7. Turn longer than 15 s: FluidAudio's own chunking (Mike's spike: 2.05% WER over 31 minutes through upstream FluidAudio; the altic-dev
//     fork dropped spans, which is why Mike uses upstream and so do we).
public final class FluidSpeechMiniTranscriber: Transcriber, @unchecked Sendable {
    public static let stageID = "fluid-speech-mini"
    public static var descriptor: StageDescriptor {
        StageDescriptor(
            displayName: "Fluid Speech Mini (batch)",
            summary: "Final English transcript over the turn's audio when it ends, on the phone (Core ML via FluidAudio). No partials.",
            params: [], needsModels: [KnownModels.fluidSpeechMini.id])
    }
    static let slot = ModelSlot<AsrManager>(KnownModels.fluidSpeechMini.id)
    /// Where the files are: the store's folder when it is whole, nil when it is not. Tests point it at a folder of their own.
    let locate: @Sendable () async -> URL?
    public init(_ p: StageParams) { locate = { await ModelHub.store.bundleDirectoryIfPresent(KnownModels.fluidSpeechMini) } }
    init(directory: @escaping @Sendable () async -> URL?) { locate = directory }

    /// The loaded recogniser: from the folder on the phone only, warm. Throws `unavailable` when the folder is not whole.
    func manager() async throws -> AsrManager {
        guard let dir = await locate() else { throw LabError.unavailable("Fluid Speech Mini is not on this phone") }
        return try await Self.slot.get {
            let models = try AsrModels.loadLocal(from: dir, version: .ultra)
            let mgr = AsrManager(config: .default)
            try await mgr.loadModels(models)
            var state = TdtDecoderState.make(decoderLayers: await mgr.decoderLayerCount)
            _ = try? await mgr.transcribe([Float](repeating: 0, count: 16_000), decoderState: &state)      // the warm-up (2 above)
            return (mgr, { await mgr.cleanup() })
        }
    }

    /// One pass over 16 kHz mono samples. Shorter than a second is padded with silence (the model's minimum).
    static func transcribe(_ samples: [Float], with mgr: AsrManager) async throws -> String {
        var audio = samples
        if audio.count < 16_000 { audio.append(contentsOf: [Float](repeating: 0, count: 16_000 - audio.count)) }
        var state = TdtDecoderState.make(decoderLayers: await mgr.decoderLayerCount)
        return try await mgr.transcribe(audio, decoderState: &state).text.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    public func begin() async throws -> TranscriptionSession {
        Session(try await manager())            // fail now, not at the end of the turn, and have the model warm before the words are done
    }

    final class Session: TranscriptionSession, @unchecked Sendable {
        let partials: AsyncStream<String>
        let cont: AsyncStream<String>.Continuation
        private let mgr: AsrManager
        private let lock = NSLock()
        private var samples: [Float] = []
        init(_ m: AsrManager) { mgr = m; (partials, cont) = AsyncStream<String>.makeStream() }
        func append(_ frame: AudioFrame) { let s = samples16k(frame); lock.lock(); samples.append(contentsOf: s); lock.unlock() }
        func finish() async throws -> String {
            lock.lock(); let s = samples; lock.unlock()
            cont.finish()
            // Less than a tenth of a second is not speech: nothing to transcribe.
            guard s.count >= 1_600 else { return "" }
            return try await FluidSpeechMiniTranscriber.transcribe(s, with: mgr)
        }
        func cancel() { cont.finish() }
    }
}
