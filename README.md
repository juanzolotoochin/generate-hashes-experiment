# generate-hashes-experiment

A reproducible, hermetic benchmark of **Bazel target-selection tools**, the tools
CI uses to decide which targets a change affects. It compares three approaches for
correctness and warm-cache performance:

| Tool | What it is |
|---|---|
| **custom** | A custom Bazel build (based on Bazel 9.2) whose `bazel query` accepts `--generate_hashes` and prints a content hash for every target. Changed hashes between two revisions = affected targets. |
| **custom-cquery** | The same custom Bazel running `bazel cquery --generate_hashes`: hashes of configured targets, so toolchains and build flags are included. |
| **bazel-diff** | [Tinder/bazel-diff](https://github.com/Tinder/bazel-diff) v60.0.0, unmodified, default options (`generate-hashes` + `get-impacted-targets`). Each external repo is hashed as one opaque blob. |
| **bazel-diff-fine** | The same, with `--fineGrainedHashExternalReposFile` listing the third-party repos (pip, `go_deps`, crates) found in `deps(//...)` on each revision, as CI setups commonly use it: those repos are hashed per target, reading their files from disk. |
| **target-determinator** | [bazel-contrib/target-determinator](https://github.com/bazel-contrib/target-determinator) v0.34.0, unmodified, with `-nocache_results`. |

## Results

Latest run: Linux x86_64, 16 CPUs, 47 scenarios, 43 targets. Full tables are in
[`results/summary.md`](results/summary.md), the interactive report in
[`results/report.html`](results/report.html) ([view online](https://juanzolotoochin.github.io/generate-hashes-experiment/results/report.html)), and raw data in
[`results/results.json`](results/results.json).

Each tool ran twice in the same work dir:
- **Local mode:** repos fetched from the internet into the tool's own empty caches.
- **`@remote`:** with `--experimental_remote_repo_contents_cache` against the
  bundled localhost cache, seeded with every revision's repos.

For how `--experimental_generate_hashes` hashes repos from the remote repo
contents cache without downloading them, see
[Query Hashes and Remote Repos](https://juanzolotoochin.github.io/generate-hashes-experiment/docs/).

My reading of these results is in [FINDINGS.md](FINDINGS.md). In the report,
"generate_hashes (query)" is the `custom` tool and "generate_hashes (cquery)" is
`custom-cquery`.

| Tool | Exact | Scenarios with misses | Missed targets | Over-selected targets | Recall | Precision | Warm no-change | Warm per change (median) |
|---|---|---|---|---|---|---|---|---|
| generate_hashes (query) | 25/47 | 1 | 10 | 150 | 94.4% | 52.7% | 0.1s | 0.2s |
| generate_hashes (cquery) | 25/47 | 0 | 0 | 143 | 100.0% | 55.3% | 0.7s | 1.3s |
| bazel-diff | 23/47 | 5 | 60 | 90 | 66.1% | 56.5% | 0.5s | 0.7s |
| bazel-diff-fine | 23/47 | 5 | 57 | 93 | 67.8% | 56.3% | 0.6s | 0.8s |
| bazel-diff-cquery | 26/47 | 2 | 20 | 144 | 88.7% | 52.2% | 4.1s | 5.0s |
| target-determinator | 28/47 | 0 | 0 | 106 | 100.0% | 62.5% | 9.9s | 11.4s |

Correctness with the remote repo contents cache is the same as in local mode
for every tool (see the `@remote` rows in the full tables).

**Determinism.** `hashbench -determinism` hashes the same revision twice with
one thing changed in between: a server restart, another environment, another
output base or checkout path, fresh caches, a build in between, or the remote
repo contents cache toggled. Targets whose hashes differ:

| Check | generate_hashes (query) | generate_hashes (cquery) | bazel-diff | bazel-diff-fine | bazel-diff-cquery | target-determinator |
|---|---|---|---|---|---|---|
| Output base at another path | 1 of 46 | 1 of 46 | 0 | 0 | 2 of 109 | 0 |
| Remote repo contents cache on, then off | 0 | 0 | 0 | 8 of 109 | 0 | 0 |
| The flag toggled in the same output base | 0 | 0 | 0 | 8 of 109 | 0 | 0 |
| A build between snapshots, remote repo contents cache on | 0 | 0 | 0 | 5 of 109 | 0 | 0 |
| Remote snapshot, then a local build and a local snapshot | 0 | 0 | 0 | 8 of 109 | 0 | 0 |
| Repeat, server restart, another user and home, fresh output base, another checkout path, fresh caches, a local build between | 0 | 0 | 0 | 0 | 0 | 0 |

- **generate_hashes (cquery)** never missed a target, at about 1s per change
  versus about 11s for target-determinator. Its hashes don't depend on whether
  external repos are materialized.
- **generate_hashes (query)** is the fastest tool (0.2s per change) and catches
  toolchain changes through the toolchain repos' definitions, including the
  contents of files those definitions point to, such as patches. Its only
  misses are the `.bazelrc` `--copt` change, which is invisible to an
  unconfigured query. Both generate_hashes modes over-select when an external
  repo's files change without changing any output (`toolchain_patch_noop`,
  `bzlmod_patch_noop`), and both depend on the output base path for a fetched
  repo that writes its own path into a file (`//hazard:path_content`).
- **bazel-diff** misses every toolchain change (Go SDK, Rust, LLVM, the Go SDK
  patch) and the `.bazelrc` change.
- **bazel-diff-fine** hashes third-party repos (pip, `go_deps`, crates) per
  target by reading their files from disk. With the remote repo contents cache,
  those repos are served lazily and aren't on disk, so it computes different
  hashes for the same commit: 8 Python and Rust targets here, depending on the
  flag and on whether a build materialized the files. Default bazel-diff hashes
  external repos as opaque blobs and isn't affected.
- **bazel-diff-cquery** (`--useCquery`) catches the toolchain version bumps and
  the `.bazelrc` change, but misses the Go SDK patch edit and a rules_multitool
  version bump, takes about 5s per change, and depends on the output base path
  for a local repo with its path in an attribute (`//hazard:path_attr`).
- **target-determinator** never missed a target and has the best precision, but
  it's the slowest. It keeps no snapshots: it checks out both revisions in one
  git repository.
- **Downloads.** With the remote repo contents cache, 93–100% of every tool's
  downloads come from the remote cache. A cold setup of generate_hashes (query)
  downloads 389 MB, most of it BUILD and `.bzl` files.

### Fixes applied

The benchmark uncovered these issues; the fixture includes fixes for the first
two.

1. **gazelle:** `go.env` contained output-base paths, so every `go_deps` repo
   missed the repo contents cache in any fresh output base. Fixed upstream in
   0.53.0 (bazel-gazelle #2385); the fixture uses 0.54.0.
2. **rules_python:** interpreter repos contain `__pycache__` symlinks that point
   outside the repo, so Bazel never uploads them to a remote repo contents
   cache. Fixed on upstream main (d03bc77c), not yet released. The fixture
   applies the fix as a patch to 2.4.2, plus
   `--repo_env=RULES_PYTHON_PYCACHE_DIR=/dev/null`.
3. **Bazel:** it doesn't upload repos with symlinks that leave the repo, and it
   says nothing when it skips them (bazelbuild/bazel#30160). Reported only.
4. **BuildBuddy:** `llvm-project` misses the remote repo contents cache there,
   but hits with a local cache. Reported only, unresolved.

rules_distroless stays at 0.8.0 because 0.9 replaced its `apt` extension API.
All other modules are at their latest BCR releases.

## Running it

```sh
/path/to/custom-bazel run //cmd/hashbench -- -custom-bazel=/path/to/custom-bazel
```

That's the whole benchmark. Then open `results/report.html`. A Markdown
summary (`summary.md`) and the raw data (`results.json`) are written next to it.
All three are rewritten after every scenario, so you can watch a run fill in, and
git keeps the previous results. Bazel state, checkouts and logs go in
`hashbench-work/`.

The custom Bazel under test builds the harness and is the only thing taken from
your machine besides `git`, which versions the benchmark workspace. Everything
else is pinned by sha256 in [`MODULE.bazel`](MODULE.bazel) and supplied by
`bazel run`: stock Bazel 9.2.0, bazel-diff, target-determinator, a static bash
and a static busybox. The harness is pure Go, built with the hermetic Go SDK.

Requirements: Linux x86_64, `git`, network access (Bazel Central Registry,
GitHub releases, busybox.net, proxy.golang.org, PyPI, crates.io,
static.rust-lang.org, gcr.io, packages.wolfi.dev, snapshot.debian.org), and about
15 GB of disk. The first run fetches and builds the fixture's toolchains (LLVM,
Go, Rust, Python). Every run starts each tool from empty caches; later runs
reuse `~/.cache/hashbench` for the oracle and the remote cache.

### Getting the custom Bazel

The custom Bazel is not distributed yet. Build it from the fork and pass it as
shown above. <!-- TODO: link the fork and build instructions once public. -->

### Measuring scaling

```sh
/path/to/custom-bazel run //cmd/hashbench -- -custom-bazel=/path/to/custom-bazel \
    -scale=0,2000,8000,16000 -tools=custom,bazel-diff
```

Each size adds that many generated service packages (Go, Python, C++, Rust,
filegroup and custom-rule targets, chained to each other and to the fixture's
libraries) and runs in its own `hashbench-work/scale-N`, with results in
`results/scale-N`. The sweep writes `results/scaling.html` and `scaling.md`
with per-tool timings by
workspace size. For runs with `-scale` above 0, the oracle is off and a
representative scenario subset runs, unless you override them.

### Remote repo contents cache

To measure the tools with `--experimental_remote_repo_contents_cache`, run the
local pass and then the remote pass with its ground truth:

```sh
/path/to/custom-bazel run //cmd/hashbench -- -custom-bazel=/path/to/custom-bazel \
    -tools=custom,custom-cquery,bazel-diff,target-determinator
/path/to/custom-bazel run //cmd/hashbench -- -custom-bazel=/path/to/custom-bazel \
    -tools=custom,custom-cquery,bazel-diff,target-determinator \
    -repo-cache-mode=remote -reuse-truth=results/results.json
```

The remote pass reads the local pass's results before it starts and writes the
combined results back to `results/`.

By default the remote pass uses the bundled cache server (`cmd/remotecache`, a
minimal Remote Execution API cache in Rust), which the harness starts on
localhost with its data in the cache dir. To use a real remote cache instead,
pass `-remote-cache=grpcs://...` and put credentials in an untracked bazelrc
passed with `-bazelrc` (e.g. `auto.bazelrc`, which is gitignored, containing
`common --remote_header=x-buildbuddy-api-key=<key>`).

The remote pass reports every tool as `<tool>@remote` next to the local
results. Before each revision is measured, a separate seeder fetches its
repositories (the custom Bazel's `--generate_hashes` queries, stock `query` and
`cquery`), so they are in the remote cache before any tool asks for them.

### Options

| Flag | Default | Meaning |
|---|---|---|
| `-custom-bazel` | (required) | Bazel binary that supports `query --generate_hashes`. |
| `-tools` | `custom,bazel-diff,target-determinator` | Which tools to benchmark (also available: `custom-cquery`, `bazel-diff-fine` with `--fineGrainedHashExternalReposFile`, and `bazel-diff-cquery` with `--useCquery`). Reports name `custom` and `custom-cquery` `generate_hashes (query)` and `generate_hashes (cquery)`. |
| `-reuse-truth` | | `results.json` of an earlier run: reuse its ground truth instead of re-running the oracle per scenario, and merge its tools' scores. Handy for adding a tool. |
| `-scenarios` | all | Comma-separated scenario names or categories, e.g. `source,toolchain` or `go_dep_bump`. See `-list`. |
| `-list` | | Print the scenarios and exit. |
| `-scale` | `0` | Comma-separated workspace sizes (generated packages). Several values run a sweep. |
| `-oracle` | on at scale 0 | Build every target to compute ground truth. |
| `-perf-reps` | `5` | Warm repetitions of the no-change scenario. |
| `-work-dir` | `hashbench-work` | Benchmark checkout, per-tool output bases and logs (relative to where you ran `bazel run`). |
| `-results-dir` | `results/` | Where `report.html`, `summary.md` and `results.json` go. |
| `-cache-dir` | `~/.cache/hashbench` | Shared repository, repo-contents and disk caches, plus the toolbox. |
| `-keep-servers` | `false` | Leave the Bazel servers running afterwards. |
| `-repo-cache-mode` | `local` | `local`, or `remote` to add `--experimental_remote_repo_contents_cache` (tools are reported as `<tool>@remote`). |
| `-remote-cache` | `local` | Remote cache for `-repo-cache-mode=remote`: `local` (bundled server on localhost) or a `grpc://`/`grpcs://` URL. |
| `-bazelrc` | | Comma-separated extra bazelrc files (e.g. credentials) imported by every Bazel invocation, by path. |

Every command the harness runs is logged to `hashbench-work/commands.log`.

## Hermeticity

- **No host toolchains.** The fixture's toolchains are all downloaded and
  pinned: the fully hermetic `llvm` C/C++ toolchain (no sysroot), Go SDK, Rust,
  and a Python interpreter. `BAZEL_DO_NOT_DETECT_CPP_TOOLCHAIN=1` stops Bazel from
  ever probing for a host compiler.
- **No host tools in build actions.** The fixture's custom rules use a small
  Go tool rather than shell commands, the oracle's digest action is the harness
  binary itself, and `--shell_executable` is the pinned static bash.
- **No host tools on PATH.** Every process the harness starts (Bazel, the
  tools, repository rules) gets a scrubbed environment whose `PATH` is only that
  toolbox plus `git`.
- What remains from the host: the Linux kernel, and glibc. The official Bazel
  binaries, your custom Bazel and some prebuilt toolchains (the official
  `rustc`, the Python interpreter) are dynamically linked against it.

## What is measured

### Scenarios

Each scenario is one change committed on top of the same base revision of the
fixture (`-list` prints them all). They cover:

- **Source files:** Go, Python, C++ header and Rust edits; comment-only edits; a
  file picked up by `glob()`; unreferenced files; renames; exec-bit changes on a
  runfile and on a source file.
- **BUILD files:** meaningful changes (deps, `copts`, test `size`, deleting a
  target, visibility) and meaningless ones (comments, kwarg order, unused `load`).
- **.bzl files:** a rule implementation change, a macro change, a comment, an
  unused function.
- **Third-party dependencies:** Go module (go.mod/go.sum), pip wheel (hashed
  lock), Rust crate (Cargo.lock), rules_multitool tool, rules_oci and rules_img
  base image digests, a rules_apko package-set change, and rules_distroless
  changes: one package, a whole snapshot move, and a snapshot move where every
  package is byte-identical.
- **Module graph:** a `bazel_dep` bump and a comment in `MODULE.bazel`.
- **Patches:** adding a patch to a module (`single_version_override`), and
  editing only an existing patch file, for a module (rules_oci) and a
  toolchain (the Go SDK, via `go_sdk.download(patches = …)`). Each has a
  variant that changes real inputs and one that only changes a README.
- **Toolchains:** Go SDK, Rust, Python and hermetic LLVM version changes.
- **Build configuration:** a `.bazelrc` flag that changes C++ compilation.

Dependency scenarios swap in real, pre-generated alternate files (produced by
each ruleset's own lock tooling) from `internal/scenario/data`.

### Ground truth

The oracle builds `//...` with stock Bazel and an aspect, injected with
`--inject_repository` so the fixture stays untouched, that records each
top-level target's output files, runfiles and executable, plus its
test-execution attributes (`size`, `timeout`, `args`, `env`, …). Two
fingerprints are computed per target:

- **Needs rebuild (the truth tools are scored against).** A Merkle fingerprint
  over the action graph from `bazel aquery`. Every listed file contributes its
  producing action's key plus, recursively, the fingerprints of that action's
  inputs. Source files contribute their contents, and source files a target
  ships directly also contribute their executable bit. A target needs
  rebuilding when this changes: some action needed to build or test it has
  different inputs, so CI must run it to know the result. This includes
  everything toolchain resolution and build flags feed into the actions, which
  query-level tools can't see.
- **Outputs changed (informational).** The same but over output *contents*. The
  gap between the two is what Bazel's early cutoff would skip (e.g. a comment
  edit recompiles a library whose outputs are byte-identical).

A target is scored as:

- **missed** if it needs rebuilding but the tool didn't select it. This is the
  dangerous error: CI would skip work it needs to do.
- **over-selected** if the tool selected it but it doesn't need rebuilding. This
  is wasted CI work.

Only main-repository targets the oracle fingerprints are scored. Labels outside
that set (source files, external repositories) are ignored. Targets without
default outputs (for example some `oci_image` helper targets) never need
rebuilding by this definition, so selecting them counts as over-selection.

### Performance

All tools run against warm Bazel servers, each in its own output base. The base
revision is processed once up front (hashes for the hash-based tools, a warm-up
run for target-determinator), as it would be in CI. Timings are the wall-clock
time to produce the affected set for the scenario's head revision:

- **custom:** `query //... --generate_hashes` on the head revision.
- **bazel-diff:** `generate-hashes` on the head plus `get-impacted-targets`.
- **target-determinator:** a full run, which by design evaluates both revisions
  (it checks the base out in place, since the benchmark checkout is always clean).

Reports show the median of the no-change scenario (repeated `-perf-reps` times)
and medians per scenario category.

### Consistency

With `--experimental_remote_repo_contents_cache`, cached external repos are
served lazily and their files are not on disk. A tool that reads external
files from disk can then compute different hashes for the same revision
depending on what is materialized. When the results contain both modes, the
report compares each hash-based tool's base-revision hashes between local and
remote mode: they should be identical.

### Downloads

Every tool has its own repository and repo-contents caches, so what it fetches
is its own. Each tool's Bazel reaches the network only through its own proxy
in the harness: internet downloads go through `HTTPS_PROXY`, and the remote
cache through `--remote_proxy=unix:…`. The report shows the bytes each tool
received:

- **Setup:** the first run on the base revision, from empty caches.
- **Per scenario:** the head revision, e.g. a toolchain bump downloads the new
  toolchain.

The oracle uses separate, shared caches and is not measured.

## The fixture

`fixture/_workspace` is a small polyglot monorepo with one package per ecosystem:

| Area | Rulesets | Targets |
|---|---|---|
| C/C++ | rules_cc + the fully hermetic `llvm` toolchain module | `//cc/lib`, `//cc/app` |
| Go | rules_go + gazelle `go_deps` | `//go/greet` (uuid), `//go/diff` (go-difflib), `//go/cmd/greeter` |
| Python | rules_python (hermetic interpreter, hashed pip lock) | `//py/lib` (six), `//py/extra` (idna), `//py/app` |
| Rust | rules_rs (toolchain + crates from Cargo.lock) | `//rust/lib`, `//rust/app` |
| Containers | rules_oci, rules_img, rules_apko, rules_distroless | `//oci`, `//img`, `//apko`, `//distroless` |
| Tools | rules_multitool (jq), a custom rule, macro and Go tool | `//tools`, `//data` |

Files that would make the fixture part of the outer workspace or a separate Go
module (`BUILD.bazel`, `MODULE.bazel`, `go.mod`) are stored with a `.fixture`
suffix, which the harness strips when it materializes the workspace.

## Repository layout

```
cmd/hashbench/         CLI entry point; `bazel run` passes the pinned tools
fixture/_workspace/    the embedded benchmark workspace
internal/scenario/     scenario definitions and alternate dependency files (data/)
internal/tools/        adapters for custom, bazel-diff and target-determinator
internal/oracle/       ground-truth aspect (repo/), digest tool and fingerprinting
internal/bench/        orchestration, toolbox, scoring
internal/report/       HTML and Markdown rendering (results and scaling)
internal/scale/        generated packages for -scale
```

### Adding a scenario

Append a `scenario.Scenario` to `internal/scenario/scenarios.go` with a list of
edits (`Append`, `Prepend`, `Replace`, `Write`, `Rename`, `Chmod`, or
`CopyData` for a file under `internal/scenario/data`). Each edit fails loudly if
its target text isn't found, so a scenario can't silently become a no-op.

## License

MIT, see [LICENSE](LICENSE).
