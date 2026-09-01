use abi_stable_host_api::InterceptorRequest;
use serde_json::{Map, Value};
use std::sync::atomic::{AtomicBool, Ordering};

use crate::worker::InboundMessage;

const MAX_ID_CHARS: usize = 256;

static CONTEXT_FALLBACK_WARNED: AtomicBool = AtomicBool::new(false);

pub fn resolve_inbound(request: &InterceptorRequest) -> Result<InboundMessage, String> {
    let sender_id = valid_id(request.sender_id.as_str(), "发送者 OpenID")?;
    let raw: Value = serde_json::from_str(request.raw_event_json.as_str())
        .map_err(|_| "原始事件不是有效 JSON".to_string())?;
    let root = raw
        .as_object()
        .ok_or_else(|| "原始事件必须是 JSON 对象".to_string())?;

    // 宿主注入的可信上下文（新版 qimenbotd）。缺失时回退到网关事件形状推导：
    // 只接受官方 QQ 事件类型白名单，账号身份回退为部署别名 bot_id。
    let account_id = match root.get("qimen_context").and_then(Value::as_object) {
        Some(context) => {
            if context.get("version").and_then(Value::as_u64) != Some(1) {
                return Err("qimen_context 版本不受支持".to_string());
            }
            if context.get("protocol").and_then(Value::as_str) != Some("qq-official") {
                return Err("不是 qq-official 消息".to_string());
            }
            match context.get("account_id").and_then(Value::as_str) {
                Some(account_id) => valid_id(account_id, "Bot account_id")?,
                None => request.bot_id.as_str().trim().to_string(),
            }
        }
        None => {
            if !CONTEXT_FALLBACK_WARNED.swap(true, Ordering::Relaxed) {
                eprintln!(
                    "仙尘：宿主事件缺少 qimen_context，已回退到网关事件推导（建议升级 qimenbotd）"
                );
            }
            request.bot_id.as_str().trim().to_string()
        }
    };

    let event_type = root
        .get("event_type")
        .and_then(Value::as_str)
        .or_else(|| root.get("t").and_then(Value::as_str))
        .or_else(|| {
            root.get("qqbot_payload")
                .and_then(|payload| payload.get("event_type"))
                .and_then(Value::as_str)
        })
        .ok_or_else(|| "官方 QQ 事件缺少 event_type".to_string())?;
    let payload = root
        .get("qqbot_payload")
        .and_then(Value::as_object)
        .or_else(|| root.get("d").and_then(Value::as_object))
        .ok_or_else(|| "官方 QQ 事件缺少消息载荷".to_string())?;
    let (group_id, is_private) = match event_type {
        "GROUP_AT_MESSAGE_CREATE" | "GROUP_MESSAGE_CREATE" => {
            let payload_group = payload
                .get("group_openid")
                .and_then(Value::as_str)
                .or_else(|| root.get("group_openid").and_then(Value::as_str))
                .ok_or_else(|| "官方 QQ 群消息缺少 group_openid".to_string())?;
            let group_id = valid_id(payload_group, "群 OpenID")?;
            if !request.group_id.is_empty() && request.group_id.as_str() != group_id {
                return Err("规范化 group_id 与 QQ group_openid 不一致".to_string());
            }
            (group_id, false)
        }
        "C2C_MESSAGE_CREATE" => (String::new(), true),
        "AT_MESSAGE_CREATE" | "MESSAGE_CREATE" | "DIRECT_MESSAGE_CREATE" => {
            return Err("频道或 DMS 暂不进入仙尘游戏命令链".to_string());
        }
        _ => return Err(format!("官方 QQ 事件类型 {event_type} 不支持游戏命令")),
    };
    let mut text = request.message_text.as_str().trim().to_string();
    if text.is_empty() || text.chars().any(char::is_control) {
        return Err("消息文本为空或包含控制字符".to_string());
    }
    if let Some(target) = resolve_target_mention(payload, event_type)?
        && !text.split_whitespace().any(|part| part == target)
    {
        text.push(' ');
        text.push_str(&target);
    }
    Ok(InboundMessage {
        kind: "msg",
        group_id,
        sender_id,
        sender_name: request.sender_nickname.as_str().trim().to_string(),
        text,
        is_private,
        account_id,
        message_id: request.message_id.as_str().trim().to_string(),
    })
}

fn resolve_target_mention(
    payload: &Map<String, Value>,
    event_type: &str,
) -> Result<Option<String>, String> {
    let Some(mentions) = payload.get("mentions") else {
        return Ok(None);
    };
    let mentions = mentions
        .as_array()
        .ok_or_else(|| "qqbot_payload.mentions 必须是数组".to_string())?;
    let mut targets = Vec::new();
    for mention in mentions {
        let mention = mention
            .as_object()
            .ok_or_else(|| "qqbot_payload.mentions 包含无效提及".to_string())?;
        if optional_bool(mention, "is_you")? == Some(true)
            || optional_bool(mention, "bot")? == Some(true)
            || optional_string(mention, "scope")? == Some("all")
        {
            continue;
        }
        let target = match event_type {
            "GROUP_AT_MESSAGE_CREATE" | "GROUP_MESSAGE_CREATE" => {
                optional_string(mention, "member_openid")?.or(optional_string(mention, "id")?)
            }
            "C2C_MESSAGE_CREATE" => {
                optional_string(mention, "user_openid")?.or(optional_string(mention, "id")?)
            }
            _ => None,
        };
        if let Some(target) = target {
            targets.push(valid_id(target, "目标用户 OpenID")?);
        }
    }
    match targets.as_slice() {
        [] => Ok(None),
        [target] => Ok(Some(target.clone())),
        _ => Err("一次只能 @ 一名目标用户".to_string()),
    }
}

fn optional_bool(object: &Map<String, Value>, field: &str) -> Result<Option<bool>, String> {
    object
        .get(field)
        .map(|value| {
            value
                .as_bool()
                .ok_or_else(|| format!("QQ 提及字段 {field} 必须是布尔值"))
        })
        .transpose()
}

fn optional_string<'a>(
    object: &'a Map<String, Value>,
    field: &str,
) -> Result<Option<&'a str>, String> {
    object
        .get(field)
        .map(|value| {
            value
                .as_str()
                .ok_or_else(|| format!("QQ 提及字段 {field} 必须是字符串"))
        })
        .transpose()
}

fn valid_id(value: &str, label: &str) -> Result<String, String> {
    let value = value.trim();
    if value.is_empty()
        || value.chars().count() > MAX_ID_CHARS
        || value.chars().any(char::is_control)
    {
        return Err(format!(
            "{label} 必须是 1 到 {MAX_ID_CHARS} 个无控制字符的字符串"
        ));
    }
    Ok(value.to_string())
}

#[cfg(test)]
mod tests {
    use abi_stable::std_types::RString;
    use serde_json::json;

    use super::*;

    fn request(event: Value) -> InterceptorRequest {
        InterceptorRequest {
            bot_id: RString::from("qq-main"),
            sender_id: RString::from("user-openid"),
            group_id: RString::from("group-openid"),
            message_text: RString::from("状态"),
            raw_event_json: RString::from(event.to_string()),
            sender_nickname: RString::from("道友"),
            message_id: RString::from("message-1"),
            timestamp: 0,
        }
    }

    #[test]
    fn resolves_trusted_group_context() {
        let inbound = resolve_inbound(&request(json!({
            "event_type": "GROUP_AT_MESSAGE_CREATE",
            "qimen_context": {"version":1,"protocol":"qq-official","account_id":"bot-account"},
            "qqbot_payload": {"group_openid":"group-openid","mentions":[]}
        })))
        .unwrap();
        assert_eq!(inbound.group_id, "group-openid");
        assert!(!inbound.is_private);
        assert_eq!(inbound.account_id, "bot-account");
    }

    #[test]
    fn appends_one_structured_target_mention() {
        let mut request = request(json!({
            "event_type": "GROUP_MESSAGE_CREATE",
            "qimen_context": {"version":1,"protocol":"qq-official","account_id":"bot-account"},
            "qqbot_payload": {
                "group_openid":"group-openid",
                "mentions":[{"member_openid":"target-openid","is_you":false}]
            }
        }));
        request.message_text = RString::from("结缘");
        assert_eq!(
            resolve_inbound(&request).unwrap().text,
            "结缘 target-openid"
        );
    }

    #[test]
    fn resolves_gateway_event_without_qimen_context() {
        let inbound = resolve_inbound(&request(json!({
            "op": 0,
            "t": "GROUP_MESSAGE_CREATE",
            "d": {
                "content": "菜单",
                "group_openid": "group-openid",
                "author": {"id": "user-openid"}
            }
        })))
        .unwrap();
        assert_eq!(inbound.group_id, "group-openid");
        assert!(!inbound.is_private);
        assert_eq!(inbound.account_id, "qq-main");
        assert_eq!(inbound.text, "状态");
    }

    #[test]
    fn untrusted_or_channel_events_fail_open_at_caller() {
        assert!(resolve_inbound(&request(json!({"qqbot_payload":{}}))).is_err());
        assert!(
            resolve_inbound(&request(json!({
                "event_type":"AT_MESSAGE_CREATE",
                "qimen_context":{"version":1,"protocol":"qq-official","account_id":"bot-account"},
                "qqbot_payload":{"channel_id":"channel"}
            })))
            .is_err()
        );
    }
}
