import os
import sys
from reportlab.lib.pagesizes import letter
from reportlab.lib import colors
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.platypus import (
    SimpleDocTemplate, Paragraph, Spacer, Table, TableStyle, PageBreak, KeepTogether, HRFlowable
)
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
            self.drawString(54, 750, "Choir 24-Hour Platform Arc & Critical Architecture Review")
            self.drawRightString(612 - 54, 750, "2026-08-20")
            self.setStrokeColor(colors.HexColor("#E2E8F0"))
            self.setLineWidth(0.75)
            self.line(54, 744, 612 - 54, 744)
        
        # Footer (all pages)
        self.setStrokeColor(colors.HexColor("#E2E8F0"))
        self.setLineWidth(0.75)
        self.line(54, 45, 612 - 54, 45)
        
        self.drawString(54, 32, "Confidential — Choir Automatic Computer Architecture Review")
        page_str = f"Page {self._pageNumber} of {page_count}"
        self.drawRightString(612 - 54, 32, page_str)
        self.restoreState()

def build_pdf(filename):
    doc = SimpleDocTemplate(
        filename,
        pagesize=letter,
        leftMargin=54,
        rightMargin=54,
        topMargin=54,
        bottomMargin=54
    )
    
    styles = getSampleStyleSheet()
    
    # Custom Palette
    c_primary = colors.HexColor("#0F172A")        # Slate 900
    c_secondary = colors.HexColor("#2563EB")      # Blue 600
    c_dark = colors.HexColor("#334155")           # Slate 700
    c_muted = colors.HexColor("#64748B")          # Slate 500
    c_bg_light = colors.HexColor("#F8FAFC")       # Slate 50
    c_border = colors.HexColor("#CBD5E1")         # Slate 300
    c_callout_bg = colors.HexColor("#EFF6FF")     # Blue 50
    c_callout_border = colors.HexColor("#93C5FD") # Blue 300
    c_warn_bg = colors.HexColor("#FFFBEB")        # Amber 50
    c_warn_border = colors.HexColor("#FCD34D")    # Amber 300
    
    title_style = ParagraphStyle(
        'DocTitle',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=18,
        leading=22,
        textColor=c_primary,
        spaceAfter=3
    )
    
    subtitle_style = ParagraphStyle(
        'DocSubTitle',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=11,
        leading=15,
        textColor=c_secondary,
        spaceAfter=10
    )
    
    meta_style = ParagraphStyle(
        'MetaText',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=8,
        leading=11,
        textColor=c_muted
    )
    
    h1_style = ParagraphStyle(
        'Heading1_Custom',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=12,
        leading=16,
        textColor=c_primary,
        spaceBefore=11,
        spaceAfter=4,
        keepWithNext=True
    )

    h2_style = ParagraphStyle(
        'Heading2_Custom',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=9.5,
        leading=13,
        textColor=c_secondary,
        spaceBefore=8,
        spaceAfter=3,
        keepWithNext=True
    )
    
    body_style = ParagraphStyle(
        'Body_Custom',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=8.5,
        leading=12,
        textColor=c_dark,
        spaceAfter=4
    )
    
    bullet_style = ParagraphStyle(
        'Bullet_Custom',
        parent=body_style,
        leftIndent=10,
        firstLineIndent=-6,
        spaceAfter=3
    )
    
    table_cell = ParagraphStyle(
        'TableCell',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=7.5,
        leading=10,
        textColor=c_dark
    )
    
    table_cell_bold = ParagraphStyle(
        'TableCellBold',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=7.5,
        leading=10,
        textColor=c_primary
    )
    
    table_header = ParagraphStyle(
        'TableHeader',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=8,
        leading=11,
        textColor=colors.white
    )

    story = []
    
    # Title & Header
    story.append(Paragraph("Choir 24-Hour Platform Arc & Critical Architecture Review", title_style))
    story.append(Paragraph("Technical Progress, Substrate Discoveries & Multi-Agent Consensus Report", subtitle_style))
    
    meta_table_data = [
        [
            Paragraph("<b>Date:</b> 2026-08-20 (UTC)", meta_style),
            Paragraph("<b>Coverage:</b> 2026-08-19 00:00 — 2026-08-20 04:00 (32 commits)", meta_style)
        ],
        [
            Paragraph("<b>Pre-A Checkpoint:</b> 99949fe2 (Epoch 324)", meta_style),
            Paragraph("<b>Consensus Panel:</b> Claude Opus, Gemini 3.7, Devin, Cursor, Codex", meta_style)
        ]
    ]
    meta_table = Table(meta_table_data, colWidths=[250, 254])
    meta_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,-1), c_bg_light),
        ('BOX', (0,0), (-1,-1), 0.75, c_border),
        ('INNERGRID', (0,0), (-1,-1), 0.5, colors.HexColor("#E2E8F0")),
        ('TOPPADDING', (0,0), (-1,-1), 4),
        ('BOTTOMPADDING', (0,0), (-1,-1), 4),
        ('LEFTPADDING', (0,0), (-1,-1), 6),
        ('RIGHTPADDING', (0,0), (-1,-1), 6),
    ]))
    story.append(meta_table)
    story.append(Spacer(1, 8))
    
    # 1. Executive Summary & Verdict
    story.append(Paragraph("1. Executive Assessment & Architectural Verdict", h1_style))
    story.append(Paragraph(
        "Over the 24-hour window from 2026-08-19 00:00 to 2026-08-20 04:00 UTC (32 commits on <code>main</code>), Choir completed a pivotal architectural transition: establishing strict single-computer semantic monism, publishing the pre-A restore checkpoint on staging, repairing non-interactive guest capsule execution, resolving a 5-stage Super continuation failure ladder to discover the Texture control-plane root cause, slashing CI test duration by 2.2x (18.9m to 8.5m), and authoring the complete Candidate A Solitaire headless engine.",
        body_style
    ))
    
    verdict_data = [[
        Paragraph("<b>PANEL VERDICT:</b> <b>ACCEPT</b> the platform convergence arc (Milestones 1–5). <b>CONDITIONAL APPROVAL</b> for live Candidate A promotion, requiring four pre-promotion safeguards (F1–F4) before initiating the live promotion transaction.", ParagraphStyle('VerdictText', parent=body_style, fontSize=8, leading=11, textColor=colors.HexColor("#1E3A8A")))
    ]]
    verdict_table = Table(verdict_data, colWidths=[504])
    verdict_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,-1), c_callout_bg),
        ('BOX', (0,0), (-1,-1), 0.75, c_callout_border),
        ('TOPPADDING', (0,0), (-1,-1), 5),
        ('BOTTOMPADDING', (0,0), (-1,-1), 5),
        ('LEFTPADDING', (0,0), (-1,-1), 8),
        ('RIGHTPADDING', (0,0), (-1,-1), 8),
    ]))
    story.append(verdict_table)
    story.append(Spacer(1, 6))

    # 2. Milestones Summary Table
    story.append(Paragraph("2. 24-Hour Platform Milestones Summary", h1_style))
    milestones_data = [
        [
            Paragraph("Milestone", table_header),
            Paragraph("Core Technical Accomplishment", table_header),
            Paragraph("Key Commits", table_header),
            Paragraph("Staging Realization", table_header)
        ],
        [
            Paragraph("<b>1. Family A Monism</b>", table_cell_bold),
            Paragraph("Scoped aliases, composite PKs, write deadline extensions (110s). Achieved 40/40 Dolt table replay equivalence (Seq 3403).", table_cell),
            Paragraph("<code>ebce5455</code><br/><code>0aa1ffb3</code><br/><code>087cf290</code>", table_cell),
            Paragraph("Checkpoint <code>99949fe2</code><br/>Epoch 324", table_cell)
        ],
        [
            Paragraph("<b>2. Capsule Broker</b>", table_cell_bold),
            Paragraph("Non-interactive POSIX job-control repair (<code>sh -c</code> without <code>+m</code>). CoSuper executed 200 clean steps without <code>getpgrp</code> crash.", table_cell),
            Paragraph("<code>651d86bc</code><br/><code>d33f245c</code>", table_cell),
            Paragraph("CoSuper <code>97191e37</code><br/>Epoch 326", table_cell)
        ],
        [
            Paragraph("<b>3. Super Actor Ladder</b>", table_cell_bold),
            Paragraph("Traversed 5 stages (non-rewake, storm, prompt, max-tokens) to uncover root cause: Super authority flows strictly from Texture <code>execution_request</code> Control.", table_cell),
            Paragraph("<code>3654d925</code><br/><code>9a55b756</code><br/><code>177a7415</code>", table_cell),
            Paragraph("Texture Rewake<br/>Epoch 331", table_cell)
        ],
        [
            Paragraph("<b>4. Doctrine Realignment</b>", table_cell_bold),
            Paragraph("Doctrinal pipeline: <code>Conductor (Intake) -> Texture (Live Supervision) -> Super (Governor) -> CoSuper (Capsule Actuator)</code>.", table_cell),
            Paragraph("<code>7559300c</code>", table_cell),
            Paragraph("Living Document<br/>Transclusion", table_cell)
        ],
        [
            Paragraph("<b>5. CI Acceleration</b>", table_cell_bold),
            Paragraph("Sub-sharded <code>internal/store</code> (310 tests) across 4 runners and expanded runtime matrix to 6 shards. Slashed test phase from 18.9m to 8.5m.", table_cell),
            Paragraph("<code>7c6c3899</code>", table_cell),
            Paragraph("CI Wall Time<br/>~13m Total", table_cell)
        ],
        [
            Paragraph("<b>6. Candidate A Solitaire</b>", table_cell_bold),
            Paragraph("Klondike Solitaire engine, additive Dolt/SQLite schema, REST API, and 5 bundle artifacts with pre-declared foundation defect.", table_cell),
            Paragraph("<code>0bff0b0c</code>", table_cell),
            Paragraph("Headless REST<br/><code>/api/solitaire</code>", table_cell)
        ]
    ]
    milestones_table = Table(milestones_data, colWidths=[80, 244, 85, 95])
    milestones_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), c_primary),
        ('BOX', (0,0), (-1,-1), 0.75, c_border),
        ('INNERGRID', (0,0), (-1,-1), 0.5, colors.HexColor("#E2E8F0")),
        ('TOPPADDING', (0,0), (-1,-1), 3),
        ('BOTTOMPADDING', (0,0), (-1,-1), 3),
        ('LEFTPADDING', (0,0), (-1,-1), 4),
        ('RIGHTPADDING', (0,0), (-1,-1), 4),
        ('ROWBACKGROUNDS', (0,1), (-1,-1), [colors.white, c_bg_light])
    ]))
    story.append(milestones_table)
    story.append(Spacer(1, 8))

    # 3. Critical Findings Table
    story.append(Paragraph("3. Adversarial Panel Findings & Pre-Promotion Adjudications", h1_style))
    story.append(Paragraph(
        "A line-level audit by the consensus panel identified four critical findings that must be addressed prior to live promotion of Candidate A:",
        body_style
    ))
    
    findings_data = [
        [
            Paragraph("Finding", table_header),
            Paragraph("Component & Line", table_header),
            Paragraph("Observed Vulnerability", table_header),
            Paragraph("Mandated Pre-Promotion Safeguard", table_header)
        ],
        [
            Paragraph("<b>F1 (High)</b>", table_cell_bold),
            Paragraph("<code>desktop_live.go:261</code>", table_cell),
            Paragraph("<code>(computer_id != '' OR computer_id = '')</code> matches all rows; deletes across computers for same owner.", table_cell),
            Paragraph("Pass exact <code>computerID</code> parameter to <code>deleteDesktopPlacementsNotIn</code>; enforce strict equality.", table_cell)
        ],
        [
            Paragraph("<b>F2 (High)</b>", table_cell_bold),
            Paragraph("<code>solitaire/store.go</code>", table_cell),
            Paragraph("Writes directly via SQL without <code>computerevent.ProjectionOp</code>; replay fails to regenerate solitaire tables -> witness diverges.", table_cell),
            Paragraph("Connect <code>solitaire/store.go</code> to computer event projection sink before promotion so mutations are in event tape.", table_cell)
        ],
        [
            Paragraph("<b>F3 (High)</b>", table_cell_bold),
            Paragraph("<code>solitaire/handler.go</code>", table_cell),
            Paragraph("Auth headers fall back open to <code>'default-owner'</code> on unauthenticated requests.", table_cell),
            Paragraph("Enforce strict proxy authentication check; return HTTP 401 on missing auth.", table_cell)
        ],
        [
            Paragraph("<b>F4 (Med)</b>", table_cell_bold),
            Paragraph("<code>solitaire/engine.go</code>", table_cell),
            Paragraph("When <code>seed==0</code>, generated seed is discarded; <code>DeckSeed: 0</code> stored. Games not reproducible from record.", table_cell),
            Paragraph("Record generated crypto seed in <code>GameState.DeckSeed</code> so all sessions are bit-for-bit replayable.", table_cell)
        ]
    ]
    findings_table = Table(findings_data, colWidths=[55, 100, 185, 164])
    findings_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), c_primary),
        ('BOX', (0,0), (-1,-1), 0.75, c_border),
        ('INNERGRID', (0,0), (-1,-1), 0.5, colors.HexColor("#E2E8F0")),
        ('TOPPADDING', (0,0), (-1,-1), 3),
        ('BOTTOMPADDING', (0,0), (-1,-1), 3),
        ('LEFTPADDING', (0,0), (-1,-1), 4),
        ('RIGHTPADDING', (0,0), (-1,-1), 4),
        ('ROWBACKGROUNDS', (0,1), (-1,-1), [colors.white, c_warn_bg])
    ]))
    story.append(findings_table)
    story.append(Spacer(1, 8))

    # 4. Texture Live Supervision & Self-Development Flywheel
    story.append(Paragraph("4. Texture Live Supervision & Self-Development Flywheel", h1_style))
    story.append(Paragraph(
        "<b>The Self-Development Flywheel:</b> Choir moves from external coding-harness git-push cycles to autonomous in-VM self-development via the <code>choir</code> CLI and Prompt Bar. Conductor intakes the user request -> Texture creates living document -> Super orchestrates -> CoSuper authors candidate changes inside guest capsules -> Texture updates living revisions (v0 -> vn) with clickable citations transcluding live diffs and test receipts -> Consensus accepts -> VM promotes its own code.",
        body_style
    ))
    story.append(Paragraph(
        "• <b>Clean Repo Cutover:</b> Removed hardcoded example packages and routes from git. Candidate changes are authored dynamically inside guest VM capsules.<br/>"
        "• <b>Living Revision Protocol:</b> Texture commits human-readable revisions (v0 intake -> v1 scope -> v2 delegation -> v3 authoring diffs -> v4 bundle freeze -> v5 consensus -> v6 promotion -> v7 falsification -> v8 restore).<br/>"
        "• <b>Live Observable Proof:</b> Trajectory progress is supervised in real-time by opening the active Texture document on <code>choir.news</code>.",
        body_style
    ))
    
    # Signature box
    story.append(Spacer(1, 6))
    sig_data = [
        [
            Paragraph("<b>Prepared by:</b> Choir Engineering", meta_style),
            Paragraph("<b>Consensus Panel:</b> Claude Opus, Gemini 3.7, Devin, Cursor, Codex", meta_style),
            Paragraph("<b>Restore Fence:</b> Checkpoint 99949fe2 (Epoch 324)", meta_style)
        ]
    ]
    sig_table = Table(sig_data, colWidths=[168, 168, 168])
    sig_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,-1), c_callout_bg),
        ('BOX', (0,0), (-1,-1), 0.75, c_callout_border),
        ('TOPPADDING', (0,0), (-1,-1), 4),
        ('BOTTOMPADDING', (0,0), (-1,-1), 4),
        ('LEFTPADDING', (0,0), (-1,-1), 5),
        ('RIGHTPADDING', (0,0), (-1,-1), 5),
    ]))
    story.append(sig_table)

    doc.build(story, canvasmaker=NumberedCanvas)
    print(f"Successfully generated PDF: {filename}")

if __name__ == '__main__':
    out_pdf = "tmp/pdfs/choir-24-hour-architecture-and-effects-progress-report-2026-08-20.pdf"
    if len(sys.argv) > 1:
        out_pdf = sys.argv[1]
    build_pdf(out_pdf)
