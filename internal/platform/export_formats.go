package platform

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/yusefmosiah/go-choir/internal/markdownstructure"
)

type publicationExportBytes struct {
	content  []byte
	metadata json.RawMessage
}

func buildPublicationExportBytes(bundle *PublicationBundle, format string) (publicationExportBytes, error) {
	if bundle == nil {
		return publicationExportBytes{}, fmt.Errorf("publication bundle is required")
	}
	profile := defaultPublicationExportProfile()
	metadata := publicationExportMetadata(bundle, format, profile)
	doc := buildPublicationDocument(bundle)
	switch format {
	case "docx":
		content, err := buildPublicationDOCX(bundle, doc, metadata, profile)
		if err != nil {
			return publicationExportBytes{}, err
		}
		return publicationExportBytes{content: content, metadata: metadata}, nil
	case "pdf":
		content, err := buildPublicationPDF(bundle, doc, profile)
		if err != nil {
			return publicationExportBytes{}, err
		}
		return publicationExportBytes{content: content, metadata: metadata}, nil
	case "html":
		return publicationExportBytes{content: []byte(renderPublicationHTML(doc, profile)), metadata: metadata}, nil
	default:
		return publicationExportBytes{content: []byte(formatPublicationExportContent(bundle, format)), metadata: metadata}, nil
	}
}

func formatPublicationExportContent(bundle *PublicationBundle, format string) string {
	if bundle == nil {
		return ""
	}
	content := bundle.Artifact.Content
	switch format {
	case "md":
		normalized, _ := markdownstructure.NormalizeTableShapedRows(content)
		return normalized
	default:
		return content
	}
}

func publicationExportMetadata(bundle *PublicationBundle, format string, profile publicationExportProfile) json.RawMessage {
	if bundle == nil {
		return json.RawMessage("{}")
	}
	sourceManifest := buildPublicationSourceManifest(bundle)
	raw, err := json.Marshal(map[string]any{
		"schema":                   "choir.publication_export.v0",
		"format":                   format,
		"publication_id":           bundle.Publication.ID,
		"publication_version_id":   bundle.Version.ID,
		"route_path":               bundle.Route.Path,
		"content_hash":             bundle.Version.ContentHash,
		"source_revision_hash":     bundle.Version.SourceRevisionHash,
		"projection_hash":          bundle.Version.ProjectionHash,
		"artifact_manifest_id":     bundle.Artifact.ManifestID,
		"generated_at":             time.Now().UTC().Format(time.RFC3339Nano),
		"export_profile":           profile,
		"provenance_scope":         profile.MetadataPolicy.PrivateScope,
		"private_material_omitted": true,
		"access_policy":            json.RawMessage(firstNonEmpty(string(bundle.Policy.Access), "{}")),
		"export_policy":            json.RawMessage(firstNonEmpty(string(bundle.Policy.Export), "{}")),
		"retrieval":                bundle.Retrieval,
		"source_entities":          bundle.SourceEntities,
		"transclusions":            bundle.Transclusions,
		"source_manifest":          sourceManifest,
	})
	if err != nil {
		return json.RawMessage("{}")
	}
	return raw
}
