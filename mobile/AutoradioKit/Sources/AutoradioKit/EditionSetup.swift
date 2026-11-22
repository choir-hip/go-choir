import Foundation

// The edition writer and the native side of the edition picks. The topics themselves (tree, picks, saved file, spoken edits) are shared Exact code (AutoradioLab/edition-picks.ts).
// THE EDITION WRITER STAYS NATIVE for one reason: it is the onboarding controller's step machine (AutoradioModule+Onboarding.swift), which moves with the onboarding page; it needs no voice.
// It writes from the picks the page's send carries, never from the pushed copy alone. No TS twin is kept (a second copy would drift).

// MARK: The edition: written from the choices

public struct Edition: Codable, Equatable, Sendable {
    public struct Section: Codable, Equatable, Sendable { public var heading: String; public var lines: [String] }
    public var title: String
    public var intro: String
    public var writingLines: [String]
    public var sections: [Section]

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

// MARK: Finishing: a deadline, never a dead end (`EditionCompletion.run` always ends: the edition, a failure, or the deadline; the topics are untouched)

public enum EditionOutcome: Equatable, Sendable {
    case written(Edition)
    case failed(String)
    case timedOut
}

public enum EditionCompletion {
    public static let deadlineSeconds = 20.0
    public static let failedLine = "I couldn't write your edition just now. Your topics are saved."

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

// Nothing here stores a pick (docs/reference/AUTORADIO-EDITION-PICKS.md). What native code needs, and why:
//  - EditionPicksMirror: the voice's native code (DJ request, on-device retrieval) reads the picks; an in-memory COPY pushed by shared code, no file.
//  - EditionTalk: the voice hears words at the setup screen; Exact has no call from native into shared code, so they wait in a mailbox that shared code takes on the screen's timer and answers.
// The old build's file (Application Support/Autoradio/edition-topics.json) is NEVER READ and NOT deleted here: it says nothing about whose it is, so an upgraded phone starts with no picks; only the account wipe removes it.

public final class EditionPicksMirror: @unchecked Sendable {
    public static let shared = EditionPicksMirror()
    private let lock = NSLock()
    private var list: [String] = []
    private var pushed = false
    public var topics: [String] { lock.lock(); defer { lock.unlock() }; return list }
    /// True once shared code has pushed a copy since the launch or the last wipe: an empty copy never pushed is "not known", not "the listener chose nothing" (nothing writes it as a choice).
    public var known: Bool { lock.lock(); defer { lock.unlock() }; return pushed }
    public func set(_ t: [String]) { lock.lock(); list = t; pushed = true; lock.unlock() }
    public func reset() { lock.lock(); list = []; pushed = false; lock.unlock() }
}

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
        case "editionMirrorReset": EditionPicksMirror.shared.reset(); return [:]
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
