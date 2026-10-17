# generate-hashes-experiment

A reproducible, hermetic benchmark of **Bazel target-selection tools**, the tools
CI uses to decide which targets a change affects. It compares three approaches for
correctness and warm-cache performance:

| Tool | What it is |
|---|---|
| **custom** | A custom Bazel build (based on Bazel 9.2) whose `bazel query` accepts `--generate_hashes` and prints a content hash for every target. Changed hashes between two revisions = affected targets. |
| **bazel-diff** | [Tinder/bazel-diff](https://github.com/Tinder/bazel-diff) v60.0.0, unmodified, default options (`generate-hashes` + `get-impacted-targets`). |
| **target-determinator** | [bazel-contrib/target-determinator](https://github.com/bazel-contrib/target-determinator) v0.34.0, unmodified, with `-nocache_results`. |

## Running it

```sh
/path/to/custom-bazel run //cmd/hashbench -- -custom-bazel=/path/to/custom-bazel
```

That's the whole benchmark. Then open `hashbench-work/report.html`. A Markdown
summary (`summary.md`) and the raw data (`results.json`) are written next to it.

The custom Bazel under test builds the harness and is the only thing taken from
your machine besides `git`, which versions the benchmark workspace. Everything
else is pinned by sha256 in [`MODULE.bazel`](MODULE.bazel) and supplied by
`bazel run`: stock Bazel 9.2.0, bazel-diff, target-determinator, a static bash
and a static busybox. The harness is pure Go, built with the hermetic Go SDK.

Requirements: Linux x86_64, `git`, network access (Bazel Central Registry,
GitHub releases, busybox.net, proxy.golang.org, PyPI, crates.io,
static.rust-lang.org, gcr.io, packages.wolfi.dev, snapshot.debian.org), and about
15 GB of disk. The first run fetches and builds the fixture's toolchains (LLVM,
Go, Rust, Python). Later runs reuse `~/.cache/hashbench` and
`hashbench-work/state` and start warm.

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
libraries) and runs in its own `hashbench-work/scale-N`. The sweep writes
`hashbench-work/scaling.html` and `scaling.md` with per-tool timings by
workspace size. For runs with `-scale` above 0, the oracle is off and a
representative scenario subset runs, unless you override them.

### Options

| Flag | Default | Meaning |
|---|---|---|
| `-custom-bazel` | (required) | Bazel binary that supports `query --generate_hashes`. |
| `-tools` | `custom,bazel-diff,target-determinator` | Which tools to benchmark. |
| `-scenarios` | all | Comma-separated scenario names or categories, e.g. `source,toolchain` or `go_dep_bump`. See `-list`. |
| `-list` | | Print the scenarios and exit. |
| `-scale` | `0` | Comma-separated workspace sizes (generated packages). Several values run a sweep. |
| `-oracle` | on at scale 0 | Build every target to compute ground truth. |
| `-perf-reps` | `5` | Warm repetitions of the no-change scenario. |
| `-work-dir` | `hashbench-work` | Benchmark checkout, per-tool output bases, logs and results (relative to where you ran `bazel run`). |
| `-cache-dir` | `~/.cache/hashbench` | Shared repository, repo-contents and disk caches, plus the toolbox. |
| `-keep-servers` | `false` | Leave the Bazel servers running afterwards. |

Every command the harness runs is logged to `hashbench-work/commands.log`.

## Hermeticity

- **No host toolchains.** The fixture's toolchains are all downloaded and
  pinned: the fully hermetic `llvm` C/C++ toolchain (no sysroot), Go SDK, Rust,
  and a Python interpreter. `BAZEL_DO_NOT_DETECT_CPP_TOOLCHAIN=1` stops Bazel from
  ever probing for a host compiler.
- **No host tools in build actions.** Every build runs in Bazel's hermetic
  Linux sandbox (`--experimental_use_hermetic_linux_sandbox`, strategy
  `linux-sandbox` only). Inside it, `/bin` and `/usr/bin` contain only the
  pinned static bash and busybox, and `--shell_executable` is that bash. The
  fixture's own custom rules use a small Go tool rather than shell commands.
  The oracle's digest action is the hermetic harness binary itself.
- **No host tools on PATH.** Every process the harness starts (Bazel, the
  tools, repository rules) gets a scrubbed environment whose `PATH` is only that
  toolbox plus `git`.
- What remains from the host: the Linux kernel, and the glibc that the official
  Bazel binaries (and your custom Bazel) are dynamically linked against.

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
and medians per scenario category. Cold-cache performance is deliberately not
measured.

## The fixture

`fixture/_workspace` is a small polyglot monorepo with one package per ecosystem:

| Area | Rulesets | Targets |
|---|---|---|
| C/C++ | rules_cc + the fully hermetic `llvm` toolchain module | `//cc/lib`, `//cc/app` |
| Go | rules_go + gazelle `go_deps` | `//go/greet`, `//go/cmd/greeter` |
| Python | rules_python (hermetic interpreter, hashed pip lock) | `//py/lib`, `//py/app` |
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
