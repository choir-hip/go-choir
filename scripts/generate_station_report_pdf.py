#!/usr/bin/env python3
"""
Generate Choir Master System Orientation & Station Report PDF
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
            self.drawString(40, 755, "Choir Master System Orientation & Station Report — 2026-08-25")
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
    os.makedirs(os.path.dirname(output_pdf_path), exist_ok=True)
    
    doc = SimpleDocTemplate(
        output_pdf_path,
        pagesize=letter,
        leftMargin=40,
        rightMargin=40,
        topMargin=54,
        bottomMargin=54
    )

    styles = getSampleStyleSheet()
    
    # Custom styles
    title_style = ParagraphStyle(
        'DocTitle',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=20,
        leading=24,
        textColor=colors.HexColor("#0F172A"),
        spaceAfter=6
    )
    
    meta_style = ParagraphStyle(
        'DocMeta',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=8.5,
        leading=12,
        textColor=colors.HexColor("#475569"),
        spaceAfter=14
    )

    h1_style = ParagraphStyle(
        'Heading1_Custom',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=13,
        leading=16,
        textColor=colors.HexColor("#0F2942"),
        spaceBefore=14,
        spaceAfter=6,
        keepWithNext=True
    )

    h2_style = ParagraphStyle(
        'Heading2_Custom',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=10.5,
        leading=13.5,
        textColor=colors.HexColor("#0E7490"),
        spaceBefore=10,
        spaceAfter=4,
        keepWithNext=True
    )

    h3_style = ParagraphStyle(
        'Heading3_Custom',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=9.5,
        leading=12.5,
        textColor=colors.HexColor("#334155"),
        spaceBefore=8,
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
        leftIndent=12,
        firstLineIndent=-8,
        spaceAfter=3
    )

    code_block_style = ParagraphStyle(
        'CodeBlock',
        parent=styles['Normal'],
        fontName='Courier',
        fontSize=7,
        leading=8.5,
        textColor=colors.HexColor("#0F172A")
    )

    table_cell_style = ParagraphStyle(
        'TableCell',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=7.5,
        leading=9.5,
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
                # End code block
                in_code_block = False
                code_text = '\n'.join(code_lines)
                code_lines = []
                
                # Wrap preformatted code in a single cell Table for background styling
                p_code = Preformatted(code_text, code_block_style)
                code_table = Table([[p_code]], colWidths=[letter[0] - 80])
                code_table.setStyle(TableStyle([
                    ('BACKGROUND', (0, 0), (-1, -1), colors.HexColor("#F8FAFC")),
                    ('BOX', (0, 0), (-1, -1), 0.5, colors.HexColor("#CBD5E1")),
                    ('TOPPADDING', (0, 0), (-1, -1), 5),
                    ('BOTTOMPADDING', (0, 0), (-1, -1), 5),
                    ('LEFTPADDING', (0, 0), (-1, -1), 6),
                    ('RIGHTPADDING', (0, 0), (-1, -1), 6),
                ]))
                story.append(Spacer(1, 4))
                story.append(code_table)
                story.append(Spacer(1, 6))
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
            # Table line
            if '---' in stripped and ('|' in stripped):
                # Separator line, skip
                i += 1
                continue
            
            cells = [c.strip() for c in stripped[1:-1].split('|')]
            table_rows.append(cells)
            in_table = True
            i += 1
            continue
        elif in_table:
            # End of table
            in_table = False
            # Render table
            if table_rows:
                num_cols = max(len(r) for r in table_rows)
                # Compute width per column
                avail_width = letter[0] - 80
                col_width = avail_width / num_cols
                
                formatted_data = []
                for row_idx, row in enumerate(table_rows):
                    row_cells = []
                    is_head = (row_idx == 0)
                    for col in row:
                        style_to_use = table_header_style if is_head else table_cell_style
                        # Format bold
                        text = col.replace('**', '<b>').replace('**', '</b>')
                        row_cells.append(Paragraph(text, style_to_use))
                    # Pad if needed
                    while len(row_cells) < num_cols:
                        row_cells.append(Paragraph("", table_cell_style))
                    formatted_data.append(row_cells)
                
                t = Table(formatted_data, colWidths=[col_width]*num_cols)
                t_style = [
                    ('BACKGROUND', (0, 0), (-1, 0), colors.HexColor("#0F2942")),
                    ('ALIGN', (0, 0), (-1, -1), 'LEFT'),
                    ('VALIGN', (0, 0), (-1, -1), 'TOP'),
                    ('TOPPADDING', (0, 0), (-1, -1), 4),
                    ('BOTTOMPADDING', (0, 0), (-1, -1), 4),
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

        # Headings
        if stripped.startswith('# '):
            text = stripped[2:].strip()
            story.append(Paragraph(text, title_style))
            i += 1
            continue
        elif stripped.startswith('## '):
            text = stripped[3:].strip()
            story.append(Paragraph(text, h1_style))
            story.append(HRFlowable(width="100%", thickness=1, color=colors.HexColor("#0E7490"), spaceBefore=2, spaceAfter=6))
            i += 1
            continue
        elif stripped.startswith('### '):
            text = stripped[4:].strip()
            story.append(Paragraph(text, h2_style))
            i += 1
            continue
        elif stripped.startswith('#### '):
            text = stripped[5:].strip()
            story.append(Paragraph(text, h3_style))
            i += 1
            continue

        # Horizontal Rule
        if stripped in ['---', '***', '___']:
            story.append(HRFlowable(width="100%", thickness=0.5, color=colors.HexColor("#CBD5E1"), spaceBefore=6, spaceAfter=8))
            i += 1
            continue

        # Metadata Header block
        if stripped.startswith('**Date:**') or stripped.startswith('**Context:**') or stripped.startswith('**Target:**') or stripped.startswith('**Authority:**'):
            text = stripped.replace('**', '<b>', 1).replace('**', '</b>', 1)
            story.append(Paragraph(text, meta_style))
            i += 1
            continue

        # Bullet lists
        if stripped.startswith('- ') or stripped.startswith('* ') or re.match(r'^\d+\.\s', stripped):
            if stripped.startswith('- ') or stripped.startswith('* '):
                bullet_text = stripped[2:]
            else:
                bullet_text = re.sub(r'^\d+\.\s', '', stripped)
            
            # Format markdown bold & italic & code
            t = bullet_text
            t = re.sub(r'\*\*(.+?)\*\*', r'<b>\1</b>', t)
            t = re.sub(r'\*(.+?)\*', r'<i>\1</i>', t)
            t = re.sub(r'`(.+?)`', r'<font name="Courier">\1</font>', t)
            story.append(Paragraph(f"• {t}", bullet_style))
            i += 1
            continue

        # Normal Paragraph
        t = stripped
        t = re.sub(r'\*\*(.+?)\*\*', r'<b>\1</b>', t)
        t = re.sub(r'\*(.+?)\*', r'<i>\1</i>', t)
        t = re.sub(r'`(.+?)`', r'<font name="Courier">\1</font>', t)
        story.append(Paragraph(t, body_style))
        i += 1

    doc.build(story, canvasmaker=NumberedCanvas)
    print(f"Built PDF: {output_pdf_path}")

if __name__ == '__main__':
    md_file = sys.argv[1] if len(sys.argv) > 1 else 'docs/reports/choir-system-orientation-and-station-report-2026-08-25.md'
    out_pdf = sys.argv[2] if len(sys.argv) > 2 else 'output/pdf/choir-system-orientation-and-station-report-2026-08-25.pdf'
    build_pdf(md_file, out_pdf)
