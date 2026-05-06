import { loadRowsFromArgv } from "./runtime.js";

const { rootPath, rows } = await loadRowsFromArgv(process.argv, process.cwd());
const reclaimableBytes = rows.reduce((sum, row) => sum + row.bytes, 0);
const safeRows = rows.filter((row) => row.hasLockfile).length;

console.log(`root=${rootPath}`);
console.log(`repos=${rows.length}`);
console.log(`safe=${safeRows}`);
console.log(`reclaimable_mb=${Math.round(reclaimableBytes / 1024 / 1024)}`);
if (rows.length > 0) {
  console.log(`first_repo=${rows[0].repoPath}`);
}
