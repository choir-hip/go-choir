import Foundation

// The edition picks, native side, and the edition writer. The topics themselves (the tree, the picks, the saved file, the spoken edits) are shared Exact code
// now (AutoradioLab/edition-picks.ts, edition-model.ts); what stays here is the local edition writer and its deadline, which the onboarding controller
// still runs, and the three native pieces below.

// MARK: The edition: written from the choices

public struct Edition: Codable, Equatable, Sendable {
    public struct Section: Codable, Equatable, Sendable { public var heading: String; public var lines: [String] }
    public var title: String
    public var intro: String
    public var writingLines: [String]
    public var sections: [Section]

    public static func parse(_ data: Data) -> Edition? { try? JSONDecoder().decode(Edition.self, from: data) }
    /// Heading, then its lines, section by section: what a plain page shows.
    public var lines: [String] { sections.flatMap { [$0.heading] + $0.lines } }
}

public enum EditionError: Error, CustomStringConvertible {
    case failed(String)
    public var description: String { switch self { case .failed(let m): return m } }
}

public protocol EditionWriter: Sendable {
    /// Writes the edition from the chosen topics (none: the general edition). `progress` receives what is happening, in plain words.
    func write(topics: [String], progress: @escaping @Sendable (String) -> Void) async throws -> Edition
}

/// The edition on this phone, from the choices alone: deterministic, no network (E6). The computer's desk is not asked.
public struct LocalEditionWriter: EditionWriter {
    public init() {}
    public static func edition(topics: [String]) -> Edition {
        let lines = topics.isEmpty ? ["The whole paper, until I say otherwise."] : topics
        return Edition(title: "My edition", intro: "This is your edition. Change anything by asking.", writingLines: ["Putting your edition together"],
                       sections: [.init(heading: "What I follow", lines: lines)])
    }
    public func write(topics: [String], progress: @escaping @Sendable (String) -> Void) async throws -> Edition {
        progress("Putting your edition together")
        return Self.edition(topics: topics)
    }
}

// MARK: Finishing: a deadline, never a dead end
//
// The first phone walk's interview ended with "Good. Writing your edition now." and then nothing, for ever. Cause (HEAD of
// lane/autoradio-qa): `AutoradioModule+Onboarding.swift:275` was the only caller of `writeEdition()` on the way to the end,
// inside a drive that no longer exists. On a phone the end of the interview came from `InterviewDesk.submit` (Interview.swift,
// `finish(limit:)` sets status `.done` and returns the closing line); nothing watched `.done`, so the step never became
// "writing". Had it been reached, `UnbuiltEditionWriter.write` threw "not connected in this build", with no deadline anywhere.
// Now Continue calls `EditionCompletion.run`, which always ends: the edition, a failure, or the deadline.
//
//  C1 The writer returns: `.written`.        C2 The writer throws: `.failed(plain words)`.
//  C3 The writer never returns: `.timedOut` at the deadline, and the writer's task is cancelled.
//  C4 Whatever happens, the reader's topics are untouched (they are saved before this runs).

public enum EditionOutcome: Equatable, Sendable {
    case written(Edition)
    case failed(String)
    case timedOut
}

public enum EditionCompletion {
    public static let deadlineSeconds = 20.0
    public static let failedLine = "I couldn't write your edition just now. Your topics are saved."
    public static let timedOutLine = "Writing your edition is taking too long. Your topics are saved."

    public static func run(topics: [String], writer: EditionWriter, deadline: Double = deadlineSeconds,
                           progress: @escaping @Sendable (String) -> Void) async -> EditionOutcome {
        await withTaskGroup(of: EditionOutcome.self) { group in
            group.addTask {
                do { return .written(try await writer.write(topics: topics, progress: progress)) }
                catch is CancellationError { return .timedOut }
                catch { return .failed(failedLine) }
            }
            group.addTask {
                try? await Task.sleep(nanoseconds: UInt64(max(deadline, 0) * 1_000_000_000))
                return .timedOut
            }
            let first = await group.next() ?? .timedOut
            group.cancelAll()
            return first
        }
    }

    /// The general edition: what "Continue anyway" writes. It needs nothing from the network and cannot fail.
    public static func general() -> Edition { LocalEditionWriter.edition(topics: []) }
}

// MARK: The native side of the edition picks

// The native side of the edition picks. The picks themselves (the saved file, the tree, tap, type, say) are shared Exact code: AutoradioLab/edition-picks.ts is the ONE reader and
// writer of `app:/data/edition-topics.json`. Nothing here stores a pick. What native code still needs and why:
//  - `EditionPicksMirror`: the voice's native code reads the picks (the DJ's request text, the on-device retrieval corpus, the edition writer's input). It is a COPY in memory, pushed by
//    shared code after the file is written and emptied at a wipe; there is no file, so nothing of it can outlive an account or a process. Empty until shared code's first push after a launch.
//  - `EditionTalk`: the voice hears the words at the setup screen ("add space and chess"); Exact has no call from native code into shared code, so the words wait in a mailbox, shared
//    code takes them on the screen's own timer, applies them and answers the one line to say back. The desk waits for that answer under a deadline.
//  - `EditionPicksLegacy`: the old build's file lives in Application Support, which shared code cannot see. Read and deleted on shared code's word (docs/reference/AUTORADIO-EDITION-PICKS.md).

/// The listener's picks as the native side sees them: a copy, in memory only.
public final class EditionPicksMirror: @unchecked Sendable {
    public static let shared = EditionPicksMirror()
    private let lock = NSLock()
    private var list: [String] = []
    public var topics: [String] { lock.lock(); defer { lock.unlock() }; return list }
    public func set(_ t: [String]) { lock.lock(); list = t; lock.unlock() }
}

/// The old build's file `Application Support/Autoradio/edition-topics.json`: nothing writes it any more.
public enum EditionPicksLegacy {
    static var file: URL? { FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask).first?.appendingPathComponent("Autoradio").appendingPathComponent("edition-topics.json") }
    static func read() -> [String: Any] {
        guard let f = file, FileManager.default.fileExists(atPath: f.path) else { return ["found": false, "text": ""] }
        return ["found": true, "text": (try? String(contentsOf: f, encoding: .utf8)) ?? ""]
    }
    /// Deletes it and looks again: true only when it is gone (the delete can fail without a sign).
    @discardableResult static func drop() -> Bool {
        guard let f = file else { return true }
        try? FileManager.default.removeItem(at: f)
        return !FileManager.default.fileExists(atPath: f.path)
    }
}

/// Words heard at the setup screen, waiting for shared code, and the answer going back.
public final class EditionTalk: @unchecked Sendable {
    public static let shared = EditionTalk()
    public typealias Answer = (line: String, finish: Bool)
    private let lock = NSLock()
    private var pending: [(id: String, words: String)] = []
    private var waiting: [String: CheckedContinuation<Answer?, Never>] = [:]
    /// Called when the reader says "continue" or "done".
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
    /// Shared code takes what was heard, in order.
    func take() -> [[String: String]] {
        lock.lock(); defer { lock.unlock() }
        let out = pending.map { ["id": $0.id, "words": $0.words] }; pending = []
        return out
    }
    func said(_ id: String, line: String, finish: Bool) { resolve(id, (line.isEmpty && !finish) ? nil : (line, finish)) }
    /// Sign-out and the end of talking: nothing heard before it is applied or answered later.
    public func clear() {
        lock.lock(); pending = []; let all = waiting; waiting = [:]; lock.unlock()
        for (_, c) in all { c.resume(returning: nil) }
    }
}

/// The ops of the app module's `editionOp` (modules/apple/ModelsEntry.swift): the one native call surface of the edition picks.
public enum EditionNative {
    public static func op(_ r: [String: Any]) -> [String: Any] {
        switch r["op"] as? String ?? "" {
        case "editionLegacy": return EditionPicksLegacy.read()
        case "editionLegacyDrop": return ["gone": EditionPicksLegacy.drop()]
        case "editionMirror": EditionPicksMirror.shared.set((r["topics"] as? [String]) ?? []); return [:]
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
