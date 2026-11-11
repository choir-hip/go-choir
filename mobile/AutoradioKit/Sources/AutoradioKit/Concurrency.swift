import Foundation

// Small, shared concurrency pieces (written for the repair of the app review, 2026-11-11).
//
// WAYS THIS CAN FAIL (written before the code, each has a test in ConcurrencyTests):
//  1. A continuation resumed twice (a completion callback and a stop path or a timer both fire): Once resumes the first time only.
//  2. A continuation never resumed because the stop path ran BEFORE the wait began: Once remembers an early resume and
//     hands it to the continuation the moment it is armed.
//  3. A wait that ignores the task being cancelled: callers pair Once with withTaskCancellationHandler, and
//     withDeadline wakes at once (with nil) when its caller is cancelled and cancels the body and the timer.
//  4. Child tasks of a turn pipeline that outlive the pipeline: TaskBag cancels everything added to it, and a task added
//     after the bag was cancelled is cancelled on the spot.
//  5. Shared mutable state read and written from several threads: Locked is the one lock-and-value wrapper.

/// A value behind one lock. `read` and `write` run the closure with the lock held: no awaiting, no calling out inside.
final class Locked<T>: @unchecked Sendable {
    private let l = NSLock()
    private var v: T
    init(_ v: T) { self.v = v }
    func read<R>(_ f: (T) -> R) -> R { l.lock(); defer { l.unlock() }; return f(v) }
    @discardableResult func write<R>(_ f: (inout T) -> R) -> R { l.lock(); defer { l.unlock() }; return f(&v) }
    var value: T { get { read { $0 } } set { write { $0 = newValue } } }
}

/// Resumes one continuation exactly once, from whichever of several paths (a completion callback, a stop, a timer, a
/// cancellation) arrives first. `resume` before `arm` is remembered; a second `resume` does nothing.
final class Once<T>: @unchecked Sendable {
    private let l = NSLock()
    private var c: CheckedContinuation<T, Never>?
    private var early: T?
    private var done = false
    init() {}
    init(_ c: CheckedContinuation<T, Never>) { self.c = c }

    /// Give the continuation to wait on. If `resume` already happened it is resumed now.
    func arm(_ cont: CheckedContinuation<T, Never>) {
        l.lock()
        if done, let e = early { early = nil; l.unlock(); cont.resume(returning: e); return }
        c = cont
        l.unlock()
    }
    /// True when this call was the one that resumed (or recorded the answer).
    @discardableResult
    func resume(_ v: T) -> Bool {
        l.lock()
        if done { l.unlock(); return false }
        done = true
        if let x = c { c = nil; l.unlock(); x.resume(returning: v) } else { early = v; l.unlock() }
        return true
    }
    var isDone: Bool { l.lock(); defer { l.unlock() }; return done }
}

/// Tasks that must not outlive their owner. `cancelAll` is idempotent; `add` after it cancels the task at once.
final class TaskBag: @unchecked Sendable {
    private let l = NSLock()
    private var cancels: [@Sendable () -> Void] = []
    private var cancelled = false
    func add<S: Sendable, F: Error>(_ t: Task<S, F>) {
        l.lock()
        if cancelled { l.unlock(); t.cancel(); return }
        cancels.append { t.cancel() }
        l.unlock()
    }
    func cancelAll() {
        l.lock(); cancelled = true; let all = cancels; cancels = []; l.unlock()
        for c in all { c() }
    }
}

/// Run `body` with a deadline; nil if it did not finish in time or the caller was cancelled. The body is cancelled but not awaited.
func withDeadline<T: Sendable>(_ seconds: Double, _ body: @escaping @Sendable () async -> T) async -> T? {
    let once = Once<T?>()
    let tasks = TaskBag()
    return await withTaskCancellationHandler {
        await withCheckedContinuation { (c: CheckedContinuation<T?, Never>) in
            once.arm(c)
            tasks.add(Task { once.resume(await body()); tasks.cancelAll() })
            tasks.add(Task {
                try? await Task.sleep(nanoseconds: UInt64(max(0, seconds) * 1e9))
                if Task.isCancelled { return }
                once.resume(nil); tasks.cancelAll()
            })
        }
    } onCancel: {
        once.resume(nil); tasks.cancelAll()
    }
}
