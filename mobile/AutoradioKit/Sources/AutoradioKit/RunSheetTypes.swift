import Foundation

// What native code still names of the run sheet: the state and outcome the deck reports (Deck.swift `airedEnd`) and the forecast the programme decoder carries
// (Choir/RadioProgramme.swift). The sheet itself, its entries and ops are shared code's (AutoradioLab/runsheet-model.ts); native decodes none of it. What the voice
// and the take recorder read of the sheet is pushed to them by shared code (AutoradioLab/voice-facts.ts, RadioBridge).

public enum RunSheetState: String, Codable, Equatable, Sendable { case ahead, onAir = "on_air", heard, skipped }
/// `failed` (amendment to the frozen interface): the phone could not make the entry play. It is not the listener's judgement and settles nothing.
public enum RunSheetOutcome: String, Codable, Equatable, Sendable { case finished, skipped, cut, left, failed }

public struct RunSheetForecast: Decodable, Equatable, Sendable {
    /// Nil when the phone holds only the claim id: the server never sends its probability or cause on the `segment` event (they are written to
    /// the tape and not sent), and the phone invents none.
    public var pHeard: Double?
    public var causedBy: String
    public var claimId: String
    public init(pHeard: Double?, causedBy: String, claimId: String) { self.pHeard = pHeard; self.causedBy = causedBy; self.claimId = claimId }
    enum CodingKeys: String, CodingKey { case pHeard = "p_heard", causedBy = "caused_by", claimId = "claim_id" }
    public init(from d: Decoder) throws {
        let c = try d.container(keyedBy: CodingKeys.self)
        pHeard = try c.decodeIfPresent(Double.self, forKey: .pHeard)
        causedBy = try c.decodeIfPresent(String.self, forKey: .causedBy) ?? ""
        claimId = try c.decodeIfPresent(String.self, forKey: .claimId) ?? ""
    }
}
