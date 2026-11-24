import Foundation
import AutoradioKit

// The developer launch hook of ModelProof, in a file of its own: the store kind of the staged Swift (scripts/sync-exact-package.sh) does not stage this file, so a store
// build has neither the hook nor its launch-argument name (scripts/store-dev-names.txt lists the names check-store-ipa.py refuses in a store binary).
extension ModelProof {
    /// On the phone: a launch argument `-ModelProofStages silero-vad,parakeet-eou` runs the proof on the turn in
    /// Documents/proof/turn (mic.caf, context.json, copied in with devicectl) and writes Documents/proof/results.json.
    static func runFromLaunchArguments() {
        guard let stages = UserDefaults.standard.string(forKey: "ModelProofStages") else { return }
        let docs = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0].appendingPathComponent("proof", isDirectory: true)
        Task.detached {
            func log(_ s: String) { fputs("autoradio proof: \(s)\n", stderr); fflush(stderr) }
            do {
                log("start on \(machine)")
                let list = stages == "all" ? nil : stages.split(separator: ",").map(String.init)
                let rows = try await run(turn: KeptTurn(folder: docs.appendingPathComponent("turn")), stages: list, warm: true) { log($0.line) }
                let e = JSONEncoder(); e.outputFormatting = [.prettyPrinted, .sortedKeys]
                try e.encode(rows).write(to: docs.appendingPathComponent("results.json"))
                log("done, \(rows.count) stages")
            } catch { log("failed: \(error)") }
        }
    }
}
