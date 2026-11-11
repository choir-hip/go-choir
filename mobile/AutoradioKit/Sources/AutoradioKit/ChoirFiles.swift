import Foundation
#if canImport(ImageIO)
import ImageIO
#endif
#if canImport(PDFKit)
import PDFKit
#endif

// Files (lane 5): the listener's computer's files as pages. Routes (internal/autoputer/files.go, custodian ownerfiles):
//   GET    /api/files              the root, a JSON array of {name, type: "file"|"directory", size, modified (RFC 3339)}
//   GET    /api/files/{path}       a folder's array, or the file's bytes (?disposition=inline for media)
//   POST   /api/files/{path}       make a folder           PUT     /api/files/{path}   write a file (raw body)
//   DELETE /api/files/{path}       delete                  PATCH   /api/files/{path} {"move_to": path}   rename or move a
//                                                           regular file only, and only when the server's move gate says so
// Nothing here prints or records the key. Demo mode serves a tree of invented files from FilesDemoData, in memory.
//
// Ways this can fail (written before the code, per the repo testing rule):
//  1. A path escapes the root ("..", an encoded "..", a NUL): FilePath.normalize refuses it before any request.
//  2. A name with "/" or "." or "" is offered for a new folder or a rename: refused (FilePath.validName), nothing sent.
//  3. A file name with spaces, "?", "#" or "%" reaches a URL: every component is percent-encoded; the cache file name is
//     sanitised (the app:/ path of a cached file is used in an iframe src and must not need quoting).
//  4. Two files with the same name in different folders (or a file replaced on the server) share a cache entry: the cache
//     key hashes path, size and modified time, so a changed file is fetched again and a same-named one is never confused.
//  5. A huge file is opened: a per-kind size cap answers "toolarge" with the facts, never a download that fills the disk.
//  6. The server answers a folder where a file was named, or the reverse: the listing of the parent decides the kind.
//  7. The network fails or the key is refused mid-list: the listing's status is "error" with one plain sentence; a stale
//     listing is never shown as current.
//  8. Sorting: equal names with different case, numbers inside names ("2" before "10"), folders mixed with files, an
//     empty modified date. Folders first except in recents; ties fall back to the path so the order is total.
//  9. Search walks folders: a bounded walk (depth 4, 80 folders) so a deep tree cannot hang the page.
// 10. Recents point at a file that was deleted: opening answers "not found" and drops it from the list.
// 11. Rename of a folder (the server moves regular files only) or onto an existing name: a plain refusal, state unchanged.
// 12. Mutations while the listing is stale: each mutation answers the refreshed listing of the folder it changed.

// MARK: Kinds

public enum FileKind: String, Codable, Sendable, CaseIterable {
    case folder, image, video, audio, pdf, epub, markdown, text, code, csv, other
}

public enum FileTypes {
    static let image: Set<String> = ["png", "jpg", "jpeg", "gif", "webp", "heic", "heif", "bmp", "tif", "tiff"]
    static let video: Set<String> = ["mp4", "m4v", "mov"]
    static let audio: Set<String> = ["mp3", "m4a", "aac", "wav", "aif", "aiff", "caf", "flac"]
    static let markdown: Set<String> = ["md", "markdown"]
    static let text: Set<String> = ["txt", "log", "text"]
    static let csv: Set<String> = ["csv", "tsv"]
    // svg is code on purpose: SVG does not draw as an image on Apple (plan section 4).
    static let code: Set<String> = ["swift", "ts", "tsx", "js", "jsx", "mjs", "json", "py", "go", "rs", "c", "h", "cpp", "java", "kt", "sh", "yml",
                                    "yaml", "toml", "xml", "html", "css", "sql", "svg", "contract", "rb", "lua"]

    public static func kind(name: String, isDirectory: Bool) -> FileKind {
        if isDirectory { return .folder }
        let ext = (name as NSString).pathExtension.lowercased()
        if image.contains(ext) { return .image }
        if video.contains(ext) { return .video }
        if audio.contains(ext) { return .audio }
        if ext == "pdf" { return .pdf }
        if ext == "epub" { return .epub }
        if markdown.contains(ext) { return .markdown }
        if csv.contains(ext) { return .csv }
        if text.contains(ext) { return .text }
        if code.contains(ext) { return .code }
        return .other
    }

    public static func label(_ k: FileKind) -> String {
        switch k {
        case .folder: return "Folder"
        case .image: return "Image"
        case .video: return "Video"
        case .audio: return "Audio"
        case .pdf: return "PDF"
        case .epub: return "Book"
        case .markdown: return "Markdown"
        case .text: return "Text"
        case .code: return "Code"
        case .csv: return "Table"
        case .other: return "File"
        }
    }

    /// An SF Symbol by its Apple name (the iOS host draws it).
    public static func symbol(_ k: FileKind) -> String {
        switch k {
        case .folder: return "symbol:sf/folder"
        case .image: return "symbol:sf/photo"
        case .video: return "symbol:sf/film"
        case .audio: return "symbol:sf/waveform"
        case .pdf: return "symbol:sf/doc.richtext"
        case .epub: return "symbol:sf/book"
        case .markdown: return "symbol:sf/doc.text"
        case .text: return "symbol:sf/doc.plaintext"
        case .code: return "symbol:sf/chevron.left.forwardslash.chevron.right"
        case .csv: return "symbol:sf/tablecells"
        case .other: return "symbol:sf/doc"
        }
    }

    /// The most a page downloads to open this kind. Beyond it the page shows the facts and says why.
    public static func maxBytes(_ k: FileKind) -> Int64 {
        switch k {
        case .markdown, .text, .code, .csv: return 5_000_000
        case .epub: return 100_000_000
        case .pdf: return 200_000_000
        case .image: return 60_000_000
        case .audio: return 300_000_000
        case .video: return 800_000_000
        case .folder, .other: return 0
        }
    }

    public static func mime(for name: String) -> String {
        switch (name as NSString).pathExtension.lowercased() {
        case "png": return "image/png"
        case "jpg", "jpeg": return "image/jpeg"
        case "gif": return "image/gif"
        case "webp": return "image/webp"
        case "heic", "heif": return "image/heic"
        case "pdf": return "application/pdf"
        case "epub": return "application/epub+zip"
        case "mp4", "m4v": return "video/mp4"
        case "mov": return "video/quicktime"
        case "mp3": return "audio/mpeg"
        case "m4a", "aac": return "audio/mp4"
        case "wav": return "audio/wav"
        case "md", "markdown": return "text/markdown"
        case "csv": return "text/csv"
        case "txt", "log": return "text/plain"
        case "json": return "application/json"
        default: return "application/octet-stream"
        }
    }
}

// MARK: Paths and addresses

public enum FilePath {
    /// A canonical path: a leading "/", no empty or "." parts, no trailing "/" (but "/" itself). Nil when it climbs out of
    /// the root with ".." or holds a NUL.
    public static func normalize(_ p: String) -> String? {
        if p.contains("\0") { return nil }
        var parts: [Substring] = []
        for s in p.split(separator: "/", omittingEmptySubsequences: true) {
            if s == "." { continue }
            if s == ".." { if parts.isEmpty { return nil }; parts.removeLast(); continue }
            parts.append(s)
        }
        return "/" + parts.joined(separator: "/")
    }

    public static func join(_ dir: String, _ name: String) -> String { (dir == "/" || dir.isEmpty) ? "/" + name : dir + "/" + name }
    public static func parent(_ p: String) -> String {
        guard let i = p.lastIndex(of: "/"), i != p.startIndex else { return "/" }
        return String(p[p.startIndex..<i])
    }
    public static func name(_ p: String) -> String { p.split(separator: "/").last.map(String.init) ?? "" }

    /// A usable name for a new folder, a rename or an upload.
    public static func validName(_ n: String) -> Bool {
        let t = n.trimmingCharacters(in: .whitespaces)
        return !t.isEmpty && t == n && n != "." && n != ".." && !n.contains("/") && !n.contains("\0") && n.utf8.count <= 255
    }

    /// The page address of a path: a folder ends with "/".
    public static func address(_ path: String, isDirectory: Bool) -> String {
        if path == "/" { return "file:/" }
        return "file:" + path + (isDirectory ? "/" : "")
    }

    /// The path and kind an address names, or nil for a malformed one.
    public static func fromAddress(_ address: String) -> (path: String, isDirectory: Bool)? {
        guard address.hasPrefix("file:") else { return nil }
        let rest = String(address.dropFirst(5))
        guard let p = normalize(rest.isEmpty ? "/" : rest) else { return nil }
        return (p, rest.isEmpty || rest.hasSuffix("/"))
    }

    public static func crumbs(_ path: String) -> [FileCrumb] {
        var out = [FileCrumb(name: "Files", path: "/")]
        var cur = "/"
        for part in path.split(separator: "/") { cur = join(cur, String(part)); out.append(FileCrumb(name: String(part), path: cur)) }
        return out
    }

    private static let unreserved = CharacterSet(charactersIn: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~")
    /// Percent-encode each component (so "?", "#", "%" and spaces stay in the name), for `ChoirClient.send`'s percentEncodedPath.
    public static func encode(_ path: String) -> String {
        path.split(separator: "/", omittingEmptySubsequences: true).map { $0.addingPercentEncoding(withAllowedCharacters: unreserved) ?? String($0) }
            .reduce("") { $0 + "/" + $1 }
    }

    /// A file name safe for the cache and for an `app:/` path used in a src: ASCII letters, digits and ".-_" only.
    public static func cacheName(_ name: String) -> String {
        let ext = (name as NSString).pathExtension.lowercased()
        var stem = (name as NSString).deletingPathExtension
        stem = String(stem.unicodeScalars.map { s in
            (s.isASCII && (CharacterSet.alphanumerics.contains(s) || s == "-" || s == "_")) ? Character(s) : "-"
        })
        if stem.isEmpty || stem.allSatisfy({ $0 == "-" }) { stem = "file" }
        let safeExt = String(ext.unicodeScalars.filter { $0.isASCII && CharacterSet.alphanumerics.contains($0) }.map(Character.init).prefix(8))
        return String(stem.prefix(60)) + (safeExt.isEmpty ? "" : "." + safeExt)
    }
}

// MARK: Shapes the Contract reads

public struct FileCrumb: Codable, Sendable, Equatable {
    public var name: String
    public var path: String
    public init(name: String, path: String) { self.name = name; self.path = path }
}

public struct FileEntry: Sendable, Equatable {
    public var name: String
    public var path: String
    public var isDirectory: Bool
    public var size: Int64
    public var modified: Date?
    public init(name: String, path: String, isDirectory: Bool, size: Int64, modified: Date?) {
        self.name = name; self.path = path; self.isDirectory = isDirectory; self.size = size; self.modified = modified
    }
    public var kind: FileKind { FileTypes.kind(name: name, isDirectory: isDirectory) }
}

public struct FileRow: Codable, Sendable, Equatable {
    public var name: String
    public var path: String
    public var address: String
    public var kind: String
    public var kindLabel: String
    public var symbol: String
    public var isDir: Bool
    public var size: Double
    public var sizeLabel: String
    public var modifiedLabel: String
    /// "12 kB · 3 Nov" (files) or "3 Nov" (folders): the second line of a list row.
    public var detail: String
    /// Where it lives, for search and recent results ("Documents"); empty in a folder listing.
    public var place: String
}

public struct FileListing: Codable, Sendable, Equatable {
    public var path: String
    public var title: String
    public var crumbs: [FileCrumb]
    public var rows: [FileRow]
    public var count: Int
    /// ready | loading | error | empty
    public var status: String
    public var message: String
    /// browse | recent | search
    public var mode: String
    public var sort: String
    public var desc: Bool
    public var query: String
    public static let loading = FileListing(path: "/", title: "Files", crumbs: [FileCrumb(name: "Files", path: "/")], rows: [], count: 0,
                                            status: "loading", message: "", mode: "browse", sort: "name", desc: false, query: "")
}

public struct CsvRow: Codable, Sendable, Equatable {
    public var cells: [String]
    public init(cells: [String]) { self.cells = cells }
}

public struct FileChapterRef: Codable, Sendable, Equatable {
    public var index: Int
    public var title: String
}

public struct FileInfo: Codable, Sendable, Equatable {
    public var address: String
    public var path: String
    public var folder: String
    public var name: String
    public var kind: String
    public var kindLabel: String
    public var size: Double
    public var sizeLabel: String
    public var modifiedLabel: String
    public var mime: String
    /// ready | loading | error | toolarge | notfound | folder
    public var status: String
    public var message: String
    /// An `app:/cache/...` path the image, video, audio and PDF elements play from. Empty for kinds with no viewer.
    public var src: String
    /// The disk path behind `src`, for the share sheet.
    public var local: String
    public var text: String
    public var truncated: Bool
    public var rows: [CsvRow]
    public var pages: Int
    /// Pixel size of an image (0 when unknown): the pinch-and-pan viewer fits it to its frame.
    public var width: Double
    public var height: Double
    public var title: String
    public var author: String
    public var chapters: [FileChapterRef]

    public static let empty = FileInfo(address: "", path: "", folder: "/", name: "", kind: "other", kindLabel: "File", size: 0, sizeLabel: "", modifiedLabel: "",
                                       mime: "", status: "loading", message: "", src: "", local: "", text: "", truncated: false, rows: [], pages: 0, width: 0, height: 0,
                                       title: "", author: "", chapters: [])
}

public struct FileChapter: Codable, Sendable, Equatable {
    public var index: Int
    public var count: Int
    public var title: String
    public var markdown: String
    public var status: String
}

public struct FilePages: Codable, Sendable, Equatable {
    public var query: String
    public var pages: [Int]
    public var message: String
}

// MARK: Labels

public enum FileLabels {
    /// "0 B", "812 B", "1.2 kB", "34 kB", "5.6 MB" (decimal units, one decimal under 10).
    public static func size(_ n: Int64) -> String {
        if n < 1000 { return "\(max(0, n)) B" }
        let units = ["kB", "MB", "GB", "TB"]
        var v = Double(n) / 1000; var i = 0
        while v >= 1000 && i < units.count - 1 { v /= 1000; i += 1 }
        return (v < 10 ? String(format: "%.1f", v) : String(format: "%.0f", v)) + " " + units[i]
    }

    /// "Today", "Yesterday", "3 Nov", "3 Nov 2025". Fixed English, fixed time zone (UTC) so the same file reads the same everywhere.
    public static func date(_ d: Date?, now: Date = Date()) -> String {
        guard let d else { return "" }
        var cal = Calendar(identifier: .gregorian); cal.timeZone = TimeZone(identifier: "UTC")!
        if cal.isDate(d, inSameDayAs: now) { return "Today" }
        if let y = cal.date(byAdding: .day, value: -1, to: now), cal.isDate(d, inSameDayAs: y) { return "Yesterday" }
        let f = DateFormatter()
        f.locale = Locale(identifier: "en_US_POSIX"); f.timeZone = TimeZone(identifier: "UTC")!
        f.dateFormat = cal.component(.year, from: d) == cal.component(.year, from: now) ? "d MMM" : "d MMM yyyy"
        return f.string(from: d)
    }

    public static func parseDate(_ s: String) -> Date? {
        let f = ISO8601DateFormatter()
        if let d = f.date(from: s) { return d }
        f.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return f.date(from: s)
    }
}

// MARK: Sorting

public enum FileSort: String, Sendable, CaseIterable {
    case name, modified, size, kind

    /// Folders first, then by the key (ascending: names A-Z, oldest first, smallest first, kind labels A-Z). `desc` flips the key,
    /// not the folders-first rule. Every comparison falls back to the path, so the order is total.
    public func sorted(_ entries: [FileEntry], desc: Bool, foldersFirst: Bool = true) -> [FileEntry] {
        func byName(_ a: FileEntry, _ b: FileEntry) -> ComparisonResult { a.name.localizedStandardCompare(b.name) }
        func key(_ a: FileEntry, _ b: FileEntry) -> ComparisonResult {
            switch self {
            case .name: return byName(a, b)
            case .modified:
                let x = a.modified ?? .distantPast, y = b.modified ?? .distantPast
                return x == y ? .orderedSame : (x < y ? .orderedAscending : .orderedDescending)
            case .size: return a.size == b.size ? .orderedSame : (a.size < b.size ? .orderedAscending : .orderedDescending)
            case .kind:
                let r = FileTypes.label(a.kind).compare(FileTypes.label(b.kind))
                return r == .orderedSame ? byName(a, b) : r
            }
        }
        return entries.sorted { a, b in
            if foldersFirst, a.isDirectory != b.isDirectory { return a.isDirectory }
            var r = key(a, b)
            if r == .orderedSame { r = a.path.compare(b.path) } else if desc { r = r == .orderedAscending ? .orderedDescending : .orderedAscending }
            return r == .orderedAscending
        }
    }
}

// MARK: CSV

public enum Csv {
    /// RFC 4180: quoted fields, doubled quotes, commas or the given separator inside quotes, CRLF or LF, a final line with no
    /// newline, a ragged row (kept as is). A stray quote inside an unquoted field stays literal. At most `maxRows` rows.
    public static func parse(_ text: String, separator: Character = ",", maxRows: Int = 500) -> [[String]] {
        var rows: [[String]] = []; var row: [String] = []; var field = ""; var inQuotes = false
        var it = text.makeIterator(); var pending: Character? = nil
        func next() -> Character? { if let p = pending { pending = nil; return p }; return it.next() }
        while let c = next() {
            if inQuotes {
                if c == "\"" { if let n = next() { if n == "\"" { field.append("\"") } else { inQuotes = false; pending = n } } else { inQuotes = false } }
                else { field.append(c) }
            } else if c == "\"" && field.isEmpty { inQuotes = true }
            else if c == separator { row.append(field); field = "" }
            else if c == "\n" || c == "\r\n" || c == "\r" {
                row.append(field); field = ""
                if !(row.count == 1 && row[0].isEmpty) { rows.append(row) }
                row = []
                if rows.count >= maxRows { return rows }
            } else { field.append(c) }
        }
        if !field.isEmpty || !row.isEmpty { row.append(field); rows.append(row) }
        return rows
    }
}

// MARK: Backends

public enum FilesError: Error, CustomStringConvertible, Sendable, Equatable {
    case notFound, exists, tooLarge(Int64), unsupported(String), badName, notADirectory, server(String)
    public var description: String {
        switch self {
        case .notFound: return "That file is not there any more."
        case .exists: return "Something with that name is already there."
        case .tooLarge(let n): return "It is \(FileLabels.size(n)), too large to open on this phone."
        case .unsupported(let m): return m
        case .badName: return "That name cannot be used. Avoid slashes and empty names."
        case .notADirectory: return "That is a file, not a folder."
        case .server(let m): return m
        }
    }
}

public protocol FilesBackend: Sendable {
    func list(_ path: String) async throws -> [FileEntry]
    /// Write the file's bytes to `dest` (a file URL; its folder exists). Throws `.tooLarge` when it is over `maxBytes`.
    func fetch(_ path: String, size: Int64, to dest: URL, maxBytes: Int64) async throws
    func put(_ path: String, from file: URL) async throws
    func mkdir(_ path: String) async throws
    func delete(_ path: String) async throws
    func move(_ from: String, to: String) async throws
}

/// The real thing: the listener's computer, with the device key.
public final class ChoirFilesBackend: FilesBackend, @unchecked Sendable {
    let env: ChoirEnvironment
    public init(env: ChoirEnvironment = .shared) { self.env = env }

    private func route(_ path: String) -> String { path == "/" ? "/api/files" : "/api/files" + FilePath.encode(path) }

    func request(_ method: String, _ path: String, query: [String: String] = [:]) async throws -> URLRequest {
        let c = env.client()
        guard let key = env.keys.get(KeyAccount.apiKey) else { throw ChoirError.notSignedIn }
        guard var comps = URLComponents(url: c.baseURL, resolvingAgainstBaseURL: false) else { throw ChoirError.decode("bad base URL") }
        comps.percentEncodedPath = route(path)
        if !query.isEmpty { comps.queryItems = query.sorted { $0.key < $1.key }.map { URLQueryItem(name: $0.key, value: $0.value) } }
        guard let url = comps.url else { throw ChoirError.decode("bad path") }
        var r = URLRequest(url: url, timeoutInterval: 30)
        r.httpMethod = method
        r.setValue("Bearer \(key)", forHTTPHeaderField: "Authorization")
        r.setValue(try await c.computerID(), forHTTPHeaderField: "X-Choir-Computer")
        return r
    }

    private func check(_ resp: URLResponse, data: Data? = nil, path: String) throws {
        guard let h = resp as? HTTPURLResponse else { throw ChoirError.decode("no HTTP response") }
        if (200..<300).contains(h.statusCode) { return }
        switch h.statusCode {
        case 401: throw ChoirError.unauthorized
        case 403: throw ChoirError.forbidden("files")
        case 404: throw FilesError.notFound
        case 409: throw FilesError.exists
        case 405: throw FilesError.unsupported("This computer does not allow that here yet.")
        default:
            let msg = data.flatMap { (try? JSONSerialization.jsonObject(with: $0)) as? [String: Any] }?["error"] as? String ?? ""
            throw ChoirError.server(status: h.statusCode, message: String(msg.prefix(200)))
        }
    }

    public func list(_ path: String) async throws -> [FileEntry] {
        let r = try await request("GET", path)
        let (data, resp) = try await env.session.data(for: r)
        try check(resp, data: data, path: path)
        guard let arr = try? JSONSerialization.jsonObject(with: data) as? [[String: Any]] else { throw ChoirError.decode("not a file listing") }
        return arr.compactMap { o in
            guard let name = o["name"] as? String, FilePath.validName(name) else { return nil }
            return FileEntry(name: name, path: FilePath.join(path, name), isDirectory: (o["type"] as? String) == "directory",
                             size: (o["size"] as? NSNumber)?.int64Value ?? 0, modified: (o["modified"] as? String).flatMap(FileLabels.parseDate))
        }
    }

    public func fetch(_ path: String, size: Int64, to dest: URL, maxBytes: Int64) async throws {
        if size > maxBytes { throw FilesError.tooLarge(size) }
        let r = try await request("GET", path, query: ["disposition": "inline"])
        let (tmp, resp) = try await env.session.download(for: r)
        defer { try? FileManager.default.removeItem(at: tmp) }
        try check(resp, path: path)
        try? FileManager.default.removeItem(at: dest)
        try FileManager.default.moveItem(at: tmp, to: dest)
    }

    public func put(_ path: String, from file: URL) async throws {
        var r = try await request("PUT", path)
        r.setValue(FileTypes.mime(for: FilePath.name(path)), forHTTPHeaderField: "Content-Type")
        let (data, resp) = try await env.session.upload(for: r, fromFile: file)
        try check(resp, data: data, path: path)
    }

    public func mkdir(_ path: String) async throws {
        let r = try await request("POST", path)
        let (data, resp) = try await env.session.data(for: r)
        try check(resp, data: data, path: path)
    }

    public func delete(_ path: String) async throws {
        let r = try await request("DELETE", path)
        let (data, resp) = try await env.session.data(for: r)
        try check(resp, data: data, path: path)
    }

    public func move(_ from: String, to: String) async throws {
        var r = try await request("PATCH", from)
        r.setValue("application/json", forHTTPHeaderField: "Content-Type")
        r.httpBody = try JSONSerialization.data(withJSONObject: ["move_to": to])
        let (data, resp) = try await env.session.data(for: r)
        try check(resp, data: data, path: from)
    }
}

/// Demo mode: a small tree of invented files, held in memory so new folders, deletes and renames can be seen working.
public final class DemoFilesBackend: FilesBackend, @unchecked Sendable {
    struct Node { var isDir: Bool; var data: Data; var modified: Date }
    private let lock = NSLock()
    private var nodes: [String: Node] = [:]

    public init(now: Date = Date()) {
        // Dates are fixed in the past relative to a reference, so "Today" and "Yesterday" appear in the demo.
        let ref = now
        let day: (Double) -> Date = { ref.addingTimeInterval(-$0 * 86400) }
        nodes["/"] = Node(isDir: true, data: Data(), modified: day(0))
        let ages: [String: Double] = ["Books/The Lantern Keeper.epub": 20, "Documents/Council minutes.pdf": 1, "Documents/Budget 2026.csv": 3,
                                      "Documents/Harbor notes.md": 0, "Documents/Key list.txt": 40, "Documents/Lamps.swift": 9,
                                      "Documents/Survey readings.dat": 12, "Pictures/Harbor test card.png": 5, "Media/Tone 440.wav": 7, "Media/Pattern.mp4": 8]
        for (k, b64) in FilesDemoData.base64 {
            let path = "/" + k
            var dir = FilePath.parent(path)
            while dir != "/" { nodes[dir] = nodes[dir] ?? Node(isDir: true, data: Data(), modified: day(2)); dir = FilePath.parent(dir) }
            nodes[path] = Node(isDir: false, data: Data(base64Encoded: b64, options: .ignoreUnknownCharacters) ?? Data(), modified: day(ages[k] ?? 10))
        }
        nodes["/Documents/Drafts"] = Node(isDir: true, data: Data(), modified: day(4))
        nodes["/Readme.txt"] = Node(isDir: false, data: Data("Demo files for Autoradio. Everything here is invented.\n".utf8), modified: day(30))
    }

    private func locked<T>(_ f: () throws -> T) rethrows -> T { lock.lock(); defer { lock.unlock() }; return try f() }

    public func list(_ path: String) async throws -> [FileEntry] {
        try locked {
            guard let n = nodes[path] else { throw FilesError.notFound }
            guard n.isDir else { throw FilesError.notADirectory }
            return nodes.filter { $0.key != "/" && FilePath.parent($0.key) == path }.map {
                FileEntry(name: FilePath.name($0.key), path: $0.key, isDirectory: $0.value.isDir, size: Int64($0.value.data.count), modified: $0.value.modified)
            }
        }
    }

    public func fetch(_ path: String, size: Int64, to dest: URL, maxBytes: Int64) async throws {
        let n = locked { nodes[path] }
        guard let n, !n.isDir else { throw FilesError.notFound }
        if Int64(n.data.count) > maxBytes { throw FilesError.tooLarge(Int64(n.data.count)) }
        try n.data.write(to: dest, options: .atomic)
    }

    public func put(_ path: String, from file: URL) async throws {
        let d = try Data(contentsOf: file)
        try locked {
            guard nodes[FilePath.parent(path)]?.isDir == true else { throw FilesError.notFound }
            if nodes[path]?.isDir == true { throw FilesError.exists }
            nodes[path] = Node(isDir: false, data: d, modified: Date())
        }
    }

    public func mkdir(_ path: String) async throws {
        try locked {
            guard nodes[FilePath.parent(path)]?.isDir == true else { throw FilesError.notFound }
            if nodes[path] != nil { throw FilesError.exists }
            nodes[path] = Node(isDir: true, data: Data(), modified: Date())
        }
    }

    public func delete(_ path: String) async throws {
        try locked {
            guard path != "/", nodes[path] != nil else { throw FilesError.notFound }
            for k in Array(nodes.keys) where k == path || k.hasPrefix(path + "/") { nodes[k] = nil }
        }
    }

    public func move(_ from: String, to: String) async throws {
        try locked {
            guard let n = nodes[from], from != "/" else { throw FilesError.notFound }
            if n.isDir { throw FilesError.unsupported("Folders cannot be renamed here yet.") }
            if nodes[to] != nil { throw FilesError.exists }
            guard nodes[FilePath.parent(to)]?.isDir == true else { throw FilesError.notFound }
            nodes[to] = n; nodes[from] = nil
        }
    }
}

// MARK: Recents

public struct RecentItem: Codable, Sendable, Equatable {
    public var path: String
    public var size: Int64
    public var modified: Date?
    public var openedAt: Date
}

/// The files the listener opened last, newest first, kept on this phone (a list of paths with the facts seen).
public final class RecentFiles: @unchecked Sendable {
    public static let limit = 30
    private let lock = NSLock()
    private var items: [RecentItem] = []
    private let file: URL?
    public init(file: URL?) {
        self.file = file
        if let file, let d = try? Data(contentsOf: file) {
            let dec = JSONDecoder(); dec.dateDecodingStrategy = .iso8601
            items = (try? dec.decode([RecentItem].self, from: d)) ?? []
        }
    }
    public func list() -> [RecentItem] { lock.lock(); defer { lock.unlock() }; return items }
    public func touch(_ e: FileEntry, at: Date = Date()) {
        lock.lock()
        items.removeAll { $0.path == e.path }
        items.insert(RecentItem(path: e.path, size: e.size, modified: e.modified, openedAt: at), at: 0)
        if items.count > Self.limit { items.removeLast(items.count - Self.limit) }
        lock.unlock(); save()
    }
    public func remove(_ path: String) {
        lock.lock(); items.removeAll { $0.path == path || $0.path.hasPrefix(path + "/") }; lock.unlock(); save()
    }
    public func rename(_ from: String, to: String) {
        lock.lock(); for i in items.indices where items[i].path == from { items[i].path = to }; lock.unlock(); save()
    }
    private func save() {
        guard let file else { return }
        lock.lock(); let snapshot = items; lock.unlock()
        let enc = JSONEncoder(); enc.dateEncodingStrategy = .iso8601
        try? FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
        if let d = try? enc.encode(snapshot) { try? d.write(to: file, options: .atomic) }
    }
}

// MARK: The service

/// What the module's sources call. Holds the backend, the on-disk cache of opened files and the recents.
public final class FilesService: @unchecked Sendable {
    let backend: FilesBackend
    let cacheRoot: URL
    let appPrefix: String
    let recents: RecentFiles
    let now: @Sendable () -> Date
    private let lock = NSLock()
    private var books: [String: EpubBook] = [:]

    /// `cacheRoot` is a folder on disk and `appPrefix` the `app:/cache/...` path that names the same folder to Exact.
    public init(backend: FilesBackend, cacheRoot: URL, appPrefix: String, recents: RecentFiles, now: @escaping @Sendable () -> Date = { Date() }) {
        self.backend = backend; self.cacheRoot = cacheRoot; self.appPrefix = appPrefix; self.recents = recents; self.now = now
    }

    // MARK: rows

    func row(_ e: FileEntry, place: String = "") -> FileRow {
        let k = e.kind
        let date = FileLabels.date(e.modified, now: now())
        return FileRow(name: e.name, path: e.path, address: FilePath.address(e.path, isDirectory: e.isDirectory), kind: k.rawValue,
                       kindLabel: FileTypes.label(k), symbol: FileTypes.symbol(k), isDir: e.isDirectory, size: Double(e.size),
                       sizeLabel: e.isDirectory ? "" : FileLabels.size(e.size), modifiedLabel: date,
                       detail: e.isDirectory ? date : [FileLabels.size(e.size), date].filter { !$0.isEmpty }.joined(separator: " · "), place: place)
    }

    private func listing(path: String, mode: String, sort: FileSort, desc: Bool, query: String, entries: [FileEntry], places: [String: String] = [:],
                         status: String? = nil, message: String = "") -> FileListing {
        let rows = entries.map { row($0, place: places[$0.path] ?? "") }
        let title = path == "/" ? "Files" : FilePath.name(path)
        return FileListing(path: path, title: mode == "recent" ? "Recent" : title, crumbs: FilePath.crumbs(path), rows: rows,
                           count: rows.count, status: status ?? (rows.isEmpty ? "empty" : "ready"),
                           message: message.isEmpty && rows.isEmpty ? (mode == "recent" ? "Files you open appear here." : mode == "search" ? "Nothing matches." : "This folder is empty.") : message,
                           mode: mode, sort: sort.rawValue, desc: desc, query: query)
    }

    public func listing(path rawPath: String, mode: String, sort sortName: String, desc: Bool, query: String) async -> FileListing {
        let sort = FileSort(rawValue: sortName) ?? .name
        let path = FilePath.normalize(rawPath) ?? "/"
        let q = query.trimmingCharacters(in: .whitespacesAndNewlines)
        do {
            if !q.isEmpty { return try await search(path: path, query: q, sort: sort, desc: desc) }
            if mode == "recent" {
                let entries = recents.list().map { FileEntry(name: FilePath.name($0.path), path: $0.path, isDirectory: false, size: $0.size, modified: $0.modified) }
                var places: [String: String] = [:]
                for e in entries { places[e.path] = FilePath.parent(e.path) == "/" ? "Files" : FilePath.name(FilePath.parent(e.path)) }
                return listing(path: "/", mode: "recent", sort: sort, desc: desc, query: "", entries: entries, places: places)
            }
            let entries = try await backend.list(path)
            return listing(path: path, mode: "browse", sort: sort, desc: desc, query: "", entries: sort.sorted(entries, desc: desc))
        } catch {
            return listing(path: path, mode: mode, sort: sort, desc: desc, query: q, entries: [], status: "error", message: Self.say(error))
        }
    }

    /// A bounded walk under `path`: names containing the query (case and accents ignored).
    func search(path: String, query: String, sort: FileSort, desc: Bool) async throws -> FileListing {
        var found: [FileEntry] = []; var places: [String: String] = [:]
        var queue: [(String, Int)] = [(path, 0)]; var visited = 0
        while !queue.isEmpty, visited < 80, found.count < 200 {
            let (dir, depth) = queue.removeFirst(); visited += 1
            guard let entries = try? await backend.list(dir) else { if dir == path { throw FilesError.notFound }; continue }
            for e in entries {
                if e.name.range(of: query, options: [.caseInsensitive, .diacriticInsensitive]) != nil {
                    found.append(e); places[e.path] = dir == "/" ? "Files" : FilePath.name(dir)
                }
                if e.isDirectory, depth < 4 { queue.append((e.path, depth + 1)) }
            }
        }
        return listing(path: path, mode: "search", sort: sort, desc: desc, query: query, entries: sort.sorted(found, desc: desc), places: places)
    }

    // MARK: opening

    static func hash(_ s: String) -> String {
        var h: UInt64 = 0xcbf29ce484222325
        for b in s.utf8 { h = (h ^ UInt64(b)) &* 0x100000001b3 }
        return String(h, radix: 16)
    }

    private func info(_ e: FileEntry, status: String, message: String = "") -> FileInfo {
        var i = FileInfo.empty
        i.address = FilePath.address(e.path, isDirectory: e.isDirectory); i.path = e.path; i.folder = FilePath.parent(e.path); i.name = e.name
        i.kind = e.kind.rawValue; i.kindLabel = FileTypes.label(e.kind); i.size = Double(e.size); i.sizeLabel = FileLabels.size(e.size)
        i.modifiedLabel = FileLabels.date(e.modified, now: now()); i.mime = FileTypes.mime(for: e.name); i.status = status; i.message = message
        i.title = (e.name as NSString).deletingPathExtension
        return i
    }

    static func say(_ error: Error) -> String {
        if let e = error as? FilesError { return e.description }
        if let e = error as? ChoirError {
            switch e {
            case .unauthorized, .notSignedIn: return "Sign in again to see your files."
            case .transport, .timeout: return "Could not reach your computer. Check the connection and try again."
            case .forbidden: return "This device is not allowed to read files."
            default: return "Could not read your files. \(e)"
            }
        }
        if error is CancellationError { return "" }
        return "Something went wrong reading the file."
    }

    /// What an address names, without bringing the file to disk: for cards and the page model.
    public func facts(address: String) async -> FileInfo {
        guard let (path, isDir) = FilePath.fromAddress(address) else { var i = FileInfo.empty; i.address = address; i.status = "error"; return i }
        if isDir {
            var i = FileInfo.empty; i.address = address; i.path = path; i.name = path == "/" ? "Files" : FilePath.name(path); i.kind = "folder"; i.kindLabel = "Folder"
            i.status = "folder"; return i
        }
        guard let e = try? await backend.list(FilePath.parent(path)).first(where: { $0.path == path }) else {
            var i = FileInfo.empty; i.address = address; i.path = path; i.name = FilePath.name(path); i.status = "notfound"; return i
        }
        return info(e, status: "ready")
    }

    /// Open the file a `file:` address names: find it in its folder, bring it to disk (cached), and read what its viewer needs.
    public func open(address: String) async -> FileInfo {
        guard let (path, isDir) = FilePath.fromAddress(address) else {
            var i = FileInfo.empty; i.address = address; i.status = "error"; i.message = "That is not a file address."; return i
        }
        if isDir { var i = FileInfo.empty; i.address = address; i.path = path; i.status = "folder"; i.name = FilePath.name(path); return i }
        let entry: FileEntry
        do {
            guard let e = try await backend.list(FilePath.parent(path)).first(where: { $0.path == path }) else {
                recents.remove(path)
                var i = FileInfo.empty; i.address = address; i.path = path; i.name = FilePath.name(path); i.status = "notfound"; i.message = FilesError.notFound.description; return i
            }
            entry = e
        } catch {
            var i = FileInfo.empty; i.address = address; i.path = path; i.name = FilePath.name(path); i.message = Self.say(error)
            i.status = (error as? FilesError) == .notFound ? "notfound" : "error"
            if i.status == "notfound" { recents.remove(path) }
            return i
        }
        if entry.isDirectory { return info(entry, status: "folder") }
        let kind = entry.kind
        var out = info(entry, status: "ready")
        if kind == .other { recents.touch(entry); return out }
        let cap = FileTypes.maxBytes(kind)
        if entry.size > cap {
            out.status = "toolarge"; out.message = FilesError.tooLarge(entry.size).description; return out
        }
        do {
            let key = Self.hash("\(entry.path)|\(entry.size)|\(entry.modified.map { String($0.timeIntervalSince1970) } ?? "")")
            let dir = cacheRoot.appendingPathComponent(key, isDirectory: true)
            let name = FilePath.cacheName(entry.name)
            let dest = dir.appendingPathComponent(name)
            if !FileManager.default.fileExists(atPath: dest.path) || ((try? FileManager.default.attributesOfItem(atPath: dest.path)[.size] as? Int64) ?? -1) != entry.size {
                try FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
                try await backend.fetch(entry.path, size: entry.size, to: dest, maxBytes: cap)
            }
            out.local = dest.path
            out.src = appPrefix + "/" + key + "/" + name
            switch kind {
            case .markdown, .text, .code:
                let (s, cut) = Self.readText(dest, limit: 120_000)
                out.text = s; out.truncated = cut
            case .csv:
                let (s, cut) = Self.readText(dest, limit: 256_000)
                let sep: Character = (entry.name as NSString).pathExtension.lowercased() == "tsv" ? "\t" : ","
                out.rows = Csv.parse(s, separator: sep, maxRows: 300).map { CsvRow(cells: Array($0.prefix(12))) }
                out.truncated = cut
            case .image:
                let (w, h) = Self.imageSize(dest); out.width = Double(w); out.height = Double(h)
            case .pdf:
                out.pages = Self.pdfPageCount(dest)
                if let t = Self.pdfTitle(dest), !t.isEmpty { out.title = t }
            case .epub:
                let book = try self.loadBook(at: dest)
                out.title = book.title.isEmpty ? out.title : book.title; out.author = book.author
                out.chapters = book.chapters.enumerated().map { FileChapterRef(index: $0.offset, title: $0.element.title) }
                if book.chapters.isEmpty { out.status = "error"; out.message = "This book has no readable chapters." }
            default: break
            }
            recents.touch(entry, at: now())
        } catch {
            out.status = "error"
            if let fe = error as? FilesError, case .tooLarge = fe { out.status = "toolarge" }
            out.message = (error as? EpubError)?.description ?? Self.say(error)
        }
        return out
    }

    static func readText(_ url: URL, limit: Int) -> (String, Bool) {
        guard let h = try? FileHandle(forReadingFrom: url) else { return ("", false) }
        defer { try? h.close() }
        let data = (try? h.read(upToCount: limit + 1)) ?? Data()
        let cut = data.count > limit
        let bytes = cut ? Self.withoutPartialUTF8(data.prefix(limit)) : data
        return (String(decoding: bytes, as: UTF8.self), cut)
    }

    /// Drop a UTF-8 sequence the cut left unfinished (and only that: a character that ends exactly at the cut stays).
    static func withoutPartialUTF8(_ d: Data) -> Data {
        let b = Array(d)
        var i = b.count - 1
        var back = 0
        while i >= 0, back < 3, b[i] & 0xC0 == 0x80 { i -= 1; back += 1 }      // continuation bytes at the end
        guard i >= 0 else { return Data(b) }
        let lead = b[i]
        let need = lead >= 0xF0 ? 4 : lead >= 0xE0 ? 3 : lead >= 0xC0 ? 2 : 1
        return b.count - i < need ? Data(b[..<i]) : Data(b)
    }

    // MARK: books and PDFs

    private func loadBook(at url: URL) throws -> EpubBook {
        lock.lock(); if let b = books[url.path] { lock.unlock(); return b }; lock.unlock()
        let b = try Epub.parse(data: Data(contentsOf: url, options: .mappedIfSafe))
        lock.lock(); books[url.path] = b; lock.unlock()
        return b
    }

    /// One chapter of the book at `address`, as Markdown (the reader's component is not importable yet: lane 3's blocks).
    public func chapter(address: String, index: Int) async -> FileChapter {
        let i = await open(address: address)
        guard i.status == "ready", i.kind == "epub", !i.local.isEmpty, let book = try? loadBook(at: URL(fileURLWithPath: i.local)) else {
            return FileChapter(index: 0, count: 0, title: "", markdown: "", status: i.status == "ready" ? "error" : i.status)
        }
        let n = book.chapters.count
        guard n > 0 else { return FileChapter(index: 0, count: 0, title: "", markdown: "", status: "empty") }
        let k = min(max(index, 0), n - 1)
        return FileChapter(index: k, count: n, title: book.chapters[k].title, markdown: Epub.markdown(book.chapters[k].blocks), status: "ready")
    }

    /// Which pages of the PDF at `address` hold `query` (1-based). PDFKit does the search; the page view cannot highlight it.
    public func findPages(address: String, query: String) async -> FilePages {
        let q = query.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !q.isEmpty else { return FilePages(query: "", pages: [], message: "") }
        let i = await open(address: address)
        guard i.status == "ready", i.kind == "pdf", !i.local.isEmpty else { return FilePages(query: q, pages: [], message: "Open a PDF to search it.") }
        let pages = Self.pdfFind(URL(fileURLWithPath: i.local), q)
        return FilePages(query: q, pages: pages, message: pages.isEmpty ? "No page has \u{201C}\(q)\u{201D}." : "")
    }

    static func imageSize(_ url: URL) -> (Int, Int) {
        #if canImport(ImageIO)
        guard let src = CGImageSourceCreateWithURL(url as CFURL, nil), let p = CGImageSourceCopyPropertiesAtIndex(src, 0, nil) as? [CFString: Any],
              let w = p[kCGImagePropertyPixelWidth] as? Int, let h = p[kCGImagePropertyPixelHeight] as? Int else { return (0, 0) }
        return (w, h)
        #else
        return (0, 0)
        #endif
    }

    static func pdfPageCount(_ url: URL) -> Int {
        #if canImport(PDFKit)
        return PDFDocument(url: url)?.pageCount ?? 0
        #else
        return 0
        #endif
    }
    static func pdfTitle(_ url: URL) -> String? {
        #if canImport(PDFKit)
        return PDFDocument(url: url)?.documentAttributes?[PDFDocumentAttribute.titleAttribute] as? String
        #else
        return nil
        #endif
    }
    static func pdfFind(_ url: URL, _ q: String) -> [Int] {
        #if canImport(PDFKit)
        guard let doc = PDFDocument(url: url) else { return [] }
        var pages = Set<Int>()
        for s in doc.findString(q, withOptions: [.caseInsensitive, .diacriticInsensitive]) {
            for p in s.pages { pages.insert(doc.index(for: p) + 1) }
        }
        return pages.sorted()
        #else
        return []
        #endif
    }

    // MARK: changes (each answers the refreshed listing of the folder it changed)

    public func makeFolder(in dir: String, name: String) async -> FileListing {
        let d = FilePath.normalize(dir) ?? "/"
        guard FilePath.validName(name) else { return await after(d, "That name cannot be used. Avoid slashes and empty names.", failed: true) }
        do { try await backend.mkdir(FilePath.join(d, name)); return await after(d, "Made the folder \u{201C}\(name)\u{201D}.") }
        catch { return await after(d, Self.say(error), failed: true) }
    }

    public func delete(path: String) async -> FileListing {
        guard let p = FilePath.normalize(path), p != "/" else { return await after("/", "That cannot be deleted.", failed: true) }
        let d = FilePath.parent(p)
        do { try await backend.delete(p); recents.remove(p); return await after(d, "Deleted \u{201C}\(FilePath.name(p))\u{201D}.") }
        catch { return await after(d, Self.say(error), failed: true) }
    }

    public func rename(path: String, to newName: String) async -> FileListing {
        guard let p = FilePath.normalize(path), p != "/" else { return await after("/", "That cannot be renamed.", failed: true) }
        let d = FilePath.parent(p)
        guard FilePath.validName(newName) else { return await after(d, "That name cannot be used. Avoid slashes and empty names.", failed: true) }
        let dest = FilePath.join(d, newName)
        do { try await backend.move(p, to: dest); recents.rename(p, to: dest); return await after(d, "Renamed to \u{201C}\(newName)\u{201D}.") }
        catch { return await after(d, Self.say(error), failed: true) }
    }

    /// Upload a copy of a local file (a picked file under `app:/tmp/picked`) into `dir`.
    public func upload(from file: URL, name: String, into dir: String) async -> FileListing {
        let d = FilePath.normalize(dir) ?? "/"
        guard FilePath.validName(name) else { return await after(d, "That name cannot be used. Avoid slashes and empty names.", failed: true) }
        do { try await backend.put(FilePath.join(d, name), from: file); return await after(d, "Added \u{201C}\(name)\u{201D}.") }
        catch { return await after(d, Self.say(error), failed: true) }
    }

    private func after(_ dir: String, _ message: String, failed: Bool = false) async -> FileListing {
        var l = await listing(path: dir, mode: "browse", sort: "name", desc: false, query: "")
        l.message = message
        _ = failed
        return l
    }
}
