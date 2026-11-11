import Foundation
import AutoradioKit
import AutoradioModels

// Measures each on-device model stage on a kept turn's audio and prints one line per stage:
//   id | load ms (first, warm) | MB fetched | MB resident after load / after unload | speed | where | result
// "first" includes the download when the model is not yet on disk. usage:
//   autoradio-models --turn <folder> [--stages silero-vad,parakeet-eou,...] [--json out.json] [--no-warm]
//   autoradio-models --setup [--only id,id] [--json out.json]
//   autoradio-models --ids
// `--setup` runs exactly what voice setup runs on the phone, for every model on every ladder: registered in the catalog,
// fetched with progress, verified, loaded, checked on the bundled utterance. One line per model.
// `--ids` prints the setup ladders' ids and whether each is in the catalog, in the store, and its stage registered (no network).

var args = Array(CommandLine.arguments.dropFirst())
func take(_ f: String) -> String? {
    guard let i = args.firstIndex(of: f), i + 1 < args.count else { return nil }
    let v = args[i + 1]; args.removeSubrange(i...i + 1); return v
}

final class Locked64: @unchecked Sendable {
    private let l = NSLock(); private var v: Int64 = 0
    func set(_ x: Int64) { l.lock(); v = x; l.unlock() }
    var value: Int64 { l.lock(); defer { l.unlock() }; return v }
}

struct SetupRow: Codable {
    var id: String; var registered: Bool; var fetchedMB: Double; var fetchSeconds: Double; var verified: String
    var checkOK: Bool; var loadMs: Double; var speed: String; var memoryMB: Double; var note: String; var error: String?
    var stage = "", role = ""
    var promptTokensPerSecond: Double?, decodeTokensPerSecond: Double?, firstAudioMs: Double?, availableMemoryMB: Double?, thermal: String?
}

if args.contains("--ids") {
    let registry = StageRegistry.builtIn()
    ModelHub.register(into: registry)
    var bad = 0
    for st in SetupStage.allCases {
        for c in SetupCatalog.ladder(st) {
            let inCatalog = c.isFloor || KnownModels.contains(c.id)
            let stageOK = c.kitStageID.isEmpty || registry.stages(st.kind).contains { $0.id == c.kitStageID }
            let registered = c.isFloor ? true : await ModelStore.shared.isRegistered(c.id)
            if !(inCatalog && stageOK && registered) { bad += 1 }
            print("\(st.rawValue) | \(c.id) | catalog \(inCatalog ? "yes" : "NO") | store \(registered ? "yes" : "NO") | stage \(c.kitStageID) \(stageOK ? "registered" : "NOT REGISTERED")")
        }
    }
    exit(bad == 0 ? 0 : 3)
}

if args.contains("--configure") {
    // The configurator, for a simulated 6, 8 and 12 GB phone: the memory budget is the PLANNER's (an assumed 60% of physical memory is
    // available), not this Mac's. Everything is really downloaded, loaded together and measured here; the Mac's speeds are not a phone's.
    args.removeAll { $0 == "--configure" }
    let registry = StageRegistry.builtIn()
    ModelHub.register(into: registry)
    ModelProviders.current = HubModelProvider.shared
    let which = take("--profile") ?? "all"
    let jsonOut = take("--json")
    let tiers: [DeviceClass] = which == "all" ? [.tier6, .tier8, .tier12] : [DeviceClass(rawValue: Int(which) ?? 6) ?? .tier6]
    var records: [ConfigurationRecord] = []
    var failed = false
    func n(_ x: Double?) -> String { x.map { String(Int($0.rounded())) } ?? "-" }
    for tier in tiers {
        let device = DeviceProbe.simulated(tier)
        print("=== profile: \(tier.label), simulated (available \(n(device.availableMB)) MB is an assumption; the planner enforces it)")
        let plan = ConfigPlanner.plan(device: device)
        print("plan: \(plan.count) configurations fit the budget, best first; the first six:")
        for (i, c) in plan.prefix(6).enumerated() {
            print("  #\(i + 1) score \(Int(c.score)), about \(Int(c.estimatedMB)) MB resident, download \(c.downloadBytes / 1_000_000) MB | \(c.key)")
        }
        let backend = LiveConfigBackend(store: .shared, probe: { device })
        let url = FileManager.default.temporaryDirectory.appendingPathComponent("autoradio-config-\(tier.rawValue).json")
        let runner = ConfigRunner(backend: backend, recordURL: url, observer: { e in
            switch e {
            case .attempt(let i, let c): print("  attempt \(i): \(c.key)"); fflush(stdout)
            case .rejected(let i, let r): print("  attempt \(i) REJECTED: \(r.joined(separator: "; "))"); fflush(stdout)
            default: break
            }
        })
        let rec = await runner.run()
        for a in rec.attempts {
            let m = a.measurement
            print("  measured attempt \(a.index): \(a.accepted ? "ACCEPTED" : "rejected") | downloads \(a.downloads.map { "\($0.id) \($0.alreadyOnDisk ? "on disk" : "\($0.bytes / 1_000_000) MB in \(Int($0.seconds)) s")" }.joined(separator: ", "))")
            if let m {
                print("    responder reads \(n(m.responderReadTPS)) tok/s, writes \(n(m.responderWriteTPS)) tok/s, warm first word \(n(m.responderWarmFirstWordMs)) ms | first sound \(n(m.ttsFirstSoundMs)) ms | listener \(m.asrRealTimeFactor.map { String(format: "%.3f", $0) } ?? "-") of real time | decision \(n(m.decisionMs)) ms | resident \(n(m.residentMB)) MB | thermal \(m.thermalBefore) -> \(m.thermalAfter) | failed roles \(m.failedRoles)")
            }
        }
        print("CHOSEN for \(tier.label):")
        for r in ConfigRole.allCases { print("  \(r.rawValue): \(rec.lines[r.rawValue] ?? "-")") }
        for note in rec.notes { print("  note: \(note)") }
        fflush(stdout)
        if let ch = rec.chosen {
            if ch.picks.values.allSatisfy({ id in CandidateTable.all.first { $0.id == id }?.isFloor == true }) { failed = true }
            for r in ConfigRole.allCases { if let id = ch.pick(r), let c = CandidateTable.candidate(id, r), !c.isFloor { await backend.unload(c, role: r) } }
        }
        records.append(rec)
    }
    if let jsonOut {
        let e = JSONEncoder(); e.outputFormatting = [.prettyPrinted, .sortedKeys]
        try e.encode(records).write(to: URL(fileURLWithPath: jsonOut))
    }
    fflush(stdout); fflush(stderr)
    _exit(failed ? 3 : 0)
}

if args.contains("--setup") {
    args.removeAll { $0 == "--setup" }
    let registry = StageRegistry.builtIn()
    ModelHub.register(into: registry)
    ModelProviders.current = HubModelProvider.shared
    let only = take("--only")?.split(separator: ",").map(String.init)
    let jsonOut = take("--json")
    var rows: [SetupRow] = []
    let backend = LiveSetupBackend(store: .shared)
    for st in SetupStage.allCases {
        for c in SetupCatalog.ladder(st) where !c.isFloor && (only == nil || only!.contains(c.id)) {
            var row = SetupRow(id: c.id, registered: KnownModels.contains(c.id), fetchedMB: 0, fetchSeconds: 0,
                               verified: KnownModels.isPinned(c.id) ? "SHA-256 pinned by the kit" : "Hugging Face LFS object ids, by FluidAudio",
                               checkOK: false, loadMs: 0, speed: "", memoryMB: 0, note: "")
            let t0 = Date()
            let last = Locked64()
            do {
                try await backend.download(c) { b in
                    last.set(b)
                    FileHandle.standardError.write(Data(String(format: "  %@ %.1f MB\n", c.id, Double(b) / 1_048_576).utf8))
                }
                row.fetchedMB = Double(last.value) / 1_048_576; row.fetchSeconds = Date().timeIntervalSince(t0)
                let chk = try await backend.check(c)
                row.checkOK = chk.outputOK; row.loadMs = chk.loadMs; row.memoryMB = chk.memoryMB; row.note = chk.note
                row.speed = chk.unit.isEmpty ? "" : String(format: "%.3f %@", chk.speed, chk.unit)
                row.stage = st.rawValue; row.role = c.role
                row.promptTokensPerSecond = chk.promptTokensPerSecond; row.decodeTokensPerSecond = chk.decodeTokensPerSecond
                row.firstAudioMs = chk.firstAudioMs; row.availableMemoryMB = chk.availableMemoryMB; row.thermal = chk.thermal
            } catch { row.error = "\(error)" }
            func n(_ x: Double?) -> String { x.map { String(Int($0.rounded())) } ?? "-" }
            print("MEASURED \(st.rawValue)/\(row.id) | reads \(n(row.promptTokensPerSecond)) tok/s | writes \(n(row.decodeTokensPerSecond)) tok/s | first audio \(n(row.firstAudioMs)) ms | resident \(Int(row.memoryMB)) MB | available \(n(row.availableMemoryMB)) MB | thermal \(row.thermal ?? "-")")
            print("\(row.id) | registered \(row.registered ? "yes" : "NO") | fetched \(Int(row.fetchedMB)) MB in \(Int(row.fetchSeconds)) s (\(row.verified)) | check \(row.checkOK ? "ok" : "FAILED") | load \(Int(row.loadMs)) ms | \(row.speed) | \(Int(row.memoryMB)) MB resident | \(row.error ?? row.note)")
            fflush(stdout)
            rows.append(row)
        }
    }
    // What each responder that loaded actually says, cold and warm, and how it behaves beside a decision.
    let responders = rows.filter { $0.role == "responder" && $0.checkOK }.map(\.id)
    if !responders.isEmpty {
        print("--- sample streams from each responder (sentence by sentence; cold cache and page-read-ahead warm cache) ---")
        var seen = Set<String>()
        for id in responders where seen.insert(id).inserted {
            for l in await HubModelProvider.sampleStreams(modelID: id) { print(l) }
            fflush(stdout)
        }
        if rows.contains(where: { $0.id == KnownModels.qwen3.id && $0.checkOK }), let first = responders.first {
            print(await HubModelProvider.contention(modelID: first)); fflush(stdout)
        }
    }
    if let jsonOut {
        let e = JSONEncoder(); e.outputFormatting = [.prettyPrinted, .sortedKeys]
        try e.encode(rows).write(to: URL(fileURLWithPath: jsonOut))
    }
    fflush(stdout); fflush(stderr)
    // _exit, not exit: ggml's Metal backend aborts in its exit-time teardown after everything has succeeded.
    _exit(rows.contains { $0.error != nil || !$0.checkOK || !$0.registered } ? 3 : 0)
}

let turnPath = take("--turn"), only = take("--stages")?.split(separator: ",").map(String.init), jsonOut = take("--json")
let warm = !args.contains("--no-warm")
guard let turnPath else {
    FileHandle.standardError.write(Data("usage: autoradio-models --turn <folder> [--stages a,b] [--json out] [--no-warm] | --setup [--only id] [--json out] | --ids\n".utf8)); exit(2)
}

let rows = try await ModelProof.run(turn: KeptTurn(folder: URL(fileURLWithPath: turnPath)), stages: only, warm: warm) { print($0.line); fflush(stdout) }
if let jsonOut {
    let e = JSONEncoder(); e.outputFormatting = [.prettyPrinted, .sortedKeys]
    try e.encode(rows).write(to: URL(fileURLWithPath: jsonOut))
}
fflush(stdout); fflush(stderr)
_exit(rows.contains { $0.error != nil } ? 3 : 0)
