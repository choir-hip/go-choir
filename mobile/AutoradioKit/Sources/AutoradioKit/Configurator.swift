import Foundation

// THE CONFIGURATOR: picks, for THIS device, one model per role of the voice pipeline, for the WHOLE pipeline resident together (owner,
// 2026-11-11: the Liquid Apollo app ran LFM3 alone, not the voice pipeline plus that model). It replaces "the first candidate that passes in
// a fixed order": it plans configurations best-quality-first among those that could fit, downloads only what the plan needs, loads the whole
// configuration at once, MEASURES it on the device, and accepts the first configuration that leaves headroom and meets the speeds. Every
// number it decides with is named data (ConfigThresholds) and every measurement is recorded (ConfigurationRecord) so the thresholds can be
// tuned from real phones. All measurement happens at setup time; nothing here is a timer around a model call.
//
// WAYS THIS CAN FAIL (each has a test in ConfiguratorTests):
//  1. A phone below the floor (under about 6 GB): a plain "not supported" result, every role on its floor, nothing downloaded.
//  2. The plan is empty (budget below even the app's own base): all floors, said so.
//  3. A configuration does not fit once loaded (memory left under the headroom), is too slow (read, write, first sound, ASR speed) or the
//     phone is hot after a short sustained run: rejected with the numbers, the next configuration is tried, what was downloaded for the
//     rejected one and is not needed by the next is deleted.
//  4. A download fails part way (network, disk): the candidate is marked unavailable for this run, the plan continues without it; the partial
//     file stays so a re-run resumes (ModelStore uses HTTP Range); a re-run starts from the top and skips what is on disk.
//  5. A model that loads but errors when measured (a quant llama.cpp cannot run): the role's failure is the rejection reason.
//  6. The accepted configuration's weights are never deleted; every other downloaded candidate's weights are, at the end.
//  7. Two roles choose the same file (the decision and the responder): its weights count once, each role adds its own context.
//  8. The device reading is missing (the Mac, the simulator): the planner's own assumption for the memory class is used, and the record
//     says it was an assumption.

// MARK: Device

public struct DeviceProbe: Codable, Equatable, Sendable {
    public var model: String                // hw.machine, e.g. "iPhone15,3"
    public var chip: String                 // "A16 Bionic" when known
    public var physicalGB: Double
    public var availableMB: Double?         // os_proc_available_memory() at probe time; nil where there is no such call
    public var availableIsAssumption: Bool
    public var freeDiskGB: Double
    public var thermal: String
    public var osVersion: String
    public var tier: DeviceClass
    public var simulated: Bool
    public var supported: Bool { tier != .unsupported }
    public var unsupportedReason: String? {
        supported ? nil : String(format: "This phone has about %.0f GB of memory. The voice needs a 6 GB phone (iPhone 13 Pro or newer), so it is not supported here.", physicalGB)
    }

    /// The memory class from the memory the OS reports (a "6 GB" phone reports about 5.6 GB, an "8 GB" one about 7.4, a "12 GB" one about 11.4).
    public static func classify(physicalGB: Double) -> DeviceClass {
        if physicalGB >= 10.5 { return .tier12 }
        if physicalGB >= 7.0 { return .tier8 }
        if physicalGB >= 5.0 { return .tier6 }
        return .unsupported
    }

    /// What a process may use of a class's memory when the device cannot say. ASSUMPTION from forum measurements (about 6.1 GB of an 8 GB
    /// iPhone with the increased-memory-limit entitlement, 2.2 to 2.3 GB of a 4 GB one): 60% of physical. The on-device reading replaces it.
    public static func assumedAvailableMB(tier: DeviceClass) -> Double { Double(tier.rawValue) * 1024 * 0.6 }

    /// This device.
    public static func current(machine: String? = nil) -> DeviceProbe {
        var size = 0
        sysctlbyname("hw.machine", nil, &size, nil, 0)
        var buf = [CChar](repeating: 0, count: max(size, 1))
        sysctlbyname("hw.machine", &buf, &size, nil, 0)
        let id = machine ?? String(decoding: buf.prefix(while: { $0 != 0 }).map { UInt8(bitPattern: $0) }, as: UTF8.self)
        let gb = Double(ProcessInfo.processInfo.physicalMemory) / 1_073_741_824
        let home = URL(fileURLWithPath: NSHomeDirectory())
        let free = ((try? home.resourceValues(forKeys: [.volumeAvailableCapacityForImportantUsageKey]))?.volumeAvailableCapacityForImportantUsage).map { Double($0) / 1_073_741_824 } ?? 0
        let avail = DeviceReading.availableMemoryMB()
        let tier = classify(physicalGB: gb)
        return DeviceProbe(model: id, chip: chipName(id) ?? "", physicalGB: gb, availableMB: avail ?? assumedAvailableMB(tier: tier), availableIsAssumption: avail == nil,
                           freeDiskGB: free, thermal: DeviceReading.thermal(), osVersion: ProcessInfo.processInfo.operatingSystemVersionString, tier: tier, simulated: false)
    }

    /// A pretend phone of a memory class, for the Mac proof and tests: the memory budget is the PLANNER's, not the Mac's.
    public static func simulated(_ tier: DeviceClass, availableMB: Double? = nil) -> DeviceProbe {
        let gbs: Double
        switch tier { case .unsupported: gbs = 4.0; case .tier6: gbs = 5.6; case .tier8: gbs = 7.4; case .tier12: gbs = 11.4 }
        return DeviceProbe(model: "simulated-\(tier.rawValue)GB", chip: "simulated", physicalGB: gbs, availableMB: availableMB ?? assumedAvailableMB(tier: tier),
                           availableIsAssumption: true, freeDiskGB: 50, thermal: "nominal", osVersion: "simulated", tier: tier, simulated: true)
    }

    static func chipName(_ machine: String) -> String? {
        let table: [String: String] = ["iPhone14,2": "A15 Bionic", "iPhone14,3": "A15 Bionic", "iPhone14,4": "A15 Bionic", "iPhone14,5": "A15 Bionic", "iPhone14,7": "A15 Bionic", "iPhone14,8": "A15 Bionic",
            "iPhone15,2": "A16 Bionic", "iPhone15,3": "A16 Bionic", "iPhone15,4": "A16 Bionic", "iPhone15,5": "A16 Bionic", "iPhone16,1": "A17 Pro", "iPhone16,2": "A17 Pro",
            "iPhone17,1": "A18 Pro", "iPhone17,2": "A18 Pro", "iPhone17,3": "A18", "iPhone17,4": "A18"]
        return table[machine]
    }
}

// MARK: Thresholds: ALL the numbers the configurator decides with, in one place, each with where it came from

public struct ConfigThresholds: Codable, Equatable, Sendable {
    /// Memory the planner keeps free of the budget for the app, the system and spikes. The Mac runs showed a process growing 0.1 to 0.25 GB while generating.
    public var planningHeadroomMB = 600.0
    /// The app's own resident memory before any model. UNMEASURED on a phone: the first phone record will say.
    public var appBaseMB = 300.0
    /// The least memory that must still be available after everything is loaded. iOS ends a process at its limit (forum: about 2.2 to 2.3 GB on a 4 GB phone);
    /// a reply stream grows the process. 500 MB is a guess to tune from real phones.
    public var minHeadroomAfterLoadMB = 500.0
    /// Tokens per second the responder must write. Speech is about 2.5 to 3 words a second (about 4 tokens); 15 keeps it about 3 times ahead so a sentence is ready before the last one ends.
    public var responderMinWriteTPS = 15.0
    /// Tokens per second the responder must read: a 400-token page cold in about 3 s (a warm cache reads only the question). Mac runs: 900 to 2,700.
    public var responderMinReadTPS = 150.0
    /// First sound of a voice, ms. A voice that takes longer than about 1.5 s to its first sound makes the hand-over from the phone's voice awkward. Mac: Kokoro 243, Pocket 98.
    public var ttsMaxFirstSoundMs = 1500.0
    /// Speech recogniser speed as a fraction of real time: it must keep up with speech using half the compute. Mac: 0.04 to 0.18.
    public var asrMaxRealTimeFactor = 0.5
    /// A decision (System 1) must come in about 1.5 s. Mac: 58 ms.
    public var decisionMaxMs = 1500.0
    /// Thermal states that are acceptable after the sustained run. "serious" and "critical" mean the phone is already throttling.
    public var acceptedThermal = ["nominal", "fair"]
    /// How many configurations are tried before the built-in voice is taken. Each attempt downloads and measures; a phone that has turned down this many in a row
    /// (too hot, too slow, too little memory) is not going to accept the seventh. A count, not a timer. A guess to tune from real phones.
    public var maxAttempts = 6
    /// The sustained run is a fixed amount of work (this many rounds of a responder stream, a spoken sentence and a recognised clip), not a duration.
    public var sustainedRounds = 3
    public static let standard = ConfigThresholds()
}

// MARK: Configurations and the plan

public struct Configuration: Codable, Equatable, Sendable {
    /// role -> candidate id.
    public var picks: [String: String]
    public var score: Double
    public var estimatedMB: Double
    public var downloadBytes: Int64
    public func pick(_ r: ConfigRole) -> String? { picks[r.rawValue] }
    public var key: String { ConfigRole.allCases.compactMap { r in picks[r.rawValue].map { id in "\(r.rawValue)=\(id)" } }.joined(separator: " ") }
}

public enum ConfigPlanner {
    /// Memory of a set of picks resident together: each file's weights once (two roles on one file share them, each adds its own context), plus the app's base.
    public static func estimateMB(_ picks: [ConfigRole: ModelCandidate], thresholds: ConfigThresholds = .standard) -> Double {
        var seen = Set<String>(), total = thresholds.appBaseMB
        for r in ConfigRole.allCases {
            guard let c = picks[r] else { continue }
            if c.isFloor { continue }
            if seen.insert(c.id).inserted { total += c.residentMB } else { total += c.contextMB }
        }
        return total
    }

    /// Every configuration that could fit this device, best quality first. Ties go to the one that uses less memory. A floor-only configuration is last and always possible.
    public static func plan(device: DeviceProbe, thresholds: ConfigThresholds = .standard, table: [ModelCandidate] = CandidateTable.all, excluding: Set<String> = []) -> [Configuration] {
        guard device.supported else { return [] }
        let budget = (device.availableMB ?? DeviceProbe.assumedAvailableMB(tier: device.tier)) - thresholds.planningHeadroomMB
        let perRole: [[ModelCandidate]] = ConfigRole.allCases.map { r in
            CandidateTable.candidates(r, in: table).filter { ($0.isFloor || ($0.minTier <= device.tier && !excluding.contains($0.id))) }
        }
        var out: [Configuration] = []
        func walk(_ i: Int, _ picks: [ConfigRole: ModelCandidate]) {
            if i == ConfigRole.allCases.count {
                let mb = estimateMB(picks, thresholds: thresholds)
                guard mb <= budget else { return }
                let score = picks.reduce(0.0) { $0 + $1.key.weight * Double($1.value.qualityRank) }
                var seen = Set<String>(), dl: Int64 = 0
                for (_, c) in picks where !c.isFloor { if seen.insert(c.id).inserted { dl += c.bytes } }
                out.append(Configuration(picks: Dictionary(uniqueKeysWithValues: picks.map { ($0.key.rawValue, $0.value.id) }), score: score, estimatedMB: mb, downloadBytes: dl))
                return
            }
            for c in perRole[i] { var p = picks; p[ConfigRole.allCases[i]] = c; walk(i + 1, p) }
        }
        walk(0, [:])
        return out.sorted { a, b in a.score != b.score ? a.score > b.score : (a.estimatedMB != b.estimatedMB ? a.estimatedMB < b.estimatedMB : a.key < b.key) }
    }

    /// Whether a measurement of a configuration is good enough, and if not, why (in plain words, with the numbers).
    public static func verdict(_ m: ConfigMeasurement, config: Configuration, device: DeviceProbe, thresholds th: ConfigThresholds = .standard,
                               table: [ModelCandidate] = CandidateTable.all) -> (ok: Bool, reasons: [String]) {
        var why: [String] = []
        for (role, msg) in m.failedRoles.sorted(by: { $0.key < $1.key }) { why.append("\(role) failed: \(msg)") }
        let headroom = m.availableMBAfterLoad ?? ((device.availableMB ?? DeviceProbe.assumedAvailableMB(tier: device.tier)) - config.estimatedMB)
        if headroom < th.minHeadroomAfterLoadMB { why.append(String(format: "only %.0f MB would be left (need %.0f)", headroom, th.minHeadroomAfterLoadMB)) }
        let responder = config.pick(.responder).flatMap { CandidateTable.candidate($0, .responder, in: table) }
        if let r = responder, !r.isFloor {
            if let w = m.responderWriteTPS, w < th.responderMinWriteTPS { why.append(String(format: "the voice writes %.0f tokens a second (need %.0f)", w, th.responderMinWriteTPS)) }
            else if m.responderWriteTPS == nil, m.failedRoles["responder"] == nil { why.append("the voice's speed was not measured") }
            if let rd = m.responderReadTPS, rd < th.responderMinReadTPS { why.append(String(format: "the voice reads %.0f tokens a second (need %.0f)", rd, th.responderMinReadTPS)) }
        }
        let voice = config.pick(.voice).flatMap { CandidateTable.candidate($0, .voice, in: table) }
        if let v = voice, !v.isFloor, let f = m.ttsFirstSoundMs, f > th.ttsMaxFirstSoundMs { why.append(String(format: "the voice takes %.0f ms to its first sound (allowed %.0f)", f, th.ttsMaxFirstSoundMs)) }
        let asr = config.pick(.transcriber).flatMap { CandidateTable.candidate($0, .transcriber, in: table) }
        if let a = asr, !a.isFloor, let x = m.asrRealTimeFactor, x > th.asrMaxRealTimeFactor { why.append(String(format: "the listener runs at %.2f of real time (allowed %.2f)", x, th.asrMaxRealTimeFactor)) }
        if let d = m.decisionMs, d > th.decisionMaxMs { why.append(String(format: "a decision takes %.0f ms (allowed %.0f)", d, th.decisionMaxMs)) }
        if !th.acceptedThermal.contains(m.thermalAfter) { why.append("the phone is \(m.thermalAfter) after the run") }
        return (why.isEmpty, why)
    }
}

// MARK: Measurement and the record

public struct ConfigMeasurement: Codable, Equatable, Sendable {
    public var availableMBAfterLoad: Double?
    public var residentMB: Double?
    public var thermalBefore = "nominal", thermalAfter = "nominal"
    public var responderReadTPS: Double?, responderWriteTPS: Double?, responderWarmFirstWordMs: Double?
    public var ttsFirstSoundMs: Double?
    public var asrRealTimeFactor: Double?
    public var decisionMs: Double?
    public var vadRealTimeFactor: Double?
    public var loadMs: [String: Double] = [:]
    public var failedRoles: [String: String] = [:]
    public var notes: [String] = []
    public init() {}
}

public struct ConfigAttempt: Codable, Equatable, Sendable {
    public struct Download: Codable, Equatable, Sendable { public var id: String, bytes: Int64, seconds: Double, alreadyOnDisk: Bool }
    public var index: Int
    public var configuration: Configuration
    public var downloads: [Download] = []
    public var measurement: ConfigMeasurement?
    public var accepted = false
    public var reasons: [String] = []
    public var startedAt = 0.0, endedAt = 0.0
}

public struct ConfigurationRecord: Codable, Equatable, Sendable {
    public var device: DeviceProbe
    public var thresholds: ConfigThresholds
    /// The plan, best first: configuration keys with score and estimated memory.
    public var plan: [Configuration] = []
    public var attempts: [ConfigAttempt] = []
    public var chosen: Configuration?
    /// One plain line per role: what was chosen and why.
    public var lines: [String: String] = [:]
    public var notes: [String] = []
    public var startedAt = 0.0, finishedAt = 0.0
}

public enum ConfigRecordStore {
    public static var url: URL {
        KnownModels.storeRoot.deletingLastPathComponent().appendingPathComponent("voice-configuration.json")
    }
    public static func save(_ r: ConfigurationRecord, to u: URL = url) {
        try? FileManager.default.createDirectory(at: u.deletingLastPathComponent(), withIntermediateDirectories: true)
        let e = JSONEncoder(); e.outputFormatting = [.prettyPrinted, .sortedKeys]
        if let d = try? e.encode(r) { try? d.write(to: u, options: .atomic) }
    }
    public static func load(from u: URL = url) -> ConfigurationRecord? {
        (try? Data(contentsOf: u)).flatMap { try? JSONDecoder().decode(ConfigurationRecord.self, from: $0) }
    }
}

// MARK: The runner

public enum ConfigEvent: Sendable {
    case probed(DeviceProbe)
    case planned(Int)
    case attempt(index: Int, Configuration)
    /// A candidate is being fetched and loaded for a role: bytes so far of its file size.
    case preparing(role: ConfigRole, candidate: ModelCandidate, bytes: Int64)
    case measuring(Configuration)
    case rejected(index: Int, reasons: [String])
    case accepted(Configuration, [String: String])
}

public protocol ConfigBackend: Sendable {
    func probe() -> DeviceProbe
    func isOnDisk(_ c: ModelCandidate) async -> Bool
    /// Download (resuming, verifying) and load it into memory beside what is loaded. `onBytes`: bytes of its file so far.
    func prepare(_ c: ModelCandidate, role: ConfigRole, onBytes: @escaping @Sendable (Int64) -> Void) async throws
    func unload(_ c: ModelCandidate, role: ConfigRole) async
    func delete(_ c: ModelCandidate) async
    /// With everything of the configuration loaded: memory left, speeds, first sound, thermal after a short sustained run.
    func measure(_ cfg: Configuration, picks: [ConfigRole: ModelCandidate]) async throws -> ConfigMeasurement
}

public final class ConfigRunner: @unchecked Sendable {
    let backend: ConfigBackend
    let thresholds: ConfigThresholds
    let table: [ModelCandidate]
    let store: URL?
    let observer: @Sendable (ConfigEvent) -> Void
    let now: @Sendable () -> Double
    public init(backend: ConfigBackend, thresholds: ConfigThresholds = .standard, table: [ModelCandidate] = CandidateTable.all, recordURL: URL? = ConfigRecordStore.url,
                now: @escaping @Sendable () -> Double = { Date().timeIntervalSince1970 }, observer: @escaping @Sendable (ConfigEvent) -> Void = { _ in }) {
        self.backend = backend; self.thresholds = thresholds; self.table = table; self.store = recordURL; self.observer = observer; self.now = now
    }

    func picks(_ c: Configuration) -> [ConfigRole: ModelCandidate] {
        var out: [ConfigRole: ModelCandidate] = [:]
        for r in ConfigRole.allCases { if let id = c.pick(r), let cand = CandidateTable.candidate(id, r, in: table) { out[r] = cand } }
        return out
    }
    func floors() -> Configuration {
        var p: [String: String] = [:]
        for r in ConfigRole.allCases { if let f = CandidateTable.candidates(r, in: table).first(where: { $0.isFloor }) { p[r.rawValue] = f.id } }
        return Configuration(picks: p, score: 0, estimatedMB: thresholds.appBaseMB, downloadBytes: 0)
    }

    /// Plan, then try configurations best first until one is accepted. Always returns a record; the chosen configuration is never nil.
    public func run() async -> ConfigurationRecord {
        let device = backend.probe()
        var rec = ConfigurationRecord(device: device, thresholds: thresholds)
        rec.startedAt = now()
        observer(.probed(device))
        guard device.supported else { rec.notes.append(device.unsupportedReason ?? "not supported"); return finalize(rec, floors()) }
        var unavailable = Set<String>()
        var plan = ConfigPlanner.plan(device: device, thresholds: thresholds, table: table)
        rec.plan = plan
        observer(.planned(plan.count))
        if plan.isEmpty { rec.notes.append("no configuration fits the memory budget of this device; the built-in voice only"); return finalize(rec, floors()) }

        var attemptIndex = 0
        while !plan.isEmpty {
            if attemptIndex >= thresholds.maxAttempts { rec.notes.append("\(attemptIndex) configurations were tried and none passed"); break }
            let cfg = plan.removeFirst()
            if cfg.picks.values.contains(where: { unavailable.contains($0) }) { continue }
            let p = picks(cfg)
            let nonFloor: [(ConfigRole, ModelCandidate)] = ConfigRole.allCases.compactMap { r in
                guard let c = p[r], !c.isFloor else { return nil }
                return (r, c)
            }
            if nonFloor.isEmpty { return finalize(rec, cfg) }                // the floors need no download and no measurement
            attemptIndex += 1
            var attempt = ConfigAttempt(index: attemptIndex, configuration: cfg)
            attempt.startedAt = now()
            observer(.attempt(index: attemptIndex, cfg))
            var loaded: [(ModelCandidate, ConfigRole)] = []
            var failure: String?
            // Download and load: the role whose quality counts most first, then the rest.
            for (role, cand) in nonFloor.sorted(by: { $0.0.weight > $1.0.weight }) {
                let onDisk = await backend.isOnDisk(cand)
                let t0 = now()
                do {
                    try await backend.prepare(cand, role: role) { [observer] b in observer(.preparing(role: role, candidate: cand, bytes: b)) }
                    loaded.append((cand, role))
                    attempt.downloads.append(.init(id: cand.id, bytes: cand.bytes, seconds: now() - t0, alreadyOnDisk: onDisk))
                } catch {
                    failure = "\(cand.label) \(cand.quant) could not be fetched or loaded: \(error)"
                    unavailable.insert(cand.id)                           // failure 4: the plan goes on without it; the partial file stays for a re-run
                    attempt.downloads.append(.init(id: cand.id, bytes: 0, seconds: now() - t0, alreadyOnDisk: onDisk))
                    break
                }
            }
            if let failure {
                attempt.reasons = [failure]; attempt.endedAt = now(); rec.attempts.append(attempt)
                observer(.rejected(index: attemptIndex, reasons: attempt.reasons))
                for (c, r) in loaded { await backend.unload(c, role: r) }
                continue
            }
            observer(.measuring(cfg))
            var m: ConfigMeasurement
            do { m = try await backend.measure(cfg, picks: p) }
            catch { m = ConfigMeasurement(); m.failedRoles["measurement"] = "\(error)" }
            let v = ConfigPlanner.verdict(m, config: cfg, device: device, thresholds: thresholds, table: table)
            attempt.measurement = m; attempt.accepted = v.ok; attempt.reasons = v.reasons; attempt.endedAt = now()
            rec.attempts.append(attempt)
            if v.ok { return await accept(cfg, rec) }
            observer(.rejected(index: attemptIndex, reasons: v.reasons))
            // Unload the rejected configuration; delete what it fetched unless the next configuration wants it (failure 3).
            for (c, r) in loaded { await backend.unload(c, role: r) }
            let next = plan.first { !$0.picks.values.contains(where: { unavailable.contains($0) }) }
            let nextIDs = Set(next.map { Array($0.picks.values) } ?? [])
            for (c, _) in loaded where !nextIDs.contains(c.id) { await backend.delete(c) }
        }
        rec.notes.append("no configuration passed on this device; the built-in voice only")
        return await accept(floors(), rec)
    }

    /// The accepted configuration stays; every other candidate's weights on disk are deleted (failure 6).
    private func accept(_ cfg: Configuration, _ rec: ConfigurationRecord) async -> ConfigurationRecord {
        let keep = Set(cfg.picks.values)
        for c in table where !c.isFloor && !keep.contains(c.id) { if await backend.isOnDisk(c) { await backend.delete(c) } }
        return finalize(rec, cfg)
    }

    private func finalize(_ record: ConfigurationRecord, _ chosen: Configuration) -> ConfigurationRecord {
        var rec = record
        rec.chosen = chosen; rec.finishedAt = now(); rec.lines = lines(for: chosen, rec)
        if let store { ConfigRecordStore.save(rec, to: store) }
        observer(.accepted(chosen, rec.lines))
        return rec
    }

    // MARK: Plain words

    func lines(for cfg: Configuration, _ rec: ConfigurationRecord) -> [String: String] {
        let m = rec.attempts.first(where: { $0.accepted })?.measurement
        var out: [String: String] = [:]
        for r in ConfigRole.allCases {
            guard let id = cfg.pick(r), let c = CandidateTable.candidate(id, r, in: table) else { continue }
            if c.isFloor {
                let better = CandidateTable.candidates(r, in: table).contains { !$0.isFloor && $0.minTier <= rec.device.tier }
                let why = !rec.device.supported ? "this phone is below the 6 GB floor" : (better ? (rec.attempts.last?.reasons.first ?? "the larger ones did not fit beside the rest") : "nothing larger applies")
                out[r.rawValue] = c.source == "your computer" ? "\(c.label): over the network (\(why))" : "\(c.label): built in (\(why))"
                continue
            }
            let name = c.quant.isEmpty ? c.label : "\(c.label), \(c.quant)"
            var tail = ""
            switch r {
            case .responder: if let w = m?.responderWriteTPS { tail = ", \(Int((w * 0.75).rounded())) words a second" }
            case .voice: if let f = m?.ttsFirstSoundMs { tail = ", first sound in \(Int(f.rounded())) ms" }
            case .transcriber: if let x = m?.asrRealTimeFactor { tail = String(format: ", %.2f of real time", x) }
            case .decision: if let d = m?.decisionMs { tail = ", \(Int(d.rounded())) ms a decision" }
            case .vad: break
            }
            let room = (m?.availableMBAfterLoad ?? ((rec.device.availableMB ?? 0) - cfg.estimatedMB)) / 1024
            out[r.rawValue] = r == .responder ? String(format: "%@: fits with %.1f GB to spare%@", name, max(0, room), tail) : "\(name)\(tail)"
        }
        return out
    }
}
