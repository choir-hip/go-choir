import Foundation

// Pieces the radio shares that used to live beside the old queue: a URL typed into the bar, and running on the main actor from any thread.

// MARK: A URL in the prompt bar

public enum BarURL {
    /// The page a submitted bar text means, or nil when it is an ordinary sentence. http and https; or one bare word with a dot, a
    /// known-looking host ("choir.news", "example.com/path"). A question ("what is choir.news?") has spaces, so it is not a URL.
    public static func detect(_ text: String) -> URL? {
        let t = text.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !t.isEmpty, t.rangeOfCharacter(from: .whitespacesAndNewlines) == nil, !t.contains("@") || t.lowercased().hasPrefix("http") else { return nil }
        let lower = t.lowercased()
        if lower.hasPrefix("http://") || lower.hasPrefix("https://") {
            guard let u = URL(string: t), let h = u.host, !h.isEmpty else { return nil }
            return u
        }
        if t.contains("://") || t.hasSuffix("?") || t.hasSuffix("!") || t.hasSuffix(",") { return nil }
        let host = t.split(separator: "/", maxSplits: 1, omittingEmptySubsequences: false).first.map(String.init) ?? t
        let hostOnly = host.split(separator: ":").first.map(String.init) ?? host
        let labels = hostOnly.split(separator: ".", omittingEmptySubsequences: false)
        guard labels.count >= 2, labels.allSatisfy({ !$0.isEmpty }) else { return nil }
        guard let tld = labels.last, tld.count >= 2, tld.allSatisfy({ $0.isLetter && $0.isLowercase }) else { return nil }
        guard labels.allSatisfy({ $0.allSatisfy { $0.isLetter || $0.isNumber || $0 == "-" } }) else { return nil }
        return URL(string: "https://" + t)
    }
}

/// Run on the main actor from any thread (the lock screen's commands arrive on whichever thread the system chooses).
func onMainThread<T>(_ f: @MainActor () -> T) -> T {
    if Thread.isMainThread { return MainActor.assumeIsolated(f) }
    return DispatchQueue.main.sync { MainActor.assumeIsolated(f) }
}
