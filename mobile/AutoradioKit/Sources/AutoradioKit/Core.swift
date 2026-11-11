import Foundation

// MARK: JSON values

/// A JSON value, used for stage parameters and anywhere a record holds free-form data.
public enum JSONValue: Codable, Equatable, Sendable, CustomStringConvertible {
    case null
    case bool(Bool)
    case number(Double)
    case string(String)
    case array([JSONValue])
    case object([String: JSONValue])

    public init(from decoder: Decoder) throws {
        let c = try decoder.singleValueContainer()
        if c.decodeNil() { self = .null }
        else if let b = try? c.decode(Bool.self) { self = .bool(b) }
        else if let n = try? c.decode(Double.self) { self = .number(n) }
        else if let s = try? c.decode(String.self) { self = .string(s) }
        else if let a = try? c.decode([JSONValue].self) { self = .array(a) }
        else { self = .object(try c.decode([String: JSONValue].self)) }
    }

    public func encode(to encoder: Encoder) throws {
        var c = encoder.singleValueContainer()
        switch self {
        case .null: try c.encodeNil()
        case .bool(let b): try c.encode(b)
        case .number(let n): try c.encode(n)
        case .string(let s): try c.encode(s)
        case .array(let a): try c.encode(a)
        case .object(let o): try c.encode(o)
        }
    }

    public var description: String {
        switch self {
        case .null: return "null"
        case .bool(let b): return String(b)
        case .number(let n): return n == n.rounded() && abs(n) < 1e15 ? String(Int(n)) : String(n)
        case .string(let s): return s
        case .array(let a): return "[" + a.map(\.description).joined(separator: ",") + "]"
        case .object(let o): return "{" + o.keys.sorted().map { "\($0):\(o[$0]!)" }.joined(separator: ",") + "}"
        }
    }
}

// MARK: Stage kinds, descriptors, parameters

/// One job in the path. Each kind has exactly one Swift protocol (see Stages.swift).
public enum StageKind: String, Codable, CaseIterable, Sendable {
    case micSource, addressing, speechBounds, speakerCheck, transcriber, yieldPolicy
    case system1, holder, voice, desk, programme
}

public struct ParamSpec: Codable, Sendable, Equatable {
    public enum Kind: String, Codable, Sendable { case number, int, bool, string, choice }
    public var name: String
    public var label: String
    public var kind: Kind
    public var defaultValue: JSONValue
    public var min: Double?
    public var max: Double?
    public var choices: [String]?
    public var note: String?
    public init(_ name: String, _ label: String, _ kind: Kind, default d: JSONValue,
                min: Double? = nil, max: Double? = nil, choices: [String]? = nil, note: String? = nil) {
        self.name = name; self.label = label; self.kind = kind; self.defaultValue = d
        self.min = min; self.max = max; self.choices = choices; self.note = note
    }
    public static func number(_ n: String, _ l: String, _ d: Double, _ lo: Double, _ hi: Double, note: String? = nil) -> ParamSpec {
        ParamSpec(n, l, .number, default: .number(d), min: lo, max: hi, note: note)
    }
    public static func text(_ n: String, _ l: String, _ d: String, note: String? = nil) -> ParamSpec {
        ParamSpec(n, l, .string, default: .string(d), note: note)
    }
    public static func flag(_ n: String, _ l: String, _ d: Bool) -> ParamSpec { ParamSpec(n, l, .bool, default: .bool(d)) }
    public static func choice(_ n: String, _ l: String, _ d: String, _ cs: [String]) -> ParamSpec {
        ParamSpec(n, l, .choice, default: .string(d), choices: cs)
    }
}

public struct StageDescriptor: Codable, Sendable, Equatable {
    public var displayName: String
    public var summary: String
    public var params: [ParamSpec]
    /// Model ids the ModelStore must hold for this stage (empty for Apple-framework stages).
    public var needsModels: [String]
    public var needsNetwork: Bool
    public init(displayName: String, summary: String = "", params: [ParamSpec] = [],
                needsModels: [String] = [], needsNetwork: Bool = false) {
        self.displayName = displayName; self.summary = summary; self.params = params
        self.needsModels = needsModels; self.needsNetwork = needsNetwork
    }
}

/// Parameters of one stage instance: the configuration's values over the descriptor's defaults.
public struct StageParams: Sendable {
    public let values: [String: JSONValue]
    public init(_ values: [String: JSONValue] = [:]) { self.values = values }
    public func double(_ k: String, _ d: Double = 0) -> Double { if case .number(let n)? = values[k] { return n }; return d }
    public func int(_ k: String, _ d: Int = 0) -> Int { Int(double(k, Double(d))) }
    public func bool(_ k: String, _ d: Bool = false) -> Bool { if case .bool(let b)? = values[k] { return b }; return d }
    public func string(_ k: String, _ d: String = "") -> String { if case .string(let s)? = values[k] { return s }; return d }
}

// MARK: Stage protocols' common base

/// Every stage implementation is a class with a stable id and a descriptor.
public protocol Stage: AnyObject, Sendable {
    static var stageID: String { get }
    static var descriptor: StageDescriptor { get }
}

// MARK: Configuration

public struct StageSetting: Codable, Sendable, Equatable {
    public var id: String
    public var params: [String: JSONValue]
    public init(id: String, params: [String: JSONValue] = [:]) { self.id = id; self.params = params }
}

/// A named choice of one implementation per stage, with parameters. Pure data.
/// How the one audio session is shared between listening (full-quality playback, phone mic, no voice
/// processing) and talking (voiceChat, voice processing, Bluetooth hands-free allowed).
public enum AudioPolicy: String, Codable, Sendable, CaseIterable {
    /// Listening until a turn is addressed, talking while it is open, listening again after.
    case switchOnAddress
    /// Talking (voice processing) for the whole run: interruptible answers, call-quality programme.
    case alwaysTalking
    /// Listening for the whole run: the phone's own microphone, never switches.
    case alwaysListening
    public var label: String {
        switch self { case .switchOnAddress: return "Switch on address"; case .alwaysTalking: return "Always talking"; case .alwaysListening: return "Always listening" }
    }
    /// The state a request for `asked` becomes under this policy.
    public func effective(_ asked: AudioState) -> AudioState {
        switch self { case .switchOnAddress: return asked; case .alwaysTalking: return .talking; case .alwaysListening: return .listening }
    }
}

/// When the holder's first acknowledgement starts. `immediate`: as soon as the turn is the listener's, in parallel
/// with System 1 and the desk; System 1's answers select only what follows. `afterSystem1`: the line is chosen from
/// System 1's answers, so it waits for them.
public enum AckTiming: String, Codable, Sendable, CaseIterable {
    case immediate, afterSystem1
}

public struct LabConfiguration: Codable, Sendable, Equatable {
    public var name: String
    public var audioPolicy: AudioPolicy?
    public var audio: AudioPolicy { audioPolicy ?? .switchOnAddress }
    public var ackTiming: AckTiming?
    public var ack: AckTiming { ackTiming ?? .immediate }
    public var stages: [String: StageSetting]   // keyed by StageKind.rawValue
    public init(name: String, stages: [StageKind: StageSetting] = [:]) {
        self.name = name
        self.stages = Dictionary(uniqueKeysWithValues: stages.map { ($0.key.rawValue, $0.value) })
    }
    public func setting(_ k: StageKind) -> StageSetting? { stages[k.rawValue] }
    public mutating func set(_ k: StageKind, _ s: StageSetting) { stages[k.rawValue] = s }

    public static func load(from url: URL) throws -> LabConfiguration {
        try JSONDecoder().decode(LabConfiguration.self, from: Data(contentsOf: url))
    }
    /// Fetch a configuration pushed from the owner's computer (any http(s) URL, or a file URL).
    public static func fetch(_ url: URL) async throws -> LabConfiguration {
        if url.isFileURL { return try load(from: url) }
        let (data, resp) = try await URLSession.shared.data(from: url)
        if let h = resp as? HTTPURLResponse, !(200..<300).contains(h.statusCode) { throw LabError.http(h.statusCode) }
        return try JSONDecoder().decode(LabConfiguration.self, from: data)
    }
    public func write(to url: URL) throws {
        let e = JSONEncoder(); e.outputFormatting = [.prettyPrinted, .sortedKeys]
        try e.encode(self).write(to: url, options: .atomic)
    }
}

public enum LabError: Error, CustomStringConvertible {
    case http(Int), unknownStage(StageKind, String), noStage(StageKind), badInput(String), unavailable(String)
    public var description: String {
        switch self {
        case .http(let c): return "HTTP \(c)"
        case .unknownStage(let k, let id): return "no \(k.rawValue) stage '\(id)' registered"
        case .noStage(let k): return "no \(k.rawValue) stage in configuration and none registered"
        case .badInput(let s): return "bad input: \(s)"
        case .unavailable(let s): return "unavailable: \(s)"
        }
    }
}

// MARK: Registry

public struct RegisteredStage: Sendable {
    public let kind: StageKind
    public let id: String
    public let descriptor: StageDescriptor
}

public final class StageRegistry: @unchecked Sendable {
    public typealias Factory = @Sendable (StageParams) throws -> AnyObject
    private let lock = NSLock()
    private var entries: [StageKind: [(RegisteredStage, Factory)]] = [:]
    public init() {}

    public func register(_ kind: StageKind, id: String, descriptor: StageDescriptor, factory: @escaping Factory) {
        lock.lock(); defer { lock.unlock() }
        entries[kind, default: []].removeAll { $0.0.id == id }
        entries[kind, default: []].append((RegisteredStage(kind: kind, id: id, descriptor: descriptor), factory))
    }
    /// Register using the type's own `stageID` and `descriptor`.
    public func register<S: Stage>(_ kind: StageKind, _ type: S.Type, factory: @escaping @Sendable (StageParams) throws -> S) {
        register(kind, id: S.stageID, descriptor: S.descriptor, factory: factory)
    }
    public func stages(_ kind: StageKind) -> [RegisteredStage] {
        lock.lock(); defer { lock.unlock() }
        return (entries[kind] ?? []).map(\.0)
    }
    public var all: [RegisteredStage] { StageKind.allCases.flatMap { stages($0) } }

    /// Defaults first, then the configuration's values. Unknown keys are kept so a newer config round-trips.
    public func params(_ kind: StageKind, _ setting: StageSetting) -> StageParams {
        var v: [String: JSONValue] = [:]
        if let d = stages(kind).first(where: { $0.id == setting.id })?.descriptor {
            for p in d.params { v[p.name] = p.defaultValue }
        }
        for (k, x) in setting.params { v[k] = x }
        return StageParams(v)
    }

    /// Make the configuration's stage for `kind`; falls back to the first registered when the configuration is silent.
    public func make(_ kind: StageKind, _ config: LabConfiguration) throws -> (instance: AnyObject, setting: StageSetting) {
        lock.lock(); let list = entries[kind] ?? []; lock.unlock()
        let setting: StageSetting
        if let s = config.setting(kind) { setting = s }
        else if let first = list.first { setting = StageSetting(id: first.0.id) }
        else { throw LabError.noStage(kind) }
        guard let entry = list.first(where: { $0.0.id == setting.id }) else { throw LabError.unknownStage(kind, setting.id) }
        return (try entry.1(params(kind, setting)), setting)
    }
    public func make<T>(_ kind: StageKind, as: T.Type, _ config: LabConfiguration) throws -> T {
        let (o, s) = try make(kind, config)
        guard let t = o as? T else { throw LabError.badInput("stage \(s.id) does not conform to \(T.self)") }
        return t
    }
}
