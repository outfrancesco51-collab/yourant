# E2E Test Infra: Yourant

## Test Philosophy
- **Opaque-Box & Requirement-Driven**: Tests are derived strictly from `ORIGINAL_REQUEST.md` and user-facing specifications, exercising the application as an end-user or client would.
- **Methodology**: Systematic 4-tier approach (Category-Partition, Boundary Value Analysis, Pairwise Combinatorial Testing, and Real-World Application Scenarios), concluding with Tier 5 Adversarial Coverage Hardening.

---

## Feature Inventory & Test Coverage Mapping
| # | Feature | Requirement | Tier 1 (Features) | Tier 2 (Boundaries) | Tier 3 (Pairwise) | Tier 4 (Scenarios) |
|---|---------|-------------|:-----------------:|:-------------------:|:-----------------:|:------------------:|
| F1 | Go Server & Health Routing | ORIGINAL_REQUEST §R1 | 5 | 5 | ✓ | ✓ |
| F2 | AniList GraphQL Client & Cache | ORIGINAL_REQUEST §R2 | 5 | 5 | ✓ | ✓ |
| F3 | User Anime Tracking & Progress | ORIGINAL_REQUEST §R2 | 5 | 5 | ✓ | ✓ |
| F4 | Controlled Storage Sandboxing | ORIGINAL_REQUEST §R3, §R5 | 5 | 5 | ✓ | ✓ |
| F5 | Offline Media Downloader | ORIGINAL_REQUEST §R3, §R5 | 5 | 5 | ✓ | ✓ |
| F6 | HTTP 206 Video Streaming | ORIGINAL_REQUEST §R3 | 5 | 5 | ✓ | ✓ |
| F7 | Metallic Blue & Blood Red UI Theme | ORIGINAL_REQUEST §R1 | 5 | 5 | ✓ | ✓ |
| F8 | Seanime App Shell Layout | ORIGINAL_REQUEST §R1 | 5 | 5 | ✓ | ✓ |
| F9 | WebGL & GLSL Shader Background | ORIGINAL_REQUEST §R1 | 5 | 5 | ✓ | ✓ |
| F10 | Web Audio 1000% Volume Booster | ORIGINAL_REQUEST §R3 | 5 | 5 | ✓ | ✓ |
| F11 | Cinema Mode Visual Isolation | ORIGINAL_REQUEST §R3 | 5 | 5 | ✓ | ✓ |
| F12 | Watch Party WebSocket Hub | ORIGINAL_REQUEST §R4 | 5 | 5 | ✓ | ✓ |
| F13 | Watch Party State Synchronization | ORIGINAL_REQUEST §R4 | 5 | 5 | ✓ | ✓ |
| F14 | Watch Party Frontend Client | ORIGINAL_REQUEST §R4 | 5 | 5 | ✓ | ✓ |

Total identified features: $N = 14$.
Required minimum test cases:
- Tier 1: $\ge 70$ tests
- Tier 2: $\ge 70$ tests
- Tier 3: $\ge 14$ tests
- Tier 4: $\ge 7$ real-world application scenarios
- Tier 5: Adversarial white-box tests and forensic audit checks

---

## Test Architecture
- **Location**: `tests/`
- **Runner**: Go test runner (`go test -v ./tests/...`) and Node test runner (`pnpm test` in `frontend/`)
- **Pass/Fail Semantics**: Exit code 0 on all tests passing, non-zero on any failure.
- **Controlled Directory Validation**: Strict assertion that `downloads/` contains only designated files and zero files exist outside this directory.
- **WebSocket Simulation**: Multiple concurrent client connections verifying real-time synchronization under variable simulated latency.

---

## Real-World Application Scenarios (Tier 4)
| # | Scenario | Features Exercised | Complexity |
|---|----------|--------------------|------------|
| 1 | Full Discover to Watch Party Flow | F1, F2, F7, F8, F9, F12, F13, F14 | High |
| 2 | Offline Media Download & Resumable Verification | F1, F4, F5, F6 | High |
| 3 | Extreme Volume Boost (1000%) with Limiter Clamping | F7, F10 | Medium |
| 4 | Cinema Mode Immersion Toggle with Hotkey State | F7, F9, F11 | Medium |
| 5 | Watch Party Multi-Client Play/Pause/Seek Drift Re-sync | F12, F13, F14 | High |
| 6 | Directory Traversal Attack Simulation & Rejection | F4, F5 | High |
| 7 | Offline Fallback Catalog & Disconnected Browsing | F2, F3 | Medium |
