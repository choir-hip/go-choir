package agentcore

func universalWirePlatformOwnerID() string {
	ownerID := firstNonEmptyEnv("SOURCE_SERVICE_RUNTIME_OWNER_ID", "SOURCECYCLED_RUNTIME_OWNER_ID")
	if ownerID == "" {
		ownerID = "universal-wire-platform"
	}
	return ownerID
}

func wirePlatformRoutePath(meta map[string]any) string {
	// Accept both the new "corpusd_route_path" key and the legacy
	// "platformd_route_path" key for backward compatibility with existing
	// published revisions in Dolt (renamed in PR 6 of store-consolidation).
	if route := metadataString(meta, "corpusd_route_path"); route != "" {
		return route
	}
	if route := metadataString(meta, "platformd_route_path"); route != "" {
		return route
	}
	if ref, ok := meta["corpusd_publication_ref"].(map[string]any); ok {
		return metadataString(ref, "route_path")
	}
	if ref, ok := meta["platformd_publication_ref"].(map[string]any); ok {
		return metadataString(ref, "route_path")
	}
	return ""
}
