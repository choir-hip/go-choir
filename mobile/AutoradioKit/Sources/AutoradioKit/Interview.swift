import Foundation

// The first-run interview as a programme over the question bank (plan section 2, "The interview").
//
// It is NOT a new agent loop. The turn engine, the System 1 questions, the holding voice and the desk are the radio's own;
// the interview supplies only (a) the bank (`mobile/interview/questions.md`), (b) the typed System 1 questions per
// bank entry, (c) a pure state machine (`Interview`) that turns what the listener said into the next lines, and
// (d) two thin stages that plug it into the existing engine: `InterviewProgramme` (the document being played) and
// `InterviewDesk` (System 2 for this programme: it answers a turn with a revision holding the next lines, locally, so
// every reply is fast and nothing waits on the network).
//
// Ways it can fail (written before the code, per the repo testing rule). Each has a test in InterviewTests.
//  1. The bank file is malformed (no entries, a key without a value, an unknown entry): parsing must not crash, must
//     skip the bad line, and the default bank must always parse to the five questions, the follow-up, @say and @faq.
//  2. The copy compiled into the app drifts from mobile/interview/questions.md: a test compares them.
//  3. The listener's answer is "everything" or "the news": the topics question offers sections once or twice, then
//     takes "the whole paper" and moves on. It never loops.
//  4. The listener asks a question instead of answering: it is answered in a sentence from the FAQ (or a default), then
//     the question is repeated rephrased once; the second time it is skipped. "A question the listener does not
//     understand is rephrased once from the bank, then skipped."
//  5. The listener interrupts mid-question: the interrupted line is marked as cut off in the document; if the words
//     answer it, the question is dropped and the next line answers what was said; if they ask something, they are
//     answered and the question is reframed. The aside (the how-Choir-works sentence) is never spoken over an
//     interruption and is not lost: it is said at the next uninterrupted beat.
//  6. Silence: a reprompt once ("Take your time; or say skip"), then the question is skipped; two silent questions in
//     a row pause the interview on its card ("I will be here when you are ready").
//  7. Noise (too loud, the radio hearing itself): the interview pauses with what was said kept; nothing falls back to
//     a button or to typing.
//  8. "That's enough", "skip", "later" at any point goes to the read-back with what has been said, even with nothing said.
//  9. The read-back is corrected more than twice: after two corrected read-backs the next change is applied and the
//     interview ends ("at most twice"). Nothing loops.
// 10. The interview runs past its time budget (six minutes of elapsed time): it goes to the read-back at the next beat.
// 11. A crash or restart mid-interview: nothing resumes by itself (repo rule). The document holds what was said; the
//     saved state loads only when the listener chooses "Continue your interview", and it comes back paused.
// 12. System 1 is slow, absent or wrong: the heuristic classifier gives a verdict from the words alone, so a turn is
//     never stuck; a System 1 option the interview does not know is ignored.
// 13. The desk cannot write the edition: the interview and its document are kept, the status is `failed`, a retry exists.
// 14. A turn arrives after the interview is done or paused: it is ignored (no lines), never an error.

// MARK: The bank

public struct InterviewQuestion: Equatable, Sendable {
    public var id: String
    public var note: String
    public var kind: String
    public var ask: String
    public var rephrase: String
    public var vague: String
    public var aside: String
    /// Prepared follow-ups by keyword; "*" is the default.
    public var follow: [String: String]
}

public struct InterviewBank: Equatable, Sendable {
    public var questions: [InterviewQuestion]
    public var say: [String: String]
    public var faq: [(keywords: [String], text: String)]

    public static func == (a: InterviewBank, b: InterviewBank) -> Bool {
        a.questions == b.questions && a.say == b.say && a.faq.count == b.faq.count
            && zip(a.faq, b.faq).allSatisfy { $0.keywords == $1.keywords && $0.text == $1.text }
    }

    /// Parse the bank's text. Lines that are not `## id` or `key: value` are ignored; an entry with no `ask` and no
    /// `kind: follow` is dropped (unless it starts with @).
    public static func parse(_ text: String) -> InterviewBank {
        var order: [String] = []
        var entries: [String: [(String, String)]] = [:]
        var current: String?
        for raw in text.split(separator: "\n", omittingEmptySubsequences: true) {
            let line = String(raw).trimmingCharacters(in: .whitespaces)
            if line.hasPrefix("## ") {
                let id = String(line.dropFirst(3)).trimmingCharacters(in: .whitespaces)
                current = id.isEmpty ? nil : id
                if let c = current, entries[c] == nil { entries[c] = []; order.append(c) }
                continue
            }
            guard let c = current, !line.hasPrefix("#"), let colon = line.firstIndex(of: ":") else { continue }
            let key = String(line[..<colon]).trimmingCharacters(in: .whitespaces)
            let value = String(line[line.index(after: colon)...]).trimmingCharacters(in: .whitespaces)
            guard !key.isEmpty, !value.isEmpty, key.allSatisfy({ $0.isLetter || $0.isNumber || "_.*-".contains($0) }) else { continue }
            entries[c, default: []].append((key, value))
        }
        var qs: [InterviewQuestion] = [], say: [String: String] = [:], faq: [(keywords: [String], text: String)] = []
        for id in order {
            let kv = entries[id] ?? []
            if id == "@say" { for (k, v) in kv { say[k] = v } ; continue }
            if id == "@faq" {
                for (k, v) in kv where k == "faq" {
                    let parts = v.components(separatedBy: "=>")
                    guard parts.count == 2 else { continue }
                    let words = parts[0].split(separator: "|").map { $0.trimmingCharacters(in: .whitespaces).lowercased() }.filter { !$0.isEmpty }
                    let t = parts[1].trimmingCharacters(in: .whitespaces)
                    if !words.isEmpty, !t.isEmpty { faq.append((words, t)) }
                }
                continue
            }
            if id.hasPrefix("@") { continue }
            func get(_ k: String) -> String { kv.first { $0.0 == k }?.1 ?? "" }
            var follow: [String: String] = [:]
            for (k, v) in kv where k.hasPrefix("follow.") { follow[String(k.dropFirst(7))] = v }
            let q = InterviewQuestion(id: id, note: get("note"), kind: get("kind"), ask: get("ask"), rephrase: get("rephrase"),
                                      vague: get("vague"), aside: get("aside"), follow: follow)
            if q.ask.isEmpty && q.kind != "follow" { continue }
            qs.append(q)
        }
        return InterviewBank(questions: qs, say: say, faq: faq)
    }

    public static var `default`: InterviewBank { parse(defaultText) }

    /// A phrase from `@say`, with `{name}` fills. A missing key gives "" (the interview then simply says less).
    public func line(_ key: String, _ fills: [String: String] = [:]) -> String {
        var s = say[key] ?? ""
        for (k, v) in fills { s = s.replacingOccurrences(of: "{\(k)}", with: v) }
        return s
    }
}

// MARK: Notes, turns, lines

/// What the interview learned; the desk writes the edition from the document, and the read-back is built from this.
public struct InterviewNotes: Codable, Equatable, Sendable {
    public var topics: [String] = []
    public var less: [String] = []
    public var trusted: [String] = []
    public var avoided: [String] = []
    public var places: [String] = []
    public var people: [String] = []
    public var listening: String = ""
    public init() {}
    public var isEmpty: Bool { topics.isEmpty && less.isEmpty && trusted.isEmpty && avoided.isEmpty && places.isEmpty && people.isEmpty && listening.isEmpty }
}

public struct InterviewTurn: Codable, Equatable, Sendable {
    /// "radio" or "you"
    public var who: String
    public var text: String
    /// The radio's line was cut off by the listener (or, for "you", the words cut in).
    public var interrupted: Bool
    public var questionID: String
    /// For a radio line that was cut off: the words the listener actually heard of it (whole words, from the audio played).
    public var heard: String?
    public init(who: String, text: String, interrupted: Bool = false, questionID: String = "", heard: String? = nil) {
        self.who = who; self.text = text; self.interrupted = interrupted; self.questionID = questionID; self.heard = heard
    }
}

/// One thing the radio says next. `kind`: say | question | aside | readback | pause | closing.
public struct InterviewLine: Equatable, Sendable {
    public var text: String
    public var kind: String
    public var questionID: String
    public init(_ text: String, kind: String, questionID: String = "") { self.text = text; self.kind = kind; self.questionID = questionID }
}

/// What System 1 (or the heuristic) says the listener just did.
public enum InterviewVerdict: String, Codable, CaseIterable, Sendable {
    case answered, vague, asked, refused, wander, done
}

public enum InterviewStatus: String, Codable, Sendable {
    case notStarted, running, paused, readback, done
}

public struct InterviewState: Codable, Equatable, Sendable {
    public var status: InterviewStatus = .notStarted
    public var pauseReason: String = ""
    /// Index into the bank's questions; == count means the read-back.
    public var index = 0
    public var turns: [InterviewTurn] = []
    public var notes = InterviewNotes()
    public var rephrased: [String] = []
    public var vagueTries: [String: Int] = [:]
    public var silentOn: [String: Int] = [:]
    public var silentRun = 0
    public var spokenAsides: [String] = []
    public var corrections = 0
    public var elapsed: Double = 0
    public var answeredIDs: [String] = []
    public var firstAnswerWords = ""
    public init() {}
}

// MARK: The interview

public struct Interview: Equatable, Sendable {
    public static let maxSeconds = 360.0
    public static let maxCorrections = 2

    public var bank: InterviewBank
    public var state: InterviewState
    /// Section names from today's paper, offered when the first answer is vague.
    public var sections: [String]

    public init(bank: InterviewBank = .default, sections: [String] = ["World", "Local", "Culture", "Tech and AI"], state: InterviewState = InterviewState()) {
        self.bank = bank; self.sections = sections; self.state = state
    }

    public var status: InterviewStatus { state.status }
    var questions: [InterviewQuestion] { bank.questions }
    public var current: InterviewQuestion? { questions.indices.contains(state.index) ? questions[state.index] : nil }

    // MARK: Beats

    /// The first lines: the opening and the first question. The microphone is open from here on.
    public mutating func start() -> [InterviewLine] {
        guard state.status == .notStarted else { return [] }
        state.status = .running
        var out: [InterviewLine] = []
        let open = bank.line("open")
        if !open.isEmpty { out.append(.init(open, kind: "say")) }
        out += askCurrent(first: true)
        return out
    }

    /// The listener said something. `verdict` comes from System 1's typed question when it answered, else the heuristic.
    /// `interrupted`: the words cut in over the radio. `followKey`: System 1's choice of prepared follow-up, if any.
    /// `heard`: when interrupted, how much of the radio's line the listener heard before cutting in.
    public mutating func hear(_ words: String, verdict: InterviewVerdict? = nil, interrupted: Bool = false, followKey: String? = nil, heard: String? = nil) -> [InterviewLine] {
        guard state.status == .running || state.status == .readback else { return [] }   // failure 14
        let text = words.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !text.isEmpty else { return [] }
        if interrupted, let i = state.turns.lastIndex(where: { $0.who == "radio" }) {
            state.turns[i].interrupted = true
            if let h = heard { state.turns[i].heard = h }
        }
        state.turns.append(InterviewTurn(who: "you", text: text, interrupted: interrupted, questionID: current?.id ?? "readback"))
        state.silentRun = 0
        if let c = current { state.silentOn[c.id] = 0 }

        if state.status == .readback { return hearReadback(text, verdict: verdict) }
        let v = verdict ?? InterviewHeuristic.verdict(text, topics: current?.note == "topics" && current?.kind != "follow")
        if v == .done { return goReadback(lead: "") }                                            // failure 8
        guard let q = current else { return goReadback(lead: "") }

        switch v {
        case .asked:
            let reply = answerAsk(text)
            record(reply, qid: q.id)
            return [InterviewLine(reply, kind: "say")] + repeatOrSkip(q, lead: "")
        case .wander:
            return repeatOrSkip(q, lead: "")
        case .refused:
            if q.note == "topics" && q.kind != "follow" { return vagueBeat(q) }
            return answered(q, text, empty: true, interrupted: interrupted, followKey: followKey)
        case .vague:
            if q.note == "topics" && q.kind != "follow" { return vagueBeat(q) }
            return answered(q, text, empty: false, interrupted: interrupted, followKey: followKey)
        case .answered, .done:
            return answered(q, text, empty: false, interrupted: interrupted, followKey: followKey)
        }
    }

    /// The question went unanswered for the listening window.
    public mutating func silence() -> [InterviewLine] {
        guard state.status == .running, let q = current else { return [] }
        let n = (state.silentOn[q.id] ?? 0) + 1
        state.silentOn[q.id] = n
        if n == 1 {
            let t = bank.line("silence1")
            record(t, qid: q.id)
            return [.init(t, kind: "say", questionID: q.id)]
        }
        state.silentRun += 1                                                                     // failure 6
        if state.silentRun >= 2 {
            state.status = .paused; state.pauseReason = "silent"
            let t = bank.line("silence_pause")
            record(t, qid: q.id)
            return [.init(t, kind: "pause", questionID: q.id)]
        }
        return advance(lead: bank.line("skip_lead"))
    }

    /// The room is too loud, or the radio is hearing itself.
    public mutating func tooLoud() -> [InterviewLine] {
        guard state.status == .running || state.status == .readback else { return [] }
        state.status = .paused; state.pauseReason = "loud"                                       // failure 7
        let t = bank.line("loud")
        record(t, qid: current?.id ?? "readback")
        return [.init(t, kind: "pause")]
    }

    /// The listener chose "Continue your interview" (an owner act): say where we were and ask again.
    public mutating func resume() -> [InterviewLine] {
        guard state.status == .paused || (state.status == .running && state.turns.isEmpty == false) else { return [] }
        let wasReadback = state.index >= questions.count
        state.status = wasReadback ? .readback : .running
        state.pauseReason = ""; state.silentRun = 0
        var out: [InterviewLine] = []
        let welcome = bank.line("resume")
        if !welcome.isEmpty { out.append(.init(welcome, kind: "say")) }
        if wasReadback { out.append(.init(readBackText(), kind: "readback")); record(readBackText(), qid: "readback") }
        else { out += askCurrent(first: false) }
        return out
    }

    /// Elapsed interview time, from the caller's clock; past the budget the next beat is the read-back.
    public mutating func tick(elapsed: Double) { state.elapsed = elapsed }

    // MARK: Internals

    private mutating func record(_ text: String, qid: String) {
        state.turns.append(InterviewTurn(who: "radio", text: text, questionID: qid))
    }

    private mutating func askCurrent(first: Bool) -> [InterviewLine] {
        guard let q = current else { return goReadback(lead: "") }
        var text = q.ask
        if q.kind == "follow" {
            text = followLine(q)
            if text.isEmpty { return advance(lead: "") }
        }
        record(text, qid: q.id)
        return [.init(text, kind: "question", questionID: q.id)]
    }

    private func followLine(_ q: InterviewQuestion) -> String {
        let toks = InterviewHeuristic.norm(state.firstAnswerWords).split(separator: " ").map(String.init)
        for (k, v) in q.follow.sorted(by: { $0.key < $1.key }) where k != "*" {
            if toks.contains(where: { $0 == k || (k.count >= 4 && $0.hasPrefix(k)) }) { return v }
        }
        return q.follow["*"] ?? ""
    }

    private mutating func vagueBeat(_ q: InterviewQuestion) -> [InterviewLine] {
        let n = (state.vagueTries[q.id] ?? 0) + 1
        state.vagueTries[q.id] = n
        if n >= 2 {                                                                              // failure 3
            state.notes.topics = ["the whole paper"]
            state.answeredIDs.append(q.id)
            state.firstAnswerWords = "everything"
            return advance(lead: bank.line("whole_paper"))
        }
        let list = sections.isEmpty ? "world, local, culture and tech" : sections.prefix(3).joined(separator: ", ")
        let line = (q.vague.isEmpty ? q.rephrase : q.vague).replacingOccurrences(of: "{sections}", with: list)
        record(line, qid: q.id)
        return [.init(line, kind: "question", questionID: q.id)]
    }

    private mutating func repeatOrSkip(_ q: InterviewQuestion, lead: String) -> [InterviewLine] {
        if state.rephrased.contains(q.id) {
            return advance(lead: bank.line("skip_lead"))
        }
        state.rephrased.append(q.id)
        var out: [InterviewLine] = []
        let l = bank.line("rephrase_lead")
        if !lead.isEmpty { out.append(.init(lead, kind: "say")) } else if !l.isEmpty { out.append(.init(l, kind: "say")) }
        let t = q.rephrase.isEmpty ? q.ask : q.rephrase
        record(t, qid: q.id)
        out.append(.init(t, kind: "question", questionID: q.id))
        return out
    }

    private func answerAsk(_ words: String) -> String {
        let w = words.lowercased()
        for f in bank.faq where f.keywords.contains(where: { w.contains($0) }) { return f.text }
        return bank.line("ask_default")
    }

    private mutating func answered(_ q: InterviewQuestion, _ text: String, empty: Bool, interrupted: Bool, followKey: String?) -> [InterviewLine] {
        let before = state.notes
        if !empty { InterviewExtract.apply(question: q, to: &state.notes, words: text) }
        if q.kind != "follow" && q.note == "topics" { state.firstAnswerWords = text }
        if let k = followKey, !k.isEmpty { state.firstAnswerWords += " " + k }
        state.answeredIDs.append(q.id)
        let noted = Self.describeChange(from: before, to: state.notes)
        var lead: String
        if noted.isEmpty { lead = interrupted ? bank.line("interrupt_other") : (empty ? bank.line("nothing_noted") : bank.line("ack")) }
        else { lead = interrupted ? bank.line("interrupt_answer", ["noted": noted]) : bank.line("noted", ["noted": noted]) }
        return advance(lead: lead, fromAnsweredAside: interrupted ? nil : q)
    }

    /// "tech and local", "less of sports", ... what the last answer added, for the acknowledgement.
    static func describeChange(from a: InterviewNotes, to b: InterviewNotes) -> String {
        func added(_ x: [String], _ y: [String]) -> [String] { y.filter { !x.contains($0) } }
        var parts: [String] = []
        let t = added(a.topics, b.topics); if !t.isEmpty { parts.append(joined(t)) }
        let l = added(a.less, b.less); if !l.isEmpty { parts.append("less " + joined(l)) }
        let tr = added(a.trusted, b.trusted); if !tr.isEmpty { parts.append("you trust " + joined(tr)) }
        let av = added(a.avoided, b.avoided); if !av.isEmpty { parts.append("not " + joined(av)) }
        let p = added(a.places, b.places) + added(a.people, b.people); if !p.isEmpty { parts.append(joined(p)) }
        if a.listening != b.listening, !b.listening.isEmpty { parts.append(b.listening) }
        return parts.joined(separator: "; ")
    }

    /// Move to the next question (skipping the follow-up when the first answer was already specific), saying `lead`
    /// first, then the one aside that is ready (only after an uninterrupted answer), then the next question or the read-back.
    private mutating func advance(lead: String, fromAnsweredAside answered: InterviewQuestion? = nil) -> [InterviewLine] {
        var out: [InterviewLine] = []
        if !lead.isEmpty { out.append(.init(lead, kind: "say")); record(lead, qid: current?.id ?? "") }
        if answered != nil,
           let a = questions.first(where: { !$0.aside.isEmpty && state.answeredIDs.contains($0.id) && !state.spokenAsides.contains($0.id) }) {
            state.spokenAsides.append(a.id)
            out.append(.init(a.aside, kind: "aside", questionID: a.id)); record(a.aside, qid: a.id)
        }
        state.index += 1
        if let q = current, q.kind == "follow",
           InterviewHeuristic.isSpecific(state.firstAnswerWords) || state.notes.topics.isEmpty || state.notes.topics == ["the whole paper"] {
            state.index += 1                                                                     // skipped when the answer was specific
        }
        if state.elapsed >= Self.maxSeconds { return out + goReadback(lead: bank.line("time_up")) }   // failure 10
        if current == nil { return out + goReadback(lead: "") }
        out += askCurrent(first: false)
        return out
    }

    private mutating func goReadback(lead: String) -> [InterviewLine] {
        var out: [InterviewLine] = []
        if !lead.isEmpty { out.append(.init(lead, kind: "say")); record(lead, qid: "") }
        state.index = questions.count
        state.status = .readback
        let t = readBackText()
        record(t, qid: "readback")
        out.append(.init(t, kind: "readback", questionID: "readback"))
        return out
    }

    // MARK: Read-back

    public func readBackText() -> String {
        let n = state.notes
        if n.isEmpty { return bank.line("readback_empty") + " " + bank.line("readback_ask") }
        var parts: [String] = []
        if !n.topics.isEmpty { parts.append("You want to hear about " + Self.joined(n.topics) + ".") }
        if !n.less.isEmpty { parts.append("Less of " + Self.joined(n.less) + ".") }
        if !n.trusted.isEmpty { parts.append("You trust " + Self.joined(n.trusted) + ".") }
        if !n.avoided.isEmpty { parts.append("Not " + Self.joined(n.avoided) + ".") }
        if !n.places.isEmpty { parts.append("Near " + Self.joined(n.places) + ".") }
        if !n.people.isEmpty { parts.append("Following " + Self.joined(n.people) + ".") }
        if !n.listening.isEmpty { parts.append("You listen " + n.listening + ".") }
        let intro = bank.line("readback_intro")
        return ([intro] + parts + [bank.line("readback_ask")]).filter { !$0.isEmpty }.joined(separator: " ")
    }

    private mutating func hearReadback(_ text: String, verdict: InterviewVerdict?) -> [InterviewLine] {
        let v = verdict ?? InterviewHeuristic.readbackVerdict(text)
        if v == .done { return finish(limit: false) }
        InterviewExtract.correct(&state.notes, words: text)
        state.corrections += 1
        if state.corrections > Self.maxCorrections { return finish(limit: true) }                // failure 9
        let t = readBackAgainText()
        record(t, qid: "readback")
        return [.init(t, kind: "readback", questionID: "readback")]
    }

    func readBackAgainText() -> String {
        let again = bank.line("readback_again")
        let body = readBackText()
        return [again, body.replacingOccurrences(of: bank.line("readback_intro") + " ", with: "")].filter { !$0.isEmpty }.joined(separator: " ")
    }

    private mutating func finish(limit: Bool) -> [InterviewLine] {
        state.status = .done
        let t = bank.line(limit ? "closing_limit" : "closing")
        record(t, qid: "")
        return [.init(t, kind: "closing")]
    }

    // MARK: Documents

    /// The interview as a Texture: the radio's lines and the listener's passages in order, as a radio turn is kept.
    public func documentMarkdown(title: String) -> String {
        var out = "# \(title)\n\n"
        for t in state.turns {
            let who = t.who == "radio" ? "Radio" : "You"
            let cut = t.interrupted ? (t.who == "radio" ? " (cut off)" : " (cut in)") : ""
            if t.who == "radio", t.interrupted, let h = t.heard {
                out += "**\(who)\(cut):** \(h.isEmpty ? "(cut off before a word was heard)" : h + " …")\n\n"   // only what was heard
            } else {
                out += "**\(who)\(cut):** \(t.text)\n\n"
            }
        }
        return out
    }

    /// The prompt-bar request that has the desk write the edition, with the interview as its source.
    public func editionRequest(sourceTitle: String) -> String {
        "Write my edition from the interview \"\(sourceTitle)\": what I follow, with my own words; what I want less of; sources I trust or avoid; places and people; when and how long I listen. Keep it short and in my words."
    }

    /// A plain, deterministic edition (the offline fallback and what tests check); the desk writes the real one.
    public func editionDraft() -> [String] {
        let n = state.notes
        var out: [String] = []
        out.append(n.topics.isEmpty ? "What I follow: the whole paper, until I say otherwise." : "What I follow: " + Self.joined(n.topics) + ".")
        if !n.less.isEmpty { out.append("Less of: " + Self.joined(n.less) + ".") }
        if !n.trusted.isEmpty || !n.avoided.isEmpty {
            out.append("Sources: " + ([n.trusted.isEmpty ? "" : "I trust " + Self.joined(n.trusted)] + [n.avoided.isEmpty ? "" : "I avoid " + Self.joined(n.avoided)]).filter { !$0.isEmpty }.joined(separator: "; ") + ".")
        }
        if !n.places.isEmpty || !n.people.isEmpty { out.append("Places and people: " + Self.joined(n.places + n.people) + ".") }
        if !n.listening.isEmpty { out.append("When I listen: " + n.listening + ".") }
        return out
    }

    static func joined(_ xs: [String]) -> String {
        switch xs.count {
        case 0: return ""
        case 1: return xs[0]
        case 2: return xs[0] + " and " + xs[1]
        default: return xs.dropLast().joined(separator: ", ") + " and " + xs.last!
        }
    }
}

// MARK: The heuristic classifier (System 1's fallback, and the offline demo)

public enum InterviewHeuristic {
    static func norm(_ s: String) -> String {
        let keep = s.lowercased().replacingOccurrences(of: "\u{2019}", with: "'").map { $0.isLetter || $0.isNumber || $0 == " " || $0 == "'" || $0 == "?" ? $0 : " " }
        return String(keep).split(separator: " ").joined(separator: " ")
    }

    static let doneExact: Set<String> = ["skip", "skip it", "skip this", "skip the rest", "skip the interview", "skip all that", "skip for now", "enough", "that's enough",
                                         "that is enough", "enough for now", "that's enough for now", "later", "maybe later", "stop", "let's stop", "that's all", "i'm done", "we're done", "not now"]
    static let genericWords: Set<String> = ["everything", "news", "anything", "whatever", "stuff", "something", "all", "of", "it", "the", "just", "i", "guess", "dunno",
                                            "don't", "know", "not", "sure", "um", "uh", "like", "so", "and", "any", "kind", "sort", "general", "headlines", "honestly", "basically", "literally", "well", "really", "pretty", "much", "what's", "happening", "going", "on"]
    static let noneWords: Set<String> = ["no", "nope", "nah", "none", "nothing", "not really", "i'm good", "no thanks", "nothing really", "none really", "not that i can think of"]
    static let questionLead = ["what", "how", "why", "who", "does", "do", "is", "are", "can", "will", "could", "should", "which", "what's", "who's", "how's"]

    public static func verdict(_ words: String, topics: Bool) -> InterviewVerdict {
        let n = norm(words)
        let toks = n.split(separator: " ").map(String.init)
        if toks.isEmpty { return .wander }
        if doneExact.contains(n.replacingOccurrences(of: "?", with: "")) || n.hasPrefix("that's enough") || n.hasPrefix("that is enough") { return .done }
        if words.contains("?") || (toks.count >= 2 && questionLead.contains(toks[0])) { return .asked }
        if noneWords.contains(n) { return .refused }
        if topics {
            let content = toks.filter { !genericWords.contains($0) }
            if content.isEmpty { return .vague }
        }
        return .answered
    }

    /// An answer is specific when it names two or more things or runs to a detail.
    public static func isSpecific(_ words: String) -> Bool {
        let n = norm(words)
        let toks = n.split(separator: " ")
        if toks.count >= 6 { return true }
        return InterviewExtract.list(words).count >= 2
    }

    public static func readbackVerdict(_ words: String) -> InterviewVerdict {
        let n = norm(words)
        let yes = ["that's it", "thats it", "that is it", "that's right", "that's correct", "looks good", "sounds good", "all good", "perfect", "yes", "yep", "yeah",
                   "correct", "right", "fine", "good", "no", "nope", "nothing", "that's all", "that is all", "no changes", "nothing to change", "okay", "ok"]
        if yes.contains(n) { return .done }
        if n.hasPrefix("that's it") || n.hasPrefix("yes that") || n.hasPrefix("yeah that") || n.hasPrefix("no that's") || n.hasPrefix("no that is") || n.hasPrefix("looks good") { return .done }
        return .answered
    }
}

// MARK: Extracting notes from words

public enum InterviewExtract {
    static let leadIns = ["i would like to hear about", "i'd like to hear about", "i want to know about", "i'd like", "i would like", "i want", "i care about", "i'm interested in",
                          "i am interested in", "i like", "mostly", "mainly", "probably", "maybe", "just", "tell me about", "tell me", "about", "i guess", "well", "so", "um", "uh",
                          "i'm tired of", "i am tired of", "tired of", "sick of", "no more", "less", "i trust", "i don't trust", "i do not trust", "don't trust", "i read", "i'm in", "i am in",
                          "i live in", "i'm from", "i am from", "in", "the", "some", "any", "also", "and also", "actually", "oh", "yeah", "yes", "okay", "ok", "hmm", "sorry", "wait", "a", "an", "my"]

    /// Split an answer into items: on commas, "and", "&", ";" and "plus"; strip lead-ins and trailing punctuation.
    public static func list(_ words: String) -> [String] {
        var s = words.lowercased().replacingOccurrences(of: "’", with: "'")
        for sep in [" and also ", " as well as ", " plus ", " & ", ";", " and ", " or "] { s = s.replacingOccurrences(of: sep, with: ",") }
        var out: [String] = []
        for part in s.split(separator: ",") {
            var p = String(part).trimmingCharacters(in: CharacterSet(charactersIn: " .!?\n\t"))
            var changed = true
            while changed {
                changed = false
                for l in leadIns.sorted(by: { $0.count > $1.count }) where p == l || p.hasPrefix(l + " ") {
                    p = String(p.dropFirst(l.count)).trimmingCharacters(in: .whitespaces); changed = true; break
                }
            }
            if !p.isEmpty, !["i", "we", "and", "or", "me"].contains(p), !InterviewHeuristic.noneWords.contains(p), !out.contains(p) { out.append(p) }
        }
        return out
    }

    static func joined(_ xs: [String]) -> String { Interview.joined(xs) }
    static func isNone(_ words: String) -> Bool { InterviewHeuristic.noneWords.contains(InterviewHeuristic.norm(words)) }

    static func add(_ xs: inout [String], _ new: [String]) { for n in new where !xs.contains(n) { xs.append(n) } }

    public static func apply(question q: InterviewQuestion, to notes: inout InterviewNotes, words: String) {
        if isNone(words) { return }
        switch q.note {
        case "topics":
            if q.kind == "follow", !notes.topics.isEmpty {
                let detail = joined(list(words))
                if !detail.isEmpty { notes.topics[notes.topics.count - 1] += " (" + detail + ")" }
            } else { add(&notes.topics, list(words)) }
        case "less": add(&notes.less, list(words))
        case "sources":
            // "I trust the public radio but not the tabloids": the part after "not"/"don't" is avoided.
            let lower = words.lowercased().replacingOccurrences(of: "’", with: "'")
            for marker in [" but not ", " but don't ", " but i don't trust ", " not "] {
                if let r = lower.range(of: marker) {
                    add(&notes.trusted, list(String(lower[..<r.lowerBound])))
                    add(&notes.avoided, list(String(lower[r.upperBound...])))
                    return
                }
            }
            if lower.contains("don't trust") || lower.contains("do not trust") || lower.hasPrefix("not ") || lower.contains("avoid") { add(&notes.avoided, list(words.replacingOccurrences(of: "avoid", with: ""))) }
            else { add(&notes.trusted, list(words)) }
        case "places":
            let lower = words.lowercased()
            if let r = lower.range(of: "follow") {
                add(&notes.places, list(String(lower[..<r.lowerBound])))
                add(&notes.people, list(String(lower[r.upperBound...])))
            } else { add(&notes.places, list(words)) }
        case "listening":
            notes.listening = words.trimmingCharacters(in: CharacterSet(charactersIn: " .!?\n")).lowercased()
        default: break
        }
    }

    /// A correction at the read-back: "drop sports", "remove X", "not X" take things out; anything else is added to topics.
    public static func correct(_ notes: inout InterviewNotes, words: String) {
        let lower = words.lowercased()
        let removers = ["drop ", "remove ", "take out ", "no more ", "without ", "not "]
        for r in removers {
            if let range = lower.range(of: r) {
                let gone = list(String(lower[range.upperBound...]))
                func strip(_ xs: inout [String]) { xs.removeAll { x in gone.contains { g in x.contains(g) || g.contains(x) } } }
                strip(&notes.topics); strip(&notes.less); strip(&notes.trusted); strip(&notes.avoided); strip(&notes.places); strip(&notes.people)
                return
            }
        }
        for r in ["add ", "also ", "and "] {
            if let range = lower.range(of: r) { add(&notes.topics, list(String(lower[range.upperBound...]))); return }
        }
        add(&notes.topics, list(words))
    }
}

// MARK: System 1's typed questions for the interview

public enum InterviewQuestions {
    /// The typed questions the existing System 1 stage answers for each turn (small, closed, options with criteria).
    public static func system1(for q: InterviewQuestion, in bank: InterviewBank = .default) -> [Question] {
        let followKeys = (bank.questions.first { $0.kind == "follow" }?.follow ?? [:]).keys.filter { $0 != "*" }.sorted()
        var out = [
            Question(id: "reply_kind", text: "What did the listener do in reply to: \"\(q.kind == "follow" ? (q.follow["*"] ?? q.rephrase) : q.ask)\"?",
                     options: InterviewVerdict.allCases.map(\.rawValue),
                     criteria: ["answered": "They answered the question, even briefly.",
                                "vague": "They answered with nothing specific (everything, the news, whatever).",
                                "asked": "They asked a question of their own instead of answering.",
                                "refused": "They declined or said there is nothing.",
                                "wander": "They said something unrelated.",
                                "done": "They want to stop or skip the rest (that's enough, skip, later)."]),
        ]
        let keys = followKeys
        if q.kind != "follow", q.note == "topics", !keys.isEmpty {
            out.append(Question(id: "follow", text: "Which prepared follow-up fits what the listener named?", options: keys + ["none"],
                                criteria: Dictionary(uniqueKeysWithValues: (keys + ["none"]).map { ($0, $0 == "none" ? "None of these fits." : "The strongest topic is about \($0).") })))
        }
        return out
    }

    public static func verdict(from answers: [System1Answer]) -> (InterviewVerdict?, String?) {
        let v = answers.first { $0.questionID == "reply_kind" }.flatMap { InterviewVerdict(rawValue: $0.option) }
        let f = answers.first { $0.questionID == "follow" }.map(\.option).flatMap { $0 == "none" ? nil : $0 }
        return (v, f)
    }
}

// MARK: The session: one interview, persisted, shared by the stages and the app

/// Holds the interview, saves it after every beat, and offers `hear` to the desk stage. Nothing resumes by itself:
/// `load()` returns a paused interview for the "Continue your interview" card.
public final class InterviewSession: @unchecked Sendable {
    nonisolated(unsafe) public static var shared: InterviewSession?

    private let lock = NSLock()
    private var interview: Interview
    private let file: URL?
    private var startedAt: Double?
    public var onChange: (@Sendable () -> Void)?
    /// Typed System 1 answers for a turn; nil means the heuristic decides (System 1 slow, absent or wrong).
    public var classify: (@Sendable (String, InterviewQuestion) async -> (InterviewVerdict?, String?))?
    public var now: @Sendable () -> Double = { Date().timeIntervalSince1970 }

    public init(interview: Interview, file: URL? = nil) { self.interview = interview; self.file = file }

    public var snapshot: Interview { lock.lock(); defer { lock.unlock() }; return interview }

    /// The listener's saved interview, paused, or nil when there is none. Never resumes the audio.
    public static func load(file: URL, bank: InterviewBank = .default, sections: [String]) -> Interview? {
        guard let d = try? Data(contentsOf: file), var st = try? JSONDecoder().decode(InterviewState.self, from: d) else { return nil }
        if st.status == .running { st.status = .paused; st.pauseReason = "restart" }          // failure 11
        return Interview(bank: bank, sections: sections, state: st)
    }

    private func save() {
        guard let file else { return }
        lock.lock(); let st = interview.state; lock.unlock()
        try? FileManager.default.createDirectory(at: file.deletingLastPathComponent(), withIntermediateDirectories: true)
        if let d = try? JSONEncoder().encode(st) { try? d.write(to: file, options: .atomic) }
    }

    private func elapsedNow() -> Double { lock.lock(); defer { lock.unlock() }; return startedAt.map { now() - $0 } ?? 0 }


    public func start() -> [InterviewLine] {
        lock.lock(); startedAt = now(); let l = interview.start(); lock.unlock()
        save(); onChange?(); return l
    }

    public func resume() -> [InterviewLine] {
        lock.lock(); startedAt = now() - interview.state.elapsed; let l = interview.resume(); lock.unlock()
        save(); onChange?(); return l
    }

    /// The listener spoke. `interrupted` and `heard` come from the engine (`DeskTurn.interrupted`, `.spokenText`): it knows,
    /// from the audio actually played, whether the listener cut into a line and how much of it they had heard. Nothing here
    /// guesses from timing; no information means not interrupted.
    public func hear(_ words: String, interrupted: Bool = false, heard: String? = nil) async -> [InterviewLine] {
        let (q, st) = { () -> (InterviewQuestion?, InterviewStatus) in lock.lock(); defer { lock.unlock() }; return (interview.current, interview.status) }()
        var verdict: InterviewVerdict?, follow: String?
        if st == .running, let q, let classify { (verdict, follow) = await classify(words, q) }
        let elapsed = elapsedNow()
        lock.lock()
        interview.tick(elapsed: elapsed)
        let lines = interview.hear(words, verdict: verdict, interrupted: interrupted, followKey: follow, heard: interrupted ? heard : nil)
        lock.unlock()
        save(); onChange?(); return lines
    }

    public func silence() -> [InterviewLine] { lock.lock(); let l = interview.silence(); lock.unlock(); save(); onChange?(); return l }
    public func tooLoud() -> [InterviewLine] { lock.lock(); let l = interview.tooLoud(); lock.unlock(); save(); onChange?(); return l }
}

// MARK: Stages: the interview on the existing engine

/// The programme the engine plays: the opening and the first question as speech blocks. Later lines arrive as
/// revisions from `InterviewDesk`, the way the desk's revisions reach any programme.
public final class InterviewProgramme: Programme, @unchecked Sendable {
    public static let stageID = "interview"
    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "Interview (first run)", summary: "The question bank as the programme being played; the listener's answers come back as revisions.")
    }
    public init(_ p: StageParams) {}
    public func load() async throws -> ProgrammeDocument {
        guard let s = InterviewSession.shared else { throw LabError.unavailable("no interview session") }
        let lines = s.snapshot.status == .notStarted ? s.start() : s.resume()
        return ProgrammeDocument(id: "interview", title: "Your interview", blocks: lines.map { .speech(text: $0.text) })
    }
}

/// System 2 for the interview: a turn is answered by a revision holding the next lines, computed locally from the
/// bank and System 1's typed answer (or the heuristic). The engine's holding voice, System 1 stage and playback are
/// unchanged. The desk that writes the edition is a different call, made once at the end.
public final class InterviewDesk: Desk, @unchecked Sendable {
    public static let stageID = "interview-desk"
    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "Interview desk (first run)", summary: "Answers each turn with the interview's next lines; local, no network.")
    }
    public init(_ p: StageParams) {}
    public func submit(_ turn: DeskTurn) -> AsyncThrowingStream<Revision, Error> {
        AsyncThrowingStream { c in
            let task = Task {
                guard let s = InterviewSession.shared else { c.finish(); return }
                let lines = await s.hear(turn.listenerWords, interrupted: turn.interrupted, heard: turn.interrupted ? turn.spokenText : nil)
                if !lines.isEmpty, !Task.isCancelled {
                    c.yield(Revision(id: "interview-\(turn.turnID)", fromBlock: turn.blockIndex + 1,
                                     blocks: lines.map { .speech(text: $0.text) }, keepRest: false, isFinal: true))
                }
                c.finish()
            }
            c.onTermination = { _ in task.cancel() }
        }
    }
}

public extension StageRegistry {
    /// Register the interview's two stages (the app does this once).
    static func registerInterviewStages(_ r: StageRegistry) {
        r.register(.programme, InterviewProgramme.self) { InterviewProgramme($0) }
        r.register(.desk, InterviewDesk.self) { InterviewDesk($0) }
    }
}

// MARK: The edition: written once, at the end, by the desk

public struct Edition: Codable, Equatable, Sendable {
    public struct Section: Codable, Equatable, Sendable { public var heading: String; public var lines: [String] }
    public var title: String
    public var intro: String
    public var writingLines: [String]
    public var sections: [Section]
    /// Demo only: the first write fails, so the recovery card ("I couldn't write your edition just now") is proven.
    public var failFirst: Bool?

    public static func parse(_ data: Data) -> Edition? { try? JSONDecoder().decode(Edition.self, from: data) }
    /// Heading, then its lines, section by section: what a plain page shows.
    public var lines: [String] { sections.flatMap { [$0.heading] + $0.lines } }
}

public enum EditionError: Error, CustomStringConvertible {
    case notBuilt(String)
    case failed(String)
    public var description: String {
        switch self { case .notBuilt(let m): return m; case .failed(let m): return m }
    }
}

public protocol EditionWriter: Sendable {
    /// One prompt-bar request focused on a new document "My edition", with the interview as its source. `progress`
    /// receives what the desk is doing, in plain words, for the alive wait.
    func write(from interview: Interview, interviewTitle: String, progress: @escaping @Sendable (String) -> Void) async throws -> Edition
}

/// Demo and simulator: the edition comes from a fixture after a paced sequence of lifecycle lines.
public final class DemoEditionWriter: EditionWriter, @unchecked Sendable {
    private let edition: Edition
    private let tickMs: Int
    private let lock = NSLock()
    private var failed = false
    public init(edition: Edition, tickMs: Int = 800) { self.edition = edition; self.tickMs = tickMs }
    public func write(from interview: Interview, interviewTitle: String, progress: @escaping @Sendable (String) -> Void) async throws -> Edition {
        for l in edition.writingLines {
            progress(l)
            try await Task.sleep(nanoseconds: UInt64(tickMs) * 1_000_000)
        }
        lock.lock()
        let shouldFail = edition.failFirst == true && !failed
        if shouldFail { failed = true }
        lock.unlock()
        if shouldFail { throw EditionError.failed("The desk did not answer.") }       // failure 13
        return edition
    }
}

/// The live write: `POST /api/prompt-bar` focused on a new Texture titled "My edition" (the route ChoirDeskStage already
/// uses for later turns). Not built here: the request shape for a new focused document and the edition marker the server
/// recognises do not exist yet (plan section 2, "What the server lacks"). It fails honestly, and the card offers Retry.
public struct UnbuiltEditionWriter: EditionWriter {
    public init() {}
    public func write(from interview: Interview, interviewTitle: String, progress: @escaping @Sendable (String) -> Void) async throws -> Edition {
        throw EditionError.notBuilt("Writing the edition on your computer is not connected in this build.")
    }
}

// MARK: The default bank, as compiled into the app (a test keeps it equal to mobile/interview/questions.md)

extension InterviewBank {
    public static let defaultText = #"""
# The interview bank (default wording)

This file is the default question bank for the first-run interview (plan: docs/design/autoradio-app-ui-2026-11-10.md,
section 2). It is plain text on purpose: the desk's package will own the live copy, so the wording can be revised
without an app release. `Interview.swift` parses it (`InterviewBank.parse`); a test keeps the copy compiled into the
app (`InterviewBank.defaultText`) identical to this file.

Format: `## id` starts an entry; `key: value` lines belong to it. `{sections}`, `{noted}`, `{topic}` and `{echo}` are
filled in. Entries starting with `@` are not questions.

- `ask` is the question as first said. `rephrase` is said once if the listener did not understand, then the question
  is skipped. `vague` is the line for an answer like "everything" or "the news".
- `aside` is the one sentence, said in passing after the answer, that tells how Choir works. Never a lecture.
- `follow.<word>` is a prepared follow-up for the topics question, chosen by the first word it finds in the answer
  (System 1 can choose by `follow` instead); `follow.*` is the default.
- `kind: follow` marks the second question; it is skipped when the first answer was already specific.

## topics
note: topics
ask: What do you want to know about every day?
rephrase: Tell me one or two things you would like your paper to cover for you.
vague: Let me help. Today's paper has {sections}. Which of those, or something else?
aside: This becomes your edition: a document on your own computer, not a chat. You can read it, change it, or ask me to.

## follow
kind: follow
note: topics
rephrase: Which part of that matters most to you?
follow.tech: When you say tech, is that the companies, the science, or what it does to work?
follow.ai: When you say AI, is that the companies, the research, or what it does to work?
follow.local: When you say local, is that the council, the schools, or what is happening on your street?
follow.world: When you say the world, is that wars and politics, the economy, or something else?
follow.sport: When you say sports, is that results, the business of it, or one team in particular?
follow.*: Which part of that matters most to you?

## tired
note: less
ask: Anything you are tired of hearing about?
rephrase: Is there a subject you would like to hear less of?
aside: The paper learns from what you do. Swipe a story right for more like it, and skip what you do not want.

## sources
note: sources
ask: Any sources you trust, or do not?
rephrase: Are there newspapers, writers or sites you rely on, or avoid?
aside: Every story cites its sources. Tap a number to see where a sentence came from.

## places
note: places
ask: Where are you, and are there places or people you follow?
rephrase: Which city are you in, and is there anyone you want me to keep an eye on?

## listening
note: listening
ask: When do you usually listen, and for how long?
rephrase: Is it mornings, the commute or the evening, and for about how long?
aside: I keep what I learn in documents you can read. Nothing is hidden in a chat log.

## @say
open: Let's make this yours.
ack: Okay.
noted: Got it: {noted}.
noted_interrupt: Right: {noted}. Let's move on.
nothing_noted: Okay, nothing to cut.
interrupt_answer: Mm, {noted}. Skipping ahead.
interrupt_other: Sure. Let me put it another way.
rephrase_lead: Let me put that another way.
skip_lead: No problem, we will leave that one.
whole_paper: Then I will start you with the whole paper and learn as we go.
silence1: Take your time. Or say skip.
silence_pause: I will be here when you are ready.
loud: It is too loud here for me to hear you well. Try headphones, or come back when it is quiet.
resume: Welcome back.
time_up: That is plenty for now.
readback_intro: Here is what I have.
readback_ask: Anything to change?
readback_empty: Not much yet, so I will start you with the whole paper.
readback_again: Okay, I changed that.
closing: Good. Writing your edition now.
closing_limit: Okay, I will go with that. You can change it in the document, just ask.
ask_default: I can answer that in a sentence. Your edition is a document on your own computer that you and I both edit.

## @faq
faq: who|what are you|what is this => I am the radio on your own computer. I read you the paper and I change it when you ask.
faq: private|privacy|listening|record => Your voice stays on this phone as sound. Only the words go to your computer.
faq: why|how come => So the paper can start with what you care about, not what everyone else does.
faq: how|work => You talk, I answer, and a writer on your computer revises the document. You can read every change.
"""#
}
