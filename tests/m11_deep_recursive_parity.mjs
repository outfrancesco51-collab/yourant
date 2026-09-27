import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { execSync } from 'node:child_process';

const SEANIME_DIR = 'C:\\Users\\Francy\\Desktop\\seanime-studio';
const YOURANT_DIR = 'c:\\Users\\Francy\\Desktop\\Yourant';

function hashFile(filePath) {
  const buffer = fs.readFileSync(filePath);
  return crypto.createHash('sha256').update(buffer).digest('hex');
}

function getAllFiles(dir, baseDir = dir) {
  let files = [];
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  for (const entry of entries) {
    const full = path.join(dir, entry.name);
    const rel = path.relative(baseDir, full).replace(/\\/g, '/');
    if (entry.name === '.git' || entry.name === 'node_modules') continue;
    if (entry.isDirectory()) {
      files = files.concat(getAllFiles(full, baseDir));
    } else {
      files.push({ rel, full, size: fs.statSync(full).size });
    }
  }
  return files;
}

console.log('=== Deep Recursive Parity Scan ===');
const seFiles = getAllFiles(SEANIME_DIR);
console.log(`Total Seanime files scanned (excl .git, node_modules): ${seFiles.length}`);

let missingInYourant = [];
let mismatchHash = [];
let matched = 0;

for (let i = 0; i < seFiles.length; i++) {
  const item = seFiles[i];
  const yrPath = path.join(YOURANT_DIR, item.rel);
  if (!fs.existsSync(yrPath)) {
    missingInYourant.push(item.rel);
  } else {
    const yrStat = fs.statSync(yrPath);
    if (yrStat.size !== item.size) {
      mismatchHash.push({ rel: item.rel, reason: `size diff: se=${item.size}, yr=${yrStat.size}` });
    } else {
      const seH = hashFile(item.full);
      const yrH = hashFile(yrPath);
      if (seH !== yrH) {
        mismatchHash.push({ rel: item.rel, reason: 'sha256 mismatch' });
      } else {
        matched++;
      }
    }
  }
}

console.log(`Matched perfectly: ${matched}/${seFiles.length} (${((matched/seFiles.length)*100).toFixed(2)}%)`);
console.log(`Missing in Yourant: ${missingInYourant.length}`);
if (missingInYourant.length > 0) {
  console.log('Sample missing:', missingInYourant.slice(0, 10));
}
console.log(`Checksum/Size mismatches: ${mismatchHash.length}`);
if (mismatchHash.length > 0) {
  console.log('Sample mismatches:', mismatchHash.slice(0, 10));
}

// Check go mod verify
console.log('\n=== Go Module Verification ===');
try {
  const modVerify = execSync('& "C:\\Program Files\\Go\\bin\\go.exe" mod verify', { cwd: YOURANT_DIR }).toString().trim();
  console.log('go mod verify output: ' + (modVerify || 'all modules verified'));
} catch (e) {
  console.error('go mod verify failed:', e.message);
}

// Deep package buildability check
console.log('\n=== Deep Go Packages Check ===');
try {
  // Test building server package
  execSync('& "C:\\Program Files\\Go\\bin\\go.exe" build -tags=nosystray ./internal/core/...', { cwd: YOURANT_DIR, stdio: 'pipe' });
  console.log('[PASS] ./internal/core/... compiles cleanly');
} catch (e) {
  console.error('[FAIL] ./internal/core/... build failed:', e.stderr?.toString() || e.message);
}

try {
  execSync('& "C:\\Program Files\\Go\\bin\\go.exe" build -tags=nosystray ./internal/server/...', { cwd: YOURANT_DIR, stdio: 'pipe' });
  console.log('[PASS] ./internal/server/... compiles cleanly');
} catch (e) {
  console.error('[FAIL] ./internal/server/... build failed:', e.stderr?.toString() || e.message);
}
