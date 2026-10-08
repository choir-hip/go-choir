package projectionbase

import "github.com/yusefmosiah/go-choir/internal/recoveryplan"

// MaxRecoveryTailEvents is the largest (start, H] a recovery path may apply.
const MaxRecoveryTailEvents = recoveryplan.MaxRecoveryTailEvents

// RecoveryAction is the boot/refresh/rematerialize decision for one retained
// store, advertised watermark, and frozen target head.
type RecoveryAction = recoveryplan.RecoveryAction

const (
	RecoveryRefuse  = recoveryplan.RecoveryRefuse
	RecoveryGenesis = recoveryplan.RecoveryGenesis
	RecoveryResume  = recoveryplan.RecoveryResume
	RecoveryInstall = recoveryplan.RecoveryInstall
	RecoveryRebase  = recoveryplan.RecoveryRebase
)

// RecoveryPlan is the pure local/W/H decision.
type RecoveryPlan = recoveryplan.RecoveryPlan

// PlanRecovery decides how a computer may reach H without lifetime-replaying.
var PlanRecovery = recoveryplan.PlanRecovery
