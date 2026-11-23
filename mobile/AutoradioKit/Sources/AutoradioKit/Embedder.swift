import Foundation

// On-device embeddings (docs/reference/EMBEDDINGGEMMA.md). The model runs in AutoradioModels (llama.cpp, the official GGUF); what is kept of the
// vectors (the store, the work list, the job, the fusion) is shared code (AutoradioLab/search.ts, retrieval-run.ts, which builds the prompt strings
// too). Nothing here talks to a server. The prompt strings below serve the model's own smoke test and the device trial.

/// The prompt strings of EmbeddingGemma 2, verbatim from the model card's config.json.
public enum EmbedPrompt {
    public static func document(title: String?, text: String) -> String {
        let t = (title ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        return "title: \(t.isEmpty ? "none" : t) | text: \(text)"
    }
    public static func query(_ q: String) -> String { "task: search result | query: \(q)" }
}

/// A model that turns finished prompt strings into unit vectors. `engine` names the model file and runtime; vectors made by different
/// engines are not interchangeable, so a store records the engine and starts again when it changes.
public protocol Embedder: Sendable {
    var engine: String { get }
    var dimension: Int { get }
    func embed(_ prompts: [String]) async throws -> [[Float]]
}

public enum EmbedderError: Error, Equatable { case unavailable(String) }

/// An embedder that can also embed one short query at once, without waiting on an async hop (the retriever runs inside a synchronous call).
public protocol QueryEmbedding: Sendable { func embedQuery(_ text: String) -> [Float]? }
