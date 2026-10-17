//! A minimal Bazel remote cache for hashbench: the Remote Execution API v2
//! ActionCache, ContentAddressableStorage and Capabilities services plus
//! google.bytestream, backed by a directory on local disk. SHA-256 only, no
//! compression, no eviction. Blobs are verified on upload; action results are
//! stored as given.
//!
//! Usage: remotecache --dir DIR [--listen 127.0.0.1:0]
//! Prints "listening on ADDR" once it accepts connections.

use std::path::PathBuf;
use std::pin::Pin;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;

use prost::Message;
use sha2::{Digest as _, Sha256};
use tokio::io::{AsyncReadExt, AsyncSeekExt, AsyncWriteExt};
use tokio_stream::{Stream, StreamExt};
use tonic::{Request, Response, Status, Streaming};

mod reapi {
    tonic::include_proto!("build.bazel.remote.execution.v2");
}
mod bytestream {
    tonic::include_proto!("google.bytestream");
}

use bytestream::byte_stream_server::{ByteStream, ByteStreamServer};
use reapi::action_cache_server::{ActionCache, ActionCacheServer};
use reapi::capabilities_server::{Capabilities, CapabilitiesServer};
use reapi::content_addressable_storage_server::{ContentAddressableStorage, ContentAddressableStorageServer};

const MAX_BATCH_BYTES: i64 = 4 << 20;
const MAX_MESSAGE_BYTES: usize = 16 << 20;
const READ_CHUNK: usize = 1 << 20;
const SHA256: i32 = 1;
const SYMLINKS_ALLOWED: i32 = 2;

#[derive(Clone)]
struct Store {
    root: Arc<PathBuf>,
    tmp_seq: Arc<AtomicU64>,
}

impl Store {
    fn new(root: PathBuf) -> std::io::Result<Self> {
        for d in ["cas", "ac", "tmp"] {
            std::fs::create_dir_all(root.join(d))?;
        }
        Ok(Store { root: Arc::new(root), tmp_seq: Arc::new(AtomicU64::new(0)) })
    }

    fn cas_path(&self, hash: &str) -> PathBuf {
        self.root.join("cas").join(&hash[..2]).join(hash)
    }

    fn ac_path(&self, hash: &str) -> PathBuf {
        self.root.join("ac").join(hash)
    }

    fn tmp_path(&self) -> PathBuf {
        let n = self.tmp_seq.fetch_add(1, Ordering::Relaxed);
        self.root.join("tmp").join(format!("{}-{}", std::process::id(), n))
    }

    async fn has(&self, d: &reapi::Digest) -> bool {
        if d.size_bytes == 0 {
            return true;
        }
        match tokio::fs::metadata(self.cas_path(&d.hash)).await {
            Ok(m) => m.len() as i64 == d.size_bytes,
            Err(_) => false,
        }
    }

    async fn get(&self, d: &reapi::Digest) -> Result<Vec<u8>, Status> {
        check_digest(d)?;
        if d.size_bytes == 0 {
            return Ok(Vec::new());
        }
        tokio::fs::read(self.cas_path(&d.hash)).await.map_err(|_| {
            eprintln!("miss: blob {}/{}", d.hash, d.size_bytes);
            Status::not_found(format!("blob {}/{} not found", d.hash, d.size_bytes))
        })
    }

    /// Moves a fully written temporary file into the CAS.
    async fn commit(&self, tmp: PathBuf, hash: &str) -> Result<(), Status> {
        let dst = self.cas_path(hash);
        tokio::fs::create_dir_all(dst.parent().unwrap()).await.map_err(internal)?;
        tokio::fs::rename(&tmp, &dst).await.map_err(internal)
    }

    async fn put(&self, d: &reapi::Digest, data: &[u8]) -> Result<(), Status> {
        check_digest(d)?;
        if data.len() as i64 != d.size_bytes || hex(&Sha256::digest(data)) != d.hash {
            return Err(Status::invalid_argument(format!("data does not match digest {}/{}", d.hash, d.size_bytes)));
        }
        if self.has(d).await {
            return Ok(());
        }
        let tmp = self.tmp_path();
        tokio::fs::write(&tmp, data).await.map_err(internal)?;
        self.commit(tmp, &d.hash).await
    }
}

fn hex(b: &[u8]) -> String {
    b.iter().map(|x| format!("{x:02x}")).collect()
}

fn internal(e: std::io::Error) -> Status {
    Status::internal(e.to_string())
}

fn check_digest(d: &reapi::Digest) -> Result<(), Status> {
    if d.hash.len() != 64 || !d.hash.bytes().all(|b| b.is_ascii_hexdigit() && !b.is_ascii_uppercase()) || d.size_bytes < 0 {
        return Err(Status::invalid_argument(format!("invalid SHA-256 digest {}/{}", d.hash, d.size_bytes)));
    }
    Ok(())
}

fn digest_of(d: Option<reapi::Digest>) -> Result<reapi::Digest, Status> {
    let d = d.ok_or_else(|| Status::invalid_argument("missing digest"))?;
    check_digest(&d)?;
    Ok(d)
}

/// Parses ".../blobs/<hash>/<size>[/...]" from a ByteStream resource name.
fn resource_digest(name: &str) -> Result<reapi::Digest, Status> {
    let parts: Vec<&str> = name.split('/').collect();
    let i = parts
        .iter()
        .position(|p| *p == "blobs")
        .ok_or_else(|| Status::invalid_argument(format!("unsupported resource name {name}")))?;
    if i > 0 && parts[i - 1] == "compressed-blobs" {
        return Err(Status::invalid_argument("compressed blobs are not supported"));
    }
    let hash = parts.get(i + 1).copied().unwrap_or_default().to_string();
    let size = parts.get(i + 2).and_then(|s| s.parse().ok()).unwrap_or(-1);
    let d = reapi::Digest { hash, size_bytes: size };
    check_digest(&d)?;
    Ok(d)
}

fn ok_status() -> Option<reapi::Status> {
    Some(reapi::Status { code: 0, message: String::new(), details: vec![] })
}

fn err_status(s: Status) -> Option<reapi::Status> {
    Some(reapi::Status { code: s.code() as i32, message: s.message().to_string(), details: vec![] })
}

#[tonic::async_trait]
impl ActionCache for Store {
    async fn get_action_result(&self, req: Request<reapi::GetActionResultRequest>) -> Result<Response<reapi::ActionResult>, Status> {
        let d = digest_of(req.into_inner().action_digest)?;
        let b = tokio::fs::read(self.ac_path(&d.hash)).await.map_err(|_| {
            eprintln!("ac miss: {}", d.hash);
            Status::not_found(format!("action {} not found", d.hash))
        })?;
        eprintln!("ac hit: {}", d.hash);
        let ar = reapi::ActionResult::decode(b.as_slice()).map_err(|e| Status::internal(e.to_string()))?;
        Ok(Response::new(ar))
    }

    async fn update_action_result(&self, req: Request<reapi::UpdateActionResultRequest>) -> Result<Response<reapi::ActionResult>, Status> {
        let req = req.into_inner();
        let d = digest_of(req.action_digest)?;
        let ar = req.action_result.unwrap_or_default();
        eprintln!("ac update: {}", d.hash);
        let tmp = self.tmp_path();
        tokio::fs::write(&tmp, ar.encode_to_vec()).await.map_err(internal)?;
        tokio::fs::rename(&tmp, self.ac_path(&d.hash)).await.map_err(internal)?;
        Ok(Response::new(ar))
    }
}

#[tonic::async_trait]
impl ContentAddressableStorage for Store {
    async fn find_missing_blobs(&self, req: Request<reapi::FindMissingBlobsRequest>) -> Result<Response<reapi::FindMissingBlobsResponse>, Status> {
        let mut missing = vec![];
        for d in req.into_inner().blob_digests {
            check_digest(&d)?;
            if !self.has(&d).await {
                missing.push(d);
            }
        }
        Ok(Response::new(reapi::FindMissingBlobsResponse { missing_blob_digests: missing }))
    }

    async fn batch_update_blobs(&self, req: Request<reapi::BatchUpdateBlobsRequest>) -> Result<Response<reapi::BatchUpdateBlobsResponse>, Status> {
        let mut responses = vec![];
        for r in req.into_inner().requests {
            let d = r.digest.unwrap_or_default();
            let status = if r.compressor != 0 {
                err_status(Status::invalid_argument("compression is not supported"))
            } else {
                match self.put(&d, &r.data).await {
                    Ok(()) => ok_status(),
                    Err(e) => {
                        eprintln!("batch update failed: {}", e.message());
                        err_status(e)
                    }
                }
            };
            responses.push(reapi::batch_update_blobs_response::Response { digest: Some(d), status });
        }
        Ok(Response::new(reapi::BatchUpdateBlobsResponse { responses }))
    }

    async fn batch_read_blobs(&self, req: Request<reapi::BatchReadBlobsRequest>) -> Result<Response<reapi::BatchReadBlobsResponse>, Status> {
        let mut responses = vec![];
        for d in req.into_inner().digests {
            let (data, status) = match self.get(&d).await {
                Ok(b) => (b, ok_status()),
                Err(e) => (vec![], err_status(e)),
            };
            responses.push(reapi::batch_read_blobs_response::Response { digest: Some(d), data, compressor: 0, status });
        }
        Ok(Response::new(reapi::BatchReadBlobsResponse { responses }))
    }

    type GetTreeStream = Pin<Box<dyn Stream<Item = Result<reapi::GetTreeResponse, Status>> + Send>>;

    async fn get_tree(&self, _: Request<reapi::GetTreeRequest>) -> Result<Response<Self::GetTreeStream>, Status> {
        Err(Status::unimplemented("GetTree is not supported"))
    }
}

#[tonic::async_trait]
impl Capabilities for Store {
    async fn get_capabilities(&self, _: Request<reapi::GetCapabilitiesRequest>) -> Result<Response<reapi::ServerCapabilities>, Status> {
        let v = |minor| Some(reapi::SemVer { major: 2, minor, patch: 0, prerelease: String::new() });
        Ok(Response::new(reapi::ServerCapabilities {
            cache_capabilities: Some(reapi::CacheCapabilities {
                digest_functions: vec![SHA256],
                action_cache_update_capabilities: Some(reapi::ActionCacheUpdateCapabilities { update_enabled: true }),
                max_batch_total_size_bytes: MAX_BATCH_BYTES,
                symlink_absolute_path_strategy: SYMLINKS_ALLOWED,
            }),
            low_api_version: v(0),
            high_api_version: v(3),
        }))
    }
}

#[tonic::async_trait]
impl ByteStream for Store {
    type ReadStream = Pin<Box<dyn Stream<Item = Result<bytestream::ReadResponse, Status>> + Send>>;

    async fn read(&self, req: Request<bytestream::ReadRequest>) -> Result<Response<Self::ReadStream>, Status> {
        let req = req.into_inner();
        let d = resource_digest(&req.resource_name)?;
        if req.read_offset < 0 || req.read_offset > d.size_bytes || req.read_limit < 0 {
            return Err(Status::out_of_range("invalid read offset or limit"));
        }
        let mut remaining = d.size_bytes - req.read_offset;
        if req.read_limit > 0 {
            remaining = remaining.min(req.read_limit);
        }
        if d.size_bytes == 0 || remaining == 0 {
            return Ok(Response::new(Box::pin(tokio_stream::iter(vec![Ok(bytestream::ReadResponse { data: vec![] })]))));
        }
        let mut f = tokio::fs::File::open(self.cas_path(&d.hash)).await.map_err(|_| {
            eprintln!("miss: read {}/{}", d.hash, d.size_bytes);
            Status::not_found(format!("blob {}/{} not found", d.hash, d.size_bytes))
        })?;
        f.seek(std::io::SeekFrom::Start(req.read_offset as u64)).await.map_err(internal)?;
        let (tx, rx) = tokio::sync::mpsc::channel(4);
        tokio::spawn(async move {
            while remaining > 0 {
                let mut buf = vec![0u8; READ_CHUNK.min(remaining as usize)];
                let res = f.read_exact(&mut buf).await.map(|_| bytestream::ReadResponse { data: buf }).map_err(internal);
                let failed = res.is_err();
                if tx.send(res).await.is_err() || failed {
                    return;
                }
                remaining -= READ_CHUNK.min(remaining as usize) as i64;
            }
        });
        Ok(Response::new(Box::pin(tokio_stream::wrappers::ReceiverStream::new(rx))))
    }

    async fn write(&self, req: Request<Streaming<bytestream::WriteRequest>>) -> Result<Response<bytestream::WriteResponse>, Status> {
        let mut stream = req.into_inner();
        let mut digest: Option<reapi::Digest> = None;
        let tmp = self.tmp_path();
        let mut file = tokio::fs::File::create(&tmp).await.map_err(internal)?;
        let mut hasher = Sha256::new();
        let mut committed: i64 = 0;
        let result: Result<reapi::Digest, Status> = async {
            while let Some(msg) = stream.next().await {
                let msg = msg?;
                if digest.is_none() {
                    digest = Some(resource_digest(&msg.resource_name)?);
                }
                if msg.write_offset != committed {
                    return Err(Status::invalid_argument(format!("write_offset {} != committed {}", msg.write_offset, committed)));
                }
                hasher.update(&msg.data);
                file.write_all(&msg.data).await.map_err(internal)?;
                committed += msg.data.len() as i64;
                if msg.finish_write {
                    break;
                }
            }
            let d = digest.clone().ok_or_else(|| Status::invalid_argument("empty write"))?;
            file.flush().await.map_err(internal)?;
            if committed != d.size_bytes || hex(&hasher.clone().finalize()) != d.hash {
                return Err(Status::invalid_argument(format!("data does not match digest {}/{}", d.hash, d.size_bytes)));
            }
            Ok(d)
        }
        .await;
        drop(file);
        match result {
            Ok(d) => {
                if self.has(&d).await {
                    let _ = tokio::fs::remove_file(&tmp).await;
                } else {
                    self.commit(tmp, &d.hash).await?;
                }
                Ok(Response::new(bytestream::WriteResponse { committed_size: d.size_bytes }))
            }
            Err(e) => {
                eprintln!("write failed: {}", e.message());
                let _ = tokio::fs::remove_file(&tmp).await;
                Err(e)
            }
        }
    }

    async fn query_write_status(&self, req: Request<bytestream::QueryWriteStatusRequest>) -> Result<Response<bytestream::QueryWriteStatusResponse>, Status> {
        let d = resource_digest(&req.into_inner().resource_name)?;
        let complete = self.has(&d).await;
        Ok(Response::new(bytestream::QueryWriteStatusResponse { committed_size: if complete { d.size_bytes } else { 0 }, complete }))
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let mut dir = None;
    let mut listen = "127.0.0.1:0".to_string();
    let mut args = std::env::args().skip(1);
    while let Some(a) = args.next() {
        match a.as_str() {
            "--dir" => dir = args.next(),
            "--listen" => listen = args.next().ok_or("--listen needs a value")?,
            _ => return Err(format!("unknown argument {a}; usage: remotecache --dir DIR [--listen ADDR]").into()),
        }
    }
    let store = Store::new(PathBuf::from(dir.ok_or("--dir is required")?))?;
    let listener = tokio::net::TcpListener::bind(&listen).await?;
    println!("listening on {}", listener.local_addr()?);
    tonic::transport::Server::builder()
        .add_service(ActionCacheServer::new(store.clone()).max_decoding_message_size(MAX_MESSAGE_BYTES))
        .add_service(ContentAddressableStorageServer::new(store.clone()).max_decoding_message_size(MAX_MESSAGE_BYTES).max_encoding_message_size(MAX_MESSAGE_BYTES))
        .add_service(CapabilitiesServer::new(store.clone()))
        .add_service(ByteStreamServer::new(store).max_decoding_message_size(MAX_MESSAGE_BYTES))
        .serve_with_incoming(tokio_stream::wrappers::TcpListenerStream::new(listener))
        .await?;
    Ok(())
}
