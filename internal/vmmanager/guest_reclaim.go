package vmmanager

import (
	"log"
	"os"
	"strconv"
	"strings"
)

// Proactive reclaim of cold guest page cache (RES-06; P4 of
// docs/archive/definitions/choir-launch-fp8-image-jailer-2026-10-21.md).
//
// Free page reporting returns to the host only pages the guest frees, and a
// busy guest frees page cache only under memory pressure, so a computer's
// host RSS tracks the most cache it ever held (the paper: about 7.9 GiB on
// the host against about 2.8 GB used). The guest kernel has DAMON_RECLAIM
// built in (CONFIG_DAMON_RECLAIM=y): it finds regions not accessed for
// min_age and reclaims them, and reporting then hands the freed blocks back.
// Built-in module parameters are kernel boot arguments, so a selected
// computer turns it on with no guest image change:
//
//	VM_GUEST_RECLAIM_ONLY        VM ids, computer ids, class:test or *
//	                             (computer_select.go); empty: off
//	VM_GUEST_RECLAIM_MIN_AGE_S   coldness floor, default 300 (5 minutes)
//
// Watermarks are free-memory permille. DAMON_RECLAIM's defaults (high 500,
// mid 400, low 200) deactivate below 20% free, which is exactly the state
// of a guest full of cold cache; ours run it from 90% free down to 0.5%.
// skip_anon keeps it off anonymous memory (the guest has no swap). Quotas
// keep the defaults (10 ms CPU and 128 MiB per second). Boot arguments are
// in the launch-config hash, so a snapshot taken before selection is
// refused and the computer cold boots once.

const (
	defaultGuestReclaimMinAgeS = 300
	maxGuestReclaimMinAgeS     = 86400
)

func loadGuestReclaimFromEnv(cfg *ManagerConfig) {
	cfg.GuestReclaimOnly = parseComputerSelector(os.Getenv("VM_GUEST_RECLAIM_ONLY"))
	if v := strings.TrimSpace(os.Getenv("VM_GUEST_RECLAIM_MIN_AGE_S")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > maxGuestReclaimMinAgeS {
			log.Printf("vmmanager: VM_GUEST_RECLAIM_MIN_AGE_S=%q ignored (want 1..%d seconds); using %d", v, maxGuestReclaimMinAgeS, defaultGuestReclaimMinAgeS)
			return
		}
		cfg.GuestReclaimMinAgeS = n
	}
}

// guestReclaimKernelParams are the boot arguments for cfg's launch; nil for
// a computer the list does not name. enabled comes last, after the
// parameters it starts with.
func (m *Manager) guestReclaimKernelParams(cfg VMConfig) []string {
	if !selectsComputer(m.cfg.GuestReclaimOnly, cfg) {
		return nil
	}
	age := m.cfg.GuestReclaimMinAgeS
	if age <= 0 {
		age = defaultGuestReclaimMinAgeS
	}
	return []string{
		"damon_reclaim.min_age=" + strconv.Itoa(age*1_000_000),
		"damon_reclaim.wmarks_high=950",
		"damon_reclaim.wmarks_mid=900",
		"damon_reclaim.wmarks_low=5",
		"damon_reclaim.skip_anon=Y",
		"damon_reclaim.enabled=Y",
	}
}
