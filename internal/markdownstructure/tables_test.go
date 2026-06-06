package markdownstructure

import (
	"strings"
	"testing"
)

func TestNormalizeTableShapedRowsRepairsFinalRowMissingDelimiter(t *testing.T) {
	content := strings.Join([]string{
		"# Proposal",
		"",
		"| Term | Definition |",
		"| --- | --- |",
		"| Vector search | Similarity lookup. |",
		"| Work product | Durable output.",
		"",
		"---",
	}, "\n")

	got, changed := NormalizeTableShapedRows(content)
	if !changed {
		t.Fatalf("NormalizeTableShapedRows changed = false")
	}
	if !strings.Contains(got, "| Work product | Durable output. |") {
		t.Fatalf("normalized content missing repaired row:\n%s", got)
	}
}

func TestNormalizeTableShapedRowsRepairsBlankSeparatedFinalRow(t *testing.T) {
	content := strings.Join([]string{
		"# Proposal",
		"",
		"| Term | Definition |",
		"| --- | --- |",
		"| Vector search | Similarity lookup. |",
		"",
		"| Work product | Durable output.",
		"",
		"---",
	}, "\n")

	got, changed := NormalizeTableShapedRows(content)
	if !changed {
		t.Fatalf("NormalizeTableShapedRows changed = false")
	}
	if !strings.Contains(got, "| Vector search | Similarity lookup. |\n| Work product | Durable output. |") {
		t.Fatalf("normalized content did not rejoin blank-separated row:\n%s", got)
	}
}

func TestNormalizeTableShapedRowsIgnoresOrdinaryPipedParagraph(t *testing.T) {
	content := "This is not a table | and should not gain a final delimiter."
	got, changed := NormalizeTableShapedRows(content)
	if changed {
		t.Fatalf("NormalizeTableShapedRows unexpectedly changed content:\n%s", got)
	}
	if got != content {
		t.Fatalf("content changed:\ngot  %q\nwant %q", got, content)
	}
}

func TestNormalizeTableShapedRowsIgnoresDifferentWidthRowAfterTable(t *testing.T) {
	content := strings.Join([]string{
		"| Term | Definition |",
		"| --- | --- |",
		"| Vector search | Similarity lookup. |",
		"",
		"| Not a continuation | has | too many cells",
	}, "\n")
	got, changed := NormalizeTableShapedRows(content)
	if changed {
		t.Fatalf("NormalizeTableShapedRows unexpectedly changed content:\n%s", got)
	}
	if got != content {
		t.Fatalf("content changed:\ngot  %q\nwant %q", got, content)
	}
}

func TestNormalizeTableShapedRowsRemovesDuplicateSeparatorRows(t *testing.T) {
	content := strings.Join([]string{
		"# Proposal",
		"",
		"| Term | Definition |",
		"| --- | --- |",
		"| --- | --- |",
		"| Vector search | Similarity lookup. |",
		"| Work product | Durable output. |",
		"",
		"Closing paragraph.",
	}, "\n")

	got, changed := NormalizeTableShapedRows(content)
	if !changed {
		t.Fatalf("NormalizeTableShapedRows changed = false")
	}
	if strings.Count(got, "| --- | --- |") != 1 {
		t.Fatalf("normalized content kept duplicate separator:\n%s", got)
	}
	if !strings.Contains(got, "| Vector search | Similarity lookup. |\n| Work product | Durable output. |") {
		t.Fatalf("normalized content dropped table body rows:\n%s", got)
	}
}

func TestTableRowCellsHandlesEscapedPipes(t *testing.T) {
	cells := TableRowCells(`| Term \| Alias | Definition with \| symbol |`)
	if len(cells) != 2 {
		t.Fatalf("cells = %#v, want 2 cells", cells)
	}
	if cells[0] != "Term | Alias" || cells[1] != "Definition with | symbol" {
		t.Fatalf("cells = %#v", cells)
	}
}
