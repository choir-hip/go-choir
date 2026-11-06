package deskprotocol

// Direction says which way a root crosses between desks. It decides whether
// a field added to the type is breaking: a required field added to a type a
// desk emits is major, because an older desk does not send it.
type Direction string

const (
	// Emitted types are built by one desk and decoded by another.
	Emitted Direction = "emitted"
	// Received types are produced by the host and read by a desk.
	Received Direction = "received"
	// Both is either.
	Both Direction = "both"
)

// Root is one starting point of the surface. The extractor includes every
// in-module type a root references, expanded, so a change inside a nested
// type is seen.
//
// Name is "pkg.Ident" for a type, constant or function, or "pkg.Prefix*" for
// a group of untyped constants (an enum spelled as consts).
type Root struct {
	Name      string
	Direction Direction
	// Open marks an enum (or constant group) readers must tolerate unknown
	// values of. A new value of an open enum is minor; of a closed one,
	// major. The default is closed.
	Open bool
	// Quota marks a numeric constant that is a bound: raising it is minor,
	// lowering it major.
	Quota bool
	// FromSource marks a function read from its source text rather than
	// type-checked, because its package is too heavy to import.
	FromSource bool
}

// Packages maps the short package names a Root uses to import paths.
var Packages = map[string]string{
	"types":        "github.com/yusefmosiah/go-choir/internal/types",
	"yaegikernel":  "github.com/yusefmosiah/go-choir/internal/yaegikernel",
	"agentcore":    "github.com/yusefmosiah/go-choir/internal/agentcore",
	"release":      "github.com/yusefmosiah/go-choir/internal/release",
	"deskmanifest": "github.com/yusefmosiah/go-choir/internal/deskmanifest",
}

// ChoirScopeType is the type whose methods are the choir verbs.
const ChoirScopeType = "yaegikernel.ChoirScope"

// ObservationVerbs are verbs every cell holds beyond a desk's own set
// (deskmanifest.Desk.Verbs). Pack is the acting pack of the PSA protocol.
var ObservationVerbs = []string{"Pack", "Records", "Context"}

// MinNames, MinVerbs and MinDesks are the floors that stop a broken
// extractor from producing a small surface that matches a small lock.
const (
	MinNames = 120
	MinVerbs = 40
	MinDesks = 6
)

// Roots lists the surface. The test for inclusion: does a different desk, a
// package desk or an older worker decode it. Store schemas, prompts and
// anything inside one desk are not here.
var Roots = []Root{
	// 1. Cast, delegation, ask, feed, decline, reply, report, escalate: request shapes.
	{Name: "yaegikernel.StagedIntent", Direction: Emitted},
	{Name: "yaegikernel.Intent*", Direction: Emitted},
	{Name: "yaegikernel.DelegateSpec", Direction: Emitted},
	{Name: "yaegikernel.TaskKind*", Direction: Emitted},
	{Name: "yaegikernel.MaxTaskCaptures", Direction: Emitted, Quota: true},
	{Name: "yaegikernel.Complete*", Direction: Emitted},
	{Name: "yaegikernel.MaxIntentsPerCell", Direction: Emitted, Quota: true},
	{Name: "yaegikernel.MaxIntentBody", Direction: Emitted, Quota: true},
	{Name: "yaegikernel.MaxTrayBytes", Direction: Emitted, Quota: true},
	{Name: "yaegikernel.TextureTurn", Direction: Emitted},
	{Name: "yaegikernel.TextureWrite", Direction: Emitted},
	{Name: "yaegikernel.TextureReject", Direction: Emitted},

	// The request record (MA1.3), sub-call records (MA1.5) and the inertness
	// flags (MA1.6): what a cell supplies with a request, and what the
	// reducer stamps. Origin classes and inert flags are open: a reader
	// tolerates a value it does not know.
	{Name: "types.RequestFields", Direction: Emitted},
	{Name: "types.ImpactClass", Direction: Both},
	{Name: "types.RequestCast", Direction: Both},
	{Name: "types.RequestAsk", Direction: Both},
	{Name: "types.RequestEscalate", Direction: Both},
	{Name: "types.RequestFeed", Direction: Both},
	{Name: "types.RequestDecline", Direction: Both},
	{Name: "types.RequestLease", Direction: Both},
	{Name: "types.RequestPrecommit", Direction: Both},
	{Name: "types.SettlementRecipient", Direction: Both},
	{Name: "types.Origin*", Direction: Both, Open: true},
	{Name: "types.CommitmentProvenance", Direction: Both},
	{Name: "types.MaterialityFreeze", Direction: Both},
	{Name: "types.TypedQuestion", Direction: Both},
	{Name: "types.CommitmentSubCall", Direction: Both},
	{Name: "types.SubCallVerdict", Direction: Both},
	{Name: "types.SubCallSelfAnswer", Direction: Both},
	{Name: "types.SubCallCast", Direction: Both},
	{Name: "types.SubCallReport", Direction: Both},
	{Name: "types.SubCallSettlement", Direction: Both},
	{Name: "types.SubCallSelfScore", Direction: Both},
	{Name: "types.SubCallJudgeWork", Direction: Both},
	{Name: "types.SubCallJudgeClose", Direction: Both},
	{Name: "types.SubCallInertJudgement", Direction: Both},
	// Desk-to-desk Casts judged by the host's judge (plan 4.4, amendment
	// 2026-11-05).
	{Name: "types.SubCallPlaneDeskCast", Direction: Both},
	{Name: "types.SubCallDeskOutcome", Direction: Both},
	{Name: "types.SubCallPhaseAnswered", Direction: Both},
	{Name: "types.SubCallPhaseFailed", Direction: Both},
	{Name: "types.SubCallPhaseInterrupted", Direction: Both},
	{Name: "types.SubCallLedgerAddressee", Direction: Both},
	{Name: "types.SubCallJudgeScorerID", Direction: Both},
	{Name: "types.SubCallRuleScorerPrefix", Direction: Both},
	{Name: "types.MaxSubCall*", Direction: Both, Quota: true},
	{Name: "types.InertBaseRate", Direction: Both},
	{Name: "types.InertNearCertain", Direction: Both},
	{Name: "types.InertNoFalsifier", Direction: Both},
	{Name: "types.InertNoDependency", Direction: Both},
	{Name: "types.InertTrivialTemplate", Direction: Both},
	{Name: "types.InertWeightBelowFloor", Direction: Both},

	// 2. Responses and deliveries.
	{Name: "yaegikernel.IncomingMessage", Direction: Received},
	{Name: "yaegikernel.InboxRecord", Direction: Received},
	{Name: "yaegikernel.InboxRequest", Direction: Received},
	{Name: "types.InboxCursorAdvance", Direction: Emitted},

	// 3. The commitment ledger.
	{Name: "types.CommitmentRecord", Direction: Both},
	{Name: "types.CommitmentRecordKind", Direction: Both},
	{Name: "types.CommitmentDirectiveSubtype", Direction: Both},
	{Name: "types.CommitmentCast", Direction: Both},
	{Name: "types.CastHypothesis", Direction: Both},
	{Name: "types.CastRevision", Direction: Both},
	{Name: "types.CastWithdrawal", Direction: Both},
	{Name: "types.CommitmentReport", Direction: Both},
	{Name: "types.ReportClaim", Direction: Both},
	{Name: "types.ReportFinding", Direction: Both},
	{Name: "types.HypothesisSettlement", Direction: Both},
	{Name: "types.HypothesisDrop", Direction: Both},
	{Name: "types.DiscrepancyClass", Direction: Both},
	{Name: "types.CommitmentEmitterKind", Direction: Both, Open: true},
	{Name: "types.CommitmentRecordSchemaV1", Direction: Both},
	{Name: "types.CastPublishAuto", Direction: Both},
	{Name: "types.HypothesisKind*", Direction: Both},

	// PSA, the precommitment-records protocol: the rule types, the
	// settlement and evidence shapes, and the release precommitment
	// (owner ruling 2026-11-04: desks message each other only via PSA).
	{Name: "types.PrecommitRuleKind", Direction: Both},
	{Name: "types.PrecommitRule", Direction: Both},
	{Name: "types.PrecommitAnswer*", Direction: Both},
	{Name: "types.PrecommitRule*", Direction: Both, Open: true},
	{Name: "types.RuleScorer*", Direction: Both},
	{Name: "types.RuleRef*", Direction: Both},
	{Name: "types.RuleStage*", Direction: Both},
	{Name: "types.RuleSettlement", Direction: Both},
	{Name: "types.RuleTiming", Direction: Both},
	{Name: "types.CommitmentPrecommit", Direction: Both},
	{Name: "types.CommitmentResolve", Direction: Both},
	{Name: "types.CommitmentDisagreement", Direction: Both},
	{Name: "types.EngineeringReceiptExit", Direction: Both},
	{Name: "types.EngineeringFreezeObservation", Direction: Both},
	{Name: "types.EngineeringVerifyObservation", Direction: Both},
	{Name: "types.EngineeringOwnerActObservation", Direction: Both},
	{Name: "types.EngineeringRequestObservation", Direction: Both},
	{Name: "types.EngineeringTerminalObservation", Direction: Both},
	{Name: "types.SayDoRecord", Direction: Received},
	{Name: "types.ActingPack", Direction: Received},
	{Name: "release.Precommitment", Direction: Both},

	// 4. Work item and trajectory lifecycle.
	{Name: "types.WorkItemRecord", Direction: Both},
	{Name: "types.WorkItemStatus", Direction: Both},
	{Name: "types.TrajectoryRecord", Direction: Both},
	{Name: "types.TrajectoryStatus", Direction: Both},
	{Name: "types.TrajectoryKind", Direction: Both, Open: true},
	{Name: "types.SettlementRule", Direction: Both},

	// 5. Engineering assignment.
	{Name: "types.EngineeringAssignment", Direction: Both},
	{Name: "types.EngineeringAssignmentReport", Direction: Both},
	{Name: "types.OpenEngineeringAssignmentRequest", Direction: Emitted},
	{Name: "types.BindEngineeringAssignmentRequest", Direction: Emitted},
	{Name: "types.RecordEngineeringAssignmentReportRequest", Direction: Emitted},
	{Name: "types.CancelEngineeringAssignmentRequest", Direction: Emitted},
	{Name: "types.SetEngineeringCapsuleDispositionRequest", Direction: Emitted},
	{Name: "types.EngineeringAssignmentBinding", Direction: Both},
	{Name: "types.EngineeringAssignmentDisposition", Direction: Both},
	{Name: "types.EngineeringCapsuleDisposition", Direction: Both},
	{Name: "types.EngineeringAssignmentResultKind", Direction: Both},
	{Name: "types.EngineeringAssignmentVerdict", Direction: Both},
	{Name: "types.EngineeringAssignmentKind", Direction: Both},
	{Name: "types.EngineeringCastAuthority", Direction: Both},
	{Name: "types.EngineeringOrphanReason", Direction: Both, Open: true},

	// 7. Who may address whom is not locked (owner ruling R5): the resolver
	// reads the registry, and the lock records no allowlist.

	// 8. The broker wire.
	{Name: "yaegikernel.ProtocolVersion", Direction: Both},
	{Name: "yaegikernel.BrokerRequest", Direction: Emitted},
	{Name: "yaegikernel.BrokerResponse", Direction: Received},
	{Name: "yaegikernel.BrokerAction", Direction: Emitted},

	// The rest of the broker wire and the packet transports a worker of
	// another build decodes (Devin amendment 1: anything a different desk or an
	// older worker decodes), and the job intent operations.
	{Name: "yaegikernel.JobOp*", Direction: Emitted},
	{Name: "yaegikernel.ExecPayload", Direction: Emitted},
	{Name: "yaegikernel.ExecResult", Direction: Received},
	{Name: "yaegikernel.ReadFilePayload", Direction: Emitted},
	{Name: "yaegikernel.ReadFileResult", Direction: Received},
	{Name: "yaegikernel.WriteFilePayload", Direction: Emitted},
	{Name: "yaegikernel.WriteFileResult", Direction: Received},
	{Name: "yaegikernel.ListDirPayload", Direction: Emitted},
	{Name: "yaegikernel.ListDirResult", Direction: Received},
	{Name: "yaegikernel.ResearchPayload", Direction: Emitted},
	{Name: "yaegikernel.ResearchChild", Direction: Emitted},
	{Name: "yaegikernel.ResearchChildResult", Direction: Received},
	{Name: "yaegikernel.ProductAPIPayload", Direction: Emitted},
	{Name: "yaegikernel.StoriesPayload", Direction: Emitted},
	{Name: "yaegikernel.StoryEntry", Direction: Received},
	{Name: "yaegikernel.StoriesPage", Direction: Received},
	{Name: "yaegikernel.HeldCaptures", Direction: Received},
	{Name: "yaegikernel.ReadPayload", Direction: Emitted},
	{Name: "yaegikernel.PollSourcePayload", Direction: Emitted},
	{Name: "yaegikernel.SourceAPIPayload", Direction: Emitted},
	{Name: "yaegikernel.ReadCapturePayload", Direction: Emitted},
	{Name: "yaegikernel.RequestLeasePayload", Direction: Emitted},
	{Name: "yaegikernel.ObjectGetPayload", Direction: Emitted},
	{Name: "yaegikernel.ObjectListPayload", Direction: Emitted},
	{Name: "yaegikernel.SearchWireCorpusPayload", Direction: Emitted},
	{Name: "yaegikernel.TextureSearchPayload", Direction: Emitted},
	{Name: "yaegikernel.SaveEvidencePayload", Direction: Emitted},
	{Name: "yaegikernel.ReadEvidencePayload", Direction: Emitted},
	{Name: "yaegikernel.ListEvidencePayload", Direction: Emitted},
	{Name: "yaegikernel.GetRunMemoryEntryPayload", Direction: Emitted},
	{Name: "yaegikernel.RelatePayload", Direction: Emitted},
	{Name: "yaegikernel.RelateStoriesPayload", Direction: Emitted},
	{Name: "yaegikernel.SearchStartPayload", Direction: Emitted},
	{Name: "yaegikernel.SearchStartResult", Direction: Received},
	{Name: "yaegikernel.System1Payload", Direction: Emitted},
	{Name: "yaegikernel.PendingUpdate", Direction: Received},
	{Name: "types.SourcePacket", Direction: Both},

	// The desk names are locked through the Desks table the extractor reads
	// from the registry, not spelled here.
	{Name: "types.ManagementTask", Direction: Both},
}
