import Foundation

// What demo mode left on a phone that was in it. Demo mode itself was removed on 2026-11-20 (docs/problems/owner-phone-stuck-in-demo-mode-2026-11-20.md);
// this is the one part that stays. Nothing here turns anything on.

public enum Demo {
    static let clearedKey = "AutoradioDemoLeftoversCleared"          // recorded once `clearLeftovers` has run on this device
    /// The sample book's shelf id (the shelf's hash of the sample's bytes).
    public static let sampleBookID = "6916a7c31cd3"
    /// The five stories demo mode served (the ids of its fixtures); a `pub:` address with one of these is demo's trace.
    static let storyIDs: Set<String> = ["demo-ev-charging", "demo-grid-storage", "demo-harbor-barrier", "demo-library-hours", "demo-river-festival"]

    /// Whether the cleanup is skipped here: the simulator and the Mac (where drives kept their own state). Compiled guard, fails closed.
    public static var allowedHere: Bool {
        #if os(macOS) || targetEnvironment(simulator)
        return true
        #else
        return false          // a phone or any platform not named: closed
        #endif
    }

    /// Native because the platform forces it: this removes on-disk state the native code wrote (the shelf, the stores' files).
    /// Needs a positive demo trace: the sample book only if the shelf lists it, pages and signals only for the fixture addresses
    /// demo produced; onboarding, topics and voice setup are never touched. Once per device, whatever the flag was; marked after the removals.
    public static func clearLeftovers(support: URL, allowed: Bool = allowedHere, defaults: UserDefaults = .standard) {
        guard !allowed, !defaults.bool(forKey: clearedKey) else { return }
        var ok = true
        let shelf = ReaderShelf(root: support.appendingPathComponent("books"))
        if shelf.doc(sampleBookID) != nil { shelf.remove(id: sampleBookID); ok = shelf.doc(sampleBookID) == nil }
        let file = support.appendingPathComponent("open-pages.json")
        if let d = try? Data(contentsOf: file), var o = try? JSONSerialization.jsonObject(with: d) as? [String: Any],
           let pages = o["pages"] as? [[String: Any]], let recency = o["recency"] as? [String] {
            let p = pages.filter { !isDemoAddress($0["address"] as? String ?? "") }, r = recency.filter { !isDemoAddress($0) }
            o["pages"] = p; o["recency"] = r
            if p.count != pages.count || r.count != recency.count { ok = (try? JSONSerialization.data(withJSONObject: o).write(to: file, options: .atomic)) != nil && ok }
        }
        let log = support.appendingPathComponent("signals.jsonl")
        if let all = (try? String(contentsOf: log, encoding: .utf8))?.split(separator: "\n") {
            let keep = all.filter { (try? JSONDecoder().decode(SignalEvent.self, from: Data($0.utf8))).map { !isDemoAddress($0.subject) } ?? true }
            if keep.count != all.count { ok = (try? keep.map { $0 + "\n" }.joined().write(to: log, atomically: true, encoding: .utf8)) != nil && ok }
        }
        if ok { defaults.set(true, forKey: clearedKey) }
    }
    /// Exactly what demo mode opens and records: `pub:` plus a fixture story id (the five stories demo mode served) (`pub:demo-x`, `pub:/demo-x`, `pub:/demo-x#b2`).
    static func isDemoAddress(_ a: String) -> Bool {
        a.hasPrefix("pub:") && storyIDs.contains(String(a.dropFirst(4).drop { $0 == "/" }.split(separator: "#", maxSplits: 1, omittingEmptySubsequences: false).first ?? ""))
    }
}
