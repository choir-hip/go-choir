import Foundation

// What voice needs from the network, as jobs with their options, for shared code to show before any byte is fetched (owner, 2026-11-23/24: the prompt names the models with the true per-file sizes, read from the model list
// (ModelList) and never typed into a string; consent comes before any byte; the person may choose per job, with one option marked as the recommendation for THIS phone). Groups, as the download step offers them:
// "voice" (the speaking voice: what a listener who only LISTENS needs first), "voice-rest" (hearing you, knowing when you speak, the small language model, knowing your voice), "search".
public struct OfferOption: Sendable, Equatable {
    public var id: String, model: String
    public var bytes: Int64, recommended: Bool, ready: Bool
    /// A limit the person should know about before choosing it (a licence limit, from the candidate table); "" when there is none.
    public var note: String = ""
    /// The credit its licence asks the app to show (CC BY 4.0 and the like), from the list entry; "" when none.
    public var credit: String = ""
    public var sizeLabel: String { SetupCatalog.sizeLabel(bytes) }
    public var dict: [String: Any] { ["id": id, "model": model, "bytes": bytes, "sizeLabel": sizeLabel, "recommended": recommended, "ready": ready, "note": note, "credit": credit] }
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
    public static let jobIDs = ["transcriber", "bounds", "voice", "decision", "responder", "speaker", "search"]
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
    /// `ready`: job -> the id on the phone and passed its check. Every voice-setup stage that downloads is a job here, the quick-decision model too: what the first-run press fetches is exactly these jobs' chosen options and nothing else.
    public static func jobs(profile: DeviceProfile, list: ModelList = .builtIn(), ready: [String: String] = [:]) -> [OfferJob] {
        var out: [OfferJob] = []
        for j in jobIDs {
            var opts: [(id: String, label: String, bytes: Int64, fits: Bool, note: String, credit: String)] = []
            if let role = ConfigRole.allCases.first(where: { $0.stage.rawValue == j }) {
                let all = list.models.filter { $0.job == role.rawValue }.sorted { $0.rank > $1.rank }
                opts = all.map { ($0.id, $0.label, bytes($0, role: role), $0.isRecommended(machine: profile.chip, memoryGB: profile.memoryGB), CandidateTable.candidate($0.id, role)?.licenseLimit ?? "", $0.credit ?? "") }
            } else if j == "speaker" {
                opts = [(speakerID, "Your voice, kept on this phone", Int64(WeSpeakerEmbedder.files.reduce(0) { $0 + $1.size }), true, "", WeSpeakerEmbedder.attribution)]
            } else if j == "search" {
                opts = [(KnownModels.embeddingGemma2.id, "EmbeddingGemma 2", KnownModels.embeddingGemma2.bytes, true, "Google's Gemma terms apply to this model: its use must follow the Gemma Prohibited Use Policy (https://ai.google.dev/gemma/prohibited_use_policy).", "")]
            }
            let opts2 = opts.filter { $0.bytes > 0 }
            if opts2.isEmpty { continue }
            let rec = opts2.first { $0.fits }?.id
            out.append(OfferJob(job: j, group: group(j), host: host,
                                options: opts2.map { OfferOption(id: $0.id, model: $0.label, bytes: $0.bytes, recommended: $0.id == rec, ready: ready[j] == $0.id, note: $0.note, credit: $0.credit) }))
        }
        return out
    }

    /// The setup candidate for a pick the ladder does not carry (a row of the list that is not on a stage's ladder): built from the list's own table row.
    public static func candidate(job: String, id: String) -> SetupCandidate? {
        guard let st = SetupStage(rawValue: job), let role = ConfigRole.allCases.first(where: { $0.stage == st }), let c = CandidateTable.candidate(id, role) else { return nil }
        return SetupCandidate(id: c.id, label: c.label, bytes: c.bytes, minMemoryGB: Double(c.minTier.rawValue), kitStageID: c.kitStageID, role: role == .responder || role == .decision ? role.rawValue : "", needsMB: c.residentMB)
    }

    /// What a first-run press runs: the stages shared code asked for, each on the model the person was shown and chose (`picks`: stage -> model id). A pick that cannot be built here (a list row this build has no fetch for) is NOT run: the stage
    /// would otherwise fetch its own ladder's model, one the person was never shown. A stage with no pick runs on its own ladder.
    public static func plan(stages: [String], picks: [String: String]) -> (run: Set<SetupStage>, chosen: [SetupStage: SetupCandidate]) {
        var run = Set<SetupStage>(), chosen: [SetupStage: SetupCandidate] = [:]
        for name in stages {
            guard let st = SetupStage(rawValue: name) else { continue }
            if let id = picks[name] {
                guard let c = SetupCatalog.ladder(st).first(where: { $0.id == id }) ?? candidate(job: name, id: id) else { continue }
                chosen[st] = c
            }
            run.insert(st)
        }
        return (run, chosen)
    }

    /// job -> the model id that is READY on this phone: the best-ranked list entry for that job the one readiness record calls ready (`Voices.readiness`, written only after voice setup's check passed). The page reads what the engine reads;
    /// a floor is never in the list, so a stage that fell to its built-in floor has no entry here.
    public static func readyChoices(list: ModelList = .builtIn(), isReady: (String) -> Bool) -> [String: String] {
        var out: [String: String] = [:]
        for role in ConfigRole.allCases {
            if let e = list.models.filter({ $0.job == role.rawValue }).sorted(by: { $0.rank > $1.rank }).first(where: { isReady($0.id) }) { out[role.stage.rawValue] = e.id }
        }
        return out
    }
    /// The stages that have a ready model.
    public static func readyStages(choices: [String: String]) -> Set<String> { Set(choices.keys) }

    /// The setup's own record of the last run (rows and fallbacks), for merging a later partial run; readiness is NOT read from it.
    public static func readRecord(_ file: URL) -> SetupRecord? {
        guard let d = try? Data(contentsOf: file) else { return nil }
        return try? JSONDecoder().decode(SetupRecord.self, from: d)
    }
}
