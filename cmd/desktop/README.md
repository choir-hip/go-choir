# Choir Desktop — Wails v3 macOS App

A native macOS desktop wrapper for the Choir web desktop, built with
[Wails v3](https://v3.wails.io/).

## What This Is

Choir Desktop wraps the existing Svelte frontend in a native macOS window
using Wails v3. In Phase 1, it connects to the staging backend
(`https://choir.news`) for all API calls — no local backend services.

## Prerequisites

- Go 1.25+
- Node.js and npm (for building the Svelte frontend)
- macOS 12.0+ (Monterey or later)
- [Task](https://taskfile.dev) (for build orchestration)

## Setup

```bash
# From the desktop module directory
cd cmd/desktop

# Download Go dependencies
task deps

# Build the frontend and run in dev mode
task dev
```

## Building

```bash
# Build the binary
task build

# Package as .app bundle
task package

# Ad-hoc sign for local testing
task sign
```

## Configuration

The backend URL defaults to `https://choir.news`. Override with:

```bash
# Connect to a local server instead of staging
CHOIR_BACKEND=http://localhost:8082 task dev
```

## Architecture

```
Wails v3 app (Go)
  -> embeds frontend/dist (Svelte build output)
  -> window loads embedded assets in production
  -> /auth/* and /api/* requests are reverse-proxied to the backend
  -> DesktopService exposes app metadata via typed Go bridge
```

The Svelte frontend works unchanged because it uses relative URLs
(`/auth/*`, `/api/*`). The Wails asset handler intercepts these and
forwards them to the backend.

## Pinned Wails Version

```
github.com/wailsapp/wails/v3 v3.0.0-alpha2.104
```

Pinned 2026-06-22. See the spec for version upgrade policy:
[spec-choir-desktop-wails-v3-2026-06-22.md](../../docs/spec-choir-desktop-wails-v3-2026-06-22.md)
