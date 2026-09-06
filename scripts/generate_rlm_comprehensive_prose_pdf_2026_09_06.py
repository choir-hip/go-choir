"""Generate the PDF rendition of the comprehensive prose report for the RLM cutover.

Run: uv run --with reportlab python3 scripts/generate_rlm_comprehensive_prose_pdf_2026_09_06.py
"""
import importlib.util
import os
import shutil
import sys

spec = importlib.util.spec_from_file_location(
    "outage_pdf", os.path.join(os.path.dirname(os.path.abspath(__file__)), "generate_outage_reports_pdf_2026_08_28.py")
)
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)

SRC = "docs/reports/choir-rlm-orchestration-as-code-comprehensive-report-2026-09-06.md"
HEADER = "CHOIR - THE TRANSITION TO ORCHESTRATION AS CODE: COMPREHENSIVE REPORT"
DATE = "2026-09-06"


def main():
    out_dir = sys.argv[1] if len(sys.argv) > 1 else "tmp/pdfs"
    os.makedirs(out_dir, exist_ok=True)
    styles = mod.build_styles()
    from reportlab.lib.pagesizes import letter
    from reportlab.platypus import BaseDocTemplate, Frame, PageTemplate

    pdf_filename = os.path.basename(SRC).replace(".md", ".pdf")
    local_pdf = os.path.join(out_dir, pdf_filename)

    doc = BaseDocTemplate(
        local_pdf,
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
    print("wrote local PDF:", doc.filename)

    icloud_dir = os.path.expanduser("~/Library/Mobile Documents/com~apple~CloudDocs/Choir Reports")
    if os.path.isdir(icloud_dir):
        icloud_pdf = os.path.join(icloud_dir, pdf_filename)
        shutil.copyfile(local_pdf, icloud_pdf)
        print("copied to iCloud Drive:", icloud_pdf)
    else:
        print("warning: iCloud Reports directory not found:", icloud_dir)


if __name__ == "__main__":
    main()
