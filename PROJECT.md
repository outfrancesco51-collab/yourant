# Project: Yourant

## Architecture
Yourant is a polished, production-ready anime tracking and streaming application inspired by Seanime.
It features a high-performance Go backend and a modern TypeScript + HTML + GLSL frontend styled with a distinct metallic blue and blood red design system.

### System Architecture Diagram
```
┌────────────────────────────────────────────────────────────────────────┐
│                          Frontend (Vite + TS)                          │
│                                                                        │
│  ┌────────────────────┐  ┌────────────────────┐  ┌──────────────────┐  │
│  │ WebGL / GLSL Canvas│  │ HTML5 Video Player │  │  Watch Party UI  │  │
│  │ Metallic Blue &    │  │ Web Audio 1000%    │  │  Room Drawer,    │  │
│  │ Blood Red FBM Wave │  │ Booster (Gain=10)  │  │  Chat & Status   │  │
│  │ u_dim (Cinema Mode)│  │ Cinema Mode Toggle │  │  Sync Pill       │  │
│  └────────────────────┘  └──────────┬─────────┘  └────────┬─────────┘  │
│                                     │                     │            │
└─────────────────────────────────────┼─────────────────────┼────────────┘
                                      │ REST / HTTP 206     │ WebSocket
                                      ▼                     ▼
┌────────────────────────────────────────────────────────────────────────┐
│                           Go Backend Server                            │
│                                                                        │
│  ┌───────────────────────┐  ┌───────────────────┐  ┌────────────────┐  │
│  │ AniList GraphQL Client│  │ Controlled Storage│  │ Watch Party Hub│  │
│  │ In-memory TTL Cache   │  │ Sandboxed to      │  │ gorilla/ws     │  │
│  │ Token Bucket (80/min) │  │ /downloads        │  │ 3-tier drift   │  │
│  │ Fallback Catalog      │  │ Path Traversal Sec│  │ compensation   │  │
│  │ REST: /api/anime/*    │  │ HTTP 206 Streaming│  │ WS: /ws/party  │  │
│  └───────────────────────┘  └───────────────────┘  └────────────────┘  │
└────────────────────────────────────────────────────────────────────────┘
```

---

## Feature Inventory
Every feature from the Survey phase and `ORIGINAL_REQUEST.md` is enumerated here with its assigned milestone.

| # | Feature | Description | Milestone | Source |
|---|---------|-------------|-----------|--------|
| F1 | Go Backend Server & Routing | High-performance Go HTTP server with standard `net/http` pattern routing, health check, CORS, and graceful shutdown | M1 | ORIGINAL_REQUEST §R1 |
| F2 | AniList GraphQL Client & Cache | Fetch trending, popular, search, and anime details from AniList GraphQL with in-memory TTL caching and 80 req/min token bucket limiter | M1 | ORIGINAL_REQUEST §R2 |
| F3 | Anime Tracking & Local Library | In-memory/local JSON tracking store for user anime status (CURRENT, PLANNING, COMPLETED) and episode progress | M1 | ORIGINAL_REQUEST §R2 |
| F4 | Controlled Storage Engine | Strict directory sandboxing confining all file writes to `c:/Users/Francy/Desktop/Yourant/downloads`. Rigorous path traversal defense (`filepath.Clean`, `filepath.Rel`, filename sanitization) | M1 | ORIGINAL_REQUEST §R3, §R5 |
| F5 | Offline Media Downloader | Background resumable streaming file downloader with `.part` chunked writing, atomic rename on completion, progress tracking, and cancellation | M1 | ORIGINAL_REQUEST §R3, §R5 |
| F6 | HTTP 206 Video Streaming | Local video file streaming endpoint supporting RFC 7233 byte-range seeking for smooth HTML5 video scrubbing | M1 | ORIGINAL_REQUEST §R3 |
| F7 | Metallic Blue & Blood Red UI Theme | Design token architecture (`theme.css`) with metallic obsidian `#070a13`, steel slate `#0f172a`, metallic cyan `#38bdf8`, deep crimson `#881337`, and blood red `#e11d48` | M2 | ORIGINAL_REQUEST §R1 |
| F8 | Seanime-Inspired App Shell | Responsive desktop launcher layout: collapsible sidebar (Library, Discover, Downloads, Watch Party), hero banner with dual gradient masking, and 2:3 media cards | M2 | ORIGINAL_REQUEST §R1 |
| F9 | WebGL & GLSL Shader Background | Fullscreen WebGL canvas running custom GLSL vertex & fragment shaders with Fractal Brownian Motion (FBM) domain warping, reactive mouse parallax, and `u_dim` uniform | M2 | ORIGINAL_REQUEST §R1 |
| F10 | Web Audio API 1000% Volume Booster | Custom audio graph (`AudioContext` -> `MediaElementSourceNode` -> `GainNode` -> `DynamicsCompressorNode` -> `destination`) supporting gain up to 10.0 (1000%), `WeakMap` singleton caching, and user gesture unlocking | M2 | ORIGINAL_REQUEST §R3 |
| F11 | Cinema Mode Visual Isolation | Visual isolation toggle dimming background chrome and GLSL shader, expanding video player to 100vw/100vh with auto-hiding controls, hotkeys `C` and `Escape` | M2 | ORIGINAL_REQUEST §R3 |
| F12 | Watch Party WebSocket Hub | Go WebSocket server hub using `gorilla/websocket` with room management (create, join, leave), host vs client roles, and broadcast channels | M3 | ORIGINAL_REQUEST §R4 |
| F13 | Watch Party State Synchronization | Bidirectional JSON message protocol (play, pause, seek, heartbeat) with 3-tier drift compensation algorithm (<300ms synced, 300-1500ms speed adjustment, >1500ms hard seek) | M3 | ORIGINAL_REQUEST §R4 |
| F14 | Watch Party Frontend Client | UI drawer for room creation, code generation, real-time sync status pill (Green/Amber/Red), participant list, and floating translucent chat stream | M3 | ORIGINAL_REQUEST §R4 |
| F15 | Comprehensive Multi-Tier E2E Test Suite | Automated end-to-end verification covering Tiers 1-4 (feature tests, boundaries, combinations, real-world scenarios) | M4 | ORIGINAL_REQUEST §Verification |
| F16 | Tier 5 Adversarial Coverage Hardening | White-box adversarial testing, edge case stress testing, and forensic integrity audit verification | M4 | Project Pattern |

---

## Milestones

| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| M1 | Backend Core, AniList APIs & Controlled Storage | Go backend server, AniList GraphQL integration with cache/rate-limiting, strict controlled storage engine for `downloads/` with path traversal defense, and HTTP 206 video range streaming. | None | DONE |
| M2 | Frontend UI, GLSL Shaders & Web Audio Booster Player | Vite + TS app shell with metallic blue and blood red theme, WebGL/GLSL animated background shader, HTML5 player with 1000% Web Audio volume booster and Cinema Mode isolation. | M1 | IN_PROGRESS |
| M3 | Real-Time Watch Party Subsystem | Full WebSocket watch party hub in Go backend and synchronized frontend party drawer, real-time play/pause/seek sync with 3-tier drift compensation, and chat. | M1, M2 | PLANNED |
| M4 | E2E Testing Track & Adversarial Coverage Hardening | 100% pass of Tiers 1-4 E2E test suite, publication of `TEST_READY.md`, followed by Tier 5 adversarial testing and forensic audit clean verification. | M1, M2, M3 | PLANNED |

---

## Interface Contracts

### 1. Frontend ↔ Backend REST API (`/api/*`)
- **Headers**: `Accept: application/json`, `Content-Type: application/json`
- **CORS**: `Access-Control-Allow-Origin: *`, `Access-Control-Allow-Headers: *`, `Access-Control-Allow-Methods: GET, POST, DELETE, OPTIONS`
- **Endpoints**:
  - `GET /api/health` -> `{ "status": "ok", "version": "1.0.0" }`
  - `GET /api/anime/trending?page=1&perPage=20` -> `{ "page": 1, "items": [AnimeMedia] }`
  - `GET /api/anime/search?q={query}` -> `{ "page": 1, "items": [AnimeMedia] }`
  - `GET /api/anime/{id}` -> `AnimeMediaDetail`
  - `GET /api/user/tracking/list` -> `{ "lists": [UserLibraryEntry] }`
  - `POST /api/user/tracking/update` -> `{ "mediaId": int, "status": string, "progress": int, "score": float }`
  - `GET /api/downloads` -> `[DownloadRecord]`
  - `POST /api/downloads` -> `{ "animeId": int, "episodeNumber": int, "title": string, "url": string, "fileName": string }`
  - `DELETE /api/downloads/{id}` -> `{ "success": true }`
  - `GET /api/downloads/files` -> `[{ "fileName": string, "size": int64, "modTime": string }]`
  - `GET /api/downloads/stream/{fileName}` -> HTTP 206 Partial Content video stream (`Accept-Ranges: bytes`)

### 2. Frontend ↔ Backend Watch Party WebSocket (`/api/watchparty/ws`)
- **URL**: `/api/watchparty/ws?roomId={roomId}&userId={userId}&name={name}&isHost={bool}`
- **Message Format**: JSON string frames conforming to `WatchPartyMessage`:
  - `type`: `room:join`, `room:joined`, `host:play`, `host:pause`, `host:seek`, `host:heartbeat`, `sync:state`, `chat:message`, `ping`, `pong`
  - `roomId`: string (e.g. `WP-7F3A`)
  - `senderId`: string
  - `senderName`: string
  - `isHost`: boolean
  - `serverTime`: int64 (Unix epoch ms)
  - `payload`: `{ isPlaying?: bool, timestamp?: float64, playbackRate?: float64, chatText?: string, members?: [] }`

### 3. Video Player ↔ Web Audio Booster API
- `AudioBoosterManager` maintains singleton audio graph per `HTMLVideoElement`:
  - `setVolume(percentage: number)`: sets gain from `0.0` (0%) to `10.0` (1000%).
  - `DynamicsCompressorNode` acts as a transparent peak limiter clamping peaks before 0dBFS.

### 4. Player Container ↔ Cinema Mode Engine
- `CinemaModeManager.toggle()`:
  - Adds/removes `.cinema-mode-active` class on `document.body`.
  - Communicates with `ShaderBackground.setCinemaMode(active: boolean)` to adjust `u_dim` uniform between 1.0 (normal) and 0.15 (cinema).
  - Handles `C` and `Escape` keyboard shortcuts.

---

## Code Layout

```
c:/Users/Francy/Desktop/Yourant/
├── cmd/
│   └── server/
│       └── main.go                 # Server entry point, CLI flags, graceful shutdown
├── internal/
│   ├── anilist/                    # AniList GraphQL client, models, TTL cache & rate limiter
│   │   ├── client.go
│   │   ├── cache.go
│   │   └── models.go
│   ├── downloader/                 # Resumable downloader & strict downloads/ sandboxing
│   │   ├── downloader.go
│   │   ├── guard.go                # Path traversal defense (filepath.Clean, Rel, reserved DOS names)
│   │   └── guard_test.go           # Unit tests asserting path traversal rejection
│   ├── party/                      # Watch party WebSocket hub and room manager
│   │   ├── hub.go
│   │   ├── client.go
│   │   └── room.go
│   ├── storage/                    # Local tracking store & metadata persistence
│   │   └── store.go
│   └── api/                        # HTTP REST route handlers
│       ├── router.go
│       ├── anime_handlers.go
│       ├── download_handlers.go
│       └── party_handlers.go
├── downloads/                      # Strictly policed directory for media downloads
├── frontend/                       # Vite + TypeScript SPA
│   ├── index.html                  # HTML entry point with WebGL canvas
│   ├── package.json
│   ├── tsconfig.json
│   ├── vite.config.ts              # Proxy /api and /api/watchparty/ws to :8080
│   └── src/
│       ├── main.ts                 # Bootstrap, state initialization, navigation
│       ├── theme.css               # Metallic blue & blood red design tokens
│       ├── api/                    # REST client & WebSocket party client
│       │   ├── client.ts
│       │   └── ws.ts
│       ├── audio/                  # Web Audio API 1000% booster manager
│       │   └── AudioBooster.ts
│       ├── cinema/                 # Cinema mode visual isolation controller
│       │   └── CinemaMode.ts
│       ├── glsl/                   # WebGL background shader pipeline
│       │   ├── background.vert
│       │   ├── background.frag
│       │   └── ShaderBackground.ts
│       ├── party/                  # Watch party sync engine & UI
│       │   ├── WatchPartySync.ts
│       │   └── WatchPartyModal.ts
│       └── components/             # Seanime-inspired UI components
│           ├── Sidebar.ts
│           ├── HeroBanner.ts
│           ├── MediaGrid.ts
│           ├── MediaModal.ts
│           └── Player.ts
├── tests/                          # Automated E2E test suites (Tiers 1-4)
│   ├── e2e_runner.go               # Unified test runner
│   ├── tier1_features_test.go      # Individual feature tests
│   ├── tier2_boundaries_test.go    # Boundary, edge case & security tests
│   ├── tier3_combinations_test.go  # Cross-feature pairwise tests
│   └── tier4_scenarios_test.go     # Real-world application scenarios
├── go.mod
├── go.sum
└── PROJECT.md
```
