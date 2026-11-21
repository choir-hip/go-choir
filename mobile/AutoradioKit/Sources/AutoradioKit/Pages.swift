import Foundation

// Everything is a page. A page is {address, kind, title}; the kind picks the renderer (the contract's page host).
// Addresses (plan section 6, "The spine"):
//   paper:front  paper:section/<id>  pub:<route>  texture:<doc>[@rev]  file:<path>  mail:<id>  mailbox:<folder>
//   textures:  activity:  settings:<screen>  lab:components  lab:trace
// The spine ships PageSource (the model behind a page) and two trivial renderers in the
// contract (plain text, "not yet"). The open set is shared code's (AutoradioLab/pages.ts); the voice is handed the pages as facts (OpenPageFacts, LocalAck.swift). A lane adds a page kind by registering a PageSource for its kinds from its own
// AutoradioModule+<Lane>.swift and a renderer in its own .contract file.

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
