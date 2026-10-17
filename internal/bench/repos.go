package bench

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// RepoSupport is how well one module's repositories work with the repo
// contents cache (local, and so also --experimental_remote_repo_contents_cache).
// Bazel only caches a repository whose rule declares it reproducible; in an
// output base such a repository is a symlink into the repo contents cache, any
// other repository is a plain directory.
type RepoSupport struct {
	Module         string   `json:"module"` // owning module; "(fixture)" for the fixture's own repo rules and extensions
	Direct         bool     `json:"direct"` // a bazel_dep of the fixture
	Cached         int      `json:"cached"`
	CachedBytes    int64    `json:"cached_bytes"`
	NotCached      int      `json:"not_cached"`
	NotCachedBytes int64    `json:"not_cached_bytes"`
	NotCachedRepos []string `json:"not_cached_repos,omitempty"`
}

// repoSupport classifies every repository fetched into the given external
// directories (the union over all tools), grouped by owning module and sorted
// by total size.
func repoSupport(externalDirs []string, moduleFile string) []RepoSupport {
	type repo struct {
		cached bool
		bytes  int64
	}
	repos := map[string]repo{}
	for _, ext := range externalDirs {
		entries, err := os.ReadDir(ext)
		if err != nil {
			continue
		}
		for _, e := range entries {
			n := e.Name()
			if strings.HasPrefix(n, "@") || strings.HasSuffix(n, ".marker") || n == "bazel_tools" || n == "_main" {
				continue
			}
			if _, seen := repos[n]; seen {
				continue
			}
			p := filepath.Join(ext, n)
			target, err := os.Readlink(p)
			cached := err == nil && strings.Contains(target, "repo_contents")
			if err == nil && !filepath.IsAbs(target) {
				target = filepath.Join(ext, target)
			} else if err != nil {
				target = p
			}
			repos[n] = repo{cached: cached, bytes: dirBytes(target)}
		}
	}
	direct := directDeps(moduleFile)
	groups := map[string]*RepoSupport{}
	for n, r := range repos {
		m := repoOwner(n)
		g := groups[m]
		if g == nil {
			g = &RepoSupport{Module: m, Direct: direct[m]}
			groups[m] = g
		}
		if r.cached {
			g.Cached++
			g.CachedBytes += r.bytes
		} else {
			g.NotCached++
			g.NotCachedBytes += r.bytes
			g.NotCachedRepos = append(g.NotCachedRepos, n)
		}
	}
	var out []RepoSupport
	for _, g := range groups {
		sort.Strings(g.NotCachedRepos)
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].CachedBytes+out[i].NotCachedBytes, out[j].CachedBytes+out[j].NotCachedBytes
		if a != b {
			return a > b
		}
		return out[i].Module < out[j].Module
	})
	return out
}

// repoOwner maps a canonical repository name to its module: "rules_go+" and
// "rules_go++go_sdk+x" to rules_go, the fixture's "+http_archive+x" to
// "(fixture)".
func repoOwner(name string) string {
	if strings.HasPrefix(name, "+") {
		return "(fixture)"
	}
	m, _, _ := strings.Cut(name, "+")
	return m
}

var bazelDepRE = regexp.MustCompile(`bazel_dep\(\s*name\s*=\s*"([^"]+)"`)

func directDeps(moduleFile string) map[string]bool {
	b, _ := os.ReadFile(moduleFile)
	m := map[string]bool{}
	for _, s := range bazelDepRE.FindAllStringSubmatch(string(b), -1) {
		m[s[1]] = true
	}
	return m
}

// dirBytes is the apparent size of the regular files under dir.
func dirBytes(dir string) int64 {
	var n int64
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}

// externalDirs returns the external/ directories of every output base under
// state (tool output bases, and target-determinator's output user root).
func externalDirs(state string) []string {
	var dirs []string
	for _, pat := range []string{"*/output_base/external", "*/output_user_root/*/external"} {
		m, _ := filepath.Glob(filepath.Join(state, pat))
		dirs = append(dirs, m...)
	}
	sort.Strings(dirs)
	return dirs
}

// RepoSupportFor computes the table for an existing work dir.
func RepoSupportFor(workDir string) []RepoSupport {
	var dirs []string
	for _, d := range externalDirs(filepath.Join(workDir, "state")) {
		if !strings.Contains(d, "/oracle/") && !strings.Contains(d, "/count/") {
			dirs = append(dirs, d)
		}
	}
	return repoSupport(dirs, filepath.Join(workDir, "repo", "MODULE.bazel"))
}
