// Lane file: reader (documents). The Reader: EPUBs and PDFs imported, read on screen, read aloud and searched. Sources and sends are
// declared in exact-package/interface/books.json; the logic is the kit's Reader (Kit/ReaderShelf.swift, Kit/Epub.swift):
//   shelfList(at)                  the listener's documents
//   shelfDoc(address)              one document: facts, chapters or pages, where the listener is, a path the page view can show (PDF)
//   shelfChapter(address, index)   one chapter or page as blocks
//   shelfFind(address, query)      blocks of the document holding the words
//   shelfFollow                    the block the radio is reading now
// Sends: shelfImport, shelfRemove, shelfPlace, shelfReadAloud, shelfEnqueue.
import Foundation

extension AutoradioModule {
    /// A file name safe for an `app:/` path used in a src: ASCII letters, digits and ".-_" only (the Files page's cacheName, files-model.ts, which this
    /// copy serves until the Reader's shelf moves).
    static func cacheName(_ name: String) -> String {
        let ext = (name as NSString).pathExtension.lowercased()
        var stem = (name as NSString).deletingPathExtension
        stem = String(stem.unicodeScalars.map { s in (s.isASCII && (CharacterSet.alphanumerics.contains(s) || s == "-" || s == "_")) ? Character(s) : "-" })
        if stem.isEmpty || stem.allSatisfy({ $0 == "-" }) { stem = "file" }
        let safeExt = String(ext.unicodeScalars.filter { $0.isASCII && CharacterSet.alphanumerics.contains($0) }.map(Character.init).prefix(8))
        return String(stem.prefix(60)) + (safeExt.isEmpty ? "" : "." + safeExt)
    }
    func registerBooks() {
        let support = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].appendingPathComponent("Autoradio")
        let lib = ReaderShelf(root: support.appendingPathComponent("books"))
        ReaderShelf.shared = lib
        lib.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["shelfList", "shelfDoc"]) } }
        ShelfFollow.shared.onChange = { [weak self] in DispatchQueue.main.async { self?.touch(["shelfFollow", "shelfList"]) } }
        Task { @MainActor in ShelfPlayback.register() }
        pageSources.register(BooksPageSource(lib: lib))
        // lane autoradio-joins: a .pdf or .epub typed in the bar is imported, then its page opens; a failure is said in the bar's line.
        BarURL.importDocument = { [weak self] url in
            Task { [weak self] in
                await BarDocumentImport.run(url: url, importer: { try await ReaderShelf.importFrom(url: $0).id },
                    open: { address in _ = self?.pages.open(address, title: lib.doc(ShelfAddress.parse(address)?.id ?? "")?.title ?? "Document") },
                    say: { line in BarTracker.shared.say(line); self?.touch(["barFacts"], now: true) })
            }
        }

        // The PDF page view is the Files viewer's: an iframe on a path under app:/cache/files. The reader's copy stays in Application Support
        // (the cache may be emptied); a clone is made in the cache the first time the PDF is shown.
        let cacheDisk = filesCacheDisk
        func shown(_ d: ShelfDoc) -> String {
            guard d.kind == "pdf", let from = lib.fileURL(d.id) else { return "" }
            let folder = cacheDisk.appendingPathComponent("reader-\(d.id)", isDirectory: true)
            let to = folder.appendingPathComponent("doc.pdf")
            if !FileManager.default.fileExists(atPath: to.path) {
                try? FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
                try? FileManager.default.copyItem(at: from, to: to)
            }
            // A copy that did not happen is not a source: the page would be a blank frame. "" makes the page read the extracted text instead.
            return FileManager.default.fileExists(atPath: to.path) ? "app:/cache/files/reader-\(d.id)/doc.pdf" : ""
        }
        /// The document's own file for the share sheet (any kind): a copy of the original under the cache, made once.
        func shareable(_ d: ShelfDoc) -> String {
            guard let from = lib.fileURL(d.id) else { return "" }
            let folder = cacheDisk.appendingPathComponent("reader-\(d.id)", isDirectory: true)
            let name = Self.cacheName(d.fileName)
            let to = folder.appendingPathComponent(name)
            if !FileManager.default.fileExists(atPath: to.path) {
                try? FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
                try? FileManager.default.copyItem(at: from, to: to)
            }
            return "app:/cache/files/reader-\(d.id)/\(name)"
        }
        func num(_ args: [String: Any], _ k: String) -> Int { (args[k] as? NSNumber)?.intValue ?? Int(args[k] as? String ?? "") ?? 0 }
        func listing(_ message: String = "") -> [String: Any] {
            let docs = lib.list()
            let rows: [[String: Any]] = docs.map { d in
                let pct = d.progress > 0 ? "\(Int((d.progress * 100).rounded()))% read" : "Not started"
                return ["id": d.id, "address": d.address, "title": d.title, "author": d.author, "kindLabel": d.kind == "pdf" ? "PDF" : "Book",
                        "detail": [d.author, d.kind == "pdf" ? "PDF" : "Book", d.origin == nil ? "" : "From Files", pct].filter { !$0.isEmpty }.joined(separator: " · "), "progress": d.progress]
            }
            return ["status": rows.isEmpty ? "empty" : "ready",
                    "message": message.isEmpty && rows.isEmpty ? "Your books and documents appear here. Add an EPUB or PDF to read it here, hear it read aloud and search it." : message,
                    "count": rows.count, "rows": rows]
        }

        handle("shelfList") { _, reply in reply(listing()) }

        handle("shelfDoc") { args, reply in
            let a = args["address"] as? String ?? ""
            // Every answer carries every field of the shape: a missing one fails the request and it keeps its last value.
            let blank: [String: Any] = ["address": a, "id": "", "title": "", "author": "", "kind": "", "kindLabel": "", "status": "loading", "message": "", "src": "",
                                        "shareSrc": "", "sections": [], "blockCount": 0, "position": 0, "resumeSection": 0, "resumeBlock": -1]
            guard let (id, block) = ShelfAddress.parse(a) else { return reply(blank) }
            if let b = ShelfJump.shared.take(id) ?? block { lib.setPosition(id: id, block: b) }
            guard let d = lib.doc(id), let t = lib.text(id) else {
                return reply(blank.merging(["id": id, "status": "notfound", "message": "This document is not in Reader any more."]) { $1 })
            }
            let sections: [[String: Any]] = t.sections.enumerated().map { ["index": $0.offset, "title": $0.element.title] }
            let src = shown(d)
            let note = d.kind == "pdf" && src.isEmpty ? "The original PDF could not be shown, so its text is here." : ""
            reply(["address": a, "id": id, "title": d.title, "author": d.author, "kind": d.kind, "kindLabel": d.kind == "pdf" ? "PDF" : "Book",
                   "status": "ready", "message": note, "src": src, "shareSrc": shareable(d), "sections": sections, "blockCount": d.length, "position": d.position,
                   "resumeSection": t.section(of: d.position), "resumeBlock": d.position])
        }

        handle("shelfChapter") { args, reply in
            let a = args["address"] as? String ?? ""
            guard let (id, _) = ShelfAddress.parse(a), let t = lib.text(id), !t.sections.isEmpty else {
                return reply(["index": 0, "count": 0, "title": "", "start": 0, "page": 1, "status": "loading", "blocks": []])
            }
            let i = min(max(num(args, "index"), 0), t.sections.count - 1), s = t.sections[i]
            let blocks: [[String: Any]] = (s.start..<(s.start + s.count)).map { n in
                let b = t.blocks[n]; return ["n": n, "kind": b.kind, "level": b.level, "text": b.text, "marker": b.marker]
            }
            // A PDF section is titled "Page N" with N the real page (pages without text are not sections).
            let page = Int(s.title.replacingOccurrences(of: "Page ", with: "")) ?? (i + 1)
            reply(["index": i, "count": t.sections.count, "title": s.title, "start": s.start, "page": page, "status": "ready", "blocks": blocks])
        }

        handle("shelfFind") { args, reply in
            let a = args["address"] as? String ?? "", q = (args["query"] as? String ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
            guard !q.isEmpty, let (id, _) = ShelfAddress.parse(a), let t = lib.text(id) else { return reply(["query": "", "hits": [], "message": ""]) }
            let found = lib.find(in: id, query: q)
            let hits: [[String: Any]] = found.map { ["block": $0.block, "section": $0.section, "label": t.sections.indices.contains($0.section) ? t.sections[$0.section].title : "", "snippet": $0.snippet] }
            reply(["query": q, "hits": hits, "message": hits.isEmpty ? "Nothing in this document matches \u{201C}\(q)\u{201D}." : ""])
        }

        handle("shelfFollow") { _, reply in
            let c = ShelfFollow.shared.current
            let sec = c.flatMap { lib.text($0.id)?.section(of: $0.block) } ?? 0
            reply(["address": c.map { ShelfAddress.address($0.id) } ?? "", "block": c?.block ?? -1, "section": sec])
        }

        handle("shelfImport") { [unowned self] args, reply in
            let from = args["from"] as? String ?? "", name = args["name"] as? String ?? ""
            guard !from.isEmpty, let url = self.context.file(from) else { return reply(listing()) }
            Task { [weak self] in
                var note = ""
                do { _ = try await Task.detached { try lib.importFile(at: url, name: name.isEmpty ? nil : name) }.value }
                catch { note = (error as? ShelfError)?.description ?? "\(error)" }
                DispatchQueue.main.async { self?.touch(["shelfList"]); reply(listing(note)) }
            }
        }
        handle("shelfRemove") { [unowned self] args, reply in
            let id = args["id"] as? String ?? ""
            lib.remove(id: id)
            try? FileManager.default.removeItem(at: cacheDisk.appendingPathComponent("reader-\(id)"))
            self.touch(["shelfList"]); reply(listing())
        }
        handle("shelfPlace") { args, reply in
            // block < 0: the start of the section (a chapter or page just opened).
            if let (id, _) = ShelfAddress.parse(args["address"] as? String ?? "") {
                let b = num(args, "block")
                if b >= 0 { lib.setPosition(id: id, block: b) }
                else if let t = lib.text(id), !t.sections.isEmpty { lib.setPosition(id: id, block: t.sections[min(max(num(args, "section"), 0), t.sections.count - 1)].start) }
            }
            reply(["ok": true, "message": ""])
        }
        // Where reading aloud starts: the chosen block when it lies in the section on screen, else the section's first block.
        func start(_ id: String, _ args: [String: Any]) -> Int? {
            guard let t = lib.text(id), !t.sections.isEmpty else { return nil }
            let s = t.sections[min(max(num(args, "section"), 0), t.sections.count - 1)]
            let b = num(args, "block")
            return (args["block"] != nil && b >= s.start && b < s.start + s.count) ? b : s.start
        }
        handle("shelfReadAloud") { [unowned self] args, reply in
            let a = args["address"] as? String ?? ""
            guard let (id, _) = ShelfAddress.parse(a), let d = lib.doc(id), let at = start(id, args) else { return reply(["ok": false, "message": "This document has nothing to read."]) }
            ShelfPlayback.play(module: self, doc: d, at: at) { ok in DispatchQueue.main.async { reply(["ok": ok, "message": ok ? "" : "Could not start reading."]) } }
        }
        handle("shelfEnqueue") { [unowned self] args, reply in
            let a = args["address"] as? String ?? ""
            guard let (id, _) = ShelfAddress.parse(a), let d = lib.doc(id), let at = start(id, args) else { return reply(["ok": false, "message": "This document has nothing to read."]) }
            ShelfPlayback.enqueue(module: self, doc: d, at: at) { ok, note in DispatchQueue.main.async { reply(["ok": ok, "message": note]) } }
        }
    }
}

/// The ONE place Reader meets the run sheet (Kit/RunSheetPlayer.swift): Read aloud is `place now` of a book entry, Add to queue is `place queued`. The player asks
/// the text source below for the blocks from a position and reports the block being read through `onDocumentPosition`, which lights it on the page
/// and moves the saved place. Nothing here speaks by itself.
enum ShelfPlayback {
    @MainActor static func register() {
        RunSheetPlayer.shared.registerTextSource(kind: "document") { id, at in
            let blocks = ReaderShelf.shared.blocks(id: id, from: at)
            return blocks.isEmpty ? nil : blocks
        }
        RunSheetPlayer.shared.onDocumentPosition = { id, block in ShelfFollow.shared.set(id: id, block: block) }
    }
    static func play(module: AutoradioModule, doc: ShelfDoc, at: Int, done: @escaping (Bool) -> Void) {
        Task { @MainActor [weak module] in
            guard let module else { return done(false) }
            ShelfFollow.shared.set(id: doc.id, block: at)
            // As the queue's own add does: the engine is started first when it is not running.
            if !module.runtime.snapshot().running { await module.runtime.start(virtual: module.virtualEngine) }
            RunSheetPlayer.shared.playNow(.book(id: doc.id, title: doc.title, at: at))
            done(true)
        }
    }
    static func enqueue(module: AutoradioModule, doc: ShelfDoc, at: Int, done: @escaping (Bool, String) -> Void) {
        Task { @MainActor in
            RunSheetPlayer.shared.queue(.book(id: doc.id, title: doc.title, at: at))
            done(true, "Added to the queue.")
        }
    }
}

struct BooksPageSource: PageSource {
    let lib: ReaderShelf
    func handles(_ address: String) -> Bool { address.hasPrefix("doc:") || address == "books:" }
    func load(_ address: String) async throws -> PageModel {
        if address == "books:" { return PageModel(address: address, kind: "books", title: "Reader", status: "ready", lines: ["\(lib.list().count) documents"]) }
        guard let (id, _) = ShelfAddress.parse(address), let d = lib.doc(id) else {
            return PageModel(address: address, kind: "doc", title: "Document", status: "error", message: "This document is not in Reader any more.")
        }
        return PageModel(address: address, kind: "doc", title: d.title, status: "ready", lines: [d.author].filter { !$0.isEmpty })
    }
}
