# API Key 创建与安全

API Key 是客户端访问 subapi 的凭证。CCSwitch、Codex、Claude Code、VS Code 插件或其它 OpenAI 兼容客户端，都靠它来调用服务。

## 创建 API Key

点击左侧 **API 密钥**，页面上方会显示 API 端点：

```text
https://subapi.loucer.cn
```

![API 密钥页面](/images/subapi/04-api-keys-page.png)

点击 **创建密钥** 后，建议这样填：

| 字段 | 建议 |
| --- | --- |
| 名称 | 写用途，例如 `ccswitch`、`电脑A`、`备用key` |
| 分组 | 选择你要使用的分组或订阅分组 |
| 额度限制 | 不限制填 0；想控制风险就填一个上限 |
| 有效期 | 长期使用选永久；临时测试可设置过期 |

![创建密钥弹窗](/images/subapi/07-create-api-key-modal.png)

## 查看客户端配置

在 API 密钥列表里点击 **使用密钥**，可以看到站内生成的配置示例。

![使用密钥弹窗](/images/subapi/08-use-api-key-modal.png)

你最终只需要记住两项：

- **Base URL**：`https://subapi.loucer.cn`
- **API Key**：你自己的 `sk-...`

如果客户端要求 OpenAI 兼容地址，一般填：

```text
https://subapi.loucer.cn/v1
```

如果客户端单独有 Base URL 字段，一般填：

```text
https://subapi.loucer.cn
```

不确定时，以站内 **使用密钥** 弹窗给出的配置为准。

## Key 安全

- API Key 等同于消费凭证，不要发到公开群、论坛或陌生人。
- 截图教程或问问题时，请打码完整 Key。
- 怀疑泄露时，立刻禁用或删除旧 Key，重新创建新 Key。
- 建议不同客户端使用不同 Key，方便限制额度和排查费用。

## 分组很重要

没有分组的 Key 可能无法正常调度。如果 CCSwitch 导入后请求失败，优先检查 Key 是否分配到了可用分组。
