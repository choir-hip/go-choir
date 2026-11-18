// My notebook (station L6; docs/design/autoradio-learned-document-2026-11-17.md section 7). One source, `notebookPage`, and one send, `notebookAct`,
// declared in interface/notebook.json. The model is the Kit's (Notebook.swift); this file registers the page and carries the gestures.
//   notebookAct op  refresh | select (arg: a line id; the same again clears it)
// The document itself is a Texture: "Edit" opens it in the Texture reader, which edits it (the page only reads it).
import Foundation

struct NotebookPageSource: PageSource {
    func handles(_ address: String) -> Bool { address.hasPrefix("notebook:") }
    func load(_ address: String) async throws -> PageModel { PageModel(address: address, kind: "notebook", title: "My notebook", status: "ready") }
}

extension AutoradioModule {
    func registerNotebook() {
        pageSources.register(NotebookPageSource())
        SteerQueue.shared.setSender { tap in await SteerClient(client: ChoirEnvironment.shared.client()).send(tap) }
        SteerQueue.shared.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["notebookPage"], now: true) } }
        Task { @MainActor [weak self] in
            NotebookStore.shared.onChange = { self?.touch(["notebookPage"], now: true) }
            await SteerQueue.shared.flush()                    // the taps an earlier launch kept
        }
        handle("notebookPage") { [unowned self] _, reply in reply(json(onMainThread { NotebookStore.shared.refreshIfStale(); return NotebookStore.shared.info() })) }
        handle("notebookAct") { [unowned self] args, reply in
            let op = args["op"] as? String ?? "", arg = args["arg"] as? String ?? ""
            Task { @MainActor [weak self] in
                guard let self else { return }
                switch op {
                case "refresh": await NotebookStore.shared.refresh()
                case "select": NotebookStore.shared.select(arg)
                default: break
                }
                reply(self.json(NotebookStore.shared.info()))
            }
        }
    }
}
