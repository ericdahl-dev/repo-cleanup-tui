import type { RepoScan } from "./scanner.js";

export type SortMode = "size" | "inactive";

export type FilterSortOptions = {
  minInactiveDays: number;
  showOnlySafe: boolean;
  sortMode: SortMode;
  searchQuery?: string;
  showOnlyDirty?: boolean;
};

export const filterAndSortRows = (rows: RepoScan[], options: FilterSortOptions): RepoScan[] => {
  const normalizedQuery = options.searchQuery?.trim().toLowerCase() ?? "";
  const base = rows.filter((row) => {
    const inactiveOk =
      options.minInactiveDays === 0 || (row.inactiveDays !== null && row.inactiveDays >= options.minInactiveDays);
    const safeOk = !options.showOnlySafe || row.hasLockfile;
    const dirtyOk = !options.showOnlyDirty || row.git?.dirty === true;
    const searchOk =
      normalizedQuery.length === 0 ||
      row.repoPath.toLowerCase().includes(normalizedQuery) ||
      row.nodeModulesPath.toLowerCase().includes(normalizedQuery) ||
      (row.git?.branch?.toLowerCase().includes(normalizedQuery) ?? false);
    return inactiveOk && safeOk && dirtyOk && searchOk;
  });

  return [...base].sort((a, b) => {
    if (options.sortMode === "size") return b.bytes - a.bytes;
    return (b.inactiveDays ?? -1) - (a.inactiveDays ?? -1);
  });
};
