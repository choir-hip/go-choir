// Lane file: texture edit. Editing your own Texture documents is shared code (AutoradioLab/texture-edit.ts, texture-body.ts; the editor is reader.contract TextureEditArea). What is left here is the
// ONE native call it makes after a save, `{op: "textureSaved"}` on the app module (modules/apple/ModelsEntry.swift calls this): the Swift reader forgets its copy of the document and the Swift
// content store keeps the new text so search finds it at once. Native only because ReaderStore and ContentStore are Swift until the reader and the content store move.
import Foundation

extension AutoradioModule {
    @MainActor public func textureSaved(doc: String, title: String, text: String, version: Int) {
        ReaderStore.shared.invalidate("texture:" + doc)
        TextureSave.kept(doc: doc, text: text, title: title, version: version)
        touch(["readerDoc"], now: true)
    }
}
