import Foundation

// Settings that are data (lane 7): the talk mode and the gesture meanings are a catalog of choices (id, label, options),
// kept in UserDefaults and shown by one component, so a new choice is a catalog line and not a new screen. Other lanes read
// the value, never the screen:
//   AppSettings.shared.value("talkMode")     button | word | open        (lane 1/8: the addressing in force)
//   AppSettings.shared.value("swipeLeft")    close | closeLess | lessInStack   (lane 4: what a left swipe on a story means)
//   AppSettings.shared.value("swipeRight")   more | nothing              (lane 4)
// "What the paper has learned" reads a LearnedProvider; the default one is derived from the gesture events in `Signals`
// (lane 4 owns their meaning; its Preferences.swift can supply its own provider).
//
// Ways it can fail (written before the code; tests in SettingsDataTests):
//  S1 A stored value that is no longer an option (the catalog changed) shows nothing selected: value() falls back to the
//     default, and set() refuses an id that is not an option.
//  S2 Talk mode applies at the wrong time or never: it is applied when set and once at launch (`apply`), mapped to the
//     addressing stage ids the engine has: button -> "button", word -> "hybrid" (the engine has no wake-word stage; the
//     hybrid window is the nearest), open -> "open".
//  L1 The learned list shows acts from before a Reset: the list is cut at the newest "learned-reset" event.
//  L2 Reset deletes the gesture events: it only records a marker; the lab records stay (they are lab records, not the list).
//  L3 A subject that is a private address is shown in full: lines use the detail text lane 4 wrote, else the bare address.

public struct SettingOptionRow: Codable, Sendable, Equatable {
    public var id = "", label = "", detail = "", selected = false
    public init(id: String, label: String, detail: String = "", selected: Bool = false) { self.id = id; self.label = label; self.detail = detail; self.selected = selected }
}
public struct SettingRow: Codable, Sendable, Equatable {
    public var id = "", label = "", help = "", value = "", valueLabel = ""
    public var choices: [SettingOptionRow] = []
    public init() {}
}

public struct SettingDef: Sendable {
    public var id: String, group: String, label: String, help: String, defaultValue: String
    public var options: [(id: String, label: String, detail: String)]
}

public enum SettingsCatalog {
    public static let all: [SettingDef] = [
        SettingDef(id: "talkMode", group: "talk", label: "How you talk to the radio",
                   help: "You can always hold the voice button. This chooses whether the radio also listens on its own.",
                   defaultValue: "button",
                   options: [("button", "Hold the button", "The radio hears you only while you hold the button, or after you tap it once."),
                             ("word", "Hold, or speak after a turn", "After each turn the radio keeps listening for five minutes, so you can answer without the button."),
                             ("open", "Always listening", "Anything you say to the radio counts when it has enough words. Uses the microphone the whole time.")]),
        SettingDef(id: "swipeLeft", group: "gestures", label: "Swipe left on a story",
                   help: "A change applies to the next swipe. Closing never costs a story its place unless you choose it.",
                   defaultValue: "close",
                   options: [("close", "Close, nothing more", "Closing is neutral. It is not a vote against the story."),
                             ("closeLess", "Close, and less of this", "Every close also tells the paper you want less of it."),
                             ("lessInStack", "Less of this, only in the stack", "In the story stack a pass means less; on open pages a close is neutral.")]),
        SettingDef(id: "swipeRight", group: "gestures", label: "Swipe right on a story",
                   help: "The one cheap way to say more of this.",
                   defaultValue: "more",
                   options: [("more", "More of this", "The card leaves to the right and the paper notes you want more like it."),
                             ("nothing", "Nothing", "Swiping right does nothing.")]),
    ]
}

public final class AppSettings: @unchecked Sendable {
    public static let shared = AppSettings()
    private let defaults: UserDefaults
    public init(defaults: UserDefaults = .standard) { self.defaults = defaults }
    static func key(_ id: String) -> String { "AutoradioSetting.\(id)" }

    public func value(_ id: String) -> String {
        guard let def = SettingsCatalog.all.first(where: { $0.id == id }) else { return "" }
        let stored = defaults.string(forKey: Self.key(id)) ?? def.defaultValue
        return def.options.contains { $0.id == stored } ? stored : def.defaultValue         // S1
    }
    @discardableResult
    public func set(_ id: String, _ value: String) -> Bool {
        guard let def = SettingsCatalog.all.first(where: { $0.id == id }), def.options.contains(where: { $0.id == value }) else { return false }
        defaults.set(value, forKey: Self.key(id)); return true
    }
    public func rows(group: String) -> [SettingRow] {
        SettingsCatalog.all.filter { $0.group == group }.map { def in
            var r = SettingRow(); r.id = def.id; r.label = def.label; r.help = def.help
            let v = value(def.id); r.value = v
            r.choices = def.options.map { SettingOptionRow(id: $0.id, label: $0.label, detail: $0.detail, selected: $0.id == v) }
            r.valueLabel = def.options.first { $0.id == v }?.label ?? ""
            return r
        }
    }
    /// The engine's addressing stage for a talk mode (S2).
    public static func addressingStage(forTalkMode m: String) -> String {
        switch m { case "open": return "open"; case "word": return "hybrid"; default: return "button" }
    }
    public func apply(to runtime: LabRuntime) { runtime.setStage(.addressing, id: Self.addressingStage(forTalkMode: value("talkMode"))) }
}

// MARK: What the paper has learned

public struct LearnedLine: Codable, Sendable, Equatable {
    public var at: Double, kind: String, text: String
    public init(at: Double, kind: String, text: String) { self.at = at; self.kind = kind; self.text = text }
}

public protocol LearnedProvider: Sendable {
    func lines() async -> [LearnedLine]
    func reset() async
}

/// Derived from the gesture events: more-of-this and less-of-this, cut at the newest reset marker (L1, L2).
public final class SignalsLearned: LearnedProvider, @unchecked Sendable {
    private let signals: Signals
    private let demoSeed: Bool
    public init(signals: Signals, demoSeed: Bool) { self.signals = signals; self.demoSeed = demoSeed }

    public static func lines(from events: [SignalEvent], seed: [LearnedLine]) -> [LearnedLine] {
        let newestFirst = events.sorted { $0.at > $1.at }
        let resetAt = newestFirst.first { $0.kind == "learned-reset" }?.at ?? 0
        var out = seed.filter { $0.at > resetAt }
        for e in newestFirst where e.at > resetAt {
            let what = e.detail.isEmpty ? e.subject : e.detail                                  // L3
            switch e.kind {
            case "more-of-this": out.append(LearnedLine(at: e.at, kind: "more", text: "More like this: \(what)"))
            case "less-of-this": out.append(LearnedLine(at: e.at, kind: "less", text: "Less of this: \(what)"))
            default: break
            }
        }
        return out.sorted { $0.at > $1.at }
    }
    public func lines() async -> [LearnedLine] {
        var seed: [LearnedLine] = []
        if demoSeed, let d = Demo.fixture("learned"), let o = try? JSONSerialization.jsonObject(with: d) as? [String: Any] {
            seed = (o["lines"] as? [[String: Any]] ?? []).compactMap { l in
                guard let t = l["text"] as? String else { return nil }
                return LearnedLine(at: l["at"] as? Double ?? 0, kind: l["kind"] as? String ?? "more", text: t)
            }
        }
        return Self.lines(from: signals.recent(limit: 200), seed: seed)
    }
    public func reset() async { signals.record(SignalEvent(kind: "learned-reset", subject: "settings:learned", detail: "reset by the listener")) }
}

public struct LearnedRow: Codable, Sendable, Equatable { public var line: String, kind: String, ago: String }
public struct LearnedView: Codable, Sendable, Equatable {
    public var rows: [LearnedRow] = []
    public var count = 0
    public var message = ""
    public init() {}
    public init(lines: [LearnedLine], now: Date = Date()) {
        rows = lines.map { LearnedRow(line: $0.text, kind: $0.kind, ago: Self.when($0.at, now: now)) }
        count = rows.count
        message = rows.isEmpty ? "Nothing yet. Swipe right on a story you want more of, and it shows here." : ""
    }
    static func when(_ at: Double, now: Date) -> String {
        let f = DateFormatter(); f.locale = Locale(identifier: "en_US_POSIX"); f.dateFormat = "d MMM HH:mm"
        return f.string(from: Date(timeIntervalSince1970: at))
    }
}
