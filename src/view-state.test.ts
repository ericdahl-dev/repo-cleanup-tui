import { test } from "node:test";
import assert from "node:assert/strict";
import type { RepoScan } from "./scanner.js";
import { filterAndSortRows } from "./view-state.js";

const row = (overrides: Partial<RepoScan>): RepoScan => ({
  repoPath: "/tmp/repo",
  nodeModulesPath: "/tmp/repo/node_modules",
  manager: "yarn",
  hasLockfile: true,
  inactiveDays: 120,
  bytes: 100,
  reinstallCommand: "yarn install --immutable",
  git: { branch: "main", dirty: false, ahead: 0, behind: 0 },
  ...overrides
});

test("filterAndSortRows filters by safety and inactivity", () => {
  const rows = [
    row({ repoPath: "/r1", hasLockfile: false, inactiveDays: 400, bytes: 500 }),
    row({ repoPath: "/r2", hasLockfile: true, inactiveDays: 20, bytes: 400 }),
    row({ repoPath: "/r3", hasLockfile: true, inactiveDays: 300, bytes: 300 })
  ];

  const result = filterAndSortRows(rows, {
    minInactiveDays: 90,
    showOnlySafe: true,
    sortMode: "size"
  });

  assert.deepEqual(
    result.map((item) => item.repoPath),
    ["/r3"]
  );
});

test("filterAndSortRows sorts by inactivity descending with unknown last", () => {
  const rows = [
    row({ repoPath: "/r1", inactiveDays: 10, bytes: 300 }),
    row({ repoPath: "/r2", inactiveDays: null, bytes: 999 }),
    row({ repoPath: "/r3", inactiveDays: 200, bytes: 100 })
  ];

  const result = filterAndSortRows(rows, {
    minInactiveDays: 0,
    showOnlySafe: false,
    sortMode: "inactive"
  });

  assert.deepEqual(
    result.map((item) => item.repoPath),
    ["/r3", "/r1", "/r2"]
  );
});

test("filterAndSortRows filters by searchQuery against path and branch", () => {
  const rows = [
    row({ repoPath: "/alpha/service-api", git: { branch: "main", dirty: false, ahead: 0, behind: 0 } }),
    row({ repoPath: "/beta/web", git: { branch: "feature/cleanup", dirty: false, ahead: 0, behind: 0 } })
  ];
  const byPath = filterAndSortRows(rows, {
    minInactiveDays: 0,
    showOnlySafe: false,
    sortMode: "size",
    searchQuery: "service"
  });
  const byBranch = filterAndSortRows(rows, {
    minInactiveDays: 0,
    showOnlySafe: false,
    sortMode: "size",
    searchQuery: "cleanup"
  });
  assert.equal(byPath.length, 1);
  assert.equal(byBranch.length, 1);
  assert.equal(byPath[0]?.repoPath, "/alpha/service-api");
  assert.equal(byBranch[0]?.repoPath, "/beta/web");
});

test("filterAndSortRows filters by dirty repos when requested", () => {
  const rows = [
    row({ repoPath: "/clean", git: { branch: "main", dirty: false, ahead: 0, behind: 0 } }),
    row({ repoPath: "/dirty", git: { branch: "main", dirty: true, ahead: 0, behind: 0 } })
  ];
  const result = filterAndSortRows(rows, {
    minInactiveDays: 0,
    showOnlySafe: false,
    showOnlyDirty: true,
    sortMode: "size"
  });
  assert.deepEqual(
    result.map((item) => item.repoPath),
    ["/dirty"]
  );
});
