// Package agentprofile defines the agent profile identifiers used throughout
// the runtime. These constants were extracted from internal/runtime so that
// packages can reference agent profiles without importing the full runtime.
package agentprofile

const (
	Conductor  = "conductor"
	Super      = "super"
	CoSuper    = "co-super"
	VSuper     = "vsuper"
	Researcher = "researcher"
	Texture    = "texture"
	Processor  = "processor"
	Reconciler = "reconciler"
	Email      = "email"
)
