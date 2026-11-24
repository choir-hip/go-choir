import Foundation

// What voice needs from the network, as jobs with their options, for shared code to show before any byte is fetched (owner, 2026-11-23/24: the prompt names the models with the true per-file sizes, read from the model list
// (ModelList) and never typed into a string; consent comes before any byte; the person may choose per job, with one option marked as the recommendation for THIS phone). Groups, as the download step offers them:
// "voice" (the speaking voice: what a listener who only LISTENS needs first), "voice-rest" (hearing you, knowing when you speak, the small language model, knowing your voice), "search".
public struct OfferOption: Sendable, Equatable {
    public var id: String, model: String
    public var bytes: Int64, recommended: Bool, ready: Bool
    public var sizeLabel: String { SetupCatalog.sizeLabel(bytes) }
    public var dict: [String: Any] { ["id": id, "model": model, "bytes": bytes, "sizeLabel": sizeLabel, "recommended": recommended, "ready": ready] }
}
public struct OfferJob: Sendable, Equatable {
    public var job: String, group: String, host: String
    public var options: [OfferOption]
    public var dict: [String: Any] { ["job": job, "group": group, "host": host, "options": options.map { $0.dict }] }
}

public enum ModelOffer {
    /// Where every file comes from: the pinned GGUF files by their own address (KnownModels), the audio models through the FluidAudio library, the voice-print model by its pinned revision, all on Hugging Face.
    public static let host = "Hugging Face"

    /// The jobs the download step lists, in order, by the name shared code uses: the voice setup stages ("bounds" is the list's "vad") and the two files that are not voice-setup stages. (No turn-detector job: the list has none yet.)
    public static let jobIDs = ["transcriber", "bounds", "voice", "responder", "speaker", "search"]
    /// The voice-print model's id (WeSpeaker, about 8 MB: the files' own sizes summed).
    public static let speakerID = "wespeaker-resnet34"

    /// Which group of the download step a job is in.
    public static func group(_ job: String) -> String { job == "voice" ? "voice" : job == "search" ? "search" : "voice-rest" }

    /// What the list's entry weighs: its pinned files when it has them, else the candidate table's size (FluidAudio fetches those itself).
    static func bytes(_ e: ModelListEntry, role: ConfigRole) -> Int64 {
        if let l = SetupCatalog.ladder(role.stage).first(where: { $0.id == e.id }), l.bytes > 0 { return l.bytes }       // what setup downloads for it, with its companions
        return e.totalBytes > 0 ? e.totalBytes : (CandidateTable.candidate(e.id, role)?.bytes ?? 0)
    }

    /// The jobs, each with every option the list holds for it (best-ranked first) and the one marked recommended for this phone: the first entry whose device rule fits it (`ModelListEntry.isRecommended`, the machine
    /// identifier and the memory). A row added to the list (a newer model, another recogniser) is an option here with no change to this code. The floors are not in the list. A job whose entries fit no rule lists them and recommends none.
    /// `ready`: job -> the id on the phone and passed its check. The decision job shares the responder's file and is not listed.
    public static func jobs(profile: DeviceProfile, list: ModelList = .builtIn(), ready: [String: String] = [:]) -> [OfferJob] {
        var out: [OfferJob] = []
        for j in jobIDs {
            var opts: [(id: String, label: String, bytes: Int64, fits: Bool)] = []
            if let role = ConfigRole.allCases.first(where: { $0.stage.rawValue == j }), j != "decision" {
                let all = list.models.filter { $0.job == role.rawValue }.sorted { $0.rank > $1.rank }
                opts = all.map { ($0.id, $0.label, bytes($0, role: role), $0.isRecommended(machine: profile.chip, memoryGB: profile.memoryGB)) }
            } else if j == "speaker" {
                opts = [(speakerID, "Your voice, kept on this phone", Int64(WeSpeakerEmbedder.files.reduce(0) { $0 + $1.size }), true)]
            } else if j == "search" {
                opts = [(KnownModels.embeddingGemma2.id, "EmbeddingGemma 2", KnownModels.embeddingGemma2.bytes, true)]
            }
            let opts2 = opts.filter { $0.bytes > 0 }
            if opts2.isEmpty { continue }
            let rec = opts2.first { $0.fits }?.id
            out.append(OfferJob(job: j, group: group(j), host: host,
                                options: opts2.map { OfferOption(id: $0.id, model: $0.label, bytes: $0.bytes, recommended: $0.id == rec, ready: ready[j] == $0.id) }))
        }
        return out
    }

    /// The setup candidate for a pick the ladder does not carry (a row of the list that is not on a stage's ladder): built from the list's own table row.
    public static func candidate(job: String, id: String) -> SetupCandidate? {
        guard let st = SetupStage(rawValue: job), let role = ConfigRole.allCases.first(where: { $0.stage == st }), let c = CandidateTable.candidate(id, role) else { return nil }
        return SetupCandidate(id: c.id, label: c.label, bytes: c.bytes, minMemoryGB: Double(c.minTier.rawValue), kitStageID: c.kitStageID, role: role == .responder ? "responder" : "", needsMB: c.residentMB)
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

    /// job -> the model id on the phone: the stage's recorded choice when it is not that stage's built-in floor.
    public static func readyChoices(record: SetupRecord?) -> [String: String] {
        guard let record else { return [:] }
        let ok = readyStages(record: record)
        var out: [String: String] = [:]
        for i in record.items where ok.contains(i.stage) { out[i.stage] = i.chosen }
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
