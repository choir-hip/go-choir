import Foundation
import AutoradioKit
import AutoradioModels

// Measures each on-device model stage on a kept turn's audio and prints one line per stage:
//   id | load ms (first, warm) | MB fetched | MB resident after load / after unload | speed | where | result
// "first" includes the download when the model is not yet on disk. usage:
//   autoradio-models --turn <folder> [--stages silero-vad,parakeet-eou,...] [--json out.json] [--no-warm]

var args = Array(CommandLine.arguments.dropFirst())
func take(_ f: String) -> String? {
    guard let i = args.firstIndex(of: f), i + 1 < args.count else { return nil }
    let v = args[i + 1]; args.removeSubrange(i...i + 1); return v
}
let turnPath = take("--turn"), only = take("--stages")?.split(separator: ",").map(String.init), jsonOut = take("--json")
let warm = !args.contains("--no-warm")
guard let turnPath else { FileHandle.standardError.write(Data("usage: autoradio-models --turn <folder> [--stages a,b] [--json out] [--no-warm]\n".utf8)); exit(2) }

let rows = try await ModelProof.run(turn: KeptTurn(folder: URL(fileURLWithPath: turnPath)), stages: only, warm: warm) { print($0.line); fflush(stdout) }
if let jsonOut {
    let e = JSONEncoder(); e.outputFormatting = [.prettyPrinted, .sortedKeys]
    try e.encode(rows).write(to: URL(fileURLWithPath: jsonOut))
}
exit(rows.contains { $0.error != nil } ? 3 : 0)
