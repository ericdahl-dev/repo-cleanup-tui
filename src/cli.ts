#!/usr/bin/env node
import path from "node:path";
import { runTui } from "./index.js";
import { loadConfig, saveConfig, upsertRoot } from "./config.js";
import { loadRowsForRoot, resolveRootPathFromArgv } from "./runtime.js";

const rawArgs = process.argv.slice(2);
const args = rawArgs.filter((arg) => arg !== "--");
const command = args[0];

const run = async () => {
  if (!command) {
    const config = await loadConfig(process.cwd());
    runTui(config.preferredRoot ?? process.cwd());
    return;
  }

  if (command === "init") {
    const cwd = process.cwd();
    const current = await loadConfig(cwd);
    const next = upsertRoot(current, cwd);
    await saveConfig(next);
    console.log(`Initialized config with root: ${cwd}`);
    return;
  }

  if (command === "scan") {
    const scanArgs = args.slice(1);
    const asJson = scanArgs.includes("--json");
    const rootArg = scanArgs.filter((arg) => arg !== "--json")[0];
    const rootPath = path.resolve(rootArg ?? process.cwd());
    const config = await loadConfig(process.cwd());
    const rows = await loadRowsForRoot(rootPath, undefined, undefined, undefined, false, config.ignore);
    if (asJson) {
      console.log(JSON.stringify({ rootPath, count: rows.length, rows }, null, 2));
      return;
    }
    rows.forEach((row) => {
      console.log(`${row.repoPath}\t${row.bytes}\t${row.manager}\t${row.inactiveDays ?? "unknown"}`);
    });
    return;
  }

  if (command === "tui") {
    const rootPath = resolveRootPathFromArgv(["node", "repo-cleanup-tui", ...args.slice(1)], process.cwd());
    runTui(rootPath);
    return;
  }

  if (command === "-h" || command === "--help" || command === "help") {
    console.log("repo-cleanup-tui [tui|scan|init]");
    console.log("  repo-cleanup-tui               Launch TUI");
    console.log("  repo-cleanup-tui init          Initialize config");
    console.log("  repo-cleanup-tui scan --json [rootPath]   Print scan results as JSON");
    return;
  }

  const pathLikeArg = command.includes("/") || command.startsWith(".") || command.startsWith("~");
  if (pathLikeArg) {
    runTui(path.resolve(command));
    return;
  }

  const config = await loadConfig(process.cwd());
  runTui(config.preferredRoot ?? process.cwd());
};

void run();
