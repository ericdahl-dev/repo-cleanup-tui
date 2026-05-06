import fs from "node:fs/promises";
import path from "node:path";
import { spawnSync } from "node:child_process";

export type PackageManager = "yarn" | "pnpm" | "npm" | "bun" | "unknown";

export type RepoScan = {
  repoPath: string;
  nodeModulesPath: string;
  manager: PackageManager;
  hasLockfile: boolean;
  inactiveDays: number | null;
  bytes: number;
  reinstallCommand: string;
  git: {
    branch: string | null;
    dirty: boolean;
    ahead: number;
    behind: number;
  };
};

export type ScanProgress = {
  directoriesScanned: number;
};

type ScanOptions = {
  onProgress?: (progress: ScanProgress) => void;
  onRepoFound?: (row: RepoScan) => void;
  onRepoUpdated?: (row: RepoScan) => void;
  progressInterval?: number;
  sizeConcurrency?: number;
  ignoreDirs?: string[];
};

const SKIP_DIRS = new Set([
  "node_modules",
  ".git",
  ".next",
  ".nuxt",
  ".turbo",
  "dist",
  "build",
  "coverage",
  "out",
  "target"
]);

const getGitMeta = (repoPath: string): RepoScan["git"] => {
  const branchResult = spawnSync("git", ["-C", repoPath, "branch", "--show-current"], { encoding: "utf8" });
  const branch = branchResult.status === 0 ? branchResult.stdout.trim() || null : null;

  const dirtyResult = spawnSync("git", ["-C", repoPath, "status", "--porcelain"], { encoding: "utf8" });
  const dirty = dirtyResult.status === 0 ? dirtyResult.stdout.trim().length > 0 : false;

  const aheadBehindResult = spawnSync(
    "git",
    ["-C", repoPath, "rev-list", "--left-right", "--count", "@{upstream}...HEAD"],
    { encoding: "utf8" }
  );
  if (aheadBehindResult.status !== 0) {
    return { branch, dirty, ahead: 0, behind: 0 };
  }
  const [behindRaw, aheadRaw] = aheadBehindResult.stdout.trim().split(/\s+/);
  const ahead = Number(aheadRaw ?? 0);
  const behind = Number(behindRaw ?? 0);
  return {
    branch,
    dirty,
    ahead: Number.isFinite(ahead) ? ahead : 0,
    behind: Number.isFinite(behind) ? behind : 0
  };
};

const detectManager = async (repoPath: string): Promise<{ manager: PackageManager; hasLockfile: boolean }> => {
  const locks: Array<{ file: string; manager: PackageManager }> = [
    { file: "yarn.lock", manager: "yarn" },
    { file: "pnpm-lock.yaml", manager: "pnpm" },
    { file: "package-lock.json", manager: "npm" },
    { file: "bun.lockb", manager: "bun" },
    { file: "bun.lock", manager: "bun" }
  ];

  for (const lock of locks) {
    try {
      await fs.access(path.join(repoPath, lock.file));
      return { manager: lock.manager, hasLockfile: true };
    } catch {
      // ignore
    }
  }

  return { manager: "unknown", hasLockfile: false };
};

const getReinstallCommand = (manager: PackageManager): string => {
  switch (manager) {
    case "yarn":
      return "yarn install --immutable";
    case "pnpm":
      return "pnpm install --frozen-lockfile";
    case "npm":
      return "npm ci";
    case "bun":
      return "bun install --frozen-lockfile";
    default:
      return "install command unknown";
  }
};

const isGitRepository = (repoPath: string): boolean => {
  const result = spawnSync("git", ["-C", repoPath, "rev-parse", "--is-inside-work-tree"], { encoding: "utf8" });
  return result.status === 0 && result.stdout.trim() === "true";
};

const getInactiveDays = (repoPath: string): number | null => {
  const result = spawnSync("git", ["-C", repoPath, "log", "-1", "--format=%ct"], { encoding: "utf8" });
  if (result.status !== 0) return null;
  const ts = Number(result.stdout.trim());
  if (!Number.isFinite(ts) || ts <= 0) return null;
  const days = (Date.now() / 1000 - ts) / 86400;
  return Math.max(0, Math.floor(days));
};

const getDirectorySize = async (dir: string): Promise<number> => {
  const du = spawnSync("du", ["-sk", dir], { encoding: "utf8" });
  if (du.status === 0) {
    const kb = Number(du.stdout.trim().split(/\s+/)[0]);
    if (Number.isFinite(kb) && kb >= 0) {
      return kb * 1024;
    }
  }

  let total = 0;
  const stack = [dir];
  while (stack.length) {
    const current = stack.pop()!;
    let entries;
    try {
      entries = await fs.readdir(current, { withFileTypes: true });
    } catch {
      continue;
    }
    for (const entry of entries) {
      const full = path.join(current, entry.name);
      if (entry.isDirectory()) {
        stack.push(full);
      } else if (entry.isFile()) {
        try {
          const stat = await fs.stat(full);
          total += stat.size;
        } catch {
          // ignore
        }
      }
    }
  }
  return total;
};

const createLimiter = (concurrency: number) => {
  let active = 0;
  const queue: Array<() => void> = [];

  const next = () => {
    active -= 1;
    const run = queue.shift();
    if (run) run();
  };

  return async <T>(task: () => Promise<T>): Promise<T> => {
    if (active >= concurrency) {
      await new Promise<void>((resolve) => queue.push(resolve));
    }
    active += 1;
    try {
      return await task();
    } finally {
      next();
    }
  };
};

export const findReposWithNodeModules = async (root: string, options: ScanOptions = {}): Promise<RepoScan[]> => {
  const scans: RepoScan[] = [];
  const queue = [root];
  let queueIndex = 0;
  let directoriesScanned = 0;
  const progressInterval = Math.max(1, options.progressInterval ?? 100);
  let lastReported = 0;
  const runSizeTask = createLimiter(Math.max(1, options.sizeConcurrency ?? 6));
  const sizeTasks: Array<Promise<void>> = [];
  const skipDirs = new Set([...SKIP_DIRS, ...(options.ignoreDirs ?? [])]);

  while (queueIndex < queue.length) {
    const current = queue[queueIndex]!;
    queueIndex += 1;
    directoriesScanned += 1;
    if (directoriesScanned === 1 || directoriesScanned - lastReported >= progressInterval) {
      lastReported = directoriesScanned;
      options.onProgress?.({ directoriesScanned });
    }
    let entries;
    try {
      entries = await fs.readdir(current, { withFileTypes: true });
    } catch {
      continue;
    }

    const names = new Set(entries.map((entry) => entry.name));
    const hasGitMarker = names.has(".git");
    const hasPackageJson = names.has("package.json");
    const hasNodeModules = names.has("node_modules");

    if (hasPackageJson && hasNodeModules && (hasGitMarker || isGitRepository(current))) {
      const nodeModulesPath = path.join(current, "node_modules");
      const { manager, hasLockfile } = await detectManager(current);
      const row: RepoScan = {
        repoPath: current,
        nodeModulesPath,
        manager,
        hasLockfile,
        inactiveDays: getInactiveDays(current),
        bytes: 0,
        reinstallCommand: getReinstallCommand(manager),
        git: getGitMeta(current)
      };
      scans.push(row);
      options.onRepoFound?.(row);

      sizeTasks.push(
        runSizeTask(async () => {
          row.bytes = await getDirectorySize(nodeModulesPath);
          options.onRepoUpdated?.(row);
        })
      );
      continue;
    }

    for (const entry of entries) {
      if (!entry.isDirectory()) continue;
      if (entry.name.startsWith(".")) continue;
      if (skipDirs.has(entry.name)) continue;
      queue.push(path.join(current, entry.name));
    }
  }

  if (directoriesScanned !== lastReported) {
    options.onProgress?.({ directoriesScanned });
  }
  await Promise.all(sizeTasks);

  return scans;
};
