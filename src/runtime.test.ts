import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { test } from "node:test";
import assert from "node:assert/strict";
import { loadRowsForRoot } from "./runtime.js";

const run = (args: string[], cwd: string) => {
  const result = spawnSync("git", args, { cwd, encoding: "utf8" });
  if (result.status !== 0) {
    throw new Error(result.stderr || result.stdout || `git ${args.join(" ")} failed`);
  }
};

const writeRepo = async (root: string, name: string): Promise<string> => {
  const repoPath = path.join(root, name);
  await fs.mkdir(path.join(repoPath, "node_modules"), { recursive: true });
  await fs.writeFile(path.join(repoPath, "package.json"), "{}");
  await fs.writeFile(path.join(repoPath, "yarn.lock"), "");
  await fs.writeFile(path.join(repoPath, "node_modules", "file.txt"), "abc");
  run(["init"], repoPath);
  run(["add", "."], repoPath);
  run(["config", "user.email", "test@example.com"], repoPath);
  run(["config", "user.name", "Test User"], repoPath);
  run(["commit", "-m", "init"], repoPath);
  return repoPath;
};

test("loadRowsForRoot writes cache file after scan", async () => {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), "repo-cleanup-runtime-"));
  try {
    await writeRepo(tempRoot, "repo-cache");
    const rows = await loadRowsForRoot(tempRoot);
    assert.equal(rows.length, 1);
    const cachePath = path.join(tempRoot, ".repo-cleanup-tui-scan-cache.json");
    const cache = await fs.readFile(cachePath, "utf8");
    assert.ok(cache.includes("\"version\":1"));
  } finally {
    await fs.rm(tempRoot, { recursive: true, force: true });
  }
});

test("loadRowsForRoot invalidates cache when node_modules changes", async () => {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), "repo-cleanup-runtime-"));
  try {
    const repoPath = await writeRepo(tempRoot, "repo-invalidate");
    const firstRows = await loadRowsForRoot(tempRoot);
    assert.equal(firstRows.length, 1);
    const firstBytes = firstRows[0]?.bytes ?? 0;

    await fs.writeFile(path.join(repoPath, "node_modules", "big.bin"), "x".repeat(2 * 1024 * 1024));

    const secondRows = await loadRowsForRoot(tempRoot);
    assert.equal(secondRows.length, 1);
    const secondBytes = secondRows[0]?.bytes ?? 0;
    assert.ok(secondBytes > firstBytes, "bytes should increase after cache invalidation and rescan");
  } finally {
    await fs.rm(tempRoot, { recursive: true, force: true });
  }
});

test("loadRowsForRoot can force full scan even with warm cache", async () => {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), "repo-cleanup-runtime-"));
  try {
    await writeRepo(tempRoot, "repo-force");
    await loadRowsForRoot(tempRoot);

    let scannedDirs = 0;
    const rows = await loadRowsForRoot(
      tempRoot,
      ({ directoriesScanned }) => {
        scannedDirs = directoriesScanned;
      },
      undefined,
      undefined,
      true
    );

    assert.equal(rows.length, 1);
    assert.ok(scannedDirs > 0, "force full scan should traverse directories");
  } finally {
    await fs.rm(tempRoot, { recursive: true, force: true });
  }
});
