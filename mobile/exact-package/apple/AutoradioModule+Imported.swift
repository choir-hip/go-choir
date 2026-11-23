// Imported documents (PDFs and e-books the person imports to read and hear): the pages, the list, the import, the text and the search are SHARED code
// (AutoradioLab/imported.ts, over the Files client; a document is a file on the person's computer). Only the audio is native, and it is two names
// (interface/imported.json): one name, `importedFollow`, the block the player is reading now. Reading aloud and queueing are shared code's now
// (radio-bridge.ts `radioHandWords` over the run sheet's own entry builders); nothing is kept on the phone by this file, and nothing is read from an older file.
import Foundation

/// The block the radio is reading now (set by the player's `onDocumentPosition`; the page lights it and follows it).
final class ImportedFollow: @unchecked Sendable {
    static let shared = ImportedFollow()
    private let lock = NSLock()
    private var id = "", block = -1
    var current: (id: String, block: Int)? { lock.lock(); defer { lock.unlock() }; return id.isEmpty ? nil : (id, block) }
    /// True when it changed.
    func set(id: String, block: Int) -> Bool {
        lock.lock(); defer { lock.unlock() }
        let same = self.id == id && self.block == block; self.id = id; self.block = block; return !same
    }
}

extension AutoradioModule {
    func registerImported() {
        // (The block being read is not reported while an imported document is read aloud: the player that reported it is shared code's now, a residual of the move.)
        handle("importedFollow") { _, reply in
            let c = ImportedFollow.shared.current
            reply(["address": c.map { "doc:" + $0.id } ?? "", "block": c?.block ?? -1])
        }
    }
}
