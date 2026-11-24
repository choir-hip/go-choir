import Foundation

// The page in front, as the shared reader hands it to the voice (AutoradioLab/reader.ts `hand`, the app module's `pageText` op): the native voice cannot ask the shared module what is on the page,
// and matches what the radio says against it, so the shared page hands over its blocks with their sentences.
// This holds them (at most `limit`, least recently used first, forgotten at sign-out) for the native readers: the page text a turn reads (`HeldPages.page`, `pageTextProvider`) and the follow-along
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

    /// The account generation the hand-over was read under; 0 when unstamped (refused once any generation is known).
    public static func generation(of r: [String: Any]) -> Int { (r["gen"] as? NSNumber)?.intValue ?? 0 }
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

    // The page of an account that has since gone is dropped by AccountWipe's registration (sign-out). A hand-over still in flight across the change is refused here: it carries the account generation shared code
    // read the page under, and one older than the newest known (`SharedGeneration`, the radioSet and editionMirror rule) is dropped, so the next account's voice never holds the old account's page.
    @discardableResult
    public func hold(gen: Int, _ p: HeldPage) -> Bool {
        SharedGeneration.admit(gen) {
            lock.lock(); defer { lock.unlock() }
            pages[p.address] = p
            recency.removeAll { $0 == p.address }; recency.append(p.address)
            while pages.count > limit, let oldest = recency.first, oldest != p.address { recency.removeFirst(); pages[oldest] = nil }
        }
    }
    public func page(_ address: String) -> HeldPage? {
        lock.lock(); defer { lock.unlock() }
        guard let p = pages[address] else { return nil }
        recency.removeAll { $0 == address }; recency.append(address)
        return p
    }
    public func all() -> [HeldPage] { lock.lock(); defer { lock.unlock() }; return Array(pages.values) }
    public var count: Int { lock.lock(); defer { lock.unlock() }; return pages.count }
    /// Lower-cased, curly quotes and dashes straightened, runs of white space collapsed: what the follow-along compares spoken words and the page's text through.
    static func normalised(_ s: String) -> String {
        var out = "", lastSpace = true
        for ch in s {
            var c = ch
            switch c { case "\u{2018}", "\u{2019}": c = "'"; case "\u{201C}", "\u{201D}": c = "\""; case "\u{2013}", "\u{2014}": c = "-"; default: break }
            if c.isWhitespace || c == "\u{00A0}" { if !lastSpace { out.append(" "); lastSpace = true } } else { out += String(c).lowercased(); lastSpace = false }
        }
        while out.hasSuffix(" ") { out.removeLast() }
        return out
    }
    /// Sign-out: every page held (the previous account's).
    public func wipe() { lock.lock(); pages = [:]; recency = []; lock.unlock() }
}
