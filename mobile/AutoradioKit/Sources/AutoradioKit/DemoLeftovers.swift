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

    /// Native because the platform forces it: this removes on-disk state the native code wrote (the stores' files).
    /// Needs a positive demo trace: pages and signals only for the fixture addresses
    /// demo produced; onboarding, topics and voice setup are never touched. Once per device, whatever the flag was; marked after the removals.
    public static func clearLeftovers(support: URL, allowed: Bool = allowedHere, defaults: UserDefaults = .standard) {
        guard !allowed, !defaults.bool(forKey: clearedKey) else { return }
        var ok = true
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

    // MARK: The wider pass (version 2). Its own marker: phones that ran the first pass have ITS marker set. Same discipline: a positive trace only (an id, a content or a
    // shape only demo mode produced), never a real item, an unreadable file is never rewritten, the marker comes after the removals. Shared files: demo-leftovers.ts.
    static let clearedMoreKey = "AutoradioDemoLeftoversCleared2"
    static let textureIDs: Set<String> = ["demo-notes", "demo-draft", "demo-edition", "demo-letter", "demo-notebook"]
    static let mailboxIDs: Set<String> = ["demo-empty", "demo-loading", "demo-offline", "demo-error"]
    static let messageIDs: Set<String> = ["m1", "m2", "m3", "m4", "m5", "m6", "q1", "s1", "demo-draft-1", "demo-draft-new"]
    /// The demo theme Texture's theme object (what `installFromTexture` wrote as themes/texture-ember.json): a file of another content is a real theme.
    static let demoThemeJSON = ##"{"schema_version":3,"id":"ember","name":"Ember","schemes":["dark"],"colors":{"paper":"#14100E","paper2":"#211A16","fg":"#F4EBE3","muted":"#C2B1A2","accent":"#FF7A45","accent2":"#FFB48F","success":"#7ED6A0","danger":"#FF8A80","onAccent":"#14100E","rule":"rgba(244,235,227,.18)"},"type":{"read":"ui-serif","ui":"system-ui","mono":"ui-monospace","scale":"compact"},"radii":{"card":"10px","control":"8px","pill":"999px","rules":"0px"},"elevation":{"1":{"shadow":"0 1px 0 rgba(255,122,69,.10), 0 8px 20px rgba(0,0,0,.45)","edge":"inset 0 1px 0 rgba(255,235,220,.10)","press":0.98},"2":{"material":"glass","shadow":"0 10px 28px rgba(0,0,0,.55)","edge":"inset 0 1px 0 rgba(255,235,220,.14)"},"3":{"material":"glass","shadow":"0 24px 60px rgba(0,0,0,.65)","behind":{"scale":0.95,"dim":0.18}}},"motion":{"fast":"110ms ease","enter":"300ms ease-out","sheet":"240ms ease-out","spring":{"stiffness":260,"damping":24,"mass":1},"stagger":"35ms","reduced":{"enter":"0ms","spring":"none"}},"mark":{"drift":"75s","tumble":["3.7s","6.1s"],"flip":{"stiffness":320,"damping":21},"listenScale":1.18,"reduced":"static"}}"##

    /// Any address only demo mode could have opened (by the id after the scheme; `/`, `#anchor` and `@revision` are not part of it).
    static func isDemoTrace(_ a: String) -> Bool {
        guard let i = a.firstIndex(of: ":") else { return false }
        let rest = String(a[a.index(after: i)...])
        let id = String(rest.drop { $0 == "/" }.split(omittingEmptySubsequences: false, whereSeparator: { $0 == "#" || $0 == "@" }).first ?? "")
        switch a[..<i] {
        case "pub": return storyIDs.contains(id)
        case "texture": return textureIDs.contains(id)
        case "mailbox": return mailboxIDs.contains(id)
        case "mail": return messageIDs.contains(id)
        case "doc": return id == sampleBookID
        case "file": let p = rest.split(separator: "/"); return p.count >= 2 && p[0] == "Mail" && messageIDs.contains(String(p[1]))
        default: return false
        }
    }

    public static func clearMoreLeftovers(support: URL, allowed: Bool = allowedHere, defaults: UserDefaults = .standard) {
        guard !allowed, !defaults.bool(forKey: clearedMoreKey) else { return }
        var ok = true
        func json(_ name: String) -> (URL, [String: Any])? {      // unreadable: never rewritten
            let f = support.appendingPathComponent(name)
            guard let d = try? Data(contentsOf: f), let o = try? JSONSerialization.jsonObject(with: d) as? [String: Any] else { return nil }
            return (f, o)
        }
        func write(_ o: [String: Any], _ f: URL) { ok = ((try? JSONSerialization.data(withJSONObject: o, options: [.sortedKeys]).write(to: f, options: .atomic)) != nil) && ok }
        if case (let f, var o)? = json("open-pages.json"), let pages = o["pages"] as? [[String: Any]], let recency = o["recency"] as? [String] {
            let p = pages.filter { !isDemoTrace($0["address"] as? String ?? "") }, r = recency.filter { !isDemoTrace($0) }
            if p.count != pages.count || r.count != recency.count { o["pages"] = p; o["recency"] = r; write(o, f) }
        }
        let log = support.appendingPathComponent("signals.jsonl")
        if let all = (try? String(contentsOf: log, encoding: .utf8))?.split(separator: "\n") {
            let keep = all.filter { (try? JSONDecoder().decode(SignalEvent.self, from: Data($0.utf8))).map { !isDemoTrace($0.subject) } ?? true }
            if keep.count != all.count { ok = (try? keep.map { $0 + "\n" }.joined().write(to: log, atomically: true, encoding: .utf8)) != nil && ok }
        }
        if case (let f, var o)? = json("what-i-told-the-paper.json"), let acts = o["acts"] as? [[String: Any]] {          // acts on demo stories; the gesture meanings stay
            let keep = acts.filter { !storyIDs.contains(String(($0["route"] as? String ?? "").drop { $0 == "/" })) }
            if keep.count != acts.count { o["acts"] = keep; write(o, f) }
        }
        let themeFile = support.appendingPathComponent("themes/texture-ember.json")          // the demo theme, and the choice of it only when that file was demo's
        if let d = try? Data(contentsOf: themeFile), let o = try? JSONSerialization.jsonObject(with: d) as? NSDictionary,
           let demo = try? JSONSerialization.jsonObject(with: Data(demoThemeJSON.utf8)) as? NSDictionary, o.isEqual(demo) {
            if (try? FileManager.default.removeItem(at: themeFile)) != nil { if defaults.string(forKey: "AutoradioThemeID") == "ember" { defaults.removeObject(forKey: "AutoradioThemeID") } } else { ok = false }
        }
        for k in ["AutoradioDemo", "AutoradioDemoSheet", "AutoradioOpenOnboarding"] { defaults.removeObject(forKey: k) }          // launch flags only demo mode read
        if ok { defaults.set(true, forKey: clearedMoreKey) }
    }
}
