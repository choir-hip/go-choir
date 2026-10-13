package objectgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	WebCaptureObjectKind    ObjectKind = "choir.web_capture"
	WebCaptureSchemaVersion            = "choir.web_capture.v1"

	UniversalWireStoryClusterObjectKind    ObjectKind = "choir.universal_wire_story_cluster"
	UniversalWireStoryClusterSchemaVersion            = "choir.universal_wire_story_cluster.v1"
)

type WebCaptureMetadata struct {
	SchemaVersion       string `json:"schema_version"`
	URL                 string `json:"url"`
	CanonicalURL        string `json:"canonical_url"`
	Title               string `json:"title,omitempty"`
	FetchedAt           string `json:"fetched_at"`
	ContentBlobID       string `json:"content_blob_id"`
	ExtractedTextBlobID string `json:"extracted_text_blob_id"`
	EmbeddingModel      string `json:"embedding_model,omitempty"`
	EmbeddingVersion    string `json:"embedding_version,omitempty"`
	// Feed fields (Autopaper W1): a polled item extends the capture's
	// metadata rather than creating a new kind.
	SourceID    string `json:"source_id,omitempty"`
	SourceType  string `json:"source_type,omitempty"`
	ItemID      string `json:"item_id,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	Language    string `json:"language,omitempty"`
	Region      string `json:"region,omitempty"`
	BodyKind    string `json:"body_kind,omitempty"`
	// Block document fields (Cut 2 unit 2a). A block capture's
	// ContentBlobID names the raw bytes as fetched, ExtractedTextBlobID the
	// block projection, and BlocksBlobID the block JSON; DocHash is its
	// identity and Extractor the rules that made its blocks. Captures written
	// before Cut 2 have none of these and one flat-text blob for both.
	BlocksBlobID string `json:"blocks_blob_id,omitempty"`
	Extractor    string `json:"extractor,omitempty"`
	DocHash      string `json:"doc_hash,omitempty"`
	BlockCount   int    `json:"block_count,omitempty"`
	MediaType    string `json:"media_type,omitempty"`
	RawBytes     int    `json:"raw_bytes,omitempty"`
}

type CreateWebCaptureRequest struct {
	// IdentityKey, when set, makes the capture's id stable for that key, so
	// writing the same item twice names the same object.
	IdentityKey         string
	SourceID            string
	SourceType          string
	ItemID              string
	PublishedAt         time.Time
	Language            string
	Region              string
	BodyKind            string
	BlocksBlobID        string
	Extractor           string
	DocHash             string
	BlockCount          int
	MediaType           string
	RawBytes            int
	OwnerID             string
	ComputerID          string
	URL                 string
	CanonicalURL        string
	Title               string
	FetchedAt           time.Time
	ContentBlobID       string
	ExtractedTextBlobID string
	EmbeddingModel      string
	EmbeddingVersion    string
	ExtractedText       []byte
	Now                 time.Time
}

func (s *Service) CreateWebCapture(ctx context.Context, req CreateWebCaptureRequest) (Object, error) {
	metadata, err := BuildWebCaptureMetadata(req)
	if err != nil {
		return Object{}, err
	}
	return s.CreateObject(ctx, CreateObjectRequest{
		Kind:        WebCaptureObjectKind,
		OwnerID:     req.OwnerID,
		ComputerID:  req.ComputerID,
		IdentityKey: strings.TrimSpace(req.IdentityKey),
		Body:        req.ExtractedText,
		Metadata:    metadata,
		Now:         req.Now,
	})
}

// CreateWebCaptureIfAbsent creates the capture only if its id is not already
// stored, atomically; an existing capture is returned unchanged with created
// false. Shared captures are written this way so two computers polling the
// same item cannot replace each other's capture.
func (s *Service) CreateWebCaptureIfAbsent(ctx context.Context, req CreateWebCaptureRequest) (Object, bool, error) {
	metadata, err := BuildWebCaptureMetadata(req)
	if err != nil {
		return Object{}, false, err
	}
	return s.CreateObjectIfAbsent(ctx, CreateObjectRequest{
		Kind:        WebCaptureObjectKind,
		OwnerID:     req.OwnerID,
		ComputerID:  req.ComputerID,
		IdentityKey: strings.TrimSpace(req.IdentityKey),
		Body:        req.ExtractedText,
		Metadata:    metadata,
		Now:         req.Now,
	})
}

func BuildWebCaptureMetadata(req CreateWebCaptureRequest) (WebCaptureMetadata, error) {
	originalURL, err := normalizeWebCaptureURL(req.URL, "url")
	if err != nil {
		return WebCaptureMetadata{}, err
	}
	canonicalURL := strings.TrimSpace(req.CanonicalURL)
	if canonicalURL == "" {
		canonicalURL = originalURL
	} else if canonicalURL, err = normalizeWebCaptureURL(canonicalURL, "canonical_url"); err != nil {
		return WebCaptureMetadata{}, err
	}
	if req.FetchedAt.IsZero() {
		return WebCaptureMetadata{}, fmt.Errorf("fetched_at is required")
	}
	metadata := WebCaptureMetadata{
		SchemaVersion:       WebCaptureSchemaVersion,
		URL:                 originalURL,
		CanonicalURL:        canonicalURL,
		Title:               strings.TrimSpace(req.Title),
		FetchedAt:           req.FetchedAt.UTC().Format(time.RFC3339Nano),
		ContentBlobID:       strings.TrimSpace(req.ContentBlobID),
		ExtractedTextBlobID: strings.TrimSpace(req.ExtractedTextBlobID),
		EmbeddingModel:      strings.TrimSpace(req.EmbeddingModel),
		EmbeddingVersion:    strings.TrimSpace(req.EmbeddingVersion),
		SourceID:            strings.TrimSpace(req.SourceID),
		SourceType:          strings.TrimSpace(req.SourceType),
		ItemID:              strings.TrimSpace(req.ItemID),
		Language:            strings.TrimSpace(req.Language),
		Region:              strings.TrimSpace(req.Region),
		BodyKind:            strings.TrimSpace(req.BodyKind),
		BlocksBlobID:        strings.TrimSpace(req.BlocksBlobID),
		Extractor:           strings.TrimSpace(req.Extractor),
		DocHash:             strings.TrimSpace(req.DocHash),
		BlockCount:          req.BlockCount,
		MediaType:           strings.TrimSpace(req.MediaType),
		RawBytes:            req.RawBytes,
	}
	if !req.PublishedAt.IsZero() {
		metadata.PublishedAt = req.PublishedAt.UTC().Format(time.RFC3339Nano)
	}
	if err := metadata.Validate(); err != nil {
		return WebCaptureMetadata{}, err
	}
	return metadata, nil
}

func WebCaptureMetadataFromObject(obj Object) (WebCaptureMetadata, error) {
	if obj.ObjectKind != WebCaptureObjectKind {
		return WebCaptureMetadata{}, fmt.Errorf("object kind %s is not %s", obj.ObjectKind, WebCaptureObjectKind)
	}
	var metadata WebCaptureMetadata
	if err := json.Unmarshal(obj.Metadata, &metadata); err != nil {
		return WebCaptureMetadata{}, fmt.Errorf("decode web capture metadata: %w", err)
	}
	if err := metadata.Validate(); err != nil {
		return WebCaptureMetadata{}, err
	}
	return metadata, nil
}

func (m WebCaptureMetadata) Validate() error {
	if strings.TrimSpace(m.SchemaVersion) != WebCaptureSchemaVersion {
		return fmt.Errorf("schema_version %q is not %s", m.SchemaVersion, WebCaptureSchemaVersion)
	}
	if _, err := normalizeWebCaptureURL(m.URL, "url"); err != nil {
		return err
	}
	if _, err := normalizeWebCaptureURL(m.CanonicalURL, "canonical_url"); err != nil {
		return err
	}
	if strings.TrimSpace(m.FetchedAt) == "" {
		return fmt.Errorf("fetched_at is required")
	}
	if _, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(m.FetchedAt)); err != nil {
		return fmt.Errorf("fetched_at must be RFC3339 timestamp: %w", err)
	}
	if strings.TrimSpace(m.ContentBlobID) == "" {
		return fmt.Errorf("content_blob_id is required")
	}
	if strings.TrimSpace(m.ExtractedTextBlobID) == "" {
		return fmt.Errorf("extracted_text_blob_id is required")
	}
	if strings.TrimSpace(m.BlocksBlobID) != "" && (strings.TrimSpace(m.Extractor) == "" || strings.TrimSpace(m.DocHash) == "") {
		return fmt.Errorf("a block capture needs its extractor and doc_hash")
	}
	if strings.TrimSpace(m.EmbeddingVersion) != "" && strings.TrimSpace(m.EmbeddingModel) == "" {
		return fmt.Errorf("embedding_model is required when embedding_version is set")
	}
	return nil
}

func normalizeWebCaptureURL(raw, field string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("%s must be an absolute URL", field)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("%s must use http or https", field)
	}
	parsed.Fragment = ""
	return parsed.String(), nil
}
