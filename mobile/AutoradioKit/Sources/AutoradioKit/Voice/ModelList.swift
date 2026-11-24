import Foundation

// THE MODEL LIST AS ONE DATA STRUCTURE (owner, 2026-11-24: "we should be downloading the model list from the server"; "recommendations per
// device can be variable for every model type").
//
// Each entry is a model for one job (speech detection, recognition, voice, ...): where it comes from (repository and the pinned revision),
// the files with their size and SHA-256 when the store fetches them itself, the licence and the credit the licence asks for, and which devices
// it is recommended for. The list decodes from JSON and a copy is built in (`builtIn`, made from the tables this build already carries), so a
// phone with no network, or with a server that is down, has the same list it shipped with. NOT BUILT: the server route that serves it, the
// fetch, and the choice reading this list instead of `CandidateTable` (today the pipeline still reads `CandidateTable`; this is the shape the
// served list will take, and `builtIn` is derived from that table so the two cannot disagree).
//
// WAYS THIS CAN FAIL (each has a test in ModelListTests):
//  1. A newer server sends a job this build does not know, or a field it has not heard of: that entry is skipped (or the field ignored), the
//     rest of the list is used. One bad entry never empties the list.
//  2. The JSON is not a list at all: decoding throws, and the caller keeps the built-in copy.
//  3. A file's digest is not 64 lower-case hex digits, or its path is listed twice, or the revision is not 40 hex digits: the entry is dropped
//     as invalid (a model the store could not verify is never offered).
//  4. A device matches no rule: it is not recommended the model, but the entry stays in the list (the person may still choose it).
public struct ModelListEntry: Codable, Equatable, Sendable {
    public struct File: Codable, Equatable, Sendable { public var path: String; public var bytes: Int64; public var sha256: String }
    /// Who a model is recommended for. A device matches a rule when its machine identifier starts with one of `machinePrefixes` (empty: any) and
    /// it has at least `minMemoryGB`. An entry with no rules is recommended for every device.
    public struct DeviceRule: Codable, Equatable, Sendable {
        public var machinePrefixes: [String]
        public var minMemoryGB: Double
        public var note: String?
        public init(machinePrefixes: [String] = [], minMemoryGB: Double = 0, note: String? = nil) { self.machinePrefixes = machinePrefixes; self.minMemoryGB = minMemoryGB; self.note = note }
        public func matches(machine: String, memoryGB: Double) -> Bool {
            memoryGB >= minMemoryGB && (machinePrefixes.isEmpty || machinePrefixes.contains { machine.hasPrefix($0) })
        }
    }
    public var id: String
    /// The job: a `ConfigRole` raw value ("vad", "transcriber", "voice", "decision", "responder").
    public var job: String
    public var label: String
    public var repository: String
    /// The repository's commit the files were read at; nil for a model a library fetches itself at the version this build pins.
    public var revision: String?
    /// Nil or empty: the store does not fetch it (FluidAudio does, and checks it).
    public var files: [File]?
    public var license: String
    public var credit: String?
    public var recommendedFor: [DeviceRule]
    /// Higher is better within a job; the same number `CandidateTable` ranks by.
    public var rank: Int

    public func isRecommended(machine: String, memoryGB: Double) -> Bool {
        recommendedFor.isEmpty || recommendedFor.contains { $0.matches(machine: machine, memoryGB: memoryGB) }
    }
    public var totalBytes: Int64 { (files ?? []).reduce(0) { let (n, o) = $0.addingReportingOverflow($1.bytes); return o ? Int64.max : n } }

    /// What the store needs to fetch a folder of pinned files; nil when the entry has no pinned files or revision.
    public func bundleSpec(folder: String, copies: [[String]] = []) -> ModelBundleSpec? {
        guard let revision, let files, !files.isEmpty else { return nil }
        return ModelBundleSpec(id: id, repo: repository, commit: revision, files: files.map { .init(path: $0.path, bytes: $0.bytes, sha256: $0.sha256) },
                               folder: folder, license: license, credit: credit ?? "", copies: copies)
    }

    var isValid: Bool {
        guard !id.isEmpty, ConfigRole(rawValue: job) != nil else { return false }
        if let revision, revision.count != 40 || !revision.allSatisfy({ $0.isHexDigit && !$0.isUppercase }) { return false }
        let fs = files ?? []
        if Set(fs.map(\.path)).count != fs.count || !fs.allSatisfy({ ModelBundleSpec.isSafeRelativePath($0.path) }) { return false }   // a served path never leaves the model's folder (S1)
        if !fs.isEmpty, revision == nil { return false }
        // Served text must be fetchable without trapping and within what a phone may be asked to hold (a served size is never trusted unbounded).
        if !fs.isEmpty, !ModelBundleSpec.isValidRepository(repository) { return false }
        if fs.count > ModelBundleSpec.maxFiles || totalBytes > ModelBundleSpec.maxModelBytes { return false }
        return fs.allSatisfy { $0.bytes > 0 && $0.bytes <= ModelBundleSpec.maxFileBytes && $0.sha256.count == 64 && $0.sha256.allSatisfy { $0.isHexDigit && !$0.isUppercase } }
    }
}

public struct ModelList: Codable, Equatable, Sendable {
    public var version: Int
    public var models: [ModelListEntry]
    public init(version: Int, models: [ModelListEntry]) { self.version = version; self.models = models }

    /// Entries that are valid, for this build's jobs; the rest are skipped (1 and 3 above).
    public init(from decoder: Decoder) throws {
        struct Lenient: Decodable { var entry: ModelListEntry?; init(from d: Decoder) throws { entry = try? ModelListEntry(from: d) } }
        let c = try decoder.container(keyedBy: Keys.self)
        version = try c.decode(Int.self, forKey: .version)
        models = try c.decode([Lenient].self, forKey: .models).compactMap(\.entry).filter(\.isValid)
    }
    enum Keys: String, CodingKey { case version, models }

    public static func decode(_ json: Data) throws -> ModelList { try JSONDecoder().decode(ModelList.self, from: json) }
    public func encoded() throws -> Data { let e = JSONEncoder(); e.outputFormatting = [.sortedKeys, .prettyPrinted]; return try e.encode(self) }

    /// The best-ranked entries for a job, best first, those recommended for the device ahead of the rest.
    public func entries(job: ConfigRole, machine: String, memoryGB: Double) -> [ModelListEntry] {
        let all = models.filter { $0.job == job.rawValue }.sorted { $0.rank > $1.rank }
        return all.filter { $0.isRecommended(machine: machine, memoryGB: memoryGB) } + all.filter { !$0.isRecommended(machine: machine, memoryGB: memoryGB) }
    }

    /// The copy built into this app, made from the tables the build carries (so it cannot disagree with them).
    public static func builtIn(table: [ModelCandidate] = CandidateTable.all, bundles: [ModelBundleSpec] = KnownModels.bundles) -> ModelList {
        let entries: [ModelListEntry] = table.filter { !$0.isFloor }.map { c in
            let b = bundles.first { $0.id == c.id }
            return ModelListEntry(
                id: c.id, job: c.role.rawValue, label: c.label, repository: b?.repo ?? c.source.replacingOccurrences(of: "hf:", with: ""), revision: b?.commit,
                files: b?.files.map { .init(path: $0.path, bytes: $0.bytes, sha256: $0.sha256) }, license: b?.license ?? c.license, credit: b?.credit ?? c.credit ?? KnownModels.managed.first { $0.id == c.id }?.credit,
                recommendedFor: [.init(minMemoryGB: Double(c.minTier.rawValue) - 1, note: c.minTier.label + " and up")], rank: c.qualityRank)
        }
        return ModelList(version: 1, models: entries.filter(\.isValid))
    }
}
