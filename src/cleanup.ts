import fs from "node:fs/promises";
import path from "node:path";
import type { RepoScan } from "./scanner.js";

export type CleanupGuard = {
  ok: boolean;
  reasons: string[];
};

export type CleanupRiskLevel = "low" | "medium" | "high";

export type CleanupAssessment = {
  ok: boolean;
  blocked: boolean;
  reasons: string[];
  warnings: string[];
  riskLevel: CleanupRiskLevel;
  confidence: "high" | "medium" | "low";
};

export type CleanupResult = {
  ok: boolean;
  dryRun: boolean;
  deletedPath: string;
  restoreCommand: string;
  reasons: string[];
  riskLevel: CleanupRiskLevel;
  confidence: "high" | "medium" | "low";
};

export const buildConfirmToken = (row: RepoScan): string => {
  return `DELETE_NODE_MODULES ${path.basename(row.repoPath)}`;
};

export const evaluateCleanupGuards = (row: RepoScan): CleanupGuard => {
  const reasons: string[] = [];
  const normalizedRepo = path.resolve(row.repoPath);
  const normalizedNodeModules = path.resolve(row.nodeModulesPath);
  const allowedNodeModulesPath = path.join(normalizedRepo, "node_modules");
  const inRepo = normalizedNodeModules.startsWith(`${normalizedRepo}${path.sep}`);

  if (!row.hasLockfile) reasons.push("missing lockfile");
  if (path.basename(normalizedNodeModules) !== "node_modules") reasons.push("target must be node_modules");
  if (!inRepo) reasons.push("target must be inside repo");
  if (normalizedNodeModules !== allowedNodeModulesPath) reasons.push("target must match repo/node_modules");

  return { ok: reasons.length === 0, reasons };
};

const exists = async (targetPath: string): Promise<boolean> => {
  try {
    await fs.access(targetPath);
    return true;
  } catch {
    return false;
  }
};

const getRiskLevel = (blocked: boolean, warnings: string[]): CleanupRiskLevel => {
  if (blocked) return "high";
  if (warnings.length > 0) return "medium";
  return "low";
};

const getConfidence = (row: RepoScan, blocked: boolean): "high" | "medium" | "low" => {
  if (blocked) return "high";
  if (row.manager === "unknown") return "low";
  if (!row.hasLockfile) return "medium";
  return "high";
};

export const assessCleanupSafety = async (row: RepoScan): Promise<CleanupAssessment> => {
  const guard = evaluateCleanupGuards(row);
  const reasons = [...guard.reasons];
  const warnings: string[] = [];
  let blocked = !guard.ok;

  if (row.manager === "unknown") {
    reasons.push("unknown package manager");
    blocked = true;
  }

  if (row.manager === "yarn") {
    const yarnCachePath = path.join(row.repoPath, ".yarn", "cache");
    if (await exists(yarnCachePath)) {
      reasons.push("yarn zero-install cache detected (.yarn/cache)");
      blocked = true;
    }
  }

  if (!row.hasLockfile) {
    reasons.push("missing lockfile");
    blocked = true;
  }

  if (row.inactiveDays !== null && row.inactiveDays < 7) {
    warnings.push("recently active repo (<7d)");
  }

  const riskLevel = getRiskLevel(blocked, warnings);
  const confidence = getConfidence(row, blocked);

  return {
    ok: !blocked,
    blocked,
    reasons: Array.from(new Set(reasons)),
    warnings,
    riskLevel,
    confidence
  };
};

export const executeCleanup = async (row: RepoScan, dryRun: boolean): Promise<CleanupResult> => {
  const assessment = await assessCleanupSafety(row);
  if (!assessment.ok) {
    return {
      ok: false,
      dryRun,
      deletedPath: row.nodeModulesPath,
      restoreCommand: row.reinstallCommand,
      reasons: assessment.reasons,
      riskLevel: assessment.riskLevel,
      confidence: assessment.confidence
    };
  }

  if (!dryRun) {
    await fs.rm(row.nodeModulesPath, { recursive: true, force: false });
  }

  return {
    ok: true,
    dryRun,
    deletedPath: row.nodeModulesPath,
    restoreCommand: row.reinstallCommand,
    reasons: [],
    riskLevel: assessment.riskLevel,
    confidence: assessment.confidence
  };
};
