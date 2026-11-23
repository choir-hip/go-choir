import Foundation

/// The page the embedded web view is on right now (the view reports each load). "Read aloud" reads this, not the address the view was
/// first given, so a reader who followed a link reads the page in front of them.
public final class EmbeddedPage: @unchecked Sendable {
    public static let shared = EmbeddedPage()
    private let lock = NSLock()
    private var u: URL?, t = "", opened = ""
    public func opened(_ url: String) { lock.lock(); opened = url; u = URL(string: url); t = ""; lock.unlock() }
    public func loaded(url: URL?, title: String?) { lock.lock(); if let url { u = url }; t = title ?? ""; lock.unlock() }
    /// The page to read for `ref` (the address the contract holds): the page the view reached from it, else `ref` itself.
    public func current(for ref: String) -> (url: URL, title: String)? {
        lock.lock(); defer { lock.unlock() }
        if ref == opened, let u { return (u, t) }
        guard let r = URL(string: ref) else { return nil }
        return (r, "")
    }
}
