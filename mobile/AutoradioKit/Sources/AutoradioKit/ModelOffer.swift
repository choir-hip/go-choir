import Foundation

// What voice needs from the network, as a list shared code can show before any byte is fetched (owner, 2026-11-23: the hearing and voice models are not optional; the files stay on Hugging Face; the prompt names the whole set
// with the true per-file sizes, read from the model list and never typed into a string; consent comes before any byte). The names and sizes are `SetupCatalog.ladder` (the first non-floor candidate of each stage, the one setup
// downloads first on a phone that fits it) and `KnownModels.embeddingGemma2` (search), so a newer model in those lists is a new row with its own size, and the same consented download.
// The three stages a listener who only LISTENS needs first: the speaking voice (group "voice"). The rest (group "voice-rest": hearing you, knowing when you speak, the small language model) is asked for when voice is turned on.
public struct OfferRow: Sendable, Equatable {
    public var stage: String, name: String, model: String, id: String, group: String, host: String
    public var bytes: Int64, ready: Bool
    public var sizeLabel: String { SetupCatalog.sizeLabel(bytes) }
    public var dict: [String: Any] { ["stage": stage, "name": name, "model": model, "id": id, "group": group, "host": host, "bytes": bytes, "sizeLabel": sizeLabel, "ready": ready] }
}

public enum ModelOffer {
    /// Where every file comes from: the pinned GGUF files by their own address (KnownModels), Parakeet, Silero and Kokoro through the FluidAudio library, which fetches from Hugging Face (README, "On-device models").
    public static let host = "Hugging Face"

    /// The stages that a model stands behind: the downloads setup would make first on this device (the floors are built in and not rows), grouped as above, plus the search model.
    public static func rows(profile: DeviceProfile, smaller: Bool = false, ready: Set<String> = []) -> [OfferRow] {
        let plan = SetupCatalog.plan(for: profile, smaller: smaller)
        var out: [OfferRow] = []
        var seen = Set<String>()
        for s in [SetupStage.voice, .transcriber, .bounds, .responder] {
            guard let c = plan[s]?.first, c.bytes > 0, seen.insert(c.id).inserted else { continue }
            out.append(OfferRow(stage: s.rawValue, name: s.title, model: c.label, id: c.id, group: s == .voice ? "voice" : "voice-rest", host: host, bytes: c.bytes, ready: ready.contains(s.rawValue)))
        }
        let e = KnownModels.embeddingGemma2
        out.append(OfferRow(stage: "search", name: "Search on this phone", model: "EmbeddingGemma 2", id: e.id, group: "search", host: host, bytes: e.bytes, ready: ready.contains("search")))
        return out
    }

    /// The stages whose chosen model passed its check on this phone (a stage that fell to its built-in floor is not ready for this purpose), from the record setup keeps (voice-setup.json).
    public static func readyStages(record: SetupRecord?) -> Set<String> {
        guard let record else { return [] }
        var out = Set<String>()
        for i in record.items {
            guard let s = SetupStage(rawValue: i.stage) else { continue }
            // A choice on the stage's own ladder is ready when it is not that ladder's built-in floor; a choice the configurator made under another id is ready unless its stage is one of the built-in ones.
            if let c = SetupCatalog.ladder(s).first(where: { $0.id == i.chosen }) { if c.bytes > 0 { out.insert(i.stage) } }
            else if !builtIn.contains(i.kitStageID) { out.insert(i.stage) }
        }
        return out
    }

    private static let builtIn: Set<String> = ["energy-vad", "apple-speech", "avspeech", "choir-system1"]

    public static func readRecord(_ file: URL) -> SetupRecord? {
        guard let d = try? Data(contentsOf: file) else { return nil }
        return try? JSONDecoder().decode(SetupRecord.self, from: d)
    }
}

/// The hearing and voice stages setup chose, kept across launches (on this phone, not the account: the models are on the phone). Before 2026-11-23 only the voice was kept (voice-choice.json); the recogniser and the
/// speech bounds went back to Apple's recogniser and the energy detector at every launch (the listener configuration is a fixed list, ChoirApp.swift). Applied by `LabRuntime.applyKeptPicks()` once the stages are registered.
public final class StagePicksStore: @unchecked Sendable {
    private let file: URL
    public init(file: URL) { self.file = file }
    /// StageKind raw value -> stage id.
    public func load() -> [String: String] {
        guard let d = try? Data(contentsOf: file), let o = try? JSONDecoder().decode([String: String].self, from: d) else { return [:] }
        return o
    }
    /// Only the hearing kinds are kept here (the voice has its own store, VoiceChoiceStore).
    public static let kinds: [StageKind] = [.transcriber, .speechBounds]
    public func save(_ choices: [StageKind: String]) {
        var cur = load()
        for k in Self.kinds { if let id = choices[k], !id.isEmpty { cur[k.rawValue] = id } }
        if let d = try? JSONEncoder().encode(cur) { try? FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true); try? d.write(to: file, options: .atomic) }
    }
}
