# VS Code 插件接入

VS Code 里的 AI 插件大致分三类，**配置方式各不相同**。先找到你用的插件属于哪一类，再按对应的方法配置。

| 插件 | 读取的配置 | 怎么配 |
| --- | --- | --- |
| Codex 插件 | `~/.codex/config.toml` | 见 [Codex 接入](/codex) |
| Claude Code 插件 | `~/.claude/settings.json` | 见 [Claude Code 接入](/claude-code) |
| 其它 OpenAI 兼容插件（Cline、Continue、Roo Code 等） | 插件自己的设置页 | 见本页下文 |

::: tip 最省事的方式
Codex 插件和 Claude Code 插件都可以用 [CCSwitch](/ccswitch) 一键导入，不用手动编辑任何文件。
:::

## Codex 插件、Claude Code 插件

这两个插件和对应的终端版**共用同一份配置文件**，所以不需要在插件里单独填地址和 Key：

1. 按 [Codex 接入](/codex) 或 [Claude Code 接入](/claude-code) 配置好。
2. 在 VS Code 里执行 **重新加载窗口**：<kbd>Ctrl</kbd>/<kbd>Cmd</kbd> + <kbd>Shift</kbd> + <kbd>P</kbd> → `Reload Window`。
3. 发一句测试消息，再到 [使用记录](https://codeplan.codes/usage) 确认。

::: warning Claude Code 插件不要选 OpenAI 兼容
Claude Code 插件使用 Anthropic 协议，读取的是 `ANTHROPIC_BASE_URL`。按 OpenAI 兼容的方式去配它，是连不上的。
:::

## 其它 OpenAI 兼容插件

Cline、Continue、Roo Code 这类插件在自己的设置页里填写配置，核心是下面四项：

| 字段 | 填写 |
| --- | --- |
| Provider / API 类型 | OpenAI Compatible（也可能叫 Custom OpenAI、OpenAI 兼容） |
| Base URL | `https://subapi.loucer.cn/v1` |
| API Key | 站内创建的 `sk-...` |
| Model | Key 所在分组支持的模型名 |

![VS Code 通用 OpenAI 兼容配置示意](/images/subapi/vscode-openai-compatible-settings.png)

::: tip Base URL 报路径错误时
少数插件会自己在地址后面拼 `/v1`。填 `https://subapi.loucer.cn/v1` 报 `404` 或路径错误时，改成 `https://subapi.loucer.cn` 再试。地址规则见 [API Key 与接入地址](/api-key#接入地址怎么填)。
:::

::: info 需要 OpenAI 分组
OpenAI 兼容插件走的是 OpenAI 协议，Key 要绑定 OpenAI 分组。模型名不确定时，点 **使用密钥** 看弹窗里生成的模型。
:::

## 确认真的走了 CodePlan

配置后做三步检查：

1. 插件能正常回复。
2. 站内 [使用记录](https://codeplan.codes/usage) 出现这次调用。
3. 记录里的模型、Key、费用符合预期。

::: warning 不要混用官方登录
有些插件同时支持「官方账号登录」和「自定义 API」。插件能回复但使用记录里没有，通常说明它还在用官方账号。退出官方登录，再重新加载窗口。
:::

## 常见现象

| 现象 | 处理方式 |
| --- | --- |
| 找不到 OpenAI Compatible 选项 | 这个插件不支持自定义接入，只能用它自带的官方渠道 |
| 填了 `/v1` 后报路径错误 | 改成 `https://subapi.loucer.cn`，该插件会自己拼接 `/v1` |
| 能回复但使用记录里没有 | 退出官方账号模式，确认 Provider 选的是 OpenAI 兼容，再重新加载窗口 |
| 改了配置不生效 | 执行 **重新加载窗口**，或完全关闭 VS Code 再打开 |

更多报错见 [常见报错排查](/troubleshooting)。
