import Foundation

// Search over the whole paper, not only what the listener has opened. The launcher's field calls the public route
// `GET /api/platform/retrieval/search?q=` (internal/proxy/platform_public.go, HandlePlatformRetrievalSearch): a substring
// count over the published stories, newest first, at most 20 results, each with a title, a route and a snippet. No sign-in.
// The server does the matching; the phone only asks, decodes and shows.
//
// Ways this can fail (written before the code, per the repo testing rule):
//  1. Empty or blank query: nothing is asked, nothing is shown, no "searching" note.
//  2. No results: the launcher says so in a plain line (it never shows a blank list under a query).
//  3. Server error, no network or a bad body: the state is `failed` and the launcher shows a plain sentence; local hits stay.
//  4. A slow answer: typing is never blocked (the ask runs in a task; the launcher re-reads when it lands).
//  5. A late answer for an older query: dropped, never shown under the newer query.
//  6. A query with `&`, `+`, `#`, spaces or non-ASCII: percent-encoded in the request, never read as part of the URL.
//  7. Typing fast: the ask waits for a short pause in typing (one ask per pause, not one per key).
//  8. A result with no route or no title: skipped (it could not be opened).
//  9. The same story twice in the answer: listed once.

public struct CorpusHit: Equatable, Sendable {
    public var route: String
    public var title: String
    public var snippet: String
    public var address: String { "pub:" + route }
}

public enum CorpusSearch {
    public static let path = "/api/platform/retrieval/search"
    /// The pause in typing before the ask goes out.
    public static let debounce = 0.35

    /// One space between words, no edges. The server trims too; this makes "nvidia " and "nvidia" the same query.
    public static func normalize(_ q: String) -> String {
        q.split(whereSeparator: { $0.isWhitespace || $0.isNewline }).joined(separator: " ")
    }

    /// The answer of the route: hits in the server's order, without a hit that has no route or title or repeats a route.
    public static func decode(_ data: Data) throws -> [CorpusHit] {
        guard let o = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any] else { throw ChoirError.decode("not a JSON object") }
        guard let list = o["results"] as? [[String: Any]] else {
            if o["results"] is NSNull || o["results"] == nil { return [] }
            throw ChoirError.decode("results is not a list")
        }
        var seen = Set<String>(), out: [CorpusHit] = []
        for r in list {
            let route = (r["route_path"] as? String) ?? "", title = oneLine((r["title"] as? String) ?? "")
            guard route.hasPrefix("/"), !title.isEmpty, seen.insert(route).inserted else { continue }
            out.append(CorpusHit(route: route, title: title, snippet: snippet((r["snippet"] as? String) ?? "", title: title)))
        }
        return out
    }

    /// A snippet is a few words of the story around the match: one line, markdown marks dropped, cut at 140 characters. When it
    /// starts with the story's own heading (the match was in the opening), the heading is not repeated under the title.
    static func snippet(_ s: String, title: String) -> String {
        var t = oneLine(s.replacingOccurrences(of: "#", with: "").replacingOccurrences(of: "*", with: ""))
        while t.hasPrefix(".") { t.removeFirst() }
        t = t.trimmingCharacters(in: .whitespaces)
        if t.lowercased().hasPrefix(title.lowercased()) { t = String(t.dropFirst(title.count)).trimmingCharacters(in: .whitespaces) }
        return t.count > 140 ? String(t.prefix(139)) + "…" : t
    }
    static func oneLine(_ s: String) -> String { s.split(whereSeparator: { $0.isWhitespace || $0.isNewline }).joined(separator: " ") }

    /// Hits as launcher rows: a story each, after `local` rows, none repeated by address.
    public static func rows(_ hits: [CorpusHit], after local: [SearchHit]) -> [SearchHit] {
        var seen = Set(local.map(\.address))
        var out = local
        for h in hits where seen.insert(h.address).inserted {
            out.append(SearchHit(address: h.address, title: h.title, kind: "story", detail: h.snippet))
        }
        return out
    }
}

/// What the launcher shows under a query.
public enum CorpusStatus: Equatable, Sendable {
    case idle                 // no query
    case searching
    case done([CorpusHit])
    case failed
}

/// The search as state: one query is current; an answer for any other query is dropped. Thread-safe. `ask` is the network call
/// (injected: tests pass a closure, the app passes the client), `onChange` tells the host to re-read the launcher.
public final class CorpusSearcher: @unchecked Sendable {
    private let lock = NSLock()
    private var current = ""
    private var state: CorpusStatus = .idle
    private var task: Task<Void, Never>?
    private let ask: @Sendable (String) async throws -> Data
    private let pause: Double
    public var onChange: (@Sendable () -> Void)?

    public init(pause: Double = CorpusSearch.debounce, ask: @escaping @Sendable (String) async throws -> Data) { self.pause = pause; self.ask = ask }

    /// The status for this query, and starts the ask when it is a new one. Safe to call on every read of the launcher.
    @discardableResult
    public func status(for raw: String) -> CorpusStatus {
        let q = CorpusSearch.normalize(raw)
        lock.lock(); defer { lock.unlock() }
        if q.isEmpty {
            task?.cancel(); task = nil; current = ""; state = .idle
            return .idle
        }
        if q == current { return state }
        task?.cancel()
        current = q; state = .searching
        task = Task { [weak self] in await self?.run(q) }
        return .searching
    }

    private func run(_ q: String) async {
        do { try await Task.sleep(nanoseconds: UInt64(pause * 1_000_000_000)) } catch { return }
        let result: CorpusStatus
        do { result = .done(try CorpusSearch.decode(try await ask(q))) }
        catch is CancellationError { return }
        catch { result = .failed }
        lock.lock()
        let fresh = (q == current) && !Task.isCancelled
        if fresh { state = result }
        lock.unlock()
        if fresh { onChange?() }          // an answer for an older query changes nothing
    }

    /// The note under the hits: empty when there is nothing to say.
    public static func note(_ s: CorpusStatus, hasHits: Bool) -> String {
        switch s {
        case .idle: return ""
        case .searching: return "Searching the paper…"
        case .failed: return "Could not search the paper right now. Check your connection and try again."
        case .done(let h): return h.isEmpty && !hasHits ? "Nothing found in the paper." : ""
        }
    }
}

public extension ChoirClient {
    /// The raw answer of the search route (public: no key is sent).
    func searchPaper(_ query: String) async throws -> Data {
        try await send("GET", CorpusSearch.path, query: ["q": query], authenticated: false, route: CorpusSearch.path).data
    }
}
