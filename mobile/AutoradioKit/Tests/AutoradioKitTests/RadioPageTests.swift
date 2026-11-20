import XCTest
@testable import AutoradioKit

// The Autoradio page's view model and the ops its gestures send. The failure list is at the top of RadioPage.swift (numbers below).

@MainActor
final class RadioPageTests: XCTestCase {
    func entry(_ id: String, _ kind: RunSheetKind = .story, by: RunSheetBy = .desk, state: RunSheetState = .ahead, title: String? = nil,
               est: Double = 60, outcome: RunSheetOutcome? = nil) -> RunSheetEntry {
        var e = RunSheetEntry(id: id, kind: kind, placedBy: by, state: state, title: title ?? "Title \(id)", text: kind.textRequired ? "Words." : nil, estSeconds: est)
        if let outcome { e.ended = RunSheetEnded(outcome: outcome) }
        return e
    }
    func input(_ entries: [RunSheetEntry]) -> RadioPageInput {
        var s = RunSheet(); s.entries = entries; s.onAir = entries.first { $0.state == .onAir }?.id
        var i = RadioPageInput(sheet: s)
        i.now = Date(timeIntervalSince1970: 1_700_000_000); i.timeZone = TimeZone(identifier: "UTC")!   // 22:13 UTC
        return i
    }

    // 1 the status line for each state
    func test01_StatusLines() {
        var on = input([entry("a", state: .onAir), entry("b")]); on.playing = true
        XCTAssertEqual(RadioPage.make(on).statusLine, "On air"); XCTAssertEqual(RadioPage.make(on).state, "onair")
        on.playing = false
        XCTAssertEqual(RadioPage.make(on).statusLine, "Paused")
        let ready = input([entry("h", state: .heard, outcome: .finished), entry("b")])
        XCTAssertEqual(RadioPage.make(ready).statusLine, "Ready"); XCTAssertEqual(RadioPage.make(ready).state, "ready")
        var writing = input([entry("h", state: .heard, outcome: .finished)]); writing.writing = true
        XCTAssertEqual(RadioPage.make(writing).statusLine, "Writing the next stretch")
        var off = input([entry("a", state: .onAir)]); off.playing = true; off.notice = "The desk can't be reached right now."
        XCTAssertEqual(RadioPage.make(off).state, "offline")
        XCTAssertEqual(RadioPage.make(off).statusLine, "Offline: the desk cannot write ahead")
        XCTAssertTrue(RadioPage.make(off).playing, "what is written still plays")
    }

    // 2 listening dims the card and wins
    func test02_ListeningDimsAndShowsWordsWhenRecognised() {
        var i = input([entry("a", state: .onAir)]); i.playing = true; i.barState = "listening"; i.heardWords = "Listening\u{2026}"
        var p = RadioPage.make(i)
        XCTAssertEqual(p.statusLine, "Listening"); XCTAssertTrue(p.dim); XCTAssertEqual(p.heardWords, "")
        i.barState = "talking"; i.heardWords = "tell me about the harbour"
        p = RadioPage.make(i)
        XCTAssertEqual(p.heardWords, "tell me about the harbour"); XCTAssertEqual(p.state, "listening")
        i.barState = "idle"; XCTAssertFalse(RadioPage.make(i).dim)
    }

    // 3 cannot speak
    func test03_CannotSpeakNamesTheReasonWithOneAction() {
        var i = input([entry("a", state: .onAir)]); i.playing = false; i.message = "Can\u{2019}t speak: the voice made no sound"
        var p = RadioPage.make(i)
        XCTAssertEqual(p.statusLine, "Can\u{2019}t speak: the voice made no sound"); XCTAssertEqual(p.cannotSpeak, "the voice made no sound")
        XCTAssertEqual(p.action, "Try again"); XCTAssertEqual(p.state, "error")
        i.message = "Could not play \u{201C}X\u{201D}."
        p = RadioPage.make(i)
        XCTAssertEqual(p.cannotSpeak, "Could not play \u{201C}X\u{201D}.")
        i.playing = true
        XCTAssertEqual(RadioPage.make(i).state, "onair", "a stale message does not hide a voice that is speaking")
        var cold = input([entry("a"), entry("b")]); cold.message = "Can\u{2019}t speak: the audio session could not be taken"
        let c = RadioPage.make(cold)
        XCTAssertEqual(c.greeting, "", "while it cannot speak the greeting does not say Press play"); XCTAssertEqual(c.state, "error")
        XCTAssertEqual(c.statusLine, "Can\u{2019}t speak: the audio session could not be taken"); XCTAssertEqual(c.action, "Try again")
    }

    // 4 an answer on air holds the entry it cut
    func test04_AnAnswerNamesWhatItGoesBackTo() {
        let cut = entry("s", .story, state: .skipped, title: "The harbour barrier", outcome: .cut)
        let older = entry("o", .story, state: .heard, title: "Older", outcome: .cut)
        var i = input([older, cut, entry("ans", .answer, by: .phone, state: .onAir, title: "Answer"), entry("b")]); i.playing = true
        let p = RadioPage.make(i)
        XCTAssertEqual(p.kindLabel, "Answer"); XCTAssertEqual(p.thenBack, "Then back to: The harbour barrier")
        var j = input([entry("a", .story, state: .onAir)]); j.playing = true
        XCTAssertEqual(RadioPage.make(j).thenBack, "")
    }

    // 5 cold start
    func test05_ColdStartGreetsWithCountsAndMinutes() {
        var open = entry("d-open", .opening, by: .phone, title: "Opening", est: 120)
        open.text = "Hello."
        let rest = (1...5).map { entry("d-\($0)", est: 96) }
        let p = RadioPage.make(input([open] + rest))
        XCTAssertEqual(p.state, "cold"); XCTAssertEqual(p.statusLine, "Ready")
        XCTAssertEqual(p.greeting, "Good evening. Six things are ready, about ten minutes. Press play and I\u{2019}ll start with the opening.")
        let after = RadioPage.make(input([entry("h", state: .heard, outcome: .finished), entry("b")]))
        XCTAssertTrue(after.greeting.hasPrefix("Good evening. One thing is ready"), "nothing on air and entries ahead: the greeting says what is lined up"); XCTAssertEqual(after.state, "ready", "but it is not the cold start")
        let one = RadioPage.make(input([entry("only", title: "Alone", est: 30)]))
        XCTAssertEqual(one.greeting, "Good evening. One thing is ready, about a minute. Press play and I\u{2019}ll start with Alone.")
        XCTAssertEqual(RadioPage.make(input([])).greeting, "")
    }

    // the cold start is nothing played lately, not only an empty tail (every real launch has yesterday's entries behind it)
    func test05b_ColdMeansNothingHeardLately() {
        func ended(_ at: String) -> RunSheetEntry { var e = entry("h", state: .heard, outcome: .finished); e.ended = RunSheetEnded(outcome: .finished, at: at); return e }
        var i = input([ended("2023-11-13T09:00:00Z"), entry("a", .opening, by: .phone, est: 120), entry("b", est: 1680)])
        XCTAssertEqual(RadioPage.make(i).state, "cold", "yesterday's entries do not stop the greeting")
        XCTAssertTrue(RadioPage.make(i).greeting.contains("two things are ready") || RadioPage.make(i).greeting.contains("Two things are ready"))
        XCTAssertTrue(RadioPage.make(i).greeting.contains("about thirty minutes"))
        i = input([ended("2023-11-14T22:00:00Z"), entry("a")])
        XCTAssertEqual(RadioPage.make(i).state, "ready", "heard fifteen minutes ago: not a cold start")
    }

    func test05d_TheDayPartIsTheAppsOneFunction() {
        var c = Calendar(identifier: .gregorian); c.timeZone = TimeZone(identifier: "UTC")!
        func at(_ h: Int) -> Date { c.date(from: DateComponents(year: 2026, month: 11, day: 17, hour: h, minute: 53))! }
        XCTAssertEqual(PhoneOpening.hello(at(0), calendar: c), "Hello"); XCTAssertEqual(PhoneOpening.hello(at(5), calendar: c), "Good morning")
        XCTAssertEqual(PhoneOpening.hello(at(12), calendar: c), "Good afternoon"); XCTAssertEqual(PhoneOpening.hello(at(18), calendar: c), "Good evening")
        XCTAssertEqual(PhoneOpening.greeting(at(0), calendar: c), "Hello.")
        var i = input([entry("a")]); i.now = at(0); i.timeZone = c.timeZone
        XCTAssertTrue(RadioPage.make(i).greeting.hasPrefix("Hello. One thing is ready"))
    }

    // 6 transitions are not rows
    func test06_ATransitionIsNeitherUpNextNorHeard() {
        var t = entry("t", .transition, title: ""); t.title = ""
        let h = entry("th", .transition, state: .heard, outcome: .finished)
        let p = RadioPage.make(input([h, entry("hh", state: .heard, outcome: .finished), t, entry("a"), entry("b")]))
        XCTAssertEqual(p.ahead.map(\.id), ["a", "b"]); XCTAssertEqual(p.heard.map(\.id), ["hh"])
    }

    func test07d_TheEntryOnAirIsNotListedInUpNext() {
        let p = RadioPage.make(input([entry("h", state: .heard, outcome: .finished), entry("on", .opening, by: .phone, state: .onAir, title: "Opening"), entry("a"), entry("b")]))
        XCTAssertEqual(p.ahead.map(\.id), ["a", "b"]); XCTAssertEqual(p.title, "Opening")
        XCTAssertEqual(p.aheadMore, 0)
        let nine = RadioPage.make(input([entry("on", state: .onAir)] + (1...8).map { entry("e\($0)") }))
        XCTAssertEqual(nine.ahead.count, 8); XCTAssertEqual(nine.aheadMore, 3, "All of it appears when more than five are ahead")
    }

    // 1b the page says "On air" only when something is on air; before the first audio it is the cold start
    func test01b_BeforeTheFirstAudioItIsTheColdStartWithOnePlay() {
        let p = RadioPage.make(input([entry("a"), entry("b")]))
        XCTAssertEqual(p.state, "cold"); XCTAssertFalse(p.onAir); XCTAssertFalse(p.playing); XCTAssertEqual(p.sentence, "")
        XCTAssertTrue(p.greeting.hasPrefix("Good evening. Two things are ready"))
    }

    // the bar's now-playing line is not drawn on the Autoradio page (it would sit over the list); everywhere else it opens the page
    func test07e_TheBarsNowLineIsHiddenOnTheRadioPage() throws {
        let dir = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().appendingPathComponent("AutoradioLab")
        let bar = try String(contentsOf: dir.appendingPathComponent("bar.contract"))
        XCTAssertTrue(bar.contains(#"st == "playing" ? (topAddr == "radio:" ? "none" : "playing")"#))
        XCTAssertTrue(bar.contains(#"press=openPage("radio:", "Autoradio") testId="bar-now""#))
    }

    // 7 up next
    func test07_UpNextHasEveryEntryInOrderWithWhoPlacedEach() {
        let es = (1...8).map { entry("e\($0)", by: $0 == 2 ? .listener : .desk, est: $0 == 3 ? 0 : 200) }
        let p = RadioPage.make(input(es))
        XCTAssertEqual(p.ahead.map(\.id), (1...8).map { "e\($0)" }); XCTAssertEqual(p.ahead.map(\.n), Array(0..<8))
        XCTAssertEqual(p.aheadMore, 3)
        XCTAssertEqual(p.ahead[1].byWord, "You"); XCTAssertEqual(p.ahead[0].byWord, "Desk")
        XCTAssertEqual(p.ahead[0].length, "3 min"); XCTAssertEqual(p.ahead[2].length, "")
        XCTAssertEqual(p.ahead[0].kind, "Story")
    }

    func test07b_TheEmptyListSaysWhatIsTrue() {
        var i = input([])
        XCTAssertEqual(RadioPage.make(i).emptyNote, "The desk will write more shortly.")
        i.dry = true; XCTAssertTrue(RadioPage.make(i).emptyNote.hasPrefix("Nothing is coming up."))
        i.writing = true; XCTAssertEqual(RadioPage.make(i).emptyNote, "The desk is writing what comes next.")
        i.writing = false; i.dry = false; i.notice = "x"; XCTAssertEqual(RadioPage.make(i).emptyNote, "Nothing is coming up yet.")
        XCTAssertEqual(RadioPage.make(input([entry("a")])).emptyNote, "")
    }

    func test07c_AStaleTransitionNeighbourIsNoted() {
        var i = input([entry("a")]); i.stale = ["a"]
        XCTAssertEqual(RadioPage.make(i).ahead[0].note, "Written for other neighbours")
    }

    // 8 why this: the stated cause and forecast when carried, else a sentence from what the entry does carry
    func test08_WhyThisSaysWhatTheEntryCarries() {
        var e = entry("a", state: .onAir)
        var i = input([e]); i.playing = true
        var p = RadioPage.make(i)
        XCTAssertTrue(p.canWhy); XCTAssertEqual(p.why, "The desk wrote this for you."); XCTAssertEqual(p.forecast, "", "no forecast is invented")
        e.cites = [RunSheetCite(ref: "pub:x", role: .subject, label: "Harbour Ledger")]
        p = RadioPage.make(input([e]))
        XCTAssertEqual(p.why, "The desk wrote this for you. It draws on Harbour Ledger.")
        XCTAssertEqual(RadioPage.make(input([entry("l", by: .listener, state: .onAir)])).why, "You added this.")
        XCTAssertEqual(RadioPage.make(input([entry("o", .opening, by: .phone, state: .onAir)])).why, "The station opens each listening, from the front page it holds.")
        e.forecast = RunSheetForecast(pHeard: 0.72, causedBy: "you finished two stories on the harbour yesterday", claimId: "c1")
        p = RadioPage.make(input([e]))
        XCTAssertEqual(p.why, "You finished two stories on the harbour yesterday.")
        XCTAssertEqual(p.forecast, "The desk forecast a 72 in 100 chance that you hear it through.")
        e.forecast = RunSheetForecast(pHeard: 0.5, causedBy: "", claimId: "c1")
        p = RadioPage.make(input([e]))
        XCTAssertTrue(p.why.hasPrefix("The desk wrote this")); XCTAssertEqual(p.forecast, "The desk forecast a 50 in 100 chance that you hear it through.")
        XCTAssertFalse(RadioPage.make(input([entry("n")])).canWhy, "nothing on air: no control")
    }

    // 9 the sentence
    func test09_TheSentenceIsTheOneAtTheIndex() {
        var i = input([entry("a", state: .onAir, est: 100)]); i.playing = true
        i.sentences = ["One.", "Two.", "Three."]; i.sentenceIndex = 1; i.progress = 0.25
        var p = RadioPage.make(i)
        XCTAssertEqual(p.sentence, "Two."); XCTAssertEqual(p.sentenceNumber, 2); XCTAssertEqual(p.sentenceCount, 3)
        XCTAssertEqual(p.elapsed, "0:25"); XCTAssertEqual(p.remaining, "-1:15"); XCTAssertEqual(p.fraction, 0.25, accuracy: 0.001)
        i.sentenceIndex = 9; p = RadioPage.make(i)
        XCTAssertEqual(p.sentence, ""); XCTAssertEqual(p.sentenceNumber, 0)
    }

    // 10 podcasts show no sentence
    func test10_AudioHasNoSentenceAndUsesItsOwnClock() {
        var e = entry("p", .episode, state: .onAir, title: "Episode"); e.source = RunSheetSource(ref: "episode:show", label: "Harbour Hour", url: "https://example.com/a.mp3")
        var i = input([e]); i.playing = true; i.remoteAudio = true; i.sentences = ["x"]; i.elapsedLabel = "1:12"; i.remainingLabel = "-3:40"
        let p = RadioPage.make(i)
        XCTAssertEqual(p.sentence, ""); XCTAssertEqual(p.elapsed, "1:12"); XCTAssertEqual(p.remaining, "-3:40")
        XCTAssertEqual(p.kindLabel, "Podcast"); XCTAssertEqual(p.source, "Harbour Hour")
    }

    // Built on: the stories an entry was built on, tappable, none drawn when there are none
    func test15_BuiltOnListsTheStoriesTheEntryWasBuiltOn() {
        var e = entry("h", .story, state: .onAir)
        e.cites = [RunSheetCite(ref: "pub:harbour-barrier@v2", role: .subject, label: "The harbour barrier passes"), RunSheetCite(ref: "pub:tram-fares@v1", role: .quoted, label: "Tram fares change"),
                   RunSheetCite(ref: "pub:harbour-barrier@v3", role: .mentioned, label: "again"), RunSheetCite(ref: "clip:x", role: .subject, label: "not a story")]
        e.source = RunSheetSource(ref: "pub:harbour-barrier", label: "The harbour barrier passes")
        let b = RadioPage.make(input([e])).builtOn
        XCTAssertEqual(b.map(\.title), ["The harbour barrier passes", "Tram fares change"], "once each, in order, stories only")
        XCTAssertEqual(b.map(\.address), ["pub:harbour-barrier", "pub:tram-fares"]); XCTAssertEqual(b.map(\.n), [0, 1])
        XCTAssertTrue(RadioPage.make(input([entry("o", .opening, by: .phone, state: .onAir)])).builtOn.isEmpty, "an opening has none: no heading")
        var t = entry("t", .story, state: .onAir); t.cites = [RunSheetCite(ref: "pub:x", role: .subject, label: "")]
        XCTAssertEqual(RadioPage.make(input([t])).builtOn.first?.title, "x", "the route stands in for a missing label")
        var heard = entry("hh", .story, state: .heard, outcome: .finished); heard.cites = e.cites
        XCTAssertEqual(RadioPage.make(input([heard, entry("a")])).heard.first?.builtOn.count, 2)
    }
    func test15c_ADeskStoryTitleOpensItsSourceAndAVersionDifferenceIsNoted() {
        var e = entry("s", .story, state: .onAir)
        e.source = RunSheetSource(ref: "pub:harbour-barrier@v2", label: "The harbour barrier")
        e.cites = [RunSheetCite(ref: "pub:harbour-barrier@v2", role: .subject, label: "The harbour barrier"), RunSheetCite(ref: "pub:tram-fares@v1", role: .quoted, label: "Tram fares")]
        var i = input([e]); i.versions = ["harbour-barrier": 3, "tram-fares": 1]
        let p = RadioPage.make(i)
        XCTAssertEqual(p.openAddress, "pub:harbour-barrier", "a story entry (not only a page) opens its source story")
        XCTAssertEqual(p.builtOn.map(\.note), ["cited v2, now v3", ""]); XCTAssertEqual(p.builtOn.map(\.role), ["", "quoted"])
        XCTAssertEqual(RadioPage.make(input([e, entry("n")])).ahead.count, 1)
    }
    func test15b_AClipsStoryRefReachesTheEntryAsACite() {
        var s = RunSheetEntry(id: "c", kind: .clip, placedBy: .desk, title: "Clip")
        s.cites = [RunSheetCite(ref: "pub:harbour-barrier", role: .subject, label: "")]
        XCTAssertEqual(RadioPage.builtOn(s).map(\.address), ["pub:harbour-barrier"])
    }

    // one state, one message: the desk is writing and the radio will start by itself: one line, no Play, no "Nothing is on air"
    func test16b_WritingAndWantedIsOneLineWithNoPlay() {
        var i = input([entry("h", state: .heard, outcome: .finished)]); i.writing = true; i.wants = true
        var p = RadioPage.make(i)
        XCTAssertEqual(p.statusLine, "Writing the next stretch. It will start by itself."); XCTAssertFalse(p.canPlay); XCTAssertFalse(p.onAir)
        i.wants = false; p = RadioPage.make(i)
        XCTAssertEqual(p.statusLine, "Writing the next stretch"); XCTAssertTrue(p.canPlay, "not wanted: pressing Play does something")
        i.writing = false; XCTAssertTrue(RadioPage.make(i).canPlay)
    }

    // 12 heard
    func test12_HeardIsNewestFirstWithOneWord() {
        var tail: [RunSheetEntry] = []
        tail.append(entry("h1", state: .heard, outcome: .finished))
        tail.append(entry("h2", state: .skipped, outcome: .skipped))
        tail.append(entry("h3", state: .heard, outcome: .left))
        tail.append(entry("h4", state: .skipped, outcome: .failed))
        tail.append(entry("h5", state: .skipped, outcome: .cut))
        tail.append(entry("h6", state: .heard, outcome: .finished))
        let p = RadioPage.make(input(tail + [entry("a")]))
        let got: [String] = p.heard.map { $0.id }
        let words: [String] = p.heard.map { $0.outcome }
        XCTAssertEqual(got, ["h6", "h5", "h3", "h2", "h1"], "h4 failed: it never made a sound, so it was not heard"); XCTAssertEqual(words, ["finished", "cut", "left", "skipped", "finished"])
    }

    func test12b_TheDateAndKindsRead() {
        XCTAssertEqual(RadioPage.make(input([])).dateLine, "Tue 14 Nov")
        XCTAssertEqual(RadioPage.kindLabel(entry("b", .document)), "Book")
        XCTAssertEqual(RadioPage.kindLabel(entry("o", .opening)), "Opening")
        XCTAssertEqual(RadioPage.length(0), ""); XCTAssertEqual(RadioPage.length(45), "45 sec"); XCTAssertEqual(RadioPage.length(600), "10 min")
    }

    func test12c_TheTitleOpensTheStoryOrBook() {
        var s = entry("s", .page, state: .onAir); s.source = RunSheetSource(ref: "pub:harbour-barrier@v2", label: "Harbour Ledger")
        XCTAssertEqual(RadioPage.make(input([s])).openAddress, "pub:harbour-barrier")
        var b = entry("b", .document, state: .onAir); b.source = RunSheetSource(ref: "doc:bk1", label: "A Book", start: 3)
        XCTAssertEqual(RadioPage.make(input([b])).openAddress, "doc:bk1")
        XCTAssertEqual(RadioPage.make(input([entry("x", .story, state: .onAir)])).openAddress, "")
    }

    // 13 the gestures
    func makePlayer(_ es: [RunSheetEntry]) -> (RunSheetPlayer, RunSheetStore) {
        let s = RunSheetStore(); let p = RunSheetPlayer(store: s); p.engine = FakeEngine()
        for e in es { s.place(e, at: .end, by: e.placedBy, reason: "test") }
        return (p, s)
    }
    func ids(_ s: RunSheetStore) -> [String] { s.sheet.ahead().map(\.id) }

    func test13a_ReorderIsOneMoveOp() {
        let (p, s) = makePlayer(["a", "b", "c", "d"].map { entry($0) })
        let before = s.pending.count
        XCTAssertNotNil(p.perform(.reorder(id: "d", before: "b")))
        XCTAssertEqual(ids(s), ["a", "d", "b", "c"])
        XCTAssertEqual(s.pending.count, before + 1)
        let op = s.pending.last!
        XCTAssertEqual(op.op, .move); XCTAssertEqual(op.at, .after); XCTAssertEqual(op.after, "a"); XCTAssertEqual(op.id, "d"); XCTAssertEqual(op.by, .listener)
        p.perform(.reorder(id: "c", before: "a")); XCTAssertEqual(ids(s), ["c", "a", "d", "b"]); XCTAssertEqual(s.pending.last?.at, .next)
        p.perform(.reorder(id: "c", before: nil)); XCTAssertEqual(ids(s), ["a", "d", "b", "c"]); XCTAssertEqual(s.pending.last?.at, .end)
        let n = s.pending.count
        XCTAssertNil(p.perform(.reorder(id: "c", before: nil)), "already last: nothing is sent")
        XCTAssertEqual(s.pending.count, n)
        XCTAssertNil(p.perform(.reorder(id: "zzz", before: "a")))
    }

    func test13b_ASwipeIsOneRemoveOp() {
        let (p, s) = makePlayer(["a", "b", "c"].map { entry($0) })
        p.perform(.remove(id: "b"))
        XCTAssertEqual(ids(s), ["a", "c"]); XCTAssertEqual(s.pending.last?.op, .remove); XCTAssertEqual(s.pending.last?.id, "b"); XCTAssertEqual(s.pending.last?.by, .listener)
        let n = s.pending.count
        XCTAssertNil(p.perform(.remove(id: "b"))); XCTAssertEqual(s.pending.count, n, "a second swipe sends nothing")
    }

    func test13c_ATapIsThePlayNowOpTheSheetUsed() {
        let (p, s) = makePlayer(["a", "b", "c"].map { entry($0) })
        p.perform(.playNow(id: "c"))
        XCTAssertEqual(s.pending.last?.op, .move); XCTAssertEqual(s.pending.last?.at, .now); XCTAssertEqual(s.pending.last?.id, "c")
        XCTAssertEqual(s.onAir()?.id, "c"); XCTAssertEqual(ids(s), ["a", "b"], "what was ahead stays ahead")
    }

    func test13d_ReplayFromHeardPlacesAFreshCopyNow() {
        let (p, s) = makePlayer(["a", "b"].map { entry($0) })
        s.airedOnAir("a"); s.airedEnded("a", as: .heard, outcome: .finished)
        XCTAssertEqual(s.sheet.heard().first?.id, "a")
        p.perform(.replay(id: "a"))
        let op = s.pending.last!
        XCTAssertEqual(op.op, .place); XCTAssertEqual(op.at, .now); XCTAssertNotEqual(op.entry?.id, "a")
        XCTAssertEqual(op.entry?.title, "Title a"); XCTAssertEqual(s.onAir()?.title, "Title a"); XCTAssertEqual(s.onAir()?.state, .onAir)
        XCTAssertNil(p.perform(.replay(id: "b")), "an entry still ahead is not replayed")
    }

    // 14 a stale gesture is dropped by the reducer, not repaired here
    func test14_AGestureForAnEntryNoLongerAheadChangesNothing() {
        let (p, s) = makePlayer(["a", "b"].map { entry($0) })
        let v = s.sheet.version
        XCTAssertNil(p.perform(.playNow(id: "gone"))); XCTAssertNil(p.perform(.remove(id: "gone")))
        XCTAssertEqual(s.sheet.version, v)
    }

    // The page is empty on the device when the shape the module answers is not the shape the contract declares (an extra or missing field makes
    // the resource fail and the page shows its empty default). Every Swift field of the page's shapes is in interface/radio.json, and the other way.
    func test17_TheModulesAnswerHasExactlyTheFieldsTheContractDeclares() throws {
        let url = URL(fileURLWithPath: #filePath).deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().deletingLastPathComponent().appendingPathComponent("exact-package/interface/radio.json")
        let j = try XCTUnwrap(JSONSerialization.jsonObject(with: Data(contentsOf: url)) as? [String: Any])
        let shapes = try XCTUnwrap(j["shapes"] as? [String: [String: String]])
        func keys<T: Encodable>(_ v: T) throws -> Set<String> { Set(try XCTUnwrap(JSONSerialization.jsonObject(with: JSONEncoder().encode(v)) as? [String: Any]).keys) }
        var info = RadioPageInfo()
        var bo = RadioBuiltOn(); bo.title = "t"
        var row = RadioPageRow(); row.builtOn = [bo]
        var heard = RadioHeardRow(); heard.builtOn = [bo]
        info.ahead = [row]; info.heard = [heard]; info.builtOn = [bo]
        for (name, got) in [("RadioPageInfo", try keys(info)), ("RadioPageRow", try keys(row)), ("RadioHeardRow", try keys(heard)), ("RadioBuiltOn", try keys(bo))] {
            XCTAssertEqual(got, Set(try XCTUnwrap(shapes[name], name).keys), "\(name): the Swift fields and the declared shape differ")
        }
        let def = try XCTUnwrap(((j["names"] as? [[String: Any]])?.first { ($0["source"] as? String) == "radioPage" }?["substitute"] as? [String: Any])?["default"] as? [String: Any])
        XCTAssertEqual(Set(def.keys), Set(shapes["RadioPageInfo"]!.keys), "the substitute default has every field")
    }
}
