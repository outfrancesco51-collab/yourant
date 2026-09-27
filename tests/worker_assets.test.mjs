import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const rootDir = path.resolve(__dirname, '..');

function assert(condition, message) {
  if (!condition) {
    throw new Error(`[ASSERTION FAILED] ${message}`);
  }
}

function sha256File(filePath) {
  const data = fs.readFileSync(filePath);
  return crypto.createHash('sha256').update(data).digest('hex');
}

console.log('=== Suite 1: Worker Assets & Vite Distribution Integrity Test ===');

const publicJassub = path.join(rootDir, 'frontend', 'public', 'jassub-worker.js');
const publicPgs = path.join(rootDir, 'frontend', 'public', 'pgs-renderer.worker.js');
const distJassub = path.join(rootDir, 'frontend', 'dist', 'jassub-worker.js');
const distPgs = path.join(rootDir, 'frontend', 'dist', 'pgs-renderer.worker.js');

// 1. Check frontend/public files exist and sizes
assert(fs.existsSync(publicJassub), `public/jassub-worker.js must exist at ${publicJassub}`);
const jassubStat = fs.statSync(publicJassub);
assert(jassubStat.size > 150000, `jassub-worker.js must be > 150KB (actual: ${jassubStat.size} bytes)`);

assert(fs.existsSync(publicPgs), `public/pgs-renderer.worker.js must exist at ${publicPgs}`);
const pgsStat = fs.statSync(publicPgs);
assert(pgsStat.size > 5000, `pgs-renderer.worker.js must be > 5KB (actual: ${pgsStat.size} bytes)`);

// 2. Content sanity check: check for key signatures from Seanime production worker assets
const jassubContent = fs.readFileSync(publicJassub, 'utf-8');
assert(jassubContent.includes('jassub-worker'), 'jassub-worker.js must include jassub-worker name check');
assert(jassubContent.includes('ASSRenderer') || jassubContent.includes('_draw'), 'jassub-worker.js must include ASSRenderer methods');
assert(jassubContent.includes('rawRender'), 'jassub-worker.js must invoke wasm rawRender');

const pgsContent = fs.readFileSync(publicPgs, 'utf-8');
assert(pgsContent.includes('OffscreenCanvas') || pgsContent.includes('offscreenCanvas'), 'pgs-renderer.worker.js must reference offscreenCanvas');
assert(pgsContent.includes('handleAddEvent') || pgsContent.includes('addEvents'), 'pgs-renderer.worker.js must handle PGS events');
assert(pgsContent.includes('handleRender') || pgsContent.includes('renderEvent'), 'pgs-renderer.worker.js must implement render loop');
assert(pgsContent.includes('setTimeOffset'), 'pgs-renderer.worker.js must support setTimeOffset');

// 3. Verify WASM & Font assets
const wasmPath = path.join(rootDir, 'frontend', 'public', 'jassub', 'jassub-worker.wasm');
const modernWasmPath = path.join(rootDir, 'frontend', 'public', 'jassub', 'jassub-worker-modern.wasm');
const fontPath = path.join(rootDir, 'frontend', 'public', 'fonts', 'Roboto-Medium.ttf');

assert(fs.existsSync(wasmPath), 'jassub-worker.wasm must exist');
assert(fs.statSync(wasmPath).size > 1500000, 'jassub-worker.wasm must be > 1.5MB');
assert(fs.existsSync(modernWasmPath), 'jassub-worker-modern.wasm must exist');
assert(fs.statSync(modernWasmPath).size > 1500000, 'jassub-worker-modern.wasm must be > 1.5MB');
assert(fs.existsSync(fontPath), 'Roboto-Medium.ttf must exist');
assert(fs.statSync(fontPath).size > 100000, 'Roboto-Medium.ttf must be > 100KB');

// 4. Verify Vite dist output parity
assert(fs.existsSync(distJassub), `dist/jassub-worker.js must exist at ${distJassub}`);
assert(fs.existsSync(distPgs), `dist/pgs-renderer.worker.js must exist at ${distPgs}`);

const hashPublicJassub = sha256File(publicJassub);
const hashDistJassub = sha256File(distJassub);
assert(hashPublicJassub === hashDistJassub, `SHA-256 mismatch for jassub-worker.js! public=${hashPublicJassub}, dist=${hashDistJassub}`);

const hashPublicPgs = sha256File(publicPgs);
const hashDistPgs = sha256File(distPgs);
assert(hashPublicPgs === hashDistPgs, `SHA-256 mismatch for pgs-renderer.worker.js! public=${hashPublicPgs}, dist=${hashDistPgs}`);

console.log('✓ Public and dist worker files verified:');
console.log(`  jassub-worker.js (${jassubStat.size} bytes) SHA-256: ${hashPublicJassub}`);
console.log(`  pgs-renderer.worker.js (${pgsStat.size} bytes) SHA-256: ${hashPublicPgs}`);
console.log('✓ WASM binaries and fallback font verified.');
console.log('=== Suite 1 Passed Cleanly ===\n');
