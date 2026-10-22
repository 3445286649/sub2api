# 找到配置文件

手动配置或排查问题时，要打开客户端的配置目录。本页列出 Codex 和 Claude Code 配置文件的位置，以及在 Windows 和 macOS 上最快的打开方法。

::: tip 用 CCSwitch 的话一般不用碰这些文件
[CCSwitch 一键导入](/ccswitch) 会自动写好配置文件。只有手动配置、或者想确认 CCSwitch 写进去的内容时，才需要按本页打开。
:::

## 文件一览

| 客户端 | 文件 | 作用 |
| --- | --- | --- |
| Codex | `~/.codex/config.toml` | 接入地址、协议、模型 |
| Codex | `~/.codex/auth.json` | API Key |
| Codex | `~/.codex/codex-models.json` | 模型目录，只有选「本地文件」模式时才需要，见 [Codex 模型目录](/codex#模型目录) |
| Claude Code | `~/.claude/settings.json` | 接入地址和 API Key（`env` 字段） |

`~` 是你的用户目录。Windows 上 `~/.codex` 就是 `%USERPROFILE%\.codex`，比如 `C:\Users\你的用户名\.codex`。

## 打开配置目录

<OsTabs>
<template #windows>

**方法一：运行窗口（最快）**

1. 按 <kbd>Win</kbd> + <kbd>R</kbd>。
2. 粘贴下面的路径，回车：

```text
%USERPROFILE%\.codex
```

Claude Code 的目录把 `.codex` 换成 `.claude` 即可。

**方法二：PowerShell 直接打开文件**

::: code-group

```powershell [Codex]
notepad "$HOME\.codex\config.toml"
```

```powershell [Claude Code]
notepad "$HOME\.claude\settings.json"
```

:::

**目录不存在时**先创建：

```powershell
New-Item -ItemType Directory -Force "$HOME\.codex"
```

::: info 保存时注意扩展名
记事本「另存为」时把保存类型选成 **所有文件**，文件名写 `config.toml`。否则会存成 `config.toml.txt`，客户端读不到。可以在资源管理器的 **查看 → 显示 → 文件扩展名** 里确认。
:::

</template>
<template #macos>

`.codex`、`.claude` 是以 `.` 开头的隐藏目录，Finder 默认不显示，这是正常现象。

**方法一：Finder「前往文件夹」（最快）**

1. 在 Finder 中按 <kbd>⌘</kbd> + <kbd>⇧</kbd> + <kbd>G</kbd>。
2. 输入下面的路径，回车：

```text
~/.codex
```

Claude Code 的目录把 `.codex` 换成 `.claude` 即可。

**方法二：终端**

按 <kbd>⌘</kbd> + <kbd>Space</kbd> 搜索「终端」，然后运行：

::: code-group

```bash [打开目录]
open ~/.codex
```

```bash [编辑 Codex 配置]
open -e ~/.codex/config.toml
```

```bash [编辑 Claude Code 配置]
open -e ~/.claude/settings.json
```

:::

**目录或文件不存在时**先创建：

```bash
mkdir -p ~/.codex && touch ~/.codex/config.toml ~/.codex/auth.json
```

想在 Finder 里一直显示隐藏文件，按 <kbd>⌘</kbd> + <kbd>⇧</kbd> + <kbd>.</kbd>，再按一次恢复。

</template>
<template #linux>

```bash
# 打开目录（不同桌面环境命令可能不同）
xdg-open ~/.codex

# 直接编辑
nano ~/.codex/config.toml
nano ~/.claude/settings.json

# 目录不存在时
mkdir -p ~/.codex ~/.claude
```

</template>
</OsTabs>

## 还是找不到

- **先启动一次客户端。** Codex、Claude Code 第一次运行时会自动创建配置目录。
- **或者先用 CCSwitch 导入一次。** 导入后文件就会出现。
- **确认用户目录对不对。** 用管理员身份或另一个账户运行时，读的是那个账户的用户目录。

::: warning 这些文件里有 API Key
`auth.json` 和 `settings.json` 含完整 Key，不要发给别人、截图外发，也不要提交到代码仓库。怀疑泄露时到 [API 密钥](https://codeplan.codes/keys) 禁用旧 Key 再新建。
:::

## 改完记得重启

客户端只在启动时读一次配置。保存后完全退出客户端再打开，VS Code 插件执行 **重新加载窗口**。详见 [改了配置不生效](/troubleshooting#改了配置不生效)。
