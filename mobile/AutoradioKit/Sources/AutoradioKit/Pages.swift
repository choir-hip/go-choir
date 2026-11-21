import Foundation

// Everything is a page. A page is {address, kind, title}; the kind picks the renderer (the contract's page host).
// Addresses (plan section 6, "The spine"):
//   paper:front  paper:section/<id>  pub:<route>  texture:<doc>[@rev]  file:<path>  mail:<id>  mailbox:<folder>
//   textures:  activity:  settings:<screen>  lab:components  lab:trace
// The spine ships OpenPages (this file), PageSource (the model behind a page) and two trivial renderers in the
// contract (plain text, "not yet"). A lane adds a page kind by registering a PageSource for its kinds from its own
// AutoradioModule+<Lane>.swift and a renderer in its own .contract file.

public struct Page: Codable, Sendable, Equatable {
    public var address: String
    public var kind: String
    public var title: String
    public var opener: String = ""
    public var pinned: Bool = false
    public init(address: String, kind: String, title: String, opener: String = "", pinned: Bool = false) {
        self.address = address; self.kind = kind; self.title = title; self.opener = opener; self.pinned = pinned
    }
}

public enum PageAddress {
    /// The address the reader's edition opens at. The server holds no edition document that this app creates (the writer is local),
    /// so "texture:my-edition" would be fetched as a document id and answer 404. Every open of it, and every saved page list
    /// that holds it, lands on the local edit page instead.
    public static let edition = "onboarding:edit"
    /// The Autoradio page. The app opens on it, it is the first card of the deck and it cannot be closed.
    public static let radio = "radio:"
    /// My notebook: what Autoradio has been told and has learned about the listener (station L6).
    public static let notebook = "notebook:"
    public static func canonical(_ address: String) -> String {
        // An imported document's page is "doc:<id>"; a "@<block>" mark (a place in it) is not part of the page's address.
        if address.hasPrefix("doc:"), let at = address.firstIndex(of: "@") { return String(address[..<at]) }
        return address.trimmingCharacters(in: .whitespaces).lowercased() == "texture:my-edition" ? edition : address
    }

    /// The renderer family for an address. One line per family; a lane adds its own here only if it adds a scheme.
    public static func kind(of address: String) -> String {
        let scheme = address.split(separator: ":", maxSplits: 1, omittingEmptySubsequences: false).first.map(String.init) ?? ""
        switch scheme {
        case "radio": return "radio"       // the Autoradio page: the deck's first card, always open
        case "notebook": return "notebook"
        case "paper": return "paper"
        case "pub", "texture": return "reader"
        case "textures": return "textures"
        case "file": return "file"
        case "mail": return "mail"
        case "mailbox": return "mailbox"
        case "activity": return "activity"
        case "settings": return "settings"
        case "lab": return "lab"
        case "imported": return "imported"
        case "doc": return "doc"
        case "conversation": return "conversation"
        case "podcasts", "podcast": return "podcast"
        case "onboarding": return "onboarding"      // lane 8 hook (first run); see mobile/README.md, Requests to the spine
        default: return "unknown"
        }
    }

    public static func rest(of address: String) -> String {
        guard let i = address.firstIndex(of: ":") else { return "" }
        return String(address[address.index(after: i)...])
    }

    /// A title for a page whose own title is not known yet.
    public static func defaultTitle(_ address: String) -> String {
        let rest = rest(of: address)
        switch address {
        case "radio:": return "Autoradio"
        case "notebook:": return "My notebook"
        case "paper:front": return "Paper"
        case "textures:": return "Textures"
        case "activity:": return "Activity"
        case "settings:", "settings:root": return "Settings"
        case "lab:components": return "Components"
        case "lab:trace": return "Trace"
        case "conversation:": return "Conversation"
        case "onboarding:setup": return "Make it yours"
        case "onboarding:edit": return "My edition"
        case "mailbox:inbox": return "Mail"
        case "podcasts:": return "Podcasts"
        case "file:/", "file:": return "Files"
        case "imported:": return "Imported documents"
        default: break
        }
        switch kind(of: address) {
        case "paper": return "Paper · " + rest.replacingOccurrences(of: "section/", with: "")
        case "settings": return "Settings · " + rest
        case "file": return (rest as NSString).lastPathComponent
        case "mailbox": return "Mail · " + rest
        case "mail": return "Message"
        case "doc": return "Document"
        case "podcast": return rest.hasPrefix("episode/") ? "Episode" : "Show"
        case "reader": return rest.isEmpty ? "Story" : rest
        default: return address
        }
    }

    /// The places the launcher lists, in order (plan section 1): name, address.
    public static let places: [(name: String, address: String)] = [
        ("Paper", "paper:front"), ("Textures", "textures:"), ("Files", "file:/"),
        ("Mail", "mailbox:inbox"), ("Podcasts", "podcasts:"), ("Activity", "activity:"), ("Settings", "settings:root"),
    ]
}

/// The ordered set of open pages: open, close, focus, opener, pin, restored at launch from a list of addresses
/// (never resumed work). Never empty: the Autoradio page is always open and cannot be closed.
public final class OpenPages: @unchecked Sendable {
    public static let limit = 30
    private let lock = NSLock()
    private var pages: [Page] = []          // open order
    private var recency: [String] = []      // most recently focused first
    private var lastClosedPage: Page?
    private let file: URL?
    public var onChange: (@Sendable () -> Void)?

    /// `file` nil keeps the set in memory only (tests).
    public init(file: URL?) {
        self.file = file
        load()
        openLocked(Page(address: PageAddress.radio, kind: "radio", title: "Autoradio"), record: false)   // every launch lands on the radio
    }

    /// Sign-out: only the radio stays open; the persisted titles and addresses go.
    public func wipe() {
        lock.lock(); pages = []; recency = []; lastClosedPage = nil; lock.unlock()
        if let file { try? FileManager.default.removeItem(at: file) }
        lock.lock(); _ = openLocked(Page(address: PageAddress.radio, kind: "radio", title: "Autoradio"), record: false); lock.unlock()
        onChange?()
    }

    // MARK: Reading

    /// Most recent first: the order of the pile.
    public func list() -> [Page] {
        lock.lock(); defer { lock.unlock() }
        return recency.compactMap { a in pages.first { $0.address == a } }
    }
    public var current: Page? { lock.lock(); defer { lock.unlock() }; return recency.first.flatMap { a in pages.first { $0.address == a } } }
    public var count: Int { lock.lock(); defer { lock.unlock() }; return pages.count }
    public var lastClosed: Page? { lock.lock(); defer { lock.unlock() }; return lastClosedPage }

    // MARK: Changing

    @discardableResult
    public func open(_ address: String, title: String? = nil, opener: String? = nil) -> Page {
        let address = PageAddress.canonical(address)
        lock.lock()
        let p = openLocked(Page(address: address, kind: PageAddress.kind(of: address),
                                title: (title?.isEmpty == false ? title! : PageAddress.defaultTitle(address)),
                                opener: opener ?? recency.first ?? ""), record: true)
        lock.unlock(); changed(); return p
    }

    @discardableResult
    private func openLocked(_ page: Page, record: Bool) -> Page {
        if let i = pages.firstIndex(where: { $0.address == page.address }) {
            if let t = Optional(page.title), !t.isEmpty, pages[i].title == PageAddress.defaultTitle(page.address) { pages[i].title = t }
            focusLocked(page.address); return pages[i]
        }
        pages.append(page)
        focusLocked(page.address)
        // The pile keeps the 30 most recent unpinned pages.
        while pages.filter({ !$0.pinned }).count > Self.limit,
              let oldest = recency.reversed().first(where: { a in pages.first { $0.address == a }?.pinned == false }) {
            pages.removeAll { $0.address == oldest }; recency.removeAll { $0 == oldest }
        }
        return page
    }

    private func focusLocked(_ address: String) {
        recency.removeAll { $0 == address }
        recency.insert(address, at: 0)
    }

    public func focus(_ address: String) {
        lock.lock()
        if pages.contains(where: { $0.address == address }) { focusLocked(address) }
        lock.unlock(); changed()
    }

    /// Close a page. When it was in front, its opener (if still open) comes forward, else the next most recent.
    public func close(_ address: String) {
        lock.lock()
        guard address != PageAddress.radio, let p = pages.first(where: { $0.address == address }) else { lock.unlock(); return }
        let wasFront = recency.first == address
        pages.removeAll { $0.address == address }
        recency.removeAll { $0 == address }
        lastClosedPage = p
        if wasFront, !p.opener.isEmpty, pages.contains(where: { $0.address == p.opener }) { focusLocked(p.opener) }
        lock.unlock()
        changed()
    }

    public func reopenLastClosed() {
        lock.lock(); let p = lastClosedPage; lastClosedPage = nil; lock.unlock()
        if let p { open(p.address, title: p.title, opener: p.opener) }
    }

    public func togglePin(_ address: String) {
        lock.lock()
        if let i = pages.firstIndex(where: { $0.address == address }) { pages[i].pinned.toggle() }
        lock.unlock(); changed()
    }

    // MARK: Persistence (addresses and titles only)

    private struct Saved: Codable { var pages: [Page]; var recency: [String] }
    private func changed() { save(); onChange?() }
    private func save() {
        guard let file else { return }
        lock.lock(); let s = Saved(pages: pages, recency: recency); lock.unlock()
        try? FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
        if let d = try? JSONEncoder().encode(s) { try? d.write(to: file, options: .atomic) }
    }
    private func load() {
        guard let file, let d = try? Data(contentsOf: file), let s = try? JSONDecoder().decode(Saved.self, from: d) else { return }
        var seen = Set<String>()
        pages = s.pages.map { var p = $0; p.address = PageAddress.canonical(p.address); p.kind = PageAddress.kind(of: p.address); return p }
            .filter { seen.insert($0.address).inserted }
        var inRecency = Set<String>()
        recency = s.recency.map { PageAddress.canonical($0) }.filter { a in pages.contains { $0.address == a } && inRecency.insert(a).inserted }
        for p in pages where !recency.contains(p.address) { recency.append(p.address) }
    }
}

// MARK: PageSource: an address to the model behind its page

/// What a renderer shows. The spine's two trivial renderers show `lines` as plain text (status "ready") or the
/// "not yet" notice (status "notyet"). A lane that needs more than text gives its own source and shapes.
public struct PageModel: Codable, Sendable, Equatable {
    public var address: String
    public var kind: String
    public var title: String
    /// ready | loading | notyet | error
    public var status: String
    public var lines: [String]
    public var message: String
    public init(address: String, kind: String, title: String, status: String, lines: [String] = [], message: String = "") {
        self.address = address; self.kind = kind; self.title = title; self.status = status; self.lines = lines; self.message = message
    }
    public static func notYet(_ address: String) -> PageModel {
        let t = PageAddress.defaultTitle(address)
        return PageModel(address: address, kind: PageAddress.kind(of: address), title: t, status: "notyet", lines: [],
                         message: "\(t) is not built yet.")
    }
}

public protocol PageSource: Sendable {
    /// Whether this source answers the address (usually by kind).
    func handles(_ address: String) -> Bool
    func load(_ address: String) async throws -> PageModel
}

/// The registry. Lanes register from their own AutoradioModule+<Lane>.swift; the first source that handles an
/// address answers, the most recently registered first, so a lane's source wins over the spine's.
public final class PageSources: @unchecked Sendable {
    private let lock = NSLock()
    private var sources: [PageSource] = []
    public init() {}
    private func snapshot() -> [PageSource] { lock.lock(); defer { lock.unlock() }; return sources }
    public func register(_ s: PageSource) { lock.lock(); sources.insert(s, at: 0); lock.unlock() }
    public func model(for address: String) async -> PageModel {
        let list = snapshot()
        for s in list where s.handles(address) {
            do { return try await s.load(address) }
            catch is CancellationError { break }
            catch {
                return PageModel(address: address, kind: PageAddress.kind(of: address), title: PageAddress.defaultTitle(address),
                                 status: "error", message: "Could not open this page. \(error)")
            }
        }
        return .notYet(address)
    }
}

/// The spine's own source: says "not yet" for every place no lane has built.
public struct NotYetPageSource: PageSource {
    public init() {}
    public func handles(_ address: String) -> Bool { true }
    public func load(_ address: String) async throws -> PageModel { .notYet(address) }
}
