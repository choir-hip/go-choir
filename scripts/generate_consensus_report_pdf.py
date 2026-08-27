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
        self.setFont("Helvetica", 7.5)
        self.setFillColor(colors.HexColor("#64748B"))
        
        # Header (pages > 1)
        if self._pageNumber > 1:
            self.drawString(45, 755, "CHOIR PLATFORM — WHOLE-SYSTEM STATE & CONSENSUS REVIEW")
            self.drawRightString(612 - 45, 755, "2026-08-27")
            self.setStrokeColor(colors.HexColor("#CBD5E1"))
            self.setLineWidth(0.5)
            self.line(45, 749, 612 - 45, 749)

        # Footer (all pages)
        self.setStrokeColor(colors.HexColor("#CBD5E1"))
        self.setLineWidth(0.5)
        self.line(45, 40, 612 - 45, 40)
        self.drawString(45, 28, "CONFIDENTIAL — FOR INTERNAL PLATFORM ENGINEERING & ARCHITECTURE REVIEW")
        self.drawRightString(612 - 45, 28, f"Page {self._pageNumber} of {page_count}")
        self.restoreState()

def build_pdf(filename):
    os.makedirs(os.path.dirname(os.path.abspath(filename)), exist_ok=True)
    doc = SimpleDocTemplate(
        filename,
        pagesize=letter,
        leftMargin=45,
        rightMargin=45,
        topMargin=45,
        bottomMargin=45
    )

    styles = getSampleStyleSheet()
    
    primary_color = colors.HexColor("#0F172A")    # Slate 900
    accent_color = colors.HexColor("#2563EB")     # Blue 600
    teal_color = colors.HexColor("#0D9488")       # Teal 600
    text_color = colors.HexColor("#1E293B")       # Slate 800
    muted_color = colors.HexColor("#475569")      # Slate 600
    bg_light = colors.HexColor("#F8FAFC")         # Slate 50

    title_style = ParagraphStyle(
        'DocTitle',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=18,
        leading=22,
        textColor=primary_color,
        spaceAfter=2
    )
    
    subtitle_style = ParagraphStyle(
        'DocSubTitle',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=10,
        leading=13,
        textColor=accent_color,
        spaceAfter=6
    )

    meta_style = ParagraphStyle(
        'MetaText',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=7.5,
        leading=10.5,
        textColor=muted_color
    )

    h1_style = ParagraphStyle(
        'Heading1_Custom',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=11,
        leading=14,
        textColor=primary_color,
        spaceBefore=8,
        spaceAfter=4,
        keepWithNext=True
    )

    h2_style = ParagraphStyle(
        'Heading2_Custom',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=9,
        leading=12,
        textColor=accent_color,
        spaceBefore=5,
        spaceAfter=2,
        keepWithNext=True
    )

    body_style = ParagraphStyle(
        'Body_Custom',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=8,
        leading=11,
        textColor=text_color,
        spaceAfter=4
    )

    bullet_style = ParagraphStyle(
        'Bullet_Custom',
        parent=body_style,
        leftIndent=10,
        firstLineIndent=-6,
        spaceAfter=3
    )

    table_text = ParagraphStyle(
        'TableText',
        parent=styles['Normal'],
        fontName='Helvetica',
        fontSize=7.5,
        leading=10,
        textColor=text_color
    )

    table_header = ParagraphStyle(
        'TableHeader',
        parent=styles['Normal'],
        fontName='Helvetica-Bold',
        fontSize=8,
        leading=10.5,
        textColor=colors.white
    )

    story = []

    # Title Banner
    story.append(Paragraph("CHOIR WHOLE-SYSTEM ARCHITECTURE REVIEW", title_style))
    story.append(Paragraph("Multi-Model Agentic Consensus on Substrate Overhauls, Genesis Derivability, and Private Go Actor Kernel", subtitle_style))
    story.append(HRFlowable(width="100%", thickness=1.2, color=accent_color, spaceBefore=0, spaceAfter=5))

    # Metadata Block Table
    meta_data = [
        [
            Paragraph("<b>Date:</b> 2026-08-27<br/><b>Review Type:</b> Agentic Consensus", meta_style),
            Paragraph("<b>Staging Host:</b> https://choir.news (Node B)<br/><b>Deployed Commit:</b> 53f80af4 (CI 33046393239)", meta_style),
            Paragraph("<b>Authority Status:</b> doccheck live PASS (0 errors)<br/><b>Panel:</b> Codex, Cursor, Gemini 3.7, Devin, Opencode", meta_style),
        ]
    ]
    meta_table = Table(meta_data, colWidths=[174, 174, 174])
    meta_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,-1), bg_light),
        ('BOX', (0,0), (-1,-1), 0.5, colors.HexColor("#CBD5E1")),
        ('TOPPADDING', (0,0), (-1,-1), 4),
        ('BOTTOMPADDING', (0,0), (-1,-1), 4),
        ('LEFTPADDING', (0,0), (-1,-1), 6),
        ('RIGHTPADDING', (0,0), (-1,-1), 6),
    ]))
    story.append(meta_table)
    story.append(Spacer(1, 6))

    # Executive Verdict Callout
    verdict_text = Paragraph(
        "<b>CONSENSUS VERDICT: SUBSTRATE SOUND & READY FOR STAGING ACTIVATIONS (94% CONFIDENCE)</b><br/>"
        "The Choir platform has successfully verified and deployed the Substrate Overhauls (Tracks K, F, M, Assurance), "
        "Genesis Surface Derivability (Asset 404 Fail-Closed + Immutable Nix Baseline Fallback), and the Private Go Actor Kernel "
        "(Yaegi + Disposable Guest-Local Capsule Broker). The platform is ready for live, sealed CoSuper activations on staging. "
        "Unrestricted effect-bearing candidate authoring should remain gated until FIFO scheduling by <code>ArrivalOrdinal</code> and pre-execution Bash secret scanning land.",
        body_style
    )
    verdict_table = Table([[verdict_text]], colWidths=[522])
    verdict_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,-1), colors.HexColor("#EFF6FF")),
        ('BOX', (0,0), (-1,-1), 0.75, colors.HexColor("#3B82F6")),
        ('LEFTPADDING', (0,0), (-1,-1), 8),
        ('RIGHTPADDING', (0,0), (-1,-1), 8),
        ('TOPPADDING', (0,0), (-1,-1), 5),
        ('BOTTOMPADDING', (0,0), (-1,-1), 5),
    ]))
    story.append(verdict_table)
    story.append(Spacer(1, 6))

    # Section 1: Verified Subsystem Accomplishments
    story.append(Paragraph("1. Verified Architectural Accomplishments", h1_style))
    
    story.append(Paragraph("<b>A. Genesis Computer Surface & Staged SPA Derivability</b>", h2_style))
    story.append(Paragraph(
        "• <b>Asset 404 Fail-Closed:</b> Static asset requests (<code>/assets/*</code>) return HTTP 404 with <code>Cache-Control: no-store</code> when missing rather than falling back to <code>index.html</code>, eliminating MIME-type mismatch CSS preload errors on mobile Safari.<br/>"
        "• <b>Route-Decoupled Baseline Import & Fallback:</b> Fresh microVMs stage <code>/mnt/persistent/choir-updater/current/frontend</code> immediately on boot and fall back to the immutable Nix store closure (<code>CHOIR_BASELINE_RELEASE_ROOT/frontend</code>) if uninitialized.<br/>"
        "• <b>Live Staging Proof:</b> Playwright automation verified passkey registration, desktop launch, Settings dynamic chunk loading, and reload 200 OK with zero 503 underivable SPA errors on <code>https://choir.news</code> (commit <code>53f80af4</code>).",
        bullet_style
    ))

    story.append(Paragraph("<b>B. Private Go Actor Kernel (Yaegi) & Capsule Broker</b>", h2_style))
    story.append(Paragraph(
        "• <b>Process-per-Activation Sidecar:</b> Disposable guest-local capsules isolate the Yaegi interpreter. CoSuper receives Go + direct Bash; Researcher receives Go-only (refusing exec/write).<br/>"
        "• <b>Adversarial Security Matrix:</b> Server-side package allowlist, AST import check, obligation revalidation before execution, every-outcome attempt receipts, broker RPC serialization, and 2 MiB output caps.<br/>"
        "• <b>Deterministic Transclusion & Salient Receipts:</b> Host-owned salient receipts (refusals, failures, activation deaths, rewarms) cannot be curated away before Texture transclusion.<br/>"
        "• <b>Cross-Model Continuity:</b> Monotonic epoch rewarm preserves open obligations and assignments across model switches without relying on volatile in-memory interpreter heaps.",
        bullet_style
    ))

    story.append(Paragraph("<b>C. Durable Substrate Overhauls (Tracks K, F, M, Assurance)</b>", h2_style))
    story.append(Paragraph(
        "• <b>Track K (Key Escrow):</b> X25519 ECDH + HKDF + XChaCha20-Poly1305 custodian wrapping enforces a 2-of-N quorum platform store gate.<br/>"
        "• <b>Track F (File-CAS):</b> Encrypted 4 MiB chunked CAS with canonical Merkle manifests and tape-committed roots (<code>file_root_committed</code>).<br/>"
        "• <b>Track M & Assurance:</b> Host fsync'd MTA spool (<code>internal/maild</code>) and automated daily restore drill runner / blob scrubber (<code>internal/recovery</code>).",
        bullet_style
    ))

    story.append(Spacer(1, 4))

    # Page Break to Page 2
    story.append(PageBreak())

    # Section 2: Verification Matrix (Top of Page 2)
    story.append(Paragraph("2. Subsystem Verification Matrix", h1_style))
    
    matrix_data = [
        [
            Paragraph("Subsystem", table_header),
            Paragraph("Scope / Mechanism", table_header),
            Paragraph("Evidence Reference", table_header),
            Paragraph("Verdict", table_header)
        ],
        [
            Paragraph("<b>Genesis Surface</b>", table_text),
            Paragraph("Mobile Safari CSS Preload + 503 Reload", table_text),
            Paragraph("<code>frontend/tests/genesis-settings-staging.spec.js</code>", table_text),
            Paragraph("<font color='#0D9488'><b>PASS (12.8s)</b></font>", table_text)
        ],
        [
            Paragraph("<b>Yaegi Kernel</b>", table_text),
            Paragraph("AST Allowlist, Handles, Sidecar Execution", table_text),
            Paragraph("<code>internal/yaegikernel/...</code> (36 unit tests)", table_text),
            Paragraph("<font color='#0D9488'><b>PASS</b></font>", table_text)
        ],
        [
            Paragraph("<b>Capsule Broker</b>", table_text),
            Paragraph("Role Verbs, Landlock, Seccomp, Capabilities", table_text),
            Paragraph("<code>internal/capsule/...</code> (3 test suites)", table_text),
            Paragraph("<font color='#0D9488'><b>PASS</b></font>", table_text)
        ],
        [
            Paragraph("<b>Cross-Compilation</b>", table_text),
            Paragraph("Linux ARM64 Architecture Build", table_text),
            Paragraph("<code>CGO_ENABLED=0 GOOS=linux GOARCH=arm64</code>", table_text),
            Paragraph("<font color='#0D9488'><b>PASS</b></font>", table_text)
        ],
        [
            Paragraph("<b>Transclusion</b>", table_text),
            Paragraph("Deterministic Formatting & Non-Omissibility", table_text),
            Paragraph("<code>internal/yaegikernel/transclusion_test.go</code>", table_text),
            Paragraph("<font color='#0D9488'><b>PASS</b></font>", table_text)
        ],
        [
            Paragraph("<b>Key Escrow</b>", table_text),
            Paragraph("Custodian Wrapping & 2-of-N Quorum Gate", table_text),
            Paragraph("<code>internal/keyescrow/...</code>", table_text),
            Paragraph("<font color='#0D9488'><b>PASS</b></font>", table_text)
        ],
        [
            Paragraph("<b>File-CAS</b>", table_text),
            Paragraph("4MiB Chunks, Merkle Tree & Sync Barrier", table_text),
            Paragraph("<code>internal/filecas/...</code>", table_text),
            Paragraph("<font color='#0D9488'><b>PASS</b></font>", table_text)
        ],
        [
            Paragraph("<b>Mail Spool</b>", table_text),
            Paragraph("Host MTA Spool, LMTP Drain, Guest Maildir", table_text),
            Paragraph("<code>internal/maild/...</code>", table_text),
            Paragraph("<font color='#0D9488'><b>PASS</b></font>", table_text)
        ],
        [
            Paragraph("<b>Recovery Drills</b>", table_text),
            Paragraph("Recovery Capsules & Background Scrubbing", table_text),
            Paragraph("<code>internal/recovery/...</code>", table_text),
            Paragraph("<font color='#0D9488'><b>PASS</b></font>", table_text)
        ],
        [
            Paragraph("<b>Doc Authority</b>", table_text),
            Paragraph("Reading Packet & Authority Manifests", table_text),
            Paragraph("<code>cmd/doccheck -mode live</code>", table_text),
            Paragraph("<font color='#0D9488'><b>PASS (6.2s)</b></font>", table_text)
        ],
    ]

    v_table = Table(matrix_data, colWidths=[90, 182, 185, 65])
    v_table.setStyle(TableStyle([
        ('BACKGROUND', (0,0), (-1,0), primary_color),
        ('BOX', (0,0), (-1,-1), 0.5, colors.HexColor("#CBD5E1")),
        ('GRID', (0,0), (-1,-1), 0.5, colors.HexColor("#E2E8F0")),
        ('TOPPADDING', (0,0), (-1,-1), 2.5),
        ('BOTTOMPADDING', (0,0), (-1,-1), 2.5),
        ('LEFTPADDING', (0,0), (-1,-1), 4),
        ('RIGHTPADDING', (0,0), (-1,-1), 4),
        ('ROWBACKGROUNDS', (0,1), (-1,-1), [colors.white, bg_light]),
    ]))
    story.append(v_table)
    story.append(Spacer(1, 6))

    # Section 3: Key Hardening Opportunities
    story.append(Paragraph("3. High-Leverage Hardening Opportunities Identified by Panel", h1_style))
    
    findings = [
        ("1. Cross-Trajectory Scheduling Selection by ArrivalOrdinal",
         "<b>Observation:</b> <code>packet.ArrivalOrdinal</code> is stamped at Texture mailbox entry (<code>internal/store/texture_turn.go</code>), but Super control updates are sorted by <code>ReducerSeq</code> (<code>internal/store/lifecycle.go</code>).<br/>"
         "<b>Risk:</b> Competing execution requests from different trajectories could be scheduled out of order, recreating assignment-supersession edge cases.<br/>"
         "<b>Remediation:</b> Prioritize <code>ArrivalOrdinal</code> over <code>ReducerSeq</code> in pending Super control selection."),
        
        ("2. Pre-Execution Secret Scanning & Output Capping on Bash Path",
         "<b>Observation:</b> <code>capsule_go_eval</code> inspects secrets and caps buffers to 2 MiB before execution. <code>capsule_exec</code> (Bash) inspects secrets post-execution without buffer caps.<br/>"
         "<b>Risk:</b> A rejected secret-bearing shell command could execute before refusal, or unboundedly large command output could cause memory exhaustion.<br/>"
         "<b>Remediation:</b> Move secret checking before command dispatch in <code>Executor.Exec</code> / broker, emit immediate refusal receipts, and cap Bash streams to 2 MiB."),
        
        ("3. Spool Purge Bound to Manifest Inclusion",
         "<b>Observation:</b> <code>internal/maild/spool.go</code> purges delivered messages based on wall-clock timestamps rather than Merkle manifest inclusion.<br/>"
         "<b>Risk:</b> Rapid concurrent deliveries could be purged before a file tree walk includes them in a committed root.<br/>"
         "<b>Remediation:</b> Bind host spool deletion to explicit verification of inclusion in a committed <code>file_root_committed</code> Merkle manifest.")
    ]

    for title, desc in findings:
        story.append(Paragraph(f"<b>{title}</b>", h2_style))
        story.append(Paragraph(desc, bullet_style))

    story.append(Spacer(1, 6))

    # Section 4: Strategic Roadmap
    story.append(Paragraph("4. Prioritized Roadmap & Execution Plan", h1_style))
    story.append(Paragraph(
        "<b>Phase 1: Immediate Substrate Hardening (Sprint 1)</b><br/>"
        "• Implement <code>ArrivalOrdinal</code> FIFO sorting in Super control selection.<br/>"
        "• Move Bash secret scanning to pre-dispatch and apply 2 MiB stream caps.<br/>"
        "• Streamline <code>docs/ACTIVE.md</code> registry footer text.<br/>"
        "<b>Phase 2: Supervised Staging Activations (Sprint 2)</b><br/>"
        "• Provision a fresh staging microVM on <code>https://choir.news</code>.<br/>"
        "• Execute live CoSuper activation testing with model-authored Go scripts, direct Bash execution, and multi-actor typed update exchanges.<br/>"
        "• Test activation forced-death and monotonic epoch rewarm under injected process kills.<br/>"
        "<b>Phase 3: Self-Development & Effect-Bearing Candidate Proof (Sprint 3)</b><br/>"
        "• Re-open <code>docs/definitions/choir-scheduling-and-candidate-proof-2026-08-21.md</code> on the validated staging computer.<br/>"
        "• Author, verify, freeze, and promote candidate self-development bundles using the hardened Private Go Actor Kernel.",
        body_style
    ))

    # Build document
    doc.build(story, canvasmaker=NumberedCanvas)
    print(f"PDF successfully generated at: {filename}")

if __name__ == "__main__":
    out_path = sys.argv[1] if len(sys.argv) > 1 else "output.pdf"
    build_pdf(out_path)
