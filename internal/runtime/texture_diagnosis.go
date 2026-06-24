package runtime

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/markdownstructure"
	"github.com/yusefmosiah/go-choir/internal/texturedoc"
	"github.com/yusefmosiah/go-choir/internal/types"
)

type textureDiagnosisResponse struct {
	OwnerID            string                            `json:"owner_id"`
	DocID              string                            `json:"doc_id,omitempty"`
	StorePath          string                            `json:"store_path"`
	TexturePath        string                            `json:"texture_path"`
	Document           *textureDocumentResponse          `json:"document,omitempty"`
	Revisions          []textureRevisionResponse         `json:"revisions"`
	RevisionStructures []textureRevisionStructureSummary `json:"revision_structures,omitempty"`
	Runs               []types.RunRecord                 `json:"runs"`
	Events             []types.EventRecord               `json:"events"`
	Messages           []types.ChannelMessage            `json:"messages"`
	Decisions          []types.TextureDecisionRecord     `json:"decisions,omitempty"`
	Evidence           []types.EvidenceRecord            `json:"evidence"`
	ErrorMatches       []string                          `json:"error_matches,omitempty"`
}

type textureRevisionStructureSummary struct {
	RevisionID        string                         `json:"revision_id"`
	DocID             string                         `json:"doc_id"`
	VersionNumber     int                            `json:"version_number"`
	ParentRevisionID  string                         `json:"parent_revision_id,omitempty"`
	AuthorKind        types.AuthorKind               `json:"author_kind"`
	AuthorLabel       string                         `json:"author_label"`
	CreatedAt         string                         `json:"created_at"`
	ContentHash       string                         `json:"content_hash"`
	LineCount         int                            `json:"line_count"`
	NonEmptyLineCount int                            `json:"non_empty_line_count"`
	HeadingCount      int                            `json:"heading_count"`
	SourceMarkerCount int                            `json:"source_marker_count"`
	TableCount        int                            `json:"table_count"`
	TableRowCount     int                            `json:"table_row_count"`
	Tables            []textureTableStructureSummary `json:"tables,omitempty"`
}

type textureTableStructureSummary struct {
	Index        int    `json:"index"`
	StartLine    int    `json:"start_line"`
	EndLine      int    `json:"end_line"`
	ColumnCount  int    `json:"column_count"`
	RowCount     int    `json:"row_count"`
	HasSeparator bool   `json:"has_separator"`
	Signature    string `json:"signature"`
}

// textureBlameResponse is the JSON response for
// GET /api/texture/revisions/{id}/blame.
type textureBlameResponse struct {
	types.BlameResult
}

func revisionStructureSummaryFromRecord(rev types.Revision) textureRevisionStructureSummary {
	lines := strings.Split(strings.ReplaceAll(rev.Content, "\r\n", "\n"), "\n")
	summary := textureRevisionStructureSummary{
		RevisionID:        rev.RevisionID,
		DocID:             rev.DocID,
		VersionNumber:     rev.VersionNumber,
		ParentRevisionID:  rev.ParentRevisionID,
		AuthorKind:        rev.AuthorKind,
		AuthorLabel:       rev.AuthorLabel,
		CreatedAt:         rev.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		ContentHash:       "sha256:" + contentHash(rev.Content),
		LineCount:         len(lines),
		SourceMarkerCount: textureRevisionSourceMarkerCount(rev),
	}
	if rev.Content == "" {
		summary.LineCount = 0
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		summary.NonEmptyLineCount++
		if strings.HasPrefix(trimmed, "#") {
			summary.HeadingCount++
		}
	}
	summary.Tables = textureTableStructureSummaries(lines)
	for _, table := range summary.Tables {
		summary.TableRowCount += table.RowCount
	}
	summary.TableCount = len(summary.Tables)
	return summary
}

var projectedTextureSourceRefRE = regexp.MustCompile(`\[[0-9]{1,3}\]`)

func textureRevisionSourceMarkerCount(rev types.Revision) int {
	if len(strings.TrimSpace(string(rev.BodyDoc))) > 0 {
		var doc texturedoc.StructuredTextureDoc
		if err := json.Unmarshal(rev.BodyDoc, &doc); err == nil {
			return countStructuredTextureSourceNodes(doc.Doc)
		}
	}
	return len(projectedTextureSourceRefRE.FindAllString(rev.Content, -1))
}

func countStructuredTextureSourceNodes(node texturedoc.Node) int {
	count := 0
	if node.Type == "source_ref" {
		count++
	}
	for _, child := range node.Content {
		count += countStructuredTextureSourceNodes(child)
	}
	return count
}

func textureTableStructureSummaries(lines []string) []textureTableStructureSummary {
	var tables []textureTableStructureSummary
	var current *textureTableStructureSummary
	var signatureCells []string

	flush := func(endLine int) {
		if current == nil {
			return
		}
		current.EndLine = endLine
		current.Signature = "sha256:" + contentHash(strings.Join(signatureCells, "\n"))
		tables = append(tables, *current)
		current = nil
		signatureCells = nil
	}

	for i, line := range lines {
		lineNumber := i + 1
		cells := markdownstructure.TableRowCells(line)
		if cells == nil {
			flush(lineNumber - 1)
			continue
		}
		if current == nil {
			current = &textureTableStructureSummary{
				Index:       len(tables),
				StartLine:   lineNumber,
				ColumnCount: len(cells),
			}
		}
		current.RowCount++
		if markdownstructure.IsTableSeparatorCells(cells) {
			current.HasSeparator = true
		}
		signatureCells = append(signatureCells, strings.Join(cells, "\x1f"))
	}
	flush(len(lines))
	return tables
}

func diagnosisIncludeContent(r *http.Request) bool {
	raw := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("include_content")))
	switch raw {
	case "0", "false", "no":
		return false
	default:
		return true
	}
}

func diagnosisOwnerRunScanLimit(limit int) int {
	scanLimit := limit * 20
	if scanLimit < 500 {
		scanLimit = 500
	}
	if scanLimit > 2000 {
		scanLimit = 2000
	}
	return scanLimit
}

func runRecordBelongsToTextureDoc(run types.RunRecord, docID string) bool {
	docID = strings.TrimSpace(docID)
	if docID == "" {
		return false
	}
	if strings.TrimSpace(run.ChannelID) == docID {
		return true
	}
	if metadataStringValue(run.Metadata, "doc_id") == docID {
		return true
	}
	if metadataStringValue(run.Metadata, runMetadataChannelID) == docID {
		return true
	}
	return false
}

func appendUniqueRunRecords(existing []types.RunRecord, more ...types.RunRecord) []types.RunRecord {
	seen := make(map[string]bool, len(existing)+len(more))
	for _, run := range existing {
		if strings.TrimSpace(run.RunID) != "" {
			seen[run.RunID] = true
		}
	}
	for _, run := range more {
		if strings.TrimSpace(run.RunID) == "" || seen[run.RunID] {
			continue
		}
		seen[run.RunID] = true
		existing = append(existing, run)
	}
	return existing
}
