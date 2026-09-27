import fs from 'node:fs';
import path from 'node:path';
import { execSync } from 'node:child_process';

const ROOT_DIR = 'c:\\Users\\Francy\\Desktop\\Yourant';
const GO_BIN = 'C:\\Program Files\\Go\\bin\\go.exe';
const TEST_BIN = path.join(ROOT_DIR, 'yourant_m12_test.exe');

let passedAssertions = 0;
let failedAssertions = 0;
const failureDetails = [];

function assert(condition, message, details = '') {
  if (condition) {
    passedAssertions++;
    console.log(`[PASS] ${message}`);
  } else {
    failedAssertions++;
    console.error(`[FAIL] ${message} - ${details}`);
    failureDetails.push({ message, details });
  }
}

console.log('================================================================');
console.log('Milestone 12 Challenger 2: Go Backend Build & Route Concurrency Test');
console.log('================================================================\n');

// 1. Verify Go Toolchain
console.log('--- 1. Go Toolchain Verification ---');
try {
  const versionOut = execSync(`"${GO_BIN}" version`, { cwd: ROOT_DIR, stdio: 'pipe' }).toString();
  assert(versionOut.includes('go version'), `Go binary is executable: ${versionOut.trim()}`);
} catch (err) {
  assert(false, 'Failed to execute go.exe', err.message);
}

// 2. Compile Backend Binary
console.log('\n--- 2. Go Backend Compilation ---');
try {
  if (fs.existsSync(TEST_BIN)) {
    fs.unlinkSync(TEST_BIN);
  }
  const buildOut = execSync(`"${GO_BIN}" build -tags=nosystray -o yourant_m12_test.exe .`, {
    cwd: ROOT_DIR,
    stdio: 'pipe',
  });
  assert(fs.existsSync(TEST_BIN), 'yourant_m12_test.exe binary produced successfully');
  const stat = fs.statSync(TEST_BIN);
  assert(stat.size > 20 * 1024 * 1024, `Binary size is substantial (${(stat.size / 1024 / 1024).toFixed(1)} MB)`);
} catch (err) {
  assert(false, 'go build -tags=nosystray failed', err.message);
}

// 3. Verify CLI Execution
console.log('\n--- 3. Binary CLI Execution ---');
try {
  const helpOut = execSync(`"${TEST_BIN}" -h`, { cwd: ROOT_DIR, stdio: 'pipe' }).toString();
  assert(helpOut.includes('Seanime'), 'CLI help output contains Seanime application header');
  assert(helpOut.includes('--datadir'), 'CLI help defines --datadir flag');
  assert(helpOut.includes('--port'), 'CLI help defines --port flag');
  assert(helpOut.includes('--desktop-sidecar'), 'CLI help defines --desktop-sidecar flag');
} catch (err) {
  assert(false, 'Execution of yourant_m12_test.exe -h failed', err.message);
}

// 4. Run Unit Tests for Ported Packages
console.log('\n--- 4. Ported Package Unit Tests ---');
try {
  const pkgTests = execSync(
    `"${GO_BIN}" test -v -count=1 ./internal/ountsu/... ./internal/community/... ./internal/anilist/...`,
    { cwd: ROOT_DIR, stdio: 'pipe' }
  ).toString();
  assert(pkgTests.includes('PASS'), 'Unit tests for ountsu, community, and anilist pass');
  assert(pkgTests.includes('TestHub_GetOrCreateRoom'), 'TestHub_GetOrCreateRoom passed');
  assert(pkgTests.includes('TestHub_GenerateInviteKey'), 'TestHub_GenerateInviteKey passed');
  assert(pkgTests.includes('TestRoom_AddRemoveClient'), 'TestRoom_AddRemoveClient passed');
  assert(pkgTests.includes('TestAutoMod_ProcessMessage'), 'TestAutoMod_ProcessMessage passed');
  assert(pkgTests.includes('TestSeedCatalog_Count'), 'TestSeedCatalog_Count passed');
  assert(pkgTests.includes('TestFallbackTrending'), 'TestFallbackTrending passed');
  assert(pkgTests.includes('TestFallbackSearch'), 'TestFallbackSearch passed');
} catch (err) {
  assert(false, 'Package unit tests failed', err.message);
}

// 5. Run Echo Handler Tests
console.log('\n--- 5. Echo Handler Unit Tests ---');
try {
  const handlerTests = execSync(
    `"${GO_BIN}" test -v -count=1 -run "TestHandleOuntsu|TestHandleCommunity" ./internal/handlers/...`,
    { cwd: ROOT_DIR, stdio: 'pipe' }
  ).toString();
  assert(handlerTests.includes('PASS'), 'Echo handler tests pass');
  assert(handlerTests.includes('TestHandleCommunity_ModerationFlow'), 'TestHandleCommunity_ModerationFlow passed');
  assert(handlerTests.includes('TestHandleOuntsuInvite'), 'TestHandleOuntsuInvite passed');
  assert(handlerTests.includes('TestHandleOuntsuWS_Validation'), 'TestHandleOuntsuWS_Validation passed');
} catch (err) {
  assert(false, 'Echo handler tests failed', err.message);
}

// 6. Architectural Route & State Container Contracts
console.log('\n--- 6. Architectural Route & State Container Verification ---');
const routesPath = path.join(ROOT_DIR, 'internal', 'handlers', 'routes.go');
const routesContent = fs.readFileSync(routesPath, 'utf8');
assert(routesContent.includes('v1Ountsu := v1.Group("/ountsu")'), 'Route group /ountsu is mounted under /api/v1');
assert(routesContent.includes('v1Ountsu.GET("/ws", h.HandleOuntsuWS)'), 'GET /api/v1/ountsu/ws is wired to HandleOuntsuWS');
assert(routesContent.includes('v1Ountsu.POST("/invite", h.HandleOuntsuInvite)'), 'POST /api/v1/ountsu/invite is wired to HandleOuntsuInvite');
assert(routesContent.includes('v1Community := v1.Group("/community")'), 'Route group /community is mounted under /api/v1');
assert(routesContent.includes('v1Community.GET("/ban/:id", h.HandleGetBanStatus)'), 'GET /api/v1/community/ban/:id is wired to HandleGetBanStatus');
assert(routesContent.includes('v1Community.POST("/post", h.HandleCommunityPost)'), 'POST /api/v1/community/post is wired to HandleCommunityPost');

const appPath = path.join(ROOT_DIR, 'internal', 'core', 'app.go');
const appContent = fs.readFileSync(appPath, 'utf8');
assert(appContent.includes('OuntsuHub') && appContent.includes('*ountsu.Hub'), 'App struct declares OuntsuHub');
assert(appContent.includes('AutoMod') && appContent.includes('*community.AutoMod'), 'App struct declares AutoMod');
assert(appContent.includes('BanSystem') && appContent.includes('*community.BanSystem'), 'App struct declares BanSystem');
assert(appContent.includes('OuntsuHub:') && appContent.includes('ountsuHub'), 'NewApp populates OuntsuHub field');
assert(appContent.includes('AutoMod:') && appContent.includes('autoMod'), 'NewApp populates AutoMod field');
assert(appContent.includes('BanSystem:') && appContent.includes('banSystem'), 'NewApp populates BanSystem field');

// 7. Clean up test binary
console.log('\n--- 7. Cleanup Test Artifacts ---');
try {
  if (fs.existsSync(TEST_BIN)) {
    fs.unlinkSync(TEST_BIN);
  }
  assert(!fs.existsSync(TEST_BIN), 'Cleaned up yourant_m12_test.exe');
} catch (err) {
  assert(false, 'Failed to clean up test executable', err.message);
}

// Summary
console.log('\n================================================================');
console.log(`Challenger 2 Suite Summary: ${passedAssertions} Passed, ${failedAssertions} Failed`);
console.log('================================================================');

if (failedAssertions > 0) {
  process.exit(1);
} else {
  process.exit(0);
}
