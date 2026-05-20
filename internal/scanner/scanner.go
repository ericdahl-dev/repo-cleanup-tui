package scanner

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type Manager string

const (
	ManagerYarn    Manager = "yarn"
	ManagerPnpm    Manager = "pnpm"
	ManagerNpm     Manager = "npm"
	ManagerBun     Manager = "bun"
	ManagerUnknown Manager = "unknown"
)

type GitContext struct {
	Branch string `json:"branch"`
	Dirty  bool   `json:"dirty"`
	Ahead  int    `json:"ahead"`
	Behind int    `json:"behind"`
}

type Candidate struct {
	RepoPath         string     `json:"repoPath"`
	NodeModulesPath  string     `json:"nodeModulesPath"`
	Manager          Manager    `json:"manager"`
	HasLockfile      bool       `json:"hasLockfile"`
	InactiveDays     *int       `json:"inactiveDays"`
	Bytes            int64      `json:"bytes"`
	ReinstallCommand string     `json:"reinstallCommand"`
	Git              GitContext `json:"git"`
}

type Progress struct {
	DirectoriesScanned int
}

type Options struct {
	OnProgress     func(Progress)
	OnFound        func(Candidate)
	OnSized        func(Candidate)
	ProgressEvery  int
	SizeConcurrent int
	IgnoreDirs     []string
}

var defaultSkip = map[string]struct{}{
	"node_modules": {}, ".git": {}, ".next": {}, ".nuxt": {},
	".turbo": {}, "dist": {}, "build": {}, "coverage": {},
	"out": {}, "target": {},
}

func Scan(root string, opts Options) ([]Candidate, error) {
	if opts.ProgressEvery <= 0 {
		opts.ProgressEvery = 100
	}
	if opts.SizeConcurrent <= 0 {
		opts.SizeConcurrent = 6
	}

	skip := make(map[string]struct{}, len(defaultSkip)+len(opts.IgnoreDirs))
	for k := range defaultSkip {
		skip[k] = struct{}{}
	}
	for _, d := range opts.IgnoreDirs {
		skip[d] = struct{}{}
	}

	var (
		mu       sync.Mutex
		out      []Candidate
		queue    = []string{filepath.Clean(root)}
		qIdx     int
		dirCount int
		lastRep  int
		lim      = make(chan struct{}, opts.SizeConcurrent)
		wg       sync.WaitGroup
	)

	reportProgress := func() {
		if dirCount == 1 || dirCount-lastRep >= opts.ProgressEvery {
			lastRep = dirCount
			if opts.OnProgress != nil {
				opts.OnProgress(Progress{DirectoriesScanned: dirCount})
			}
		}
	}

	for qIdx < len(queue) {
		current := queue[qIdx]
		qIdx++
		dirCount++
		reportProgress()

		entries, err := os.ReadDir(current)
		if err != nil {
			continue
		}

		var hasGit, hasPkg, hasNM bool
		for _, e := range entries {
			switch e.Name() {
			case ".git":
				if e.IsDir() {
					hasGit = true
				}
			case "package.json":
				if !e.IsDir() {
					hasPkg = true
				}
			case "node_modules":
				if e.IsDir() {
					hasNM = true
				}
			}
		}

		if hasPkg && hasNM && (hasGit || isGitWorkTree(current)) {
			row := buildCandidate(current)
			mu.Lock()
			out = append(out, row)
			mu.Unlock()
			if opts.OnFound != nil {
				opts.OnFound(row)
			}

			wg.Add(1)
			lim <- struct{}{}
			go func(r Candidate, idx int) {
				defer wg.Done()
				defer func() { <-lim }()
				size, _ := dirSize(r.NodeModulesPath)
				mu.Lock()
				out[idx].Bytes = size
				updated := out[idx]
				mu.Unlock()
				if opts.OnSized != nil {
					opts.OnSized(updated)
				}
			}(row, len(out)-1)
			continue
		}

		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			if _, skipIt := skip[e.Name()]; skipIt {
				continue
			}
			queue = append(queue, filepath.Join(current, e.Name()))
		}
	}

	if dirCount != lastRep && opts.OnProgress != nil {
		opts.OnProgress(Progress{DirectoriesScanned: dirCount})
	}
	wg.Wait()
	return out, nil
}

func buildCandidate(repoPath string) Candidate {
	nm := filepath.Join(repoPath, "node_modules")
	mgr, hasLock := detectManager(repoPath)
	return Candidate{
		RepoPath:         repoPath,
		NodeModulesPath:  nm,
		Manager:          mgr,
		HasLockfile:      hasLock,
		InactiveDays:     inactiveDays(repoPath),
		ReinstallCommand: reinstallCommand(mgr),
		Git:              gitContext(repoPath),
	}
}

func isGitWorkTree(dir string) bool {
	repo, err := git.PlainOpen(dir)
	if err != nil {
		return false
	}
	_, err = repo.Head()
	return err == nil
}

func detectManager(repoPath string) (Manager, bool) {
	locks := []struct {
		file string
		mgr  Manager
	}{
		{"yarn.lock", ManagerYarn},
		{"pnpm-lock.yaml", ManagerPnpm},
		{"package-lock.json", ManagerNpm},
		{"bun.lockb", ManagerBun},
		{"bun.lock", ManagerBun},
	}
	for _, l := range locks {
		if fileExists(filepath.Join(repoPath, l.file)) {
			return l.mgr, true
		}
	}
	return ManagerUnknown, false
}

func reinstallCommand(m Manager) string {
	switch m {
	case ManagerYarn:
		return "yarn install --immutable"
	case ManagerPnpm:
		return "pnpm install --frozen-lockfile"
	case ManagerNpm:
		return "npm ci"
	case ManagerBun:
		return "bun install --frozen-lockfile"
	default:
		return "install command unknown"
	}
}

func gitContext(repoPath string) GitContext {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return GitContext{}
	}
	head, err := repo.Head()
	if err != nil {
		return GitContext{}
	}
	branch := head.Name().Short()
	if branch == "" {
		branch = head.Name().String()
	}

	wt, err := repo.Worktree()
	dirty := false
	if err == nil {
		st, err := wt.Status()
		dirty = err == nil && !st.IsClean()
	}

	ahead, behind := aheadBehind(repo, head)
	return GitContext{Branch: branch, Dirty: dirty, Ahead: ahead, Behind: behind}
}

func aheadBehind(repo *git.Repository, head *plumbing.Reference) (ahead, behind int) {
	name := head.Name().Short()
	if name == "" {
		return 0, 0
	}
	branch, err := repo.Branch(name)
	if err != nil || branch == nil || branch.Merge == "" {
		return 0, 0
	}
	mergeRef := branch.Merge.Short()
	if mergeRef == "" {
		mergeRef = branch.Merge.String()
	}
	remoteRef := plumbing.NewRemoteReferenceName(branch.Remote, mergeRef)
	upstream, err := repo.Reference(remoteRef, true)
	if err != nil {
		return 0, 0
	}
	ahead, _ = countReachableUntil(repo, head.Hash(), upstream.Hash())
	behind, _ = countReachableUntil(repo, upstream.Hash(), head.Hash())
	return ahead, behind
}

func countReachableUntil(repo *git.Repository, from, until plumbing.Hash) (int, error) {
	if from == until {
		return 0, nil
	}
	iter, err := repo.Log(&git.LogOptions{From: from})
	if err != nil {
		return 0, err
	}
	defer iter.Close()
	count := 0
	for {
		c, err := iter.Next()
		if err != nil {
			break
		}
		if c.Hash == until {
			return count, nil
		}
		count++
	}
	return count, nil
}

func inactiveDays(repoPath string) *int {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil
	}
	head, err := repo.Head()
	if err != nil {
		return nil
	}
	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return nil
	}
	when := commit.Committer.When
	if when.IsZero() {
		when = commit.Author.When
	}
	days := int(time.Since(when).Hours() / 24)
	if days < 0 {
		days = 0
	}
	return &days
}

func dirSize(dir string) (int64, error) {
	if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
		out, err := exec.Command("du", "-sk", dir).Output()
		if err == nil {
			fields := strings.Fields(string(out))
			if len(fields) > 0 {
				var kb int64
				if _, err := fmt.Sscanf(fields[0], "%d", &kb); err == nil && kb >= 0 {
					return kb * 1024, nil
				}
			}
		}
	}

	var total int64
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total, err
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
