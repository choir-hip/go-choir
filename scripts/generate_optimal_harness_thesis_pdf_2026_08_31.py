"""Generate the PDF rendition of the Optimal Harness Thesis.

Run: uv run --with reportlab python scripts/generate_optimal_harness_thesis_pdf_2026_08_31.py <out-dir>
"""
import importlib.util
import os
import sys

spec = importlib.util.spec_from_file_location(
    "outage_pdf", os.path.join(os.path.dirname(os.path.abspath(__file__)), "generate_outage_reports_pdf_2026_08_28.py")
)
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)

SRC = sys.argv[2] if len(sys.argv) > 2 else "docs/reviews/optimal-harness-thesis-2026-08-31.md"
HEADER = "CHOIR - THE OPTIMAL HARNESS THESIS, v2"
DATE = "2026-08-31"


def main():
    out_dir = sys.argv[1] if len(sys.argv) > 1 else "tmp/pdfs"
    os.makedirs(out_dir, exist_ok=True)
    styles = mod.build_styles()
    from reportlab.lib.pagesizes import letter
    from reportlab.platypus import BaseDocTemplate, Frame, PageTemplate

    doc = BaseDocTemplate(
        os.path.join(out_dir, os.path.basename(SRC).replace(".md", ".pdf")),
        pagesize=letter, leftMargin=mod.MARGIN, rightMargin=mod.MARGIN,
        topMargin=mod.MARGIN, bottomMargin=mod.MARGIN,
        title=HEADER, author="Choir Platform Engineering",
    )
    frame = Frame(mod.MARGIN, mod.MARGIN, mod.PAGE_W - 2 * mod.MARGIN, mod.PAGE_H - 2 * mod.MARGIN, id="main")
    doc.addPageTemplates([PageTemplate(id="page", frames=[frame])])
    doc.build(
        mod.md_to_flowables(SRC, styles),
        canvasmaker=lambda *a, **k: mod.NumberedCanvas(*a, header_text=HEADER, date_text=DATE, **k),
    )
    print("wrote", doc.filename)


if __name__ == "__main__":
    main()
