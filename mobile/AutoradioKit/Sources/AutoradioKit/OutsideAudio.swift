import Foundation

/// Sound the engine does not play itself (a podcast through AVPlayer) has no reference signal for `ReferenceSpeechDetector`. While it plays,
/// voice is on with voice processing, which cancels it from the microphone.
public enum OutsideAudio {
    private static let flag = Locked<Bool>(false)
    public static var isPlaying: Bool { flag.value }
    /// Called when the flag changes (the session follows).
    nonisolated(unsafe) public static var onChange: (@Sendable () -> Void)?
    public static func set(_ playing: Bool) {
        let changed = flag.write { let was = $0; $0 = playing; return was != playing }
        if changed { onChange?() }
    }
}
