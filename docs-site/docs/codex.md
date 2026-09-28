# Codex 接入

让 Codex CLI 和 Codex 桌面端通过 CodePlan 调用模型。有两种方式，**推荐用 CCSwitch**。

::: info 需要 OpenAI 分组
Codex 使用 OpenAI Responses 协议，Key 要绑定支持它的分组。点击 **使用密钥** 能看到 **Codex CLI** 标签就说明分组没问题，见 [模型与分组怎么选](/models-and-groups)。
:::

## 方式一：CCSwitch 一键导入（推荐）

按 [CCSwitch 接入](/ccswitch) 完成「导入 → 启用到 **使用中** → 刷新用量」，然后：

1. **完全关闭**已经打开的 Codex 终端、Codex 桌面端或 VS Code 窗口。
2. 重新打开 Codex，发一句测试：

```text
用一句话回复：Codex 连接成功。
```

3. 到 [使用记录](https://codeplan.codes/usage) 确认出现这次调用。

## 方式二：手动写入配置文件

不用 CCSwitch 时，把站内生成的配置写进 Codex 的配置目录。

### 1. 复制站内生成的配置

打开 [API 密钥](https://codeplan.codes/keys)，在目标 Key 上点击 **使用密钥**，选择 **Codex CLI** 标签，再选你的系统。认证模式保持默认的 **兼容模式**。

弹窗会生成两个文件，**直接复制弹窗里的内容**。下面是结构示意，模型名等细节以弹窗为准：

::: code-group

```toml [config.toml]
model_provider = "OpenAI"
model = "以弹窗生成为准"
review_model = "以弹窗生成为准"
disable_response_storage = true
network_access = "enabled"

[model_providers.OpenAI]
name = "OpenAI"
base_url = "https://subapi.loucer.cn/v1"
wire_api = "responses"
requires_openai_auth = true
```

```json [auth.json]
{
  "OPENAI_API_KEY": "sk-你的密钥"
}
```

:::

::: warning 以弹窗内容为准
弹窗里的 `config.toml` 还带有模型目录等配置项，比上面的示意更完整。请复制弹窗原文，不要照抄示意。
:::

### 2. 保存到 Codex 配置目录

| 系统 | 配置目录 |
| --- | --- |
| macOS / Linux | `~/.codex/` |
| Windows | `%userprofile%\.codex\` |

- macOS / Linux：目录不存在时先运行 `mkdir -p ~/.codex`。
- Windows：按 <kbd>Win</kbd> + <kbd>R</kbd>，输入 `%userprofile%\.codex` 打开目录；不存在就手动新建一个。

把两个文件分别保存为 `config.toml` 和 `auth.json`。如果 `config.toml` 已经存在，**把弹窗内容放在文件开头**。

### 3. 重启 Codex 并验证

完全退出 Codex 再重新打开，发一句测试消息，然后到 [使用记录](https://codeplan.codes/usage) 确认有新记录。

::: tip 为什么一定要重启
Codex 只在启动时读取一次 `config.toml`。改完配置不重启，会继续用旧配置。
:::

## VS Code 里的 Codex 插件

插件和 CLI 共用同一份 `~/.codex/config.toml`。按上面任意一种方式配好后，在 VS Code 里执行 **重新加载窗口**（<kbd>Ctrl</kbd>/<kbd>Cmd</kbd> + <kbd>Shift</kbd> + <kbd>P</kbd> → `Reload Window`）即可。

![VS Code Codex 插件配置示意](/images/subapi/vscode-codex-settings.png)

## 确认真的走了 CodePlan

以站内 [使用记录](https://codeplan.codes/usage) 为准：

- **有记录**：请求已经走 CodePlan。
- **没记录**：Codex 还在用官方账号登录，或者用的是旧配置。退出官方登录、确认配置已保存，再完全重启 Codex。

## 常见现象

| 现象 | 优先检查 |
| --- | --- |
| 能回复，但使用记录里没有 | Codex 仍在用官方账号或其它供应商，见 [排查](/troubleshooting#能回复但没有使用记录) |
| `401` / `invalid api key` | `auth.json` 里的 Key 是否完整、有没有多余空格 |
| `model not found` | 模型名是否来自弹窗；Key 分组是否支持该模型 |
| 改了配置不生效 | 是否完全退出并重启了 Codex |

更多报错见 [常见报错排查](/troubleshooting)。想在 Codex 里直接生成图片，见 [Codex 生图 Skill](/codex-imagegen2)。
