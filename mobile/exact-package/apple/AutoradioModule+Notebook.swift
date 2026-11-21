// My notebook (station L6). The page, the scorecard and the learning route are shared code (AutoradioLab/notebook.ts, notebook-run.ts); no source or send of this package remains.
// What is left is registration and one narrow call from shared code, `notebookOp`, reached through the app module (AutoradioLab/modules/apple/ModelsEntry.swift):
//   notebookState                     REASON: writes the listener's topics into the notebook
//                                     (a Texture canonical write through the Swift TextureEditor: red, native until that editor moves).
//   notebookText   (text)             REASON: the DJ's request is built natively (RadioProgramme.swift) and cannot read the shared `app:/data`; the notebook text just read is kept for it.
//   contentBusy    -> {busy}          REASON: remote audio on air is player state; the content sync (AutoradioLab/content-run.ts) does not take the network from it.
// The document itself is a Texture: "Edit" opens it in the Texture reader, which edits it.
import Foundation

struct NotebookPageSource: PageSource {
    func handles(_ address: String) -> Bool { address.hasPrefix("notebook:") }
    func load(_ address: String) async throws -> PageModel { PageModel(address: address, kind: "notebook", title: "My notebook", status: "ready") }
}

extension AutoradioModule {
    func registerNotebook() {
        pageSources.register(NotebookPageSource())
    }

    @MainActor public func notebookOp(_ r: [String: Any]) async -> [String: Any] {
        switch r["op"] as? String ?? "" {
        case "notebookState":
            await NotebookStore.shared.writePendingTopics()
            return [:]
        case "notebookText":
            if let t = r["text"] as? String { NotebookCache.shared.set(t) }
            return [:]
        case "contentBusy":
            return ["busy": RunSheetPlayer.shared.isRemote]
        default:
            return [:]
        }
    }
}
