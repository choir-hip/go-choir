import Foundation

// "Play that", "open the second one", "queue the third": what the model named is what the phone retrieved, so these are resolved on the phone
// against the items of the last answer, in the order the model was shown them (newest first, Passage 1..n). Nothing is sent to the server.
//
// WAYS THIS CAN FAIL (tests in RetrievedCommandTests):
//  1. No answer retrieved, or it is older than `freshFor` (10 minutes): not a command, the words go to the desk as usual.
//  2. An ordinal beyond the list ("the fifth one" of three): not a command (never the wrong item); the desk answers.
//  3. A question that merely contains the words ("who will open the second round") is not a command: the WHOLE utterance must be one.
//  4. "that" with several items: the item the answer spoke from (the best); "the latest" is the newest.
//  5. A passage of a book opens at its block; a story plays or opens by its route; an item that cannot play (a file name) can still open.
//  6. Two passages of one book: each keeps its own place (address `doc:<id>@<block>`).

public enum RetrievedAction: String, Sendable, Equatable { case play, open, queue }

public final class RetrievedRefs: @unchecked Sendable {
    public static let shared = RetrievedRefs()
    public static let freshFor: TimeInterval = 600
    private let lock = NSLock()
    private var items: [RetrievalItem] = [], best: String?, at = Date.distantPast
    public init() {}
    /// What the last answer was built from: `items` as the model saw them, `best` the id it spoke from.
    public func set(_ items: [RetrievalItem], best: String?, now: Date = Date()) { lock.lock(); self.items = items; self.best = best; at = now; lock.unlock() }
    public func current(now: Date = Date()) -> (items: [RetrievalItem], best: String?) {
        lock.lock(); defer { lock.unlock() }
        return now.timeIntervalSince(at) <= Self.freshFor ? (items, best) : ([], nil)
    }
    public func clear() { lock.lock(); items = []; best = nil; lock.unlock() }
}

public enum RetrievedCommand {
    static let ordinals: [String: Int] = ["first": 1, "1st": 1, "second": 2, "2nd": 2, "third": 3, "3rd": 3, "fourth": 4, "4th": 4, "fifth": 5, "5th": 5]
    static let numbers: [String: Int] = ["one": 1, "1": 1, "two": 2, "2": 2, "three": 3, "3": 3, "four": 4, "4": 4, "five": 5, "5": 5]
    static let nouns: Set<String> = ["one", "story", "item", "article", "result", "episode", "book", "passage", "chapter", "piece", "thing"]
    static let filler: Set<String> = ["the", "that", "this", "it", "number", "up"]
    static let latestWords: Set<String> = ["latest", "newest", "newer", "recent", "most"]

    public static func match(_ text: String, items: [RetrievalItem], best: String? = nil) -> (RetrievedAction, RetrievalItem)? {
        guard !items.isEmpty else { return nil }
        let w = SpokenCommand.normalise(text).split(separator: " ").map(String.init)
        guard let first = w.first else { return nil }
        var action: RetrievedAction, ref: [String]
        let tail = Array(w.suffix(3))
        if first == "add", tail == ["to", "the", "queue"] || tail == ["to", "my", "queue"] { action = .queue; ref = Array(w.dropFirst().dropLast(3)) }
        else if first == "add", w.suffix(2) == ["to", "queue"] { action = .queue; ref = Array(w.dropFirst().dropLast(2)) }
        else if first == "queue" { action = .queue; ref = Array(w.dropFirst()) }
        else if first == "play" || first == "read" { action = .play; ref = Array(w.dropFirst()) }
        else if first == "open" || first == "show" { action = .open; ref = Array(w.dropFirst()) }
        else { return nil }
        // What is left must be exactly a reference: filler words, one ordinal or "number N" or "latest", nouns.
        var index: Int?, latest = false, pointing = false, prev = ""
        for t in ref {
            defer { prev = t }
            if let n = ordinals[t] { index = n }
            else if prev == "number", let n = numbers[t] { index = n }
            else if latestWords.contains(t) { latest = true }
            else if ["that", "this", "it"].contains(t) { pointing = true }
            else if filler.contains(t) || nouns.contains(t) { continue }
            else { return nil }
        }
        if latest { return (action, items[0]) }
        if let i = index { return i >= 1 && i <= items.count ? (action, items[i - 1]) : nil }
        guard pointing else { return nil }
        if let b = best, let it = items.first(where: { $0.id == b }) { return (action, it) }
        return items.count == 1 ? (action, items[0]) : nil
    }

    /// What the run sheet plays for an item (nil: cannot be played, only opened).
    public static func entry(for i: RetrievalItem) -> RunSheetEntry? {
        if i.address.hasPrefix("pub:") { return RunSheetEntry.paperStory(route: String(i.address.dropFirst(4)), title: i.title) }
        if let d = ShelfAddress.parse(i.address) { return RunSheetEntry.book(id: d.id, title: i.title, at: d.block ?? 0) }
        if i.address.hasPrefix("mail:") || i.address.hasPrefix("texture:") { return RunSheetEntry.readAloud(address: i.address, title: i.title) }
        return nil
    }

    public static func line(_ a: RetrievedAction, _ i: RetrievalItem) -> String {
        let t = String(i.title.prefix(60))
        switch a { case .play: return "Okay, playing \(t)."; case .open: return "Okay, opening \(t)."; case .queue: return "Okay, \(t) is in the queue." }
    }
}
