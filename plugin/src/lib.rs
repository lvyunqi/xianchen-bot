//! 仙尘 QimenBot 动态插件（P0 骨架）。
//!
//! 计划：P0 只提供插件身份、在线配置与诊断命令；
//! P1 起引入 Go worker 子进程桥接（见 docs/qimenbot-migration-plan.md）。

use std::sync::{OnceLock, RwLock};

use abi_stable_host_api::{CommandRequest, CommandResponse, PluginConfigRequest, PluginConfigResult, PluginInitConfig, PluginInitResult};
use qimen_dynamic_plugin_derive::dynamic_plugin;
use serde_json::Value;

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

static CONFIG: OnceLock<RwLock<PluginConfig>> = OnceLock::new();

fn config_slot() -> &'static RwLock<PluginConfig> {
    CONFIG.get_or_init(|| RwLock::new(PluginConfig::default()))
}

fn current_config() -> PluginConfig {
    config_slot().read().map(|slot| slot.clone()).unwrap_or_default()
}

fn replace_config(config: PluginConfig) {
    if let Ok(mut slot) = config_slot().write() {
        *slot = config;
    }
}

fn u64_field(value: &Value, key: &str) -> Option<u64> {
    value.get(key).and_then(Value::as_u64)
}

fn string_field(value: &Value, key: &str) -> Option<String> {
    value
        .get(key)
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|trimmed| !trimmed.is_empty())
        .map(str::to_string)
}

fn bool_field(value: &Value, key: &str) -> Option<bool> {
    value.get(key).and_then(Value::as_bool)
}

/// 解析宿主传入的 JSON 配置；空配置使用默认值，字段非法返回错误。
pub fn parse_config(config_json: &str) -> Result<PluginConfig, String> {
    let trimmed = config_json.trim();
    if trimmed.is_empty() {
        return Ok(PluginConfig::default());
    }
    let root: Value =
        serde_json::from_str(trimmed).map_err(|error| format!("插件配置不是有效 JSON: {error}"))?;
    let mut config = PluginConfig::default();
    if let Some(worker) = root.get("worker") {
        if let Some(enabled) = bool_field(worker, "enabled") {
            config.worker_enabled = enabled;
        }
        if let Some(timeout) = u64_field(worker, "spawn_timeout_secs") {
            config.spawn_timeout_secs = timeout.clamp(5, 120);
        }
        if let Some(timeout) = u64_field(worker, "io_timeout_secs") {
            config.io_timeout_secs = timeout.clamp(5, 29);
        }
        if let Some(subdir) = string_field(worker, "data_subdir") {
            config.data_subdir = subdir;
        }
    }
    if let Some(messages) = root.get("messages") {
        if let Some(markdown) = bool_field(messages, "qq_official_markdown") {
            config.qq_official_markdown = markdown;
        }
    }
    Ok(config)
}

/// 诊断命令文本：P0 阶段反映插件自身状态（尚无 worker 桥接）。
pub fn diagnostic_text(config: &PluginConfig) -> String {
    let markdown_state = if config.qq_official_markdown { "开启" } else { "关闭" };
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
    fn timeouts_are_clamped_to_protocol_limits() {
        let config = parse_config(
            r#"{"worker":{"spawn_timeout_secs":1,"io_timeout_secs":999}}"#,
        )
        .unwrap();
        assert_eq!(config.spawn_timeout_secs, 5);
        assert_eq!(config.io_timeout_secs, 29);
    }

    #[test]
    fn diagnostic_text_reflects_p0_state() {
        let text = diagnostic_text(&PluginConfig::default());
        assert!(text.contains("P0"));
        assert!(text.contains("未启用"));
        assert!(text.contains(PLUGIN_VERSION));
    }
}
