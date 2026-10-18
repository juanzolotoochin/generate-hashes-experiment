package bench

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/juanzolotoochin/generate-hashes-experiment/internal/tools"
	"github.com/juanzolotoochin/generate-hashes-experiment/internal/ws"
)

// relocate hashes the base revision again with every hash-based tool, as a
// second machine or CI worktree would: from a clone of the checkout at a
// different path, with fresh output bases and caches at different paths.
// Hashes that embed an absolute path (the checkout, the output base, a cache)
// then differ from the tool's hashes in res.BaseHashes. Everything lives under
// <WorkDir>/relocated, which is recreated on every run.
func relocate(ctx context.Context, cfg Config, runner *ws.Runner, logf *os.File, repo, baseRev string, res *Results, say func(string, ...any)) error {
	root := filepath.Join(cfg.WorkDir, "relocated")
	if err := removeAll(root); err != nil {
		return err
	}
	checkout := filepath.Join(root, "elsewhere", "checkout")
	state := filepath.Join(root, "state")
	if err := os.MkdirAll(state, 0o755); err != nil {
		return err
	}
	if r := runner.Run(ctx, cfg.WorkDir, cfg.Git, "clone", "-q", repo, checkout); r.Err != nil {
		return r.Err
	}
	git := &ws.Git{Bin: cfg.Git, Dir: checkout, Runner: runner}
	if err := git.Reset(ctx, baseRev); err != nil {
		return err
	}
	for _, name := range cfg.Tools {
		if name == "target-determinator" {
			continue // compares both revisions in one run, keeps no snapshots
		}
		dir := filepath.Join(state, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		rc := filepath.Join(dir, "tool.bazelrc")
		if err := os.WriteFile(rc, []byte(toolBazelrc(cfg, dir, "")), 0o644); err != nil {
			return err
		}
		t, err := tools.New(name, tools.Env{
			Runner:    &ws.Runner{Log: logf, Env: runner.Env},
			Workspace: checkout, StateDir: state, RC: rc, Suffix: "@relocated",
			StockBazel: cfg.StockBazel, CustomBazel: cfg.CustomBazel, BazelDiff: cfg.BazelDiff, TD: cfg.TD,
		})
		if err != nil {
			return err
		}
		start := time.Now()
		say("Hashing the base revision with %s from another checkout and output base...", name)
		err = t.Prepare(ctx, baseRev)
		if h := t.BaseHashes(); err == nil && h != nil {
			res.BaseHashes[t.Name()] = h
			say("  %s done in %s", t.Name(), time.Since(start).Round(time.Second))
		} else if err != nil {
			say("  warning: %s failed: %s", t.Name(), firstLine(err.Error()))
		}
		t.Shutdown(ctx)
	}
	return nil
}
