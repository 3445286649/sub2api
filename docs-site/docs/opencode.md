# OpenCode 接入

OpenCode 配置会随 Key 的平台变化，优先复制站内生成的 `opencode.json`。

## 推荐方式

1. 打开 [API 密钥](https://subapi.loucer.cn/keys)。
2. 找到目标 Key，点击 **使用密钥**。
3. 选择 **OpenCode** 标签。
4. 复制生成的 Provider、Base URL、模型和 API Key 配置。
5. 保存后重启 OpenCode。

## 为什么不能只给一份通用配置

OpenAI、Anthropic、Gemini 和 Antigravity 分组使用的协议与路径不同。站内弹窗会根据当前 Key 的分组生成匹配内容，因此比手工模板可靠。

## 成功标志

- OpenCode 能完成一次最小对话。
- [使用记录](https://subapi.loucer.cn/usage) 出现对应记录。
- 模型名和费用符合预期。

## 排查

- 没有 OpenCode 标签：检查 Key 分组和当前平台。
- `401`：重新复制 Key，检查是否包含空格。
- `403`：当前分组或客户端类型不允许。
- `model not found`：从站内生成配置重新复制模型名。

::: warning 保护 API Key
不要把包含真实 Key 的 `opencode.json` 上传到公开仓库或发到群聊。
:::
