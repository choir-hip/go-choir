package apppackage

// GrantUIShell is the grant a T2 package holds: it draws part of the shell's
// own screen and calls the shell-wide SDK functions.
const GrantUIShell = "ui.shell"

// SDKNameGrants are the SDK names that imply a grant: a copy of the SDK
// lock's sections.grants (frontend/src/lib/sdk-surface/sections.json), which
// check-packages.mjs reads for the in-tree build. The installing computer
// compares a module's extracted uses with it (pkgrelease.CheckUses, slice
// L3). TestSDKNameGrantsAreTheLock keeps the two equal; change both together.
var SDKNameGrants = map[string]string{
	"closeOtherWindows":             GrantUIShell,
	"focus":                         GrantUIShell,
	"listWindows":                   GrantUIShell,
	"logout":                        GrantUIShell,
	"providerSlots":                 GrantUIShell,
	"resetDesktop":                  GrantUIShell,
	"restore":                       GrantUIShell,
	"selectProvider":                GrantUIShell,
	"setTheme":                      GrantUIShell,
	"showDesktop":                   GrantUIShell,
	"showOverview":                  GrantUIShell,
	"submitPrompt":                  GrantUIShell,
	"suspendBackgroundHeavyWindows": GrantUIShell,
	"useDefaultProvider":            GrantUIShell,
	"windowAction":                  GrantUIShell,
	"windowList":                    GrantUIShell,
}
