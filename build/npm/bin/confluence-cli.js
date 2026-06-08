#!/usr/bin/env node
'use strict';
// Thin launcher for the confluence-cli binary. It execs the platform binary
// downloaded by install.js, fetching it on demand if it is not present yet
// (e.g. when the package was installed with --ignore-scripts).

const fs = require('fs');
const { spawnSync } = require('child_process');
const { binPath, install, maybeWelcome } = require('../install.js');

async function main() {
  const { file } = binPath();

  if (!fs.existsSync(file)) {
    process.stderr.write('confluence-cli: downloading binary...\n');
    await install();
  }

  // First interactive run: print the getting-started banner once (no-op for
  // non-TTY / CI / agent use, and it only touches stderr).
  maybeWelcome();

  const res = spawnSync(file, process.argv.slice(2), { stdio: 'inherit' });
  if (res.error) {
    process.stderr.write(`confluence-cli: ${res.error.message}\n`);
    process.exit(1);
  }
  process.exit(res.status === null ? 1 : res.status);
}

main().catch((err) => {
  process.stderr.write(`confluence-cli: ${err.message}\n`);
  process.exit(1);
});
