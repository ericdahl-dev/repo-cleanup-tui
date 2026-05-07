import { test } from "node:test";
import assert from "node:assert/strict";
import { getDefaultConfig, upsertRoot } from "./config.js";

test("upsertRoot prepends root and sets preferredRoot", () => {
  const cfg = getDefaultConfig("/tmp/one");
  const updated = upsertRoot(cfg, "/tmp/two");
  assert.equal(updated.preferredRoot, "/tmp/two");
  assert.equal(updated.roots[0], "/tmp/two");
  assert.ok(updated.roots.includes("/tmp/one"));
});
