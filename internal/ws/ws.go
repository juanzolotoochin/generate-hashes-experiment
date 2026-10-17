// Package ws materializes the fixture workspace and manages its git history.
package ws

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// fixtureSuffix marks files stored under a different name so the outer
// (harness) workspace and the go tool don't interpret them: BUILD.bazel and
// MODULE.bazel would make the fixture part of the outer Bazel workspace, and a
// go.mod would make it a separate Go module excluded from go:embed.
const fixtureSuffix = ".fixture"

// Materialize writes the tree rooted at root inside fsys into dir. Shell
// scripts are made executable, since embed.FS does not preserve file modes.
// The .fixture suffix is stripped from file names.
func Materialize(fsys fs.FS, root, dir string) error {
	return fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(p, root), "/")
		rel = strings.TrimSuffix(rel, fixtureSuffix)
		dst := filepath.Join(dir, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(p, ".sh") {
			mode = 0o755
		}
		return os.WriteFile(dst, data, mode)
	})
}

// Runner runs commands, logging each invocation to a file.
type Runner struct {
	Log *os.File
	// Env is the complete environment of every command; nothing is inherited
	// from the harness's own environment.
	Env []string
}

// Result is the outcome of a command.
type Result struct {
	Stdout   []byte
	Stderr   []byte
	Duration time.Duration
	Err      error
}

// Run executes name with args in dir.
func (r *Runner) Run(ctx context.Context, dir, name string, args ...string) Result {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = r.Env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if r.Log != nil {
		fmt.Fprintf(r.Log, "\n$ (cd %s && %s %s)\n", dir, name, strings.Join(args, " "))
	}
	start := time.Now()
	err := cmd.Run()
	res := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), Duration: time.Since(start), Err: err}
	if r.Log != nil {
		fmt.Fprintf(r.Log, "# exit=%v duration=%s\n", err, res.Duration.Round(time.Millisecond))
		if err != nil {
			r.Log.Write(tail(stderr.Bytes(), 8000))
		}
	}
	if err != nil {
		res.Err = fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, tail(stderr.Bytes(), 4000))
	}
	return res
}

func tail(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[len(b)-n:]
}

// Git wraps git operations on the benchmark repository.
type Git struct {
	Bin    string
	Dir    string
	Runner *Runner
}

func (g *Git) run(ctx context.Context, args ...string) (string, error) {
	res := g.Runner.Run(ctx, g.Dir, g.Bin, args...)
	return strings.TrimSpace(string(res.Stdout)), res.Err
}

// Init creates a repository and commits the current tree as the base revision.
func (g *Git) Init(ctx context.Context) (string, error) {
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "hashbench@example.com"},
		{"config", "user.name", "hashbench"},
		{"config", "commit.gpgsign", "false"},
		{"add", "-A"},
		{"commit", "-q", "-m", "base"},
	} {
		if _, err := g.run(ctx, args...); err != nil {
			return "", err
		}
	}
	return g.run(ctx, "rev-parse", "HEAD")
}

// Reset checks out rev and removes untracked (non-ignored) files.
func (g *Git) Reset(ctx context.Context, rev string) error {
	if _, err := g.run(ctx, "checkout", "-q", "-f", "--detach", rev); err != nil {
		return err
	}
	_, err := g.run(ctx, "clean", "-q", "-fd")
	return err
}

// Commit commits all changes (allowing an empty commit) and returns the new revision.
func (g *Git) Commit(ctx context.Context, msg string) (string, error) {
	if _, err := g.run(ctx, "add", "-A"); err != nil {
		return "", err
	}
	if _, err := g.run(ctx, "commit", "-q", "--allow-empty", "-m", msg); err != nil {
		return "", err
	}
	return g.run(ctx, "rev-parse", "HEAD")
}

// ChangedFiles lists paths changed between two revisions.
func (g *Git) ChangedFiles(ctx context.Context, from, to string) ([]string, error) {
	out, err := g.run(ctx, "diff", "--name-only", from, to)
	if err != nil || out == "" {
		return nil, err
	}
	return strings.Split(out, "\n"), nil
}

// DiffStat summarizes the change between two revisions.
func (g *Git) DiffStat(ctx context.Context, from, to string) string {
	out, _ := g.run(ctx, "diff", "--stat", "--summary", from, to)
	return out
}
