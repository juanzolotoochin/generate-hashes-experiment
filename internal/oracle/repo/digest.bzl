"""Ground-truth oracle: digests what each top-level target produces.

Applied with --aspects to every target matched by //... . For each target it
hashes the files in DefaultInfo (outputs), its default runfiles and its
executable, including file modes. For tests it also records the attributes that
change how the test runs without changing its outputs. Two builds that produce
the same digest file for a target would give CI identical results for it.
"""

_TEST_ATTRS = ["args", "env", "env_inherit", "flaky", "shard_count", "size", "timeout"]

def _digest_impl(target, ctx):
    if not DefaultInfo in target:
        return []
    info = target[DefaultInfo]
    files = info.files.to_list()
    if info.default_runfiles:
        files.extend(info.default_runfiles.files.to_list())
    if info.files_to_run and info.files_to_run.executable:
        files.append(info.files_to_run.executable)

    attrs = {}
    if ctx.rule.kind.endswith("_test") or ctx.rule.kind == "test_suite":
        for name in _TEST_ATTRS:
            if hasattr(ctx.rule.attr, name):
                attrs[name] = str(getattr(ctx.rule.attr, name))

    out = ctx.actions.declare_file(ctx.label.name + ".hashbench_digest")
    args = ctx.actions.args()
    args.add(out)
    args.add(str(ctx.label))
    args.add(json.encode(attrs))
    args.add_all(depset(files), expand_directories = False)
    ctx.actions.run(
        executable = ctx.executable._digest,
        arguments = [args],
        inputs = depset(files),
        outputs = [out],
        mnemonic = "HashbenchDigest",
        progress_message = "Digesting %{label}",
    )
    return [OutputGroupInfo(hashbench_digest = depset([out]))]

digest_aspect = aspect(
    implementation = _digest_impl,
    attrs = {
        "_digest": attr.label(
            default = Label("//:digest"),
            allow_single_file = True,
            executable = True,
            cfg = "exec",
        ),
    },
)
