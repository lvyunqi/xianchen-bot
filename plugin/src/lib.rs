//! 仙尘 QimenBot 动态插件（P0 骨架）。
//!
//! 计划：P0 只提供插件身份、在线配置与诊断命令；
//! P1 起引入 Go worker 子进程桥接（见 docs/qimenbot-migration-plan.md）。

use std::sync::{OnceLock, RwLock};

use abi_stable_host_api::{
    CommandRequest, CommandResponse, PluginConfigRequest, PluginConfigResult, PluginInitConfig,
    PluginInitResult,
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
            worker_enabled: false,
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
            enabled: false,
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

fn config_slot() -> &'static RwLock<PluginConfig> {
    CONFIG.get_or_init(|| RwLock::new(PluginConfig::default()))
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

/// 诊断命令文本：P0 阶段反映插件自身状态（尚无 worker 桥接）。
pub fn diagnostic_text(config: &PluginConfig) -> String {
    let markdown_state = if config.qq_official_markdown {
        "开启"
    } else {
        "关闭"
    };
    let worker_state = if config.worker_enabled {
        format!(
            "已启用（P1 接入子进程，data_subdir={}, spawn>={}s, io>={}s）",
            config.data_subdir, config.spawn_timeout_secs, config.io_timeout_secs
        )
    } else {
        "未启用（P0 骨架阶段）".to_string()
    };
    format!(
        "仙尘插件 v{}（P0 骨架）\n协议版本：{}\nGo 内核桥接：{}\nMarkdown 渲染：{}\n迁移方案：docs/qimenbot-migration-plan.md",
        PLUGIN_VERSION, PROTOCOL_VERSION, worker_state, markdown_state
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
        match parse_config(config.config_json.as_str()) {
            Ok(parsed) => {
                replace_config(parsed);
                PluginInitResult::ok()
            }
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
        // P0 无后台线程与子进程；P1 起在此停止并 join worker IO 线程、
        // 杀死 xianchen-worker 子进程后再返回。
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
    fn diagnostic_text_reflects_p0_state() {
        let text = diagnostic_text(&PluginConfig::default());
        assert!(text.contains("P0"));
        assert!(text.contains("未启用"));
        assert!(text.contains(PLUGIN_VERSION));
    }
}
