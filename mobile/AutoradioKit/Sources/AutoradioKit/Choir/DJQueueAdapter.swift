import Foundation

// The "Hear it" plays on a story's page (the queue's "Open episode" is shared code: radio-bridge.ts `openEpisodeOf`).
// Plays go onto the run sheet (shared code: RadioBridge.place): Play is `place now`, Add to queue is `place queued`.

/// Play and Add to queue of a clip. The rows of "Hear it" and a show's "Clips" are shared code (AutoradioLab/clips.ts).
public enum HearIt {
    /// Play: the clip when it can be found in the served file (the player locates it); else the episode from its start. (The shared row offers Play only for a usable fingerprint; the phone no longer marks a mismatch from any live path.)
    @MainActor public static func play(_ clipID: String, store: ClipStore = .shared) async {
        guard let c = store.clip(clipID) else { return }
        RadioBridge.shared.place(c.isRelocatable ? c.clipEntry : c.episodeEntry, mode: "now")
    }
    @MainActor public static func enqueue(_ clipID: String, store: ClipStore = .shared) {
        if let c = store.clip(clipID) { RadioBridge.shared.place(c.isRelocatable ? c.clipEntry : c.episodeEntry, mode: "add") }
    }
}
