// Smoke test for the npm bin shim (npm/root/bin/openrpc-linter.js).
//
// Verifies, on the CI runner's own platform, that:
//   1. the shim resolves the platform binary from node_modules and runs it
//      (exit 0, real output from the Go binary);
//   2. the binary's exit code is propagated back through the shim;
//   3. a missing optionalDependency produces a clear error (exit 1);
//   4. an unsupported platform produces a clear error (exit 1).
//
// No cross-compilation needed: each runner OS exercises its own host
// binary, which is exactly what an end user gets from `npm install`.
//
// Run: node --test npm/test/   (requires a prior `go build -o <bin> .`
// of the host binary, passed via the OPENRPC_LINTER_TEST_BINARY env var;
// tests 3 and 4 run even without a binary).

import { test } from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..", "..");
const shimPath = path.join(repoRoot, "npm", "root", "bin", "openrpc-linter.js");

// `go build -o X .` writes "X.exe" on Windows even though -o names "X";
// resolve whichever exists.
function resolveBinary() {
  const p = process.env.OPENRPC_LINTER_TEST_BINARY;
  if (!p) return null;
  if (fs.existsSync(p)) return p;
  if (process.platform === "win32" && fs.existsSync(`${p}.exe`)) return `${p}.exe`;
  return p; // missing -> copyFileSync throws with a clear ENOENT
}

const binaryPath = resolveBinary();

const PLATFORM_PACKAGES = {
  "darwin-arm64": "@open-rpc/openrpc-linter-darwin-arm64",
  "darwin-x64": "@open-rpc/openrpc-linter-darwin-x64",
  "linux-arm64": "@open-rpc/openrpc-linter-linux-arm64",
  "linux-x64": "@open-rpc/openrpc-linter-linux-x64",
  "win32-arm64": "@open-rpc/openrpc-linter-win32-arm64",
  "win32-x64": "@open-rpc/openrpc-linter-win32-x64",
};

function layout(pkgName, withBinary) {
  // Lay out node_modules the way npm would after `npm install`:
  // node_modules/@open-rpc/openrpc-linter/bin/openrpc-linter.js  (the shim)
  // node_modules/@open-rpc/openrpc-linter-<plat>/bin/openrpc-linter
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "shimtest-"));
  const scope = path.join(root, "node_modules", "@open-rpc");
  fs.mkdirSync(scope, { recursive: true });
  const binDir = path.join(scope, "openrpc-linter", "bin");
  fs.mkdirSync(binDir, { recursive: true });
  fs.copyFileSync(shimPath, path.join(binDir, "openrpc-linter.js"));

  if (withBinary) {
    assert.ok(binaryPath, "OPENRPC_LINTER_TEST_BINARY must point to a built host binary");
    // npm installs "@open-rpc/x" at node_modules/@open-rpc/x — scope already
    // lives in `scope`, so append only the unscoped part of the name.
    const pkgDir = path.join(scope, pkgName.replace(/^@open-rpc\//, ""));
    const pkgBin = path.join(pkgDir, "bin");
    fs.mkdirSync(pkgBin, { recursive: true });
    const binName = process.platform === "win32" ? "openrpc-linter.exe" : "openrpc-linter";
    fs.copyFileSync(binaryPath, path.join(pkgBin, binName));
    fs.chmodSync(path.join(pkgBin, binName), 0o755);
  }
  return path.join(binDir, "openrpc-linter.js");
}

const hostPkg = PLATFORM_PACKAGES[`${process.platform}-${process.arch}`];

test("shim dispatches to the platform binary and exits 0", () => {
  if (!hostPkg) {
    console.log(`# SKIP: ${process.platform}-${process.arch} not a published platform`);
    return;
  }
  const shim = layout(hostPkg, true);
  const out = execFileSync(process.execPath, [shim, "--help"], { encoding: "utf8" });
  assert.match(out, /openrpc-linter/);
  assert.match(out, /Usage:/);
});

test("shim propagates the binary's exit code", () => {
  if (!hostPkg) {
    console.log(`# SKIP: ${process.platform}-${process.arch} not a published platform`);
    return;
  }
  const shim = layout(hostPkg, true);
  let err = null;
  try {
    execFileSync(process.execPath, [shim, "--definitely-not-a-flag"], {
      encoding: "utf8",
      stdio: "pipe",
    });
  } catch (e) {
    err = e;
  }
  assert.ok(err, "shim must exit non-zero on unknown flag");
  assert.equal(err.status, 1, "unknown flag must surface as exit 1");
  // Must be the Go binary rejecting the flag, not the shim's
  // missing-optionalDependency error (which would mask a broken dispatch).
  assert.match(err.stderr, /unknown flag: --definitely-not-a-flag/);
  assert.doesNotMatch(err.stderr, /optional dependency/);
});

test("shim errors clearly when the optional dependency is missing", () => {
  const pkgName = hostPkg ?? "@open-rpc/openrpc-linter-linux-x64";
  const shim = layout(pkgName, false);
  let err = null;
  try {
    execFileSync(process.execPath, [shim, "--help"], { encoding: "utf8", stdio: "pipe" });
  } catch (e) {
    err = e;
  }
  assert.ok(err, "shim must exit non-zero when the binary is absent");
  assert.equal(err.status, 1);
  assert.match(err.stderr, /optional dependency/);
  assert.match(err.stderr, new RegExp(pkgName.replace("/", "\\/")));
});

test("shim errors clearly on an unsupported platform", () => {
  const shim = layout(hostPkg ?? "@open-rpc/openrpc-linter-linux-x64", false);
  const code = [
    'Object.defineProperty(process, "platform", { value: "haiku" });',
    'Object.defineProperty(process, "arch", { value: "mips" });',
    `require(${JSON.stringify(shim)});`,
  ].join("\n");
  let err = null;
  try {
    execFileSync(process.execPath, ["-e", code], {
      encoding: "utf8",
      stdio: "pipe",
    });
  } catch (e) {
    err = e;
  }
  assert.ok(err, "shim must exit non-zero on unsupported platform");
  assert.equal(err.status, 1);
  assert.match(err.stderr, /unsupported platform "haiku-mips"/);
});
