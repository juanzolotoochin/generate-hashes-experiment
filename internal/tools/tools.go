// Package tools adapts each target-selection tool to a common interface.
package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/juanzolotoochin/generate-hashes-experiment/internal/ws"
)

// Env is what every tool needs to run against the benchmark workspace.
type Env struct {
	Runner      *ws.Runner
	Workspace   string // the git checkout the scenarios mutate
	StateDir    string // per-tool scratch space (output bases, hash files)
	StockBazel  string
	CustomBazel string
	BazelDiff   string
	TD          string
}

// Outcome is one tool's answer for one scenario.
type Outcome struct {
	Affected map[string]bool
	Labels   []string
	Duration time.Duration
	Phases   map[string]time.Duration
	Err      string
}

// Tool computes the targets affected between the base and head revisions.
type Tool interface {
	Name() string
	// Prepare is called once with the base revision checked out.
	Prepare(ctx context.Context, baseRev string) error
	// Affected is called with headRev checked out in env.Workspace.
	Affected(ctx context.Context, baseRev, headRev string) Outcome
	// Shutdown stops any Bazel servers the tool started.
	Shutdown(ctx context.Context)
}

// New returns the tool with the given name.
func New(name string, env Env) (Tool, error) {
	dir := filepath.Join(env.StateDir, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	switch name {
	case "custom":
		return &customTool{env: env, dir: dir}, nil
	case "bazel-diff":
		return &bazelDiffTool{env: env, dir: dir}, nil
	case "target-determinator":
		return &tdTool{env: env, dir: dir}, nil
	}
	return nil, fmt.Errorf("unknown tool %q (want custom, bazel-diff or target-determinator)", name)
}

// NormalizeLabel maps @@//pkg:name and @//pkg:name to //pkg:name, and expands
// the short form //pkg/name to //pkg/name:name. Labels in other repositories
// are returned unchanged.
func NormalizeLabel(l string) string {
	l = strings.TrimSpace(l)
	for _, p := range []string{"@@//", "@//"} {
		if strings.HasPrefix(l, p) {
			l = "//" + strings.TrimPrefix(l, p)
			break
		}
	}
	if strings.HasPrefix(l, "//") && !strings.Contains(l, ":") {
		l += ":" + l[strings.LastIndex(l, "/")+1:]
	}
	return l
}

func labelSet(lines []string) map[string]bool {
	m := map[string]bool{}
	for _, l := range lines {
		if l = NormalizeLabel(l); l != "" {
			m[l] = true
		}
	}
	return m
}

func finish(o Outcome) Outcome {
	for l := range o.Affected {
		o.Labels = append(o.Labels, l)
	}
	return o
}

// ---- custom Bazel with `query --generate_hashes` ----

type customTool struct {
	env  Env
	dir  string
	base map[string]string
}

func (t *customTool) Name() string { return "custom" }

func (t *customTool) hashes(ctx context.Context) (map[string]string, time.Duration, error) {
	res := t.env.Runner.Run(ctx, t.env.Workspace, t.env.CustomBazel,
		"--output_base="+filepath.Join(t.dir, "output_base"),
		"query", "//...", "--generate_hashes")
	if res.Err != nil {
		return nil, res.Duration, res.Err
	}
	m := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(res.Stdout))
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		label, hash, ok := strings.Cut(strings.TrimSpace(sc.Text()), " ")
		if ok {
			m[NormalizeLabel(label)] = hash
		}
	}
	return m, res.Duration, sc.Err()
}

func (t *customTool) Prepare(ctx context.Context, _ string) error {
	m, _, err := t.hashes(ctx)
	t.base = m
	return err
}

func (t *customTool) Affected(ctx context.Context, _, _ string) Outcome {
	head, d, err := t.hashes(ctx)
	o := Outcome{Duration: d, Affected: map[string]bool{}}
	if err != nil {
		o.Err = err.Error()
		return finish(o)
	}
	for l, h := range head {
		if t.base[l] != h {
			o.Affected[l] = true
		}
	}
	return finish(o)
}

func (t *customTool) Shutdown(ctx context.Context) {
	t.env.Runner.Run(ctx, t.env.Workspace, t.env.CustomBazel, "--output_base="+filepath.Join(t.dir, "output_base"), "shutdown")
}

// ---- bazel-diff (unmodified release) ----

type bazelDiffTool struct {
	env  Env
	dir  string
	base string
}

func (t *bazelDiffTool) Name() string { return "bazel-diff" }

func (t *bazelDiffTool) startup() string {
	return "--bazelStartupOptions=--output_base=" + filepath.Join(t.dir, "output_base")
}

func (t *bazelDiffTool) generate(ctx context.Context, out string) ws.Result {
	return t.env.Runner.Run(ctx, t.env.Workspace, t.env.BazelDiff, "generate-hashes",
		"--workspacePath="+t.env.Workspace, "--bazelPath="+t.env.StockBazel, t.startup(), out)
}

func (t *bazelDiffTool) Prepare(ctx context.Context, baseRev string) error {
	t.base = filepath.Join(t.dir, "base-"+baseRev[:12]+".json")
	return t.generate(ctx, t.base).Err
}

func (t *bazelDiffTool) Affected(ctx context.Context, _, headRev string) Outcome {
	o := Outcome{Affected: map[string]bool{}, Phases: map[string]time.Duration{}}
	head := filepath.Join(t.dir, "head-"+headRev[:12]+".json")
	gen := t.generate(ctx, head)
	o.Phases["generate-hashes"] = gen.Duration
	o.Duration = gen.Duration
	if gen.Err != nil {
		o.Err = gen.Err.Error()
		return finish(o)
	}
	impacted := filepath.Join(t.dir, "impacted-"+headRev[:12]+".txt")
	imp := t.env.Runner.Run(ctx, t.env.Workspace, t.env.BazelDiff, "get-impacted-targets",
		"--startingHashes="+t.base, "--finalHashes="+head, "--workspacePath="+t.env.Workspace,
		"--bazelPath="+t.env.StockBazel, t.startup(), "--output="+impacted)
	o.Phases["get-impacted-targets"] = imp.Duration
	o.Duration += imp.Duration
	if imp.Err != nil {
		o.Err = imp.Err.Error()
		return finish(o)
	}
	b, err := os.ReadFile(impacted)
	if err != nil {
		o.Err = err.Error()
		return finish(o)
	}
	o.Affected = labelSet(strings.Split(string(b), "\n"))
	return finish(o)
}

func (t *bazelDiffTool) Shutdown(ctx context.Context) {
	t.env.Runner.Run(ctx, t.env.Workspace, t.env.StockBazel, "--output_base="+filepath.Join(t.dir, "output_base"), "shutdown")
}

// ---- target-determinator ----

type tdTool struct {
	env Env
	dir string
}

func (t *tdTool) Name() string { return "target-determinator" }

func (t *tdTool) userRoot() string { return filepath.Join(t.dir, "output_user_root") }

func (t *tdTool) Prepare(ctx context.Context, baseRev string) error {
	// Warm up the Bazel server on a no-op comparison. The benchmark checkout is
	// always clean, so target-determinator checks the base revision out in
	// place (same directory, same server) rather than in a worktree.
	o := t.Affected(ctx, baseRev, baseRev)
	if o.Err != "" {
		return fmt.Errorf("%s", o.Err)
	}
	return nil
}

func (t *tdTool) Affected(ctx context.Context, baseRev, _ string) Outcome {
	o := Outcome{Affected: map[string]bool{}}
	res := t.env.Runner.Run(ctx, t.env.Workspace, t.env.TD,
		"-bazel="+t.env.StockBazel,
		"-bazel-startup-opts=--output_user_root="+t.userRoot(),
		"-cache-dir="+filepath.Join(t.dir, "cache"),
		"-nocache_results",
		"-working-directory="+t.env.Workspace,
		baseRev)
	o.Duration = res.Duration
	if res.Err != nil {
		o.Err = res.Err.Error()
		return finish(o)
	}
	o.Affected = labelSet(strings.Split(string(res.Stdout), "\n"))
	return finish(o)
}

func (t *tdTool) Shutdown(ctx context.Context) {
	// Stop every server under the tool's output user root (normally one; a
	// worktree, if target-determinator ever needed one, gets its own).
	entries, _ := filepath.Glob(filepath.Join(t.userRoot(), "*", "server", "server.pid.txt"))
	for _, pid := range entries {
		ob := filepath.Dir(filepath.Dir(pid))
		t.env.Runner.Run(ctx, t.env.Workspace, t.env.StockBazel, "--output_user_root="+t.userRoot(), "--output_base="+ob, "shutdown")
	}
}

// MarshalJSON writes durations in seconds.
func (o Outcome) MarshalJSON() ([]byte, error) {
	phases := map[string]float64{}
	for k, v := range o.Phases {
		phases[k] = v.Seconds()
	}
	return json.Marshal(struct {
		Affected     []string           `json:"affected"`
		Seconds      float64            `json:"seconds"`
		PhaseSeconds map[string]float64 `json:"phase_seconds,omitempty"`
		Err          string             `json:"error,omitempty"`
	}{o.Labels, o.Duration.Seconds(), phases, o.Err})
}
