import path from "node:path";
import React, { useEffect, useMemo, useRef, useState } from "react";
import { Box, Text, render, useApp, useInput } from "ink";
import { assessCleanupSafety, buildConfirmToken, executeCleanup, type CleanupAssessment } from "./cleanup.js";
import type { RepoScan } from "./scanner.js";
import { getDefaultConfig, loadConfig, saveConfig, upsertRoot, type AppConfig } from "./config.js";
import { loadRowsForRoot, resolveInitialRoot, loadRuntimeConfig, resolveRootPathFromArgv } from "./runtime.js";
import { filterAndSortRows, type SortMode } from "./view-state.js";
import { pathToFileURL } from "node:url";

type UiMode = "browse" | "preview" | "confirm" | "search" | "workspace";

const formatBytes = (bytes: number): string => {
  const gb = bytes / 1024 / 1024 / 1024;
  if (gb >= 1) return `${gb.toFixed(2)} GB`;
  const mb = bytes / 1024 / 1024;
  return `${mb.toFixed(0)} MB`;
};

const SPINNER_FRAMES = ["|", "/", "-", "\\"];
const PAGE_SIZE = 20;
const DISCOVERY_BAR_WIDTH = 24;
const SIZING_BAR_WIDTH = 24;

const buildIndeterminateBar = (frame: number, width: number): string => {
  const cells = Array.from({ length: width }, () => " ");
  const pos = frame % width;
  cells[pos] = "=";
  if (pos > 0) cells[pos - 1] = "-";
  if (pos + 1 < width) cells[pos + 1] = "-";
  return `[${cells.join("")}]`;
};

const buildRatioBar = (done: number, total: number, width: number): string => {
  if (total <= 0) return `[${" ".repeat(width)}]`;
  const clamped = Math.max(0, Math.min(done, total));
  const filled = Math.round((clamped / total) * width);
  return `[${"#".repeat(filled)}${" ".repeat(width - filled)}]`;
};

const App = ({ initialRoot }: { initialRoot: string }) => {
  const { exit } = useApp();
  const [rows, setRows] = useState<RepoScan[]>([]);
  const [loading, setLoading] = useState(true);
  const [currentRoot, setCurrentRoot] = useState(initialRoot);
  const [sortMode, setSortMode] = useState<SortMode>("size");
  const [minInactiveDays, setMinInactiveDays] = useState(90);
  const [showOnlySafe, setShowOnlySafe] = useState(true);
  const [showOnlyDirty, setShowOnlyDirty] = useState(false);
  const [showGitContext, setShowGitContext] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [workspaceInput, setWorkspaceInput] = useState("");
  const [selected, setSelected] = useState(0);
  const [directoriesScanned, setDirectoriesScanned] = useState(0);
  const [spinnerFrame, setSpinnerFrame] = useState(0);
  const [mode, setMode] = useState<UiMode>("browse");
  const [showHelp, setShowHelp] = useState(false);
  const [confirmInput, setConfirmInput] = useState("");
  const [auditLog, setAuditLog] = useState<string[]>([]);
  const [activeAssessment, setActiveAssessment] = useState<CleanupAssessment | null>(null);
  const [scanGeneration, setScanGeneration] = useState(0);
  const [forceFullScan, setForceFullScan] = useState(false);
  const [cleanupInProgress, setCleanupInProgress] = useState(false);
  const [cleanupSpinnerFrame, setCleanupSpinnerFrame] = useState(0);
  const [reposDiscovered, setReposDiscovered] = useState(0);
  const [reposSized, setReposSized] = useState(0);
  const discoveredRepoPathsRef = useRef<Set<string>>(new Set());
  const sizedRepoPathsRef = useRef<Set<string>>(new Set());
  const [config, setConfig] = useState<AppConfig>(getDefaultConfig(initialRoot));
  const appendAudit = (entry: string) => {
    const line = `${new Date().toISOString()} ${entry}`;
    console.log(line);
    setAuditLog((prev) => [line, ...prev].slice(0, 5));
  };

  useInput((input, key) => {
    if (key.escape || input === "q") {
      if (showHelp) {
        setShowHelp(false);
        return;
      }
      if (mode === "search" || mode === "workspace") {
        setMode("browse");
        setWorkspaceInput("");
        return;
      }
      if (mode !== "browse") {
        setMode("browse");
        setConfirmInput("");
        return;
      }
      exit();
      return;
    }
    const row = filtered[selected];
    if (mode === "search") {
      if (key.return) {
        setMode("browse");
        return;
      }
      if (key.backspace || key.delete) {
        setSearchQuery((prev) => prev.slice(0, -1));
        return;
      }
      if (input.length > 0 && !key.ctrl && !key.meta) {
        setSearchQuery((prev) => prev + input);
      }
      return;
    }
    if (mode === "workspace") {
      if (key.return) {
        const target = workspaceInput.trim().length > 0 ? path.resolve(workspaceInput.trim()) : currentRoot;
        const next = upsertRoot(config, target);
        setConfig(next);
        void saveConfig(next);
        setCurrentRoot(target);
        setMode("browse");
        setLoading(true);
        setDirectoriesScanned(0);
        setForceFullScan(true);
        setScanGeneration((prev) => prev + 1);
        setWorkspaceInput("");
        return;
      }
      if (key.backspace || key.delete) {
        setWorkspaceInput((prev) => prev.slice(0, -1));
        return;
      }
      if (input.length > 0 && !key.ctrl && !key.meta) {
        setWorkspaceInput((prev) => prev + input);
      }
      return;
    }
    if (mode === "confirm") {
      const token = row ? buildConfirmToken(row) : "";
      if (cleanupInProgress) {
        return;
      }
      if (key.return) {
        if (!row) return;
        if (confirmInput.trim() !== token) {
          appendAudit(`blocked cleanup ${row.repoPath} reason=bad-confirm-token`);
          return;
        }
        setCleanupInProgress(true);
        void executeCleanup(row, false)
          .then((result) => {
            if (!result.ok) {
              appendAudit(`blocked cleanup ${row.repoPath} reason=${result.reasons.join(",")}`);
              return;
            }
            appendAudit(`deleted ${result.deletedPath} restore="${result.restoreCommand}"`);
            setRows((prev) => prev.filter((item) => item.repoPath !== row.repoPath));
            setMode("browse");
            setConfirmInput("");
          })
          .catch((error) => appendAudit(`failed cleanup ${row.repoPath} reason=${String(error)}`))
          .finally(() => setCleanupInProgress(false));
        return;
      }
      if (key.backspace || key.delete) {
        setConfirmInput((prev) => prev.slice(0, -1));
        return;
      }
      if (input.length > 0 && !key.ctrl && !key.meta) {
        setConfirmInput((prev) => prev + input);
      }
      return;
    }
    if (mode === "preview") {
      if (input === "n") {
        setMode("browse");
        setConfirmInput("");
        return;
      }
      if (!row) return;
      if (input === "p") {
        void executeCleanup(row, true)
          .then((result) => {
            if (!result.ok) {
              appendAudit(`blocked dry-run ${row.repoPath} reason=${result.reasons.join(",")}`);
              return;
            }
            appendAudit(`dry-run ${result.deletedPath} restore="${result.restoreCommand}"`);
          })
          .catch((error) => appendAudit(`failed dry-run ${row.repoPath} reason=${String(error)}`));
        return;
      }
      if (input === "y") {
        if (!activeAssessment) {
          appendAudit(`blocked cleanup ${row.repoPath} reason=assessment-pending`);
          return;
        }
        if (!activeAssessment.ok) {
          appendAudit(`blocked cleanup ${row.repoPath} reason=${activeAssessment.reasons.join(",")}`);
          return;
        }
        setMode("confirm");
        setConfirmInput("");
      }
      return;
    }
    if (input === "s") setSortMode((prev) => (prev === "size" ? "inactive" : "size"));
    if (input === "f") setMinInactiveDays((prev) => (prev === 0 ? 30 : prev === 30 ? 90 : prev === 90 ? 180 : 0));
    if (input === "k") setShowOnlySafe((prev) => !prev);
    if (input === "d") setShowOnlyDirty((prev) => !prev);
    if (input === "g") setShowGitContext((prev) => !prev);
    if (input === "/") {
      setMode("search");
      return;
    }
    if (input === "c") {
      setSearchQuery("");
      return;
    }
    if (input === "w") {
      setMode("workspace");
      setWorkspaceInput(currentRoot);
      return;
    }
    if (input === "?") {
      setShowHelp((prev) => !prev);
      return;
    }
    if (input === "r") {
      setLoading(true);
      setDirectoriesScanned(0);
      setForceFullScan(true);
      setScanGeneration((prev) => prev + 1);
    }
    if (input === "x" && row) {
      setMode("preview");
      setConfirmInput("");
    }
    if (input === "[") setSelected((prev) => Math.max(0, prev - PAGE_SIZE));
    if (input === "]") setSelected((prev) => Math.min(filtered.length - 1, Math.max(0, prev + PAGE_SIZE)));
    if (key.downArrow || input === "j") setSelected((prev) => prev + 1);
    if (key.upArrow || input === "u") setSelected((prev) => Math.max(prev - 1, 0));
  });

  useEffect(() => {
    void loadRuntimeConfig(initialRoot).then((loaded) => {
      setConfig(loaded);
    });
  }, [initialRoot]);

  useEffect(() => {
    if (config.roots.length === 0) return;
    void loadConfig(currentRoot).then(setConfig);
  }, []);

  useEffect(() => {
    const run = async () => {
      setRows([]);
      setReposDiscovered(0);
      setReposSized(0);
      discoveredRepoPathsRef.current = new Set();
      sizedRepoPathsRef.current = new Set();
      const data = await loadRowsForRoot(
        currentRoot,
        ({ directoriesScanned: count }) => {
          setDirectoriesScanned(count);
        },
        (row) => {
          if (!discoveredRepoPathsRef.current.has(row.repoPath)) {
            discoveredRepoPathsRef.current.add(row.repoPath);
            setReposDiscovered(discoveredRepoPathsRef.current.size);
          }
          setRows((prev) => (prev.some((item) => item.repoPath === row.repoPath) ? prev : [...prev, row]));
        },
        (row) => {
          if (!sizedRepoPathsRef.current.has(row.repoPath) && row.bytes > 0) {
            sizedRepoPathsRef.current.add(row.repoPath);
            setReposSized(sizedRepoPathsRef.current.size);
          }
          setRows((prev) => prev.map((item) => (item.repoPath === row.repoPath ? { ...item, ...row } : item)));
        },
        forceFullScan,
        config.ignore
      );
      setRows(data);
      setReposDiscovered(data.length);
      setReposSized(data.filter((item) => item.bytes > 0).length);
      setLoading(false);
      setForceFullScan(false);
    };
    void run();
  }, [currentRoot, scanGeneration, forceFullScan, config.ignore]);

  useEffect(() => {
    if (!loading) return;
    const timer = setInterval(() => {
      setSpinnerFrame((prev) => (prev + 1) % SPINNER_FRAMES.length);
    }, 120);
    return () => clearInterval(timer);
  }, [loading]);

  useEffect(() => {
    if (!cleanupInProgress) return;
    const timer = setInterval(() => {
      setCleanupSpinnerFrame((prev) => (prev + 1) % SPINNER_FRAMES.length);
    }, 100);
    return () => clearInterval(timer);
  }, [cleanupInProgress]);

  const filtered = useMemo(
    () => filterAndSortRows(rows, { minInactiveDays, showOnlySafe, showOnlyDirty, sortMode, searchQuery }),
    [rows, minInactiveDays, showOnlySafe, showOnlyDirty, sortMode, searchQuery]
  );

  useEffect(() => {
    if (selected >= filtered.length) {
      setSelected(Math.max(0, filtered.length - 1));
    }
  }, [filtered.length, selected]);

  const totalReclaimable = filtered.reduce((sum, row) => sum + row.bytes, 0);
  const activeRow = filtered[selected];
  const confirmToken = activeRow ? buildConfirmToken(activeRow) : "";
  const pageStart = Math.max(0, Math.min(selected - Math.floor(PAGE_SIZE / 2), Math.max(filtered.length - PAGE_SIZE, 0)));
  const visibleRows = filtered.slice(pageStart, pageStart + PAGE_SIZE);

  useEffect(() => {
    if (!activeRow) {
      setActiveAssessment(null);
      return;
    }
    void assessCleanupSafety(activeRow).then(setActiveAssessment);
  }, [activeRow]);

  return (
    <Box flexDirection="column">
      <Text>Repo Cleanup TUI (safe preview)</Text>
      <Text>
        Found {rows.length} repos with node_modules | visible {filtered.length} | reclaimable {formatBytes(totalReclaimable)}
      </Text>
      <Text>
        Mode: {mode} | selected {filtered.length === 0 ? 0 : selected + 1}/{filtered.length}
      </Text>
      <Text color={loading ? "yellow" : "green"}>
        {loading
          ? `${SPINNER_FRAMES[spinnerFrame]} scanning ${directoriesScanned} dirs in ${currentRoot}`
          : `scan complete in ${currentRoot}`}
      </Text>
      {loading ? (
        <Text color="yellow">
          Discovery {buildIndeterminateBar(spinnerFrame, DISCOVERY_BAR_WIDTH)} {directoriesScanned} dirs | repos found {reposDiscovered}
        </Text>
      ) : null}
      {reposDiscovered > 0 ? (
        <Text color={reposSized === reposDiscovered ? "green" : "yellow"}>
          Sizing {buildRatioBar(reposSized, reposDiscovered, SIZING_BAR_WIDTH)} {reposSized}/{reposDiscovered}
        </Text>
      ) : null}
      <Text>
        Keys: q quit | ? help | / search | r rescan | x cleanup | j/down next | u/up prev
      </Text>
      {showHelp ? (
        <Box marginTop={1} flexDirection="column">
          <Text color="yellow">Full keymap</Text>
          <Text>
            s sort({sortMode}) | f inactivity({minInactiveDays === 0 ? "all" : `>=${minInactiveDays}d`}) | k safe-only(
            {showOnlySafe ? "on" : "off"}) | d dirty-only({showOnlyDirty ? "on" : "off"})
          </Text>
          <Text>
            / search | c clear search | [ prev-page | ] next-page | w workspace | g git-cols({showGitContext ? "on" : "off"}) |
            r force rescan | x cleanup
          </Text>
        </Box>
      ) : null}
      {mode === "search" ? <Text color="cyan">Search: {searchQuery}</Text> : null}
      {mode === "workspace" ? <Text color="cyan">Workspace path: {workspaceInput}</Text> : null}
      <Box marginTop={1} flexDirection="column">
        <Text color="cyan">      size | inactive | repo</Text>
        {visibleRows.map((row, idx) => {
          const absoluteIdx = pageStart + idx;
          const isSelected = absoluteIdx === selected;
          const rel = path.relative(currentRoot, row.repoPath) || ".";
          const inactive = row.inactiveDays === null ? "unknown" : `${row.inactiveDays}d`;
          return (
            <Text key={row.repoPath} color={isSelected ? "green" : undefined}>
              {isSelected ? ">" : " "} {formatBytes(row.bytes).padStart(8)} | {inactive.padStart(7)} | {rel}
            </Text>
          );
        })}
        {loading && filtered.length === 0 ? <Text color="yellow">Scanning... list will populate as soon as results arrive.</Text> : null}
        {!loading && filtered.length > PAGE_SIZE ? (
          <Text color="gray">
            showing {pageStart + 1}-{Math.min(pageStart + PAGE_SIZE, filtered.length)} of {filtered.length}
          </Text>
        ) : null}
      </Box>
      {activeRow ? (
        <Box marginTop={1} flexDirection="column">
          <Text color="cyan">Selected cleanup plan</Text>
          <Text>
            Repo: {activeRow.repoPath} | manager: {activeRow.manager} | safety: {activeRow.hasLockfile ? "safe" : "unsafe"} |
            dirty: {activeRow.git.dirty ? "yes" : "no"}{showGitContext ? ` | branch: ${activeRow.git.branch ?? "-"}` : ""}
          </Text>
          <Text>Delete: {activeRow.nodeModulesPath}</Text>
          <Text>Restore: (cd {activeRow.repoPath} && {activeRow.reinstallCommand})</Text>
          {mode === "browse" ? <Text color="yellow">Next: press x for preview, then p (dry-run) or y (confirm).</Text> : null}
        </Box>
      ) : loading ? (
        <Text>Waiting for first matching repo...</Text>
      ) : (
        <Text>No rows match filter.</Text>
      )}
      {mode === "preview" && activeRow && activeAssessment ? (
        <Box marginTop={1} flexDirection="column">
          <Text color="yellow">Preview (dry-run)</Text>
          <Text>Target: {activeRow.nodeModulesPath}</Text>
          <Text>
            Risk: {activeAssessment.riskLevel} | confidence: {activeAssessment.confidence} | guards:{" "}
            {activeAssessment.ok ? "ok" : activeAssessment.reasons.join(", ")}
          </Text>
          {activeAssessment.warnings.length > 0 ? <Text>Warnings: {activeAssessment.warnings.join(", ")}</Text> : null}
          <Text>Keys: p run dry-run | y continue to confirm | n cancel</Text>
        </Box>
      ) : null}
      {mode === "confirm" && activeRow ? (
        <Box marginTop={1} flexDirection="column">
          <Text color="red">Confirm cleanup: remove node_modules only (repository is not deleted).</Text>
          <Text>Action: delete {activeRow.nodeModulesPath}</Text>
          <Text color="yellow">Type the confirmation token exactly, then press Enter.</Text>
          <Text>Token: {confirmToken}</Text>
          <Text>Input: {confirmInput}</Text>
          {cleanupInProgress ? <Text color="yellow">{SPINNER_FRAMES[cleanupSpinnerFrame]} deleting node_modules...</Text> : null}
        </Box>
      ) : null}
      <Box marginTop={1} flexDirection="column">
        <Text color="green">Audit log</Text>
        {auditLog.length === 0 ? <Text>none yet</Text> : auditLog.map((line) => <Text key={line}>{line}</Text>)}
      </Box>
    </Box>
  );
};

const rootPath = resolveRootPathFromArgv(process.argv, process.cwd());
const defaultConfig = await loadRuntimeConfig(process.cwd());
const initialRoot = resolveInitialRoot(process.argv, process.cwd(), defaultConfig);

export const runTui = (root: string = initialRoot) => {
  const app = <App initialRoot={root} />;
  render(app);
};

if (import.meta.url === pathToFileURL(process.argv[1] ?? "").href) {
  runTui(rootPath);
}
