// Lane file: texture edit. Editing your own Texture documents is shared code (AutoradioLab/texture-edit.ts, texture-body.ts; the editor is reader.contract TextureEditArea). What is left here is native
// only for the reasons written at each place:
//  - `{op: "textureSaved"}` (ModelsEntry.swift calls `textureSaved` below): the Swift reader forgets its copy of the document and the Swift content store keeps the new text so search finds it at once.
//    Native only because ReaderStore and ContentStore are Swift until the reader and the content store move.
//  - the app going to the background: `willResignActive` (Platform.flushNotes), counted in `textureResigns`, and the editor's source asked again. Exact's `visibilityState` is not established on an
//    iPhone for lock and unlock (docs/reference/exact/ui-model.md), so the one reliable signal stays native; shared code reads the count with `{op: "textureResigned"}` and flushes when it moves.
//  - the unsent edit of the build before this one (LegacyTexturePending, ModelsEntry.swift `textureOldPending`): a platform file location only native code can reach.
import Foundation

extension AutoradioModule {
    func registerTextureEdit() {
        for note in Platform.flushNotes {
            NotificationCenter.default.addObserver(forName: note, object: nil, queue: .main) { [weak self] _ in
                Task { @MainActor in
                    guard let self else { return }
                    self.textureResigns += 1
                    Platform.finishInBackground { _ = try? await Task.sleep(nanoseconds: 8_000_000_000) }       // a little background time for the post shared code makes when asked again
                    self.touch(["textureEditInfo"], now: true)
                }
            }
        }
    }
    @MainActor public func textureSaved(doc: String, title: String, text: String, version: Int) {
        ReaderStore.shared.invalidate("texture:" + doc)
        TextureSave.kept(doc: doc, text: text, title: title, version: version)
        touch(["readerDoc"], now: true)
    }
}
