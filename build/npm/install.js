'use strict';
// install.js downloads the prebuilt confluence-cli binary that matches the host
// platform from the matching GitHub Release. It runs as the npm `postinstall`
// script, and is also called lazily by the bin shim when the binary is missing
// (so installs with `--ignore-scripts` still work on first run).

const fs = require('fs');
const path = require('path');
const https = require('https');
const crypto = require('crypto');

const pkg = require('./package.json');

const REPO = 'angelmsger/confluence-cli';

const goosByPlatform = { darwin: 'darwin', linux: 'linux', win32: 'windows' };
const goarchByArch = { x64: 'amd64', arm64: 'arm64' };

// assetName returns the release asset file name for the current platform.
function assetName() {
  const goos = goosByPlatform[process.platform];
  const goarch = goarchByArch[process.arch];
  if (!goos || !goarch) {
    throw new Error(
      `unsupported platform ${process.platform}/${process.arch}; ` +
        `build from source instead (see https://github.com/${REPO})`
    );
  }
  return `confluence-cli-${goos}-${goarch}` + (goos === 'windows' ? '.exe' : '');
}

// binPath returns the directory and file path for the installed binary.
function binPath() {
  const dir = path.join(__dirname, 'binary');
  const exe = process.platform === 'win32' ? 'confluence-cli.exe' : 'confluence-cli';
  return { dir, file: path.join(dir, exe) };
}

// releaseBaseURL is the GitHub Release download prefix for this package version.
function releaseBaseURL() {
  return `https://github.com/${REPO}/releases/download/v${pkg.version}`;
}

// httpGet fetches a URL into a Buffer, following redirects.
function httpGet(url, redirects = 0) {
  return new Promise((resolve, reject) => {
    if (redirects > 8) {
      reject(new Error('too many redirects'));
      return;
    }
    https
      .get(url, { headers: { 'User-Agent': 'confluence-cli-npm-installer' } }, (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          res.resume();
          resolve(httpGet(res.headers.location, redirects + 1));
          return;
        }
        if (res.statusCode !== 200) {
          res.resume();
          reject(new Error(`GET ${url} -> HTTP ${res.statusCode}`));
          return;
        }
        const chunks = [];
        res.on('data', (c) => chunks.push(c));
        res.on('end', () => resolve(Buffer.concat(chunks)));
      })
      .on('error', reject);
  });
}

// expectedChecksum fetches checksums.txt and returns the SHA-256 for asset.
// Returns null when the checksum file is unavailable (verification skipped).
async function expectedChecksum(asset) {
  try {
    const text = (await httpGet(`${releaseBaseURL()}/checksums.txt`)).toString('utf8');
    for (const line of text.split('\n')) {
      const [hash, name] = line.trim().split(/\s+/);
      if (name === asset && hash) return hash.toLowerCase();
    }
  } catch {
    // No checksums published for this release; skip verification.
  }
  return null;
}

// install downloads, verifies and writes the binary. It is idempotent.
async function install() {
  if (!pkg.version || pkg.version === '0.0.0') {
    throw new Error('package version is unset; install from a published release');
  }
  const asset = assetName();
  const { dir, file } = binPath();

  const data = await httpGet(`${releaseBaseURL()}/${asset}`);

  const want = await expectedChecksum(asset);
  if (want) {
    const got = crypto.createHash('sha256').update(data).digest('hex');
    if (got !== want) {
      throw new Error(`checksum mismatch for ${asset} (want ${want}, got ${got})`);
    }
  }

  fs.mkdirSync(dir, { recursive: true });
  fs.writeFileSync(file, data, { mode: 0o755 });
  return file;
}

// welcomeText is the getting-started banner printed once at install time (by the
// postinstall script below). It is never printed by the CLI itself, so command
// output — JSON and everything else — is never touched.
function welcomeText() {
  return [
    '',
    'confluence-cli is ready. First-time setup:',
    '',
    '  confluence-cli config init --pretty   configure your server + credentials (interactive)',
    '  confluence-cli skill install          install the coding-agent Skill',
    '',
    'Everyday use:',
    '  confluence-cli search "<text>"',
    '  confluence-cli page get <pageId>',
    '  confluence-cli --help',
    '',
    'Docs: https://angelmsger.github.io/confluence-cli/',
    '',
  ].join('\n');
}

module.exports = { install, binPath, assetName, REPO };

// When run directly as the npm postinstall script, download best-effort: a
// failure here is not fatal because the bin shim retries lazily on first run.
// The getting-started banner is printed here (install time) and nowhere else.
// Note: npm v7+ hides postinstall output unless `npm install --foreground-scripts`
// is used, so this may not be visible on a default install.
if (require.main === module) {
  install()
    .then((file) => {
      process.stdout.write(`confluence-cli: installed ${file}\n`);
    })
    .catch((err) => {
      process.stderr.write(
        `confluence-cli: postinstall download skipped (${err.message}); ` +
          'the binary will be fetched on first run.\n'
      );
    })
    .finally(() => {
      if (!process.env.CI) process.stdout.write(welcomeText());
    });
}
