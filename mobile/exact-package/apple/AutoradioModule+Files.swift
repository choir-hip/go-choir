// Lane file: files. Owned by the files lane (mobile/README.md, "File ownership"). Sources and sends (declared in
// exact-package/interface/files.json), all answered by the kit's FilesService (Kit/ChoirFiles.swift, Kit/Epub.swift):
//   fileList(path, mode, sort, desc, query) -> FileListing        a folder, recents, or a search
//   fileOpen(address) -> FileInfo                                  a file brought to app:/cache/files, with what its viewer reads
//   fileChapter(address, index) -> FileChapter                     one chapter of an e-book as Markdown
//   filePages(address, query) -> FilePages                         the pages of a PDF that hold a word
//   fileMkdir / fileDelete / fileRename / fileUpload -> FileListing
// Demo mode serves an in-memory tree of invented files; otherwise the listener's computer (GET/PUT/POST/PATCH/DELETE /api/files).
import Foundation

extension AutoradioModule {
    func registerFiles() {
        let support = FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0].appendingPathComponent("Autoradio")
        // Exact names its cache as app:/cache/...; context.file maps that to the disk path. Without it (no file roots) use Caches.
        let disk = context.file("app:/cache/files")
            ?? FileManager.default.urls(for: .cachesDirectory, in: .userDomainMask)[0].appendingPathComponent("Autoradio/files")
        try? FileManager.default.createDirectory(at: disk, withIntermediateDirectories: true)
        let backend: FilesBackend = Demo.isOn ? DemoFilesBackend() : ChoirFilesBackend()
        let service = FilesService(backend: backend, cacheRoot: disk, appPrefix: "app:/cache/files",
                                   recents: RecentFiles(file: support.appendingPathComponent("recent-files.json")))
        pageSources.register(FilesPageSource(service: service))

        func str(_ args: [String: Any], _ k: String) -> String { args[k] as? String ?? "" }
        func num(_ args: [String: Any], _ k: String) -> Int { (args[k] as? NSNumber)?.intValue ?? Int(args[k] as? String ?? "") ?? 0 }
        /// Answer from a Task on the main queue, the way the spine's page model does.
        func answer<T: Encodable>(_ reply: @escaping ([String: Any]) -> Void, _ work: @escaping @Sendable () async -> T) {
            Task { [weak self] in
                let v = await work()
                DispatchQueue.main.async { reply(self?.json(v) ?? [:]) }
            }
        }

        handle("fileList") { args, reply in
            let (p, m, s, d, q) = (str(args, "path"), str(args, "mode"), str(args, "sort"), str(args, "desc") == "true", str(args, "query"))
            answer(reply) { await service.listing(path: p, mode: m, sort: s, desc: d, query: q) }
        }
        handle("fileOpen") { [unowned self] args, reply in
            let a = str(args, "address")
            guard a.hasPrefix("file:"), !a.hasSuffix("/") else { return reply(self.json(FileInfo.empty)) }
            answer(reply) { await service.open(address: a) }
        }
        handle("fileChapter") { args, reply in
            let (a, i) = (str(args, "address"), num(args, "index"))
            answer(reply) { await service.chapter(address: a, index: i) }
        }
        handle("filePages") { args, reply in
            let (a, q) = (str(args, "address"), str(args, "query"))
            answer(reply) { await service.findPages(address: a, query: q) }
        }

        // Each change answers the refreshed listing of the folder it changed, and refreshes the live sources.
        func change(_ reply: @escaping ([String: Any]) -> Void, _ work: @escaping @Sendable () async -> FileListing) {
            Task { [weak self] in
                let l = await work()
                DispatchQueue.main.async { self?.touch(["fileList", "fileOpen"]); reply(self?.json(l) ?? [:]) }
            }
        }
        handle("fileMkdir") { args, reply in
            let (d, n) = (str(args, "dir"), str(args, "name"))
            change(reply) { await service.makeFolder(in: d, name: n) }
        }
        handle("fileDelete") { args, reply in
            let p = str(args, "path")
            change(reply) { await service.delete(path: p) }
        }
        handle("fileRename") { args, reply in
            let (p, n) = (str(args, "path"), str(args, "name"))
            change(reply) { await service.rename(path: p, to: n) }
        }
        handle("fileUpload") { [unowned self] args, reply in
            // `from` is the picker's copy under app:/tmp/picked (Picked.path); in demo mode it may be empty, and a small note is added.
            let (from, n, d) = (str(args, "from"), str(args, "name"), str(args, "dir"))
            let url = from.isEmpty ? nil : self.context.file(from)
            change(reply) {
                if let url { return await service.upload(from: url, name: n, into: d) }
                // Demo mode and the agent have no picker: add a small invented note so the flow can be seen.
                let tmp = FileManager.default.temporaryDirectory.appendingPathComponent("added-\(UUID().uuidString).txt")
                try? Data("Added from the phone. Invented text.\n".utf8).write(to: tmp)
                defer { try? FileManager.default.removeItem(at: tmp) }
                return await service.upload(from: tmp, name: n.isEmpty ? "Added note.txt" : n, into: d)
            }
        }
    }
}

/// `file:` pages for the cards and anything that wants a title and a line of facts without opening the file.
struct FilesPageSource: PageSource {
    let service: FilesService
    func handles(_ address: String) -> Bool { address.hasPrefix("file:") }
    func load(_ address: String) async throws -> PageModel {
        let i = await service.facts(address: address)
        let facts = [i.kindLabel, i.sizeLabel, i.modifiedLabel].filter { !$0.isEmpty }.joined(separator: " · ")
        return PageModel(address: address, kind: "file", title: i.name.isEmpty ? PageAddress.defaultTitle(address) : i.name,
                         status: i.status == "notfound" ? "error" : "ready", lines: facts.isEmpty ? [] : [facts],
                         message: i.status == "notfound" ? "That file is not there any more." : "")
    }
}
