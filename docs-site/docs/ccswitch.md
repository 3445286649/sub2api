# CCSwitch 接入（推荐）

CCSwitch 用来管理和切换 AI 客户端的接入配置。在 CodePlan 点一下 **导入到 CCS**，就能把地址和 Key 一次性配好，Codex、Claude Code、VS Code 插件都能直接用。

**这是最省事的接入方式，第一次使用建议走这条路。** 其他客户端的教程都会引用本页的步骤。

## 1. 下载并安装

优先使用官方入口；官方下载不稳定时再用网盘备用链接。

![CCSwitch 下载入口](/images/subapi/12-ccswitch-download-links.png)

| 系统 | 推荐选择 | 说明 |
| --- | --- | --- |
| Windows | `.msi` 安装包 | 推荐，安装后会自动注册 `ccswitch://` 协议 |
| Windows | `Portable.zip` | 便携版，解压即用，但可能要手动处理协议注册 |
| macOS | `.dmg` | 推荐，拖入「应用程序」后使用 |
| Linux | GitHub Releases | 按发行版和架构选择，需要能访问 GitHub |

- 官方网站：<https://ccswitch.io>
- Windows 备用网盘：<https://wwarq.lanzn.com/ie5Si3ss9ckb>
- macOS 备用网盘：<https://wwarq.lanzn.com/iGvms3ss9fgf>
- GitHub Releases：<https://github.com/farion1231/cc-switch/releases>

![CCSwitch 视频说明](/images/subapi/13-ccswitch-video-guide.png)

## 2. 一键导入

**导入到 CCS** 依赖浏览器识别 `ccswitch://` 协议，建议用 **Google Chrome**：

1. 用 Chrome 打开 [API 密钥](https://codeplan.codes/keys)。
2. 找到要用的 Key，点击右侧 **导入到 CCS**。
3. Chrome 弹出「是否打开 CC Switch」时，选择 **打开**。

![Chrome 导入 CCSwitch](/images/subapi/10-chrome-import-to-ccswitch.png)

正常情况下 CCSwitch 会被拉起，并自动新增一条名为 **CodePlan** 的配置，地址是 `https://subapi.loucer.cn`。

::: info 老用户注意
以前导入的配置可能叫 `loucer_api`，它和 **CodePlan** 指向同一个站点，继续用没有问题。不想留两条的话，删掉其中一条即可。
:::

## 3. 启用并确认「使用中」

::: warning 最容易漏的一步
**导入成功不等于已经生效。** CCSwitch 可以同时保存很多配置，真正生效的只有显示 **使用中** 的那一条。
:::

1. 在 CCSwitch 列表里找到 **CodePlan**。
2. 如果显示 **启用**，点击它，直到状态变成 **使用中**。

![CCSwitch 确认配置正在使用](/images/subapi/ccswitch-active-config.png)

## 4. 刷新用量，验证连通

点击 **刷新用量**。能看到余额或用量，说明 Key、地址和 CCSwitch 配置都正常。

刷新失败时依次检查：

- **CodePlan** 这条是不是 **使用中**。
- Key 所在分组是否可用，见 [模型与分组怎么选](/models-and-groups)。
- 账户余额或订阅是否有效。

## 5. 重启客户端

::: tip 为什么要重启
多数客户端（Codex、Claude Code、VS Code 插件）只在**启动时**读一次配置。在 CCSwitch 切换之后，已经打开的窗口还在用旧配置。
:::

关闭已经打开的终端或 VS Code 窗口，再重新打开客户端，发一句测试消息：

```text
用一句话回复：连接测试成功。
```

然后到站内 [使用记录](https://codeplan.codes/usage) 看是否出现这次调用。**有记录才说明真的走了 CodePlan。**

## 一键导入失败时：手动添加

一键导入始终不成功时，在 CCSwitch 里手动新增一条配置：

| 字段 | 填写 |
| --- | --- |
| 名称 | `CodePlan` |
| 地址 | `https://subapi.loucer.cn` |
| API Key | 站内创建的 `sk-...` |

保存后回到列表点击 **启用**，直到显示 **使用中**。

## 下一步

- [Codex 接入](/codex)
- [Claude Code 接入](/claude-code)
- [VS Code 插件接入](/vscode)
- 遇到问题：[常见报错排查](/troubleshooting)
