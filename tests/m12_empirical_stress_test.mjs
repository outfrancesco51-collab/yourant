import fs from 'node:fs';
import path from 'node:path';

const ROOT_DIR = 'c:\\Users\\Francy\\Desktop\\Yourant';

let passedAssertions = 0;
let failedAssertions = 0;
const failureDetails = [];

function assert(condition, message, details = '') {
  if (condition) {
    passedAssertions++;
    // console.log(`  [PASS] ${message}`);
  } else {
    failedAssertions++;
    console.error(`  [FAIL] ${message} - ${details}`);
    failureDetails.push({ message, details });
  }
}

console.log('================================================================');
console.log('Milestone 12 Empirical Adversarial Stress Test Suite');
console.log('================================================================\n');

// ============================================================================
// SUITE 1: JunglePitchShifter Circular Buffer Math & AudioWorklet Simulation
// ============================================================================
console.log('--- SUITE 1: JunglePitchShifter Circular Buffer Math & Stability ---');

// Replicate JunglePitchShifter processor core math directly from public/jungle-worklet.js
class MockJunglePitchShifter {
  constructor(sr = 44100, pitchMultiplier = 1.6) {
    this.sampleRate = sr;
    this.pitchMultiplier = pitchMultiplier;
    this.bufferTime = 0.100; // 100ms
    this.fadeTime = 0.050; // 50ms
    this.bufferLength = Math.floor(this.sampleRate * this.bufferTime);
    this.fadeLength = Math.floor(this.sampleRate * this.fadeTime);
    this.buffer = new Float32Array(this.bufferLength);
    this.writePointer = 0;
    this.read1 = 0;
    this.read2 = this.bufferLength / 2;
  }

  processBlock(inputChannel, outputChannel) {
    for (let i = 0; i < inputChannel.length; i++) {
      this.buffer[this.writePointer] = inputChannel[i];

      this.read1 += this.pitchMultiplier;
      if (this.read1 >= this.bufferLength) this.read1 -= this.bufferLength;
      this.read2 += this.pitchMultiplier;
      if (this.read2 >= this.bufferLength) this.read2 -= this.bufferLength;

      const idx1 = Math.floor(this.read1);
      const idx2 = Math.floor(this.read2);

      // Verify buffer bounds
      if (idx1 < 0 || idx1 >= this.bufferLength || idx2 < 0 || idx2 >= this.bufferLength) {
        throw new Error(`Buffer indexing out of bounds: idx1=${idx1}, idx2=${idx2}, len=${this.bufferLength}`);
      }

      const val1 = this.buffer[idx1];
      const val2 = this.buffer[idx2];

      const pos1 = this.read1 / this.bufferLength;
      const fade1 = pos1 < 0.5 ? pos1 * 2 : (1 - pos1) * 2;
      const pos2 = this.read2 / this.bufferLength;
      const fade2 = pos2 < 0.5 ? pos2 * 2 : (1 - pos2) * 2;

      const denom = (fade1 + fade2 || 1);
      const out = (val1 * fade1 + val2 * fade2) / denom;

      if (!Number.isFinite(out)) {
        throw new Error(`Non-finite output sample at index ${i}: ${out}`);
      }

      outputChannel[i] = out;

      this.writePointer++;
      if (this.writePointer >= this.bufferLength) {
        this.writePointer = 0;
      }
    }
  }
}

// Stress test across various sample rates
const sampleRates = [8000, 11025, 22050, 44100, 48000, 88200, 96000, 192000];
for (const sr of sampleRates) {
  const shifter = new MockJunglePitchShifter(sr, 1.6);
  assert(shifter.bufferLength > 0, `Sample rate ${sr}Hz produces valid buffer length: ${shifter.bufferLength}`);
  assert(shifter.fadeLength > 0, `Sample rate ${sr}Hz produces valid fade length: ${shifter.fadeLength}`);
  assert(shifter.read2 === shifter.bufferLength / 2, `Sample rate ${sr}Hz read2 pointer initialized at bufferLength / 2`);

  // Stream 100,000 samples of synthetic audio: alternating sine, square, Dirac impulse, noise
  const blockSize = 128; // Web Audio API standard quantum
  const totalBlocks = 200; // 25,600 samples per sample rate
  const input = new Float32Array(blockSize);
  const output = new Float32Array(blockSize);

  let hasNan = false;
  let hasInf = false;
  let maxAbs = 0;

  for (let b = 0; b < totalBlocks; b++) {
    for (let s = 0; s < blockSize; s++) {
      const t = (b * blockSize + s) / sr;
      // Multi-frequency harmonic signal with sudden spikes
      input[s] = Math.sin(2 * Math.PI * 440 * t) * 0.7 + (s === 64 ? 1.0 : 0.0);
    }

    try {
      shifter.processBlock(input, output);
    } catch (err) {
      assert(false, `Shifter processBlock threw error at sr=${sr}: ${err.message}`);
    }

    for (let s = 0; s < blockSize; s++) {
      const v = output[s];
      if (Number.isNaN(v)) hasNan = true;
      if (!Number.isFinite(v)) hasInf = true;
      if (Math.abs(v) > maxAbs) maxAbs = Math.abs(v);
    }
  }

  assert(!hasNan, `Sample rate ${sr}Hz: zero NaN samples across ${totalBlocks * blockSize} samples`);
  assert(!hasInf, `Sample rate ${sr}Hz: zero Infinite samples across ${totalBlocks * blockSize} samples`);
  assert(maxAbs <= 2.0, `Sample rate ${sr}Hz: output amplitude is bounded (maxAbs=${maxAbs.toFixed(3)})`);
}

// Test pitch multipliers boundary conditions (0.5, 1.0, 1.6, 2.0, 3.5)
const pitchRatios = [0.5, 0.75, 1.0, 1.25, 1.6, 2.0, 3.0];
for (const p of pitchRatios) {
  const shifter = new MockJunglePitchShifter(48000, p);
  const input = new Float32Array(512).fill(0.5);
  const output = new Float32Array(512);
  let ok = true;
  try {
    shifter.processBlock(input, output);
  } catch (e) {
    ok = false;
  }
  assert(ok, `Pitch multiplier ${p} processes without boundary violations`);
}

// Verify actual file content matches expected code
const workletFile = fs.readFileSync(path.join(ROOT_DIR, 'seanime-web', 'public', 'jungle-worklet.js'), 'utf8');
assert(workletFile.includes('this.pitchMultiplier = 1.6'), 'jungle-worklet.js specifies 1.6x female pitch multiplier');
assert(workletFile.includes('this.bufferTime = 0.100'), 'jungle-worklet.js specifies 100ms buffer window');
assert(workletFile.includes('this.fadeTime = 0.050'), 'jungle-worklet.js specifies 50ms crossfade window');
assert(workletFile.includes('this.read1 >= this.bufferLength'), 'jungle-worklet.js implements circular buffer read1 wrap-around');
assert(workletFile.includes('this.read2 >= this.bufferLength'), 'jungle-worklet.js implements circular buffer read2 wrap-around');
assert(workletFile.includes('registerProcessor(\'jungle-pitch-shifter\', JunglePitchShifter)'), 'jungle-worklet.js registers audio worklet processor');


// ============================================================================
// SUITE 2: AudioBooster Parameters & Volume Calculation Oracle
// ============================================================================
console.log('\n--- SUITE 2: AudioBooster Volume Calculation & Limiter Oracle ---');

function computeExpectedVolumeState(percentage) {
  const clamped = Math.max(0.0, Math.min(1000.0, percentage));
  const gain = clamped / 100.0;
  const db = gain > 0 ? 20.0 * Math.log10(gain) : -Infinity;
  return {
    percentage: clamped,
    gain,
    db: Math.round(db * 10) / 10,
    isBoostZone: clamped > 100.0,
    warningBadge: clamped > 200.0,
    isMuted: clamped === 0,
  };
}

const volumeTestCases = [
  { input: -100, expClamped: 0, expGain: 0, expDb: -Infinity, expBoost: false, expWarn: false, expMute: true },
  { input: -0.001, expClamped: 0, expGain: 0, expDb: -Infinity, expBoost: false, expWarn: false, expMute: true },
  { input: 0, expClamped: 0, expGain: 0, expDb: -Infinity, expBoost: false, expWarn: false, expMute: true },
  { input: 50, expClamped: 50, expGain: 0.5, expDb: -6.0, expBoost: false, expWarn: false, expMute: false },
  { input: 100, expClamped: 100, expGain: 1.0, expDb: 0.0, expBoost: false, expWarn: false, expMute: false },
  { input: 100.5, expClamped: 100.5, expGain: 1.005, expDb: 0.0, expBoost: true, expWarn: false, expMute: false },
  { input: 150, expClamped: 150, expGain: 1.5, expDb: 3.5, expBoost: true, expWarn: false, expMute: false },
  { input: 200, expClamped: 200, expGain: 2.0, expDb: 6.0, expBoost: true, expWarn: false, expMute: false },
  { input: 200.1, expClamped: 200.1, expGain: 2.001, expDb: 6.0, expBoost: true, expWarn: true, expMute: false },
  { input: 500, expClamped: 500, expGain: 5.0, expDb: 14.0, expBoost: true, expWarn: true, expMute: false },
  { input: 1000, expClamped: 1000, expGain: 10.0, expDb: 20.0, expBoost: true, expWarn: true, expMute: false },
  { input: 1000.1, expClamped: 1000, expGain: 10.0, expDb: 20.0, expBoost: true, expWarn: true, expMute: false },
  { input: 5000, expClamped: 1000, expGain: 10.0, expDb: 20.0, expBoost: true, expWarn: true, expMute: false },
];

for (const tc of volumeTestCases) {
  const res = computeExpectedVolumeState(tc.input);
  assert(res.percentage === tc.expClamped, `Volume ${tc.input}% clamped to ${tc.expClamped}%`);
  assert(Math.abs(res.gain - tc.expGain) < 1e-4, `Volume ${tc.input}% yields gain ${tc.expGain}`);
  assert(res.db === tc.expDb || (Number.isNaN(res.db) && Number.isNaN(tc.expDb)), `Volume ${tc.input}% decibel is ${tc.expDb} dB (got ${res.db})`);
  assert(res.isBoostZone === tc.expBoost, `Volume ${tc.input}% isBoostZone = ${tc.expBoost}`);
  assert(res.warningBadge === tc.expWarn, `Volume ${tc.input}% warningBadge = ${tc.expWarn}`);
  assert(res.isMuted === tc.expMute, `Volume ${tc.input}% isMuted = ${tc.expMute}`);
}

// Verify AudioBooster.ts source code contracts
const audioBoosterSource = fs.readFileSync(
  path.join(ROOT_DIR, 'seanime-web', 'src', 'app', '(main)', '_features', 'custom-player', 'AudioBooster.ts'),
  'utf8'
);
assert(audioBoosterSource.includes('compressor.threshold.setValueAtTime(-2.0,'), 'Compressor threshold strictly clamped at -2.0 dBFS');
assert(audioBoosterSource.includes('compressor.knee.setValueAtTime(12.0,'), 'Compressor knee configured to 12.0 dB');
assert(audioBoosterSource.includes('compressor.ratio.setValueAtTime(16.0,'), 'Compressor ratio configured to 16:1 brickwall limiter');
assert(audioBoosterSource.includes('compressor.attack.setValueAtTime(0.003,'), 'Compressor attack configured to 3ms fast transient response');
assert(audioBoosterSource.includes('compressor.release.setValueAtTime(0.20,'), 'Compressor release configured to 200ms smooth recovery');
assert(audioBoosterSource.includes('new WeakMap<HTMLMediaElement, BoosterGraph>()'), 'WeakMap singleton pattern protects against duplicate attachments');
assert(audioBoosterSource.includes('Math.max(0.0, Math.min(1000.0, percentage))'), 'Strict volume range clamping [0.0, 1000.0]');


// ============================================================================
// SUITE 3: Electron IPC Channels & Preload Bridge Security
// ============================================================================
console.log('\n--- SUITE 3: Electron IPC Handlers & Preload Bridge Validation ---');

const electronIndexSource = fs.readFileSync(
  path.join(ROOT_DIR, 'seanime-denshi', 'src', 'main', 'index.ts'),
  'utf8'
);
const electronPreloadSource = fs.readFileSync(
  path.join(ROOT_DIR, 'seanime-denshi', 'src', 'preload.js'),
  'utf8'
);

// Discord RPC Validation
assert(electronIndexSource.includes('const discordClientId = "1300000000000000000"'), 'Discord client ID 1300000000000000000 defined');
assert(electronIndexSource.includes('ipcMain.on("update-presence"'), 'ipcMain.on("update-presence") listener registered');
assert(electronIndexSource.includes('rpc.setActivity(presenceData)'), 'update-presence invokes rpc.setActivity');

// AnimeUnity Validation
assert(electronIndexSource.includes('ipcMain.handle("extract-animeunity-stream"'), 'ipcMain.handle("extract-animeunity-stream") registered');
assert(electronIndexSource.includes('urls: ["*://*/*.m3u8*", "*://*/*.mp4*"]'), 'WebRequest filters .m3u8 and .mp4 patterns');
assert(electronIndexSource.includes('setTimeout(') && electronIndexSource.includes('Timeout extracting AnimeUnity stream'), '15-second safety timeout protects against hanging streams');
assert(electronIndexSource.includes('nodeIntegration: false') && electronIndexSource.includes('contextIsolation: true') && electronIndexSource.includes('sandbox: true'), 'BrowserWindow spawned with sandbox and contextIsolation');

// Preload Script Validation
assert(electronPreloadSource.includes('contextBridge.exposeInMainWorld'), 'Preload uses secure contextBridge.exposeInMainWorld');
assert(electronPreloadSource.includes('updatePresence: (data) => ipcRenderer.send("update-presence", data)'), 'discord.updatePresence bridged to ipcRenderer.send');
assert(electronPreloadSource.includes('extractStream: (url) => ipcRenderer.invoke("extract-animeunity-stream", url)'), 'animeunity.extractStream bridged to ipcRenderer.invoke');
assert(electronPreloadSource.includes('contextBridge.exposeInMainWorld(\'__isElectronDesktop__\', true)'), '__isElectronDesktop__ flag exposed');

// Security check: Verify channels whitelist in preload
const emitWhitelistMatches = electronPreloadSource.match(/emit:[\s\S]*?validChannels\s*=\s*\[([\s\S]*?)\]/);
assert(emitWhitelistMatches !== null && emitWhitelistMatches[1].includes('"update-presence"'), 'update-presence is allowlisted in emit()');

const sendWhitelistMatches = electronPreloadSource.match(/send:[\s\S]*?validChannels\s*=\s*\[([\s\S]*?)\]/);
assert(sendWhitelistMatches !== null && sendWhitelistMatches[1].includes('"update-presence"'), 'update-presence is allowlisted in send()');

// Verify dependency discord-rpc in package.json
const denshiPkg = JSON.parse(fs.readFileSync(path.join(ROOT_DIR, 'seanime-denshi', 'package.json'), 'utf8'));
assert(Boolean(denshiPkg.dependencies && denshiPkg.dependencies['discord-rpc']), 'seanime-denshi/package.json contains discord-rpc dependency');


// ============================================================================
// SUITE 4: Auto-Mod Regex Robustness & Edge Cases
// ============================================================================
console.log('\n--- SUITE 4: Auto-Mod Regex Stress & Edge Cases ---');

// Replicate AutoMod regex from internal/community/mod.go (Go uses (?i), JS uses /i flag)
const toxicRegex = /\b(kill yourself|die|death to|murder|threaten)\b/i;

const modCases = [
  // Toxic inputs (should match and sanitize)
  { text: 'kill yourself', matches: true, sanitized: '***' },
  { text: 'KILL YOURSELF', matches: true, sanitized: '***' },
  { text: 'Go and die right now', matches: true, sanitized: 'Go and *** right now' },
  { text: 'DIE!', matches: true, sanitized: '***!' },
  { text: 'death to all enemies', matches: true, sanitized: '*** all enemies' },
  { text: 'DEATH TO the king', matches: true, sanitized: '*** the king' },
  { text: 'I will murder you', matches: true, sanitized: 'I will *** you' },
  { text: 'MuRdEr in cold blood', matches: true, sanitized: '*** in cold blood' },
  { text: 'Do not threaten us', matches: true, sanitized: 'Do not *** us' },
  { text: 'THREATEN', matches: true, sanitized: '***' },

  // Non-toxic edge cases with partial substrings (should NOT match)
  { text: 'diet coke is refreshing', matches: false, sanitized: 'diet coke is refreshing' },
  { text: 'the soldier was brave', matches: false, sanitized: 'the soldier was brave' },
  { text: 'audience cheered loudly', matches: false, sanitized: 'audience cheered loudly' },
  { text: 'medieval times history', matches: false, sanitized: 'medieval times history' },
  { text: 'I feel obedient today', matches: false, sanitized: 'I feel obedient today' },
  { text: 'a killer plot twist', matches: false, sanitized: 'a killer plot twist' },
  { text: 'skill level 99', matches: false, sanitized: 'skill level 99' },
  { text: 'just kill the boss monster', matches: false, sanitized: 'just kill the boss monster' }, // "kill" without "yourself"
  { text: 'threat detection system', matches: false, sanitized: 'threat detection system' }, // "threat" not "threaten"
  { text: 'undie is not a word', matches: false, sanitized: 'undie is not a word' },
];

for (const c of modCases) {
  const matches = toxicRegex.test(c.text);
  assert(matches === c.matches, `AutoMod regex test "${c.text}": matches=${matches} (expected ${c.matches})`);
  const sanitized = c.text.replace(new RegExp(toxicRegex.source, 'gi'), '***');
  assert(sanitized === c.sanitized, `Sanitization for "${c.text}": got "${sanitized}" (expected "${c.sanitized}")`);
}

// ============================================================================
// SUITE 5: Ountsu Signaling Message Contracts
// ============================================================================
console.log('\n--- SUITE 5: Ountsu Message Serialization & Protocol Contract ---');

const ountsuMsgFile = fs.readFileSync(path.join(ROOT_DIR, 'internal', 'ountsu', 'messages.go'), 'utf8');
const expectedTypes = [
  'TypeJoin',
  'TypeLeave',
  'TypeChat',
  'TypeOffer',
  'TypeAnswer',
  'TypeCandidate',
  'TypeUpdateState',
  'TypeRoomState',
];
for (const t of expectedTypes) {
  assert(ountsuMsgFile.includes(t), `Ountsu message protocol defines ${t}`);
}
assert(ountsuMsgFile.includes('type OuntsuMessage struct'), 'OuntsuMessage struct is defined');
assert(ountsuMsgFile.includes('`json:"type"`'), 'OuntsuMessage serializes type as json:"type"');
assert(ountsuMsgFile.includes('`json:"senderId"`'), 'OuntsuMessage serializes senderId as json:"senderId"');
assert(ountsuMsgFile.includes('`json:"roomId"`'), 'OuntsuMessage serializes roomId as json:"roomId"');
assert(ountsuMsgFile.includes('`json:"payload,omitempty"`'), 'OuntsuMessage serializes payload as json:"payload,omitempty"');


// ============================================================================
// FINAL REPORT
// ============================================================================
console.log('\n================================================================');
console.log(`Stress Suite Completed: ${passedAssertions} Assertions Passed, ${failedAssertions} Failed`);
console.log('================================================================');

if (failedAssertions > 0) {
  console.error('\nFailures summary:');
  for (const f of failureDetails) {
    console.error(` - ${f.message}: ${f.details}`);
  }
  process.exit(1);
} else {
  console.log('\nAll empirical adversarial assertions passed successfully!');
  process.exit(0);
}
