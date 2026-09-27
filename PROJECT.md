# Project: Seanime Mirroring & Feature Adaptation

## Architecture
Reorganize and overwrite the `Yourant` repository to directly mirror Seanime's directory architecture, package naming conventions, and documentation files (`.md`), while porting Yourant's specific capabilities (Ountsu WebSocket hub, JunglePitchShifter AudioWorklet, Electron Discord RPC, AnimeUnity extraction, Auto-Mod) so they continue to function without breaking Seanime's architectural patterns.

```
┌────────────────────────────────────────────────────────────────────────┐
│               Top-Level Layout (Mirrors Seanime-Studio)                │
│                                                                        │
│  main.go, go.mod, go.sum, .gitignore, .golangci.yml, README.md,        │
│  CHANGELOG.md, CONTRIBUTING.md, DEVELOPMENT_AND_BUILD.md, LICENSE      │
└───────────────────────────────────┬────────────────────────────────────┘
                                    │
    ┌───────────────────────────────┼──────────────────────────────┐
    ▼                               ▼                              ▼
┌───────────────────────┐ ┌───────────────────────┐ ┌───────────────────────┐
│     internal/...      │ │     seanime-web/      │ │    seanime-denshi/    │
│  53 Go domain packages│ │  React / Vite / TS    │ │  Electron wrapper     │
│  - core, server       │ │  - public/            │ │  - src/main (RPC,     │
│  - handlers (Echo)    │ │    jungle-worklet.js  │ │    AnimeUnity hook)   │
│  - events, nakama     │ │  - AudioBooster       │ │  - src/preload.js     │
│  - ountsu (Hub, WS)   │ │  - Ountsu component   │ │  - src/cast           │
│  - community (AutoMod)│ │  - WatchParty sync    │ │  - package.json       │
└───────────────────────┘ └───────────────────────┘ └───────────────────────┘
```

## Feature Inventory
| # | Feature | Description | Milestone | Source |
|---|---------|-------------|-----------|--------|
| F1 | Seanime Top-Level & Docs Mirroring | Mirror Seanime root layout, config files, 21 markdown docs, and directory tree while archiving legacy docs in `docs/yourant/` | M11 | ORIGINAL_REQUEST §R2 |
| F2 | Go Backend Domain Package Architecture | Reorganize `internal/` into Seanime's 53 modular domain packages, root `main.go`, and `go.mod` (`module seanime`) | M11 | ORIGINAL_REQUEST §R1, §R2 |
| F3 | Ountsu WebSocket Hub Porting | Port Ountsu hub, room management, WebRTC signaling, invite generation to `internal/ountsu/` and Echo endpoints in `internal/handlers/ountsu.go` (`/api/v1/ountsu/ws`, `/api/v1/ountsu/invite`) | M12 | ORIGINAL_REQUEST §R3 |
| F4 | JunglePitchShifter & Audio Booster Porting | Copy `jungle-worklet.js` to `seanime-web/public/` and `web/`, port `AudioBooster.ts` and pitch shift pipeline | M12 | ORIGINAL_REQUEST §R3 |
| F5 | Electron Shell & Discord RPC Porting | Port Discord RPC (client ID `1300000000000000000`) and IPC channel `update-presence` into `seanime-denshi/` main and preload scripts | M12 | ORIGINAL_REQUEST §R3 |
| F6 | AnimeUnity Stream Extraction Porting | Port headless BrowserWindow webRequest interceptor (`extract-animeunity-stream`) into `seanime-denshi/` and seed fallback catalog into Go backend | M12 | ORIGINAL_REQUEST §R3 |
| F7 | Auto-Mod & Ban System Porting | Port `BanSystem` and regex `AutoMod` into `internal/community/` and Echo endpoints in `internal/handlers/community.go` (`/api/v1/community/ban/:id`, `/api/v1/community/post`) | M12 | ORIGINAL_REQUEST §R3 |
| F8 | Go Backend Compilation & Package Alignment | Resolve all Go imports, route registrations in `internal/handlers/routes.go`, embed FS (`web/.gitkeep`), compile cleanly via `go build -tags=nosystray` | M13 | ORIGINAL_REQUEST §Acceptance Criteria |
| F9 | Acceptance Criteria Verification Suite | Programmatic test verifying Seanime clone presence, structural parity, Ountsu & Electron shell endpoints, and Go compilation | M14 | ORIGINAL_REQUEST §Acceptance Criteria |

## Milestones
| # | Name | Scope | Dependencies | Status |
|---|------|-------|-------------|--------|
| M10 | Seanime Setup & Comprehensive Survey | Clone Seanime repo to `C:\Users\Francy\Desktop\seanime-studio` and survey architecture | None | DONE |
| M11 | Structural Reorganization & File Mirroring | Mirror top-level directory layout, documentation files (.md), configs, and package structures | M10 | DONE |
| M12 | Feature Porting & Integration | Port Ountsu, JunglePitchShifter, Discord RPC, AnimeUnity extraction, and Auto-Mod | M11 | DONE |
| M13 | Go Backend Compilation & Package Alignment | Align imports with `module seanime`, wire handlers into Echo routes, verify `go build -tags=nosystray` | M12 | IN_PROGRESS |
| M14 | Verification Suite & Forensic Integrity Audit | Multi-tier test suite, automated verification script, reviewer, challenger, and forensic audit | M13 | PLANNED |

## Interface Contracts
### Ountsu Endpoints
- `GET /api/v1/ountsu/ws?roomId={roomId}&userId={userId}` -> WebSocket upgrade, WebRTC signaling & room sync.
- `POST /api/v1/ountsu/invite` -> JSON `{ "inviteKey": "<hex8>" }`.

### Auto-Mod Endpoints
- `GET /api/v1/community/ban/:id` -> JSON `{ "isBanned": bool, "reason": string, "daysLeft": float64 }`.
- `POST /api/v1/community/post` -> Body `{ "userId": string, "message": string }` -> JSON `{ "sanitized": string, "banned": bool }`.

### Electron IPC Channels
- `update-presence` -> Discord Rich Presence activity payload.
- `extract-animeunity-stream` -> URL extraction returning `.m3u8` or `.mp4` stream string.

## Code Layout
- Root: `main.go`, `go.mod`, `go.sum`, `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `DEVELOPMENT_AND_BUILD.md`, `LICENSE`, `.gitignore`, `.golangci.yml`.
- Backend: `internal/...` (53 Seanime packages + `internal/ountsu/` + `internal/community/`).
- Web client: `seanime-web/` and embedded assets in `web/`.
- Desktop shell: `seanime-denshi/` and legacy `electron/`.
- Docs: `docs/` and `docs/yourant/`.
- Tests: `test/` and `tests/`.
