"""Rules in a separate file; .bzl scenarios on defs.bzl must not affect them."""

def _upper_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".txt")
    ctx.actions.run(
        executable = ctx.executable._tool,
        arguments = ["upper", ctx.file.src.path, out.path],
        inputs = [ctx.file.src],
        outputs = [out],
        mnemonic = "Upper",
    )
    return [DefaultInfo(files = depset([out]))]

upper = rule(
    implementation = _upper_impl,
    attrs = {
        "src": attr.label(allow_single_file = True),
        "_tool": attr.label(
            default = "//tools/fixturetool",
            executable = True,
            cfg = "exec",
        ),
    },
)

def _tool_output_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".txt")
    args = ctx.actions.args()
    args.add("stdout")
    args.add("--out", out)
    args.add("--")
    args.add(ctx.executable.tool)
    args.add_all(ctx.attr.args)
    args.add_all(ctx.files.srcs)
    ctx.actions.run(
        executable = ctx.executable._tool,
        arguments = [args],
        inputs = ctx.files.srcs,
        tools = [ctx.executable.tool],
        outputs = [out],
        mnemonic = "ToolOutput",
    )
    return [DefaultInfo(files = depset([out]))]

tool_output = rule(
    implementation = _tool_output_impl,
    doc = "Runs a tool on srcs and captures its stdout (a hermetic genrule).",
    attrs = {
        "args": attr.string_list(),
        "srcs": attr.label_list(allow_files = True),
        "tool": attr.label(executable = True, cfg = "exec", allow_files = True),
        "_tool": attr.label(
            default = "//tools/fixturetool",
            executable = True,
            cfg = "exec",
        ),
    },
)
