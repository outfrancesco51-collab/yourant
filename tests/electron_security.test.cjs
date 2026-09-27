/**
 * tests/electron_security.test.cjs
 *
 * Empirical Adversarial Security & Stress Test Suite for Electron Preload & Sender:
 * 1. IPC Channel Allowlist verification (asserting unauthorized channels are blocked)
 * 2. Preload sandbox integrity & leak detection
 * 3. URL rewriting in sender.js against localhost, 127.0.0.1, LAN IPs, IPv6, external URLs,
 *    and edge-case inputs
 * 4. DesktopSender IPC bridge delegation
 */

const Module = require('module');
const path = require('path');
const os = require('os');

function assert(condition, message) {
  if (!condition) {
    throw new Error(`[SECURITY TEST FAILURE] ${message}`);
  }
}

console.log('=== Suite 3: Electron Preload & Sender Security Stress Tests ===\n');

// --------------------------------------------------------------------------
// 1. Setup Mock Electron IPC Environment for Preload Testing
// --------------------------------------------------------------------------
const originalRequire = Module.prototype.require;

const exposedAPIs = {};
const ipcEvents = {
  send: [],
  on: new Map(),
  invocations: [],
  removedListeners: [],
};

const mockIpcRenderer = {
  send: (channel, ...args) => {
    ipcEvents.send.push({ channel, args });
  },
  on: (channel, callback) => {
    if (!ipcEvents.on.has(channel)) {
      ipcEvents.on.set(channel, []);
    }
    ipcEvents.on.get(channel).push(callback);
  },
  removeListener: (channel, callback) => {
    ipcEvents.removedListeners.push({ channel, callback });
    if (ipcEvents.on.has(channel)) {
      const list = ipcEvents.on.get(channel).filter(cb => cb !== callback);
      ipcEvents.on.set(channel, list);
    }
  },
  invoke: (channel, ...args) => {
    ipcEvents.invocations.push({ channel, args });
    if (channel === 'is-local-server-reachable') return Promise.resolve(true);
    if (channel === 'get-local-server-port') return Promise.resolve(8080);
    if (channel === 'window:isMaximized') return Promise.resolve(false);
    return Promise.resolve({ success: true });
  },
};

const mockContextBridge = {
  exposeInMainWorld: (key, api) => {
    exposedAPIs[key] = api;
  },
};

// Hook require('electron')
Module.prototype.require = function (modulePath) {
  if (modulePath === 'electron') {
    return {
      contextBridge: mockContextBridge,
      ipcRenderer: mockIpcRenderer,
    };
  }
  return originalRequire.apply(this, arguments);
};

// Load preload.js
const preloadPath = path.resolve(__dirname, '..', 'electron', 'preload.js');
require(preloadPath);

// Restore original require for sender.js
Module.prototype.require = originalRequire;

const electronBridge = exposedAPIs['electron'];
assert(Boolean(electronBridge), 'contextBridge must expose "electron"');
assert(exposedAPIs['__isElectronDesktop__'] === true, 'contextBridge must expose __isElectronDesktop__ as true');

// --------------------------------------------------------------------------
// TEST 3.1: IPC Channel Allowlist Enforcement (Sending)
// --------------------------------------------------------------------------
console.log('--- Test 3.1: IPC Channel Allowlist Enforcement (Send) ---');

// 1. Authorized send channels must succeed
const sampleAuthorizedSend = [
  'restart-app',
  'quit-app',
  'restart-server',
  'kill-server',
  'window:minimize',
  'window:maximize',
  'window:close',
  'window:toggleMaximize',
  'window:setFullscreen',
  'window:hide',
  'window:show',
  'startup:renderer-ready',
];

for (const channel of sampleAuthorizedSend) {
  ipcEvents.send = [];
  electronBridge.send(channel, 'arg1', 42);
  assert(ipcEvents.send.length === 1, `Authorized channel "${channel}" should trigger ipcRenderer.send`);
  assert(ipcEvents.send[0].channel === channel, `Channel sent must be "${channel}"`);
}
console.log(`  ✓ All ${sampleAuthorizedSend.length} authorized send channels successfully dispatched.`);

// 2. Unauthorized send channels MUST BE BLOCKED
const maliciousSendChannels = [
  'shell:openPath',
  'shell:openExternal',
  'child_process:exec',
  'fs:readFile',
  'fs:writeFile',
  'eval',
  'node:eval',
  '__proto__',
  'constructor',
  'toString',
  'execute-script',
  'admin:grantRole',
  'auth:bypass',
  'arbitrary:channel:123',
  '',
];

for (const channel of maliciousSendChannels) {
  ipcEvents.send = [];
  electronBridge.send(channel, 'payload');
  assert(ipcEvents.send.length === 0, `SECURITY VIOLATION: Unauthorized send channel "${channel}" was NOT blocked!`);
}
console.log(`  ✓ All ${maliciousSendChannels.length} unauthorized send channels strictly blocked.`);

// --------------------------------------------------------------------------
// TEST 3.2: IPC Channel Allowlist Enforcement (Listening / on)
// --------------------------------------------------------------------------
console.log('--- Test 3.2: IPC Channel Allowlist Enforcement (Listen / on) ---');

// 1. Authorized on channels must register and return unsubscribe
const sampleAuthorizedOn = [
  'message',
  'crash',
  'window:minimized',
  'window:maximized',
  'update-downloaded',
  'cast:deviceFound',
  'cast:sessionUpdate',
  'cast:mediaStatus',
  'server-status',
];

for (const channel of sampleAuthorizedOn) {
  let callCount = 0;
  const cb = () => { callCount++; };
  const unsub = electronBridge.on(channel, cb);
  assert(typeof unsub === 'function', `electron.on("${channel}") must return an unsubscribe function`);
  assert(ipcEvents.on.has(channel), `Authorized channel "${channel}" must be registered in ipcRenderer`);

  // Test unsubscribe
  unsub();
  assert(ipcEvents.removedListeners.some(r => r.channel === channel), `Unsubscribe must call removeListener for "${channel}"`);
}
console.log(`  ✓ All ${sampleAuthorizedOn.length} authorized listen channels registered and deregistered cleanly.`);

// 2. Unauthorized on channels MUST BE BLOCKED
const maliciousOnChannels = [
  'secret:token',
  'remote:eval',
  'shell:stream',
  'ipc:raw',
  'system:keystrokes',
  'password:dump',
  'arbitrary-event',
];

for (const channel of maliciousOnChannels) {
  const initialRegistered = ipcEvents.on.get(channel) ? ipcEvents.on.get(channel).length : 0;
  const unsub = electronBridge.on(channel, () => {});
  const afterRegistered = ipcEvents.on.get(channel) ? ipcEvents.on.get(channel).length : 0;
  assert(afterRegistered === initialRegistered, `SECURITY VIOLATION: Unauthorized listen channel "${channel}" was registered!`);
  assert(typeof unsub === 'function', 'Even blocked channels must return a safe no-op cleanup function');
  // calling no-op unsub should not throw
  unsub();
}
console.log(`  ✓ All ${maliciousOnChannels.length} unauthorized listen channels strictly blocked.`);

// --------------------------------------------------------------------------
// TEST 3.3: Preload Sandbox Integrity & No Internal Leaks
// --------------------------------------------------------------------------
console.log('--- Test 3.3: Preload Sandbox & Principle of Least Privilege ---');
assert(!('ipcRenderer' in electronBridge), 'ipcRenderer must NOT be leaked to renderer');
assert(!('require' in electronBridge), 'require must NOT be leaked to renderer');
assert(!('process' in electronBridge), 'process object must NOT be leaked to renderer');
assert(typeof electronBridge.platform === 'string', 'Only process.platform string should be exposed');
assert(!('fs' in electronBridge), 'Node fs must NOT be exposed');
assert(!('child_process' in electronBridge), 'child_process must NOT be exposed');
console.log('  ✓ No Node.js internals or raw ipcRenderer instances leaked into window.electron.');

// --------------------------------------------------------------------------
// TEST 3.4: CastSender URL Rewriting Stress Testing
// --------------------------------------------------------------------------
console.log('--- Test 3.4: CastSender URL Rewriting Stress Testing ---');

const { CastSender, DesktopSender } = require('../electron/sender.js');
const sender = new CastSender();

const lanIP = sender.getLanIP();
assert(typeof lanIP === 'string' && lanIP.length > 0, `LAN IP must be a non-empty string, got: ${lanIP}`);
console.log(`  Detected Host LAN IP for testing: ${lanIP}`);

const urlTestCases = [
  {
    name: 'localhost standard HTTP video stream',
    input: 'http://localhost:8080/api/stream/video.mp4',
    expected: `http://${lanIP}:8080/api/stream/video.mp4`,
  },
  {
    name: '127.0.0.1 standard HTTP video stream',
    input: 'http://127.0.0.1:8080/api/stream/video.mp4',
    expected: `http://${lanIP}:8080/api/stream/video.mp4`,
  },
  {
    name: '{{SERVER_URL}} placeholder with default port 8080',
    input: '{{SERVER_URL}}/api/subtitles/track.ass',
    expected: `http://${lanIP}:8080/api/subtitles/track.ass`,
  },
  {
    name: '{{SERVER_URL}} placeholder with custom port 9090',
    input: '{{SERVER_URL}}/api/stream/chunk.m4s',
    port: 9090,
    expected: `http://${lanIP}:9090/api/stream/chunk.m4s`,
  },
  {
    name: 'localhost with custom port 3000',
    input: 'http://localhost:3000/fonts/Roboto.ttf',
    expected: `http://${lanIP}:3000/fonts/Roboto.ttf`,
  },
  {
    name: '127.0.0.1 with query parameters containing localhost',
    input: 'http://127.0.0.1:8080/api/stream?src=http://localhost:8080/anime.mkv',
    expected: `http://${lanIP}:8080/api/stream?src=http://${lanIP}:8080/anime.mkv`,
  },
  {
    name: 'Existing private LAN IP (must remain unchanged)',
    input: 'http://192.168.1.150:8080/api/stream/video.mp4',
    expected: 'http://192.168.1.150:8080/api/stream/video.mp4',
  },
  {
    name: 'External internet HTTPS CDN (must remain unchanged)',
    input: 'https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4',
    expected: 'https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4',
  },
  {
    name: 'IPv6 localhost [::1] (observed behavior)',
    input: 'http://[::1]:8080/api/stream/video.mp4',
    // sender.js targets 127.0.0.1 and localhost
    expected: 'http://[::1]:8080/api/stream/video.mp4',
  },
];

for (const tc of urlTestCases) {
  const result = sender.rewriteUrlForCast(tc.input, tc.port || 8080);
  assert(result === tc.expected, `${tc.name} failed:\n  Input:    ${tc.input}\n  Expected: ${tc.expected}\n  Actual:   ${result}`);
  console.log(`  ✓ Sub-case passed: ${tc.name}`);
}

// Edge Cases and Hostile Inputs
const hostileInputs = [
  { input: null, expected: null },
  { input: undefined, expected: undefined },
  { input: '', expected: '' },
  { input: 12345, expected: 12345 },
  { input: { url: 'http://localhost' }, expected: { url: 'http://localhost' } },
];

for (const hi of hostileInputs) {
  const result = sender.rewriteUrlForCast(hi.input);
  assert(result === hi.input, `Hostile input ${JSON.stringify(hi.input)} failed to return gracefully`);
}
console.log('  ✓ Hostile / non-string inputs handled safely without throwing.');

// --------------------------------------------------------------------------
// TEST 3.5: DesktopSender Renderer Helper Integration
// --------------------------------------------------------------------------
console.log('--- Test 3.5: DesktopSender Renderer Helper ---');

const desktopSender = new DesktopSender(electronBridge);
assert(desktopSender.isDesktop() === true, 'DesktopSender.isDesktop() must return true when bridge exists');
assert(desktopSender.getPlatform() === process.platform, 'DesktopSender.getPlatform() must match host platform');

// Window controls
ipcEvents.send = [];
desktopSender.minimize();
assert(ipcEvents.send.some(s => s.channel === 'window:minimize'), 'desktopSender.minimize() must dispatch window:minimize');

ipcEvents.send = [];
desktopSender.maximize();
assert(ipcEvents.send.some(s => s.channel === 'window:maximize'), 'desktopSender.maximize() must dispatch window:maximize');

ipcEvents.send = [];
desktopSender.close();
assert(ipcEvents.send.some(s => s.channel === 'window:close'), 'desktopSender.close() must dispatch window:close');

// Local server queries
desktopSender.getLocalServerPort().then(port => {
  assert(port === 8080, `Expected server port 8080, got ${port}`);
});

desktopSender.isLocalServerReachable().then(reachable => {
  assert(reachable === true, `Expected server reachable true, got ${reachable}`);
});

console.log('✓ 3.5 Passed: DesktopSender bridge proxies methods and queries correctly.');

console.log('\n=== Suite 3: All Electron Preload & Sender Security Tests Passed Cleanly ===\n');
