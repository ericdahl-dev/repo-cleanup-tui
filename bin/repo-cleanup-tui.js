#!/usr/bin/env node
import { spawn } from "node:child_process";
import path from "node:path";
import fs from "node:fs";
import { fileURLToPath } from "node:url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const entry = path.resolve(__dirname, "../src/cli.ts");
const localTsx = path.resolve(__dirname, "../node_modules/.bin/tsx");
const command = fs.existsSync(localTsx) ? localTsx : "yarn";
const args = fs.existsSync(localTsx) ? [entry, ...process.argv.slice(2)] : ["tsx", entry, ...process.argv.slice(2)];

const child = spawn(command, args, {
  stdio: "inherit",
  env: process.env
});

child.on("error", (error) => {
  if ("code" in error && error.code === "ENOENT") {
    console.error("Unable to start repo-cleanup-tui: yarn/tsx launcher not found.");
    console.error("Install dependencies with `yarn install` and retry.");
    process.exit(1);
  }
  throw error;
});

child.on("exit", (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code ?? 0);
});
