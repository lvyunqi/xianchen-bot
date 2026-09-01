//! 仙尘 QimenBot 动态插件。
//!
//! Rust 负责 QimenBot 边界与 Go worker 生命周期；游戏语义保留在 Go 内核。

mod context;
mod message;
mod worker;

use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Mutex, OnceLock, RwLock};
use std::time::{Duration, Instant};

use abi_stable_host_api::{
    CommandRequest, CommandResponse, InterceptorRequest, InterceptorResponse, PluginConfigRequest,
    PluginConfigResult, PluginInitConfig, PluginInitResult,
};
use qimen_dynamic_plugin_derive::dynamic_plugin;
use serde::Deserialize;

const PLUGIN_VERSION: &str = env!("CARGO_PKG_VERSION");
const PROTOCOL_VERSION: u32 = 1;

/// 插件运行配置（config.schema.json 的 Rust 投影）。
#[derive(Clone, Debug, PartialEq)]
pub struct PluginConfig {
    pub worker_enabled: bool,
    pub spawn_timeout_secs: u64,
    pub io_timeout_secs: u64,
    pub data_subdir: String,
    pub admin_listen_host: String,
    pub qq_official_markdown: bool,
}

impl Default for PluginConfig {
    fn default() -> Self {
        Self {
            worker_enabled: true,
            spawn_timeout_secs: 20,
            io_timeout_secs: 25,
            data_subdir: "xianchen".to_string(),
            admin_listen_host: "127.0.0.1".to_string(),
            qq_official_markdown: true,
        }
    }
}

#[derive(Debug, Default, Deserialize)]
#[serde(default, deny_unknown_fields)]
struct ConfigDocument {
    worker: WorkerConfigDocument,
    admin: AdminConfigDocument,
    messages: MessageConfigDocument,
}

#[derive(Debug, Deserialize)]
#[serde(default, deny_unknown_fields)]
struct WorkerConfigDocument {
    enabled: bool,
    spawn_timeout_secs: u64,
    io_timeout_secs: u64,
    data_subdir: String,
}

impl Default for WorkerConfigDocument {
    fn default() -> Self {
        Self {
            enabled: true,
            spawn_timeout_secs: 20,
            io_timeout_secs: 25,
            data_subdir: "xianchen".to_string(),
        }
    }
}

#[derive(Debug, Deserialize)]
#[serde(default, deny_unknown_fields)]
struct AdminConfigDocument {
    listen_host: String,
}

impl Default for AdminConfigDocument {
    fn default() -> Self {
        Self {
            listen_host: "127.0.0.1".to_string(),
        }
    }
}

#[derive(Debug, Deserialize)]
#[serde(default, deny_unknown_fields)]
struct MessageConfigDocument {
    qq_official_markdown: bool,
}

impl Default for MessageConfigDocument {
    fn default() -> Self {
        Self {
            qq_official_markdown: true,
        }
    }
}

#[derive(Clone, Debug)]
struct WorkerLaunch {
    data_root: PathBuf,
    data_subdir: String,
    admin_host: String,
    spawn_timeout: Duration,
    io_timeout: Duration,
}

static CONFIG: OnceLock<RwLock<PluginConfig>> = OnceLock::new();
static WORKER: OnceLock<Mutex<Option<worker::WorkerRuntime>>> = OnceLock::new();
static WORKER_LAUNCH: OnceLock<RwLock<Option<WorkerLaunch>>> = OnceLock::new();
static RUNTIME_TRANSITION: OnceLock<Mutex<()>> = OnceLock::new();
static LIFECYCLE_BUSY: AtomicBool = AtomicBool::new(false);
static SHUTDOWN_REQUESTED: AtomicBool = AtomicBool::new(false);

fn config_slot() -> &'static RwLock<PluginConfig> {
    CONFIG.get_or_init(|| RwLock::new(PluginConfig::default()))
}

fn worker_slot() -> &'static Mutex<Option<worker::WorkerRuntime>> {
    WORKER.get_or_init(|| Mutex::new(None))
}

fn runtime_transition_slot() -> &'static Mutex<()> {
    RUNTIME_TRANSITION.get_or_init(|| Mutex::new(()))
}

fn worker_launch_slot() -> &'static RwLock<Option<WorkerLaunch>> {
    WORKER_LAUNCH.get_or_init(|| RwLock::new(None))
}

fn replace_worker_launch(launch: Option<WorkerLaunch>) {
    if let Ok(mut slot) = worker_launch_slot().write() {
        *slot = launch;
    }
}

fn current_worker_launch() -> Option<WorkerLaunch> {
    worker_launch_slot()
        .read()
        .ok()
        .and_then(|slot| slot.clone())
}

fn begin_lifecycle() -> Result<(), String> {
    LIFECYCLE_BUSY
        .compare_exchange(false, true, Ordering::AcqRel, Ordering::Acquire)
        .map(|_| ())
        .map_err(|_| "插件运行时正在切换".to_string())
}

fn end_lifecycle() {
    LIFECYCLE_BUSY.store(false, Ordering::Release);
}

fn current_config() -> PluginConfig {
    config_slot()
        .read()
        .map(|slot| slot.clone())
        .unwrap_or_default()
}

/// 最近一次 pre_handle 消息链路的一行摘要，供 `仙尘状态` 诊断静默问题。
static LAST_TRACE: OnceLock<Mutex<Option<(Instant, String)>>> = OnceLock::new();

fn last_trace_slot() -> &'static Mutex<Option<(Instant, String)>> {
    LAST_TRACE.get_or_init(|| Mutex::new(None))
}

fn note_trace(summary: String) {
    if let Ok(mut slot) = last_trace_slot().lock() {
        *slot = Some((Instant::now(), summary));
    }
}

fn last_trace_text() -> String {
    last_trace_slot()
        .lock()
        .ok()
        .and_then(|slot| slot.clone())
        .map(|(at, summary)| {
            format!(
                "{}（{} 秒前）",
                summary,
                at.elapsed().as_secs()
            )
        })
        .unwrap_or_else(|| "自插件加载以来从未收到消息".to_string())
}

fn replace_config(config: PluginConfig) {
    if let Ok(mut slot) = config_slot().write() {
        *slot = config;
    }
}

/// 解析宿主传入的 JSON 配置；空配置使用默认值，字段非法返回错误。
pub fn parse_config(config_json: &str) -> Result<PluginConfig, String> {
    let trimmed = config_json.trim();
    if trimmed.is_empty() {
        return Ok(PluginConfig::default());
    }
    let document: ConfigDocument =
        serde_json::from_str(trimmed).map_err(|error| format!("插件配置无效: {error}"))?;
    if !(5..=120).contains(&document.worker.spawn_timeout_secs) {
        return Err("worker.spawn_timeout_secs 必须在 5 到 120 秒之间".to_string());
    }
    if !(5..=29).contains(&document.worker.io_timeout_secs) {
        return Err("worker.io_timeout_secs 必须在 5 到 29 秒之间".to_string());
    }
    let data_subdir = document.worker.data_subdir.trim();
    if data_subdir.is_empty() || data_subdir.chars().count() > 64 {
        return Err("worker.data_subdir 长度必须在 1 到 64 个字符之间".to_string());
    }
    if data_subdir == "."
        || data_subdir == ".."
        || data_subdir.contains('/')
        || data_subdir.contains('\\')
        || data_subdir.chars().any(char::is_control)
    {
        return Err("worker.data_subdir 必须是安全的单级目录名".to_string());
    }
    let admin_listen_host = document.admin.listen_host.trim();
    if admin_listen_host.is_empty() {
        return Ok(PluginConfig {
            worker_enabled: document.worker.enabled,
            spawn_timeout_secs: document.worker.spawn_timeout_secs,
            io_timeout_secs: document.worker.io_timeout_secs,
            data_subdir: data_subdir.to_string(),
            admin_listen_host: "127.0.0.1".to_string(),
            qq_official_markdown: document.messages.qq_official_markdown,
        });
    }
    if admin_listen_host.chars().count() > 64
        || admin_listen_host.contains('/')
        || admin_listen_host.contains('\\')
        || admin_listen_host.contains(char::is_whitespace)
        || admin_listen_host.chars().any(char::is_control)
    {
        return Err("admin.listen_host 必须是不含空白与路径分隔符的主机名或 IP".to_string());
    }
    Ok(PluginConfig {
        worker_enabled: document.worker.enabled,
        spawn_timeout_secs: document.worker.spawn_timeout_secs,
        io_timeout_secs: document.worker.io_timeout_secs,
        data_subdir: data_subdir.to_string(),
        admin_listen_host: admin_listen_host.to_string(),
        qq_official_markdown: document.messages.qq_official_markdown,
    })
}

fn initialize(config: PluginInitConfig) -> Result<(), String> {
    begin_lifecycle()?;
    let result = initialize_inner(config);
    end_lifecycle();
    result
}

fn initialize_inner(config: PluginInitConfig) -> Result<(), String> {
    let parsed = parse_config(config.config_json.as_str())?;
    let data_dir = config.data_dir.as_str().trim();
    if parsed.worker_enabled && data_dir.is_empty() {
        return Err("QimenBot 未提供 data_dir，无法启动 Go worker".to_string());
    }
    let launch = parsed.worker_enabled.then(|| WorkerLaunch {
        data_root: PathBuf::from(data_dir),
        data_subdir: parsed.data_subdir.clone(),
        admin_host: parsed.admin_listen_host.clone(),
        spawn_timeout: Duration::from_secs(parsed.spawn_timeout_secs),
        io_timeout: Duration::from_secs(parsed.io_timeout_secs),
    });
    let previous = {
        let _transition = runtime_transition_slot()
            .lock()
            .map_err(|_| "插件运行时切换锁已损坏".to_string())?;
        SHUTDOWN_REQUESTED.store(false, Ordering::Release);
        replace_worker_launch(None);
        worker_slot()
            .lock()
            .map_err(|_| "worker 运行时锁已损坏".to_string())?
            .take()
    };
    if let Some(mut previous) = previous {
        previous.stop();
    }

    // 启动和握手可能触发磁盘初始化，不能持有 worker_slot 或切换锁。
    let replacement = match launch.as_ref() {
        Some(launch) => Some(worker::WorkerRuntime::start(
            &launch.data_root,
            &launch.data_subdir,
            &launch.admin_host,
            launch.spawn_timeout,
            launch.io_timeout,
        )?),
        None => None,
    };
    if SHUTDOWN_REQUESTED.load(Ordering::Acquire) {
        if let Some(mut replacement) = replacement {
            replacement.stop();
        }
        return Err("插件在 worker 启动期间收到关闭请求".to_string());
    }
    {
        let _transition = runtime_transition_slot()
            .lock()
            .map_err(|_| "插件运行时切换锁已损坏".to_string())?;
        if SHUTDOWN_REQUESTED.load(Ordering::Acquire) {
            if let Some(mut replacement) = replacement {
                replacement.stop();
            }
            return Err("插件在 worker 安装期间收到关闭请求".to_string());
        }
        let mut slot = worker_slot()
            .lock()
            .map_err(|_| "worker 运行时锁已损坏".to_string())?;
        *slot = replacement;
        replace_config(parsed);
        replace_worker_launch(launch);
    }
    Ok(())
}

fn recover_worker() -> Result<(), String> {
    if SHUTDOWN_REQUESTED.load(Ordering::Acquire) {
        return Err("插件正在关闭".to_string());
    }
    begin_lifecycle()?;
    let result = recover_worker_inner();
    end_lifecycle();
    result
}

fn recover_worker_inner() -> Result<(), String> {
    let launch = current_worker_launch().ok_or_else(|| "没有可用的 worker 启动配置".to_string())?;
    let previous = {
        let _transition = runtime_transition_slot()
            .lock()
            .map_err(|_| "插件运行时切换锁已损坏".to_string())?;
        worker_slot()
            .lock()
            .map_err(|_| "worker 运行时锁已损坏".to_string())?
            .take()
    };
    if let Some(mut previous) = previous {
        previous.stop();
    }
    let replacement = worker::WorkerRuntime::start(
        &launch.data_root,
        &launch.data_subdir,
        &launch.admin_host,
        launch.spawn_timeout.min(Duration::from_secs(20)),
        launch.io_timeout,
    )?;
    if SHUTDOWN_REQUESTED.load(Ordering::Acquire) {
        let mut replacement = replacement;
        replacement.stop();
        return Err("插件在 worker 恢复期间收到关闭请求".to_string());
    }
    let _transition = runtime_transition_slot()
        .lock()
        .map_err(|_| "插件运行时切换锁已损坏".to_string())?;
    if SHUTDOWN_REQUESTED.load(Ordering::Acquire) {
        let mut replacement = replacement;
        replacement.stop();
        return Err("插件在 worker 安装期间收到关闭请求".to_string());
    }
    let mut slot = worker_slot()
        .lock()
        .map_err(|_| "worker 运行时锁已损坏".to_string())?;
    *slot = Some(replacement);
    Ok(())
}

fn shutdown_runtime() {
    let previous = {
        let Ok(_transition) = runtime_transition_slot().lock() else {
            SHUTDOWN_REQUESTED.store(true, Ordering::Release);
            return;
        };
        SHUTDOWN_REQUESTED.store(true, Ordering::Release);
        replace_worker_launch(None);
        worker_slot().lock().ok().and_then(|mut slot| slot.take())
    };
    if let Some(mut previous) = previous {
        previous.stop();
    }
}

pub fn diagnostic_text(config: &PluginConfig) -> String {
    let markdown_state = if config.qq_official_markdown {
        "开启"
    } else {
        "关闭"
    };
    let mut slot = worker_slot().lock().ok();
    let worker_state = slot
        .as_mut()
        .and_then(|slot| slot.as_mut())
        .filter(|worker| worker.is_running())
        .map(|worker| {
            let probe = worker.health();
            let version = if worker.version.is_empty() {
                "未知"
            } else {
                worker.version.as_str()
            };
            let admin = if worker.admin_url.is_empty() {
                "未提供"
            } else {
                worker.admin_url.as_str()
            };
            let kernel = match &probe {
                Ok(pong) if pong.kernel_ready => "就绪".to_string(),
                Ok(pong) if !pong.init_error.is_empty() => {
                    format!("初始化失败：{}", pong.init_error)
                }
                Ok(_) => "初始化中（首次建库需要一些时间，稍后再试游戏指令）".to_string(),
                Err(error) => format!("探测失败：{}", error),
            };
            format!(
                "运行中（worker={}，后台={}）\n内核状态：{}",
                version, admin, kernel
            )
        })
        .unwrap_or_else(|| {
            if config.worker_enabled {
                "已启用但未运行".to_string()
            } else {
                "已停用".to_string()
            }
        });
    format!(
        "仙尘插件 v{}（P1 桥接）\n协议版本：{}\nGo 内核桥接：{}\n最近消息：{}\nMarkdown 渲染：{}\n数据目录：{}",
        PLUGIN_VERSION,
        PROTOCOL_VERSION,
        worker_state,
        last_trace_text(),
        markdown_state,
        config.data_subdir
    )
}

#[dynamic_plugin(
    id = "xianchen",
    version = "0.1.0-rc.5",
    api = "0.6",
    config_schema = "../config.schema.json",
    config_ui = "../config.ui.json",
    config_version = 1,
    config_apply = "reload"
)]
mod plugin {
    use super::*;

    #[init]
    fn init(config: PluginInitConfig) -> PluginInitResult {
        match initialize(config) {
            Ok(()) => PluginInitResult::ok(),
            Err(error) => PluginInitResult::err(&error),
        }
    }

    #[validate_config]
    fn validate(request: &PluginConfigRequest) -> PluginConfigResult {
        match parse_config(request.config_json.as_str()) {
            Ok(_) => PluginConfigResult::ok(),
            Err(error) => PluginConfigResult::err(&error),
        }
    }

    #[pre_handle]
    fn intercept(request: &InterceptorRequest) -> InterceptorResponse {
        note_trace(format!(
            "收到消息 sender={} text={:.24}",
            request.sender_id.as_str(),
            request.message_text.as_str()
        ));
        let config = current_config();
        if !config.worker_enabled
            || LIFECYCLE_BUSY.load(Ordering::Acquire)
            || SHUTDOWN_REQUESTED.load(Ordering::Acquire)
        {
            note_trace("消息被忽略：worker 未启用或插件正在切换".to_string());
            return InterceptorResponse::allow();
        }
        let inbound = match context::resolve_inbound(request) {
            Ok(inbound) => inbound,
            Err(error) => {
                note_trace(format!("消息解析失败：{error}"));
                eprintln!("仙尘消息解析失败：{error}");
                return InterceptorResponse::allow();
            }
        };
        let result = {
            let Ok(mut slot) = worker_slot().lock() else {
                note_trace("worker 运行时锁已损坏，消息放行".to_string());
                return InterceptorResponse::allow();
            };
            if !matches!(slot.as_ref(), Some(runtime) if runtime.is_running()) {
                drop(slot);
                let _ = recover_worker();
                note_trace("worker 未运行，已尝试恢复；本条消息放行".to_string());
                return InterceptorResponse::allow();
            }
            slot.as_mut().expect("worker 状态已检查").request(&inbound)
        };
        let Ok(reply) = result else {
            // 超时消息立即放行；下一条消息再恢复，避免超过宿主回调预算。
            note_trace(format!(
                "sender={} worker 请求失败（超时或管道断开），已放行",
                inbound.sender_id
            ));
            return InterceptorResponse::allow();
        };
        note_trace(format!(
            "sender={} worker 回复：kind={} handled={} error={}",
            inbound.sender_id, reply.kind, reply.handled, reply.error
        ));
        if reply.kind == "error" && !reply.error.is_empty() {
            eprintln!("仙尘内核错误：{}", reply.error);
        }
        if reply.kind != "reply" || !reply.handled {
            if reply.kind == "reply" && !reply.error.is_empty() {
                // 内核未就绪/初始化失败等降级提示写入宿主日志，避免完全静默。
                eprintln!("仙尘内核提示：{}", reply.error);
            }
            return InterceptorResponse::allow();
        }
        let Some(payload) = reply.result else {
            return InterceptorResponse::allow();
        };
        let response_queued =
            message::queue_response(request, &inbound, &payload, config.qq_official_markdown);
        message::queue_broadcasts(&inbound, &payload, config.qq_official_markdown);
        if !response_queued {
            return InterceptorResponse::allow();
        }
        InterceptorResponse::block()
    }

    #[command(
        name = "仙尘状态",
        description = "查看仙尘插件桥接与配置状态",
        aliases = "xianchen-status",
        category = "仙尘·系统",
        scope = "all"
    )]
    fn status(_req: &CommandRequest) -> CommandResponse {
        CommandResponse::text(&diagnostic_text(&current_config()))
    }

    #[shutdown]
    fn shutdown() {
        shutdown_runtime();
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn empty_config_uses_defaults() {
        let config = parse_config("").unwrap();
        assert_eq!(config, PluginConfig::default());
        assert_eq!(parse_config("   ").unwrap(), PluginConfig::default());
        assert_eq!(config.admin_listen_host, "127.0.0.1");
    }

    #[test]
    fn admin_listen_host_is_parsed() {
        let config = parse_config(r#"{"admin":{"listen_host":"0.0.0.0"}}"#).unwrap();
        assert_eq!(config.admin_listen_host, "0.0.0.0");
    }

    #[test]
    fn unsafe_admin_listen_hosts_are_rejected() {
        for host in ["0.0.0.0 -x", "a/b", "a\\b"] {
            let json = serde_json::json!({ "admin": { "listen_host": host } }).to_string();
            assert!(parse_config(&json).is_err(), "host 应被拒绝: {host}");
        }
    }

    #[test]
    fn invalid_json_is_rejected() {
        assert!(parse_config("{not json").is_err());
    }

    #[test]
    fn partial_config_keeps_defaults() {
        let config = parse_config(
            r#"{"worker":{"enabled":true},"messages":{"qq_official_markdown":false}}"#,
        )
        .unwrap();
        assert!(config.worker_enabled);
        assert!(!config.qq_official_markdown);
        assert_eq!(config.data_subdir, "xianchen");
        assert_eq!(config.io_timeout_secs, 25);
    }

    #[test]
    fn out_of_range_timeouts_are_rejected() {
        assert!(parse_config(r#"{"worker":{"spawn_timeout_secs":1}}"#).is_err());
        assert!(parse_config(r#"{"worker":{"io_timeout_secs":999}}"#).is_err());
    }

    #[test]
    fn invalid_types_and_unknown_fields_are_rejected() {
        assert!(parse_config(r#"{"worker":{"enabled":"yes"}}"#).is_err());
        assert!(parse_config(r#"{"unknown":true}"#).is_err());
    }

    #[test]
    fn unsafe_data_subdirs_are_rejected() {
        for subdir in ["", ".", "..", "../outside", r#"..\outside"#] {
            let json = serde_json::json!({ "worker": { "data_subdir": subdir } }).to_string();
            assert!(
                parse_config(&json).is_err(),
                "accepted unsafe subdir {subdir:?}"
            );
        }
    }

    #[test]
    fn diagnostic_text_reflects_p1_state() {
        let text = diagnostic_text(&PluginConfig::default());
        assert!(text.contains("P1"));
        assert!(text.contains("已启用但未运行"));
        assert!(text.contains(PLUGIN_VERSION));
    }
}
