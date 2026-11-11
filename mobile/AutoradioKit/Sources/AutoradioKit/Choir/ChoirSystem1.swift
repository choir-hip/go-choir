import Foundation

/// System 1 on the listener's own computer: `POST /api/system1/judgments`.
/// One request per turn carries the shared state and all of the turn's questions (owner rule: one call
/// asks many questions; never concurrent calls). A 429 fails the stage for this turn; there is no retry
/// and no client-side limiter.
public final class ChoirSystem1: System1, @unchecked Sendable {
    public static let stageID = "choir-system1"
    public static var descriptor: StageDescriptor {
        StageDescriptor(displayName: "Choir (Jev on your computer)",
                        summary: "One call per turn to your computer's System 1 route with every question; needs sign-in.",
                        params: [.text("baseURL", "Base URL", ChoirEnvironment.defaultBaseURL),
                                 .number("timeoutSeconds", "Timeout (s)", 5, 0.5, 30),
                                 .text("label", "Record label", "autoradio", note: "recorded as the Jev site client:<label>")],
                        needsNetwork: true)
    }

    // Route limits (internal/agentcore/api_system1.go).
    static let maxQuestions = 12, maxChoices = 8, maxInstructions = 2000, maxState = 48 << 10, maxName = 64

    let baseURL: String, timeout: Double, label: String
    public init(_ p: StageParams) {
        baseURL = p.string("baseURL", ChoirEnvironment.defaultBaseURL)
        timeout = p.double("timeoutSeconds", 5)
        let l = p.string("label", "autoradio")
        label = l.range(of: "^[a-z0-9_.-]{1,40}$", options: .regularExpression) != nil ? l : "autoradio"
    }

    /// One call at a time, across all instances: the engine makes a stage per turn.
    private static let gate = NSLock()
    nonisolated(unsafe) private static var inFlight = false
    private static func enter() -> Bool { gate.lock(); defer { gate.unlock() }; if inFlight { return false }; inFlight = true; return true }
    private static func leave() { gate.lock(); inFlight = false; gate.unlock() }

    /// The request body for a turn. Public so tests (and the README) can show exactly what is sent.
    public static func body(for i: System1Input, label: String) throws -> (body: [String: Any], names: [String]) {
        var context = i.context
        var state: [String: Any] = ["context": context, "listener_words": i.listenerWords]
        // Clip the context, never the listener's words, until the state fits.
        while let d = try? JSONSerialization.data(withJSONObject: state), d.count > maxState - 512, context.count > 200 {
            context = String(context.suffix(context.count / 2))
            state["context"] = context
        }
        var qs: [String: Any] = [:]
        var names: [String] = []
        for q in i.questions.prefix(maxQuestions) {
            let name = String(q.id.prefix(maxName))
            let choices = Array(q.options.prefix(maxChoices))
            guard choices.count >= 2, !name.isEmpty, qs[name] == nil else { continue }
            var ins = "The state holds what the radio is playing (context) and what the listener just said (listener_words). " + q.text
            if ins.utf8.count > maxInstructions { ins = String(decoding: Array(ins.utf8.prefix(maxInstructions)), as: UTF8.self) }
            // The route requires `criteria` to name exactly the choices (choice -> one line on when it is right).
            var crit: [String: String] = [:]
            for c in choices { crit[c] = String((q.criteria?[c] ?? "The answer to the question is \"\(c)\".").prefix(300)) }
            qs[name] = ["type": "choice", "instructions": ins, "choices": choices, "criteria": crit]
            names.append(name)
        }
        if names.isEmpty { throw LabError.badInput("no usable System 1 question (a question needs an id and 2 to \(maxChoices) options)") }
        return (["state": state, "questions": qs, "label": label], names)
    }

    public func answer(_ i: System1Input) async throws -> [System1Answer] {
        let (body, names) = try Self.body(for: i, label: label)
        guard Self.enter() else { throw ChoirError.busy("a System 1 call is already in flight") }
        defer { Self.leave() }
        let env = ChoirEnvironment.shared
        let client = env.client(baseURL: baseURL, timeout: timeout)
        let r = try await client.postJSON("/api/system1/judgments", body, route: "/api/system1/judgments", timeout: timeout)
        let o = try r.object()
        guard let answers = o["answers"] as? [String: Any] else { throw ChoirError.decode("no answers") }
        var out: [System1Answer] = []
        var dropped: [String] = []
        for n in names {
            guard let a = answers[n] as? [String: Any], a["error"] == nil, let choice = a["choice"] as? String else { dropped.append(n); continue }
            let conf = (a["confidence"] as? Double) ?? ((a["probabilities"] as? [String: Double])?[choice] ?? 0)
            out.append(System1Answer(questionID: n, option: choice, score: conf, ms: r.ms))
        }
        let up = (o["upstream_ms"] as? Double).map { String(format: "%.0f", $0) } ?? "-"
        let rec = (o["record_id"] as? String) ?? "-"
        env.log?.emit(.note("choir system1: \(names.count) questions in one call, \(out.count) answered, \(String(format: "%.0f", r.ms)) ms (upstream \(up) ms), record \(rec)"
                            + (dropped.isEmpty ? "" : ", no answer for \(dropped.joined(separator: ","))")))
        if out.isEmpty { throw ChoirError.decode("the judge answered none of \(names.count) questions") }
        return out
    }
}
