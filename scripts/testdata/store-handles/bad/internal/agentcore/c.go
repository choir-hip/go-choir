package agentcore

func read(g G) { _, _ = g.ListObjectsByMetadata(nil, "k", "$.a", "v", 1) }
