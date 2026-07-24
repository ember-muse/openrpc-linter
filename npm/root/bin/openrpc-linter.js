#!/usr/bin/env node
"use strict";

const { spawnSync } = require("node:child_process");

const PLATFORM_PACKAGES = {
  "darwin-arm64": "@open-rpc/openrpc-linter-darwin-arm64",
  "darwin-x64": "@open-rpc/openrpc-linter-darwin-x64",
  "linux-arm64": "@open-rpc/openrpc-linter-linux-arm64",
  "linux-x64": "@open-rpc/openrpc-linter-linux-x64",
  "win32-arm64": "@open-rpc/openrpc-linter-win32-arm64",
  "win32-x64": "@open-rpc/openrpc-linter-win32-x64",
};

const key = `${process.platform}-${process.arch}`;
const pkg = PLATFORM_PACKAGES[key];

if (!pkg) {
  console.error(
    `openrpc-linter: unsupported platform "${key}". ` +
      `Supported: ${Object.keys(PLATFORM_PACKAGES).join(", ")}`
  );
  process.exit(1);
}

const binName = process.platform === "win32" ? "openrpc-linter.exe" : "openrpc-linter";

let binPath;
try {
  binPath = require.resolve(`${pkg}/bin/${binName}`);
} catch (err) {
  console.error(
    `openrpc-linter: optional dependency "${pkg}" is missing.\n` +
      `Reinstall with npm (not yarn/pnpm --ignore-optional) so the platform binary can install, ` +
      `or install "${pkg}" directly.`
  );
  process.exit(1);
}

const result = spawnSync(binPath, process.argv.slice(2), { stdio: "inherit" });

if (result.error) {
  console.error(`openrpc-linter: failed to run binary: ${result.error.message}`);
  process.exit(1);
}

process.exit(result.status === null ? 1 : result.status);
