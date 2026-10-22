# 模型与分组怎么选

::: info 先选客户端，再选分组
**分组决定一个 Key 能用哪些客户端、哪些模型、按什么规则计费。** 账户有余额不代表所有分组都能用，选错分组常见的结果是 `403`、`model_not_found`，或者 **使用密钥** 弹窗里找不到你要的客户端。
:::

## 分组平台与客户端

创建 Key 时选的分组属于某个平台。平台决定点击 **使用密钥** 后，弹窗里会出现哪些客户端标签：

| 分组平台 | 弹窗里会出现的客户端 |
| --- | --- |
| OpenAI | Codex CLI、Codex CLI (WebSocket)、OpenCode；分组开启「Messages 调度」时还有 Claude Code |
| Anthropic / Claude | Claude Code、Codex CLI、OpenCode |
| Gemini | Gemini CLI、Codex CLI、OpenCode |
| Antigravity | Claude Code、Gemini CLI、Codex CLI、OpenCode |
| Grok | Grok CLI、Claude Code、Codex CLI、OpenCode |
| DeepSeek、MiniMax、组合路由 | Claude Code、Codex CLI、OpenCode |

**反过来用这张表：** 想用哪个客户端，就选一个弹窗里会出现它的分组。

::: tip 为什么我的弹窗里没有 Claude Code
最常见的原因是 Key 绑的是 **OpenAI 分组**，而这个分组没有开启 Messages 调度。换一个 Anthropic 或 Antigravity 分组的 Key 即可。
:::

## 创建 Key 时选分组

1. 打开 [API 密钥](https://codeplan.codes/keys)，点击 **创建密钥**。
2. 名称写清用途，例如 `codex-laptop`、`claude-vscode`。
3. 先选 **厂商**（OpenAI / Anthropic / 国产模型 / 其他），再按上表选择目标分组；需要控制风险时设置额度和有效期。
4. 保存后妥善保管完整 Key。

![创建 Key 并选择分组](/images/subapi/07-create-api-key-modal.png)

## 确认分组选对了

- 点击 **使用密钥**，能看到目标客户端的标签。
- 复制生成的配置后，客户端能完成一次测试对话。
- [使用记录](https://codeplan.codes/usage) 里出现了正确的 Key 和模型。

## 用图片模型

图片生成（如 [Codex 生图 Skill](/codex-imagegen2)）需要 Key 所在分组里有可用的图片模型，例如 `gpt-image-2`（以分组实际开放的模型为准）。不确定时，先用一个小尺寸（1K）请求试一下。

## 常见误区

- **账户有余额，不代表所有分组都能用。** 余额是账户级的，能用哪些模型由分组决定。
- **客户端里列出了某个模型，不代表 Key 的分组支持它。** 以 **使用密钥** 弹窗生成的模型为准。
- **改了 Key 的分组之后，要完全重启客户端**，否则会继续用旧配置。
