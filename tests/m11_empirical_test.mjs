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

function getRelativePaths(dir, baseDir = dir) {
  let results = [];
  const list = fs.readdirSync(dir, { withFileTypes: true });
  for (const dirent of list) {
    const fullPath = path.join(dir, dirent.name);
    const relPath = path.relative(baseDir, fullPath).replace(/\\/g, '/');
    if (dirent.name === '.git' || dirent.name === 'node_modules' || dirent.name === '.agents') {
      continue;
    }
    if (dirent.isDirectory()) {
      results.push({ type: 'dir', relPath, fullPath });
      results = results.concat(getRelativePaths(fullPath, baseDir));
    } else {
      results.push({ type: 'file', relPath, fullPath });
    }
  }
  return results;
}

console.log('====================================================');
console.log('Milestone 11 Parity & Stress Testing Suite');
console.log('Target: Seanime Studio vs Yourant');
console.log('====================================================\n');

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

// 1. Source existence & git inspection
console.log('--- 1. Seanime Source Verification ---');
assert(fs.existsSync(SEANIME_DIR), 'Seanime directory exists at ' + SEANIME_DIR);
let gitCommit = 'unknown';
try {
  gitCommit = execSync('git rev-parse HEAD', { cwd: SEANIME_DIR }).toString().trim();
  console.log(`  Seanime HEAD Commit: ${gitCommit}`);
} catch (e) {
  console.log(`  Failed to get git commit: ${e.message}`);
}
assert(gitCommit.length === 40, 'Seanime git repository is intact', `Commit: ${gitCommit}`);

// 2. Top-level files mirroring
console.log('\n--- 2. Top-Level Root Files Parity ---');
const rootFilesSeanime = fs.readdirSync(SEANIME_DIR, { withFileTypes: true })
  .filter(d => d.isFile())
  .map(d => d.name);

console.log(`Found ${rootFilesSeanime.length} root files in Seanime:`, rootFilesSeanime);

for (const file of rootFilesSeanime) {
  const yourantFile = path.join(YOURANT_DIR, file);
  const seFile = path.join(SEANIME_DIR, file);
  const exists = fs.existsSync(yourantFile);
  assert(exists, `Root file '${file}' exists in Yourant`);
  if (exists) {
    const seHash = hashFile(seFile);
    const yrHash = hashFile(yourantFile);
    const seSize = fs.statSync(seFile).size;
    const yrSize = fs.statSync(yourantFile).size;
    assert(seHash === yrHash, `Root file '${file}' exact SHA256 match`, `Seanime: ${seSize}b, Yourant: ${yrSize}b`);
  }
}

// 3. Top-level directories parity
console.log('\n--- 3. Top-Level Root Directories Parity ---');
const rootDirsSeanime = fs.readdirSync(SEANIME_DIR, { withFileTypes: true })
  .filter(d => d.isDirectory() && d.name !== '.git')
  .map(d => d.name);

console.log(`Found ${rootDirsSeanime.length} root directories in Seanime:`, rootDirsSeanime);

for (const dir of rootDirsSeanime) {
  const yourantDir = path.join(YOURANT_DIR, dir);
  assert(fs.existsSync(yourantDir) && fs.statSync(yourantDir).isDirectory(), `Directory '${dir}/' exists in Yourant`);
}

// Check extra root dirs in Yourant
const rootDirsYourant = fs.readdirSync(YOURANT_DIR, { withFileTypes: true })
  .filter(d => d.isDirectory() && d.name !== '.git')
  .map(d => d.name);
const extraDirs = rootDirsYourant.filter(d => !rootDirsSeanime.includes(d));
console.log(`Extra root directories preserved in Yourant:`, extraDirs);
assert(extraDirs.includes('electron'), "Preserved 'electron/' directory exists");
assert(extraDirs.includes('frontend'), "Preserved 'frontend/' directory exists");
assert(extraDirs.includes('tests'), "Preserved 'tests/' directory exists");
assert(fs.existsSync(path.join(YOURANT_DIR, 'docs', 'yourant')), "Preserved legacy archive 'docs/yourant/' exists");

// 4. All Seanime Markdown (.md) files parity
console.log('\n--- 4. Markdown (.md) Parity (All Files) ---');
const allSeanime = getRelativePaths(SEANIME_DIR);
const seMdFiles = allSeanime.filter(item => item.type === 'file' && item.relPath.endsWith('.md'));
console.log(`Found ${seMdFiles.length} Markdown files in Seanime.`);

let mdMatches = 0;
for (const md of seMdFiles) {
  const yrPath = path.join(YOURANT_DIR, md.relPath);
  const exists = fs.existsSync(yrPath);
  assert(exists, `Markdown file '${md.relPath}' exists in Yourant`);
  if (exists) {
    const seHash = hashFile(md.fullPath);
    const yrHash = hashFile(yrPath);
    if (seHash === yrHash) {
      mdMatches++;
    } else {
      console.warn(`[WARN] Content mismatch for ${md.relPath}`);
    }
  }
}
assert(mdMatches === seMdFiles.length, `All ${seMdFiles.length} Seanime markdown files have identical hash parity (${mdMatches}/${seMdFiles.length})`);

// 5. Seanime Internal Packages Parity
console.log('\n--- 5. Internal Packages Parity ---');
const seInternalDir = path.join(SEANIME_DIR, 'internal');
const yrInternalDir = path.join(YOURANT_DIR, 'internal');

const sePackages = fs.readdirSync(seInternalDir, { withFileTypes: true })
  .filter(d => d.isDirectory())
  .map(d => d.name);

console.log(`Seanime has ${sePackages.length} packages under internal/`);

let missingInternalPkgs = [];
for (const pkg of sePackages) {
  const yrPkgPath = path.join(yrInternalDir, pkg);
  const exists = fs.existsSync(yrPkgPath) && fs.statSync(yrPkgPath).isDirectory();
  if (!exists) {
    missingInternalPkgs.push(pkg);
  }
}
assert(missingInternalPkgs.length === 0, `All ${sePackages.length} Seanime packages exist under internal/`, `Missing: ${missingInternalPkgs.join(', ')}`);

// Custom Yourant packages in internal
assert(fs.existsSync(path.join(yrInternalDir, 'ountsu')), "Custom package 'internal/ountsu' preserved");
assert(fs.existsSync(path.join(yrInternalDir, 'community')), "Custom package 'internal/community' preserved");

// Total packages in Yourant internal
const yrPackages = fs.readdirSync(yrInternalDir, { withFileTypes: true })
  .filter(d => d.isDirectory())
  .map(d => d.name);
console.log(`Yourant has ${yrPackages.length} packages under internal/ (53 Seanime + 2 Custom)`);
assert(yrPackages.length === sePackages.length + 2, `Yourant internal package count is exactly 55 (53 + 2)`);

// Check Go file counts across internal packages
let totalSeGoFiles = 0;
let totalYrGoFiles = 0;
for (const pkg of sePackages) {
  const seGo = fs.readdirSync(path.join(seInternalDir, pkg)).filter(f => f.endsWith('.go')).length;
  const yrGo = fs.existsSync(path.join(yrInternalDir, pkg)) 
    ? fs.readdirSync(path.join(yrInternalDir, pkg)).filter(f => f.endsWith('.go')).length 
    : 0;
  totalSeGoFiles += seGo;
  totalYrGoFiles += yrGo;
  if (seGo !== yrGo) {
    console.warn(`[WARN] File count mismatch in package ${pkg}: Seanime=${seGo}, Yourant=${yrGo}`);
  }
}
console.log(`Direct Go files in internal/*: Seanime=${totalSeGoFiles}, Yourant=${totalYrGoFiles}`);
assert(totalSeGoFiles === totalYrGoFiles, `Direct Go file counts match in all ${sePackages.length} internal packages`);

// 6. Embed Filesystem Verification
console.log('\n--- 6. Embed Filesystem Verification ---');
const mainGoPath = path.join(YOURANT_DIR, 'main.go');
const mainGoContent = fs.readFileSync(mainGoPath, 'utf8');
assert(mainGoContent.includes('//go:embed all:web'), "main.go embeds 'all:web'");
assert(mainGoContent.includes('//go:embed internal/icon/logo.png'), "main.go embeds 'internal/icon/logo.png'");

const webDir = path.join(YOURANT_DIR, 'web');
assert(fs.existsSync(webDir), "Directory 'web/' exists for embed");
const webFiles = fs.readdirSync(webDir);
assert(webFiles.length > 0, "'web/' directory is non-empty (contains placeholder or web assets)");

const logoPath = path.join(YOURANT_DIR, 'internal', 'icon', 'logo.png');
assert(fs.existsSync(logoPath), "'internal/icon/logo.png' exists");
if (fs.existsSync(logoPath)) {
  const origLogo = path.join(SEANIME_DIR, 'internal', 'icon', 'logo.png');
  assert(hashFile(origLogo) === hashFile(logoPath), "'logo.png' SHA256 matches Seanime's original logo");
}

// 7. Compilation & Test Stress Test
console.log('\n--- 7. Empirical Build & Unit Test Execution ---');
const goBin = 'C:\\Program Files\\Go\\bin\\go.exe';
const testBinPath = path.join(YOURANT_DIR, 'challenger_verify.exe');

let buildSuccess = false;
let buildError = null;
try {
  console.log('Compiling main.go with -tags=nosystray...');
  execSync(`"${goBin}" build -tags=nosystray -o "${testBinPath}" main.go`, {
    cwd: YOURANT_DIR,
    stdio: 'pipe'
  });
  buildSuccess = true;
} catch (e) {
  buildError = e.stderr ? e.stderr.toString() : e.message;
}

assert(buildSuccess, "Go backend successfully compiles via 'go build -tags=nosystray'", buildError);

if (fs.existsSync(testBinPath)) {
  const binStat = fs.statSync(testBinPath);
  console.log(`Generated binary size: ${(binStat.size / (1024 * 1024)).toFixed(2)} MB`);
  assert(binStat.size > 10 * 1024 * 1024, 'Binary size is substantial (>10MB indicative of full server bundle)');
  fs.unlinkSync(testBinPath);
  console.log('Cleaned up challenger_verify.exe');
}

// Test custom feature packages
let countsuTest = false;
try {
  execSync(`"${goBin}" test ./internal/ountsu/...`, { cwd: YOURANT_DIR, stdio: 'pipe' });
  countsuTest = true;
} catch (e) {
  console.error(e.stderr ? e.stderr.toString() : e.message);
}
assert(countsuTest, "internal/ountsu unit tests pass");

let communityTest = false;
try {
  execSync(`"${goBin}" test ./internal/community/...`, { cwd: YOURANT_DIR, stdio: 'pipe' });
  communityTest = true;
} catch (e) {
  console.error(e.stderr ? e.stderr.toString() : e.message);
}
assert(communityTest, "internal/community unit tests pass");

// 8. Custom Capabilities Verification
console.log('\n--- 8. Preserved Yourant Features & Electron ---');
const electronPkg = path.join(YOURANT_DIR, 'electron', 'package.json');
assert(fs.existsSync(electronPkg), "Electron package.json exists");
if (fs.existsSync(electronPkg)) {
  const epkg = JSON.parse(fs.readFileSync(electronPkg, 'utf8'));
  assert(!!epkg.repository, "Electron package.json contains repository field (CI fix)");
}
assert(fs.existsSync(path.join(YOURANT_DIR, 'electron', 'main.js')), "Electron main.js exists");
assert(fs.existsSync(path.join(YOURANT_DIR, 'electron', 'preload.js')), "Electron preload.js exists");
assert(fs.existsSync(path.join(YOURANT_DIR, 'electron', 'sender.js')), "Electron sender.js exists");

// Frontend preservation
assert(fs.existsSync(path.join(YOURANT_DIR, 'frontend', 'package.json')), "Frontend package.json exists");
assert(fs.existsSync(path.join(YOURANT_DIR, 'frontend', 'public', 'jassub-worker.js')), "Frontend jassub-worker.js exists");
assert(fs.existsSync(path.join(YOURANT_DIR, 'frontend', 'public', 'pgs-renderer.worker.js')), "Frontend pgs-renderer.worker.js exists");

console.log('\n====================================================');
console.log(`Stress Test Summary: PASS=${passCount}, FAIL=${failCount}`);
console.log('====================================================\n');

if (failCount > 0) {
  console.error('FAILURES DETECTED:');
  console.error(JSON.stringify(failures, null, 2));
  process.exit(1);
} else {
  console.log('ALL EMPIRICAL PARITY CHECKS PASSED PERFECTLY!');
  process.exit(0);
}
