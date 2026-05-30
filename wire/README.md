# Choir Global Wire — Ingestion & Synthesis Engine (V0)

The Choir Global Wire is an autonomous, high-signal global news feed that reads the world in multiple languages and synthesizes its findings into a cohesive, deeply reported newspaper front page every fifteen minutes.

It is designed to be highly decoupled, lightweight, and easily deployable as a persistent background service on a single Ubuntu VM (such as a Manus Cloud Computer).

---

## Architecture

The system is split into three clean, independent components:

```
  ┌────────────────────────────────────────────────────────┐
  │                        INGESTION                       │
  │  (Go binary: concurrent poll of 48 RSS/Telegram)       │
  └───────────────────────────┬────────────────────────────┘
                              │ (write unique items)
                              ▼
  ┌────────────────────────────────────────────────────────┐
  │                         STORAGE                        │
  │                  (SQLite: vanguard.db)                 │
  └───────────────────────────┬────────────────────────────┘
                              │ (read latest signals)
                              ▼
  ┌────────────────────────────────────────────────────────┐
  │                        SYNTHESIS                       │
  │  (Python: LLM call to gpt-4.1-mini / deepseek-v4)     │
  └───────────────────────────┬────────────────────────────┘
                              │ (write synthesized issue)
                              ▼
  ┌────────────────────────────────────────────────────────┐
  │                       PUBLICATION                      │
  │  (Go binary: web server serving JSON API & SPA)        │
  └────────────────────────────────────────────────────────┘
```

1.  **Ingestion (`ingest.go`)**: A high-performance Go program that concurrently polls 48 validated global RSS feeds and Telegram web previews. It uses HTTP conditional requests (`ETag` / `If-Modified-Since`) to minimize bandwidth and only inserts new, unique items into SQLite.
2.  **Storage (`vanguard.db`)**: A lightweight SQLite database containing two main tables: `items` (raw signals) and `issues` (synthesized publications).
3.  **Synthesis (`synthesize.py`)**: A Python script that pulls the most recent 50 signals, ranks them using an **Importance × Rarity** scoring model, and calls the LLM (`gpt-4.1-mini` or `deepseek-v4-flash`) to write 5 in-depth, source-cited stories (approx. 4,000 words total) with explicit attention to multilingual signals.
4.  **Web Server (`server.go`)**: A Go HTTP server that exposes a JSON API (`/api/latest`, `/api/issue/:id`, `/api/archive`) and serves a mobile-first Single Page Application. It uses [Pretext](https://github.com/chenglou/pretext) for progressive layout/text-measurement enhancement in the browser.

---

## Directory Structure

```
wire/
├── static/
│   ├── index.html        # SPA frontend (newspaper grid, expand/collapse, pretext)
│   ├── about.html        # Editorial & architecture about page
│   └── pretext.js        # Pretext text measurement library
├── ingest.go             # Go ingestion engine
├── server.go             # Go web server & API
├── synthesize.py         # Python synthesis script (Importance × Rarity)
├── cycle_runner.py       # Python process orchestrator (runs every 15 min)
├── go.mod                # Go module definition
├── go.sum                # Go module checksums
└── README.md             # This file
```

---

## Setup & Local Run

### 1. Prerequisites
- **Go**: 1.25 or higher
- **Python**: 3.10 or higher
- **API Key**: `OPENAI_API_KEY` (pre-configured in Manus sandbox) or `FIREWORKS_API_KEY`

### 2. Install Dependencies
```bash
# Install Go dependencies
cd wire
go mod tidy

# Install Python dependencies
pip3 install openai sqlite3
```

### 3. Run Ingestion (Go)
Build and run the concurrent poller to populate the database:
```bash
go build -o ingest-engine ingest.go
./ingest-engine
```
This will poll all 48 sources concurrently and populate `vanguard.db` with raw items.

### 4. Run Synthesis (Python)
Ensure your `OPENAI_API_KEY` is set in your environment, then run:
```bash
python3 synthesize.py
```
This will pull the latest 50 items from `vanguard.db`, call the LLM, and save the synthesized issue back to the database.

### 5. Start the Web Server (Go)
Build and run the web server:
```bash
go build -o newspaper-server server.go
./newspaper-server
```
The frontend will be live at **http://localhost:8080**.

### 6. Run the Auto-Cycle (Python)
To keep the newspaper updating in real-time every 15 minutes, run the orchestrator:
```bash
python3 cycle_runner.py
```
This script handles running `./ingest-engine` followed by `python3 synthesize.py` on a strict 15-minute ticker.

---

## Production Deployment (Manus Cloud Computer)

To deploy this permanently on a $10/month or $30/month Manus Cloud Computer:

### 1. Clone the Repo
```bash
git clone <your-repo-url>
cd go-choir/wire
go mod tidy
```

### 2. Configure systemd Services
To ensure the server and cycle runner survive reboots, set them up as systemd services.

Create `/etc/systemd/system/newspaper-server.service`:
```ini
[Unit]
Description=Choir Global Wire Web Server
After=network.target

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/home/ubuntu/go-choir/wire
ExecStart=/home/ubuntu/go-choir/wire/newspaper-server
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Create `/etc/systemd/system/newspaper-cycle.service`:
```ini
[Unit]
Description=Choir Global Wire Ingestion & Synthesis Cycle
After=network.target

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/home/ubuntu/go-choir/wire
Environment=OPENAI_API_KEY=your_key_here
ExecStart=/usr/bin/python3 /home/ubuntu/go-choir/wire/cycle_runner.py
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

### 3. Enable and Start Services
```bash
sudo systemctl daemon-reload
sudo systemctl enable newspaper-server newspaper-cycle
sudo systemctl start newspaper-server newspaper-cycle
```

### 4. Open Firewall Ports
Manus Cloud Computers have a restrictive UFW firewall by default. Open port 80 (HTTP) and 443 (HTTPS):
```bash
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
```

### 5. Set up Domain & HTTPS
Use Certbot to get a free SSL certificate from Let's Encrypt:
```bash
sudo apt-get install certbot -y
# Set up Nginx or reverse proxy to forward port 8080 to 80/443
```
