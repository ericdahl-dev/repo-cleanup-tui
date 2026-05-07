import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";

export type AppConfig = {
  roots: string[];
  ignore: string[];
  preferredRoot: string | null;
};

const DEFAULT_IGNORE = ["node_modules", ".git", ".next", ".nuxt", "dist", "build", "coverage"];

const getConfigPath = (): string => {
  return path.join(os.homedir(), ".config", "repo-cleanup-tui", "config.json");
};

export const getDefaultConfig = (cwd: string = process.cwd()): AppConfig => ({
  roots: [cwd],
  ignore: DEFAULT_IGNORE,
  preferredRoot: cwd
});

export const loadConfig = async (cwd: string = process.cwd()): Promise<AppConfig> => {
  const configPath = getConfigPath();
  try {
    const raw = await fs.readFile(configPath, "utf8");
    const parsed = JSON.parse(raw) as Partial<AppConfig>;
    const roots = Array.isArray(parsed.roots) && parsed.roots.length > 0 ? parsed.roots.map((r) => path.resolve(r)) : [cwd];
    const ignore = Array.isArray(parsed.ignore) ? parsed.ignore.filter((v): v is string => typeof v === "string") : DEFAULT_IGNORE;
    const preferredRoot =
      typeof parsed.preferredRoot === "string" && parsed.preferredRoot.length > 0 ? path.resolve(parsed.preferredRoot) : roots[0] ?? cwd;
    return { roots, ignore, preferredRoot };
  } catch {
    return getDefaultConfig(cwd);
  }
};

export const saveConfig = async (config: AppConfig): Promise<void> => {
  const configPath = getConfigPath();
  await fs.mkdir(path.dirname(configPath), { recursive: true });
  await fs.writeFile(configPath, JSON.stringify(config, null, 2), "utf8");
};

export const upsertRoot = (config: AppConfig, rootPath: string): AppConfig => {
  const resolved = path.resolve(rootPath);
  const roots = [resolved, ...config.roots.filter((root) => path.resolve(root) !== resolved)];
  return { ...config, roots, preferredRoot: resolved };
};
