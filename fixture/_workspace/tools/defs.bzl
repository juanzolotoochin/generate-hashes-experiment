"""A custom rule and macro, used by the .bzl scenarios."""

def _text_bundle_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".txt")
    args = ctx.actions.args()
    args.add("bundle")
    args.add("--out", out)
    args.add("--header", ctx.attr.header)
    args.add_all(ctx.files.srcs)
    ctx.actions.run(
        executable = ctx.executable._tool,
        arguments = [args],
        inputs = ctx.files.srcs,
        outputs = [out],
        mnemonic = "TextBundle",
    )
    return [DefaultInfo(files = depset([out]))]

text_bundle = rule(
    implementation = _text_bundle_impl,
    attrs = {
        "header": attr.string(default = "# bundle"),
        "srcs": attr.label_list(allow_files = True),
        "_tool": attr.label(
            default = "//tools/fixturetool",
            executable = True,
            cfg = "exec",
        ),
    },
)

def bundle_pair(name, srcs, **kwargs):
    """Declares <name>_a and <name>_b bundles over the same sources."""
    text_bundle(name = name + "_a", srcs = srcs, header = "# a", **kwargs)
    text_bundle(name = name + "_b", srcs = srcs, header = "# b", **kwargs)
