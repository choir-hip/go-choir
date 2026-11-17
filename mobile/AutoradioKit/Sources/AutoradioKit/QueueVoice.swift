import Foundation

// Asking about playback order (lane autoradio-talk, owner 2026-11-15: "change the playback order by talking"). Two paths:
//  (a) plain commands the phone recognises at once and acts on within a second, with no network: skip, back, pause, play, "play this
//      next", "add this to the queue", "clear the queue". A small deterministic matcher over the WHOLE utterance, so a question that
//      merely contains the word "next" is never taken for a command.
//  (b) anything else goes to the server as a turn; its `queue_ops` event is handed, unread, to the queue lane's apply
//      (`PlayQueue.shared.apply(serverOpsJSON:)`), which owns decoding, the rebase rule and dedupe. The server speaks its own
//      confirmation, so the phone adds no spoken line for server ops.
// The queue lane's `PlayQueue` is not in this branch: everything it needs from the queue goes through the closures below, set once by the
// host (one adapter, so a rename is one edit).
//
// WAYS THIS CAN FAIL (written before the code; each has a test in QueueVoiceTests):
//  1. A question that contains a command word ("what is next in the paper", "is the next election close") is taken for a command: the
//     match needs the whole utterance to be a command phrase.
//  2. Capital letters, punctuation, "please", "okay radio" in front change the match: they are normalised away.
//  3. A command with no queue behind it (the host did not set the closure): nothing is claimed; the turn goes to the server as usual.
//  4. "this" means the page in front: with no page in front "play this next" is not a command.
//  5. The queue snapshot or the candidates are missing: the turn is sent without them (a server that ignores them behaves as today).
//  6. A server `queue_ops` event with a bad body: dropped with a log line, never a crash, nothing spoken.

public enum QueueRef: Equatable, Sendable {
    /// The page or story in front of the reader.
    case front
}

public enum TalkQueueOp: Equatable, Sendable {
    case playNext(QueueRef), enqueue(QueueRef), skip, previous, clear, pause, resume
}

public enum SpokenCommand {
    static let skip: Set<String> = ["skip", "skip this", "skip it", "skip that", "next", "next one", "next story", "next item", "play the next one", "go to the next one", "go to the next story", "play next one"]
    static let previous: Set<String> = ["back", "go back", "previous", "previous one", "previous story", "last one", "play the previous one", "go to the previous one", "play the last one"]
    static let pause: Set<String> = ["pause", "pause it", "pause the radio", "stop", "stop it", "stop the radio", "hold on", "wait"]
    static let resume: Set<String> = ["play", "resume", "continue", "keep going", "unpause", "carry on", "go on", "keep playing", "play it"]
    static let playNext: Set<String> = ["play this next", "play this one next", "read this next", "play this after this", "put this next", "play this after"]
    static let enqueue: Set<String> = ["add this to the queue", "add this to my queue", "add to the queue", "add to queue", "queue this", "queue this up", "queue it", "queue it up", "add it to the queue", "put this in the queue", "add this to up next"]
    static let clear: Set<String> = ["clear the queue", "clear my queue", "empty the queue", "clear the up next", "clear up next", "clear the playlist", "clear queue"]

    static func normalise(_ text: String) -> String {
        var words = text.lowercased().split(whereSeparator: { !($0.isLetter || $0.isNumber || $0 == "'") }).map(String.init)
        for lead in [["okay"], ["ok"], ["hey"], ["radio"], ["please"], ["can", "you"], ["could", "you"], ["would", "you"]] {
            while words.count > lead.count, Array(words.prefix(lead.count)) == lead { words.removeFirst(lead.count) }
        }
        while words.last == "please" || words.last == "thanks" { words.removeLast() }
        return words.joined(separator: " ")
    }

    /// The ops a plain command means, or nil when the words are not exactly a command. `pageInFront`: whether "this" has something to mean.
    public static func match(_ text: String, pageInFront: Bool = true) -> [TalkQueueOp]? {
        let t = normalise(text)
        if t.isEmpty { return nil }
        if skip.contains(t) { return [.skip] }
        if previous.contains(t) { return [.previous] }
        if pause.contains(t) { return [.pause] }
        if resume.contains(t) { return [.resume] }
        if clear.contains(t) { return [.clear] }
        if pageInFront, playNext.contains(t) { return [.playNext(.front)] }
        if pageInFront, enqueue.contains(t) { return [.enqueue(.front)] }
        return nil
    }

    /// What was done, in one short line (shown, not spoken: the change itself is audible).
    public static func line(_ ops: [TalkQueueOp]) -> String {
        guard let op = ops.first else { return "Okay." }
        switch op {
        case .skip: return "Okay, skipping."
        case .previous: return "Okay, going back."
        case .pause: return "Okay, paused."
        case .resume: return "Okay, playing."
        case .clear: return "Okay, the queue is clear."
        case .playNext: return "Okay, this is next."
        case .enqueue: return "Okay, added to the queue."
        }
    }
}

/// The host's side of the queue, set once (see the adapter in AutoradioModule). All optional: an unset closure means "no queue here".
public enum QueueVoice {
    public struct Hooks: Sendable {
        /// Carries out plain commands (the queue lane's PlayQueue and the player). Returns false when it could not.
        public var perform: (@Sendable ([TalkQueueOp], _ front: FrontPage?) async -> Bool)?
        /// `PlayQueue.shared.snapshot()` as the wire's `queue` object, JSON bytes (nil: no queue).
        public var snapshot: (@Sendable () async -> Data?)?
        /// The paper's current list, route and title, for the wire's `candidates` (up to 40).
        public var candidates: (@Sendable () -> [(route: String, title: String)])?
        /// The `queue_ops` event's JSON, as the server sent it, for `PlayQueue.shared.apply(serverOpsJSON:)`.
        public var applyServerOps: (@Sendable (Data) async -> Void)?
        /// Plays, opens or queues an item the last answer retrieved ("play that", "open the second one"). False: it could not.
        /// Carries out a command the phone resolved from its own stores ("read my book", "play the latest <show>", "what is in my mail"). False: could not.
        public var actOnCommand: (@Sendable (LocalCommand) async -> Bool)?
        /// The stores commands are resolved against (nil: the real ones).
        public var catalog: (@Sendable () -> LocalCatalog)?
        public var actOnRetrieved: (@Sendable (RetrievedAction, RetrievalItem) async -> Bool)?
        public init() {}
    }
    private static let box = Locked<Hooks>(Hooks())
    public static var hooks: Hooks { get { box.value } set { box.value = newValue } }
}
