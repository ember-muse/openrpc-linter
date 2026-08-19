// Version-tracking guard for the npm packages.
//
// The published version is NOT stored in the repo: release.yml derives it from
// the git tag (github-tag-action -> new_tag -> VERSION env var, leading "v"
// stripped by publish.mjs). These placeholder files are always "0.0.0" and
// must stay that way.
//
// This test fails when:
//   - any version was hand-edited away from the 0.0.0 placeholder,
//   - the 6 platform directories and the root's optionalDependencies get out
//     of sync,
//   - the platform table in publish.mjs drifts from the on-disk directories,
//   - a package's os/cpu fields stop matching its directory name.
//
// Run: node --test npm/test/versions.test.mjs

import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, "..", "..");
const npmRoot = path.join(repoRoot, "npm");

function readPkgJson(name) {
  const p = JSON.parse(fs.readFileSync(path.join(npmRoot, name, "package.json"), "utf8"));
  return p;
}

const PLACEHOLDER = "0.0.0";

test("all npm package versions stay at the 0.0.0 placeholder", () => {
  const platformsDir = path.join(npmRoot, "platforms");
  const names = ["root", ...fs.readdirSync(platformsDir).map((d) => path.join("platforms", d))];
  for (const name of names) {
    const pkg = readPkgJson(name);
    assert.equal(
      pkg.version,
      PLACEHOLDER,
      `${name}/package.json version is ${pkg.version} — versions are git-tag-driven (see release.yml); keep the placeholder at ${PLACEHOLDER}`
    );
  }
});

test("root optionalDependencies match the platform directories exactly", () => {
  const root = readPkgJson("root");
  const dirs = fs.readdirSync(path.join(npmRoot, "platforms")).map((d) => {
    const pkg = JSON.parse(fs.readFileSync(path.join(npmRoot, "platforms", d, "package.json"), "utf8"));
    return { dir: d, name: pkg.name };
  });
  const expected = new Set(dirs.map((d) => d.name));
  const actual = new Set(Object.keys(root.optionalDependencies));
  assert.deepEqual(
    [...actual].sort(),
    [...expected].sort(),
    "root optionalDependencies must list exactly the published platform packages"
  );
});

test("os/cpu fields match each platform directory name", () => {
  for (const dir of fs.readdirSync(path.join(npmRoot, "platforms"))) {
    const pkg = readPkgJson(path.join("platforms", dir));
    const [osName, cpu] = dir.split("-");
    assert.deepEqual(pkg.os, [osName], `${dir}: os must be ["${osName}"]`);
    assert.deepEqual(pkg.cpu, [cpu], `${dir}: cpu must be ["${cpu}"]`);
  }
});

test("publish.mjs platform table covers the directories it publishes", () => {
  const publishSrc = fs.readFileSync(path.join(npmRoot, "scripts", "publish.mjs"), "utf8");
  const dirs = fs.readdirSync(path.join(npmRoot, "platforms"));
  for (const dir of dirs) {
    assert.ok(
      publishSrc.includes(`dir: "${dir}"`),
      `publish.mjs has no entry for the ${dir} directory`
    );
  }
  // every PLATFORMS entry must point at an existing directory
  for (const m of publishSrc.matchAll(/dir:\s*"([^"]+)"/g)) {
    assert.ok(
      fs.existsSync(path.join(npmRoot, "platforms", m[1])),
      `publish.mjs references missing platform directory "${m[1]}"`
    );
  }
});
