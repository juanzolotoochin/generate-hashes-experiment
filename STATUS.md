# Status

`results/` is current (October 2026): five tools, 47 scenarios, with the remote
repo contents cache off and on, using a bundled localhost cache.

## Open issues

- BuildBuddy: `llvm-project` misses its remote repo contents cache but hits with
  a local cache. Not investigated further.
- rules_distroless is held at 0.8.0. Migrating to 0.9 (new `apt` API) needs new
  lock data for the three distroless scenarios.

## Next steps

1. Scaling sweep: `-scale=0,2000,8000,16000 -tools=custom,custom-cquery,bazel-diff`.
2. Link the custom Bazel fork and its build instructions in the README.
