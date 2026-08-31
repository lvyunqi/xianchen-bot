//! 仙尘 QimenBot 动态插件。
//!
//! Rust 负责 QimenBot 边界与 Go worker 生命周期；游戏语义保留在 Go 内核。

mod context;
mod message;
mod worker;

use std::path::Path;
use std::sync::{Mutex, OnceLock, RwLock};
use std::time::Duration;

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
    pub qq_official_markdown: bool,
}

impl Default for PluginConfig {
    fn default() -> Self {
        Self {
            worker_enabled: true,
            spawn_timeout_secs: 20,
            io_timeout_secs: 25,
            data_subdir: "xianchen".to_string(),
            qq_official_markdown: true,
        }
    }
}

#[derive(Debug, Default, Deserialize)]
#[serde(default, deny_unknown_fields)]
struct ConfigDocument {
    worker: WorkerConfigDocument,
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

static CONFIG: OnceLock<RwLock<PluginConfig>> = OnceLock::new();
static WORKER: OnceLock<Mutex<Option<worker::WorkerRuntime>>> = OnceLock::new();
static RUNTIME_TRANSITION: OnceLock<Mutex<()>> = OnceLock::new();

fn config_slot() -> &'static RwLock<PluginConfig> {
    CONFIG.get_or_init(|| RwLock::new(PluginConfig::default()))
}

fn worker_slot() -> &'static Mutex<Option<worker::WorkerRuntime>> {
    WORKER.get_or_init(|| Mutex::new(None))
}

fn runtime_transition_slot() -> &'static Mutex<()> {
    RUNTIME_TRANSITION.get_or_init(|| Mutex::new(()))
}

fn current_config() -> PluginConfig {
    config_slot()
        .read()
        .map(|slot| slot.clone())
        .unwrap_or_default()
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
    Ok(PluginConfig {
        worker_enabled: document.worker.enabled,
        spawn_timeout_secs: document.worker.spawn_timeout_secs,
        io_timeout_secs: document.worker.io_timeout_secs,
        data_subdir: data_subdir.to_string(),
        qq_official_markdown: document.messages.qq_official_markdown,
    })
}

fn initialize(config: PluginInitConfig) -> Result<(), String> {
    let _transition = runtime_transition_slot()
        .lock()
        .map_err(|_| "插件运行时切换锁已损坏".to_string())?;
    let parsed = parse_config(config.config_json.as_str())?;
    let data_dir = config.data_dir.as_str().trim();
    if parsed.worker_enabled && data_dir.is_empty() {
        return Err("QimenBot 未提供 data_dir，无法启动 Go worker".to_string());
    }
    let mut slot = worker_slot()
        .lock()
        .map_err(|_| "worker 运行时锁已损坏".to_string())?;
    if let Some(mut previous) = slot.take() {
        previous.stop();
    }
    let replacement = if parsed.worker_enabled {
        Some(worker::WorkerRuntime::start(
            Path::new(data_dir),
            &parsed.data_subdir,
            Duration::from_secs(parsed.spawn_timeout_secs),
            Duration::from_secs(parsed.io_timeout_secs),
        )?)
    } else {
        None
    };
    *slot = replacement;
    replace_config(parsed);
    Ok(())
}

fn shutdown_runtime() {
    let Ok(_transition) = runtime_transition_slot().lock() else {
        return;
    };
    if let Ok(mut slot) = worker_slot().lock()
        && let Some(mut runtime) = slot.take()
    {
        runtime.stop();
    }
}

pub fn diagnostic_text(config: &PluginConfig) -> String {
    let markdown_state = if config.qq_official_markdown { "开启" } else { "关闭" };
    let runtime = worker_slot().lock().ok();
    let worker_state = runtime
        .as_ref()
        .and_then(|slot| slot.as_ref())
        .filter(|worker| worker.is_running())
        .map(|worker| {
            format!(
                "运行中（worker={}，后台={}）",
                if worker.version.is_empty() { "未知" } else { &worker.version },
                if worker.admin_url.is_empty() { "未提供" } else { &worker.admin_url }
            )
        })
        .unwrap_or_else(|| {
            if config.worker_enabled { "已启用但未运行".to_string() } else { "已停用".to_string() }
        });
    format!(
        "仙尘插件 v{}（P1 桥接）\n协议版本：{}\nGo 内核桥接：{}\nMarkdown 渲染：{}\n数据目录：{}",
        PLUGIN_VERSION, PROTOCOL_VERSION, worker_state, markdown_state, config.data_subdir
    )
}

#[dynamic_plugin(
    id = "xianchen",
    version = "0.1.0",
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
        let config = current_config();
        if !config.worker_enabled {
            return InterceptorResponse::allow();
        }
        let Ok(inbound) = context::resolve_inbound(request) else {
            return InterceptorResponse::allow();
        };
        let Ok(mut slot) = worker_slot().lock() else {
            return InterceptorResponse::allow();
        };
        let Some(runtime) = slot.as_mut() else {
            return InterceptorResponse::allow();
        };
        let Ok(reply) = runtime.request(&inbound) else {
            return InterceptorResponse::allow();
        };
        if reply.kind != "reply" || !reply.handled {
            return InterceptorResponse::allow();
        }
        let Some(payload) = reply.result else {
            return InterceptorResponse::allow();
        };
        if !message::queue_response(request, &inbound, &payload, config.qq_official_markdown) {
            return InterceptorResponse::allow();
        }
        message::queue_broadcasts(&inbound, &payload, config.qq_official_markdown);
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
        assert_eq!(parse_config("").unwrap(), PluginConfig::default());
        assert_eq!(parse_config("   ").unwrap(), PluginConfig::default());
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
