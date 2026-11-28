import Foundation

/// Run on the main actor from any thread (the lock screen's commands arrive on whichever thread the system chooses).
func onMainThread<T>(_ f: @MainActor () -> T) -> T {
    if Thread.isMainThread { return MainActor.assumeIsolated(f) }
    return DispatchQueue.main.sync { MainActor.assumeIsolated(f) }
}
