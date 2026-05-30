"""
Choir Global Wire — Cycle Runner
Runs ingest (via subprocess) + synthesis every 15 minutes.
"""
import subprocess
import time
import sys
import os
from datetime import datetime

CYCLE_INTERVAL = 15 * 60  # 15 minutes

def run_ingest(timeout=90):
    """Run the Go ingest engine for up to `timeout` seconds."""
    print(f"  [ingest] Starting ({timeout}s window)...")
    try:
        result = subprocess.run(
            ["./ingest-engine"],
            cwd="/home/ubuntu/ingest-vanguard",
            timeout=timeout,
            capture_output=False
        )
    except subprocess.TimeoutExpired:
        print(f"  [ingest] Timeout reached — continuing to synthesis")

def run_synthesis():
    """Run the Python synthesis script."""
    print(f"  [synthesis] Starting...")
    result = subprocess.run(
        [sys.executable, "synthesize.py"],
        cwd="/home/ubuntu/ingest-vanguard",
    )
    if result.returncode != 0:
        print(f"  [synthesis] ERROR: exit code {result.returncode}")

def main():
    print("Choir Global Wire — Cycle Runner")
    print(f"Cycle interval: 15 minutes")
    print(f"Started: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
    print()

    cycle = 0
    while True:
        cycle += 1
        now = datetime.now().strftime('%Y-%m-%d %H:%M:%S')
        print(f"══ Cycle #{cycle} — {now} ══")

        run_ingest(timeout=90)
        run_synthesis()

        next_run = datetime.fromtimestamp(time.time() + CYCLE_INTERVAL).strftime('%H:%M:%S')
        print(f"  Next cycle at {next_run}")
        print()
        time.sleep(CYCLE_INTERVAL)

if __name__ == "__main__":
    main()
