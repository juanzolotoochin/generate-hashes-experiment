// Compiles the trimmed protos with protox (a pure-Rust protobuf compiler), so
// no protoc is needed, and generates the tonic server code.
fn main() -> Result<(), Box<dyn std::error::Error>> {
    let dir = std::path::Path::new(&std::env::var("CARGO_MANIFEST_DIR")?).join("proto");
    let fds = protox::compile(["remote_execution.proto", "bytestream.proto"], [&dir])?;
    tonic_build::configure().build_client(false).compile_fds(fds)?;
    Ok(())
}
