import Foundation

// The conversation: every question the reader asked (typed or spoken) and what came back, kept so the answer is never only a sound.
// Before this file the desk's answer was spoken and nothing else: the bar showed "Revised · Show" for six seconds, "Show" opened the
// player, and the answer text was nowhere. A failure with no spoken line left only a four-second bar message.
//
// WAYS THIS CAN FAIL (written before the code; each has a test in ConversationTests):
//  1. The answer arrives and is spoken but not kept: every desk sentence is added to the entry as it is spoken.
//  2. The turn fails with nothing spoken (no network, no sign-in, the stream died before a sentence): the entry becomes `failed` with a
//     plain sentence and `canRetry`; the engine speaks the same sentence.
//  3. The reader cancels or asks again: the open entry becomes `cancelled` and is not shown as an answer or a failure.
//  4. The desk finishes with no sentence: `failed`, never a card that says "working" forever.
//  5. System 1 says nothing was asked (a thank you): the entry is dropped from the card (`cancelled`).
//  6. Two questions in quick succession: each has its own entry; late sentences for the first never land in the second.
//  7. A first answer from saved stories and the server's answer for one question stay one entry: the first is kept and marked as such,
//     the full answer is shown as the answer.
//  8. A long session: at most `limit` entries are kept.

public struct ConversationEntry: Codable, Equatable, Sendable {
    public var id: String
    public var question: String
    /// What the phone said first, from the reader's saved stories (empty: nothing was).
    public var first = ""
    /// "From the paper's story on X" (empty when `first` is empty).
    public var firstSource = ""
    /// The server's answer, as spoken, sentence by sentence.
    public var answer = ""
    /// asking | answering | answered | failed | cancelled
    public var status = "asking"
    /// Plain words for a failure.
    public var note = ""
    public var startedAt: Date
    public init(id: String, question: String, startedAt: Date = Date()) { self.id = id; self.question = question; self.startedAt = startedAt }
}

public final class Conversation: @unchecked Sendable {
    public static let shared = Conversation()
    public static let limit = 30
    private let lock = NSLock()
    private var list: [ConversationEntry] = []
    private var current: String?
    public var onChange: (@Sendable () -> Void)?
    public init() {}

    private func change(_ body: (inout [ConversationEntry]) -> Void) {
        lock.lock(); body(&list); if list.count > Self.limit { list.removeFirst(list.count - Self.limit) }; lock.unlock()
        onChange?()
    }
    private func mutate(_ id: String, _ body: (inout ConversationEntry) -> Void) {
        change { l in if let i = l.lastIndex(where: { $0.id == id }) { body(&l[i]) } }
    }

    /// The id of the turn being answered now (what the local holder, which does not know it, writes to).
    public var currentID: String? { lock.lock(); defer { lock.unlock() }; return current }

    public func begin(id: String, question: String) {
        let q = question.trimmingCharacters(in: .whitespacesAndNewlines)
        lock.lock(); current = id; lock.unlock()
        // A question still open when another begins is over: never two cards "working".
        change { l in
            for i in l.indices where l[i].status == "asking" || l[i].status == "answering" { l[i].status = "cancelled" }
            l.append(ConversationEntry(id: id, question: q))
        }
    }

    public func setFirst(id: String? = nil, text: String, source: String) {
        guard let id = id ?? currentID else { return }
        mutate(id) { e in
            guard e.status == "asking" || e.status == "answering" else { return }
            e.first = e.first.isEmpty ? text : e.first + " " + text
            if !source.isEmpty { e.firstSource = source }
        }
    }

    public func addAnswer(id: String, sentence: String) {
        mutate(id) { e in
            guard e.status == "asking" || e.status == "answering" else { return }
            e.answer = e.answer.isEmpty ? sentence : e.answer + " " + sentence
            e.status = "answering"
        }
    }

    /// The desk's stream ended. `status` is the desk's own word ("completed", "interrupted", "error").
    public func finishAnswer(id: String, status: String) {
        mutate(id) { e in
            guard e.status == "asking" || e.status == "answering" else { return }
            if !e.answer.isEmpty { e.status = "answered"; if status == "interrupted" { e.note = "The answer was cut off." } }
            else if status == "error" { e.status = "failed"; if e.note.isEmpty { e.note = "Your computer could not answer that. Try again." } }
            else { e.status = "failed"; e.note = "Your computer had nothing to say to that. Try again." }
        }
    }

    public func fail(id: String, note: String) {
        mutate(id) { e in
            guard e.status == "asking" || e.status == "answering" else { return }
            e.status = e.answer.isEmpty ? "failed" : "answered"
            e.note = note
        }
    }

    /// The reader cancelled or asked again, or System 1 said nothing was asked.
    public func cancelOpen() {
        change { l in for i in l.indices where l[i].status == "asking" || l[i].status == "answering" { l[i].status = "cancelled" } }
    }

    /// The turn's pipeline ended: anything still open is settled by what it has.
    public func settle(id: String) {
        mutate(id) { e in
            guard e.status == "asking" || e.status == "answering" else { return }
            if !e.answer.isEmpty { e.status = "answered" }
            else if !e.first.isEmpty { e.status = "answered"; e.note = "The full answer did not arrive." }
            else { e.status = "failed"; e.note = "Your computer had nothing to say to that. Try again." }
        }
    }

    public func entries() -> [ConversationEntry] { lock.lock(); defer { lock.unlock() }; return list }
    public func entry(_ id: String) -> ConversationEntry? { entries().last { $0.id == id } }
    public func clear() { lock.lock(); list = []; current = nil; lock.unlock(); onChange?() }

    /// Plain words for an error that ended a turn with nothing spoken.
    public static func plain(_ error: Error) -> String {
        ChoirApp.plain(stage: "desk", message: "\(error)")
    }
}
