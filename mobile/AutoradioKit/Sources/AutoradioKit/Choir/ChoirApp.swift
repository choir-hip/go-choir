import Foundation

extension StageRegistry {
    public static func registerChoirStages(_ r: StageRegistry) {
        r.register(.system1, ChoirSystem1.self) { ChoirSystem1($0) }
        r.register(.programme, ChoirFrontProgramme.self) { ChoirFrontProgramme($0) }
        r.register(.desk, ChoirDeskStage.self) { ChoirDeskStage($0) }
    }

    /// What the listener's app runs: the real programme, System 1 and desk on the listener's computer, and
    /// the best Apple stages for the rest. Stubs and fixtures stay registered and selectable in the Lab tab.
    public static func listenerConfiguration() -> LabConfiguration {
        LabConfiguration(name: "listener", stages: [
            .micSource: .init(id: "builtin"), .addressing: .init(id: "button"), .speechBounds: .init(id: "energy-vad"),
            .speakerCheck: .init(id: "none"), .transcriber: .init(id: "apple-speech"), .yieldPolicy: .init(id: "dip-then-pause"),
            .system1: .init(id: ChoirSystem1.stageID), .holder: .init(id: "local-ack"), .voice: .init(id: "avspeech"),
            .desk: .init(id: ChoirDeskStage.stageID), .programme: .init(id: ChoirFrontProgramme.stageID),
        ])
    }
}
func registerChoirStages(_ r: StageRegistry) { StageRegistry.registerChoirStages(r) }

// MARK: Views for the host (plain value types)

public struct ChoirPlayerView: Codable, Sendable {
    public var signedIn = false
    public var loading = false
    public var title = ""
    public var passage = ""
    public var position = ""        // "3 of 12"
    public var playing = false
    public var talking = false      // the listener's turn is in progress
    public var heard = ""           // what the radio heard, while talking
    public var message = ""         // plain words about anything wrong
}
public struct StoryRow: Codable, Sendable { public var title, dek, route, section: String; public var current: Bool }

/// The listener-facing controller: sign-in, the story list, loading the programme, plain-words status.
public final class ChoirApp: @unchecked Sendable {
    public let runtime: LabRuntime
    public let signIn: ChoirSignIn
    private let lock = NSLock()
    private var stories: [ChoirStory] = []
    private var message = ""
    private var loading = false
    private var currentRoute = ""
    private var eventTask: Task<Void, Never>?
    public var onChange: (@Sendable () -> Void)?

    public init(runtime: LabRuntime, keys: KeyStore, baseURL: String = ChoirEnvironment.defaultBaseURL, openURL: (@Sendable (URL) -> Void)?) {
        self.runtime = runtime
        ChoirEnvironment.shared.keys = keys
        ChoirEnvironment.shared.log = runtime.log
        signIn = ChoirSignIn(baseURL: baseURL)
        signIn.openURL = openURL
        signIn.onChange = { [weak self] in
            guard let self else { return }
            if self.signIn.current.status == "idle", self.signIn.current.signedIn, self.signIn.current.message == "signed in" { Task { await self.refresh() } }
            self.onChange?()
        }
        eventTask = Task { [weak self] in
            guard let self else { return }
            for await te in self.runtime.log.stream() {
                if case .error(let stage, let m) = te.event, stage == "system1" || stage == "desk" || stage == "programme" {
                    self.say(Self.plain(stage: stage, message: m))
                }
            }
        }
    }

    private func say(_ m: String) { lock.lock(); message = m; lock.unlock(); onChange?() }

    /// Plain words for the listener; the details stay in the Trace.
    static func plain(stage: String, message m: String) -> String {
        if m.contains("not signed in") { return "Sign in to talk to your computer." }
        if m.contains("401") { return "Your sign-in is no longer accepted. Please sign in again." }
        if m.contains("429") { return "Your computer is busy right now. Try again in a moment." }
        if m.contains("network") || m.contains("timed out") { return "Could not reach choir.news. Check your connection." }
        switch stage {
        case "system1": return "Your computer was slow to understand that; carrying on."
        case "desk": return "Your computer could not act on that. Try asking again."
        default: return "Something went wrong loading the radio."
        }
    }

    /// First run after launch or sign-in: load the story list and the programme.
    public func refresh() async {
        guard signIn.current.signedIn else { onChange?(); return }
        await ChoirEnvironment.shared.warm(baseURL: signIn.baseURL)
        await loadStories()
        await loadProgramme()
    }

    public func loadStories() async {
        let cfg = runtime.configuration.setting(.programme)
        let base = cfg?.params["baseURL"].map { "\($0)" } ?? signIn.baseURL
        do {
            let list = try await ChoirFront.stories(client: ChoirEnvironment.shared.client(baseURL: base), limit: 30)
            lock.lock(); stories = list; lock.unlock()
            if list.isEmpty { say("The front page is empty right now.") }
        } catch { say(Self.plain(stage: "programme", message: "\(error)")) }
        onChange?()
    }

    /// Load the configured programme stage into the player (the real front page, or a fixture picked in the Lab).
    public func loadProgramme() async {
        lock.lock(); loading = true; message = ""; lock.unlock(); onChange?()
        do {
            let prog = try runtime.registry.make(.programme, as: Programme.self, runtime.configuration)
            let doc = try await prog.load()
            runtime.player.setDocument(doc)
            runtime.player.start(at: 0)
            lock.lock(); currentRoute = ChoirEnvironment.shared.playing(doc.id)?.route ?? ""; lock.unlock()
        } catch is CancellationError {
        } catch {
            runtime.player.setDocument(ProgrammeDocument(id: "none", title: "", blocks: []))
            runtime.log.emit(.error(stage: "programme", message: "\(error)"))
            if case ChoirError.empty = error { say("There is nothing to play right now.") }
        }
        lock.lock(); loading = false; lock.unlock(); onChange?()
    }

    public func play(route: String) async {
        runtime.setStage(.programme, id: ChoirFrontProgramme.stageID)
        runtime.setParam(.programme, name: "route", value: route)
        await loadProgramme()
        await startPlaying()
    }

    /// Play/pause from the big button: starts the audio engine on first use.
    public func playPause() async {
        if !runtime.snapshot().running { await runtime.start() }
        if runtime.player.userPaused { runtime.player.play() } else { runtime.player.pause() }
        onChange?()
    }
    /// Start the radio and leave it playing, whatever it was doing (a toggle would pause it if the interview had left it
    /// running): the setup's "Start the radio". A programme is loaded first when none is.
    public func startRadio() async {
        if !runtime.snapshot().running { await runtime.start() }
        if runtime.player.document.blocks.isEmpty { await loadProgramme() }
        runtime.player.play()
        onChange?()
    }
    public func startPlaying() async {
        if !runtime.snapshot().running { await runtime.start() }
        runtime.player.play()
        onChange?()
    }
    public func talkDown() async { if !runtime.snapshot().running { await runtime.start() }; runtime.talkDown() }

    public func signOut() async {
        await signIn.signOut()
        runtime.player.pause()
        runtime.player.setDocument(ProgrammeDocument(id: "none", title: "", blocks: []))
        lock.lock(); stories = []; message = ""; lock.unlock()
        onChange?()
    }

    public func playerView() -> ChoirPlayerView {
        let snap = runtime.snapshot()
        let p = runtime.player
        var v = ChoirPlayerView()
        v.signedIn = signIn.current.signedIn
        lock.lock(); v.loading = loading; v.message = message; lock.unlock()
        v.title = p.document.title
        if p.document.blocks.indices.contains(p.blockIndex) { v.passage = p.document.blocks[p.blockIndex].spokenText ?? p.document.blocks[p.blockIndex].summary }
        v.position = p.document.blocks.isEmpty ? "" : "\(p.blockIndex + 1) of \(p.document.blocks.count)"
        v.playing = snap.running && !p.userPaused
        v.talking = snap.phase != "playing"
        v.heard = snap.phase == "playing" ? "" : snap.transcript
        if v.message.isEmpty, v.signedIn, !v.loading, p.document.blocks.isEmpty { v.message = "Pick a story below." }
        return v
    }
    public func storyRows() -> [StoryRow] {
        lock.lock(); let s = stories, r = currentRoute; lock.unlock()
        return s.map { StoryRow(title: $0.title, dek: $0.dek, route: $0.route, section: $0.section, current: $0.route == r) }
    }
    public func authView() -> ChoirAuthState { signIn.current }
}
