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
            self.drawString(54, 750, "Choir Autonomous Self-Supervision & Self-Development Master Blueprint")
            self.drawRightString(612 - 54, 750, "2026-08-20")
            self.setStrokeColor(colors.HexColor("#E2E8F0"))
            self.setLineWidth(0.75)
            self.line(54, 744, 612 - 54, 744)
        
        # Footer (all pages)
        self.setStrokeColor(colors.HexColor("#E2E8F0"))
        self.setLineWidth(0.75)
        self.line(54, 45, 612 - 54, 45)
        
        self.drawString(54, 32, "Confidential — Choir Architecture Master Blueprint")
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
    c_accent_bg = colors.HexColor("#F0FDF4")      # Green 50
    c_accent_border = colors.HexColor("#86EFAC")  # Green 300
    
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
        fontSize=10.5,
        leading=14,
        textColor=c_secondary,
        spaceAfter=8
    )
    
    meta_style = ParagraphStyle(
        'MetaText',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=7.5,
        leading=11,
        textColor=c_muted
    )
    
    h1_style = ParagraphStyle(
        'Heading1_Custom',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=11.5,
        leading=15,
        textColor=c_primary,
        spaceBefore=10,
        spaceAfter=4,
        keepWithNext=True
    )

    h2_style = ParagraphStyle(
        'Heading2_Custom',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=9,
        leading=12,
        textColor=c_secondary,
        spaceBefore=7,
        spaceAfter=3,
        keepWithNext=True
    )
    
    body_style = ParagraphStyle(
        'Body_Custom',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=8,
        leading=11.5,
        textColor=c_dark,
        spaceAfter=4
    )
    
    bullet_style = ParagraphStyle(
        'Bullet_Custom',
        parent=body_style,
        leftIndent=10,
        firstLineIndent=-6,
        spaceAfter=2.5
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
    
    # Title & Metadata Banner
    story.append(Paragraph("Choir Autonomous Computer Self-Supervision Master Blueprint", title_style))
    story.append(Paragraph("Architectural Synthesis & Execution Roadmap for Arbitrary Goals with Verification", subtitle_style))
    
    meta_table_data = [
        [
            Paragraph("<b>Date:</b> 2026-08-20 (UTC)", meta_style),
            Paragraph("<b>Supervision Rounds:</b> Divergent → Lateral → Convergent (3 Rounds)", meta_style)
        ],
        [
            Paragraph("<b>Restore Fence:</b> Checkpoint 99949fe2 (Epoch 324)", meta_style),
            Paragraph("<b>Panelists:</b> Claude Opus, Gemini 3.7, Grok 4.6, Devin, Cursor, Opencode, Codex", meta_style)
        ]
    ]
    meta_table = Table(meta_table_data, colWidths=[250, 254])
    meta_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,-1), c_bg_light),
        ('BOX', (0,0), (-1,-1), 0.75, c_border),
        ('INNERGRID', (0,0), (-1,-1), 0.5, colors.HexColor("#E2E8F0")),
        ('TOPPADDING', (0,0), (-1,-1), 3.5),
        ('BOTTOMPADDING', (0,0), (-1,-1), 3.5),
        ('LEFTPADDING', (0,0), (-1,-1), 6),
        ('RIGHTPADDING', (0,0), (-1,-1), 6),
    ]))
    story.append(meta_table)
    story.append(Spacer(1, 6))
    
    # 1. Executive Verdict & Core Invariants
    story.append(Paragraph("1. Executive Verdict & Core Doctrinal Invariants", h1_style))
    story.append(Paragraph(
        "The three-round agentic supervision panel confirms that Choir is transitioning from an externally driven coding harness to an <b>autonomous, self-supervising, self-developing computer</b>. Self-development is not synthetic milestone playback or external git manipulation: it is an audited user computer modifying its own code within guest capsules via the <code>choir</code> CLI, supervised through live, monotonically incrementing Texture document revisions (v0 -> vn) on <code>choir.news</code>, and committed to the computer event tape only after multi-domain consensus and verification pass.",
        body_style
    ))
    
    invariants_data = [
        [
            Paragraph("<b>1. One Semantic Authority:</b> Single <code>ComputerID</code> + canonical event chain. Capsules are inert until policy acceptance.", bullet_style),
            Paragraph("<b>2. Dynamic Document Revisions:</b> Version numbers (v0 -> vn) represent living revisions, NOT hardcoded static stages.", bullet_style)
        ],
        [
            Paragraph("<b>3. Conductor-Intake Pipeline:</b> <code>Conductor (Intake) -> Texture (Supervision) -> Super (Governor) -> CoSuper (Actuator)</code>.", bullet_style),
            Paragraph("<b>4. In-VM Flywheel Authorship:</b> The computer modifies its own code inside guest capsules via the <code>choir</code> CLI.", bullet_style)
        ]
    ]
    invariants_table = Table(invariants_data, colWidths=[250, 254])
    invariants_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,-1), c_callout_bg),
        ('BOX', (0,0), (-1,-1), 0.75, c_callout_border),
        ('TOPPADDING', (0,0), (-1,-1), 4),
        ('BOTTOMPADDING', (0,0), (-1,-1), 4),
        ('LEFTPADDING', (0,0), (-1,-1), 5),
        ('RIGHTPADDING', (0,0), (-1,-1), 5),
    ]))
    story.append(invariants_table)
    story.append(Spacer(1, 6))

    # 2. 3-Round Supervision Synthesis Table
    story.append(Paragraph("2. Three-Round Agentic Supervision Synthesis", h1_style))
    rounds_data = [
        [
            Paragraph("Round & Mode", table_header),
            Paragraph("Core Conceptual Shift / Inversion", table_header),
            Paragraph("Architectural Consequence for Choir", table_header)
        ],
        [
            Paragraph("<b>Round 1</b><br/>Divergent Mode", table_cell_bold),
            Paragraph("Texture as an Executable Notebook / AST Projection Canvas rather than passive log viewer; probe-first goal intake.", table_cell),
            Paragraph("Revisions reflect structured AST updates and evidence receipts; Conductor probes repo before committing scope.", table_cell)
        ],
        [
            Paragraph("<b>Round 2</b><br/>Lateral Mode", table_cell_bold),
            Paragraph("<b>The Tactic Engine Frame:</b> Models author Go tactics; capsules execute at machine speed. <b>The Autopilot Frame:</b> Computer is the agent; LLM is an exception handler.", table_cell),
            Paragraph("Eliminates 200-iteration LLM tool loop caps; iteration occurs in compiled Go cells; activations are gated on provable ignorance.", table_cell)
        ],
        [
            Paragraph("<b>Round 3</b><br/>Convergent Mode", table_cell_bold),
            Paragraph("Document-driven supervision without chat-turn regression; Conductor intake; sequenced Yaegi RLM cutover.", table_cell),
            Paragraph("Proven effects spine on staging baseline first; Yaegi RLM single-actuator engine in successor definition.", table_cell)
        ]
    ]
    rounds_table = Table(rounds_data, colWidths=[75, 215, 214])
    rounds_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), c_primary),
        ('BOX', (0,0), (-1,-1), 0.75, c_border),
        ('INNERGRID', (0,0), (-1,-1), 0.5, colors.HexColor("#E2E8F0")),
        ('TOPPADDING', (0,0), (-1,-1), 3),
        ('BOTTOMPADDING', (0,0), (-1,-1), 3),
        ('LEFTPADDING', (0,0), (-1,-1), 4),
        ('RIGHTPADDING', (0,0), (-1,-1), 4),
        ('ROWBACKGROUNDS', (0,1), (-1,-1), [colors.white, c_bg_light])
    ]))
    story.append(rounds_table)
    story.append(Spacer(1, 6))

    # 3. Dynamic Live Supervision Protocol
    story.append(Paragraph("3. Dynamic Live Supervision & Citation Transclusion Protocol", h1_style))
    story.append(Paragraph(
        "When an arbitrary goal is submitted via the CLI (<code>choir run start --prompt \'...\'</code>) or Prompt Bar, Texture maintains an active living document with clickable citations that unfold into live transcluded content on <code>choir.news</code>:",
        body_style
    ))
    
    citations_data = [
        [
            Paragraph("Citation Tag", table_header),
            Paragraph("Transcluded Living Content", table_header),
            Paragraph("Supervisory Role in Living Document", table_header)
        ],
        [
            Paragraph("<code>[prompt:seed]</code>", table_cell_bold),
            Paragraph("Raw user goal text, commitment hash, and intake timestamp.", table_cell),
            Paragraph("Records goal provenance as received by Conductor.", table_cell)
        ],
        [
            Paragraph("<code>[trajectory:work]</code>", table_cell_bold),
            Paragraph("Open work items, role bindings, and settlement rules.", table_cell),
            Paragraph("Tracks active multi-agent supervisory obligations.", table_cell)
        ],
        [
            Paragraph("<code>[source:diff]</code>", table_cell_bold),
            Paragraph("Live unified diff of modified files in the guest capsule overlay.", table_cell),
            Paragraph("Allows real-time inspection of code changes before freeze.", table_cell)
        ],
        [
            Paragraph("<code>[test:receipts]</code>", table_cell_bold),
            Paragraph("Command execution receipts, test exit codes, and verifier logs.", table_cell),
            Paragraph("Proves empirical test passage directly from capsule execution.", table_cell)
        ],
        [
            Paragraph("<code>[consensus:quorum]</code>", table_cell_bold),
            Paragraph("Multi-agent policy votes, dissents, and acceptance signature.", table_cell),
            Paragraph("Authorizes promotion under frozen decision policy.", table_cell)
        ]
    ]
    citations_table = Table(citations_data, colWidths=[90, 214, 200])
    citations_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), c_primary),
        ('BOX', (0,0), (-1,-1), 0.75, c_border),
        ('INNERGRID', (0,0), (-1,-1), 0.5, colors.HexColor("#E2E8F0")),
        ('TOPPADDING', (0,0), (-1,-1), 3),
        ('BOTTOMPADDING', (0,0), (-1,-1), 3),
        ('LEFTPADDING', (0,0), (-1,-1), 4),
        ('RIGHTPADDING', (0,0), (-1,-1), 4),
        ('ROWBACKGROUNDS', (0,1), (-1,-1), [colors.white, c_bg_light])
    ]))
    story.append(citations_table)
    story.append(Spacer(1, 6))

    # 4. Phased Execution Roadmap
    story.append(Paragraph("4. Phased Execution Roadmap for Arbitrary Goals", h1_style))
    story.append(Paragraph(
        "• <b>Phase 1 (Current Active Effects Definition):</b> Prove the live effects acceptance spine on staging (Epoch 333). CoSuper authors candidate code inside guest capsule; freeze 5 bundle artifacts; run qualified consensus under <code>reversible-selfdev-v1</code>; promote, verify live play, submit falsification, and execute acceptance-fenced restore to pre-A checkpoint <code>99949fe2</code>.<br/>"
        "• <b>Phase 2 (Yaegi RLM Private Go Kernel Cutover):</b> Implement <code>choir-private-go-actor-kernel-2026-08-12</code> as the permanent execution upgrade. Models author Go cells executed in milliseconds inside capsules, eliminating 200-iteration LLM tool loop limits.<br/>"
        "• <b>Phase 3 (Self-Development Flywheel):</b> Enable general-purpose autonomous in-VM self-development for arbitrary user goals via the <code>choir</code> CLI, with real-time supervision streaming over Texture on <code>choir.news</code>.",
        body_style
    ))
    
    # Signature box
    story.append(Spacer(1, 5))
    sig_data = [
        [
            Paragraph("<b>Architecture:</b> Choir Engineering", meta_style),
            Paragraph("<b>Consensus Panel:</b> Claude, Gemini 3.7, Grok 4.6, Devin, Cursor, Opencode, Codex", meta_style),
            Paragraph("<b>Target Horizon:</b> General In-VM Self-Development", meta_style)
        ]
    ]
    sig_table = Table(sig_data, colWidths=[140, 224, 140])
    sig_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,-1), c_accent_bg),
        ('BOX', (0,0), (-1,-1), 0.75, c_accent_border),
        ('TOPPADDING', (0,0), (-1,-1), 3),
        ('BOTTOMPADDING', (0,0), (-1,-1), 3),
        ('LEFTPADDING', (0,0), (-1,-1), 5),
        ('RIGHTPADDING', (0,0), (-1,-1), 5),
    ]))
    story.append(sig_table)

    doc.build(story, canvasmaker=NumberedCanvas)
    print(f"Successfully generated PDF: {filename}")

if __name__ == '__main__':
    out_pdf = "tmp/pdfs/choir-autonomous-supervision-and-self-development-master-blueprint-2026-08-20.pdf"
    if len(sys.argv) > 1:
        out_pdf = sys.argv[1]
    build_pdf(out_pdf)
