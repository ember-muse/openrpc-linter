#!/usr/bin/env node
// Publishes @open-rpc/openrpc-linter to npm as a root package plus one
// platform-specific optionalDependency package per goos/goarch, each carrying
// a single prebuilt binary. Mirrors the pattern esbuild/turbo/swc use instead
// of goreleaser's paid npm pipe.
//
// Run from the repo root, after `goreleaser release`/`build` has populated
// dist/artifacts.json. Requires VERSION (a leading "v", as produced by the
// tag step, is stripped automatically) and a registry auth token available
// to `npm publish` (e.g. via .npmrc / NODE_AUTH_TOKEN).
import fs from "node:fs";
import path from "node:path";
import { execFileSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..", "..");
const npmRoot = path.join(repoRoot, "npm");

const rawVersion = process.env.VERSION;
if (!rawVersion) {
  console.error("publish.mjs: VERSION env var is required (e.g. VERSION=1.2.3)");
  process.exit(1);
}
const version = rawVersion.replace(/^v/, "");

const dryRun = process.argv.includes("--dry-run");

// goos/goarch -> npm os/cpu + platform package dir name. Only pairs actually
// built by .goreleaser.yaml are listed; 386 is skipped as a publish target.
const PLATFORMS = [
  { goos: "linux", goarch: "amd64", dir: "linux-x64" },
  { goos: "linux", goarch: "arm64", dir: "linux-arm64" },
  { goos: "darwin", goarch: "amd64", dir: "darwin-x64" },
  { goos: "darwin", goarch: "arm64", dir: "darwin-arm64" },
  { goos: "windows", goarch: "amd64", dir: "win32-x64" },
  { goos: "windows", goarch: "arm64", dir: "win32-arm64" },
];

function readArtifacts() {
  const artifactsPath = path.join(repoRoot, "dist", "artifacts.json");
  if (!fs.existsSync(artifactsPath)) {
    console.error(`publish.mjs: ${artifactsPath} not found — run goreleaser first`);
    process.exit(1);
  }
  return JSON.parse(fs.readFileSync(artifactsPath, "utf8"));
}

function findBinary(artifacts, goos, goarch) {
  const match = artifacts.find(
    (a) => a.type === "Binary" && a.goos === goos && a.goarch === goarch
  );
  if (!match) return null;
  return path.join(repoRoot, match.path);
}

function npmPublish(dir) {
  const args = ["publish", "--access", "public"];
  if (dryRun) args.push("--dry-run");
  console.log(`> npm ${args.join(" ")} (in ${dir})`);
  execFileSync("npm", args, { cwd: dir, stdio: "inherit" });
}

function publishPlatformPackage(platform, binaryPath) {
  const pkgDir = path.join(npmRoot, "platforms", platform.dir);
  const binDir = path.join(pkgDir, "bin");
  fs.mkdirSync(binDir, { recursive: true });

  const binName = platform.goos === "windows" ? "openrpc-linter.exe" : "openrpc-linter";
  fs.copyFileSync(binaryPath, path.join(binDir, binName));
  fs.chmodSync(path.join(binDir, binName), 0o755);

  const pkgJsonPath = path.join(pkgDir, "package.json");
  const pkgJson = JSON.parse(fs.readFileSync(pkgJsonPath, "utf8"));
  pkgJson.version = version;
  fs.writeFileSync(pkgJsonPath, JSON.stringify(pkgJson, null, 2) + "\n");

  npmPublish(pkgDir);
}

function publishRootPackage() {
  const pkgDir = path.join(npmRoot, "root");
  const pkgJsonPath = path.join(pkgDir, "package.json");
  const pkgJson = JSON.parse(fs.readFileSync(pkgJsonPath, "utf8"));
  pkgJson.version = version;
  for (const name of Object.keys(pkgJson.optionalDependencies)) {
    pkgJson.optionalDependencies[name] = version;
  }
  fs.writeFileSync(pkgJsonPath, JSON.stringify(pkgJson, null, 2) + "\n");

  npmPublish(pkgDir);
}

function main() {
  const artifacts = readArtifacts();
  let publishedAny = false;

  for (const platform of PLATFORMS) {
    const binaryPath = findBinary(artifacts, platform.goos, platform.goarch);
    if (!binaryPath) {
      console.error(
        `publish.mjs: no built binary for ${platform.goos}/${platform.goarch}, skipping`
      );
      continue;
    }
    publishPlatformPackage(platform, binaryPath);
    publishedAny = true;
  }

  if (!publishedAny) {
    console.error("publish.mjs: no platform packages published, aborting root publish");
    process.exit(1);
  }

  publishRootPackage();
}

main();
