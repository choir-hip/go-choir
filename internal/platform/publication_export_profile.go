package platform

type publicationExportProfile struct {
	ID                string                          `json:"id"`
	Name              string                          `json:"name"`
	Typography        publicationExportTypography     `json:"typography"`
	Headings          publicationExportHeadings       `json:"headings"`
	Table             publicationExportTable          `json:"table"`
	CitationPlacement string                          `json:"citation_placement"`
	SourceDetailLevel string                          `json:"source_detail_level"`
	Page              publicationExportPage           `json:"page"`
	HeaderFooter      publicationExportHeaderFooter   `json:"header_footer"`
	MetadataPolicy    publicationExportMetadataPolicy `json:"metadata_policy"`
}

type publicationExportTypography struct {
	FontStack     string `json:"font_stack"`
	BodySizePX    int    `json:"body_size_px"`
	LineHeightPct int    `json:"line_height_pct"`
	MaxWidthPX    int    `json:"max_width_px"`
}

type publicationExportHeadings struct {
	TitleSizePX int `json:"title_size_px"`
	H1SizePX    int `json:"h1_size_px"`
	H2SizePX    int `json:"h2_size_px"`
	H3SizePX    int `json:"h3_size_px"`
}

type publicationExportTable struct {
	BorderColor string `json:"border_color"`
	HeaderFill  string `json:"header_fill"`
}

type publicationExportPage struct {
	MarginTopPt    int `json:"margin_top_pt"`
	MarginRightPt  int `json:"margin_right_pt"`
	MarginBottomPt int `json:"margin_bottom_pt"`
	MarginLeftPt   int `json:"margin_left_pt"`
}

type publicationExportHeaderFooter struct {
	HeaderText string `json:"header_text"`
	FooterText string `json:"footer_text"`
}

type publicationExportMetadataPolicy struct {
	EmbedSourceManifest bool   `json:"embed_source_manifest"`
	EmbedExportMetadata bool   `json:"embed_export_metadata"`
	VisibleSources      bool   `json:"visible_sources"`
	PrivateScope        string `json:"private_scope"`
}

func defaultPublicationExportProfile() publicationExportProfile {
	return publicationExportProfile{
		ID:   "default-professional",
		Name: "Default Professional",
		Typography: publicationExportTypography{
			FontStack:     `-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,sans-serif`,
			BodySizePX:    16,
			LineHeightPct: 162,
			MaxWidthPX:    820,
		},
		Headings: publicationExportHeadings{
			TitleSizePX: 30,
			H1SizePX:    30,
			H2SizePX:    22,
			H3SizePX:    18,
		},
		Table: publicationExportTable{
			BorderColor: "#d6dbe3",
			HeaderFill:  "#f1f4f8",
		},
		CitationPlacement: "inline_marker_appendix",
		SourceDetailLevel: "labels_snapshots_manifest",
		Page: publicationExportPage{
			MarginTopPt:    72,
			MarginRightPt:  72,
			MarginBottomPt: 72,
			MarginLeftPt:   72,
		},
		HeaderFooter: publicationExportHeaderFooter{
			HeaderText: "",
			FooterText: "Choir publication export",
		},
		MetadataPolicy: publicationExportMetadataPolicy{
			EmbedSourceManifest: true,
			EmbedExportMetadata: true,
			VisibleSources:      true,
			PrivateScope:        "public_publication_version_only",
		},
	}
}
