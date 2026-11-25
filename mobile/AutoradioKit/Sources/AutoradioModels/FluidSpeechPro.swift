import Foundation
import FluidAudio
import AutoradioKit

// Fluid Speech Pro by FluidVoice (CC BY 4.0): the full-size sibling of Fluid Speech Mini (owner, 2026-11-24: "fluid speech pro is better than
// parakeet. so fluid speech mini or pro is the way to go rn"; "the recognizers should be part of the onboarding, with the recommended one ...
// depending on device size"). A multilingual Parakeet TDT 0.6B v3 fine-tune with one extra encoder block and an 8-bit encoder, 652 MB.
//
// It is Mini's stage with Pro's files and its own slot: the same three rules (never downloads, warm at load, one pass over the speech-bounded
// audio) and the same session. The loader is the pinned FluidAudio 0.18.3's `.ultra`: Pro has the v3 contract (Preprocessor, Encoder, Decoder,
// JointDecision under the v3 name, `parakeet_vocab.json`, blank 8192, 15 s windows). The vendor's example uses `.fluidParakeetPro`, which exists
// only in the altic-dev fork of FluidAudio; that fork's change (9e15ef9) only adds the enum case and routes it to the same v3 decoder, so no fork
// and no change of pin is needed. (The fork also dropped spans over long audio, see FluidSpeechMini.swift, which is why we stay on upstream.)
//
// WAYS THIS CAN FAIL: the same as Mini's (FluidSpeechMini.swift), plus
//  8. A phone with too little memory: Pro is never offered to it (`SetupCatalog.ladder`, `minMemoryGB`), and the stage does not guess.
public final class FluidSpeechProTranscriber: Transcriber, @unchecked Sendable {
    public static let stageID = "fluid-speech-pro"
    public static var descriptor: StageDescriptor {
        StageDescriptor(
            displayName: "Fluid Speech Pro (batch)",
            summary: "Final transcript over the turn's audio when it ends, on the phone (Core ML via FluidAudio), the larger and more accurate of the two Fluid Speech listeners. No partials.",
            params: [], needsModels: [KnownModels.fluidSpeechPro.id])
    }
    static let slot = ModelSlot<AsrManager>(KnownModels.fluidSpeechPro.id)
    let locate: @Sendable () async -> URL?
    public init(_ p: StageParams) { locate = { await ModelHub.store.bundleDirectoryIfPresent(KnownModels.fluidSpeechPro) } }
    init(directory: @escaping @Sendable () async -> URL?) { locate = directory }

    func manager() async throws -> AsrManager {
        guard let dir = await locate() else { throw LabError.unavailable("Fluid Speech Pro is not on this phone") }
        return try await Self.slot.get {
            let models = try AsrModels.loadLocal(from: dir, version: .ultra)
            let mgr = AsrManager(config: .default)
            try await mgr.loadModels(models)
            var state = TdtDecoderState.make(decoderLayers: await mgr.decoderLayerCount)
            _ = try? await mgr.transcribe([Float](repeating: 0, count: 16_000), decoderState: &state)      // the warm-up
            return (mgr, { await mgr.cleanup() })
        }
    }

    public func begin() async throws -> TranscriptionSession {
        FluidSpeechMiniTranscriber.Session(try await manager())
    }
}
