import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { test } from "node:test";
import assert from "node:assert/strict";
import { findReposWithNodeModules } from "./scanner.js";

const run = (args: string[], cwd: string) => {
  const result = spawnSync("git", args, { cwd, encoding: "utf8" });
  if (result.status !== 0) {
    throw new Error(result.stderr || result.stdout || `git ${args.join(" ")} failed`);
  }
};

const writeRepo = async (
  root: string,
  name: string,
  options: { lockfiles?: string[]; commitDate?: string } = {}
): Promise<string> => {
  const repoPath = path.join(root, name);
  await fs.mkdir(path.join(repoPath, "node_modules"), { recursive: true });
  await fs.writeFile(path.join(repoPath, "package.json"), "{}");
  for (const lockfile of options.lockfiles ?? []) {
    await fs.writeFile(path.join(repoPath, lockfile), "");
  }
  await fs.writeFile(path.join(repoPath, "node_modules", "file.txt"), "x");

  run(["init"], repoPath);
  run(["add", "."], repoPath);
  run(["config", "user.email", "test@example.com"], repoPath);
  run(["config", "user.name", "Test User"], repoPath);
  const env = {
    ...process.env,
    ...(options.commitDate ? { GIT_AUTHOR_DATE: options.commitDate, GIT_COMMITTER_DATE: options.commitDate } : {})
  };
  const result = spawnSync("git", ["commit", "-m", "init"], { cwd: repoPath, encoding: "utf8", env });
  if (result.status !== 0) {
    throw new Error(result.stderr || result.stdout || "git commit failed");
  }

  return repoPath;
};

test("findReposWithNodeModules detects package manager from lockfile priority", async () => {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), "repo-cleanup-test-"));
  try {
    await writeRepo(tempRoot, "repo-multi-lock", { lockfiles: ["yarn.lock", "package-lock.json"] });

    const rows = await findReposWithNodeModules(tempRoot);
    assert.equal(rows.length, 1);
    assert.equal(rows[0]?.manager, "yarn");
    assert.equal(rows[0]?.hasLockfile, true);
    assert.equal(rows[0]?.reinstallCommand, "yarn install --immutable");
    assert.ok((rows[0]?.git.branch?.length ?? 0) > 0);
    assert.equal(rows[0]?.git.ahead, 0);
    assert.equal(rows[0]?.git.behind, 0);
  } finally {
    await fs.rm(tempRoot, { recursive: true, force: true });
  }
});

test("findReposWithNodeModules computes inactivity days from git history", async () => {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), "repo-cleanup-test-"));
  try {
    await writeRepo(tempRoot, "repo-old", { lockfiles: ["pnpm-lock.yaml"], commitDate: "2020-01-01T00:00:00Z" });
    const rows = await findReposWithNodeModules(tempRoot);
    assert.equal(rows.length, 1);
    assert.equal(rows[0]?.manager, "pnpm");
    assert.ok((rows[0]?.inactiveDays ?? 0) > 365, "inactiveDays should reflect old commit");
  } finally {
    await fs.rm(tempRoot, { recursive: true, force: true });
  }
});

test("findReposWithNodeModules reports scan progress", async () => {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), "repo-cleanup-test-"));
  try {
    await writeRepo(tempRoot, "repo-progress", { lockfiles: ["yarn.lock"] });
    let progressEvents = 0;
    let lastCount = 0;
    await findReposWithNodeModules(tempRoot, {
      onProgress: ({ directoriesScanned }) => {
        progressEvents += 1;
        lastCount = directoriesScanned;
      }
    });
    assert.ok(progressEvents > 0);
    assert.ok(lastCount > 0);
  } finally {
    await fs.rm(tempRoot, { recursive: true, force: true });
  }
});

test("findReposWithNodeModules emits rows as they are found", async () => {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), "repo-cleanup-test-"));
  try {
    const repoA = await writeRepo(tempRoot, "repo-a", { lockfiles: ["yarn.lock"] });
    const repoB = await writeRepo(tempRoot, "repo-b", { lockfiles: ["pnpm-lock.yaml"] });
    const found: string[] = [];
    const rows = await findReposWithNodeModules(tempRoot, {
      onRepoFound: (row) => found.push(row.repoPath)
    });

    assert.equal(rows.length, 2);
    assert.equal(found.length, 2);
    assert.ok(found.includes(repoA));
    assert.ok(found.includes(repoB));
  } finally {
    await fs.rm(tempRoot, { recursive: true, force: true });
  }
});

test("findReposWithNodeModules emits size updates for discovered rows", async () => {
  const tempRoot = await fs.mkdtemp(path.join(os.tmpdir(), "repo-cleanup-test-"));
  try {
    await writeRepo(tempRoot, "repo-size-update", { lockfiles: ["yarn.lock"] });
    const updates: number[] = [];
    const rows = await findReposWithNodeModules(tempRoot, {
      onRepoUpdated: (row) => updates.push(row.bytes)
    });

    assert.equal(rows.length, 1);
    assert.ok(updates.length >= 1);
    assert.ok(updates.some((bytes) => bytes > 0));
    assert.ok((rows[0]?.bytes ?? 0) > 0);
  } finally {
    await fs.rm(tempRoot, { recursive: true, force: true });
  }
});
