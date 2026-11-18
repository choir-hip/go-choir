import Foundation

// Writes the next opening ahead of need (see OpeningCache). Triggers, each one request at most: the session's opening has aired, and the app
// goes to the background. (The Exact host exposes no BGAppRefreshTask: docs/reference/exact has no background-task surface, so there is no
// third trigger.) Never a loop, never a second request while one is in flight (this one's, or the run sheet writer's).
//
// Ways this can fail (each has a test in OpeningCacheTests):
//  1. a trigger while a request is in flight asks again
//  2. a background with a fresh opening already kept (under `refreshAfter`) asks again
//  3. the opening's speech is made for text the player will not speak (the blocks must be the ones SegmentPrep makes)
//  4. a voice that cannot make the sound loses the text (it is kept without audio)
//  5. the first entry is written ahead with all its blocks (only its first block is made; the rest is made as it plays)
//  6. an entry already heard, or already the one kept, is made again

@MainActor public final class OpeningWriteAhead {
    let cache: OpeningCache
    let store: RunSheetStore
    var feed: () -> RunSheetFeed?
    var voice: () -> String
    var synth: (String) async -> PreparedAudio?
    /// The run sheet writer has a request in flight.
    var busy: () -> Bool
    var clock: () -> Date
    /// A kept opening younger than this is not asked for again on a background (the part of the day still has to match).
    public var refreshAfter: TimeInterval
    public private(set) var inFlight = false
    /// A trigger refused because the writer was busy: it runs when the writer next goes idle (`writerIdle`), once.
    private var pendingReason: String?
    private var pendingGen = 0           // the account generation when the trigger was refused: a sign-out in between drops it
    private var gen = 0
    public private(set) var requests: [String] = []
    public private(set) var synthesised: [String] = []

    public init(cache: OpeningCache, store: RunSheetStore, feed: @escaping () -> RunSheetFeed?, voice: @escaping () -> String,
                synth: @escaping (String) async -> PreparedAudio?, busy: @escaping () -> Bool = { false }, clock: @escaping () -> Date = { Date() }) {
        self.cache = cache; self.store = store; self.feed = feed; self.voice = voice; self.synth = synth; self.busy = busy; self.clock = clock
        self.refreshAfter = cache.bound / 2
    }

    /// The blocks the player will speak for an entry: the same cut the player makes (`SegmentPrep`).
    static func blocks(of e: RunSheetEntry) -> [String] {
        let kind = e.kind == .opening ? "opening" : e.kind == .transition ? "transition" : "story"
        let inc = IncomingSegment(stretchID: "rs", segmentID: e.id, kind: kind, title: e.title, text: e.text ?? "", cites: [], blockCount: nil)
        return (SegmentPrep.prepare(inc)?.blocks ?? []).compactMap(\.spokenText)
    }

    /// One trigger. Returns when everything it does is done.
    public func refresh(reason: String) async { await refresh(reason: reason, generation: AccountWipe.generation) }

    func refresh(reason: String, generation: Int) async {
        guard !inFlight, generation == AccountWipe.generation else { return }
        inFlight = true
        defer { inFlight = false }
        gen = generation          // a put after the account was wiped is dropped
        let v = voice()
        // The first entry: no request, only the first block made. Done on every trigger.
        await keepFirstEntry(voice: v)
        let kept = cache.opening(voice: v)
        if let k = kept, reason == "background" {
            let age = clock().timeIntervalSince(k.writtenAt)
            if age >= 0, age < refreshAfter, cache.dayPart(k.writtenAt) == cache.dayPart(clock()) { return }
        }
        guard !busy() else { pendingReason = reason; pendingGen = gen; print("AUTORADIO write-ahead deferred: writer busy"); return }
        guard let feed = feed() else { return }
        pendingReason = nil
        requests.append(reason)
        var opening: RunSheetEntry?
        do {
            let stream = try await feed.ask(RunSheetAsk(cause: "open"))
            for try await e in stream {
                if case .ops(let ops) = e, opening == nil {
                    opening = ops.compactMap { $0.op == .place ? $0.entry : nil }.first { $0.kind == .opening && !($0.text ?? "").isEmpty }
                }
            }
        } catch { return }
        guard var o = opening else { return }
        // Nothing that depends on the moment of speaking is pre-rendered (Moment.swift): the body is made true whenever it is played.
        o.text = Moment.body(o.text ?? "")
        var audio: [PreparedAudio] = []
        for b in Self.blocks(of: o) {
            guard let a = await synth(b) else { audio = []; break }       // a voice that cannot make it: the text is kept without sound
            synthesised.append(b)
            audio.append(a)
        }
        guard gen == AccountWipe.generation else { return }
        cache.put(.opening, entry: o, titles: (o.cites ?? []).map(\.label).filter { !$0.isEmpty }, audio: audio, voice: v)
    }

    /// The run sheet writer went idle (an ask ended). A refused trigger runs now; otherwise, when no first entry is kept (or the kept one has
    /// been heard), the first entry ahead is kept: its first block only, with no request to the desk (the story is already written).
    public func writerIdle() async {
        if let r = pendingReason {
            pendingReason = nil
            await refresh(reason: r, generation: pendingGen)           // the generation of the refusal, not of now: a wipe in between drops it
            return
        }
        guard !inFlight else { return }
        let v = voice()
        if let k = cache.first(voice: v), store.entry(k.entry.id).map({ $0.state == .ahead }) ?? true { return }
        inFlight = true
        defer { inFlight = false }
        gen = AccountWipe.generation
        await keepFirstEntry(voice: v)
    }

    /// The first entry ahead on the sheet (a desk story the listener has not heard), with its first block made.
    private func keepFirstEntry(voice v: String) async {
        guard let e = store.ahead(limit: 20).first(where: { $0.kind == .story && $0.placedBy == .desk && !($0.text ?? "").isEmpty }),
              store.entry(e.id)?.state == .ahead else { return }
        if let k = cache.first(voice: v), k.entry.id == e.id, !k.files.isEmpty || Self.blocks(of: e).isEmpty { return }
        guard let b = Self.blocks(of: e).first else { return }
        let a = Moment.dependsOnMoment(b) ? nil : await synth(b)             // a first block that names the moment is kept as text, not as sound
        if a != nil { synthesised.append(b) }
        guard gen == AccountWipe.generation else { return }
        cache.put(.first, entry: e, titles: (e.cites ?? []).map(\.label).filter { !$0.isEmpty }, audio: a.map { [$0] } ?? [], voice: v)
    }
}


/// What plays when the desk cannot be reached: the newest stories whose bodies the phone holds (the ContentStore), not already on the sheet.
public enum LocalStories {
    @MainActor public static func entries(from content: ContentStore, sheet: RunSheetStore, limit: Int = 5) -> [RunSheetEntry] {
        let told = Set(sheet.sheet.entries.map { $0.title.lowercased() })
        var out: [RunSheetEntry] = []
        for item in content.items(.story).sorted(by: { ($0.date ?? .distantPast) > ($1.date ?? .distantPast) }) {
            let title = RunSheetEntry.cleanTitle(item.title)
            let body = item.text.trimmingCharacters(in: .whitespacesAndNewlines)
            guard !title.isEmpty, !body.isEmpty, !told.contains(title.lowercased()) else { continue }
            var id = "s-"
            for u in item.id.unicodeScalars where id.utf8.count < 64 {
                let ok = (u.value >= 0x30 && u.value <= 0x39) || (u.value >= 0x41 && u.value <= 0x5a) || (u.value >= 0x61 && u.value <= 0x7a) || u == "_" || u == "." || u == "-"
                id.append(ok ? Character(u) : "-")
            }
            guard sheet.entry(id) == nil else { continue }
            let text = StretchCandidates.cut(ChoirText.paragraphs(body, maxChars: ChoirText.speechChars).joined(separator: "\n\n"), bytes: RunSheetBounds.maxTextBytes)
            var e = RunSheetEntry(id: id, kind: .story, placedBy: .phone, title: title, text: text, estSeconds: RunSheetEntry.estimateSeconds(words: text))
            e.source = RunSheetSource(ref: item.address, label: title)
            out.append(e)
            if out.count >= limit { break }
        }
        return out
    }
}
