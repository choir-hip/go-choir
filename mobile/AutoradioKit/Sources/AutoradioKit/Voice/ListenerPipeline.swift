import Foundation

// WHAT THE PHONE LISTENS WITH BY DEFAULT, chosen from the model list.
//
// Owner, 2026-11-23/24: the phone deciding whether I am speaking "is a loudness detector? wtf? Don't we have a local model for this?"; "The speech
// recognizer isn't supposed to be Apple's"; "we are only using that currently because of a coding bug. (we shuold be using fluid speech mini
// etc)"; "we will keep updating the models as better ones come". The listener configuration used to be a fixed list written in the app's first
// commit (an energy detector and Apple's recogniser). It is now built from the model list (`CandidateTable`, ModelCandidates.swift): for each
// job the best-ranked model that is on the phone and was heard to work (`Voices.readiness`, written by voice setup after the person agreed to the
// download) and whose stage this build has. A better model later is a new row in the list with a higher rank, not a change here.
//
//   speech detection   the best `vad` candidate that is ready (Silero), else the level detector built in
//   live words         the best ready transcriber that streams partial words (Parakeet end-of-utterance): they drive turn-taking while the
//                      listener speaks (cutting in, the turn's silence) and are never the transcript when a better final one exists
//   final transcript   the best-ranked ready transcriber of all (Fluid Speech Mini), one pass over the speech-bounded audio
//   speaking           the saved or default voice, already neural-with-system-fallback (`makeSpeaker`); not chosen here
//
// WHAT RUNS BEFORE THE MODELS ARE ON THE PHONE (stated, owner: Apple's recogniser may remain only as that fallback): nothing downloaded, so the
// level detector and Apple's recogniser, exactly what ran before. Part of the models present: each job takes its best ready model and Apple's
// recogniser covers only a job no model of ours is ready for; when a final model is ready but no streaming one is, Apple's recogniser gives the
// live words only (they steer turn-taking; the transcript is the final model's).
//
// WAYS THIS CAN FAIL (each has a test in ListenerPipelineTests):
//  1. Nothing is ready (a fresh install, or the person declined the download): the floors, the same as before: the app listens and answers.
//  2. A ready model's stage is not in this build (a build without the model libraries): that model is skipped, never named in the configuration.
//  3. The list changes (a new row with a higher rank): the choice follows it with no code change.
//  4. Only a streaming model is ready (no final model): it is both the live words and the transcript, with no wrapper.
//  5. A model is ready by the record but its files went (storage purged): the stage fails at `begin`; the composite falls back (below) and
//     the engine falls back to the floors if even that cannot make a stage (HandsFree `hfPrepare`).

public enum ListenerPipeline {
    /// The one stage that holds two recognisers: live words from one, the final transcript from the other.
    public static let liveThenFinalStageID = "live-then-final"
    /// Apple's recogniser: the floor, built in, needs no download.
    public static let floorTranscriber = "apple-speech"
    public static let floorBounds = "energy-vad"

    /// Settings for the stages that need parameters (not model choices): the level detector's hangover, and Silero's silence before it calls speech over (the same quarter second).
    static let boundsParams: [String: [String: JSONValue]] = [
        "energy-vad": ["hangoverSeconds": .number(0.25)],
        "silero-vad": ["minSilenceSeconds": .number(0.25)],
    ]

    public struct Choice: Equatable, Sendable {
        public var bounds: StageSetting
        public var transcriber: StageSetting
        /// The model ids chosen, by job ("bounds", "live", "final"); a job with no model of ours has no entry.
        public var models: [String: String]
        /// Plain words saying what runs, for the log.
        public var summary: String
    }

    /// `isReady(modelID)`: on the phone and heard to work. `hasStage(kind, stageID)`: this build registered it.
    public static func choose(table: [ModelCandidate] = CandidateTable.all,
                              isReady: (String) -> Bool, hasStage: (StageKind, String) -> Bool) -> Choice {
        func best(_ role: ConfigRole, _ kind: StageKind, where ok: (ModelCandidate) -> Bool = { _ in true }) -> ModelCandidate? {
            CandidateTable.candidates(role, in: table).first { !$0.isFloor && ok($0) && isReady($0.id) && hasStage(kind, $0.kitStageID) }
        }
        var models: [String: String] = [:]
        // Speech detection.
        let vad = best(.vad, .speechBounds)
        let boundsID = vad?.kitStageID ?? floorBounds
        if let vad { models["bounds"] = vad.id }
        let bounds = StageSetting(id: boundsID, params: boundsParams[boundsID] ?? [:])
        // Recognition.
        let final = best(.transcriber, .transcriber)
        let live = best(.transcriber, .transcriber) { $0.streamsWords }
        var transcriber = StageSetting(id: floorTranscriber)
        var said = "Apple's recogniser (no listening model is on this phone yet)"
        if let final {
            if let live, live.id == final.id {
                transcriber = StageSetting(id: final.kitStageID); models["live"] = live.id; models["final"] = final.id
                said = "\(final.label) for live words and the transcript"
            } else {
                let liveStage = live?.kitStageID ?? floorTranscriber
                transcriber = StageSetting(id: liveThenFinalStageID, params: ["live": .string(liveStage), "final": .string(final.kitStageID)])
                models["final"] = final.id
                if let live { models["live"] = live.id }
                said = "\(live?.label ?? "Apple's recogniser") for live words, \(final.label) for the transcript"
            }
        } else if let live {
            transcriber = StageSetting(id: live.kitStageID); models["live"] = live.id
            said = "\(live.label) for live words and the transcript"
        }
        let summary = "listening: \(vad?.label ?? "level detector built in") hears speech; \(said)"
        return Choice(bounds: bounds, transcriber: transcriber, models: models, summary: summary)
    }

    /// The choice for a running app: what is ready on the phone (`readiness`) and registered in this build's `registry`.
    public static func choose(registry: StageRegistry?, readiness: VoiceReadiness) -> Choice {
        choose(isReady: { readiness.isReady(model: $0) }, hasStage: { kind, id in registry?.stages(kind).contains { $0.id == id } ?? false })
    }

    /// What a configuration's listening stages are, in plain words (written to the trace when a voice session starts, so a run says which recogniser it used).
    public static func summary(of c: LabConfiguration) -> String {
        func name(_ s: StageSetting?) -> String {
            guard let s else { return "none" }
            if s.id == liveThenFinalStageID {
                if case .string(let l)? = s.params["live"], case .string(let f)? = s.params["final"] { return "live words \(l), transcript \(f)" }
                return "live words and a final transcript (stage defaults)"
            }
            return s.id
        }
        return "listening with speech detection \(name(c.setting(.speechBounds))); recognition \(name(c.setting(.transcriber)))"
    }

    /// The models voice setup fetches along with a chosen one, because the pipeline needs them together (the live words beside the final
    /// transcript): the chosen model's own file is fetched first, these after it. Read from the list, never written in the setup code.
    public static func companions(of id: String, table: [ModelCandidate] = CandidateTable.all) -> [String] {
        guard let c = table.first(where: { $0.id == id && $0.role == .transcriber }), !c.streamsWords, !c.isFloor else { return [] }
        // A final-only model (batch) gets the best streaming model beside it.
        return CandidateTable.candidates(.transcriber, in: table).filter { $0.streamsWords && !$0.isFloor }.prefix(1).map(\.id)
    }
}

// MARK: The stage that runs the two together

/// Live words from one recogniser, the final transcript from another. Both hear every frame; the live one's partials are the session's partials
/// (they steer turn-taking while the listener speaks), and `finish()` is the final one's single pass over the whole turn. Only models that are
/// on the phone and ready are started (this stage never downloads): a model that is not ready is skipped, and a final recogniser that fails gives
/// way to the live one's own text, so a turn is never lost to a missing file.
///
/// WAYS THIS CAN FAIL (tests in ListenerPipelineTests):
///  1. The final model is missing or throws at `begin`: the live recogniser's own transcript is the answer (and says so in `notes`).
///  2. The final model throws at `finish`: the same.
///  3. The live model is missing: Apple's recogniser gives the live words; with that missing too, the session has no partials and the silence
///     alone ends the turn (HandsFree H1).
///  4. Neither starts: `begin` throws, and the engine goes on with a turn that has no transcript (TurnEngine), as before.
public final class LiveThenFinalTranscriber: Transcriber, @unchecked Sendable {
    public static let stageID = ListenerPipeline.liveThenFinalStageID
    public static var descriptor: StageDescriptor {
        StageDescriptor(
            displayName: "Live words, then the final transcript",
            summary: "Live words from one recogniser while the listener speaks; the final transcript from a second, in one pass over the turn.",
            params: [.text("live", "Live words (a recogniser stage id)", ""), .text("final", "Final transcript (a recogniser stage id)", "")],
            needsModels: [])
    }
    private let live: String, final: String
    /// Held strongly: the registry holds makers, never stages, so there is no cycle, and a stage must outlive the caller's reference to the registry.
    private let registry: StageRegistry?
    private let readiness: VoiceReadiness
    /// What the last `begin` ran, in plain words (for the trace).
    public private(set) var notes: [String] = []
    public init(_ p: StageParams, registry: StageRegistry?, readiness: VoiceReadiness = Voices.readiness) {
        live = p.string("live", ""); final = p.string("final", "")   // no model is named here: `ListenerPipeline.choose` fills them from the list
        self.registry = registry; self.readiness = readiness
    }

    /// The stage's session if its models are ready and it starts; nil (with the reason in `why`) otherwise.
    private func start(_ id: String, _ why: inout [String]) async -> TranscriptionSession? {
        guard id != Self.stageID, let registry, let reg = registry.stages(.transcriber).first(where: { $0.id == id }) else { why.append("\(id) is not in this build"); return nil }
        let missing = reg.descriptor.needsModels.filter { !readiness.isReady(model: $0) }
        guard missing.isEmpty else { why.append("\(id) is not on this phone yet"); return nil }
        do {
            let stage = try registry.make(.transcriber, as: Transcriber.self, LabConfiguration(name: "live-then-final", stages: [.transcriber: StageSetting(id: id)]))
            return try await stage.begin()
        } catch { why.append("\(id) could not start (\(error))"); return nil }
    }

    public func begin() async throws -> TranscriptionSession {
        var why: [String] = []
        var liveSession = await start(live, &why)
        if liveSession == nil, live != ListenerPipeline.floorTranscriber { liveSession = await start(ListenerPipeline.floorTranscriber, &why) }
        let finalSession = final == live ? nil : await start(final, &why)
        notes = why
        guard liveSession != nil || finalSession != nil else { throw LabError.unavailable("no recogniser could start: " + why.joined(separator: "; ")) }
        return Session(live: liveSession, final: finalSession)
    }

    final class Session: TranscriptionSession, @unchecked Sendable {
        let live: TranscriptionSession?, final: TranscriptionSession?
        let partials: AsyncStream<String>
        init(live: TranscriptionSession?, final: TranscriptionSession?) {
            self.live = live; self.final = final
            partials = live?.partials ?? AsyncStream<String> { $0.finish() }
        }
        func append(_ frame: AudioFrame) { live?.append(frame); final?.append(frame) }
        func finish() async throws -> String {
            // The live session is always finished, never cancelled: its worker is joined before the next turn reuses the shared manager (S3).
            if let final {
                if let text = try? await final.finish() {
                    // An empty final transcript is not an answer when the live recogniser heard words (S4).
                    let liveText = (try? await live?.finish()) ?? ""
                    return text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty ? liveText : text
                }
                // the final recogniser failed: the live one's own transcript is the answer
            }
            guard let live else { throw LabError.unavailable("the final recogniser failed and no live one ran") }
            return try await live.finish()
        }
        func cancel() { live?.cancel(); final?.cancel() }
    }
}

public func registerListenerStages(_ r: StageRegistry) {
    r.register(.transcriber, id: LiveThenFinalTranscriber.stageID, descriptor: LiveThenFinalTranscriber.descriptor) { [weak r] p in
        LiveThenFinalTranscriber(p, registry: r)
    }
}
