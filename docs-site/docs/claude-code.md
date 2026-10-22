# Claude Code 接入

让 Claude Code（终端版和 VS Code 插件）通过 CodePlan 调用模型。有两种方式，**推荐用 CCSwitch**。

::: info 需要支持 Claude Code 的分组
点击 **使用密钥** 能看到 **Claude Code** 标签，就说明这个 Key 的分组支持它，见 [模型与分组怎么选](/models-and-groups)。
:::

## 方式一：CCSwitch 一键导入（推荐）

按 [CCSwitch 接入](/ccswitch) 完成「导入 → 启用到 **使用中** → 刷新用量」，然后：

1. **完全关闭**已经打开的 Claude Code 终端或 VS Code 窗口。
2. 重新打开 Claude Code，发一句测试：

```text
用一句话回复：连接测试成功。
```

3. 到 [使用记录](https://codeplan.codes/usage) 确认出现这次调用。

::: warning 先启用，再打开
先打开 Claude Code 再去 CCSwitch 切换，已经运行的 Claude Code 不会读取新配置。
:::

## 方式二：手动配置

Claude Code 使用 **Anthropic 协议**，要设置的是 `ANTHROPIC_BASE_URL` 和 `ANTHROPIC_AUTH_TOKEN`。

::: danger 不要用 OPENAI_ 开头的变量
Claude Code 不读取 `OPENAI_API_KEY`、`OPENAI_BASE_URL`。按 OpenAI 的方式配置会连不上，或者悄悄走回官方账号。
:::

打开 [API 密钥](https://codeplan.codes/keys)，在目标 Key 上点击 **使用密钥** → **Claude Code** 标签，复制弹窗生成的配置。两种写法任选一种：

![使用密钥弹窗：Claude Code 标签](/images/subapi/use-key-claude-code.png)

### 写法 A：写进 settings.json（推荐，永久生效）

编辑用户级配置文件，终端版和 VS Code 插件都会读取它：

| 系统 | 文件路径 |
| --- | --- |
| macOS / Linux | `~/.claude/settings.json` |
| Windows | `%USERPROFILE%\.claude\settings.json` |

不知道怎么打开这个目录，见 [找到配置文件](/config-files)。

```json
{
  "$schema": "https://json.schemastore.org/claude-code-settings.json",
  "env": {
    "ANTHROPIC_BASE_URL": "https://codeplan.codes",
    "ANTHROPIC_AUTH_TOKEN": "sk-你的密钥",
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1"
  }
}
```

::: warning 这个文件包含 API Key
`settings.json` 里有完整 Key，不要提交到代码仓库，也不要截图外发。
:::

### 写法 B：环境变量（只在当前终端生效）

::: code-group

```bash [macOS / Linux]
export ANTHROPIC_BASE_URL="https://codeplan.codes"
export ANTHROPIC_AUTH_TOKEN="sk-你的密钥"
export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
```

```powershell [PowerShell]
$env:ANTHROPIC_BASE_URL="https://codeplan.codes"
$env:ANTHROPIC_AUTH_TOKEN="sk-你的密钥"
$env:CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
```

```bat [CMD]
set ANTHROPIC_BASE_URL=https://codeplan.codes
set ANTHROPIC_AUTH_TOKEN=sk-你的密钥
set CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
```

:::

设置后在**同一个终端**里启动 `claude`。关掉终端就失效了，想长期使用请用写法 A。

::: tip 地址不带 /v1
`ANTHROPIC_BASE_URL` 填 `https://codeplan.codes`，**不要加 `/v1`**。Claude Code 会自己拼接 `/v1/messages`，加了反而报 404。
:::

## VS Code 里的 Claude Code 插件

插件读取的也是 `~/.claude/settings.json`。按上面的 **写法 A** 配好后，在 VS Code 里执行 **重新加载窗口**（<kbd>Ctrl</kbd>/<kbd>Cmd</kbd> + <kbd>Shift</kbd> + <kbd>P</kbd> → `Reload Window`）即可。


## 确认真的走了 CodePlan

以站内 [使用记录](https://codeplan.codes/usage) 为准：

- **有记录**：请求已经走 CodePlan。
- **没记录**：Claude Code 可能还在用官方账号登录。在终端运行 `claude auth status` 查看当前登录状态，需要时运行 `claude auth logout` 退出官方账号，确认配置后再重新打开。

## 常见现象

| 现象 | 优先检查 |
| --- | --- |
| 能回复，但使用记录里没有 | 仍在用官方 Claude 登录，见 [排查](/troubleshooting#能回复但没有使用记录) |
| `401` / `unauthorized` | `ANTHROPIC_AUTH_TOKEN` 是否完整；变量名是否写成了 `OPENAI_` 开头 |
| `404` / 路径错误 | `ANTHROPIC_BASE_URL` 末尾是否多加了 `/v1` |
| `model not found` | 当前模型不在 Key 分组内 |
| 改了配置不生效 | 是否完全关闭并重新打开了终端或 VS Code |

更多报错见 [常见报错排查](/troubleshooting)。
