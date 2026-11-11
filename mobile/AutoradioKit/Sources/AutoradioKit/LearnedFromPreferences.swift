import Foundation

// "What the paper has learned" (Settings, lane 7) reads the scored acts that lane 4 keeps in `Preferences`
// (what-i-told-the-paper.json): the one list the front page's shading is derived from, so the screen and the effect cannot disagree.
// (Lane 7 first derived the list from raw gesture events; that left two sources of "what was learned".)
//
// Ways this can fail (written before the code):
//  1. No acts: the list is empty (the screen then says so), nothing is invented.
//  2. An act with no entities: the line shows the title alone, no empty "()".
//  3. Reset: the acts are removed from Preferences (the shading goes with them) and the demo seed stays gone for the rest of the run.
//  4. Order: newest first, the demo seed (a fixture, demo mode only) merged by time; a seed older than a reset is not shown again.
//  5. Called from any thread: Preferences holds its own lock; the seed flag has one here.
public final class PreferencesLearned: LearnedProvider, @unchecked Sendable {
    private let prefs: @Sendable () -> Preferences?
    private let demoSeed: Bool
    private let lock = NSLock()
    private var seedCleared = false

    public init(prefs: @escaping @Sendable () -> Preferences?, demoSeed: Bool) { self.prefs = prefs; self.demoSeed = demoSeed }

    public static func lines(from acts: [ToldAct], seed: [LearnedLine]) -> [LearnedLine] {
        var out = seed
        for a in acts {
            let who = a.entities.isEmpty ? "" : " (\(a.entities.joined(separator: ", ")))"
            out.append(LearnedLine(at: a.at, kind: a.kind == "more" ? "more" : "less",
                                   text: "\(a.kind == "more" ? "More like this" : "Less of this"): \(a.title)\(who)"))
        }
        return out.sorted { $0.at > $1.at }
    }

    private func seedLines() -> [LearnedLine] {
        lock.lock(); let cleared = seedCleared; lock.unlock()
        guard demoSeed, !cleared, let d = Demo.fixture("learned"), let o = try? JSONSerialization.jsonObject(with: d) as? [String: Any] else { return [] }
        return (o["lines"] as? [[String: Any]] ?? []).compactMap { l in
            guard let t = l["text"] as? String else { return nil }
            return LearnedLine(at: l["at"] as? Double ?? 0, kind: l["kind"] as? String ?? "more", text: t)
        }
    }

    public func lines() async -> [LearnedLine] { Self.lines(from: prefs()?.all ?? [], seed: seedLines()) }

    public func reset() async {
        lock.lock(); seedCleared = true; lock.unlock()
        _ = prefs()?.reset()
    }
}
