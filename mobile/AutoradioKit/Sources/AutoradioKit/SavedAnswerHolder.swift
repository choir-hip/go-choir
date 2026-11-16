import Foundation

// The phone's first answer, from what it already holds. While the desk's answer is on its way, this holder answers in two or three
// sentences from the reader's saved stories (SavedCorpus) using the small on-device model that is already downloaded, and says which story
// it spoke from. It never answers from the model's own knowledge: the model sees only the passages and is told to write NO_ANSWER when they
// do not hold the answer; with no passage that clears the relevance floor the model is not asked at all and the holding line says what is
// being looked up.
//
// WAYS THIS CAN FAIL (written before the code; each has a test in SavedAnswerTests):
//  1. No passage clears the floor: the model is NOT called; one honest holding line is spoken; no invented answer.
//  2. The model says NO_ANSWER (the passages touch the words but do not answer): the holding line, nothing else.
//  3. The model is not downloaded or not loaded, or fails: the holding line only (no error shown); the desk's answer is the whole answer.
//  4. The model rambles past three sentences: only the first three are spoken.
//  5. The first answer is spoken and the desk's answer arrives first: the desk speaks after the first answer finishes
//     (`keepsSpeakingForDesk`), it does not cut it mid-answer; the first answer is cut only if the listener cancels.
//  6. Cancelled while composing: nothing more is spoken and the entry is not marked as answered.
//  7. A question made only of stop words ("what about it"): no passage, so the holding line.
//  8. Words with quotes or newlines cannot break the prompt (flattened).

public enum SavedAnswerPrompt {
    public static let maxSentences = 3
    public static let noAnswer = "NO_ANSWER"
    public static func system() -> String {
        "You answer a listener's question using ONLY the numbered passages you are given, which come from stories saved on their phone. "
            + "Never use anything you know that is not written in the passages, and never guess. "
            + "If the passages do not answer the question, write exactly \(noAnswer) and nothing else. "
            + "Otherwise answer in two or three short plain sentences that a voice can say, the most important first, each making sense on its own. "
            + "No lists, no markup, no quotes, no emoji."
    }
    public static func prefix(_ hits: [PassageHit]) -> String {
        hits.prefix(3).enumerated().map { i, h in
            "Passage \(i + 1) (from \"\(AckPrompt.flat(h.passage.title))\"): \(AckPrompt.flat(String(h.passage.text.prefix(700))))"
        }.joined(separator: "\n") + "\n"
    }
    public static func suffix(_ question: String) -> String { "Question: \"\(AckPrompt.flat(question))\"\nAnswer:" }

    /// "From the paper's story on X" for a passage; "From this page" for the page in front.
    public static func source(_ h: PassageHit) -> String {
        if h.passage.docID.hasPrefix("page:") { return "From this page" }
        if let t = AckPrompt.usableTitle(h.passage.title) { return "From the paper's story on \(String(t.prefix(90)))" }
        return "From a story you saved"
    }

    /// The honest line when nothing in the saved stories answers: it says what is being looked up and claims nothing.
    public static func holdingLine(_ question: String) -> String {
        let words = question.split(whereSeparator: \.isWhitespace).prefix(10).joined(separator: " ")
            .trimmingCharacters(in: CharacterSet(charactersIn: " ?!.,;:\"'"))
        return words.isEmpty ? "Let me look that up." : "Let me look that up: \(words)."
    }
}

public final class SavedAnswerHolder: Holder, @unchecked Sendable {
    public static let stageID = "saved-answer"
    public static var descriptor: StageDescriptor {
        StageDescriptor(
            displayName: "First answer from your saved stories",
            summary: "While your computer works on the full answer, the phone answers first from the stories it already holds, naming the story, using the small on-device model. With nothing relevant it says what it is looking up and no more.",
            params: [.number("temperature", "Variety", 0.2, 0, 1.5)])
    }
    let temperature: Double
    let corpus: SavedCorpus
    let modelSource: @Sendable () -> (any LocalLanguageModel)?
    private let lock = NSLock()
    private var task: Task<Void, Never>?
    private var note = ""
    private var answering = false
    private var generation: GenerationReport?

    public init(_ p: StageParams, corpus: SavedCorpus = .shared, model: @escaping @Sendable () -> (any LocalLanguageModel)? = { LocalLanguage.model }) {
        temperature = p.double("temperature", 0.2); self.corpus = corpus; modelSource = model
    }

    public var keepsSpeakingForDesk: Bool { lock.lock(); defer { lock.unlock() }; return answering }
    public func diagnostics() -> String { lock.lock(); defer { lock.unlock() }; return note }
    public func report() -> GenerationReport? { lock.lock(); defer { lock.unlock() }; return generation }
    private func say(_ s: String) { lock.lock(); note = s; lock.unlock() }
    public func cancel() { lock.lock(); let t = task; lock.unlock(); t?.cancel() }

    public func lines(for i: HolderInput) -> AsyncStream<String> {
        let words = i.listenerWords.trimmingCharacters(in: .whitespacesAndNewlines)
        guard !words.isEmpty else { say("no words, no call"); return AsyncStream { $0.finish() } }
        lock.lock(); answering = false; lock.unlock()
        let model = modelSource(), temperature = temperature, corpus = corpus
        return AsyncStream { c in
            let t = Task { [self] in
                defer { c.finish() }
                if let f = i.pageText { corpus.offer(address: i.pageAddress, title: i.pageTitle, text: await f()) }
                if Task.isCancelled { say("cancelled"); return }
                let hits = await corpus.search(words, pageAddress: i.pageAddress)
                if Task.isCancelled { say("cancelled"); return }
                func holding(_ why: String) { say(why); c.yield(SavedAnswerPrompt.holdingLine(words)) }
                guard !hits.isEmpty else { return holding("nothing in the saved stories answers this; the model was not asked") }
                guard let m = model, m.isLoaded else { return holding("the local model is not loaded; found \(hits.count) passages") }
                m.willGenerate(); defer { m.generationEnded() }
                var splitter = SentenceSplitter()
                var spoken = 0, first = true
                let source = SavedAnswerPrompt.source(hits[0])
                var refused = false
                func offer(_ raw: String) {
                    guard spoken < SavedAnswerPrompt.maxSentences, !refused, let s = AckPrompt.speakable(raw) else { return }
                    if s.contains(SavedAnswerPrompt.noAnswer) { refused = true; return }
                    let line = spoken == 0 ? "\(source): \(s)" : s
                    spoken += 1
                    lock.lock(); answering = true; lock.unlock()
                    Conversation.shared.setFirst(text: s, source: source)
                    c.yield(line)
                }
                do {
                    for try await piece in m.stream(system: SavedAnswerPrompt.system(), userPrefix: SavedAnswerPrompt.prefix(hits), userSuffix: SavedAnswerPrompt.suffix(words), temperature: temperature) {
                        if Task.isCancelled || refused || spoken >= SavedAnswerPrompt.maxSentences { break }
                        if first { first = false; let r = m.lastReport(); lock.lock(); generation = r; lock.unlock() }
                        for s in splitter.push(piece) { offer(s) }
                    }
                } catch { if Task.isCancelled { say("cancelled"); return }; if spoken == 0 { return holding("the model failed: \(error)") } }
                if Task.isCancelled { say("cancelled"); return }
                for s in splitter.finish() { offer(s) }
                if spoken == 0 { return holding(refused ? "the passages do not answer the question (the model said so)" : "the model wrote nothing") }
                say("\(spoken) sentences from \"\(hits[0].passage.title)\"")
            }
            self.lock.lock(); self.task = t; self.lock.unlock()
            c.onTermination = { _ in t.cancel() }
        }
    }
}
