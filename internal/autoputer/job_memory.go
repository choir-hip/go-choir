package autoputer

import (
	"github.com/yusefmosiah/go-choir/internal/capsule"
)

// configureJobMemory installs the live capsule budget: a job that declares
// memory_mib is admitted against 3/4 of the guest's MemTotal as it reads
// now. staticBudget is true when CHOIR_CAPSULE_MEMORY_TOTAL_BYTES pins it.
// (The virtio-mem lease hook that once raised MemTotal on demand was
// deleted: the hotplug controller was disproved on Node B.)
func configureJobMemory(executor *capsule.Executor, staticBudget bool) {
	if !staticBudget {
		executor.SetLiveMemoryBudget(capsule.LiveMemoryBudget(nil))
	}
}
