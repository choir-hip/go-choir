import Foundation

// The page in front, as the shared reader hands it to the voice (AutoradioLab/reader.ts `hand`, the app module's `pageText` op): the native voice cannot ask the shared module what is on the page,
// and matches what the radio says against it, so the shared page hands over its blocks with their sentences.
// This holds them (at most `limit`, least recently used first, forgotten at sign-out) for three native readers: the page text a turn reads (HeldPageSource), the saved corpus and the follow-along
// (ReaderFollow.locate). Nothing here parses a document.

public struct HeldBlock: Sendable, Equatable {
    public var id: String, kind: String, text: String      // kind: p, h, li, quote, code, embed
    public var sentences: [String]
    public init(id: String, kind: String, text: String, sentences: [String] = []) { self.id = id; self.kind = kind; self.text = text; self.sentences = sentences }
}

public struct HeldPage: Sendable, Equatable {
    public var address: String, title: String, dek: String
    /// story | opinion | document
    public var kind: String
    public var blocks: [HeldBlock]
    public init(address: String, title: String = "", dek: String = "", kind: String = "document", blocks: [HeldBlock] = []) {
        self.address = address; self.title = title; self.dek = dek; self.kind = kind; self.blocks = blocks
    }
    /// What the turn reads: the blocks with text, no code.
    public var lines: [String] { blocks.filter { !$0.text.isEmpty && $0.kind != "code" }.map { $0.text } }
    /// The readable body for the corpus: no code or embeds.
    public var body: String { blocks.filter { !["code", "rule", "embed"].contains($0.kind) }.map(\.text).filter { !$0.isEmpty }.joined(separator: "\n") }

    /// The hand-over as the shared code sends it; nil when it names no address.
    public static func parse(_ r: [String: Any]) -> HeldPage? {
        guard let address = r["address"] as? String, !address.isEmpty else { return nil }
        let blocks = ((r["blocks"] as? [[String: Any]]) ?? []).compactMap { b -> HeldBlock? in
            guard let id = b["id"] as? String, let text = b["text"] as? String else { return nil }
            return HeldBlock(id: id, kind: (b["kind"] as? String) ?? "p", text: text, sentences: (b["sentences"] as? [String]) ?? [])
        }
        return HeldPage(address: address, title: (r["title"] as? String) ?? "", dek: (r["dek"] as? String) ?? "", kind: (r["kind"] as? String) ?? "document", blocks: blocks)
    }
}

public final class HeldPages: @unchecked Sendable {
    public static let shared = HeldPages()
    public static let defaultLimit = 40
    private let lock = NSLock()
    private var pages: [String: HeldPage] = [:]
    /// Addresses held, least recently used first.
    private var recency: [String] = []
    private let limit: Int
    public init(limit: Int = HeldPages.defaultLimit) { self.limit = max(1, limit) }

    // The page of an account that has since gone is dropped by AccountWipe's registration (sign-out). A hand-over still in flight across the change is not refused here: that boundary arrives with the wipe's
    // own generation in the account change (docs/problems/unclean-sign-out-leaves-the-old-accounts-data-for-the-next-2026-11-21.md, "HeldPages needs the wipe's generation as its boundary").
    public func hold(_ p: HeldPage) {
        lock.lock(); defer { lock.unlock() }
        pages[p.address] = p
        recency.removeAll { $0 == p.address }; recency.append(p.address)
        while pages.count > limit, let oldest = recency.first, oldest != p.address { recency.removeFirst(); pages[oldest] = nil }
    }
    public func page(_ address: String) -> HeldPage? {
        lock.lock(); defer { lock.unlock() }
        guard let p = pages[address] else { return nil }
        recency.removeAll { $0 == address }; recency.append(address)
        return p
    }
    public func all() -> [HeldPage] { lock.lock(); defer { lock.unlock() }; return Array(pages.values) }
    public var count: Int { lock.lock(); defer { lock.unlock() }; return pages.count }
    /// Sign-out: every page held (the previous account's).
    public func wipe() { lock.lock(); pages = [:]; recency = []; lock.unlock() }
}

/// `pub:` and `texture:` as pages for the voice: the lines are the plain text of the page the shared reader handed over; before it has, the page is "loading".
public struct HeldPageSource: PageSource {
    public let held: HeldPages
    public init(held: HeldPages = .shared) { self.held = held }
    public func handles(_ address: String) -> Bool { address.hasPrefix("pub:") || address.hasPrefix("texture:") }
    public func load(_ address: String) async throws -> PageModel {
        guard let p = held.page(address) else { return PageModel(address: address, kind: "reader", title: PageAddress.defaultTitle(address), status: "loading") }
        return PageModel(address: address, kind: "reader", title: p.title.isEmpty ? PageAddress.defaultTitle(address) : p.title, status: "ready", lines: p.lines)
    }
}
