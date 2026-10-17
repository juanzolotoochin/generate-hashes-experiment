package oracle

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// actionGraph is the subset of `bazel aquery --output=jsonproto` we use.
type actionGraph struct {
	Artifacts []struct {
		ID             int  `json:"id"`
		PathFragmentID int  `json:"pathFragmentId"`
		IsTreeArtifact bool `json:"isTreeArtifact"`
	} `json:"artifacts"`
	Actions []struct {
		ActionKey      string `json:"actionKey"`
		Mnemonic       string `json:"mnemonic"`
		InputDepSetIDs []int  `json:"inputDepSetIds"`
		OutputIDs      []int  `json:"outputIds"`
	} `json:"actions"`
	DepSetOfFiles []struct {
		ID                  int   `json:"id"`
		DirectArtifactIDs   []int `json:"directArtifactIds"`
		TransitiveDepSetIDs []int `json:"transitiveDepSetIds"`
	} `json:"depSetOfFiles"`
	PathFragments []struct {
		ID       int    `json:"id"`
		Label    string `json:"label"`
		ParentID int    `json:"parentId"`
	} `json:"pathFragments"`
}

// inputFingerprinter computes, for any exec path, a fingerprint of everything
// that went into producing it: source files contribute their content and
// mode; generated files contribute their producing action's key plus the
// fingerprints of that action's inputs (recursively). Two builds give a file
// the same fingerprint exactly when no action in its transitive closure has
// different inputs, i.e. when Bazel would not need to re-run anything for it.
type inputFingerprinter struct {
	execRoot string
	outBase  string

	producer map[string]int    // exec path -> action index
	inputs   [][]string        // action index -> input exec paths
	keys     []string          // action index -> action key
	memo     map[string]string // exec path -> fingerprint
	src      *sourceHasher

	// Unproduced counts generated files with no producing action in the graph;
	// their built content is used instead.
	Unproduced int
}

func newInputFingerprinter(aqueryJSON io.Reader, execRoot, outBase string, src *sourceHasher) (*inputFingerprinter, error) {
	var g actionGraph
	if err := json.NewDecoder(aqueryJSON).Decode(&g); err != nil {
		return nil, fmt.Errorf("decode aquery output: %w", err)
	}
	frags := map[int]struct {
		label  string
		parent int
	}{}
	for _, f := range g.PathFragments {
		frags[f.ID] = struct {
			label  string
			parent int
		}{f.Label, f.ParentID}
	}
	fragPath := map[int]string{}
	var pathOf func(id int) string
	pathOf = func(id int) string {
		if p, ok := fragPath[id]; ok {
			return p
		}
		f := frags[id]
		p := f.label
		if f.parent != 0 {
			p = pathOf(f.parent) + "/" + f.label
		}
		fragPath[id] = p
		return p
	}
	artPath := map[int]string{}
	for _, a := range g.Artifacts {
		artPath[a.ID] = pathOf(a.PathFragmentID)
	}
	depsets := map[int][2][]int{}
	for _, d := range g.DepSetOfFiles {
		depsets[d.ID] = [2][]int{d.DirectArtifactIDs, d.TransitiveDepSetIDs}
	}
	expanded := map[int][]string{}
	var expand func(id int) []string
	expand = func(id int) []string {
		if e, ok := expanded[id]; ok {
			return e
		}
		expanded[id] = nil // guard against cycles
		d := depsets[id]
		var out []string
		for _, a := range d[0] {
			out = append(out, artPath[a])
		}
		for _, t := range d[1] {
			out = append(out, expand(t)...)
		}
		expanded[id] = out
		return out
	}

	f := &inputFingerprinter{execRoot: execRoot, outBase: outBase, producer: map[string]int{}, memo: map[string]string{}, src: src}
	for i, a := range g.Actions {
		var ins []string
		seen := map[string]bool{}
		for _, d := range a.InputDepSetIDs {
			for _, p := range expand(d) {
				if !seen[p] {
					seen[p] = true
					ins = append(ins, p)
				}
			}
		}
		sort.Strings(ins)
		f.inputs = append(f.inputs, ins)
		f.keys = append(f.keys, a.Mnemonic+":"+a.ActionKey)
		for _, o := range a.OutputIDs {
			f.producer[artPath[o]] = i
		}
	}
	return f, nil
}

// Fingerprint returns the input fingerprint of an exec path.
func (f *inputFingerprinter) Fingerprint(path string) (string, error) {
	if fp, ok := f.memo[path]; ok {
		return fp, nil
	}
	f.memo[path] = "cycle:" + path
	var fp string
	if idx, ok := f.producer[path]; ok {
		h := sha256.New()
		fmt.Fprintf(h, "action %s\n", f.keys[idx])
		for _, in := range f.inputs[idx] {
			sub, err := f.Fingerprint(in)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(h, "%s %s\n", in, sub)
		}
		fp = hex.EncodeToString(h.Sum(nil))
	} else if dir, ok := f.treeArtifact(path); ok {
		// A file inside a tree artifact: fingerprinted through the action
		// that produced the directory, plus its position in it.
		sub, err := f.Fingerprint(dir)
		if err != nil {
			return "", err
		}
		sum := sha256.Sum256([]byte(sub + "/" + strings.TrimPrefix(path, dir+"/")))
		fp = hex.EncodeToString(sum[:])
	} else {
		if strings.HasPrefix(path, "bazel-out/") {
			f.Unproduced++
		}
		var err error
		if fp, err = f.src.Hash(f.resolve(path)); err != nil {
			return "", fmt.Errorf("hash %s: %w", path, err)
		}
	}
	f.memo[path] = fp
	return fp, nil
}

// treeArtifact returns the closest enclosing directory of path that an
// action produces (a tree artifact), if any.
func (f *inputFingerprinter) treeArtifact(path string) (string, bool) {
	if !strings.HasPrefix(path, "bazel-out/") {
		return "", false
	}
	for dir := filepath.Dir(path); dir != "." && dir != "/" && dir != "bazel-out"; dir = filepath.Dir(dir) {
		if _, ok := f.producer[dir]; ok {
			return dir, true
		}
	}
	return "", false
}

// Mode returns "x" if a source file is executable and "-" otherwise
// (following symlinks). Generated files (under bazel-out/) return "": their
// mode depends on whether they were built locally or restored from a cache, so
// it carries no information.
func (f *inputFingerprinter) Mode(path string) string {
	if strings.HasPrefix(path, "bazel-out/") {
		return ""
	}
	st, err := os.Stat(f.resolve(path))
	switch {
	case err != nil:
		return "missing"
	case st.Mode().Perm()&0o111 != 0:
		return "x"
	default:
		return "-"
	}
}

// resolve maps an exec path to a readable filesystem path.
func (f *inputFingerprinter) resolve(path string) string {
	p := filepath.Join(f.execRoot, path)
	if _, err := os.Lstat(p); err == nil {
		return p
	}
	if rest, ok := strings.CutPrefix(path, "external/"); ok {
		return filepath.Join(f.outBase, "external", rest)
	}
	return p
}

// sourceHasher hashes files (or directory trees) by content, caching by size
// and modification time across builds. Modes are deliberately excluded: Bazel
// keys actions on input contents, so a mode change alone never re-runs an
// action. Modes of files a target ships directly are added per target.
type sourceHasher struct {
	mu    sync.Mutex
	cache map[string]cachedHash
}

type cachedHash struct {
	size, mtime int64
	sum         string
}

func newSourceHasher() *sourceHasher { return &sourceHasher{cache: map[string]cachedHash{}} }

func (s *sourceHasher) Hash(p string) (string, error) {
	st, err := os.Stat(p)
	if os.IsNotExist(err) {
		// Inputs that don't exist on disk (e.g. optional or never-materialized
		// files) are identified by path alone.
		return "missing:" + p, nil
	}
	if err != nil {
		return "", err
	}
	if st.IsDir() {
		h := sha256.New()
		err := filepath.WalkDir(p, func(q string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			sum, err := s.Hash(q)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(p, q)
			fmt.Fprintf(h, "%s %s\n", rel, sum)
			return nil
		})
		return "dir:" + hex.EncodeToString(h.Sum(nil)), err
	}
	s.mu.Lock()
	c, ok := s.cache[p]
	s.mu.Unlock()
	if ok && c.size == st.Size() && c.mtime == st.ModTime().UnixNano() {
		return c.sum, nil
	}
	fh, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer fh.Close()
	h := sha256.New()
	if _, err := io.Copy(h, fh); err != nil {
		return "", err
	}
	sum := hex.EncodeToString(h.Sum(nil))
	s.mu.Lock()
	s.cache[p] = cachedHash{st.Size(), st.ModTime().UnixNano(), sum}
	s.mu.Unlock()
	return sum, nil
}
