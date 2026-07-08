#!/usr/bin/env node
const { spawn } = require('child_process');
const { existsSync } = require('fs');
const path = require('path');

const TRIPLE_MAP = {
  'linux-x64':   { triple: 'x86_64-unknown-linux-musl', pkg: 'okp-cli-linux-x64' },
  'linux-arm64': { triple: 'aarch64-unknown-linux-musl', pkg: 'okp-cli-linux-arm64' },
  'darwin-x64':  { triple: 'x86_64-apple-darwin', pkg: 'okp-cli-darwin-x64' },
  'darwin-arm64':{ triple: 'aarch64-apple-darwin', pkg: 'okp-cli-darwin-arm64' },
  'win32-x64':   { triple: 'x86_64-pc-windows-msvc', pkg: 'okp-cli-win32-x64' },
};

const arch = process.arch === 'x64' ? 'x64' : process.arch;
const key = `${process.platform}-${arch}`;
const info = TRIPLE_MAP[key];

if (!info) {
  console.error(`okp: unsupported platform ${key}`);
  process.exit(1);
}

function findBinary() {
  // Try platform-specific package first
  try {
    const pkgPath = path.dirname(require.resolve(`${info.pkg}/package.json`));
    const binPath = path.join(pkgPath, 'vendor', info.triple, 'bin', 'okp');
    if (existsSync(binPath)) return binPath;
  } catch (_) {}

  // Fallback: look in sibling directory (monorepo)
  const siblingPath = path.join(__dirname, '..', 'packages', info.pkg, 'vendor', info.triple, 'bin', 'okp');
  if (existsSync(siblingPath)) return siblingPath;

  console.error(`okp: binary not found. Reinstall: npm install -g okp-cli`);
  process.exit(1);
}

const bin = findBinary();
const child = spawn(bin, process.argv.slice(2), { stdio: 'inherit' });

child.on('exit', (code) => process.exit(code || 0));
child.on('error', (err) => { console.error(err); process.exit(1); });
