package scancache

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/ericdahl-dev/repo-cleanup-tui/internal/scanner"
)

const (
	CurrentVersion = 1
	DefaultTTL     = 10 * time.Minute
	cacheFileName  = ".repo-cleanup-tui-scan-cache.json"
)

var (
	ErrMiss      = errors.New("scan cache miss")
	ErrCorrupt   = errors.New("scan cache corrupt")
	ErrVersion   = errors.New("scan cache unsupported version")
	ErrRoot      = errors.New("scan cache root mismatch")
	ErrExpired   = errors.New("scan cache expired")
	ErrSignature = errors.New("scan cache signature mismatch")
)

var lockFiles = []string{
	"yarn.lock", "pnpm-lock.yaml", "package-lock.json", "bun.lockb", "bun.lock",
}

type fileSig struct {
	ModTimeUnixNano int64 `json:"modTimeUnixNano"`
	Size            int64 `json:"size"`
}

type signature struct {
	PackageJSON fileSig            `json:"packageJson"`
	NodeModules fileSig            `json:"nodeModules"`
	Lockfiles   map[string]fileSig `json:"lockfiles,omitempty"`
}

type cachedRow struct {
	Candidate scanner.Candidate `json:"candidate"`
	Signature signature         `json:"signature"`
}

type cacheFile struct {
	Version    int         `json:"version"`
	Root       string      `json:"root"`
	SavedAt    time.Time   `json:"savedAt"`
	Candidates []cachedRow `json:"candidates"`
}

// Path returns the on-disk cache path for a workspace root.
func Path(root string) string {
	return filepath.Join(filepath.Clean(root), cacheFileName)
}

// Load reads a fresh, valid cache for root. Returns ErrMiss family on failure.
func Load(root string, ttl time.Duration) ([]scanner.Candidate, error) {
	if ttl <= 0 {
		ttl = DefaultTTL
	}
	root = filepath.Clean(root)
	data, err := os.ReadFile(Path(root))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrMiss
		}
		return nil, err
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, ErrCorrupt
	}
	if cf.Version != CurrentVersion {
		return nil, ErrVersion
	}
	if filepath.Clean(cf.Root) != root {
		return nil, ErrRoot
	}
	if time.Since(cf.SavedAt) > ttl {
		return nil, ErrExpired
	}
	out := make([]scanner.Candidate, 0, len(cf.Candidates))
	for _, row := range cf.Candidates {
		sig, err := computeSignature(row.Candidate.RepoPath)
		if err != nil {
			return nil, ErrSignature
		}
		if !sig.equal(row.Signature) {
			return nil, ErrSignature
		}
		out = append(out, row.Candidate)
	}
	return out, nil
}

// Save writes candidates and signatures for root.
func Save(root string, rows []scanner.Candidate) error {
	root = filepath.Clean(root)
	entries := make([]cachedRow, 0, len(rows))
	for _, row := range rows {
		sig, err := computeSignature(row.RepoPath)
		if err != nil {
			continue
		}
		entries = append(entries, cachedRow{Candidate: row, Signature: sig})
	}
	cf := cacheFile{
		Version:    CurrentVersion,
		Root:       root,
		SavedAt:    time.Now().UTC(),
		Candidates: entries,
	}
	data, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(Path(root), data, 0o600)
}

func computeSignature(repoPath string) (signature, error) {
	pkg := filepath.Join(repoPath, "package.json")
	pkgSig, err := statFile(pkg)
	if err != nil {
		return signature{}, err
	}
	nm := filepath.Join(repoPath, "node_modules")
	nmSig, err := statFile(nm)
	if err != nil {
		return signature{}, err
	}
	locks := make(map[string]fileSig)
	for _, name := range lockFiles {
		p := filepath.Join(repoPath, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			locks[name] = fileSigFrom(st)
		}
	}
	return signature{
		PackageJSON: pkgSig,
		NodeModules: nmSig,
		Lockfiles:   locks,
	}, nil
}

func statFile(path string) (fileSig, error) {
	st, err := os.Stat(path)
	if err != nil {
		return fileSig{}, err
	}
	return fileSigFrom(st), nil
}

func fileSigFrom(st os.FileInfo) fileSig {
	return fileSig{
		ModTimeUnixNano: st.ModTime().UTC().UnixNano(),
		Size:            st.Size(),
	}
}

func (s signature) equal(other signature) bool {
	if s.PackageJSON != other.PackageJSON || s.NodeModules != other.NodeModules {
		return false
	}
	if len(s.Lockfiles) != len(other.Lockfiles) {
		return false
	}
	for k, v := range s.Lockfiles {
		if other.Lockfiles[k] != v {
			return false
		}
	}
	return true
}
