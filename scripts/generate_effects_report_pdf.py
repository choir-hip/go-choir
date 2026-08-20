import os
import sys
from reportlab.lib.pagesizes import letter
from reportlab.lib import colors
from reportlab.lib.units import inch
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
            self.drawString(54, 750, "Choir Supervised Self-Development Effects Mission — Technical Report")
            self.drawRightString(612 - 54, 750, "2026-08-20")
            self.setStrokeColor(colors.HexColor("#E2E8F0"))
            self.setLineWidth(0.75)
            self.line(54, 744, 612 - 54, 744)
        
        # Footer (all pages)
        self.setStrokeColor(colors.HexColor("#E2E8F0"))
        self.setLineWidth(0.75)
        self.line(54, 45, 612 - 54, 45)
        
        self.drawString(54, 32, "Confidential — Choir Automatic Computer Architecture")
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
    c_primary = colors.HexColor("#0F172A")    # Deep slate
    c_secondary = colors.HexColor("#2563EB")  # Accent blue
    c_dark = colors.HexColor("#334155")       # Text dark
    c_muted = colors.HexColor("#64748B")      # Muted slate
    c_bg_light = colors.HexColor("#F8FAFC")   # Light table bg
    c_border = colors.HexColor("#CBD5E1")     # Border grey
    c_callout_bg = colors.HexColor("#EFF6FF") # Light blue bg
    c_callout_border = colors.HexColor("#93C5FD") # Border blue
    
    # Custom Typography Styles
    title_style = ParagraphStyle(
        'DocTitle',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=20,
        leading=24,
        textColor=c_primary,
        spaceAfter=4
    )
    
    subtitle_style = ParagraphStyle(
        'DocSubTitle',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=12,
        leading=16,
        textColor=c_secondary,
        spaceAfter=12
    )
    
    meta_style = ParagraphStyle(
        'MetaText',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=8.5,
        leading=12,
        textColor=c_muted
    )
    
    h1_style = ParagraphStyle(
        'Heading1_Custom',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=13,
        leading=17,
        textColor=c_primary,
        spaceBefore=14,
        spaceAfter=6,
        keepWithNext=True
    )

    h2_style = ParagraphStyle(
        'Heading2_Custom',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=10.5,
        leading=14,
        textColor=c_secondary,
        spaceBefore=10,
        spaceAfter=4,
        keepWithNext=True
    )
    
    body_style = ParagraphStyle(
        'Body_Custom',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=9,
        leading=13,
        textColor=c_dark,
        spaceAfter=6
    )

    body_bold = ParagraphStyle(
        'Body_Bold',
        parent=body_style,
        fontName='Helvetica-Bold'
    )
    
    bullet_style = ParagraphStyle(
        'Bullet_Custom',
        parent=body_style,
        leftIndent=12,
        firstLineIndent=-8,
        spaceAfter=4
    )
    
    code_style = ParagraphStyle(
        'Code_Custom',
        parent=styles['Normal'],
        fontName='Courier',
        fontSize=8,
        leading=11,
        textColor=colors.HexColor("#1E293B")
    )
    
    table_cell = ParagraphStyle(
        'TableCell',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=8,
        leading=11,
        textColor=c_dark
    )
    
    table_cell_bold = ParagraphStyle(
        'TableCellBold',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=8,
        leading=11,
        textColor=c_primary
    )
    
    table_header = ParagraphStyle(
        'TableHeader',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=8.5,
        leading=12,
        textColor=colors.white
    )

    story = []
    
    # Title & Metadata Banner
    story.append(Paragraph("Choir Supervised Self-Development Effects Mission", title_style))
    story.append(Paragraph("Technical Progress, Substrate Discoveries & Architecture Consensus Report", subtitle_style))
    
    meta_table_data = [
        [
            Paragraph("<b>Date:</b> 2026-08-20 (UTC)", meta_style),
            Paragraph("<b>Classification:</b> Orange/Red Substrate & Architecture", meta_style)
        ],
        [
            Paragraph("<b>Parent Definition:</b> choir-supervised-self-development-effects-2026-08-11", meta_style),
            Paragraph("<b>Consensus Panel:</b> Claude Opus, Gemini 3.7 Flash, Devin, Codex", meta_style)
        ]
    ]
    meta_table = Table(meta_table_data, colWidths=[250, 254])
    meta_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,-1), c_bg_light),
        ('BOX', (0,0), (-1,-1), 0.75, c_border),
        ('INNERGRID', (0,0), (-1,-1), 0.5, colors.HexColor("#E2E8F0")),
        ('TOPPADDING', (0,0), (-1,-1), 5),
        ('BOTTOMPADDING', (0,0), (-1,-1), 5),
        ('LEFTPADDING', (0,0), (-1,-1), 8),
        ('RIGHTPADDING', (0,0), (-1,-1), 8),
    ]))
    story.append(meta_table)
    story.append(Spacer(1, 10))
    
    # 1. Executive Summary
    story.append(Paragraph("1. Executive Summary", h1_style))
    story.append(Paragraph(
        "Over the 24-hour window of 2026-08-19 to 2026-08-20 UTC, the Choir team executed foundational substrate repairs and platform convergence required to achieve the first autonomous supervised self-development effects run on staging. The mission objective is live proof on staging of a capsule-authored Solitaire candidate change A: CoSuper authors, builds, tests, freezes, and proposes candidate A; multi-agent qualified consensus accepts the proposal; the change is promoted to live computer code; live API/DB verification exercises the new capability; candidate A is subsequently falsified and superseded by candidate B; and the computer is restored to pre-A checkpoint <code>99949fe2</code>.",
        body_style
    ))
    story.append(Paragraph(
        "During this period, the platform reached major milestones across three foundational layers:",
        body_style
    ))
    story.append(Paragraph(
        "• <b>Phase 1 Family A Computer Monism & Checkpoint Publication:</b> Completed strict single-computer data isolation, removed cross-computer alias table leakage, unified the residue import pipeline, and published pre-A checkpoint <code>99949fe2e16d3c4c...</code> at realization epoch 324 on staging computer <code>computer-03335285269bdba4f94377e56879f9e6</code>.",
        bullet_style
    ))
    story.append(Paragraph(
        "• <b>Capsule Execution Substrate Unification:</b> Diagnosed and fixed POSIX job-control failures (<code>getpgrp</code> / <code>initialize_job_control</code> crash) in non-interactive containerized guest capsules (<code>651d86bc</code>), enabling live CoSuper <code>assignment-97191e37</code> to execute commands natively inside guest capsules on staging.",
        bullet_style
    ))
    story.append(Paragraph(
        "• <b>The 5-Stage Super Actor Continuation Ladder:</b> Systematically diagnosed and solved four cascading symptoms in the persistent Super continuation loop following CoSuper cancellation, ultimately discovering the foundational substrate truth: Super execution authority derives strictly from Texture <code>execution_request</code> Control packets carrying the canonical <code>operation:</code> URI, rather than unbound <code>producer_report</code> continuation loops.",
        bullet_style
    ))
    
    # 2. Phase 1: Family A Monism & Pre-A Checkpoint Publication
    story.append(Paragraph("2. Phase 1: Family A Monism & Pre-A Checkpoint Publication", h1_style))
    story.append(Paragraph(
        "Prior to Phase 1 completion, Texture document aliases suffered from cross-computer data contamination due to unscoped global alias tables. Under Family A monism:",
        body_style
    ))
    story.append(Paragraph(
        "• All Texture document aliases, event projections, and desktop bindings were strictly scoped to exact <code>computer_id</code> boundaries (<code>ebce5455</code>, <code>0aa1ffb3</code>, <code>087cf290</code>).",
        bullet_style
    ))
    story.append(Paragraph(
        "• Schema migrations were established to ensure <code>computer_id</code> columns and primary key indices are applied in deterministic order (<code>997f25cb</code>), eliminating boot-time crash risks.",
        bullet_style
    ))
    story.append(Paragraph(
        "• Database write deadlines were expanded on both proxy and guest endpoints (<code>0d8b8920</code>, <code>b26ce208</code>) to accommodate full residue replay and snapshot exports without 504 timeouts.",
        bullet_style
    ))
    story.append(Paragraph(
        "At realization epoch 324 on staging computer <code>computer-03335285269bdba4f94377e56879f9e6</code>, pre-A checkpoint <code>99949fe2e16d3c4c446838c0e59517b108accecab7afefd9329c3a6c4a1209f7</code> was successfully committed, verified, and published (<code>5b7a7b73</code>). Mode was confirmed locked in <code>propose_only</code> generation 1 (ModeReceipt <code>01a0091b-bf12-771e-97e7-9a42752ad036</code>), ensuring all live send, mail, and materialization effects remain strictly OFF until candidate evaluation passes.",
        body_style
    ))

    # 3. Candidate A Solitaire Formulation
    story.append(Paragraph("3. Candidate A Solitaire Formulation & Decision Policy", h1_style))
    story.append(Paragraph(
        "Per <code>74e1f51c</code>, the candidate change A specification and decision-policy manifest were authored:",
        body_style
    ))
    story.append(Paragraph(
        "• <b>Product Object:</b> Classic Solitaire game engine added to the user computer's local application surface.<br/>"
        "• <b>Intentional Foundation Defect:</b> Pre-declared foundation defect (invalid card-drag state machine rule on foundation piles) designed to be detected and falsified in Phase 3.<br/>"
        "• <b>Five Capsule-Bound Bundle Artifacts:</b> 1. <code>source_patch_ref</code> (Solitaire engine), 2. <code>test_patch_ref</code> (unit tests), 3. <code>verifier_manifest_ref</code> (execution receipt), 4. <code>doc_patch_ref</code> (documentation), 5. <code>changelog_patch_ref</code> (changelog entry).<br/>"
        "• <b>Autonomy Boundary:</b> Multi-agent qualified consensus operating under frozen reversible decision policy <code>reversible-selfdev-v1</code>.",
        body_style
    ))

    # 4. Capsule Broker Execution Substrate
    story.append(Paragraph("4. Capsule Broker Execution Substrate Repair", h1_style))
    story.append(Paragraph(
        "During live candidate authoring probe on staging, CoSuper <code>assignment-97191e37</code> crashed immediately upon attempting bash execution inside guest capsule <code>capsule-c5e35066</code>. The capsule broker aborted with: <code>bash: cannot set terminal process group (-1): Inappropriate ioctl for device / bash: no job control in this shell</code>.<br/>"
        "<b>Resolution:</b> In commit <code>651d86bc</code> (deployed as <code>d33f245c</code> at epoch 326), the capsule broker was refactored to execute non-interactive commands via <code>sh -c</code> with clean environment arguments, eliminating job-control initialization requirements. Live re-probe confirmed CoSuper executed 200 clean tool iterations without process group faults.",
        body_style
    ))

    # 5. The Super Actor Continuation Ladder
    story.append(Paragraph("5. The Super Actor Continuation Ladder (5-Stage Discovery)", h1_style))
    story.append(Paragraph(
        "When CoSuper <code>assignment-97191e37</code> exhausted its 200-iteration budget without freezing candidate A, the system initiated an intensive series of platform investigations that revealed how the privileged Super actor must be driven:",
        body_style
    ))
    
    ladder_data = [
        [
            Paragraph("Stage", table_header),
            Paragraph("Problem / Symptom", table_header),
            Paragraph("Substrate Finding & Fix", table_header),
            Paragraph("Commit / Epoch", table_header)
        ],
        [
            Paragraph("<b>Stage 1</b>", table_cell_bold),
            Paragraph("Super stayed completed after CoSuper cancel; no rewake.", table_cell),
            Paragraph("Woke Super actor on CoSuper cancel <code>producer_report</code>.", table_cell),
            Paragraph("<code>9bc99f90</code><br/>Epoch 327", table_cell)
        ],
        [
            Paragraph("<b>Stage 2</b>", table_cell_bold),
            Paragraph("10-minute continuation storm (repeating Super loop).", table_cell),
            Paragraph("Claimed <code>producer_report_ids</code> in Super metadata to prevent restorm.", table_cell),
            Paragraph("<code>3654d925</code><br/>Epoch 328", table_cell)
        ],
        [
            Paragraph("<b>Stage 3</b>", table_cell_bold),
            Paragraph("Super 200-looped on generic inbox prompt.", table_cell),
            Paragraph("Specialized prompt to <code>assign_co_super</code> + stamped replacement flag.", table_cell),
            Paragraph("<code>5e01ac3a</code><br/>Epoch 329", table_cell)
        ],
        [
            Paragraph("<b>Stage 4</b>", table_cell_bold),
            Paragraph("Replacement Super failed at iter 7 on <code>max_tokens</code>.", table_cell),
            Paragraph("Omitted claimed cancel report bodies (<code>cosuper_replacement_omit_reports</code>).", table_cell),
            Paragraph("<code>9a55b756</code><br/>Epoch 330", table_cell)
        ],
        [
            Paragraph("<b>Stage 5 (Root)</b>", table_cell_bold),
            Paragraph("Omit-reports Super <code>f515dd0f</code> still 200-failed without assign.", table_cell),
            Paragraph("<b>Root cause:</b> Report continuation lacks Texture Control packets and operation ID. Mint Texture <code>execution_request</code> rewake.", table_cell),
            Paragraph("<b>In Progress</b><br/>Epoch 331", table_cell)
        ]
    ]
    ladder_table = Table(ladder_data, colWidths=[55, 145, 230, 74])
    ladder_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), c_primary),
        ('BOX', (0,0), (-1,-1), 0.75, c_border),
        ('INNERGRID', (0,0), (-1,-1), 0.5, colors.HexColor("#E2E8F0")),
        ('TOPPADDING', (0,0), (-1,-1), 4),
        ('BOTTOMPADDING', (0,0), (-1,-1), 4),
        ('LEFTPADDING', (0,0), (-1,-1), 5),
        ('RIGHTPADDING', (0,0), (-1,-1), 5),
        ('ROWBACKGROUNDS', (0,1), (-1,-1), [colors.white, c_bg_light])
    ]))
    story.append(ladder_table)
    story.append(Spacer(1, 6))

    story.append(Paragraph(
        "<b>The Root Cause Insight:</b> Line-level audit of <code>super_controller.go:201-203</code> revealed that <code>self_development_operation_id</code> and <code>lifecycle_control_bindings</code> are ONLY established when Super is started from a Texture Control packet (<code>request_source=lifecycle_texture_control</code>) whose <code>Sources[0].Target.URI</code> carries <code>operation:selfdev-...</code>. Report-continuation Super lacked these bindings entirely, running as an unbound loop with no operation context. The only Super that successfully called <code>assign_co_super</code> was <code>f009f383</code>, started from a canonical Texture <code>execution_request</code>. When CoSuper cancels, the system must trigger <code>ensureSelfDevelopmentTextureJoin</code> with <code>wakeToken = terminalSuper.RunID</code>, minting <code>turn:selfdev-texture-rewake</code>.",
        body_style
    ))

    # 6. CI Substrate Robustness
    story.append(Paragraph("6. CI Substrate Robustness & Flake Triage", h1_style))
    story.append(Paragraph(
        "During deployment of <code>3654d925</code>, two distinct CI substrate failure modes were isolated:<br/>"
        "• <b>Heresy Detector ripgrep Stall (CI 32293816782):</b> Hung for 66+ min on <code>apt-get install ripgrep</code> due to upstream repository stall. Resolved via cancellation and forced workflow dispatch.<br/>"
        "• <b>Dolt Race-Shard Timeout Flake (CI 32299981780):</b> Shard 3 Dolt scan timeout in <code>TestCancelRunTrajectoryDrainsMoreThanOneActivePage</code>. Re-dispatch passed cleanly, confirming test runner contention rather than a code regression.",
        body_style
    ))

    # 7. Agentic Consensus Panel Findings & Dissent
    story.append(Paragraph("7. Agentic Consensus Panel Findings & Adjudicated Improvements", h1_style))
    story.append(Paragraph(
        "An independent consensus panel was convened across four distinct agent architectures: Claude (Opus tier), Gemini 3.7 Flash, Devin, and Codex.",
        body_style
    ))
    story.append(Paragraph(
        "• <b>Unanimous Architectural Verdict:</b> All agents approved minting canonical Texture <code>execution_request</code> rewake via <code>ensureSelfDevelopmentTextureJoin</code> as the correct substrate-level resolution.<br/>"
        "• <b>Confirmation of Root Cause:</b> Confirmed that <code>self_development_operation_id</code> is strictly parsed from <code>Sources[0].Target.URI</code>, explaining why report-continuation Super was unbound.<br/>"
        "• <b>Claude Opus Adjudicated Improvements:</b><br/>"
        "  - <b>B1 (Delete Fallback):</b> Do not keep report-continuation as a silent fallback when Texture rewake is available.<br/>"
        "  - <b>B2 (Bound Rewake Cycle):</b> Add an attempt counter (capped at 3-5 attempts) to prevent runaway rewake loops.<br/>"
        "  - <b>B3 (Prompt Recovery):</b> Retrieve original prompt from <code>Operation.PromptArtifactRef</code> rather than relying solely on run history scans.<br/>"
        "  - <b>H4 (Skip When Blocked):</b> Ensure rewake skips if Super is currently <code>RunBlocked</code>.<br/>"
        "  - <b>H5 (Control Dedup):</b> Check if an unconsumed <code>execution_request</code> is already pending before minting another turn.",
        body_style
    ))

    # 8. Strategic Roadmap & Next Actions
    story.append(Paragraph("8. Strategic Roadmap & Next Actions", h1_style))
    story.append(Paragraph(
        "1. <b>Deploy Texture Rewake Substrate (Epoch 331):</b> Land <code>maybeRewakeSelfDevelopmentTextureAfterTerminalSuper</code> in <code>selfdev_texture_join.go</code> with consensus safeguards (B1, B2, B3, H4, H5). Push to <code>main</code>, deploy Node B, and refresh retained computer to Epoch 331.<br/>"
        "2. <b>Verify Live CoSuper Authorship:</b> Observe Super rewake with bound Texture Control packet and operation ID, confirm turn 1 call to <code>assign_co_super</code>, and monitor CoSuper authoring candidate A inside guest capsule.<br/>"
        "3. <b>Phase 2 & Phase 3 Execution:</b> Freeze candidate A with five bundle artifacts, run qualified consensus under <code>reversible-selfdev-v1</code>, promote candidate A, verify game play via API/DB tests, execute falsification with candidate B, and perform acceptance-fenced restore to pre-A checkpoint <code>99949fe2</code>.",
        body_style
    ))
    
    # Signature box
    story.append(Spacer(1, 10))
    sig_data = [
        [
            Paragraph("<b>Prepared by:</b> Choir Engineering", meta_style),
            Paragraph("<b>Reviewed by:</b> Claude Opus, Gemini 3.7 Flash, Devin, Codex", meta_style),
            Paragraph("<b>Target Fence:</b> Pre-A Checkpoint 99949fe2 (Epoch 324)", meta_style)
        ]
    ]
    sig_table = Table(sig_data, colWidths=[168, 168, 168])
    sig_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,-1), c_callout_bg),
        ('BOX', (0,0), (-1,-1), 0.75, c_callout_border),
        ('TOPPADDING', (0,0), (-1,-1), 4),
        ('BOTTOMPADDING', (0,0), (-1,-1), 4),
        ('LEFTPADDING', (0,0), (-1,-1), 6),
        ('RIGHTPADDING', (0,0), (-1,-1), 6),
    ]))
    story.append(sig_table)

    doc.build(story, canvasmaker=NumberedCanvas)
    print(f"Successfully generated PDF: {filename}")

if __name__ == '__main__':
    out_pdf = "tmp/pdfs/choir-effects-super-rewake-and-self-development-report-2026-08-20.pdf"
    if len(sys.argv) > 1:
        out_pdf = sys.argv[1]
    build_pdf(out_pdf)
