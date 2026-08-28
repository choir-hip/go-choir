"""Generate PDF renditions of the 2026-08-28 outage post-mortem and
whole-system status retrospective from their markdown sources.

Run: uv run --with reportlab python scripts/generate_outage_reports_pdf_2026_08_28.py <out-dir>
"""
import os
import re
import sys

from reportlab.lib import colors
from reportlab.lib.pagesizes import letter
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.lib.units import inch
from reportlab.platypus import (
    BaseDocTemplate, Frame, PageTemplate, Paragraph, Spacer, Table, TableStyle, HRFlowable
)
from reportlab.pdfgen import canvas as pdfcanvas

REPORTS = [
    (
        "docs/reports/choir-held-computer-boot-outage-postmortem-2026-08-28.md",
        "CHOIR PLATFORM - HELD-COMPUTER BOOT OUTAGE POST-MORTEM",
        "2026-08-28",
    ),
    (
        "docs/reports/choir-whole-system-status-retrospective-2026-08-28.md",
        "CHOIR PLATFORM - WHOLE-SYSTEM STATUS RETROSPECTIVE",
        "2026-08-28",
    ),
]

PAGE_W, PAGE_H = letter
MARGIN = 0.75 * inch


class NumberedCanvas(pdfcanvas.Canvas):
    def __init__(self, *args, header_text="", date_text="", **kwargs):
        super().__init__(*args, **kwargs)
        self._header = header_text
        self._date = date_text
        self._saved = []

    def showPage(self):
        self._saved.append(dict(self.__dict__))
        self._startPage()

    def save(self):
        total = len(self._saved)
        for state in self._saved:
            self.__dict__.update(state)
            self._decorate(total)
            super().showPage()
        super().save()

    def _decorate(self, total):
        self.saveState()
        self.setFont("Helvetica", 7.5)
        self.setFillColor(colors.HexColor("#64748B"))
        if self._pageNumber > 1:
            self.drawString(MARGIN, PAGE_H - 0.5 * inch, self._header)
            self.drawRightString(PAGE_W - MARGIN, PAGE_H - 0.5 * inch, self._date)
            self.setStrokeColor(colors.HexColor("#CBD5E1"))
            self.setLineWidth(0.5)
            self.line(MARGIN, PAGE_H - 0.55 * inch, PAGE_W - MARGIN, PAGE_H - 0.55 * inch)
        self.setStrokeColor(colors.HexColor("#CBD5E1"))
        self.line(MARGIN, 0.55 * inch, PAGE_W - MARGIN, 0.55 * inch)
        self.drawString(MARGIN, 0.4 * inch, "CONFIDENTIAL - FOR INTERNAL PLATFORM ENGINEERING & ARCHITECTURE REVIEW")
        self.drawRightString(PAGE_W - MARGIN, 0.4 * inch, f"Page {self._pageNumber} of {total}")
        self.restoreState()


def build_styles():
    ss = getSampleStyleSheet()
    return {
        "title": ParagraphStyle("RTitle", parent=ss["Title"], fontSize=17, leading=22, spaceAfter=4),
        "meta": ParagraphStyle(
            "RMeta", parent=ss["Normal"], fontSize=8.5, textColor=colors.HexColor("#475569"), leading=12
        ),
        "h2": ParagraphStyle(
            "RH2", parent=ss["Heading1"], fontSize=13, leading=16, spaceBefore=14, spaceAfter=6,
            textColor=colors.HexColor("#0F172A"),
        ),
        "h3": ParagraphStyle(
            "RH3", parent=ss["Heading2"], fontSize=11, leading=14, spaceBefore=10, spaceAfter=4,
            textColor=colors.HexColor("#1E293B"),
        ),
        "body": ParagraphStyle("RBody", parent=ss["Normal"], fontSize=9, leading=12.5, spaceAfter=5),
        "bullet": ParagraphStyle(
            "RBullet", parent=ss["Normal"], fontSize=9, leading=12.5, leftIndent=14, bulletIndent=4, spaceAfter=3
        ),
        "cell": ParagraphStyle("RCell", parent=ss["Normal"], fontSize=7.8, leading=10),
        "cellhead": ParagraphStyle(
            "RCellHead", parent=ss["Normal"], fontSize=7.8, leading=10, fontName="Helvetica-Bold",
            textColor=colors.white,
        ),
        "code": ParagraphStyle(
            "RCode", parent=ss["Normal"], fontName="Courier", fontSize=7.8, leading=10,
            backColor=colors.HexColor("#F1F5F9"), leftIndent=8, spaceBefore=2, spaceAfter=6,
        ),
    }


def esc(t):
    t = t.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")
    t = re.sub(r"\*\*(.+?)\*\*", r"<b>\1</b>", t)
    t = re.sub(r"`([^`]+)`", r"<font face='Courier' size='7.6'>\1</font>", t)
    t = re.sub(r"\*(.+?)\*", r"<i>\1</i>", t)
    return t


def md_to_flowables(path, styles):
    flow = []
    lines = Path_lines = open(path, encoding="utf-8").read().splitlines()
    i, n = 0, len(lines)
    in_code = False
    code_buf = []
    while i < n:
        line = lines[i]
        if line.strip().startswith("```"):
            if in_code:
                flow.append(Paragraph("<br/>".join(esc(c) for c in code_buf) or "&nbsp;", styles["code"]))
                code_buf = []
            in_code = not in_code
            i += 1
            continue
        if in_code:
            code_buf.append(line)
            i += 1
            continue
        if line.startswith("| ") or line.startswith("|-"):
            # collect table block
            rows = []
            while i < n and lines[i].startswith("|"):
                cells = [c.strip() for c in lines[i].strip().strip("|").split("|")]
                if not set("".join(cells)) <= set("-: "):
                    rows.append(cells)
                i += 1
            if rows:
                data = [[Paragraph(esc(c), styles["cellhead"]) for c in rows[0]]]
                for r in rows[1:]:
                    data.append([Paragraph(esc(c), styles["cell"]) for c in r])
                ncols = len(rows[0])
                avail = PAGE_W - 2 * MARGIN
                if ncols >= 5:
                    widths = [avail * w for w in (0.09, 0.12, 0.07, 0.13, 0.59)]
                elif ncols == 3:
                    widths = [avail * w for w in (0.18, 0.14, 0.68)]
                elif ncols == 4:
                    widths = [avail * w for w in (0.30, 0.14, 0.14, 0.42)]
                else:
                    widths = [avail / ncols] * ncols
                t = Table(data, colWidths=widths, repeatRows=1)
                t.setStyle(TableStyle([
                    ("BACKGROUND", (0, 0), (-1, 0), colors.HexColor("#0F172A")),
                    ("GRID", (0, 0), (-1, -1), 0.4, colors.HexColor("#CBD5E1")),
                    ("VALIGN", (0, 0), (-1, -1), "TOP"),
                    ("TOPPADDING", (0, 0), (-1, -1), 2.5),
                    ("BOTTOMPADDING", (0, 0), (-1, -1), 2.5),
                    ("LEFTPADDING", (0, 0), (-1, -1), 4),
                    ("RIGHTPADDING", (0, 0), (-1, -1), 4),
                    ("ROWBACKGROUNDS", (0, 1), (-1, -1), [colors.white, colors.HexColor("#F8FAFC")]),
                ]))
                flow.append(Spacer(1, 3))
                flow.append(t)
                flow.append(Spacer(1, 6))
            continue
        s = line.strip()
        if not s:
            i += 1
            continue
        if s.startswith("## "):
            flow.append(Paragraph(esc(s[3:]), styles["h2"]))
        elif s.startswith("### "):
            flow.append(Paragraph(esc(s[4:]), styles["h3"]))
        elif s.startswith("# "):
            flow.append(Paragraph(esc(s[2:]), styles["title"]))
        elif s.startswith("- "):
            flow.append(Paragraph(esc(s[2:]), styles["bullet"], bulletText="\u2022"))
        elif s.startswith("<") and s.endswith(">") and ": " in s:
            flow.append(Paragraph(esc(s), styles["meta"]))
        else:
            flow.append(Paragraph(esc(s), styles["body"]))
        i += 1
    return flow


def main():
    out_dir = sys.argv[1] if len(sys.argv) > 1 else "tmp/pdfs"
    os.makedirs(out_dir, exist_ok=True)
    styles = build_styles()
    for src, header, date in REPORTS:
        doc = BaseDocTemplate(
            os.path.join(out_dir, os.path.basename(src).replace(".md", ".pdf")),
            pagesize=letter, leftMargin=MARGIN, rightMargin=MARGIN,
            topMargin=MARGIN, bottomMargin=MARGIN,
            title=header, author="Choir Platform Engineering",
        )
        frame = Frame(MARGIN, MARGIN, PAGE_W - 2 * MARGIN, PAGE_H - 2 * MARGIN, id="main")
        doc.addPageTemplates([PageTemplate(id="page", frames=[frame])])
        doc.build(md_to_flowables(src, styles), canvasmaker=lambda *a, **k: NumberedCanvas(*a, header_text=header, date_text=date, **k))
        print("wrote", doc.filename)


if __name__ == "__main__":
    main()
