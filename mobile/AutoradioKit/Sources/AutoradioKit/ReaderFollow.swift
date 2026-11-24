import Foundation

// What the voice shares with the shared reader: the block and sentence the radio is reading (set by the bar's voice turn, matched against the page the shared reader handed over) and what the
// bar's prompt is aimed at (the voice turn reads it). The reader PAGE is shared code (AutoradioLab/reader.ts); the document fetch, the edition lookup and the address splitter that lived beside these
// are gone with the Swift Texture editor and notebook.

public struct ReaderFollowInfo: Codable, Sendable, Equatable {
    public var address: String
    public var blockId: String
    public var sentence: Int
    public init(address: String = "", blockId: String = "", sentence: Int = -1) { self.address = address; self.blockId = blockId; self.sentence = sentence }
}

/// Which block and sentence the radio is reading. The bar calls `set(address:spoken:)` with the text it is saying; the match is by text, so the radio's own chunking does not matter, against the page
/// the shared reader handed over (HeldPages), so the block ids are the ones the page draws. (Moves to shared code with the bar's read-along.)
public final class ReaderFollow: @unchecked Sendable {
    public static let shared = ReaderFollow()
    private let lock = NSLock()
    private var _info = ReaderFollowInfo()
    public var onChange: (@Sendable () -> Void)?
    public init() {}
    public var info: ReaderFollowInfo { lock.lock(); defer { lock.unlock() }; return _info }

    public func clear() { lock.lock(); _info = ReaderFollowInfo(); lock.unlock(); onChange?() }

    @discardableResult
    public func set(address: String, spoken: String, held: HeldPages = .shared) -> ReaderFollowInfo {
        let found = Self.locate(spoken: spoken, in: held.page(address))
        lock.lock(); _info = found.map { ReaderFollowInfo(address: address, blockId: $0.block, sentence: $0.sentence) } ?? ReaderFollowInfo(); let v = _info; lock.unlock()
        onChange?()
        return v
    }

    /// The block whose text contains the spoken text, and the sentence that contains its start. Nil when nothing matches.
    public static func locate(spoken: String, in doc: HeldPage?) -> (block: String, sentence: Int)? {
        guard let doc, !spoken.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return nil }
        let key = HeldPages.normalised(String(spoken.trimmingCharacters(in: .whitespacesAndNewlines).prefix(60)))
        if key.count < 6 { return nil }
        for b in doc.blocks where !b.text.isEmpty {
            let nb = HeldPages.normalised(b.text)
            guard nb.contains(key) else { continue }
            var best = 0
            for (i, s) in b.sentences.enumerated() where HeldPages.normalised(s).contains(String(key.prefix(24))) { best = i; break }
            return (b.id, best)
        }
        return nil
    }
}

public struct ReaderAimInfo: Codable, Sendable, Equatable {
    public var address: String
    public var blockId: String
    public var label: String
    public init(address: String = "", blockId: String = "", label: String = "") { self.address = address; self.blockId = blockId; self.label = label }
}

/// What the bar's prompt is aimed at: a document, or one block of it. Cleared when the listener leaves the document.
public final class ReaderAim: @unchecked Sendable {
    public static let shared = ReaderAim()
    private let lock = NSLock()
    private var _info = ReaderAimInfo()
    public init() {}
    public var info: ReaderAimInfo { lock.lock(); defer { lock.unlock() }; return _info }
    public func set(_ i: ReaderAimInfo) { lock.lock(); _info = i; lock.unlock() }
    public func clear() { set(ReaderAimInfo()) }
}
