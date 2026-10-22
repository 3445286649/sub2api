# CCSwitch 接入（推荐）

CCSwitch 用来管理和切换 AI 客户端的接入配置。在 CodePlan 点一下 **导入到 CCS**，就能把地址和 Key 一次性配好，不用手动改配置文件。

::: info 一次导入配一类客户端
导入的是哪一类客户端，由这个 Key 的**分组**决定：OpenAI 分组的 Key 导入到 **Codex**，Anthropic / Claude 分组的 Key 导入到 **Claude Code**，Gemini 分组导入到 **Gemini CLI**。对应的 VS Code 插件和命令行共用同一份配置，所以也能直接用。

Codex 和 Claude Code 都要用的话，建两个不同分组的 Key，各导入一次。
:::

**这是最省事的接入方式，第一次使用建议走这条路。** 其他客户端的教程都会引用本页的步骤。

<StepFlow :steps="['下载安装|#_1-下载并安装', '一键导入|#_2-一键导入', '启用到使用中|#_3-启用并确认「使用中」', '刷新用量|#_4-刷新用量-验证连通', '重启客户端|#_5-重启客户端']" />

## 1. 下载并安装

选择你的电脑系统。优先用官方下载；官方打不开时再用网盘备用。

<OsTabs>
<template #windows>

<DownloadCard name="CCSwitch 官方下载" badge="CC" href="https://ccswitch.io/zh/download" hint="选择 .msi 安装包，安装后会自动注册 ccswitch:// 协议" />
<DownloadCard name="Windows 备用网盘" badge="云" href="https://wwarq.lanzn.com/ie5Si3ss9ckb" hint="官方下载慢或打不开时使用" />

不推荐 `Portable.zip` 便携版：它可能没有注册 `ccswitch://` 协议，**导入到 CCS** 时会提示「未安装」。

</template>
<template #macos>

<DownloadCard name="CCSwitch 官方下载" badge="CC" href="https://ccswitch.io/zh/download" hint="选择 .dmg 安装包，拖入「应用程序」后打开一次" />
<DownloadCard name="macOS 备用网盘" badge="云" href="https://wwarq.lanzn.com/iGvms3ss9fgf" hint="官方下载慢或打不开时使用" />

第一次打开如果提示「无法验证开发者」，到 **系统设置 → 隐私与安全性** 里点 **仍要打开**。

</template>
<template #linux>

<DownloadCard name="GitHub Releases" badge="GH" href="https://github.com/farion1231/cc-switch/releases/latest" hint="按发行版和架构选择安装包，需要能访问 GitHub" />

</template>
</OsTabs>

::: details 下载页长什么样 / 视频说明
![CCSwitch 下载入口](/images/subapi/12-ccswitch-download-links.png)

![CCSwitch 视频说明](/images/subapi/13-ccswitch-video-guide.png)
:::

装好后**先打开一次 CCSwitch**，让系统记住 `ccswitch://` 协议，再进行下一步。

## 2. 一键导入

**导入到 CCS** 依赖浏览器识别 `ccswitch://` 协议，建议用 **Google Chrome**：

1. 用 Chrome 打开 [API 密钥](https://codeplan.codes/keys)。
2. 找到要用的 Key，点击操作栏里的 **导入到 CCS**。
3. Chrome 弹出「是否打开 CC Switch」时，选择 **打开**。

![API 密钥列表里的「导入到 CCS」按钮](/images/subapi/04-api-keys-page.png)

正常情况下 CCSwitch 会被拉起，并自动新增一条配置，地址是 `https://codeplan.codes`。配置名就是站点名，一般显示为 **CodePlan**。

::: details 点了没反应，或者提示「CC-Switch 未安装」
![导入到 CCS 提示未安装或协议未注册](/images/subapi/06-ccswitch-import-action.png)

这个提示说明浏览器没找到 CCSwitch：

- 确认 CCSwitch 已经安装，并且**打开过一次**。
- Windows 用 `.msi` 安装包重新安装，它会注册 `ccswitch://` 协议。
- 换 Google Chrome 再试。
- 还是不行就 [手动添加](#一键导入失败时-手动添加)。
:::

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

截图里的配置名是老名字 `loucer_api`，新导入的会叫 **CodePlan**，看 **使用中** 标记即可。

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
| 地址 | `https://codeplan.codes` |
| API Key | 站内创建的 `sk-...` |

保存后回到列表点击 **启用**，直到显示 **使用中**。

## 下一步

- [Codex 接入](/codex)
- [Claude Code 接入](/claude-code)
- [VS Code 插件接入](/vscode)
- 遇到问题：[常见报错排查](/troubleshooting)
