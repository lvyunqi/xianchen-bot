use std::fs;
use std::io::{BufRead, BufReader, Write};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::{Arc, Mutex, mpsc};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use serde::{Deserialize, Serialize};

const EMBEDDED_WORKER: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/xianchen-worker"));
const MAX_WORKER_RESPONSE_BYTES: usize = 16 * 1024 * 1024;

#[derive(Clone, Debug, Serialize)]
pub struct InboundMessage {
    #[serde(rename = "type")]
    pub kind: &'static str,
    pub group_id: String,
    pub sender_id: String,
    pub sender_name: String,
    pub text: String,
    pub is_private: bool,
    pub account_id: String,
    pub message_id: String,
}

#[derive(Clone, Debug, Default, Deserialize, PartialEq)]
pub struct GamePayload {
    pub title: String,
    pub content: String,
    pub markdown: String,
    #[serde(default)]
    pub markdown_content: String,
    pub text_fallback: String,
    #[serde(default)]
    pub image_url: String,
    #[serde(default)]
    pub image_base64: String,
    pub image_only: bool,
    #[serde(default)]
    pub actions: Vec<String>,
    #[serde(default)]
    pub broadcast: String,
    #[serde(default)]
    pub broadcast_targets: Vec<String>,
}

#[derive(Clone, Debug, Default, Deserialize)]
pub struct WorkerReply {
    #[serde(rename = "type")]
    pub kind: String,
    #[serde(default)]
    pub handled: bool,
    pub result: Option<GamePayload>,
    #[serde(default)]
    pub error: String,
    pub protocol_version: Option<u32>,
    #[serde(default)]
    pub version: String,
    #[serde(default)]
    pub admin_url: String,
}

struct Request {
    line: String,
    response: mpsc::SyncSender<Result<WorkerReply, String>>,
}

pub struct WorkerRuntime {
    sender: Option<mpsc::Sender<Request>>,
    child: Arc<Mutex<Child>>,
    io_thread: Option<JoinHandle<()>>,
    io_timeout: Duration,
    running: bool,
    pub version: String,
    pub admin_url: String,
}

impl WorkerRuntime {
    pub fn start(
        data_root: &Path,
        data_subdir: &str,
        spawn_timeout: Duration,
        io_timeout: Duration,
    ) -> Result<Self, String> {
        let runtime_dir = data_root.join(data_subdir);
        fs::create_dir_all(&runtime_dir)
            .map_err(|error| format!("创建 worker 数据目录失败：{error}"))?;
        let executable = extract_worker(&runtime_dir)?;
        let mut child = Command::new(&executable)
            .arg("--data-dir")
            .arg(&runtime_dir)
            .arg("--host-pid")
            .arg(std::process::id().to_string())
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::inherit())
            .spawn()
            .map_err(|error| format!("启动 Go worker {} 失败：{error}", executable.display()))?;
        let mut stdin = match child.stdin.take() {
            Some(stdin) => stdin,
            None => {
                let _ = child.kill();
                let _ = child.wait();
                return Err("Go worker stdin 未建立".to_string());
            }
        };
        let stdout = match child.stdout.take() {
            Some(stdout) => stdout,
            None => {
                let _ = child.kill();
                let _ = child.wait();
                return Err("Go worker stdout 未建立".to_string());
            }
        };
        let child = Arc::new(Mutex::new(child));
        let (sender, receiver) = mpsc::channel::<Request>();
        let io_thread = thread::Builder::new()
            .name("xianchen-worker-io".to_string())
            .spawn(move || {
                let mut reader = BufReader::new(stdout);
                for request in receiver {
                    let result = exchange_line(&mut stdin, &mut reader, &request.line);
                    let failed = result.is_err();
                    let _ = request.response.send(result);
                    if failed {
                        break;
                    }
                }
            })
            .map_err(|error| format!("启动 worker IO 线程失败：{error}"))?;
        let mut runtime = Self {
            sender: Some(sender),
            child,
            io_thread: Some(io_thread),
            io_timeout,
            running: true,
            version: String::new(),
            admin_url: String::new(),
        };
        let ping = match runtime.request_line(r#"{"type":"ping"}"#, spawn_timeout) {
            Ok(ping) => ping,
            Err(error) => {
                runtime.stop();
                return Err(format!("Go worker 握手失败：{error}"));
            }
        };
        if ping.kind != "pong" || ping.protocol_version != Some(crate::PROTOCOL_VERSION) {
            runtime.stop();
            return Err(format!(
                "Go worker 协议不兼容：type={} protocol={:?}",
                ping.kind, ping.protocol_version
            ));
        }
        runtime.version = ping.version;
        runtime.admin_url = ping.admin_url;
        Ok(runtime)
    }

    pub fn request(&mut self, message: &InboundMessage) -> Result<WorkerReply, String> {
        if !self.running {
            return Err("Go worker 已停止".to_string());
        }
        let exited = {
            let mut child = self
                .child
                .lock()
                .map_err(|_| "worker 进程锁已损坏".to_string())?;
            child
                .try_wait()
                .map_err(|error| format!("检查 worker 状态失败：{error}"))?
        };
        if let Some(status) = exited {
            self.running = false;
            self.sender.take();
            if let Some(thread) = self.io_thread.take() {
                let _ = thread.join();
            }
            return Err(format!("Go worker 已退出：{status}"));
        }
        let line = serde_json::to_string(message)
            .map_err(|error| format!("编码 worker 请求失败：{error}"))?;
        let result = self.request_line(&line, self.io_timeout);
        if result.is_err() {
            // 超时或协议损坏后通道可能已失步，禁止复用并尽快回收子进程。
            self.stop();
        }
        result
    }

    pub fn stop(&mut self) {
        if !self.running {
            return;
        }
        self.running = false;
        if self.sender.is_some() {
            let _ = self.request_line(r#"{"type":"shutdown"}"#, Duration::from_secs(2));
        }
        self.sender.take();
        if let Ok(mut child) = self.child.lock() {
            match child.try_wait() {
                Ok(Some(_)) => {}
                Ok(None) | Err(_) => {
                    let _ = child.kill();
                    let _ = child.wait();
                }
            }
        }
        if let Some(thread) = self.io_thread.take() {
            let _ = thread.join();
        }
    }

    pub fn is_running(&self) -> bool {
        self.running
    }

    fn request_line(&self, line: &str, timeout: Duration) -> Result<WorkerReply, String> {
        let sender = self
            .sender
            .as_ref()
            .ok_or_else(|| "Go worker 已停止".to_string())?;
        let (response_tx, response_rx) = mpsc::sync_channel(1);
        sender
            .send(Request {
                line: line.to_string(),
                response: response_tx,
            })
            .map_err(|_| "Go worker IO 线程已退出".to_string())?;
        response_rx
            .recv_timeout(timeout)
            .map_err(|error| match error {
                mpsc::RecvTimeoutError::Timeout => {
                    format!("Go worker 响应超时（{} 秒）", timeout.as_secs())
                }
                mpsc::RecvTimeoutError::Disconnected => "Go worker IO 线程已断开".to_string(),
            })?
    }
}

impl Drop for WorkerRuntime {
    fn drop(&mut self) {
        self.stop();
    }
}

fn exchange_line(
    stdin: &mut impl Write,
    reader: &mut impl BufRead,
    line: &str,
) -> Result<WorkerReply, String> {
    writeln!(stdin, "{line}").map_err(|error| format!("写入 worker 失败：{error}"))?;
    stdin
        .flush()
        .map_err(|error| format!("刷新 worker stdin 失败：{error}"))?;
    let mut response = String::new();
    let read = {
        let mut limited = std::io::Read::take(reader, (MAX_WORKER_RESPONSE_BYTES + 1) as u64);
        limited
            .read_line(&mut response)
            .map_err(|error| format!("读取 worker 失败：{error}"))?
    };
    if read > MAX_WORKER_RESPONSE_BYTES {
        return Err("worker 返回行超过16MiB上限".to_string());
    }
    if read == 0 {
        return Err("Go worker stdout 已关闭".to_string());
    }
    serde_json::from_str(response.trim_end())
        .map_err(|error| format!("worker 返回无效 JSON：{error}"))
}

fn extract_worker(runtime_dir: &Path) -> Result<PathBuf, String> {
    if EMBEDDED_WORKER.is_empty() {
        return Err("动态插件未嵌入 Go worker".to_string());
    }
    let bin_dir = runtime_dir.join("bin");
    fs::create_dir_all(&bin_dir).map_err(|error| format!("创建 worker bin 目录失败：{error}"))?;
    let executable = bin_dir.join(format!(
        "xianchen-worker-{}{}",
        env!("CARGO_PKG_VERSION"),
        if cfg!(windows) { ".exe" } else { "" }
    ));
    let current = fs::read(&executable).ok();
    if current.as_deref() != Some(EMBEDDED_WORKER) {
        let temporary = executable.with_extension("tmp");
        fs::write(&temporary, EMBEDDED_WORKER)
            .map_err(|error| format!("写入 worker 临时文件失败：{error}"))?;
        set_executable(&temporary)?;
        if executable.exists() {
            fs::remove_file(&executable).map_err(|error| format!("替换旧 worker 失败：{error}"))?;
        }
        fs::rename(&temporary, &executable)
            .map_err(|error| format!("安装 worker 失败：{error}"))?;
    } else {
        set_executable(&executable)?;
    }
    Ok(executable)
}

#[cfg(unix)]
fn set_executable(path: &Path) -> Result<(), String> {
    use std::os::unix::fs::PermissionsExt;
    let mut permissions = fs::metadata(path)
        .map_err(|error| format!("读取 worker 权限失败：{error}"))?
        .permissions();
    permissions.set_mode(0o755);
    fs::set_permissions(path, permissions)
        .map_err(|error| format!("设置 worker 可执行权限失败：{error}"))
}

#[cfg(not(unix))]
fn set_executable(_path: &Path) -> Result<(), String> {
    Ok(())
}
