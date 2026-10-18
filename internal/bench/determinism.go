package bench

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/juanzolotoochin/generate-hashes-experiment/fixture"
	"github.com/juanzolotoochin/generate-hashes-experiment/internal/tools"
	"github.com/juanzolotoochin/generate-hashes-experiment/internal/ws"
)

// DetResult is one determinism check for one tool: the same revision hashed
// twice with one thing changed in between. Every target whose hash differs
// would be selected although nothing changed.
type DetResult struct {
	Check     string   `json:"check"`
	Tool      string   `json:"tool"`
	Compared  int      `json:"compared"`
	Different []string `json:"different,omitempty"`
	Err       string   `json:"error,omitempty"`
	Seconds   float64  `json:"seconds"`
}

// DetCheck describes one determinism check.
type DetCheck struct {
	Name, Description string
	Remote            bool // needs a remote cache
}

// DetChecks are the checks, in the order they run for each tool.
var DetChecks = []DetCheck{
	{Name: "repeat", Description: "Hash twice in a row with the same Bazel server."},
	{Name: "restart", Description: "Shut down the Bazel server between the two snapshots."},
	{Name: "environment", Description: "Restart the server with a different HOME, USER and TMPDIR."},
	{Name: "output_base_path", Description: "Second snapshot from an output base at another path (same checkout and caches), as on another machine."},
	{Name: "fresh_output_base", Description: "Delete the output base between the two snapshots (same path, caches kept)."},
	{Name: "checkout_path", Description: "Second snapshot from a clone of the checkout at another path (output base recreated at the same path), as in another CI worktree."},
	{Name: "fresh_caches", Description: "Delete the output base and the repository and repo contents caches between the two snapshots (same paths)."},
	{Name: "build_between", Description: "Build //... between the two snapshots."},
	{Name: "remote_cache", Remote: true, Description: "First snapshot with --experimental_remote_repo_contents_cache, second without it (output base recreated at the same path)."},
	{Name: "remote_off_same_output_base", Remote: true, Description: "First snapshot with --experimental_remote_repo_contents_cache, second without it in the same output base (as when toggling the flag in .bazelrc)."},
	{Name: "remote_on_again", Remote: true, Description: "Then with --experimental_remote_repo_contents_cache again, same output base, compared with the first snapshot."},
	{Name: "build_between_remote", Remote: true, Description: "With --experimental_remote_repo_contents_cache: build //... between the two snapshots."},
	{Name: "remote_then_local_build", Remote: true, Description: "First snapshot with --experimental_remote_repo_contents_cache (as in CI), second without it after a local build //... (as on a laptop), same output base path."},
}

// RunDeterminism runs the determinism suite for every hash-based tool in
// cfg.Tools, from its own checkout of the fixture plus hazard repos (see
// addHazards) under <WorkDir>/determinism, which is recreated every run. The
// remote checks run only if cfg.RemoteCache is set.
func RunDeterminism(ctx context.Context, cfg Config) ([]DetResult, error) {
	say := func(format string, args ...any) { fmt.Fprintf(cfg.Out, format+"\n", args...) }
	root := filepath.Join(cfg.WorkDir, "determinism")
	if err := removeAll(root); err != nil {
		return nil, err
	}
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(home, 0o755); err != nil {
		return nil, err
	}
	logf, err := os.Create(filepath.Join(root, "commands.log"))
	if err != nil {
		return nil, err
	}
	defer logf.Close()
	runner := &ws.Runner{Log: logf, Env: []string{
		"PATH=" + filepath.Join(cfg.Toolbox, "bin"),
		"HOME=" + home,
		"USER=hashbench",
		"LANG=C.UTF-8",
	}}
	other := filepath.Join(root, "other-user")
	for _, d := range []string{filepath.Join(other, "home"), filepath.Join(other, "tmp")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}
	otherRunner := &ws.Runner{Log: logf, Env: []string{
		"PATH=" + filepath.Join(cfg.Toolbox, "bin"),
		"HOME=" + filepath.Join(other, "home"),
		"USER=someone-else",
		"TMPDIR=" + filepath.Join(other, "tmp"),
		"LANG=C.UTF-8",
	}}

	// The checkout: the fixture plus the hazard repos, committed with an
	// up-to-date lockfile, and a clone of it at a different path.
	checkout := filepath.Join(root, "checkout")
	say("Materializing fixture with hazard repos in %s", checkout)
	if err := ws.Materialize(fixture.FS, fixture.Root, checkout); err != nil {
		return nil, err
	}
	if err := addHazards(checkout); err != nil {
		return nil, err
	}
	for name, content := range map[string]string{"user.bazelrc": userBazelrc(cfg), ".gitignore": "/bazel-*\n"} {
		if err := os.WriteFile(filepath.Join(checkout, name), []byte(content), 0o644); err != nil {
			return nil, err
		}
	}
	sharedRC := filepath.Join(root, "shared.bazelrc")
	if err := os.WriteFile(sharedRC, []byte(sharedBazelrc(cfg)), 0o644); err != nil {
		return nil, err
	}
	lockOB := filepath.Join(root, "lock", "output_base")
	if r := runner.Run(ctx, checkout, cfg.StockBazel, "--bazelrc="+sharedRC, "--output_base="+lockOB, "mod", "deps", "--lockfile_mode=update"); r.Err != nil {
		return nil, fmt.Errorf("updating the lockfile: %w", r.Err)
	}
	runner.Run(ctx, checkout, cfg.StockBazel, "--bazelrc="+sharedRC, "--output_base="+lockOB, "shutdown")
	git := &ws.Git{Bin: cfg.Git, Dir: checkout, Runner: runner}
	rev, err := git.Init(ctx)
	if err != nil {
		return nil, err
	}
	elsewhere := filepath.Join(root, "elsewhere", "another", "checkout")
	if r := runner.Run(ctx, root, cfg.Git, "clone", "-q", checkout, elsewhere); r.Err != nil {
		return nil, r.Err
	}

	remote := cfg.RemoteCache != ""
	cfgR := cfg
	cfgR.RepoCacheMode = "remote"
	cfgL := cfg
	cfgL.RepoCacheMode = "local"

	if remote {
		// Upload every repository the checks need (hashing and building), so
		// remote snapshots see lazily served repositories, not fresh fetches.
		seedDir := filepath.Join(root, "seed")
		rc := filepath.Join(seedDir, "seed.bazelrc")
		if err := os.MkdirAll(seedDir, 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(rc, []byte(toolBazelrc(cfgR, seedDir, "")), 0o644); err != nil {
			return nil, err
		}
		type step struct {
			bazel string
			args  []string
		}
		steps := []step{{cfg.StockBazel, []string{"build", "//...", "--keep_going"}}}
		if cfg.CustomBazel != "" {
			steps = append(steps, step{cfg.CustomBazel, []string{"query", "//...", tools.HashFlag}}, step{cfg.CustomBazel, []string{"cquery", "//...", tools.HashFlag}})
		}
		say("Seeding the remote cache with the checkout's repositories...")
		for _, st := range steps {
			ob := filepath.Join(seedDir, filepath.Base(st.bazel), "output_base")
			runner.Run(ctx, checkout, st.bazel, append([]string{"--bazelrc=" + rc, "--output_base=" + ob}, st.args...)...)
			runner.Run(ctx, checkout, st.bazel, "--bazelrc="+rc, "--output_base="+ob, "shutdown")
		}
		removeAll(seedDir)
	}

	var out []DetResult
	for _, name := range cfg.Tools {
		if name == "target-determinator" {
			continue // keeps no snapshots: it hashes both revisions in one run
		}
		say("Determinism checks for %s:", name)
		d := detTool{cfg: cfg, ctx: ctx, name: name, rev: rev, say: say, root: root}
		out = append(out, d.run(runner, otherRunner, checkout, elsewhere, cfgL, cfgR, remote)...)
		if cfg.OnDetSave != nil {
			cfg.OnDetSave(out)
		}
	}
	return out, nil
}

type detTool struct {
	cfg       Config
	ctx       context.Context
	name, rev string
	root      string
	say       func(string, ...any)
}

// inst is one tool instance: a state dir (output base under it), a checkout,
// a bazelrc (caches, mode) and an environment.
func (d *detTool) inst(state, checkout, rc string, r *ws.Runner) (tools.Tool, error) {
	return tools.New(d.name, tools.Env{
		Runner: r, Workspace: checkout, StateDir: state, RC: rc,
		StockBazel: d.cfg.StockBazel, CustomBazel: d.cfg.CustomBazel, BazelDiff: d.cfg.BazelDiff, TD: d.cfg.TD,
	})
}

func (d *detTool) snap(t tools.Tool) (map[string]string, error) {
	if err := t.Prepare(d.ctx, d.rev); err != nil {
		return nil, err
	}
	h := map[string]string{}
	for k, v := range t.BaseHashes() {
		h[k] = v
	}
	return h, nil
}

// writeRC writes a tool bazelrc for mode into dir, with caches under cacheDir.
func writeRC(cfg Config, dir, cacheDir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	rc := filepath.Join(dir, "tool.bazelrc")
	return rc, os.WriteFile(rc, []byte(toolBazelrc(cfg, cacheDir, "")), 0o644)
}

func (d *detTool) run(runner, otherRunner *ws.Runner, checkout, elsewhere string, cfgL, cfgR Config, remote bool) []DetResult {
	var out []DetResult
	start := time.Now()
	record := func(check string, a, b map[string]string, errs ...error) {
		r := DetResult{Check: check, Tool: d.name, Seconds: time.Since(start).Seconds()}
		var msgs []string
		for _, e := range errs {
			if e != nil {
				msgs = append(msgs, firstLine(e.Error()))
			}
		}
		r.Err = strings.Join(msgs, "; ")
		for l, h := range a {
			if h2, ok := b[l]; ok {
				r.Compared++
				if h != h2 {
					r.Different = append(r.Different, l)
				}
			}
		}
		sort.Strings(r.Different)
		status := fmt.Sprintf("%d of %d targets differ", len(r.Different), r.Compared)
		if r.Err != "" {
			status += " (error: " + r.Err + ")"
		}
		d.say("  %-24s %6.1fs  %s", check, r.Seconds, status)
		out = append(out, r)
		start = time.Now()
	}
	// Every instance's state lives in <stateRoot>/<tool>; output base under it.
	a := filepath.Join(d.root, "a")
	ob := func(state string) string { return filepath.Join(state, d.name, "output_base") }
	wipe := func(t tools.Tool, dirs ...string) {
		t.Shutdown(d.ctx)
		for _, p := range dirs {
			removeAll(p)
		}
	}
	build := func(t tools.Tool) error {
		b, ok := t.(tools.Bazeler)
		if !ok {
			return fmt.Errorf("%s has no Bazel", d.name)
		}
		r := b.Bazel(d.ctx, "build", "//...", "--keep_going")
		if r.Err != nil {
			return fmt.Errorf("build: %w", r.Err)
		}
		return nil
	}

	cachesA := filepath.Join(a, d.name)
	rcA, err := writeRC(cfgL, filepath.Join(d.root, "rc", d.name), cachesA)
	if err != nil {
		return []DetResult{{Check: "setup", Tool: d.name, Err: err.Error()}}
	}
	tA, err := d.inst(a, checkout, rcA, runner)
	if err != nil {
		return []DetResult{{Check: "setup", Tool: d.name, Err: err.Error()}}
	}
	ref, err := d.snap(tA)
	if err != nil {
		tA.Shutdown(d.ctx)
		return []DetResult{{Check: "setup", Tool: d.name, Err: firstLine(err.Error())}}
	}
	start = time.Now()

	h, err := d.snap(tA)
	record("repeat", ref, h, err)

	tA.Shutdown(d.ctx)
	h, err = d.snap(tA)
	record("restart", ref, h, err)

	tA.Shutdown(d.ctx)
	if t, e := d.inst(a, checkout, rcA, otherRunner); e == nil {
		h, err = d.snap(t)
		t.Shutdown(d.ctx)
		record("environment", ref, h, err)
	}

	b := filepath.Join(d.root, "b")
	if t, e := d.inst(b, checkout, rcA, runner); e == nil {
		h, err = d.snap(t)
		wipe(t, filepath.Join(b, d.name))
		record("output_base_path", ref, h, err)
	}

	wipe(tA, ob(a))
	h, err = d.snap(tA)
	record("fresh_output_base", ref, h, err)

	wipe(tA, ob(a))
	if t, e := d.inst(a, elsewhere, rcA, runner); e == nil {
		h, err = d.snap(t)
		wipe(t, ob(a))
		record("checkout_path", ref, h, err)
	}

	wipe(tA, cachesA)
	if t, e := d.inst(a, checkout, rcA, runner); e == nil {
		tA = t
		h, err = d.snap(tA)
		record("fresh_caches", ref, h, err)
	}

	pre, err1 := d.snap(tA)
	err2 := build(tA)
	h, err = d.snap(tA)
	record("build_between", pre, h, err1, err2, err)
	wipe(tA, cachesA)

	if !remote {
		return out
	}
	// Remote checks: one state dir, recreated at the same path for each.
	r := filepath.Join(d.root, "r")
	cachesR := filepath.Join(r, d.name)
	rcR, err := writeRC(cfgR, filepath.Join(d.root, "rc-remote", d.name), cachesR)
	if err != nil {
		return out
	}
	rcL, err := writeRC(cfgL, filepath.Join(d.root, "rc-local", d.name), cachesR)
	if err != nil {
		return out
	}
	mk := func(rc string) tools.Tool {
		t, _ := d.inst(r, checkout, rc, runner)
		return t
	}

	tR := mk(rcR)
	hR, errR := d.snap(tR)
	wipe(tR, ob(r))
	tL := mk(rcL)
	h, err = d.snap(tL)
	wipe(tL, cachesR)
	record("remote_cache", hR, h, errR, err)

	// Toggle the flag in one output base: on, off, on.
	tR = mk(rcR)
	hR, errR = d.snap(tR)
	tR.Shutdown(d.ctx)
	tL = mk(rcL)
	h, err = d.snap(tL)
	tL.Shutdown(d.ctx)
	record("remote_off_same_output_base", hR, h, errR, err)
	tR = mk(rcR)
	h, err = d.snap(tR)
	wipe(tR, cachesR)
	record("remote_on_again", hR, h, errR, err)

	tR = mk(rcR)
	pre, err1 = d.snap(tR)
	err2 = build(tR)
	h, err = d.snap(tR)
	wipe(tR, cachesR)
	record("build_between_remote", pre, h, err1, err2, err)

	tR = mk(rcR)
	hR, errR = d.snap(tR)
	wipe(tR, ob(r))
	tL = mk(rcL)
	err2 = build(tL)
	h, err = d.snap(tL)
	wipe(tL, cachesR)
	record("remote_then_local_build", hR, h, errR, err2, err)
	return out
}

// addHazards adds repositories with known determinism hazards, each used by
// its own target in //hazard (whose outputs don't depend on the hazard):
//   - @hazard_symlink (//hazard:symlink) contains an absolute symlink to a file
//     in the repo. Bazel only rewrites absolute symlinks as relative in
//     repositories whose rule declares itself reproducible; this one doesn't.
//   - @hazard_path_attr (//hazard:path_attr) is a local repository with its own
//     absolute path in a string attribute, like rules_java's local_jdk
//     (fail_rule(message = "...<output_base>...")).
//   - @hazard_path_content (//hazard:path_content) is a fetched repository with
//     its own absolute path in a BUILD file comment, i.e. in a file's contents.
func addHazards(dir string) error {
	files := map[string]string{
		"hazard/defs.bzl": `"""Repositories with determinism hazards (added by hashbench -determinism)."""

def _hazard_repo_impl(rctx):
    rctx.file("real.txt", "hazard\n")
    header = ""
    tags = []
    if rctx.attr.kind == "symlink":
        rctx.symlink(rctx.path("real.txt"), "link.txt")
    elif rctx.attr.kind == "path_attr":
        tags = [str(rctx.path("."))]
    elif rctx.attr.kind == "path_content":
        header = "# Generated in %s\n" % rctx.path(".")
    rctx.file("BUILD.bazel", header + """
filegroup(
    name = "files",
    srcs = glob(["*.txt"]),
    tags = %r,
    visibility = ["//visibility:public"],
)
""" % tags)

hazard_repo = repository_rule(
    implementation = _hazard_repo_impl,
    attrs = {"kind": attr.string()},
)

hazard_local_repo = repository_rule(
    implementation = _hazard_repo_impl,
    attrs = {"kind": attr.string()},
    local = True,
)
`,
		"hazard/BUILD.bazel": `[genrule(
    name = kind,
    srcs = ["@hazard_%s//:files" % kind],
    outs = [kind + ".txt"],
    cmd = "cat $(SRCS) > $@",
) for kind in [
    "symlink",
    "path_attr",
    "path_content",
]]
`,
	}
	for p, c := range files {
		full := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(c), 0o644); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(filepath.Join(dir, "MODULE.bazel"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(`
# Determinism hazards (added by hashbench -determinism).
hazard_repo = use_repo_rule("//hazard:defs.bzl", "hazard_repo")

hazard_local_repo = use_repo_rule("//hazard:defs.bzl", "hazard_local_repo")

hazard_repo(name = "hazard_symlink", kind = "symlink")

hazard_local_repo(name = "hazard_path_attr", kind = "path_attr")

hazard_repo(name = "hazard_path_content", kind = "path_content")
`)
	return err
}
