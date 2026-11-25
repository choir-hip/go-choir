import Foundation

// The native side of the edition picks. The topics themselves (tree, picks, saved file, spoken edits) are shared Exact code (AutoradioLab/edition-picks.ts).
// The edition's writer and its deadline are shared code (AutoradioLab/edition-writer.ts); what stays here is the voice's side: the words said at the setup screen.

// MARK: The native side of the edition picks

// Nothing here stores a pick (docs/reference/AUTORADIO-EDITION-PICKS.md). What native code needs, and why:
//  - EditionTalk: the voice hears words at the setup screen; Exact has no call from native into shared code, so they wait in a mailbox that shared code takes on the screen's timer and answers.
// The old build's file (Application Support/Autoradio/edition-topics.json) is NEVER READ and NOT deleted here: it says nothing about whose it is, so an upgraded phone starts with no picks; only the account wipe removes it.

public final class EditionTalk: @unchecked Sendable {
    public static let shared = EditionTalk()
    public typealias Answer = (line: String, finish: Bool)
    private let lock = NSLock()
    private var pending: [(id: String, words: String)] = []
    private var waiting: [String: CheckedContinuation<Answer?, Never>] = [:]
    public var onFinish: (@Sendable () -> Void)?

    /// The line to say back (nil when shared code did not answer in time or had nothing to say).
    public func say(_ words: String, deadline: Double = 6) async -> Answer? {
        let id = UUID().uuidString
        return await withCheckedContinuation { (c: CheckedContinuation<Answer?, Never>) in
            lock.lock(); pending.append((id, words)); waiting[id] = c; lock.unlock()
            Task { [weak self] in
                try? await Task.sleep(nanoseconds: UInt64(max(deadline, 0) * 1_000_000_000))
                self?.resolve(id, nil)
            }
        }
    }
    private func resolve(_ id: String, _ a: Answer?) {
        lock.lock(); let c = waiting.removeValue(forKey: id); lock.unlock()
        c?.resume(returning: a)
    }
    func take() -> [[String: String]] {
        lock.lock(); defer { lock.unlock() }
        let out = pending.map { ["id": $0.id, "words": $0.words] }; pending = []
        return out
    }
    func said(_ id: String, line: String, finish: Bool) { resolve(id, (line.isEmpty && !finish) ? nil : (line, finish)) }
    public func clear() {
        lock.lock(); pending = []; let all = waiting; waiting = [:]; lock.unlock()
        for (_, c) in all { c.resume(returning: nil) }
    }
}

public enum EditionNative {
    public static func op(_ r: [String: Any]) -> [String: Any] {
        switch r["op"] as? String ?? "" {
        case "editionHeard": return ["items": EditionTalk.shared.take()]
        case "editionSaid": EditionTalk.shared.said(r["id"] as? String ?? "", line: r["line"] as? String ?? "", finish: (r["finish"] as? Bool) ?? false); return [:]
        default: return [:]
        }
    }
}

// MARK: Voice at the setup screen, on the existing engine

/// The programme the engine plays while the reader talks to the setup: one line that says what to do.
public final class EditionSetupProgramme: Programme, @unchecked Sendable {
    public static let stageID = "edition-setup"
    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "Edition setup (first run)", summary: "One spoken line; the reader's words add and remove topics.")
    }
    public init(_ p: StageParams) {}
    public func load() async throws -> ProgrammeDocument {
        ProgrammeDocument(id: "edition-setup", title: "Your edition", blocks: [.speech(text: "Tap the topics you want, or tell me. Say, add space and chess.")])
    }
}

/// System 2 for the setup: the words go to shared code, which adds or removes topics and answers one short line.
public final class EditionSetupDesk: Desk, @unchecked Sendable {
    public static let stageID = "edition-setup-desk"
    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "Edition setup desk (first run)", summary: "Adds and removes edition topics from what the reader says; shared code applies it.")
    }
    public init(_ p: StageParams) {}
    public func submit(_ turn: DeskTurn) -> AsyncThrowingStream<Revision, Error> {
        AsyncThrowingStream { c in
            let words = turn.listenerWords, id = turn.turnID, at = turn.blockIndex
            Task {
                if let a = await EditionTalk.shared.say(words) {
                    if !a.line.isEmpty { c.yield(Revision(id: "edition-setup-\(id)", fromBlock: at + 1, blocks: [.speech(text: a.line)], keepRest: false, isFinal: true)) }
                    if a.finish { EditionTalk.shared.onFinish?() }
                }
                c.finish()
            }
        }
    }
}

public extension StageRegistry {
    static func registerEditionSetupStages(_ r: StageRegistry) {
        r.register(.programme, EditionSetupProgramme.self) { EditionSetupProgramme($0) }
        r.register(.desk, EditionSetupDesk.self) { EditionSetupDesk($0) }
    }
}
