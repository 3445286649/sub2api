# 模型与分组怎么选

::: info 先选客户端，再选分组
分组决定一个 Key 可以使用哪些协议、模型和计费规则。创建 Key 时选错分组，常见结果是 `403`、`model not found` 或客户端标签缺失。
:::

## 客户端与分组

| 目标客户端 | 需要确认 |
| --- | --- |
| Codex | 分组支持 OpenAI Responses，使用密钥弹窗有 Codex CLI 标签 |
| Claude Code | 分组支持 Messages / Claude Code，弹窗有 Claude Code 标签 |
| Gemini CLI | 分组支持 Gemini，弹窗有 Gemini CLI 标签 |
| OpenCode | 选择与分组平台一致的 OpenCode 配置 |
| 图片生成 | 分组中存在可用图片模型，并允许图片调用 |

## 创建 Key

1. 打开 [API 密钥](https://subapi.loucer.cn/keys)。
2. 点击 **创建密钥**。
3. 名称写清用途，例如 `codex-laptop`、`claude-vscode`。
4. 选择目标分组；需要控制风险时设置额度和有效期。
5. 创建后妥善保存完整 Key。

![创建 Key 并选择分组](/images/subapi/07-create-api-key-modal.png)

## 怎么确认分组选对了

- 点击 **使用密钥** 后能看到目标客户端标签。
- 复制生成配置后，客户端能发起最小测试。
- [使用记录](https://subapi.loucer.cn/usage) 中出现正确的 Key 和模型。

## 常见误区

- 账户有余额，不代表所有分组都能用。
- 模型显示在客户端里，不代表 Key 绑定分组支持它。
- 修改 Key 分组后，应完全重启客户端，避免继续使用旧配置。
