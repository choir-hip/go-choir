import Foundation

// What native code still names of the run sheet: the state and outcome the deck reports (Deck.swift `airedEnd`). The sheet itself, its entries and ops are shared code's (AutoradioLab/runsheet-model.ts); native decodes none of it. What the voice
// and the take recorder read of the sheet is pushed to them by shared code (AutoradioLab/voice-facts.ts, RadioBridge).

public enum RunSheetState: String, Codable, Equatable, Sendable { case ahead, onAir = "on_air", heard, skipped }
/// `failed` (amendment to the frozen interface): the phone could not make the entry play. It is not the listener's judgement and settles nothing.
public enum RunSheetOutcome: String, Codable, Equatable, Sendable { case finished, skipped, cut, left, failed }
