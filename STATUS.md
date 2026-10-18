# Status

`results/` is current (October 2026): five tools, 47 scenarios, with the remote
repo contents cache off and on, using a bundled localhost cache.

## Open issues

- Building C++ with `--experimental_remote_repo_contents_cache` fails, with
  stock Bazel 9.2.0 and the fork alike (seen October 18, 2026, against the
  bundled localhost cache). `bazel build //... --keep_going` with the remote
  repo contents cache on fails to compile libc++ from
  `@llvm++llvm+llvm-project//libcxx`: `'stddef.h' file not found`, `use of
  undeclared identifier 'UINT32_C'`, `reference to unresolved using declaration`
  (`using ::uint8_t _LIBCPP_USING_IF_EXISTS` in `libcxx/include/cstdint`). The
  same build succeeds with the flag off. Probably headers from a lazily served
  repo (or behind a symlink in one) never reach the compile sandbox. Not
  investigated yet; if it reproduces in a small workspace, report it to Bazel.
  Repro: `hashbench -determinism` (the `build_between_remote` check and the
  remote cache seeding build; see `<work-dir>/determinism/commands.log`).
  Python, Go and Rust targets still build.
- BuildBuddy: `llvm-project` misses its remote repo contents cache but hits with
  a local cache. Not investigated further.
- rules_distroless is held at 0.8.0. Migrating to 0.9 (new `apt` API) needs new
  lock data for the three distroless scenarios.

## Next steps

1. Scaling sweep: `-scale=0,2000,8000,16000 -tools=custom,custom-cquery,bazel-diff`.
2. Link the custom Bazel fork and its build instructions in the README.
