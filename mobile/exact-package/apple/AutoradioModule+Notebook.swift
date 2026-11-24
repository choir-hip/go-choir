// The notebook page, the scorecard and the topics write are shared code (AutoradioLab/notebook.ts, notebook-run.ts, notebook-topics.ts, texture-edit.ts); no source or send of this package remains.
// What is left is one narrow call from shared code, `contentBusy`, reached through the app module (AutoradioLab/modules/apple/ModelsEntry.swift):
//   contentBusy    -> {busy}          REASON: remote audio on air is player state; the content sync (AutoradioLab/content-run.ts) does not take the network from it.
import Foundation

extension AutoradioModule {
    @MainActor public func notebookOp(_ r: [String: Any]) async -> [String: Any] {
        switch r["op"] as? String ?? "" {
        case "contentBusy":
            return ["busy": await MainActor.run { Deck.shared.current?.kind == "audio" }]
        default:
            return [:]
        }
    }
}
