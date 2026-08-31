use abi_stable_host_api::{BotApi, InterceptorRequest};
use serde_json::{Value, json};

use crate::worker::{GamePayload, InboundMessage};

pub fn queue_response(
    request: &InterceptorRequest,
    inbound: &InboundMessage,
    payload: &GamePayload,
    markdown_enabled: bool,
) -> bool {
    let Some(segments) = response_segments(payload, markdown_enabled) else {
        return false;
    };
    if inbound.is_private {
        BotApi::send_private_rich(request.sender_id.as_str(), &segments);
    } else {
        BotApi::send_group_rich(&inbound.group_id, &segments);
    }
    if !payload.image_base64.trim().is_empty() {
        let image_segments = image_segments(payload);
        if inbound.is_private {
            BotApi::send_private_rich(request.sender_id.as_str(), &image_segments);
        } else {
            BotApi::send_group_rich(&inbound.group_id, &image_segments);
        }
    }
    true
}

pub fn queue_broadcasts(inbound: &InboundMessage, payload: &GamePayload, markdown_enabled: bool) {
    let content = payload.broadcast.trim();
    if content.is_empty() || inbound.account_id.is_empty() {
        return;
    }
    let segments = if markdown_enabled {
        json!([{"type":"markdown","data":{"content":content}}]).to_string()
    } else {
        json!([{"type":"text","data":{"text":content}}]).to_string()
    };
    for group_id in &payload.broadcast_targets {
        if !group_id.trim().is_empty() {
            let _accepted = BotApi::for_account(&inbound.account_id)
                .send_rich("group", group_id, "{}", &segments)
                .is_accepted();
        }
    }
}

fn response_segments(payload: &GamePayload, markdown_enabled: bool) -> Option<String> {
    if markdown_enabled {
        let Some(markdown) = official_markdown(payload) else {
            return text_segment(payload);
        };
        let mut segments = vec![json!({
            "type":"markdown",
            "data":{"content":markdown}
        })];
        if let Some(keyboard) = command_keyboard(&payload.actions) {
            segments.push(keyboard);
        }
        return Some(Value::Array(segments).to_string());
    }
    text_segment(payload)
}

fn image_segments(payload: &GamePayload) -> String {
    json!([{
        "type":"image",
        "data":{"file":format!("base64://{}", payload.image_base64.trim())}
    }])
    .to_string()
}

fn text_segment(payload: &GamePayload) -> Option<String> {
    let text = if !payload.text_fallback.trim().is_empty() {
        payload.text_fallback.trim()
    } else if !payload.content.trim().is_empty() {
        payload.content.trim()
    } else {
        payload.title.trim()
    };
    (!text.is_empty()).then(|| json!([{"type":"text","data":{"text":text}}]).to_string())
}

fn official_markdown(payload: &GamePayload) -> Option<String> {
    let title = payload.title.trim();
    let body = if !payload.markdown_content.trim().is_empty() {
        payload.markdown_content.trim()
    } else if !payload.content.trim().is_empty() {
        payload.content.trim()
    } else if !payload.markdown.trim().is_empty() {
        payload.markdown.trim()
    } else {
        ""
    };
    if title.is_empty() && body.is_empty() {
        return (!payload.image_base64.trim().is_empty()).then(|| "图片已生成，请查看下一条消息。".to_string());
    }
    let mut sections = Vec::new();
    if !title.is_empty() {
        sections.push(format!("## {}", escape_heading(title)));
    }
    if !body.is_empty() {
        sections.push(strip_legacy_inline_commands(body));
    } else if !payload.image_base64.trim().is_empty() {
        sections.push("图片已生成，请查看下一条消息。".to_string());
    }
    if !payload.image_url.trim().is_empty() && is_http_url(payload.image_url.trim()) {
        sections.insert(
            1.min(sections.len()),
            format!("![#250px #100px]({})", payload.image_url.trim()),
        );
    }
    Some(sections.join("\n"))
}

fn escape_heading(value: &str) -> String {
    value
        .replace('\\', "\\\\")
        .replace('*', "\\*")
        .replace('_', "\\_")
        .replace('[', "\\[")
        .replace(']', "\\]")
        .replace('#', "\\#")
}

fn is_http_url(value: &str) -> bool {
    value.starts_with("https://") || value.starts_with("http://")
}

fn strip_legacy_inline_commands(value: &str) -> String {
    const PREFIX: &str = "mqqapi://aio/inlinecmd?";
    let mut output = String::with_capacity(value.len());
    let mut cursor = 0;
    while let Some(relative_open) = value[cursor..].find('[') {
        let open = cursor + relative_open;
        output.push_str(&value[cursor..open]);
        let Some(relative_close) = value[open + 1..].find("](") else {
            output.push_str(&value[open..]);
            return output;
        };
        let close = open + 1 + relative_close;
        let target_start = close + 2;
        let Some(relative_end) = value[target_start..].find(')') else {
            output.push_str(&value[open..]);
            return output;
        };
        let target_end = target_start + relative_end;
        let target = &value[target_start..target_end];
        if target.starts_with(PREFIX) {
            output.push_str(&value[open + 1..close]);
        } else {
            output.push_str(&value[open..target_end + 1]);
        }
        cursor = target_end + 1;
    }
    output.push_str(&value[cursor..]);
    output
}

fn command_keyboard(actions: &[String]) -> Option<Value> {
    let buttons = actions
        .iter()
        .map(|action| action.trim())
        .filter(|action| is_clickable_command(action))
        .take(8)
        .enumerate()
        .map(|(index, action)| {
            let label = command_button_label(action);
            json!({
                "label":label,
                "visited_label":label,
                "action_type":2,
                "action_data":action,
                "style":if index == 0 { 1 } else { 0 },
                "permission_type":2
            })
        })
        .collect::<Vec<_>>();
    if buttons.is_empty() {
        return None;
    }
    Some(json!({
        "type":"keyboard",
        "data":{"content":{"rows":buttons
            .chunks(2)
            .map(|row| json!({"buttons":row}))
            .collect::<Vec<_>>()}}
    }))
}

fn is_clickable_command(command: &str) -> bool {
    !command.is_empty()
        && !command
            .chars()
            .any(|character| matches!(character, '<' | '>' | '[' | ']'))
}

fn command_button_label(command: &str) -> String {
    const MAX_LABEL_CHARS: usize = 12;
    if command.chars().count() <= MAX_LABEL_CHARS {
        return command.to_string();
    }
    let mut label = command
        .chars()
        .take(MAX_LABEL_CHARS - 3)
        .collect::<String>();
    label.push_str("...");
    label
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn markdown_reply_contains_keyboard_rows() {
        let payload = GamePayload {
            markdown: "## 状态".to_string(),
            actions: vec!["菜单".to_string(), "状态".to_string()],
            ..GamePayload::default()
        };
        let segments = response_segments(&payload, true).unwrap();
        assert!(segments.contains("markdown"));
        assert!(segments.contains("keyboard"));
        assert!(segments.contains("action_data"));
    }

    #[test]
    fn image_reply_uses_followup_base64_segment() {
        let payload = GamePayload {
            title: "状态图".to_string(),
            image_base64: "YWJj".to_string(),
            image_only: true,
            ..GamePayload::default()
        };
        let main = response_segments(&payload, true).unwrap();
        assert!(main.contains("markdown"));
        assert!(main.contains("下一条消息"));
        assert!(image_segments(&payload).contains("base64://YWJj"));
    }

    #[test]
    fn placeholder_commands_are_not_buttons() {
        assert!(command_keyboard(&["购买 <物品>".to_string()]).is_none());
    }

    #[test]
    fn legacy_inline_links_are_rendered_as_plain_labels() {
        let payload = GamePayload {
            title: "菜单".to_string(),
            markdown_content: "[状态](mqqapi://aio/inlinecmd?command=%E7%8A%B6%E6%80%81&enter=false&reply=false)".to_string(),
            ..GamePayload::default()
        };
        let markdown = official_markdown(&payload).unwrap();
        assert!(markdown.contains("状态"));
        assert!(!markdown.contains("mqqapi://"));
    }
}
