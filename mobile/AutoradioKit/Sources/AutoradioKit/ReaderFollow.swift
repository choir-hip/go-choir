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

/// The sentence the radio is saying, by address, so the reader lights it (the app module's `radioView` feeds it). Only a CHANGE is passed on; when nothing plays the follow is cleared. A follow that was
/// cleared (the sign-out wipe) is set again by the very next feed even when the address and the sentence are the same as before.
public final class ReaderFeed: @unchecked Sendable {
    public static let shared = ReaderFeed()
    private let lock = NSLock()
    private var last = ""
    public init() {}
    public func feed(_ r: RadioInfo, address: String, follow: ReaderFollow = .shared) {
        let spoken = r.playing && !address.isEmpty && r.sentences.indices.contains(r.sentenceIndex) ? r.sentences[r.sentenceIndex] : ""
        let key = spoken.isEmpty ? "" : "\(address)|\(spoken)"
        lock.lock(); let changed = key != last || (!key.isEmpty && follow.info.address.isEmpty); last = key; lock.unlock()
        guard changed else { return }
        if key.isEmpty { follow.clear() } else { follow.set(address: address, spoken: spoken) }
    }
}
