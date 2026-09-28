# API Key 与接入地址

API Key 是客户端访问 CodePlan 的凭证。CCSwitch、Codex、Claude Code、VS Code 插件和其它兼容客户端，都靠它来调用服务。

## 创建 API Key

1. 打开 [API 密钥](https://codeplan.codes/keys)，点击 **创建密钥**。
2. 按下表填写，然后保存。

![创建密钥弹窗](/images/subapi/07-create-api-key-modal.png)

| 字段 | 建议 |
| --- | --- |
| 名称 | 写清用途，例如 `codex-laptop`、`claude-vscode`、`备用` |
| 分组 | **必须选**。分组决定能用哪些客户端和模型，见 [模型与分组怎么选](/models-and-groups) |
| 额度限制 | 不限制填 `0`；想控制风险就填一个上限 |
| 有效期 | 长期使用选永久；临时测试可以设置过期 |

::: warning 没有分组的 Key 用不了
没有分组的 Key 无法调度，点击 **使用密钥** 时也会提示「请先分配分组」。
:::

## 获取客户端配置

在 API 密钥列表里点击 **使用密钥**，站内会按这个 Key 的分组生成对应客户端的配置，可以直接复制。

![使用密钥弹窗](/images/subapi/08-use-api-key-modal.png)

**配置以这个弹窗为准。** 它会根据分组自动选好地址、协议和模型，比手写可靠。本文档里的配置示例都和弹窗生成的内容一致。

## 接入地址怎么填

CodePlan 有两个访问地址，指向同一个站点，账户、余额和 Key 完全通用：

- `https://codeplan.codes`
- `https://subapi.loucer.cn`

**在客户端里填的地址统一用 `subapi.loucer.cn`**，和站内生成的配置保持一致。不同客户端对结尾要不要加 `/v1` 要求不同：

| 客户端 | 地址填写 | 说明 |
| --- | --- | --- |
| Claude Code（及 Anthropic 协议插件） | `https://subapi.loucer.cn` | **不加 `/v1`**，客户端会自己拼接 `/v1/messages` |
| Codex / OpenAI 兼容客户端 | `https://subapi.loucer.cn/v1` | 需要带 `/v1` |
| CCSwitch | `https://subapi.loucer.cn` | 用 **导入到 CCS** 自动填写，无需手动输入 |
| Gemini CLI、OpenCode | 以站内生成为准 | 不同分组路径不同，例如 Antigravity 分组带 `/antigravity` |

::: tip 地址填错的典型表现
- 报 `404` 或路径错误：多半是 `/v1` 加多了或加少了，对照上表改一下。
- 能回复但站内没有使用记录：请求没有走到 CodePlan，见 [常见报错排查](/troubleshooting#能回复但没有使用记录)。
:::

## Key 安全

- API Key 等同于消费凭证，不要发到公开群、论坛或陌生人。
- 截图提问时请打码完整 Key，只说 Key 的名称。
- 含真实 Key 的配置文件（`config.toml`、`settings.json`、`opencode.json`）不要提交到代码仓库。
- 怀疑泄露时，立刻禁用或删除旧 Key，再新建一个。
- 建议每个客户端用一个单独的 Key，方便分别限额、看消耗和排查问题。
