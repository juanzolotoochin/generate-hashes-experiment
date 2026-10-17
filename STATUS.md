# Status

Work in progress (October 2026). `results/` is from earlier runs and does not
reflect the changes below yet.

## Done since the last published results

- Patch scenarios (rules_oci and the Go SDK, add/edit/no-op).
- Download metric: per-tool caches and a byte-counting proxy per tool
  (internet and remote cache), setup and per-scenario downloads in the report.
- Remote mode (`-repo-cache-mode=remote`) with
  `--experimental_remote_repo_contents_cache`, a seeder, and a bundled local
  remote cache in Rust (`cmd/remotecache`, rules_rs).
- Repo contents cache support table by module.
- Fixture bumped to the latest BCR releases (rules_python 2.4.2, gazelle
  0.54.0, llvm 0.8.24, rules_cc 0.2.26, tar.bzl 0.10.9, rules_img 0.3.23,
  rules_distroless 0.9.4), plus a rules_python patch so interpreter repos are
  cacheable. Not validated yet.

## Fixes applied (for the report)

1. gazelle: `go.env` held output-base paths, so `go_deps` repos missed the
   repo contents cache in every fresh output base. Fixed upstream in 0.53.0
   (bazel-gazelle #2385). Version bump.
2. rules_python: `__pycache__` symlinks leaving the repo, so interpreter repos
   were never uploaded. Fixed on upstream main (d03bc77c), unreleased. Patch
   plus `--repo_env=RULES_PYTHON_PYCACHE_DIR=/dev/null`.
3. Bazel: a repo with symlinks leaving it is silently not uploaded to the
   remote repo contents cache (bazelbuild/bazel#30160). Reported only.
4. BuildBuddy: `llvm-project` misses there but hits on a local cache. Reported
   only, unresolved.

## Next

1. Validate the bumped fixture (all targets build, scenario data still valid).
2. Commit a `MODULE.bazel.lock` for the fixture (and per-scenario lockfiles)
   instead of `--lockfile_mode=off`.
3. Rerun the miss inventory (expect 0 misses), then the local and remote passes
   with the current custom Bazel, and publish.
