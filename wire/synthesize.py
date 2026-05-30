"""
Choir Global Wire — Synthesis Engine
Pulls the latest signals from the DB and synthesizes a 5-story issue via LLM.
Run standalone or called from the cycle loop.
"""
import sqlite3
import time
from datetime import datetime
from openai import OpenAI

SOURCE_LABELS = {
    "bbc_world": "BBC World (EN)",
    "aljazeera": "Al Jazeera (EN/AR)",
    "france24_en": "France 24 (EN)",
    "dw_world": "Deutsche Welle (EN)",
    "rfi_en": "RFI English",
    "nikkei": "Nikkei Asia (EN/JA)",
    "scmp": "SCMP (EN, Hong Kong)",
    "the_hindu": "The Hindu (EN, India)",
    "straits_times": "Straits Times (EN, Singapore)",
    "vietnam_news": "Vietnam News (EN/VI)",
    "bangkok_post": "Bangkok Post (EN, Thailand)",
    "dawn_pk": "Dawn (EN/UR, Pakistan)",
    "arab_news": "Arab News (EN/AR)",
    "middle_east_eye": "Middle East Eye (EN)",
    "al_monitor": "Al-Monitor (EN/AR/HE/FA)",
    "the_africa_report": "The Africa Report (EN/FR)",
    "premium_times_ng": "Premium Times Nigeria (EN)",
    "nation_africa": "Nation Africa (EN, Kenya)",
    "rfi_afrique": "RFI Afrique (FR)",
    "mercopress": "MercoPress (EN, South America)",
    "folha_sp": "Folha de S.Paulo (PT, Brazil)",
    "la_nacion_ar": "La Nación (ES, Argentina)",
    "the_verge": "The Verge (EN)",
    "techcrunch": "TechCrunch (EN)",
    "mit_tech_review": "MIT Tech Review (EN)",
    "wired": "Wired (EN)",
    "rest_of_world": "Rest of World (EN, Global Tech)",
    "stat_news": "STAT News (EN, Health)",
    "ft_world": "Financial Times (EN)",
    "economist": "The Economist (EN)",
    "icg_alerts": "ICG Crisis Alerts (EN)",
    "bellingcat": "Bellingcat (EN, OSINT)",
    "aeon": "Aeon Magazine (EN)",
    "the_baffler": "The Baffler (EN)",
    "conflict_monitor": "Conflict Monitor (Telegram)",
}

def main():
    today = datetime.now().strftime("%B %d, %Y")
    print(f"Synthesis starting — {today}")

    conn = sqlite3.connect("vanguard.db")
    cursor = conn.cursor()
    # Get the most recent 50 items, prioritising non-English sources
    cursor.execute("""
        SELECT title, body, source_id, url
        FROM items
        ORDER BY fetched_at DESC
        LIMIT 50
    """)
    rows = cursor.fetchall()
    conn.close()

    if not rows:
        print("No items in database. Run ingestion first.")
        return

    input_data = ""
    for i, (title, body, source_id, url) in enumerate(rows):
        label = SOURCE_LABELS.get(source_id, source_id)
        body_clean = (body or "").replace("<p>", "").replace("</p>", " ").replace("<b>", "").replace("</b>", "")[:400]
        input_data += f"[S{i+1}] {label}\nTitle: {title}\nURL: {url}\nBody: {body_clean}\n\n"

    prompt = f"""Today is {today}. You are the lead editor of the Choir Global Wire — the Automatic Newspaper of Record for Planet Earth.

Your mission: synthesize the raw signals below into a high-fidelity, deeply reported news issue. This is original journalism, not a summary.

REQUIREMENTS:
1. Write EXACTLY 5 distinct stories. Each story must be 400-600 words.
2. Each story must have a sharp, specific headline — no generic titles.
3. Cite sources inline using [S1], [S2] etc. Each story needs at least 2 citations.
4. PRIORITIZE multilingual and non-Western signals. What does the Pakistani, Brazilian, Nigerian, or French-African press say that the English-language wire misses? Name the source language when relevant.
5. Cover a DIVERSE range of verticals: geopolitics, technology/AI, economics, public health, culture. Do not write 5 geopolitics stories.
6. Structure each story with these exact Markdown headers:
   ## [Headline]
   **The Signal:** (1 sentence — what just happened, with today's date {today})
   **The Context:** (2-3 paragraphs of deep analysis, cross-regional perspective)
   **The Contested Ground:** (who disagrees and why — include non-Western perspectives)
   **What to Watch:** (1-2 specific, concrete things to track in the next 24-72 hours)
   *Sources: [S1], [S2]*
7. End with a brief **Editor's Note** (2-3 sentences) on the multilingual signal quality of this issue — which languages and regions provided the most distinctive insights.

RAW SIGNALS ({len(rows)} items from {len(set(r[2] for r in rows))} sources):
{input_data}"""

    client = OpenAI()
    print(f"Calling LLM with {len(rows)} signals from {len(set(r[2] for r in rows))} sources...")

    response = client.chat.completions.create(
        model="gpt-4.1-mini",
        messages=[
            {"role": "system", "content": f"You are the Choir Global Wire synthesis engine. Today is {today}. Write original, deeply reported journalism with precise citations. Never use placeholder dates."},
            {"role": "user", "content": prompt}
        ],
        max_tokens=4500
    )

    issue_content = response.choices[0].message.content
    tokens_used = response.usage.total_tokens

    # Save to database
    conn = sqlite3.connect("vanguard.db")
    cursor = conn.cursor()
    issue_id = f"issue-{int(time.time())}"
    cursor.execute("INSERT INTO issues (id, timestamp, content) VALUES (?, ?, ?)",
                   (issue_id, datetime.now().strftime('%Y-%m-%d %H:%M:%S'), issue_content))
    conn.commit()
    conn.close()

    print(f"Issue saved: {issue_id} ({tokens_used} tokens)")
    print("── PREVIEW ──")
    print(issue_content[:600])
    print("...")

if __name__ == "__main__":
    main()
