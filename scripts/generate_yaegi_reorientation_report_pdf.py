#!/usr/bin/env python3
"""
Generate Choir Yaegi / Private Go Actor Kernel Reorientation Report PDF
"""

import os
import sys
import shutil
import re
from reportlab.lib.pagesizes import letter
from reportlab.lib import colors
from reportlab.platypus import (
    SimpleDocTemplate, Paragraph, Spacer, Table, TableStyle, PageBreak, KeepTogether, HRFlowable, Preformatted
)
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.pdfgen import canvas

class NumberedCanvas(canvas.Canvas):
    def __init__(self, *args, **kwargs):
        super(NumberedCanvas, self).__init__(*args, **kwargs)
        self._saved_page_states = []

    def showPage(self):
        self._saved_page_states.append(dict(self.__dict__))
        self._startPage()

    def save(self):
        num_pages = len(self._saved_page_states)
        for state in self._saved_page_states:
            self.__dict__.update(state)
            self.draw_page_decorations(num_pages)
            super(NumberedCanvas, self).showPage()
        super(NumberedCanvas, self).save()

    def draw_page_decorations(self, page_count):
        self.saveState()
        self.setFont("Helvetica", 8)
        self.setFillColor(colors.HexColor("#64748B"))
        
        # Header (pages > 1)
        if self._pageNumber > 1:
            self.drawString(40, 755, "Choir Yaegi / Private Go Actor Kernel — Mission Reorientation Report")
            self.drawRightString(letter[0] - 40, 755, "CONFIDENTIAL / ENGINEERING")
            self.setStrokeColor(colors.HexColor("#CBD5E1"))
            self.setLineWidth(0.5)
            self.line(40, 748, letter[0] - 40, 748)

        # Footer (all pages)
        self.setStrokeColor(colors.HexColor("#CBD5E1"))
        self.setLineWidth(0.5)
        self.line(40, 42, letter[0] - 40, 42)
        
        page_str = f"Page {self._pageNumber} of {page_count}"
        self.drawString(40, 30, "Choir Automatic Computer Architecture & Mission Authority")
        self.drawRightString(letter[0] - 40, 30, page_str)
        self.restoreState()

def build_pdf(md_path, output_pdf_path):
    os.makedirs(os.path.dirname(os.path.abspath(output_pdf_path)), exist_ok=True)
    
    doc = SimpleDocTemplate(
        output_pdf_path,
        pagesize=letter,
        leftMargin=40,
        rightMargin=40,
        topMargin=54,
        bottomMargin=54
    )

    styles = getSampleStyleSheet()
    
    title_style = ParagraphStyle(
        'DocTitle',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=18,
        leading=22,
        textColor=colors.HexColor("#0F172A"),
        spaceAfter=4
    )
    
    meta_style = ParagraphStyle(
        'DocMeta',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=8,
        leading=11.5,
        textColor=colors.HexColor("#475569"),
        spaceAfter=10
    )

    h1_style = ParagraphStyle(
        'Heading1_Custom',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=12,
        leading=15,
        textColor=colors.HexColor("#0F2942"),
        spaceBefore=12,
        spaceAfter=5,
        keepWithNext=True
    )

    h2_style = ParagraphStyle(
        'Heading2_Custom',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=10,
        leading=13,
        textColor=colors.HexColor("#0E7490"),
        spaceBefore=9,
        spaceAfter=4,
        keepWithNext=True
    )

    h3_style = ParagraphStyle(
        'Heading3_Custom',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=9,
        leading=12,
        textColor=colors.HexColor("#334155"),
        spaceBefore=7,
        spaceAfter=3,
        keepWithNext=True
    )

    body_style = ParagraphStyle(
        'Body_Custom',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=8.5,
        leading=11.5,
        textColor=colors.HexColor("#1E293B"),
        spaceAfter=5
    )

    bullet_style = ParagraphStyle(
        'Bullet_Custom',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=8.5,
        leading=11.5,
        textColor=colors.HexColor("#1E293B"),
        leftIndent=14,
        firstLineIndent=-9,
        spaceAfter=3
    )

    code_block_style = ParagraphStyle(
        'CodeBlock',
        parent=styles['Normal'],
        fontName='Courier',
        fontSize=6.8,
        leading=8.5,
        textColor=colors.HexColor("#0F172A")
    )

    table_cell_style = ParagraphStyle(
        'TableCell',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=7.2,
        leading=9.2,
        textColor=colors.HexColor("#1E293B")
    )

    table_header_style = ParagraphStyle(
        'TableHeader',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=7.5,
        leading=9.5,
        textColor=colors.white
    )

    with open(md_path, 'r', encoding='utf-8') as f:
        content = f.read()

    story = []
    lines = content.split('\n')
    i = 0
    
    in_code_block = False
    code_lines = []

    in_table = False
    table_rows = []

    while i < len(lines):
        line = lines[i]
        stripped = line.strip()

        # Handle Code Blocks
        if stripped.startswith('```'):
            if in_code_block:
                in_code_block = False
                code_text = '\n'.join(code_lines)
                code_lines = []
                
                p_code = Preformatted(code_text, code_block_style)
                code_table = Table([[p_code]], colWidths=[letter[0] - 80])
                code_table.setStyle(TableStyle([
                    ('BACKGROUND', (0, 0), (-1, -1), colors.HexColor("#F8FAFC")),
                    ('BOX', (0, 0), (-1, -1), 0.5, colors.HexColor("#CBD5E1")),
                    ('TOPPADDING', (0, 0), (-1, -1), 4),
                    ('BOTTOMPADDING', (0, 0), (-1, -1), 4),
                    ('LEFTPADDING', (0, 0), (-1, -1), 6),
                    ('RIGHTPADDING', (0, 0), (-1, -1), 6),
                ]))
                story.append(Spacer(1, 3))
                story.append(code_table)
                story.append(Spacer(1, 5))
            else:
                in_code_block = True
                code_lines = []
            i += 1
            continue

        if in_code_block:
            code_lines.append(line)
            i += 1
            continue

        # Handle Tables
        if stripped.startswith('|') and stripped.endswith('|'):
            if '---' in stripped and ('|' in stripped):
                i += 1
                continue
            
            cells = [c.strip() for c in stripped[1:-1].split('|')]
            table_rows.append(cells)
            in_table = True
            i += 1
            continue
        elif in_table:
            in_table = False
            if table_rows:
                num_cols = max(len(r) for r in table_rows)
                avail_width = letter[0] - 80
                
                # Proportional widths if 4 columns (table matrix)
                if num_cols == 4:
                    col_widths = [avail_width * 0.22, avail_width * 0.38, avail_width * 0.16, avail_width * 0.24]
                else:
                    col_widths = [avail_width / num_cols] * num_cols
                
                formatted_data = []
                for row_idx, row in enumerate(table_rows):
                    row_cells = []
                    is_head = (row_idx == 0)
                    for col in row:
                        style_to_use = table_header_style if is_head else table_cell_style
                        text = col
                        # Convert markdown bold/code
                        text = re.sub(r'\*\*(.*?)\*\*', r'<b>\1</b>', text)
                        text = re.sub(r'`(.*?)`', r'<font name="Courier">\1</font>', text)
                        row_cells.append(Paragraph(text, style_to_use))
                    while len(row_cells) < num_cols:
                        row_cells.append(Paragraph("", table_cell_style))
                    formatted_data.append(row_cells)
                
                t = Table(formatted_data, colWidths=col_widths)
                t_style = [
                    ('BACKGROUND', (0, 0), (-1, 0), colors.HexColor("#0F2942")),
                    ('ALIGN', (0, 0), (-1, -1), 'LEFT'),
                    ('VALIGN', (0, 0), (-1, -1), 'TOP'),
                    ('TOPPADDING', (0, 0), (-1, -1), 3),
                    ('BOTTOMPADDING', (0, 0), (-1, -1), 3),
                    ('LEFTPADDING', (0, 0), (-1, -1), 4),
                    ('RIGHTPADDING', (0, 0), (-1, -1), 4),
                    ('GRID', (0, 0), (-1, -1), 0.5, colors.HexColor("#CBD5E1")),
                ]
                for r_i in range(1, len(formatted_data)):
                    if r_i % 2 == 0:
                        t_style.append(('BACKGROUND', (0, r_i), (-1, r_i), colors.HexColor("#F8FAFC")))
                t.setStyle(TableStyle(t_style))
                story.append(Spacer(1, 4))
                story.append(t)
                story.append(Spacer(1, 6))
            table_rows = []

        # Empty lines
        if not stripped:
            i += 1
            continue

        # Horizontal rules
        if stripped in ('---', '***', '___'):
            story.append(Spacer(1, 4))
            story.append(HRFlowable(width="100%", thickness=0.5, color=colors.HexColor("#CBD5E1"), spaceBefore=3, spaceAfter=5))
            i += 1
            continue

        # Title
        if stripped.startswith('# '):
            title_text = stripped[2:].strip()
            story.append(Paragraph(title_text, title_style))
            i += 1
            continue

        # Headings
        if stripped.startswith('### '):
            text = stripped[4:].strip()
            text = re.sub(r'\*\*(.*?)\*\*', r'<b>\1</b>', text)
            text = re.sub(r'`(.*?)`', r'<font name="Courier">\1</font>', text)
            story.append(Paragraph(text, h3_style))
            i += 1
            continue

        if stripped.startswith('## '):
            text = stripped[3:].strip()
            text = re.sub(r'\*\*(.*?)\*\*', r'<b>\1</b>', text)
            text = re.sub(r'`(.*?)`', r'<font name="Courier">\1</font>', text)
            story.append(Paragraph(text, h1_style))
            i += 1
            continue

        # Bullets
        if stripped.startswith('- ') or stripped.startswith('* ') or re.match(r'^\d+\.\s', stripped):
            is_num = bool(re.match(r'^\d+\.\s', stripped))
            if is_num:
                m = re.match(r'^(\d+\.)\s*(.*)', stripped)
                bullet_prefix = m.group(1) + " "
                body_part = m.group(2)
            else:
                bullet_prefix = "&bull; "
                body_part = stripped[2:]
            
            body_part = re.sub(r'\*\*(.*?)\*\*', r'<b>\1</b>', body_part)
            body_part = re.sub(r'`(.*?)`', r'<font name="Courier">\1</font>', body_part)
            story.append(Paragraph(f"{bullet_prefix}{body_part}", bullet_style))
            i += 1
            continue

        # Metadata or standard body paragraph
        p_text = stripped
        p_text = re.sub(r'\*\*(.*?)\*\*', r'<b>\1</b>', p_text)
        p_text = re.sub(r'`(.*?)`', r'<font name="Courier">\1</font>', p_text)
        
        if p_text.startswith('<b>Date:</b>') or p_text.startswith('<b>Author:</b>') or p_text.startswith('<b>Mission'):
            story.append(Paragraph(p_text, meta_style))
        else:
            story.append(Paragraph(p_text, body_style))

        i += 1

    doc.build(story, canvasmaker=NumberedCanvas)
    print(f"Built PDF: {output_pdf_path}")

if __name__ == "__main__":
    md_file = "docs/reports/choir-yaegi-private-go-actor-kernel-reorientation-report-2026-08-26.md"
    
    # Destination 1: iCloud Drive Choir Reports
    icloud_dir = os.path.expanduser("~/Library/Mobile Documents/com~apple~CloudDocs/Choir Reports")
    icloud_pdf = os.path.join(icloud_dir, "choir-yaegi-private-go-actor-kernel-status-and-reorientation-report-2026-08-26.pdf")
    
    # Destination 2: Local tmp for rendering verification
    tmp_pdf = "tmp/pdfs/choir-yaegi-private-go-actor-kernel-status-and-reorientation-report-2026-08-26.pdf"
    
    build_pdf(md_file, icloud_pdf)
    build_pdf(md_file, tmp_pdf)
