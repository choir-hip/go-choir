import Foundation

// What the voice shares with the shared reader: what the bar's prompt is aimed at (the voice turn reads it). The reader PAGE is shared code (AutoradioLab/reader.ts), and so is the block and sentence the
// radio is reading (radio-follow.ts); the document fetch, the edition lookup and the address splitter that lived beside these are gone with the Swift Texture editor and notebook.

/// Nothing is kept here: the block and sentence the radio is reading are shared code's now (AutoradioLab/radio-follow.ts, radio-bridge.ts `readerFollow`), made from the page the shared reader holds.
/// This stays only because AccountWipe.swift still names it in the "reader" wipe; delete it with that one call.
public final class ReaderFollow: @unchecked Sendable {
    public static let shared = ReaderFollow()
    public func clear() {}
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
    public var onChange: (@Sendable () -> Void)?
    public var info: ReaderAimInfo { lock.lock(); defer { lock.unlock() }; return _info }
    public func set(_ i: ReaderAimInfo) { lock.lock(); _info = i; lock.unlock(); onChange?() }
    public func clear() { set(ReaderAimInfo()) }
}
