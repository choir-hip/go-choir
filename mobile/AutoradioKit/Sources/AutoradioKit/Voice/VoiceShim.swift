import Foundation
import os

// The two names the engine's pure files take from files that are not copied here (VoiceEngine.swift, OnDeviceSpeech.swift).
// Everything else in this folder is the Mike speech engine's code, unchanged; see docs/design/autoradio-voice-engine-2026-11-15.md.
enum VoiceMode {
    static let log = Logger(subsystem: "AutoradioKit", category: "voice")
}
enum OnDeviceSpeech {
    enum Failure: Error { case failed }
}
