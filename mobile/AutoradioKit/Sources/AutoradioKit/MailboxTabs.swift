import Foundation

// MailboxTabs: the Mail page's tab row as a model, so the motion rule "the frame never leaves" is testable without a screen.
//
// Failure list (each has a test in MailboxTabsTests):
//  1. Tapping a tab replaced the page with the words "Loading your mail" and the tab row vanished until the fetch came back.
//  2. A tab tapped during a load was not obeyed: the page followed the slower answer.
//  3. The transition began only when the fetch returned, so the tap gave no feedback.
//  4. A folder seen before showed nothing on a return visit until it was fetched again.
//  5. Any rendered state could carry the word "Loading" or "Opening".
//
// The rule: the frame (title, the four tabs with their counts) is `service.frame`, a value that needs no fetch. `select` is
// synchronous: the same call that names the new tab sets the transition (the outgoing content leaves in the tab's direction)
// before any fetch is started. A tab with a cached list runs the whole transition at once; one without holds the skeleton until
// its answer arrives. An answer for a tab that is no longer selected is discarded.

public enum TabPhase: String, Sendable {
    /// Nothing is moving.
    case settled
    /// The tap happened: the old content is leaving and a skeleton (no words) holds the place until the answer arrives.
    case leaving
    /// The new content is in and is sliding to rest (cached content enters at once, fetched content when it arrives).
    case entering
}

public struct MailboxTabsState: Equatable, Sendable {
    public var frame: MailBoxView
    public var phase: TabPhase
    /// +1: the new tab is to the right of the old one, so the old content leaves to the left; -1 the other way; 0 none.
    public var direction: Int
    public var selected: String
    /// What the content area shows: the rows (cached or fetched), or none (then it shows the skeleton while `phase` is leaving).
    public var rows: [MailRow]
    public var showsSkeleton: Bool
    /// "" until a fetch answers for the selected tab, then ready | empty | offline | error; the words of an error are `message`.
    public var status: String
    public var message: String
    /// Everything a person can read in this state, for the check that no word of waiting appears.
    public var words: [String] {
        frame.folders.map { $0.badge.isEmpty ? $0.label : "\($0.label) \($0.badge)" } + [frame.title, message] + rows.flatMap { [$0.sender, $0.subject, $0.snippet] }
    }
}

public final class MailboxTabs: @unchecked Sendable {
    private let service: MailService
    private let lock = NSLock()
    private var generation = 0
    private var s: MailboxTabsState
    public var onChange: (() -> Void)?

    public init(service: MailService, folder: String = "inbox") {
        self.service = service
        let f = service.frame(address: "mailbox:" + folder)
        s = MailboxTabsState(frame: f, phase: .settled, direction: 0, selected: folder, rows: service.cachedRows(folder: folder) ?? [], showsSkeleton: service.cachedRows(folder: folder) == nil, status: "", message: "")
    }

    public var state: MailboxTabsState { lock.lock(); defer { lock.unlock() }; return s }

    /// Synchronous. Returns the generation the answer must carry to be accepted.
    @discardableResult
    public func select(_ folder: String) -> Int {
        lock.lock()
        generation += 1
        let g = generation
        let names = MailService.folders.map(\.name)
        let from = names.firstIndex(of: s.selected) ?? 0, to = names.firstIndex(of: folder) ?? 0
        s.direction = to == from ? 0 : (to > from ? 1 : -1)
        s.selected = folder
        s.frame = service.frame(address: "mailbox:" + folder)
        s.status = ""; s.message = ""
        if let c = service.cachedRows(folder: folder) {
            s.rows = c; s.showsSkeleton = false; s.phase = .entering      // cached: the whole transition runs now
        } else {
            s.phase = .leaving; s.showsSkeleton = true                     // outgoing content leaves; the skeleton holds the place
        }
        lock.unlock()
        onChange?()
        return g
    }

    /// Selects, fetches and applies the answer. A newer `select` makes this one's answer a no-op.
    public func open(_ folder: String) async {
        let g = select(folder)
        let v = await service.box(address: "mailbox:" + folder)
        accept(generation: g, view: v)
    }

    @discardableResult
    public func accept(generation g: Int, view v: MailBoxView) -> Bool {
        lock.lock()
        guard g == generation, v.status != "" else { lock.unlock(); return false }
        if !v.folders.isEmpty { s.frame.folders = v.folders; s.frame.title = v.title }   // the same four tabs, now with the fresh counts
        s.rows = v.rows; s.showsSkeleton = false; s.status = v.status; s.message = v.message
        s.phase = .entering
        lock.unlock()
        onChange?()
        return true
    }

    /// The enter slide has finished.
    public func settle() { lock.lock(); s.phase = .settled; lock.unlock(); onChange?() }
}
