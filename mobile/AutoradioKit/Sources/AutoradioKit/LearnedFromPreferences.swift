import Foundation

// "What the paper has learned" (Settings, lane 7) reads the scored acts that lane 4 keeps in `Preferences`
// (what-i-told-the-paper.json): the one list the front page's shading is derived from, so the screen and the effect cannot disagree.
// (Lane 7 first derived the list from raw gesture events; that left two sources of "what was learned".)
//
// Ways this can fail (written before the code):
//  1. No acts: the list is empty (the screen then says so), nothing is invented.
//  2. An act with no entities: the line shows the title alone, no empty "()".
//  3. Reset: the acts are removed from Preferences (the shading goes with them).
//  4. Order: newest first.
//  5. Called from any thread: Preferences holds its own lock.
public final class PreferencesLearned: LearnedProvider, @unchecked Sendable {
    private let prefs: @Sendable () -> Preferences?

    public init(prefs: @escaping @Sendable () -> Preferences?) { self.prefs = prefs }

    public static func lines(from acts: [ToldAct]) -> [LearnedLine] {
        var out: [LearnedLine] = []
        for a in acts {
            let who = a.entities.isEmpty ? "" : " (\(a.entities.joined(separator: ", ")))"
            out.append(LearnedLine(at: a.at, kind: a.kind == "more" ? "more" : "less",
                                   text: "\(a.kind == "more" ? "More like this" : "Less of this"): \(a.title)\(who)"))
        }
        return out.sorted { $0.at > $1.at }
    }

    public func lines() async -> [LearnedLine] { Self.lines(from: prefs()?.all ?? []) }

    public func reset() async {
        _ = prefs()?.reset()
    }
}
