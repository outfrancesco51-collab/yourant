import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { execSync } from 'node:child_process';

const ROOT_DIR = 'c:\\Users\\Francy\\Desktop\\Yourant';
const GO_BIN = 'C:\\Program Files\\Go\\bin\\go.exe';

let passCount = 0;
let failCount = 0;
const failures = [];

function assert(condition, message, details = '') {
  if (condition) {
    passCount++;
    console.log(`[PASS] ${message}`);
  } else {
    failCount++;
    console.error(`[FAIL] ${message} - ${details}`);
    failures.push({ message, details });
  }
}

console.log('====================================================');
console.log('Milestone 12: Feature Porting & Integration Verification');
console.log('====================================================\n');

// 1. Ountsu WebSocket Hub
console.log('--- 1. Ountsu WebSocket Hub & Handlers ---');
const countsuFiles = ['client.go', 'hub.go', 'messages.go', 'room.go'];
for (const f of countsuFiles) {
  const p = path.join(ROOT_DIR, 'internal', 'ountsu', f);
  assert(fs.existsSync(p), `Ountsu package file exists: internal/ountsu/${f}`);
}

const ountsuHandlersPath = path.join(ROOT_DIR, 'internal', 'handlers', 'ountsu.go');
assert(fs.existsSync(ountsuHandlersPath), 'internal/handlers/ountsu.go exists');
if (fs.existsSync(ountsuHandlersPath)) {
  const content = fs.readFileSync(ountsuHandlersPath, 'utf8');
  assert(content.includes('HandleOuntsuWS'), 'HandleOuntsuWS is implemented in ountsu.go');
  assert(content.includes('HandleOuntsuInvite'), 'HandleOuntsuInvite is implemented in ountsu.go');
  assert(content.includes('gorilla/websocket'), 'gorilla/websocket upgrader is used');
  assert(content.includes('inviteKey'), 'HandleOuntsuInvite returns inviteKey');
}

const appGoPath = path.join(ROOT_DIR, 'internal', 'core', 'app.go');
assert(fs.existsSync(appGoPath), 'internal/core/app.go exists');
if (fs.existsSync(appGoPath)) {
  const content = fs.readFileSync(appGoPath, 'utf8');
  assert(content.includes('OuntsuHub') && content.includes('*ountsu.Hub'), 'App struct defines OuntsuHub');
  assert(content.includes('ountsu.NewHub()'), 'NewApp initializes OuntsuHub');
}

const routesGoPath = path.join(ROOT_DIR, 'internal', 'handlers', 'routes.go');
assert(fs.existsSync(routesGoPath), 'internal/handlers/routes.go exists');
if (fs.existsSync(routesGoPath)) {
  const content = fs.readFileSync(routesGoPath, 'utf8');
  assert(content.includes('v1.Group("/ountsu")'), 'Route group /ountsu is registered');
  assert(content.includes('v1Ountsu.GET("/ws", h.HandleOuntsuWS)'), 'GET /api/v1/ountsu/ws is registered');
  assert(content.includes('v1Ountsu.POST("/invite", h.HandleOuntsuInvite)'), 'POST /api/v1/ountsu/invite is registered');
}

// 2. Auto-Mod & Community Handlers
console.log('\n--- 2. Auto-Mod & Community Handlers ---');
const communityModPath = path.join(ROOT_DIR, 'internal', 'community', 'mod.go');
assert(fs.existsSync(communityModPath), 'internal/community/mod.go exists');
if (fs.existsSync(communityModPath)) {
  const content = fs.readFileSync(communityModPath, 'utf8');
  assert(content.includes('type BanSystem struct'), 'BanSystem is defined');
  assert(content.includes('type AutoMod struct'), 'AutoMod is defined');
  assert(content.includes('ProcessMessage'), 'ProcessMessage moderation logic is implemented');
}

const communityHandlersPath = path.join(ROOT_DIR, 'internal', 'handlers', 'community.go');
assert(fs.existsSync(communityHandlersPath), 'internal/handlers/community.go exists');
if (fs.existsSync(communityHandlersPath)) {
  const content = fs.readFileSync(communityHandlersPath, 'utf8');
  assert(content.includes('HandleGetBanStatus'), 'HandleGetBanStatus is implemented');
  assert(content.includes('HandleCommunityPost'), 'HandleCommunityPost is implemented');
  assert(content.includes('isBanned') && content.includes('daysLeft'), 'BanStatusResponse contract is respected');
}

if (fs.existsSync(appGoPath)) {
  const content = fs.readFileSync(appGoPath, 'utf8');
  assert(content.includes('AutoMod') && content.includes('*community.AutoMod'), 'App struct defines AutoMod');
  assert(content.includes('BanSystem') && content.includes('*community.BanSystem'), 'App struct defines BanSystem');
  assert(content.includes('community.NewAutoMod'), 'NewApp initializes AutoMod');
  assert(content.includes('community.NewBanSystem'), 'NewApp initializes BanSystem');
}

if (fs.existsSync(routesGoPath)) {
  const content = fs.readFileSync(routesGoPath, 'utf8');
  assert(content.includes('v1.Group("/community")'), 'Route group /community is registered');
  assert(content.includes('HandleGetBanStatus'), 'GET /api/v1/community/ban/:id is registered');
  assert(content.includes('HandleCommunityPost'), 'POST /api/v1/community/post is registered');
}

// 3. JunglePitchShifter & Audio Booster
console.log('\n--- 3. JunglePitchShifter AudioWorklet & Audio Booster ---');
const webWorkletPath = path.join(ROOT_DIR, 'seanime-web', 'public', 'jungle-worklet.js');
const rootWebWorkletPath = path.join(ROOT_DIR, 'web', 'jungle-worklet.js');
assert(fs.existsSync(webWorkletPath), 'seanime-web/public/jungle-worklet.js exists');
assert(fs.existsSync(rootWebWorkletPath), 'web/jungle-worklet.js exists');

if (fs.existsSync(webWorkletPath)) {
  const content = fs.readFileSync(webWorkletPath, 'utf8');
  assert(content.includes('class JunglePitchShifter extends AudioWorkletProcessor'), 'JunglePitchShifter class is defined');
  assert(content.includes("registerProcessor('jungle-pitch-shifter', JunglePitchShifter)"), 'Processor is registered');
}

const audioBoosterPath = path.join(ROOT_DIR, 'seanime-web', 'src', 'app', '(main)', '_features', 'custom-player', 'AudioBooster.ts');
assert(fs.existsSync(audioBoosterPath), 'AudioBooster.ts exists in seanime-web custom-player');
if (fs.existsSync(audioBoosterPath)) {
  const content = fs.readFileSync(audioBoosterPath, 'utf8');
  assert(content.includes('class AudioBoosterManager'), 'AudioBoosterManager class is implemented');
  assert(content.includes('createGain') && content.includes('createDynamicsCompressor'), 'GainNode and DynamicsCompressorNode safety limiter are configured');
  assert(content.includes('1000.0'), 'Up to 1000% volume boosting is supported');
}

// 4. Electron Discord RPC
console.log('\n--- 4. Electron Discord RPC ---');
const denshiIndexPath = path.join(ROOT_DIR, 'seanime-denshi', 'src', 'main', 'index.ts');
assert(fs.existsSync(denshiIndexPath), 'seanime-denshi/src/main/index.ts exists');
if (fs.existsSync(denshiIndexPath)) {
  const content = fs.readFileSync(denshiIndexPath, 'utf8');
  assert(content.includes('1300000000000000000'), 'Discord client ID 1300000000000000000 is configured');
  assert(content.includes('ipcMain.on("update-presence"'), 'ipcMain.on("update-presence") handler is registered');
}

const denshiPreloadPath = path.join(ROOT_DIR, 'seanime-denshi', 'src', 'preload.js');
assert(fs.existsSync(denshiPreloadPath), 'seanime-denshi/src/preload.js exists');
if (fs.existsSync(denshiPreloadPath)) {
  const content = fs.readFileSync(denshiPreloadPath, 'utf8');
  assert(content.includes('discord: {'), 'window.electron.discord namespace exists');
  assert(content.includes('updatePresence: (data) => ipcRenderer.send("update-presence", data)'), 'discord.updatePresence invokes ipcRenderer.send');
}

// 5. AnimeUnity Stream Extraction & Fallback Catalog
console.log('\n--- 5. AnimeUnity Stream Extraction & Fallback Catalog ---');
if (fs.existsSync(denshiIndexPath)) {
  const content = fs.readFileSync(denshiIndexPath, 'utf8');
  assert(content.includes('ipcMain.handle("extract-animeunity-stream"'), 'ipcMain.handle("extract-animeunity-stream") handler is registered');
  assert(content.includes('.m3u8') && content.includes('.mp4'), 'webRequest filters .m3u8 and .mp4 streams');
}

if (fs.existsSync(denshiPreloadPath)) {
  const content = fs.readFileSync(denshiPreloadPath, 'utf8');
  assert(content.includes('animeunity: {'), 'window.electron.animeunity namespace exists');
  assert(content.includes('extractStream: (url) => ipcRenderer.invoke("extract-animeunity-stream", url)'), 'animeunity.extractStream invokes ipcRenderer.invoke');
}

const catalogPath = path.join(ROOT_DIR, 'internal', 'anilist', 'catalog.go');
assert(fs.existsSync(catalogPath), 'internal/anilist/catalog.go exists');
if (fs.existsSync(catalogPath)) {
  const content = fs.readFileSync(catalogPath, 'utf8');
  assert(content.includes('SeedCatalog = []AnimeMedia{'), 'SeedCatalog is defined');
  assert(content.includes('FallbackTrending'), 'FallbackTrending function exists');
  assert(content.includes('FallbackPopular'), 'FallbackPopular function exists');
  assert(content.includes('FallbackSearch'), 'FallbackSearch function exists');
  assert(content.includes('FallbackDetail'), 'FallbackDetail function exists');
}

// 6. Go Tests & Compilation
console.log('\n--- 6. Backend Compilation & Test Verification ---');
try {
  const testOut = execSync(`"${GO_BIN}" test -v ./internal/ountsu/... ./internal/community/... ./internal/anilist/... ./internal/handlers/... -run "TestHandleOuntsu|TestHandleCommunity"`, {
    cwd: ROOT_DIR,
    stdio: 'pipe',
  }).toString();
  assert(testOut.includes('PASS'), 'Go unit tests for ported packages pass cleanly');
  console.log('  Unit tests executed successfully.');
} catch (err) {
  assert(false, 'Go unit tests execution failed', err.message);
}

try {
  execSync(`"${GO_BIN}" build -tags=nosystray .`, {
    cwd: ROOT_DIR,
    stdio: 'pipe',
  });
  assert(true, 'go build -tags=nosystray . compiles successfully with exit code 0');
  console.log('  Backend build succeeded.');
} catch (err) {
  assert(false, 'go build -tags=nosystray . failed to compile', err.message);
}

console.log('\n====================================================');
console.log(`Summary: ${passCount} Passed, ${failCount} Failed`);
console.log('====================================================');

if (failCount > 0) {
  process.exit(1);
} else {
  process.exit(0);
}
