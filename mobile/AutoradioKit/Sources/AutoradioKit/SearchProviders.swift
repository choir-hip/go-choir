import Foundation

// One place for the universal search to ask the other apps of the app (podcasts, the reader's documents). A provider is a name and a
// synchronous function from the query to hits; it answers from what it has now and may start its own work, calling `onChange` when
// more is ready (the launcher is then re-read). Registering a name again replaces it, so a second lane's line never doubles one.
public enum SearchProviders {
    private static let lock = NSLock()
    nonisolated(unsafe) private static var providers: [(name: String, search: @Sendable (String) -> [SearchHit])] = []

    public static func register(name: String, search: @escaping @Sendable (String) -> [SearchHit]) {
        lock.lock(); defer { lock.unlock() }
        providers.removeAll { $0.name == name }
        providers.append((name, search))
    }
    public static func hits(for query: String) -> [SearchHit] {
        lock.lock(); let list = providers; lock.unlock()
        var seen = Set<String>(), out: [SearchHit] = []
        for p in list { for h in p.search(query) where seen.insert(h.address).inserted { out.append(h) } }
        return out
    }
    public static func unregister(name: String) { lock.lock(); providers.removeAll { $0.name == name }; lock.unlock() }
}
