// Imported documents (PDFs and e-books the person imports to read and hear): the pages, the list, the import, the text and the search are SHARED code
// (AutoradioLab/imported.ts, over the Files client; a document is a file on the person's computer). Only the audio is native, and it is two names
// (interface/imported.json), each with the one reason only native code can meet:
//   importedHand    the shared page hands the player the words to read aloud (one line per block from block `at`) as data: the run sheet's player and the audio session are native.
//   importedFollow  the block the player is reading now: only the player knows; the page lights it and follows it.
// The words are held for the entry (RunSheetPlayer `provided`); nothing is kept on the phone by this file, and nothing is read from an older file.
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
        RunSheetPlayer.shared.onDocumentPosition = { [weak self] id, block in
            if ImportedFollow.shared.set(id: id, block: block) { DispatchQueue.main.async { self?.touch(["importedFollow"]) } }
        }
        handle("importedFollow") { _, reply in
            let c = ImportedFollow.shared.current
            reply(["address": c.map { "doc:" + $0.id } ?? "", "block": c?.block ?? -1])
        }
        handle("importedHand") { [unowned self] args, reply in
            let op = args["op"] as? String ?? "", id = args["id"] as? String ?? "", title = args["title"] as? String ?? ""
            let at = (args["at"] as? NSNumber)?.intValue ?? 0
            let words = (args["text"] as? String ?? "").split(separator: "\n", omittingEmptySubsequences: true).map(String.init)
            guard !id.isEmpty, !words.isEmpty else { return reply(["ok": false, "message": "This document has nothing to read."]) }
            Task { @MainActor [weak self] in
                guard let self else { return }
                let entry = RunSheetEntry.book(id: id, title: title, at: at)
                if op == "queue" { RunSheetPlayer.shared.queue(entry, words: (title, words)); return reply(["ok": true, "message": "Added to the queue."]) }
                _ = ImportedFollow.shared.set(id: id, block: at)
                // As the queue's own add does: the engine is started first when it is not running.
                if !self.runtime.snapshot().running { await self.runtime.start(virtual: self.virtualEngine) }
                RunSheetPlayer.shared.playNow(entry, words: (title, words))
                self.touch(["importedFollow"])
                reply(["ok": true, "message": ""])
            }
        }
    }
}
