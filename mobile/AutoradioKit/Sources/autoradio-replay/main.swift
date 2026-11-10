import Foundation
import AutoradioKit
import AutoradioModels

func usage() -> Never {
    FileHandle.standardError.write(Data("""
    usage: autoradio-replay --turn <folder> --config <file.json> [--set kind=id[,param=v]]... [--out record.json] [--realtime] [--api-key-file <path>] [--base-url <url>]
           autoradio-replay --list-stages

    """.utf8))
    exit(2)
}

var args = Array(CommandLine.arguments.dropFirst())
func take(_ flag: String) -> String? {
    guard let i = args.firstIndex(of: flag), i + 1 < args.count else { return nil }
    let v = args[i + 1]; args.removeSubrange(i...i + 1); return v
}

let registry = StageRegistry.builtIn()
ModelHub.register(into: registry)   // the on-device models; each downloads on first use

/// `--set kind=id` or `--set kind=id,param=value,...` replaces one stage of the configuration (repeatable).
var overrides: [(StageKind, StageSetting)] = []
while let o = take("--set") {
    let parts = o.split(separator: ",").map(String.init)
    guard let head = parts.first, let eq = head.firstIndex(of: "="), let kind = StageKind(rawValue: String(head[..<eq])) else { usage() }
    var params: [String: JSONValue] = [:]
    for p in parts.dropFirst() {
        guard let e = p.firstIndex(of: "=") else { usage() }
        let k = String(p[..<e]), v = String(p[p.index(after: e)...])
        params[k] = Double(v).map { .number($0) } ?? (v == "true" ? .bool(true) : v == "false" ? .bool(false) : .string(v))
    }
    overrides.append((kind, StageSetting(id: String(head[head.index(after: eq)...]), params: params)))
}

if args.contains("--list-stages") {
    let e = JSONEncoder(); e.outputFormatting = [.prettyPrinted, .sortedKeys]
    struct Row: Encodable { let kind: String; let id: String; let descriptor: StageDescriptor }
    let rows = registry.all.map { Row(kind: $0.kind.rawValue, id: $0.id, descriptor: $0.descriptor) }
    print(String(data: try e.encode(rows), encoding: .utf8)!)
    exit(0)
}

// The key comes from a file only. It is never a flag value and is never printed.
if args.contains(where: { $0 == "--api-key" || $0.hasPrefix("--api-key=") }) {
    FileHandle.standardError.write(Data("error: the key is never taken as a flag value; use --api-key-file <path>\n".utf8)); exit(2)
}
if let keyFile = take("--api-key-file") {
    ChoirEnvironment.shared.keys = OverlayKeyStore(base: FileKeyStore(url: URL(fileURLWithPath: keyFile), readOnlyKeyFile: true))
    if ChoirEnvironment.shared.keys.get(KeyAccount.apiKey) == nil {
        FileHandle.standardError.write(Data("error: the key file is empty or unreadable\n".utf8)); exit(2)
    }
}
let baseURLOverride = take("--base-url")
let turnPath = take("--turn"), configPath = take("--config"), out = take("--out")
let realtime = args.contains("--realtime")
guard let turnPath, let configPath else { usage() }

Task {
    do {
        // Learn the listener's computer before the turn so the turn path makes no discovery call.
        await ChoirEnvironment.shared.warm(baseURL: baseURLOverride)
        var cfg = try LabConfiguration.load(from: URL(fileURLWithPath: configPath))
        if let b = baseURLOverride {
            for k in [StageKind.system1, .desk, .programme] where cfg.setting(k)?.id.hasPrefix("choir-") == true {
                var s = cfg.setting(k)!; s.params["baseURL"] = .string(b); cfg.set(k, s)
            }
        }
        for (k, st) in overrides { cfg.set(k, st) }
        let rec = try await Replayer.run(turn: KeptTurn(folder: URL(fileURLWithPath: turnPath)), configuration: cfg,
                                         registry: registry, options: ReplayOptions(realTime: realtime))
        let data = try rec.encoded()
        if let out { try data.write(to: URL(fileURLWithPath: out)) } else { print(String(data: data, encoding: .utf8)!) }
        let s = rec.summary
        let notes = rec.events.compactMap { te -> String? in if case .note(let n) = te.event, n.hasPrefix("choir ") { return n }; return nil }
        for n in notes { FileHandle.standardError.write(Data((n + "\n").utf8)) }
        FileHandle.standardError.write(Data("transcript: \(s.transcript ?? "(none)")\nack ms: \(s.turnEndToAckMs.map { String(Int($0)) } ?? "-")  revision ms: \(s.turnEndToRevisionMs.map { String(Int($0)) } ?? "-")  errors: \(s.errors.count)\n".utf8))
        exit(s.errors.isEmpty ? 0 : 3)
    } catch {
        FileHandle.standardError.write(Data("error: \(error)\n".utf8))
        exit(1)
    }
}
dispatchMain()
