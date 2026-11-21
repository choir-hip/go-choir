import Foundation

// The steer taps (POST /api/autoradio/steer: more, less, mute) and their queue are shared code (AutoradioLab/steer.ts), sent through the one signed-in client.
// Only the route name of a story ref stays here, for the turn wire (TurnQueueWire.swift), which names the stories the entry on air cites.
public enum SteerTap {
    /// The route a `pub:<route>[@v<n>]` ref names, without the version.
    static func story(_ ref: String) -> String {
        let r = ref.hasPrefix("pub:") ? String(ref.dropFirst(4)) : ref
        return TextureAddress.splitRoute(String(r.split(separator: "@", maxSplits: 1, omittingEmptySubsequences: false).first ?? "")).bare
    }
}
