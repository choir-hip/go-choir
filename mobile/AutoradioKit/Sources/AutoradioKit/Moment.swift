import Foundation

// Nothing that depends on the moment of speaking is ever pre-rendered (owner, 2026-11-17: "the time is out of date, like 45 minutes ago. So the
// time should be a function which gets computed actually when you open the app").
//
// The written-ahead opening (OpeningWriteAhead, OpeningCache) is written by the desk at request time, with the phone's local time in the request
// (`local_time`), so its text may greet by day part, say the clock time, "N minutes", "today", "tonight" or count what is ready. All of that is
// true only at the hour it was written. The rule:
//   (a) at write time every sentence that depends on the moment is dropped from the body (`body`), and the body starts with the constant line
//       "This is Autoradio.", so what is pre-synthesised is true whenever it is played;
//   (b) at launch one short sentence is computed from the clock at that instant (`clause`: the greeting by PhoneOpening.hello and the time) and
//       placed as the second block (`compose`): the cached first block starts the sound at once with no synthesis, and the clause is synthesised
//       while it plays (a warm voice makes one sentence in about a second; the first block is longer than that).
// The first entry kept ahead is a story: its first block is pre-rendered only when it carries no moment phrase.

public enum Moment {
    static let numbers = "\\d+|one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|several|a few|a couple"
    static let patterns: [String] = [
        "\\b(good )?(morning|afternoon|evening|night|overnight)\\b", "\\bhello\\b", "\\bhi there\\b", "\\bwelcome back\\b",
        "\\b\\d{1,2}[:.]\\d{2}\\b", "\\b\\d{1,2} ?(am|pm|a\\.m\\.|p\\.m\\.)", "o.clock", "\\b(minutes?|hours?|days?) (ago|from now|old)\\b",
        "\\b(today|tonight|yesterday|tomorrow|this week|this weekend|right now|just now|moments ago|so far|latest|breaking|currently|at the moment)\\b",
        "\\b(monday|tuesday|wednesday|thursday|friday|saturday|sunday|january|february|march|april|may \\d|june|july|august|september|october|november|december)\\b",
        "\\b(\(numbers)) (new |top |major |more )?(stor(y|ies)|things|items|headlines|segments|updates)\\b",
    ]
    static let regexes: [NSRegularExpression] = patterns.compactMap { try? NSRegularExpression(pattern: $0, options: [.caseInsensitive]) }

    /// True when the words depend on when they are spoken.
    public static func dependsOnMoment(_ s: String) -> Bool {
        let r = NSRange(s.startIndex..., in: s)
        return regexes.contains { $0.firstMatch(in: s, options: [], range: r) != nil }
    }

    static let brand = "This is Autoradio."

    /// The text with every sentence that depends on the moment dropped (paragraph by paragraph), led by the constant brand line. True whenever played.
    public static func body(_ text: String) -> String {
        var paras: [String] = []
        for p in text.components(separatedBy: "\n\n") {
            let kept = BarTracker.sentences(p).filter { !dependsOnMoment($0) && !isBrand($0) }
            if !kept.isEmpty { paras.append(kept.joined(separator: " ")) }
        }
        return ([brand] + paras).joined(separator: "\n\n")
    }
    static func isBrand(_ s: String) -> Bool { s.trimmingCharacters(in: CharacterSet(charactersIn: ". ")).lowercased() == "this is autoradio" }

    /// "10:07 PM" in the calendar's zone.
    public static func timeWords(_ now: Date, calendar: Calendar = .current) -> String {
        let c = calendar.dateComponents([.hour, .minute], from: now)
        let h = c.hour ?? 0, m = c.minute ?? 0
        return String(format: "%d:%02d %@", h % 12 == 0 ? 12 : h % 12, m, h < 12 ? "AM" : "PM")
    }

    /// The app's one day-part word: morning from 05:00, afternoon from 12:00, evening from 18:00, and plain "Hello" from midnight to 05:00.
    public static func hello(_ now: Date, calendar: Calendar = .current) -> String {
        let h = calendar.component(.hour, from: now)
        return h < 5 ? "Hello" : h < 12 ? "Good morning" : h < 18 ? "Good afternoon" : "Good evening"
    }

    /// The one fresh sentence, computed at the moment the app opens: the greeting by the app's one day-part function, and the time.
    public static func clause(now: Date, calendar: Calendar = .current) -> String {
        "\(Self.hello(now, calendar: calendar)), it\u{2019}s \(timeWords(now, calendar: calendar))."
    }

    /// The body with the fresh clause as its second block (after the constant first one): the cached first block starts at once.
    public static func compose(body: String, now: Date, calendar: Calendar = .current) -> String {
        var paras = self.body(body).components(separatedBy: "\n\n")
        paras.insert(clause(now: now, calendar: calendar), at: min(1, paras.count))
        return paras.joined(separator: "\n\n")
    }
}
