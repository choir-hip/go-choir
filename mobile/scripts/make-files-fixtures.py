#!/usr/bin/env python3
"""Generates the Files lane's demo files (all invented, no third-party content) and embeds them as base64 in
AutoradioKit/Sources/AutoradioKit/FilesDemo.generated.swift, so the app carries them with no resource bundle (Exact
compiles Swift only). The output is tracked so `swift test` works; rerun this script to change a fixture.

  PNG   a test card, drawn here with PIL (no photograph)
  PDF   three pages of invented council minutes, written by hand (Helvetica, no embedded fonts)
  EPUB  "The Lantern Keeper", three short invented chapters, zipped with deflate (and one stored entry)
  WAV   1.5 seconds of a soft 440 Hz tone, 8 kHz, 8 bit
  MP4   2 seconds of a generated test pattern (ffmpeg's lavfi), 160x90, no sound
  CSV, Markdown, text, Swift: small invented files
Needs python3 with PIL, and ffmpeg for the MP4 (the MP4 is skipped, with a note, when ffmpeg is missing)."""
import base64, io, math, os, shutil, struct, subprocess, sys, tempfile, wave, zipfile

HERE = os.path.dirname(os.path.abspath(__file__))
OUT = os.path.join(HERE, "..", "AutoradioKit", "Sources", "AutoradioKit", "FilesDemo.generated.swift")

def png():
    from PIL import Image, ImageDraw
    w, h = 640, 400
    im = Image.new("RGB", (w, h), (24, 38, 58))
    d = ImageDraw.Draw(im)
    for y in range(h):  # a vertical gradient, drawn in bands so the PNG stays small
        t = y / h
        d.line([(0, y), (w, y)], fill=(int(24 + 150 * t), int(38 + 90 * t), int(58 + 40 * (1 - t))))
    bars = [(235, 235, 235), (235, 220, 60), (60, 220, 220), (60, 200, 60), (220, 60, 220), (220, 60, 60), (60, 60, 220)]
    bw = w // len(bars)
    for i, c in enumerate(bars):
        d.rectangle([i * bw, 40, (i + 1) * bw - 1, 140], fill=c)
    d.ellipse([w // 2 - 90, 190, w // 2 + 90, 370], outline=(255, 255, 255), width=4)
    d.line([(w // 2 - 90, 280), (w // 2 + 90, 280)], fill=(255, 255, 255), width=2)
    d.line([(w // 2, 190), (w // 2, 370)], fill=(255, 255, 255), width=2)
    for k in range(0, w, 40):  # a ruler along the foot, so a zoom shows detail
        d.line([(k, h - 24), (k, h - 8)], fill=(255, 255, 255), width=1)
    d.text((16, 12), "HARBOR TEST CARD  640 x 400", fill=(255, 255, 255))
    b = io.BytesIO(); im.quantize(colors=64).save(b, "PNG", optimize=True)
    return b.getvalue()

def pdf():
    pages = [
        ("Harbor Council - minutes", ["Meeting of 4 November, held at the Harbor Hall.", "Present: the chair and six members.",
                                      "1. The minutes of the last meeting were read and approved.",
                                      "2. The tide barrier works are on schedule for the spring.",
                                      "3. The clerk reported two letters, both from residents, both kind."]),
        ("Item 4 - the quay lights", ["The council considered new lamps for the east quay.",
                                      "A member asked that the lamps be warm in colour and shielded from the water.",
                                      "Resolved: the clerk will ask three suppliers for a price and a sample.",
                                      "Resolved: the lamps will be switched on at dusk and off at midnight."]),
        ("Item 5 - any other business", ["The chair thanked the volunteers who painted the railings.",
                                         "The next meeting is on 2 December.",
                                         "The meeting closed at nine o'clock.", "", "(These minutes are invented for the demo.)"]),
    ]
    objs = []
    def add(b): objs.append(b); return len(objs)
    cat = add(b""); pgs = add(b""); font = add(b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
    boldf = add(b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold >>")
    kids = []
    for title, lines in pages:
        def esc(s): return s.replace("\\", "\\\\").replace("(", "\\(").replace(")", "\\)")
        s = "BT /F2 20 Tf 56 720 Td (%s) Tj ET\n" % esc(title)
        y = 680
        for ln in lines:
            s += "BT /F1 13 Tf 56 %d Td (%s) Tj ET\n" % (y, esc(ln)); y -= 24
        c = add(b"<< /Length %d >>\nstream\n%s\nendstream" % (len(s), s.encode()))
        p = add(b"<< /Type /Page /Parent %d 0 R /MediaBox [0 0 612 792] /Contents %d 0 R /Resources << /Font << /F1 %d 0 R /F2 %d 0 R >> >> >>" % (pgs, c, font, boldf))
        kids.append(p)
    objs[cat - 1] = b"<< /Type /Catalog /Pages %d 0 R >>" % pgs
    objs[pgs - 1] = b"<< /Type /Pages /Count %d /Kids [%s] >>" % (len(kids), b" ".join(b"%d 0 R" % k for k in kids))
    info = add(b"<< /Title (Harbor Council minutes) /Producer (Autoradio demo) >>")
    out = b"%PDF-1.4\n"; offs = []
    for i, o in enumerate(objs):
        offs.append(len(out)); out += b"%d 0 obj\n%s\nendobj\n" % (i + 1, o)
    x = len(out)
    out += b"xref\n0 %d\n0000000000 65535 f \n" % (len(objs) + 1)
    for o in offs: out += b"%010d 00000 n \n" % o
    out += b"trailer\n<< /Size %d /Root %d 0 R /Info %d 0 R >>\nstartxref\n%d\n%%%%EOF\n" % (len(objs) + 1, cat, info, x)
    return out

CH = [
 ("The lamp on the point", """<h1>The lamp on the point</h1>
<p>Nobody remembered who had first lit the lamp on the point, only that it had never gone out in the keeper&#8217;s lifetime, and the keeper was old.</p>
<p>Each evening she climbed the forty-one steps with a can of oil and a cloth&nbsp;&mdash; the cloth mattered more than the oil &mdash; and she wiped the glass until the town below it looked a little nearer.</p>
<blockquote><p>A lamp is only a promise, she liked to say, that someone is still awake.</p></blockquote>
<hr/>
<p>On the night this story begins the sea was <em>very</em> quiet, and that worried her.</p>"""),
 ("Three visitors", """<h1>Three visitors</h1>
<p>The first visitor was a boy with a notebook. He wanted to know how many ships the lamp had saved. She told him she did not keep that count.</p>
<h2>What the boy asked</h2>
<ul><li>How old is the lamp?</li><li>Does it ever flicker?</li><li>Who pays for the oil?</li></ul>
<p>The second visitor was a man from the council, who counted the steps himself and left without a word. The third did not knock at all.</p>
<div class="note"><p>It was the sea, which came up the steps one at a time, and sat down on the fortieth.</p></div>"""),
 ("What the sea said", """<h1>What the sea said</h1>
<p>The sea said very little, as the sea does. It looked at the lamp, and the lamp looked back.</p>
<p>&#8220;You could rest,&#8221; said the sea, at last. &#8220;Others would keep it.&#8221;</p>
<p>&#8220;Others would,&#8221; said the keeper. &#8220;That is what I am afraid of, and what I hope for.&#8221;</p>
<p>In the morning the town found the lamp lit, the glass bright, and two cups on the fortieth step. <em>The End.</em></p>"""),
]

def epub():
    b = io.BytesIO()
    z = zipfile.ZipFile(b, "w")
    z.writestr(zipfile.ZipInfo("mimetype"), "application/epub+zip", compress_type=zipfile.ZIP_STORED)
    z.writestr("META-INF/container.xml", """<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>""", compress_type=zipfile.ZIP_DEFLATED)
    items = "".join('<item id="c%d" href="ch%d.xhtml" media-type="application/xhtml+xml"/>' % (i + 1, i + 1) for i in range(len(CH)))
    spine = "".join('<itemref idref="c%d"/>' % (i + 1) for i in range(len(CH)))
    z.writestr("OEBPS/content.opf", """<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="id">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="id">urn:uuid:00000000-0000-4000-8000-00000000a1a1</dc:identifier>
    <dc:title>The Lantern Keeper</dc:title><dc:creator>A. Ferris (invented)</dc:creator><dc:language>en</dc:language>
  </metadata>
  <manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>%s</manifest>
  <spine><itemref idref="nav" linear="no"/>%s</spine>
</package>""" % (items, spine), compress_type=zipfile.ZIP_DEFLATED)
    z.writestr("OEBPS/nav.xhtml", """<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><head><title>Contents</title></head>
<body><nav epub:type="toc"><ol>%s</ol></nav></body></html>""" % "".join('<li><a href="ch%d.xhtml">%s</a></li>' % (i + 1, t) for i, (t, _) in enumerate(CH)), compress_type=zipfile.ZIP_DEFLATED)
    for i, (t, body) in enumerate(CH):
        z.writestr("OEBPS/ch%d.xhtml" % (i + 1), """<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml"><head><title>%s</title></head><body>%s</body></html>""" % (t, body), compress_type=zipfile.ZIP_DEFLATED)
    z.close()
    return b.getvalue()

def wav():
    b = io.BytesIO()
    w = wave.open(b, "wb"); w.setnchannels(1); w.setsampwidth(1); w.setframerate(8000)
    n = int(8000 * 1.5); frames = bytearray()
    for i in range(n):
        env = min(1.0, i / 400, (n - i) / 800)
        frames.append(int(128 + 70 * env * math.sin(2 * math.pi * 440 * i / 8000)))
    w.writeframes(bytes(frames)); w.close()
    return b.getvalue()

def mp4():
    if not shutil.which("ffmpeg"): print("note: no ffmpeg, skipping the MP4", file=sys.stderr); return None
    d = tempfile.mkdtemp(); p = os.path.join(d, "v.mp4")
    r = subprocess.run(["ffmpeg", "-v", "error", "-y", "-f", "lavfi", "-i", "testsrc=size=160x90:rate=10", "-t", "2", "-c:v", "libx264",
                        "-preset", "veryslow", "-crf", "38", "-pix_fmt", "yuv420p", "-movflags", "+faststart", "-an", p])
    if r.returncode != 0: print("note: ffmpeg failed, skipping the MP4", file=sys.stderr); return None
    data = open(p, "rb").read(); shutil.rmtree(d); return data

CSV = '''Quarter,Item,Planned,Actual,Note
Q1,Quay lamps,1200,1100,"Warm white, shielded"
Q1,Railings,400,455,"Paint, two coats"
Q2,Tide barrier survey,5200,5100,
Q2,"Benches, east side",900,870,"Oak, ""harbour grade"""
Q3,Hall roof,7600,8020,Over by the slates
Q4,Volunteer tea,150,149,A good year
'''
MD = """# Harbor notes

A few lines kept by the clerk, **invented** for the demo.

## This week

- Ask three suppliers for a lamp price and a sample
- Check the *east quay* railings after the rain
- Send the minutes to the members

> The sea is quiet; that is when to look at the wall.

## A small table of tides

| Day | High | Low |
|---|---|---|
| Mon | 06:12 | 12:30 |
| Tue | 06:58 | 13:14 |

```swift
let lamps = suppliers.map { $0.price }.min()
```
"""
TXT = """Harbor Hall - key list

East door ........ brass, on the hook by the stove
Store room ....... iron, the clerk has it
Boat shed ........ two keys, one is spare and lives under the third stair

If you read this on a phone, the file is plain text and has no formatting at all.
"""
CODE = """import Foundation

/// Sums the quay lamp invoices that were paid this year.
func totalPaid(_ invoices: [Invoice]) -> Double {
    invoices.filter { $0.paid }.reduce(0) { $0 + $1.amount }
}

struct Invoice { let amount: Double; let paid: Bool }
"""
BIN = bytes(range(256)) * 2   # not a type anything previews

def main():
    files = {
        "Books/The Lantern Keeper.epub": epub(), "Documents/Council minutes.pdf": pdf(), "Documents/Budget 2026.csv": CSV.encode(),
        "Documents/Harbor notes.md": MD.encode(), "Documents/Key list.txt": TXT.encode(), "Documents/Lamps.swift": CODE.encode(),
        "Documents/Survey readings.dat": BIN, "Pictures/Harbor test card.png": png(), "Media/Tone 440.wav": wav(),
    }
    v = mp4()
    if v: files["Media/Pattern.mp4"] = v
    lines = ["// Generated by scripts/make-files-fixtures.py. Do not edit. Invented demo files, base64.", "import Foundation", "",
             "public enum FilesDemoData {", "    /// Demo path (no leading slash) -> base64 of the file's bytes.", "    public static let base64: [String: String] = ["]
    for k in sorted(files):
        enc = base64.b64encode(files[k]).decode()
        chunks = [enc[i:i + 100] for i in range(0, len(enc), 100)]
        lines.append('        "%s": #"""\n%s\n"""#,' % (k, "\n".join(chunks)))
    lines += ["    ]", "}", ""]
    open(OUT, "w").write("\n".join(lines))
    print("wrote", os.path.relpath(OUT), {k: len(v) for k, v in files.items()})

if __name__ == "__main__":
    main()
