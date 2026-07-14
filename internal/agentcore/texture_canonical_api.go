package agentcore

import contentowner "github.com/yusefmosiah/go-choir/internal/content"

// TextureContentService exposes the concrete content service used when Texture
// imports or preserves canonical source material.
func (rt *Runtime) TextureContentService() *contentowner.Service {
	if rt == nil {
		return nil
	}
	return rt.content
}

// TextureTestAPIsEnabled reports whether explicitly local-only Texture test
// seams are enabled in the runtime configuration.
func (rt *Runtime) TextureTestAPIsEnabled() bool {
	return rt != nil && rt.cfg.EnableTestAPIs
}
