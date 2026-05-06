import path from "node:path";
import { test } from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import os from "node:os";
import { assessCleanupSafety, buildConfirmToken, evaluateCleanupGuards, executeCleanup } from "./cleanup.js";
import type { RepoScan } from "./scanner.js";

const row = (overrides: Partial<RepoScan> = {}): RepoScan => {
  const repoPath = "/tmp/demo-repo";
  return {
    repoPath,
    nodeModulesPath: path.join(repoPath, "node_modules"),
    manager: "yarn",
    hasLockfile: true,
    inactiveDays: 120,
    bytes: 1234,
    reinstallCommand: "yarn install --immutable",
    git: { branch: "main", dirty: false, ahead: 0, behind: 0 },
    ...overrides
  };
};

test("guard passes for repo/node_modules with lockfile", () => {
  const guard = evaluateCleanupGuards(row());
  assert.equal(guard.ok, true);
  assert.deepEqual(guard.reasons, []);
});

test("guard fails when lockfile missing", () => {
  const guard = evaluateCleanupGuards(row({ hasLockfile: false }));
  assert.equal(guard.ok, false);
  assert.match(guard.reasons.join(","), /missing lockfile/);
});

test("guard fails when target not node_modules", () => {
  const guard = evaluateCleanupGuards(row({ nodeModulesPath: "/tmp/demo-repo/dist" }));
  assert.equal(guard.ok, false);
  assert.match(guard.reasons.join(","), /target must be node_modules/);
});

test("guard fails when target is outside repo", () => {
  const guard = evaluateCleanupGuards(row({ nodeModulesPath: "/tmp/other/node_modules" }));
  assert.equal(guard.ok, false);
  assert.match(guard.reasons.join(","), /target must be inside repo/);
});

test("guard fails when target does not match repo/node_modules exactly", () => {
  const guard = evaluateCleanupGuards(row({ nodeModulesPath: "/tmp/demo-repo/sub/node_modules" }));
  assert.equal(guard.ok, false);
  assert.match(guard.reasons.join(","), /target must match repo\/node_modules/);
});

test("dry-run executes without touching fs", async () => {
  const result = await executeCleanup(row(), true);
  assert.equal(result.ok, true);
  assert.equal(result.dryRun, true);
  assert.equal(result.deletedPath, "/tmp/demo-repo/node_modules");
});

test("confirm token is explicit and repo-scoped", () => {
  assert.equal(buildConfirmToken(row()), "DELETE_NODE_MODULES demo-repo");
});

test("safety blocks unknown package manager", async () => {
  const assessment = await assessCleanupSafety(row({ manager: "unknown", hasLockfile: false }));
  assert.equal(assessment.ok, false);
  assert.equal(assessment.riskLevel, "high");
  assert.match(assessment.reasons.join(","), /unknown package manager/);
});

test("safety blocks yarn zero-install cache", async () => {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), "cleanup-risk-"));
  const repoPath = path.join(tempRoot, "repo");
  await fs.mkdir(path.join(repoPath, ".yarn", "cache"), { recursive: true });
  const assessment = await assessCleanupSafety(
    row({
      repoPath,
      nodeModulesPath: path.join(repoPath, "node_modules"),
      manager: "yarn",
      hasLockfile: true
    })
  );
  assert.equal(assessment.ok, false);
  assert.match(assessment.reasons.join(","), /\.yarn\/cache/);
  await fs.rm(tempRoot, { recursive: true, force: true });
});

test("safety warns for recently active repo", async () => {
  const assessment = await assessCleanupSafety(row({ inactiveDays: 2 }));
  assert.equal(assessment.ok, true);
  assert.equal(assessment.riskLevel, "medium");
  assert.match(assessment.warnings.join(","), /recently active/);
});
