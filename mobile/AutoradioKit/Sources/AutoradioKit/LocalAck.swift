import Foundation

// The immediate response to the listener comes from a language model ON THE PHONE, written from what was just said, the page in
// front and that page's text, not from a list of stock phrases (owner rulings 23, 2026-11-11). It is a STREAM, not a line: it goes
// on for as long as the desk has not produced speech, sentence by sentence, the most useful thing first, so it can be cut cleanly
// at any sentence boundary and replaced by the desk's answer at any time. No wall-clock cutoff anywhere (owner: no
// deterministic timers around model calls): the signals are the model's own end of stream, the desk's speech, and the listener.
//
// There are NO deterministic filters on what the model says (owner, 2026-11-11: "We should be able to steer models with prompts, not
// deterministic rules, which will lead to weird behaviors"): no word limit, no number or yes/no rejection, no answer-claim pattern, no
// repeat filter, no token budget, no time budget. The prompt says what the voice is for and carries the recent lines, so the model itself
// avoids repeating them. What stays are real signals and sanitising for speech.
//
// WAYS THIS CAN FAIL (written before the code; each has a test in LocalAckTests):
//  1. The model is not loaded or errors: NO words. The engine's non-verbal cue (played at once at the end of speech, "heard you") is all
//     there is. A stock phrase list must not survive as the fallback.
//  2. The model is slow: nothing cuts it off. A sentence is spoken when it is ready. Real desk speech cancels the generation.
//  3. The model's text holds markup the voice cannot say (think tags, bullets, list numbers, asterisks, quotes): it is stripped. Nothing
//     is judged on meaning.
//  4. The model ends its stream when it has nothing more: the holder ends (a real signal). If a tiny model repeats itself or pads, that
//     is a finding about the model, reported with samples, and a reason for a larger one: not a reason for a rule.
//  5. The turn is interrupted while generating: the generation is cancelled and nothing more is spoken.
//  6. Text with quotes or newlines in the listener's words or the page text cannot break the prompt's shape (they are flattened).
//  7. A sentence boundary inside a number ("3.5") is not a cut: a terminator ends a sentence only when whitespace follows it; the end
//     of the stream flushes what is left.
//  8. The prompt must carry: the voice's purpose, front-loading, stand-alone sentences, truthfulness about the page and the words, no
//     passing off a guess as the desk's answer, and the recent lines (tested as content).

/// The page in front of the listener when they asked: what the acknowledgement may name.
public struct FrontPage: Codable, Sendable, Equatable {
    public var address: String
    public var title: String
    public var kind: String
    public init(address: String = "", title: String = "", kind: String = "") { self.address = address; self.title = title; self.kind = kind }
}

/// What a generation cost, for the trace: the prompt had `promptTokens` tokens, of which the first `prefixTokens` are the stable prefix
/// (instructions and the page), `reusedTokens` were already in the model's cache (a warm cache: the page was read before the question).
public struct GenerationReport: Codable, Sendable, Equatable {
    public var promptTokens: Int, prefixTokens: Int, reusedTokens: Int
    public var prefillMs: Double
    public init(promptTokens: Int, prefixTokens: Int, reusedTokens: Int, prefillMs: Double) {
        self.promptTokens = promptTokens; self.prefixTokens = prefixTokens; self.reusedTokens = reusedTokens; self.prefillMs = prefillMs
    }
    /// The whole stable prefix was already in the cache: only the listener's words were read at end of speech.
    public var warm: Bool { prefixTokens > 0 && reusedTokens >= prefixTokens }
}

/// A small language model that runs on this phone. The app module (which links llama.cpp) installs one; the kit never
/// imports it.
///
/// The prompt is three parts so the model's cache can be reused: `system` (the instructions, fixed) and `userPrefix` (the page in
/// front: its title, kind and text) are STABLE for as long as the page is in front, byte for byte; `userSuffix` (the recent lines and
/// the listener's words) changes every turn. A model that keeps its cache reads the prefix once, when the page comes to the front
/// (`prefill`), and at end of speech only the suffix.
public protocol LocalLanguageModel: Sendable {
    /// Loaded in memory now. The acknowledgement never waits for a load: if false, there is no spoken line.
    var isLoaded: Bool { get }
    /// One complete completion (the setup check uses it).
    func generate(system: String, user: String, temperature: Double) async throws -> String
    /// The completion as it is produced, piece by piece. The stream ends when the model ends it (or its context is full: a physical
    /// limit); cancelling the consumer cancels the generation. There is no token budget.
    func stream(system: String, userPrefix: String, userSuffix: String, temperature: Double) -> AsyncThrowingStream<String, Error>
    /// Read the stable prefix into the model's cache ahead of a question. Cooperative: cancelling the caller stops it at the next
    /// chunk, and what was read stays. A turn that starts preempts it. Models without a cache do nothing.
    func prefill(system: String, userPrefix: String) async
    /// What the last generation cost (nil: unknown), and what the last prefill cost.
    func lastReport() -> GenerationReport?
    func lastPrefill() -> GenerationReport?
    /// Called synchronously when a generation for an acknowledgement is about to be asked for, and when it has ended (any way).
    /// The model uses them to let the acknowledgement go first: a decision waits until no acknowledgement is pending.
    func willGenerate()
    func generationEnded()
    /// Load from disk when the model file is already on the phone (never downloads). Called at start.
    func warm() async
}
public extension LocalLanguageModel {
    func warm() async {}
    func willGenerate() {}
    func generationEnded() {}
    func prefill(system: String, userPrefix: String) async {}
    func lastReport() -> GenerationReport? { nil }
    func lastPrefill() -> GenerationReport? { nil }
    /// A model that can only complete in one piece streams that one piece.
    func stream(system: String, userPrefix: String, userSuffix: String, temperature: Double) -> AsyncThrowingStream<String, Error> {
        AsyncThrowingStream { c in
            let t = Task {
                do { c.yield(try await generate(system: system, user: userPrefix + userSuffix, temperature: temperature)); c.finish() }
                catch { c.finish(throwing: error) }
            }
            c.onTermination = { _ in t.cancel() }
        }
    }
}

public enum LocalLanguage {
    private static let box = Locked<(any LocalLanguageModel)?>(nil)
    public static var model: (any LocalLanguageModel)? {
        get { box.value }
        set { box.value = newValue }
    }
}

/// The last few lines the phone said, so the next turn does not say the same.
public final class AckHistory: @unchecked Sendable {
    public static let shared = AckHistory()
    private let lock = NSLock()
    private var lines: [String] = []
    private let keep: Int
    public init(keep: Int = 12) { self.keep = keep }
    public var recent: [String] { lock.lock(); defer { lock.unlock() }; return lines }
    public func add(_ l: String) { lock.lock(); lines.append(l); if lines.count > keep { lines.removeFirst(lines.count - keep) }; lock.unlock() }
    public func clear() { lock.lock(); lines = []; lock.unlock() }
}

/// What the phone actually said aloud during a turn, in order. The engine hands it to the desk source, so a desk that is told
/// what the listener already heard does not repeat it (the server route's request has a `heard` list; the polling desk ignores it).
public final class HeardLines: @unchecked Sendable {
    private let lock = NSLock()
    private var lines: [String] = []
    public init() {}
    public func add(_ l: String) { lock.lock(); lines.append(l); lock.unlock() }
    public var snapshot: [String] { lock.lock(); defer { lock.unlock() }; return lines }
}

/// Cuts a stream of text pieces into sentences as they complete. A terminator ends a sentence only when whitespace follows it (so
/// "3.5" is not a cut); a newline always does; `finish` flushes the rest.
public struct SentenceSplitter {
    private var buf = ""
    public init() {}
    public mutating func push(_ piece: String) -> [String] {
        buf += piece
        var out: [String] = []
        while let cut = Self.boundary(buf) {
            let s = String(buf[..<cut]).trimmingCharacters(in: .whitespacesAndNewlines)
            buf = String(buf[cut...])
            if !s.isEmpty { out.append(s) }
        }
        return out
    }
    public mutating func finish() -> [String] {
        let s = buf.trimmingCharacters(in: .whitespacesAndNewlines)
        buf = ""
        return s.isEmpty ? [] : [s]
    }
    static func boundary(_ s: String) -> String.Index? {
        var i = s.startIndex
        var lineStart = s.startIndex
        while i < s.endIndex {
            let ch = s[i]
            let next = s.index(after: i)
            if ch.isNewline { return next }
            if ".!?".contains(ch), next < s.endIndex, s[next].isWhitespace {
                // A list number ("1.", "2)") at the start of a line is markup, not the end of a sentence.
                let before = s[lineStart..<i].trimmingCharacters(in: .whitespaces)
                if ch == ".", !before.isEmpty, before.allSatisfy(\.isNumber) { i = next; continue }
                return next
            }
            i = next
        }
        return nil
    }
}

public enum AckPrompt {
    /// How much of the page's text the prompt carries (characters): the model's context window is finite, so the input is fitted to it.
    public static let pageTextLimit = 1800

    /// What the local voice is for, in words. FIXED text: it is the front of the stable prefix, so the model's cache can hold it.
    public static func system() -> String {
        "You are the voice of a personal radio. The listener has just asked the radio something, and the radio's desk is now working on the answer, which takes a little while. "
            + "Until the desk's answer is ready you talk, so the listener is not left in silence: you think aloud, briefly, about what they asked. "
            + "Speak in plain sentences, one per line. The most useful thing comes first, because you can be interrupted after any sentence and replaced by the desk's answer at any moment. "
            + "Sentence one: show that you understood what they asked, in their own terms, and say the most useful thing you can truthfully say right now from the page in front (use the page text). "
            + "Each later sentence adds something less important: more context from the page, or what is being checked. "
            + "Every sentence must make sense on its own, because any of them may be the last one the listener hears. "
            + "Say only what the page in front and the listener's words support. You do not have the desk's answer: never present a guess as the answer, and never claim the desk has found something. "
            + "Do not repeat what you have just said. When you have nothing more that is true and useful to add, stop. "
            + "Everything you write is spoken aloud, so write only words a voice can say: no lists, no markup, no quotes, no emoji."
    }

    public static func kindLabel(_ kind: String) -> String {
        switch kind {
        case "paper": return "the front page of the paper"
        case "texture", "story", "reader": return "a story"
        case "files": return "the Files page"
        case "mail": return "the Mail page"
        case "settings": return "the Settings page"
        case "": return ""
        default: return "a \(kind) page"
        }
    }

    /// A title that is an address ("texture:abc", a URL, a path) says nothing to the listener and is left out.
    static func usableTitle(_ t: String) -> String? {
        let s = t.trimmingCharacters(in: .whitespacesAndNewlines)
        if s.isEmpty { return nil }
        if !s.contains(" ") && (s.contains(":") || s.contains("/")) { return nil }
        return s
    }

    static func flat(_ s: String) -> String {
        s.replacingOccurrences(of: "\"", with: "'").replacingOccurrences(of: "\n", with: " ").trimmingCharacters(in: .whitespacesAndNewlines)
    }

    /// The page in front: title, kind and text. Byte-stable for a given page, so it can be read into the model's cache ahead of the
    /// question. Empty when there is nothing to say about the page.
    public static func userPrefix(pageTitle: String, pageKind: String, pageText: String = "") -> String {
        var s = ""
        let title = usableTitle(pageTitle), kind = kindLabel(pageKind)
        if let title, !kind.isEmpty { s += "The page in front of the listener is \"\(flat(title))\", \(kind).\n" }
        else if let title { s += "The page in front of the listener is \"\(flat(title))\".\n" }
        else if !kind.isEmpty { s += "The page in front of the listener is \(kind).\n" }
        let text = flat(pageText)
        if !text.isEmpty { s += "Page text: \(String(text.prefix(pageTextLimit)))\n" }
        return s
    }

    /// What changes every turn: the recent lines (context so the model itself avoids repeating them) and the listener's words.
    public static func userSuffix(listener: String, previous: [String]) -> String {
        var s = ""
        let prior = previous.suffix(6)
        if !prior.isEmpty {
            s += "Lines you said recently, which you have already said and should not say again: " + prior.map { "\"\(flat($0))\"" }.joined(separator: "; ") + ".\n"
        }
        s += "The listener said: \"\(flat(listener))\"\nYour sentences, one per line:"
        return s
    }

    /// The whole user message (the setup check and tests).
    public static func user(listener: String, pageTitle: String, pageKind: String, pageText: String = "", previous: [String] = []) -> String {
        userPrefix(pageTitle: pageTitle, pageKind: pageKind, pageText: pageText) + userSuffix(listener: listener, previous: previous)
    }

    /// Sanitising for speech, nothing more: the think tags a model may emit, list markers, markdown and quotes are stripped. Whatever is
    /// left is spoken, whatever it says. Nil only when nothing is left to say.
    public static func speakable(_ raw: String) -> String? {
        var s = raw
        if let r = s.range(of: "</think>") { s = String(s[r.upperBound...]) }
        s = s.replacingOccurrences(of: "<think>", with: "")
        s = s.replacingOccurrences(of: "**", with: "").replacingOccurrences(of: "__", with: "").replacingOccurrences(of: "`", with: "")
        s = s.trimmingCharacters(in: CharacterSet(charactersIn: "\"'*_# \n\t"))
        if s.hasPrefix("- ") { s = String(s.dropFirst(2)) }
        if let r = s.range(of: #"^\d+[.)]\s+"#, options: .regularExpression) { s = String(s[r.upperBound...]) }
        s = s.trimmingCharacters(in: .whitespacesAndNewlines)
        // Nothing a voice can say (only digits and punctuation, such as a stray list number): nothing to speak.
        if s.isEmpty || !s.contains(where: \.isLetter) { return nil }
        return s
    }

    /// The first sentence of a whole completion (the setup check and tests use it).
    public static func firstSentence(_ raw: String) -> String? {
        var s = raw
        if let r = s.range(of: "</think>") { s = String(s[r.upperBound...]) }
        guard let first = s.split(whereSeparator: \.isNewline).map({ $0.trimmingCharacters(in: .whitespaces) }).first(where: { !$0.isEmpty }) else { return nil }
        var sp = SentenceSplitter()
        let sentences = sp.push(first + " ") + sp.finish()
        return speakable(sentences.first ?? first)
    }
}

/// Short, soft, non-verbal sounds. They say something without words and are never a stand-in phrase.
public enum NonVerbalCue {
    /// "Heard you": two quick rising tones, played at once at the end of speech.
    public static func frames(sampleRate: Double = 24_000, now: () -> Double = { ProcessInfo.processInfo.systemUptime }) -> [AudioFrame] {
        render([(523.25, 0.07), (783.99, 0.10)], sampleRate: sampleRate, now: now)
    }
    /// The hand-over from the phone thinking aloud to the desk's answer: one soft falling pair, so the switch sounds intended.
    public static func handover(sampleRate: Double = 24_000, now: () -> Double = { ProcessInfo.processInfo.systemUptime }) -> [AudioFrame] {
        render([(659.25, 0.08), (440.0, 0.12)], sampleRate: sampleRate, now: now)
    }
    static func render(_ tones: [(hz: Double, secs: Double)], sampleRate: Double, now: () -> Double) -> [AudioFrame] {
        var out: [Float] = []
        for (hz, secs) in tones {
            let n = Int(sampleRate * secs)
            for i in 0..<n {
                let x = Double(i) / Double(n)
                let env = sin(Double.pi * x)                      // rises and falls: no click
                out.append(Float(0.18 * env * sin(2 * Double.pi * hz * Double(i) / sampleRate)))
            }
            out.append(contentsOf: [Float](repeating: 0, count: Int(sampleRate * 0.02)))
        }
        let step = Int(sampleRate / 10)                           // 100 ms frames, like the voices
        var frames: [AudioFrame] = []
        var i = 0
        while i < out.count {
            let j = min(out.count, i + step)
            frames.append(AudioFrame(samples: Array(out[i..<j]), sampleRate: sampleRate, t: now()))
            i = j
        }
        return frames
    }
}

/// The holder: the phone's own model thinking aloud, sentence by sentence, for as long as it has something true to add and the
/// desk has not spoken. No wall-clock cutoff: the engine cancels it when real desk speech is ready.
public final class LocalAckHolder: Holder, @unchecked Sendable {
    public static let stageID = "local-ack"
    public static var descriptor: StageDescriptor {
        StageDescriptor(
            displayName: "Local response (on-device model, streamed)",
            summary: "The small language model on this phone thinks aloud while the desk works: most useful sentence first, from what you said and the page in front, stopping by itself when it has nothing more to add, and cut when the desk's answer is ready. If the model is not loaded, only the soft cue plays.",
            params: [.number("temperature", "Variety", 0.7, 0, 1.5)])
    }
    let temperature: Double
    let history: AckHistory
    let modelSource: @Sendable () -> (any LocalLanguageModel)?
    private let lock = NSLock()
    private var task: Task<Void, Never>?
    private var note = ""
    private var generation: GenerationReport?
    public init(_ p: StageParams, history: AckHistory = .shared, model: @escaping @Sendable () -> (any LocalLanguageModel)? = { LocalLanguage.model }) {
        temperature = p.double("temperature", 0.7)
        self.history = history; modelSource = model
    }

    public func diagnostics() -> String { lock.lock(); defer { lock.unlock() }; return note }
    public func report() -> GenerationReport? { lock.lock(); defer { lock.unlock() }; return generation }
    private func say(_ s: String) { lock.lock(); note = s; lock.unlock() }

    public func lines(for i: HolderInput) -> AsyncStream<String> {
        let (temperature, history) = (temperature, history)
        let words = i.listenerWords.trimmingCharacters(in: .whitespacesAndNewlines)
        // The checks that need no model run now, synchronously, so the model is only told a generation is coming when one is.
        guard !words.isEmpty else { say("no words, no call"); return AsyncStream { $0.finish() } }
        guard let m = modelSource(), m.isLoaded else { say("the local model is not loaded"); return AsyncStream { $0.finish() } }
        m.willGenerate()
        return AsyncStream { c in
            let t = Task { [self] in
                defer { m.generationEnded(); c.finish() }
                let pageText = await i.pageText?() ?? ""
                if Task.isCancelled { say("cancelled"); return }
                let prior = history.recent          // context for the prompt, not a filter
                let system = AckPrompt.system()
                let prefix = AckPrompt.userPrefix(pageTitle: i.pageTitle, pageKind: i.pageKind, pageText: pageText)
                let suffix = AckPrompt.userSuffix(listener: words, previous: prior)
                let t0 = ProcessInfo.processInfo.systemUptime
                var splitter = SentenceSplitter()
                var spoken = 0
                var first = true
                func offer(_ raw: String) {
                    guard let line = AckPrompt.speakable(raw) else { return }
                    spoken += 1; history.add(line)
                    c.yield(line)
                }
                do {
                    for try await piece in m.stream(system: system, userPrefix: prefix, userSuffix: suffix, temperature: temperature) {
                        if Task.isCancelled { break }
                        if first { first = false; let r = m.lastReport(); lock.lock(); generation = r; lock.unlock() }
                        for s in splitter.push(piece) { if Task.isCancelled { break }; offer(s) }
                    }
                } catch { say(Task.isCancelled ? "cancelled" : "the model failed: \(error)"); return }
                if Task.isCancelled { say("cancelled: \(spoken) sentences were written before the desk spoke"); return }
                for s in splitter.finish() { offer(s) }
                let ms = Int((ProcessInfo.processInfo.systemUptime - t0) * 1000)
                say(spoken == 0 ? "the model wrote nothing after \(ms) ms" : "\(spoken) sentences in \(ms) ms, then the model had nothing more")
            }
            self.lock.lock(); self.task = t; self.lock.unlock()
            c.onTermination = { _ in t.cancel() }
        }
    }
    public func cancel() { lock.lock(); let t = task; lock.unlock(); t?.cancel() }
}
