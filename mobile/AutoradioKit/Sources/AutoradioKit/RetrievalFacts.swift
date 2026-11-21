import Foundation

// The voice turn's retrieval, as FACTS. The index, the ranking and the vectors are shared code (AutoradioLab/retrieval-run.ts, search.ts). The
// turn engine is native (the microphone and the speech are) and needs the phone's passages for the words it just heard, so it asks through here:
// it raises ONE ask (the source `retrievalAsk`, interface/retrieval.json), shared code answers it through the native op `retrievalAnswer`, and
// the turn waits for that answer with a deadline. Exact has no call from native into shared code (docs/reference/exact/native-modules.md,
// "Out (Swift to the UI)"); asking a source again is the way back. This file decides nothing about what is relevant.
//
// WAYS THIS CAN FAIL (tests in RetrievalFactsTests):
//  1. Shared code never answers (the app is not showing, the source is not read): the deadline passes and the turn goes on with no context,
//     which the holder says plainly; it never waits for ever.
//  2. An answer that is not the shape of the context: no context (the turn goes on), never a crash.
//  3. An answer for an ask that is no longer waiting (the deadline passed, a newer ask came): dropped.
//  4. Two asks at once (a second question in quick succession): each has its own id and its own answer; the first is dropped when the second is
//     raised, because the turn that asked it was cancelled.

public struct RetrievalItem: Codable, Sendable, Equatable {
    public var id: String
    public var address: String
    public var kind: String
    public var title: String
    public var source: String
    public var date: Date?
    public var place: String
    public var score: Double
    public var text: String
}

public struct RetrievalRecord: Codable, Sendable, Equatable {
    public struct Hit: Codable, Sendable, Equatable { public var id: String, address: String, kind: String, score: Double, date: Date?, chars: Int }
    public var at: Date
    public var query: String
    public var onAir: String
    public var terms: [String]
    public var usedOnAir: Bool
    public var considered: Int
    public var indexed: Int
    public var hits: [Hit]
    public var budgetChars: Int
    public var usedChars: Int
    public var sources: [String]?
    public var nothing: Bool { hits.isEmpty }
}

public struct RetrievalContext: Sendable {
    /// Ready for the model's prompt (header, one block per item, or the nothing-found sentence).
    public var text: String
    public var items: [RetrievalItem]
    public var record: RetrievalRecord
    /// "On air now: X. Up next: A; B." for both desks.
    public var brief = ""
    public var isEmpty: Bool { items.isEmpty }
    public init(text: String, items: [RetrievalItem], record: RetrievalRecord, brief: String = "") { self.text = text; self.items = items; self.record = record; self.brief = brief }

    /// The wire answer of shared code: plain JSON, dates in milliseconds.
    struct Wire: Decodable { var text: String; var items: [RetrievalItem]; var record: RetrievalRecord; var brief: String? }
    public static func decode(_ json: [String: Any]) -> RetrievalContext? {
        guard let data = try? JSONSerialization.data(withJSONObject: json) else { return nil }
        let d = JSONDecoder(); d.dateDecodingStrategy = .millisecondsSince1970
        guard let w = try? d.decode(Wire.self, from: data) else { return nil }
        return RetrievalContext(text: w.text, items: w.items, record: w.record, brief: w.brief ?? "")
    }
}

public struct RetrievalAsk: Sendable, Equatable {
    public var id: String, words: String, pageTitle: String, pageAddress: String
    public var now: Double          // epoch milliseconds
    public var facts: String        // JSON: what only native holds (run sheet entries on air and ahead, open pages, edition topics, the page's text)
}

public final class RetrievalBridge: @unchecked Sendable {
    public static let shared = RetrievalBridge()
    /// How long the turn waits for shared code. A deadline, not a limit on calls: past it the turn goes on without passages.
    public var deadline: Double = 2.5
    /// Set by the app module: the facts only native holds, as JSON, for the page in front; and the raise (asks the source again).
    public var facts: @Sendable (_ pageAddress: String) async -> String = { _ in "{}" }
    public var raise: @Sendable () -> Void = {}
    private let lock = NSLock()
    private var pending: RetrievalAsk?
    private var waiting: [String: CheckedContinuation<RetrievalContext?, Never>] = [:]
    private var last: RetrievalRecord?
    private var counter = 0
    public init() {}

    /// What the last turn was shown, for the kept turn's context.json.
    public var lastRecord: RetrievalRecord? { lock.lock(); defer { lock.unlock() }; return last }
    public func clear() { lock.lock(); last = nil; pending = nil; lock.unlock() }
    /// The source `retrievalAsk` reads this.
    public var current: RetrievalAsk? { lock.lock(); defer { lock.unlock() }; return pending }

    /// Ask shared code for the passages of this question. nil: it did not answer in time, or answered something that is not a context.
    public func ask(words: String, pageTitle: String, pageAddress: String, now: Date = Date()) async -> RetrievalContext? {
        lock.lock(); last = nil; lock.unlock()      // this turn's record only: a turn that times out never inherits the last turn's
        let factsJSON = await facts(pageAddress)
        let a: RetrievalAsk = {
            lock.lock(); defer { lock.unlock() }
            counter += 1
            return RetrievalAsk(id: "ask-\(counter)-\(UUID().uuidString.prefix(8))", words: words, pageTitle: pageTitle, pageAddress: pageAddress, now: (now.timeIntervalSince1970 * 1000).rounded(), facts: factsJSON)
        }()
        let wait = deadline, t0 = Date()
        let c: RetrievalContext? = await withCheckedContinuation { (k: CheckedContinuation<RetrievalContext?, Never>) in
            lock.lock()
            if let old = pending { waiting.removeValue(forKey: old.id)?.resume(returning: nil) }      // the turn that asked it was cancelled
            pending = a; waiting[a.id] = k
            lock.unlock()
            raise()
            Task.detached { [self] in
                try? await Task.sleep(nanoseconds: UInt64(wait * 1_000_000_000))
                finish(a.id, nil)
            }
        }
        // Counts only, never content: the proof reads this line to tell an answered ask from one that timed out.
        fputs("AUTORADIO retrieval ask \(c == nil ? "no-answer" : "answered") items=\(c?.items.count ?? 0) ms=\(Int(Date().timeIntervalSince(t0) * 1000))\n", stderr)
        return c
    }

    /// The op `retrievalAnswer`: shared code's answer to an ask.
    public func answer(id: String, context: [String: Any]) {
        finish(id, RetrievalContext.decode(context))
    }

    private func finish(_ id: String, _ c: RetrievalContext?) {
        lock.lock()
        let k = waiting.removeValue(forKey: id)
        if k != nil, pending?.id == id { pending = nil }
        if k != nil, let c { last = c.record }
        lock.unlock()
        k?.resume(returning: c)
    }
}
