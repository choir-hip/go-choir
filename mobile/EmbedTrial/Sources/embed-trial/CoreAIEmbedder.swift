import Foundation
import EmbeddingGemma2CoreAI

/// Route B: Everpaper's Core AI port through its own Swift runner. The prompt prefix is already in the text, so no `kind`.
final class CoreAIEmbedder: TrialEmbedder {
    let name = "coreai-everpaper-v1.2"
    let model: EmbeddingGemma2
    let loadMs: Double
    init(bundle: URL, compute: ComputePreference) async throws {
        let t0 = nowMs()
        model = try await EmbeddingGemma2(bundle: bundle, compute: compute)
        loadMs = nowMs() - t0
    }
    func prewarm() async throws -> Double { try await model.prewarm() }
    func embed(_ text: String) async throws -> (vec: [Float], tokens: Int) {
        let ids = model.tokenize(text, kind: nil)
        return (try await model.embed(ids: ids), ids.count)
    }
}
