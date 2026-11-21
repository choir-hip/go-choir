// Lane file: texture edit. Your own Texture documents are edited in rich text with Exact's Markdown editor (AutoradioLab/reader.contract
// TextureEditArea, content.contract TextureExpanded); the logic is Kit/TextureEditor.swift and Kit/TextureBody.swift. Declared in
// interface/textedit.json:
//   textureEditInfo(address)            what the editor says about the document in front: idle | dirty | saving | saved | offline | failed
//   textureEdit(address, markdown)      the listener typed: the text is now this (posted after a pause in typing)
//   textureFlush(address)               post what is waiting now (Done, leaving the page)
import Foundation

extension AutoradioModule {
    func registerTextureEdit() {
        Task { @MainActor [weak self] in
            TextureEditor.shared.onChange = { DispatchQueue.main.async { self?.touch(["textureEditInfo"], now: true) } }
            await TextureEditor.shared.retryPending()                   // what an earlier launch or a bad network kept
        }
        handle("textureEditInfo") { [unowned self] args, reply in
            let a = args["address"] as? String ?? ""
            reply(self.json(onMainThread { TextureEditor.shared.info(address: a) }))
        }
        handle("textureEdit") { [weak self] args, reply in
            let (a, text) = (args["address"] as? String ?? "", args["markdown"] as? String ?? "")
            Task { @MainActor in
                await TextureEditor.shared.edit(address: a, markdown: text)
                reply(self?.json(TextureEditor.shared.info(address: a)) ?? [:])
            }
        }
        handle("textureFlush") { [weak self] args, reply in
            let a = args["address"] as? String ?? ""
            Task { @MainActor in
                await TextureEditor.shared.flush(address: a)
                reply(self?.json(TextureEditor.shared.info(address: a)) ?? [:])
            }
        }
        // The app goes to the background: what is waiting is posted now, not after the pause.
        for note in Platform.flushNotes {
            NotificationCenter.default.addObserver(forName: note, object: nil, queue: .main) { _ in
                Task { @MainActor in await TextureEditor.shared.flushAll() }
            }
        }
    }
}
