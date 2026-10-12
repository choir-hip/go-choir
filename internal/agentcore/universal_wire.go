package agentcore

func universalWirePlatformOwnerID() string {
	ownerID := firstNonEmptyEnv("SOURCE_SERVICE_RUNTIME_OWNER_ID", "SOURCECYCLED_RUNTIME_OWNER_ID")
	if ownerID == "" {
		ownerID = "universal-wire-platform"
	}
	return ownerID
}
