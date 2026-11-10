import Foundation
import AutoradioKit

func usage() -> Never {
    FileHandle.standardError.write(Data("""
    usage: autoradio-replay --turn <folder> --config <file.json> [--out record.json] [--realtime]
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

if args.contains("--list-stages") {
    let e = JSONEncoder(); e.outputFormatting = [.prettyPrinted, .sortedKeys]
    struct Row: Encodable { let kind: String; let id: String; let descriptor: StageDescriptor }
    let rows = registry.all.map { Row(kind: $0.kind.rawValue, id: $0.id, descriptor: $0.descriptor) }
    print(String(data: try e.encode(rows), encoding: .utf8)!)
    exit(0)
}

let turnPath = take("--turn"), configPath = take("--config"), out = take("--out")
let realtime = args.contains("--realtime")
guard let turnPath, let configPath else { usage() }

Task {
    do {
        let cfg = try LabConfiguration.load(from: URL(fileURLWithPath: configPath))
        let rec = try await Replayer.run(turn: KeptTurn(folder: URL(fileURLWithPath: turnPath)), configuration: cfg,
                                         registry: registry, options: ReplayOptions(realTime: realtime))
        let data = try rec.encoded()
        if let out { try data.write(to: URL(fileURLWithPath: out)) } else { print(String(data: data, encoding: .utf8)!) }
        let s = rec.summary
        FileHandle.standardError.write(Data("transcript: \(s.transcript ?? "(none)")\nack ms: \(s.turnEndToAckMs.map { String(Int($0)) } ?? "-")  revision ms: \(s.turnEndToRevisionMs.map { String(Int($0)) } ?? "-")  errors: \(s.errors.count)\n".utf8))
        exit(s.errors.isEmpty ? 0 : 3)
    } catch {
        FileHandle.standardError.write(Data("error: \(error)\n".utf8))
        exit(1)
    }
}
dispatchMain()
