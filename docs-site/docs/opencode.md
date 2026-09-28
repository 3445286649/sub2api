# OpenCode 接入

OpenCode 的配置会随 Key 的分组变化，**直接复制站内生成的 `opencode.json`** 最可靠。

## 配置步骤

1. 打开 [API 密钥](https://codeplan.codes/keys)。
2. 找到目标 Key，点击 **使用密钥**。
3. 选择 **OpenCode** 标签。
4. 复制生成的 `opencode.json`（包含 Provider、地址、模型和 API Key）。
5. 保存后重启 OpenCode。

::: tip 为什么不给一份通用配置
OpenAI、Anthropic、Gemini、Antigravity 分组使用的协议和路径都不同。Antigravity 分组还会同时生成 Claude 和 Gemini 两份配置。站内弹窗会按当前 Key 的分组生成匹配的内容，比手写模板可靠。
:::

## 验证

- OpenCode 能完成一次测试对话。
- [使用记录](https://codeplan.codes/usage) 出现对应的调用。
- 模型名和费用符合预期。

## 常见问题

| 现象 | 处理方式 |
| --- | --- |
| 弹窗里没有 OpenCode 标签 | 检查 Key 是否绑定了分组 |
| `401` | 重新复制 Key，检查有没有多余空格 |
| `403` | 余额不足，或当前分组不允许这个客户端，读报错里的提示 |
| `404` · `model_not_found` | 从站内生成的配置重新复制模型名 |

::: warning 保护 API Key
`opencode.json` 里有完整 Key，不要上传到公开仓库或发到群聊。
:::

更多报错见 [常见报错排查](/troubleshooting)。
