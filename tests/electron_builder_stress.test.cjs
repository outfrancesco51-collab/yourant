/**
 * Empirical Adversarial & Stress Verification Suite:
 * electron-builder configuration, repository resolution, hosted-git-info,
 * and PublishManager CI detection.
 *
 * Authored by: m9_challenger_1 (Empirical Challenger)
 */

const assert = require('assert');
const path = require('path');
const fs = require('fs');
const { execSync } = require('child_process');

const projectRoot = path.resolve(__dirname, '..');
const electronDir = path.join(projectRoot, 'electron');

// Helper to reset hosted-git-info and app-builder-lib require caches between isolated test runs
function resetModuleCache() {
  const hgiPath = require.resolve('../electron/node_modules/hosted-git-info');
  const normPath = require.resolve('../electron/node_modules/app-builder-lib/out/util/normalizePackageData');
  const pkgMetaPath = require.resolve('../electron/node_modules/app-builder-lib/out/util/packageMetadata');
  const packagerPath = require.resolve('../electron/node_modules/app-builder-lib/out/packager');
  const repoInfoPath = require.resolve('../electron/node_modules/app-builder-lib/out/util/repositoryInfo');
  const pubManagerPath = require.resolve('../electron/node_modules/app-builder-lib/out/publish/PublishManager');

  delete require.cache[hgiPath];
  delete require.cache[normPath];
  delete require.cache[pkgMetaPath];
  delete require.cache[packagerPath];
  delete require.cache[repoInfoPath];
  delete require.cache[pubManagerPath];
}

let totalTests = 0;
let passedTests = 0;

function it(name, fn) {
  totalTests++;
  try {
    fn();
    console.log(`  ✓ [PASS] ${name}`);
    passedTests++;
  } catch (err) {
    console.error(`  ✗ [FAIL] ${name}`);
    console.error(`    Error: ${err.message}`);
    throw err;
  }
}

async function itAsync(name, fn) {
  totalTests++;
  try {
    await fn();
    console.log(`  ✓ [PASS] ${name}`);
    passedTests++;
  } catch (err) {
    console.error(`  ✗ [FAIL] ${name}`);
    console.error(`    Error: ${err.message}`);
    throw err;
  }
}

async function runAllSuites() {
  console.log('================================================================');
  console.log('=== Milestone 9 Empirical Challenger Stress Test Suite ===');
  console.log('================================================================\n');

  const hostedGitInfo = require('../electron/node_modules/hosted-git-info');
  const { getRepositoryInfo } = require('../electron/node_modules/app-builder-lib/out/util/repositoryInfo');
  const { getConfig, validateConfiguration } = require('../electron/node_modules/app-builder-lib/out/util/config/config');

  // =========================================================================
  // SUITE 1: Stress-testing hosted-git-info against repository strings
  // =========================================================================
  console.log('--- Suite 1: hosted-git-info & Repository Parsing Stress Tests ---');

  const pkgPath = path.join(electronDir, 'package.json');
  const pkg = JSON.parse(fs.readFileSync(pkgPath, 'utf8'));

  it('1.1: Verify current electron/package.json contains expected canonical URL', () => {
    assert.strictEqual(
      pkg.repository,
      'https://github.com/outfrancesco51-collab/yourant',
      'electron/package.json repository must match exact canonical URL'
    );
  });

  it('1.2: hosted-git-info parses exact current repository URL correctly', () => {
    const info = hostedGitInfo.fromUrl(pkg.repository);
    assert.ok(info != null, 'hostedGitInfo.fromUrl must not return null/undefined');
    assert.strictEqual(info.user, 'outfrancesco51-collab', 'user must be outfrancesco51-collab');
    assert.strictEqual(info.project, 'yourant', 'project must be yourant');
    assert.strictEqual(info.type, 'github', 'type must be github');
    assert.strictEqual(info.domain, 'github.com', 'domain must be github.com');

    // Check helper URL generators
    assert.strictEqual(info.browse(), 'https://github.com/outfrancesco51-collab/yourant');
    assert.strictEqual(info.bugs(), 'https://github.com/outfrancesco51-collab/yourant/issues');
    assert.strictEqual(info.docs(), 'https://github.com/outfrancesco51-collab/yourant#readme');
    assert.strictEqual(info.https(), 'git+https://github.com/outfrancesco51-collab/yourant.git');
    assert.strictEqual(info.ssh(), 'git@github.com:outfrancesco51-collab/yourant.git');
  });

  it('1.3: hosted-git-info handles valid alternative repository notations', () => {
    const validVariants = [
      'https://github.com/outfrancesco51-collab/yourant.git',
      'git@github.com:outfrancesco51-collab/yourant.git',
      'git+https://github.com/outfrancesco51-collab/yourant.git',
      'git+ssh://git@github.com/outfrancesco51-collab/yourant.git',
      'github:outfrancesco51-collab/yourant'
    ];

    for (const v of validVariants) {
      const parsed = hostedGitInfo.fromUrl(v);
      assert.ok(parsed != null, `Variant should be parsed: ${v}`);
      assert.strictEqual(parsed.user, 'outfrancesco51-collab', `User match for ${v}`);
      assert.strictEqual(parsed.project, 'yourant', `Project match for ${v}`);
      assert.strictEqual(parsed.type, 'github', `Type match for ${v}`);
    }
  });

  it('1.4: hosted-git-info rejects malformed / adversarial repository strings', () => {
    const malformedVariants = [
      '',
      '   ',
      'not-a-url',
      'http://',
      'https://github.com',
      'https://github.com/',
      'https://github.com/outfrancesco51-collab',
      'https://github.com/outfrancesco51-collab/',
      'https://github.com//yourant',
      'ftp://github.com/outfrancesco51-collab/yourant',
      'https://example.com/outfrancesco51-collab/yourant',
      'https://github.com/outfrancesco51-collab/yourant; calc.exe',
      'https://github.com/outfrancesco51-collab/yourant\r\nmalicious',
      'https://gіthub.com/outfrancesco51-collab/yourant', // Cyrillic homograph
      null,
      undefined,
      12345,
      true,
      {},
      []
    ];

    for (const m of malformedVariants) {
      let parsed = null;
      try {
        parsed = hostedGitInfo.fromUrl(m);
      } catch (e) {
        parsed = null;
      }
      if (parsed != null) {
        assert.ok(
          parsed.user !== 'outfrancesco51-collab' || parsed.project !== 'yourant' || parsed.type !== 'github',
          `Malformed variant should not produce valid target: ${m}`
        );
      } else {
        assert.ok(parsed == null, `Should return null/undefined for: ${m}`);
      }
    }
  });

  await itAsync('1.5: app-builder-lib getRepositoryInfo() resolves exact package.json metadata', async () => {
    // Clone metadata object so getRepositoryInfo does not mutate shared reference
    const metaClone = { ...pkg };
    const repoInfo = await getRepositoryInfo(electronDir, metaClone, null);
    assert.ok(repoInfo != null, 'getRepositoryInfo must not return null');
    assert.strictEqual(repoInfo.user, 'outfrancesco51-collab');
    assert.strictEqual(repoInfo.project, 'yourant');
    assert.strictEqual(repoInfo.type, 'github');
  });

  // =========================================================================
  // SUITE 2: PublishManager & CI Environment Simulation
  // =========================================================================
  console.log('\n--- Suite 2: PublishManager & CI Simulation Stress Tests ---');

  await itAsync('2.1: PublishManager successfully resolves GitHub config under simulated CI (GH_TOKEN set)', async () => {
    resetModuleCache();
    const { Packager } = require('../electron/node_modules/app-builder-lib');
    const { AppInfo } = require('../electron/node_modules/app-builder-lib/out/appInfo');
    const { PublishManager } = require('../electron/node_modules/app-builder-lib/out/publish/PublishManager');

    const savedGhToken = process.env.GH_TOKEN;
    const savedCi = process.env.CI;
    try {
      process.env.GH_TOKEN = 'ghp_mock_challenger_token_99999';
      process.env.CI = 'true';

      const packager = new Packager({ projectDir: electronDir });
      await packager.validateConfig();
      packager._appInfo = new AppInfo(packager, null);

      const publishManager = new PublishManager(packager, { publish: undefined });
      const configs = await publishManager.getGlobalPublishConfigurations();

      assert.ok(Array.isArray(configs), 'Configs must be an array');
      assert.strictEqual(configs.length, 1, 'Should resolve exactly 1 publish config');
      assert.strictEqual(configs[0].provider, 'github', 'Provider must be github');
      assert.strictEqual(configs[0].owner, 'outfrancesco51-collab', 'Owner must be outfrancesco51-collab');
      assert.strictEqual(configs[0].repo, 'yourant', 'Repo must be yourant');
    } finally {
      if (savedGhToken !== undefined) process.env.GH_TOKEN = savedGhToken; else delete process.env.GH_TOKEN;
      if (savedCi !== undefined) process.env.CI = savedCi; else delete process.env.CI;
    }
  });

  await itAsync('2.2: PublishManager successfully resolves GitHub config under simulated CI (GITHUB_TOKEN set)', async () => {
    resetModuleCache();
    const { Packager } = require('../electron/node_modules/app-builder-lib');
    const { AppInfo } = require('../electron/node_modules/app-builder-lib/out/appInfo');
    const { PublishManager } = require('../electron/node_modules/app-builder-lib/out/publish/PublishManager');

    const savedGhToken = process.env.GH_TOKEN;
    const savedGithubToken = process.env.GITHUB_TOKEN;
    const savedCi = process.env.CI;
    try {
      delete process.env.GH_TOKEN;
      process.env.GITHUB_TOKEN = 'ghp_mock_github_actions_token_88888';
      process.env.CI = 'true';

      const packager = new Packager({ projectDir: electronDir });
      await packager.validateConfig();
      packager._appInfo = new AppInfo(packager, null);

      const publishManager = new PublishManager(packager, { publish: undefined });
      const configs = await publishManager.getGlobalPublishConfigurations();

      assert.strictEqual(configs.length, 1);
      assert.strictEqual(configs[0].provider, 'github');
      assert.strictEqual(configs[0].owner, 'outfrancesco51-collab');
      assert.strictEqual(configs[0].repo, 'yourant');
    } finally {
      if (savedGhToken !== undefined) process.env.GH_TOKEN = savedGhToken; else delete process.env.GH_TOKEN;
      if (savedGithubToken !== undefined) process.env.GITHUB_TOKEN = savedGithubToken; else delete process.env.GITHUB_TOKEN;
      if (savedCi !== undefined) process.env.CI = savedCi; else delete process.env.CI;
    }
  });

  await itAsync('2.3: PublishManager triggers FATAL Error if repository is absent in package.json (Empirical Bug Reproduction)', async () => {
    resetModuleCache();
    const { Packager } = require('../electron/node_modules/app-builder-lib');
    const { AppInfo } = require('../electron/node_modules/app-builder-lib/out/appInfo');
    const { PublishManager } = require('../electron/node_modules/app-builder-lib/out/publish/PublishManager');
    const { Lazy } = require('../electron/node_modules/lazy-val');
    const { getRepositoryInfo: getRepoInfo } = require('../electron/node_modules/app-builder-lib/out/util/repositoryInfo');

    const savedGhToken = process.env.GH_TOKEN;
    const savedCi = process.env.CI;
    try {
      process.env.GH_TOKEN = 'ghp_mock_token_for_repro';
      process.env.CI = 'true';

      const packager = new Packager({ projectDir: electronDir });
      await packager.validateConfig();
      packager._appInfo = new AppInfo(packager, null);

      // Mutate packager metadata to simulate previous missing repository state
      delete packager.metadata.repository;
      delete packager.devMetadata.repository;
      packager._repositoryInfo = new Lazy(() => getRepoInfo(packager.projectDir, packager.metadata, packager.devMetadata));

      const publishManager = new PublishManager(packager, { publish: undefined });

      let errorThrown = null;
      try {
        await publishManager.getGlobalPublishConfigurations();
      } catch (err) {
        errorThrown = err;
      }

      assert.ok(errorThrown != null, 'PublishManager MUST throw error when repository is missing');
      assert.match(
        errorThrown.message,
        /Cannot detect repository by \.git\/config/,
        'Error message must match verbatim electron-builder failure string'
      );
    } finally {
      if (savedGhToken !== undefined) process.env.GH_TOKEN = savedGhToken; else delete process.env.GH_TOKEN;
      if (savedCi !== undefined) process.env.CI = savedCi; else delete process.env.CI;
    }
  });

  await itAsync('2.4: PublishManager triggers FATAL Error if repository is malformed in package.json', async () => {
    resetModuleCache();
    const { Packager } = require('../electron/node_modules/app-builder-lib');
    const { AppInfo } = require('../electron/node_modules/app-builder-lib/out/appInfo');
    const { PublishManager } = require('../electron/node_modules/app-builder-lib/out/publish/PublishManager');
    const { Lazy } = require('../electron/node_modules/lazy-val');
    const { getRepositoryInfo: getRepoInfo } = require('../electron/node_modules/app-builder-lib/out/util/repositoryInfo');

    const savedGhToken = process.env.GH_TOKEN;
    const savedCi = process.env.CI;
    try {
      process.env.GH_TOKEN = 'ghp_mock_token_for_repro';
      process.env.CI = 'true';

      const packager = new Packager({ projectDir: electronDir });
      await packager.validateConfig();
      packager._appInfo = new AppInfo(packager, null);

      // Mutate packager metadata to simulate malformed repository URL
      packager.metadata.repository = 'https://github.com/';
      packager.devMetadata.repository = 'https://github.com/';
      packager._repositoryInfo = new Lazy(() => getRepoInfo(packager.projectDir, packager.metadata, packager.devMetadata));

      const publishManager = new PublishManager(packager, { publish: undefined });

      let errorThrown = null;
      try {
        await publishManager.getGlobalPublishConfigurations();
      } catch (err) {
        errorThrown = err;
      }

      assert.ok(errorThrown != null, 'PublishManager MUST throw error when repository is malformed');
      assert.match(
        errorThrown.message,
        /Cannot detect repository by \.git\/config/
      );
    } finally {
      if (savedGhToken !== undefined) process.env.GH_TOKEN = savedGhToken; else delete process.env.GH_TOKEN;
      if (savedCi !== undefined) process.env.CI = savedCi; else delete process.env.CI;
    }
  });

  await itAsync('2.5: PublishManager behaves cleanly when CI tokens are omitted', async () => {
    resetModuleCache();
    const { Packager } = require('../electron/node_modules/app-builder-lib');
    const { AppInfo } = require('../electron/node_modules/app-builder-lib/out/appInfo');
    const { PublishManager } = require('../electron/node_modules/app-builder-lib/out/publish/PublishManager');

    const savedGhToken = process.env.GH_TOKEN;
    const savedGithubToken = process.env.GITHUB_TOKEN;
    const savedCi = process.env.CI;
    try {
      delete process.env.GH_TOKEN;
      delete process.env.GITHUB_TOKEN;
      process.env.CI = 'false';

      const packager = new Packager({ projectDir: electronDir });
      await packager.validateConfig();
      packager._appInfo = new AppInfo(packager, null);

      const publishManager = new PublishManager(packager, { publish: undefined });
      const configs = await publishManager.getGlobalPublishConfigurations();

      assert.ok(Array.isArray(configs), 'Configs must be an array');
      assert.strictEqual(configs.length, 0, 'No auto-publishers configured when tokens absent');
    } finally {
      if (savedGhToken !== undefined) process.env.GH_TOKEN = savedGhToken; else delete process.env.GH_TOKEN;
      if (savedGithubToken !== undefined) process.env.GITHUB_TOKEN = savedGithubToken; else delete process.env.GITHUB_TOKEN;
      if (savedCi !== undefined) process.env.CI = savedCi; else delete process.env.CI;
    }
  });

  // =========================================================================
  // SUITE 3: Schema Violations & Electron-Builder Configuration
  // =========================================================================
  console.log('\n--- Suite 3: Schema Validation & Electron-Builder Integrity ---');

  await itAsync('3.1: validateConfiguration() passes with zero errors on electron/package.json', async () => {
    const config = await getConfig(electronDir, null, null);
    assert.strictEqual(config.appId, 'app.yourant.desktop', 'appId must match app.yourant.desktop');
    assert.strictEqual(config.productName, 'Yourant', 'productName must match Yourant');
    assert.strictEqual(config.asar, true, 'asar must be true');

    const dummyLogger = { isEnabled: false, warn: console.warn };
    await validateConfiguration(config, dummyLogger);
  });

  await itAsync('3.2: validateConfiguration() actively rejects invalid properties (Probe Verification)', async () => {
    const dummyLogger = { isEnabled: false, warn: () => {} };
    let rejected = false;
    try {
      await validateConfiguration({ appId: 'test', adversarialBogusField: 123 }, dummyLogger);
    } catch (err) {
      rejected = true;
      assert.match(err.message, /adversarialBogusField/);
    }
    assert.strictEqual(rejected, true, 'Schema validator must actively reject unknown keys');
  });

  it('3.3: electron/package.json conforms to npm package specifications', () => {
    assert.ok(typeof pkg.name === 'string' && pkg.name.length > 0, 'name is required');
    assert.ok(typeof pkg.version === 'string' && /^\d+\.\d+\.\d+/.test(pkg.version), 'semver version is required');
    assert.ok(typeof pkg.main === 'string', 'main entrypoint is required');

    const entryFile = path.join(electronDir, pkg.main);
    assert.ok(fs.existsSync(entryFile), `Main entrypoint file must exist at ${entryFile}`);

    // Verify scripts
    assert.ok(pkg.scripts.build, 'build script required');
    assert.ok(pkg.scripts['build:win'], 'build:win script required');
    assert.ok(pkg.scripts['build:linux'], 'build:linux script required');
    assert.ok(pkg.scripts['build:mac'], 'build:mac script required');

    // Verify dependencies and devDependencies
    assert.ok(pkg.devDependencies['electron-builder'], 'electron-builder devDependency required');
    assert.ok(pkg.devDependencies['electron'], 'electron devDependency required');
  });

  it('3.4: electron build config file mappings exist or match valid patterns', () => {
    const files = pkg.build.files;
    assert.ok(Array.isArray(files), 'build.files must be an array');
    assert.ok(files.includes('main.js'), 'files must include main.js');
    assert.ok(files.includes('preload.js'), 'files must include preload.js');
    assert.ok(files.includes('sender.js'), 'files must include sender.js');
    assert.ok(files.includes('package.json'), 'files must include package.json');

    // Verify preload and sender exist on disk
    assert.ok(fs.existsSync(path.join(electronDir, 'preload.js')), 'preload.js exists');
    assert.ok(fs.existsSync(path.join(electronDir, 'sender.js')), 'sender.js exists');
  });

  // =========================================================================
  // SUITE 4: Git Tracking & Remote Push Audit
  // =========================================================================
  console.log('\n--- Suite 4: Git Tracking & Remote Deployment Audit ---');

  it('4.1: Verify git status is clean regarding source files', () => {
    const status = execSync('git status --porcelain', { cwd: projectRoot, encoding: 'utf8' });
    const lines = status.split(/\r?\n/).filter(Boolean);
    for (const line of lines) {
      // Allow untracked tests or dist
      const file = line.substring(3).trim();
      assert.ok(
        file.startsWith('electron/dist') || file.startsWith('.agents/teamwork') || file.startsWith('tests/electron_builder_stress'),
        `Unexpected unstaged/untracked project file in working tree: ${file}`
      );
    }
  });

  it('4.2: Latest commit incorporates the CI fix with conventional commit format', () => {
    const log = execSync('git log -1 --pretty=%s', { cwd: projectRoot, encoding: 'utf8' }).trim();
    assert.strictEqual(
      log,
      'fix(ci): add repository field to electron package.json to fix electron-builder publish error',
      'Commit message must match conventional format'
    );
  });

  it('4.3: Local branch is up-to-date with remote origin/main', () => {
    const localSha = execSync('git rev-parse HEAD', { cwd: projectRoot, encoding: 'utf8' }).trim();
    const remoteRef = execSync('git ls-remote origin refs/heads/main', { cwd: projectRoot, encoding: 'utf8' }).trim();
    assert.ok(remoteRef.startsWith(localSha), `Remote ref (${remoteRef}) must match local HEAD (${localSha})`);
  });

  // =========================================================================
  // SUMMARY
  // =========================================================================
  console.log('\n================================================================');
  console.log(`=== Summary: ${passedTests}/${totalTests} Tests Passed Cleanly (100%) ===`);
  console.log('================================================================\n');
}

runAllSuites().catch(err => {
  console.error('\nFatal suite execution error:', err);
  process.exit(1);
});
