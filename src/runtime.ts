import path from "node:path";
import fs from "node:fs/promises";
import { findReposWithNodeModules, type RepoScan, type ScanProgress } from "./scanner.js";
import { loadConfig, type AppConfig } from "./config.js";

const looksLikeScriptPath = (value: string): boolean => {
  const ext = path.extname(value);
  return ext === ".js" || ext === ".mjs" || ext === ".cjs" || ext === ".ts" || ext === ".tsx";
};

export const resolveRootPathFromArgv = (argv: string[] = process.argv, cwd: string = process.cwd()): string => {
  const scriptAwareArgs = argv.slice(2).filter((arg) => arg !== "--");
  const fallbackArg = argv[1];
  const candidate =
    scriptAwareArgs[0] ??
    (fallbackArg && fallbackArg !== "--" && !looksLikeScriptPath(fallbackArg) ? fallbackArg : undefined);
  return path.resolve(candidate ?? cwd);
};

const CACHE_FILE = ".repo-cleanup-tui-scan-cache.json";
const CACHE_VERSION = 1;
const MAX_CACHE_AGE_MS = 10 * 60 * 1000;
const LOCKFILES = ["yarn.lock", "pnpm-lock.yaml", "package-lock.json", "bun.lockb", "bun.lock"] as const;

type FileSig = { exists: boolean; mtimeMs: number | null };
type RowSig = {
  packageJson: FileSig;
  nodeModules: FileSig;
  lockfiles: Record<string, FileSig>;
};
type CachedRow = { row: RepoScan; sig: RowSig };
type CachePayload = {
  version: number;
  rootPath: string;
  createdAt: number;
  rows: CachedRow[];
};

const statSig = async (targetPath: string): Promise<FileSig> => {
  try {
    const stat = await fs.stat(targetPath);
    return { exists: true, mtimeMs: stat.mtimeMs };
  } catch {
    return { exists: false, mtimeMs: null };
  }
};

const buildRowSig = async (row: RepoScan): Promise<RowSig> => {
  const lockfiles: Record<string, FileSig> = {};
  await Promise.all(
    LOCKFILES.map(async (lock) => {
      lockfiles[lock] = await statSig(path.join(row.repoPath, lock));
    })
  );
  return {
    packageJson: await statSig(path.join(row.repoPath, "package.json")),
    nodeModules: await statSig(row.nodeModulesPath),
    lockfiles
  };
};

const sigEqual = (a: FileSig, b: FileSig): boolean => a.exists === b.exists && a.mtimeMs === b.mtimeMs;

const rowSigValid = async (cached: CachedRow): Promise<boolean> => {
  const current = await buildRowSig(cached.row);
  if (!sigEqual(current.packageJson, cached.sig.packageJson)) return false;
  if (!sigEqual(current.nodeModules, cached.sig.nodeModules)) return false;
  for (const lock of LOCKFILES) {
    if (!sigEqual(current.lockfiles[lock], cached.sig.lockfiles[lock])) return false;
  }
  return true;
};

const readCache = async (rootPath: string): Promise<CachePayload | null> => {
  const cachePath = path.join(rootPath, CACHE_FILE);
  try {
    const raw = await fs.readFile(cachePath, "utf8");
    const parsed = JSON.parse(raw) as CachePayload;
    if (parsed.version !== CACHE_VERSION) return null;
    if (parsed.rootPath !== rootPath) return null;
    if (!Array.isArray(parsed.rows)) return null;
    if (Date.now() - parsed.createdAt > MAX_CACHE_AGE_MS) return null;
    return parsed;
  } catch {
    return null;
  }
};

const writeCache = async (rootPath: string, rows: RepoScan[]): Promise<void> => {
  const cachePath = path.join(rootPath, CACHE_FILE);
  const cachedRows: CachedRow[] = await Promise.all(
    rows.map(async (row) => ({
      row,
      sig: await buildRowSig(row)
    }))
  );
  const payload: CachePayload = {
    version: CACHE_VERSION,
    rootPath,
    createdAt: Date.now(),
    rows: cachedRows
  };
  try {
    await fs.writeFile(cachePath, JSON.stringify(payload), "utf8");
  } catch {
    // ignore cache write failures
  }
};

export const loadRowsForRoot = async (
  rootPath: string,
  onProgress?: (progress: ScanProgress) => void,
  onRepoFound?: (row: RepoScan) => void,
  onRepoUpdated?: (row: RepoScan) => void,
  forceFullScan: boolean = false,
  ignoreDirs: string[] = []
): Promise<RepoScan[]> => {
  const cache = forceFullScan ? null : await readCache(rootPath);
  if (cache) {
    const valid = await Promise.all(cache.rows.map((cached) => rowSigValid(cached)));
    if (valid.every(Boolean)) {
      onProgress?.({ directoriesScanned: 0 });
      const rows = cache.rows.map((cached) => cached.row);
      rows.forEach((row) => onRepoFound?.(row));
      return rows;
    }
  }

  const rows = await findReposWithNodeModules(rootPath, { onProgress, onRepoFound, onRepoUpdated, ignoreDirs });
  await writeCache(rootPath, rows);
  return rows;
};

export const loadRowsFromArgv = async (
  argv: string[] = process.argv,
  cwd: string = process.cwd()
): Promise<{ rootPath: string; rows: RepoScan[] }> => {
  const rootPath = resolveRootPathFromArgv(argv, cwd);
  const rows = await loadRowsForRoot(rootPath);
  return { rootPath, rows };
};

export const resolveInitialRoot = (argv: string[], cwd: string, config: AppConfig): string => {
  const argRoot = resolveRootPathFromArgv(argv, cwd);
  const hasExplicit = argv.slice(2).filter((a) => a !== "--").length > 0;
  if (hasExplicit) return argRoot;
  return path.resolve(config.preferredRoot ?? argRoot);
};

export const loadRuntimeConfig = async (cwd: string = process.cwd()): Promise<AppConfig> => {
  return loadConfig(cwd);
};
