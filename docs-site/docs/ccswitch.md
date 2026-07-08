# CCSwitch 接入

CCSwitch 用来管理和切换客户端 API 配置。导入成功后，还要确认对应配置已经启用。

## 下载并安装

优先使用官方入口；如果官方下载不稳定，再使用网盘备用链接。

![CCSwitch 下载入口](/images/subapi/12-ccswitch-download-links.png)

| 系统 | 推荐选择 | 说明 |
| --- | --- | --- |
| Windows | `.msi` 安装包 | 推荐，安装后更容易自动注册 `ccswitch://` 协议 |
| Windows | `Portable.zip` | 便携版，解压即用，但可能需要手动处理协议注册 |
| macOS | `.dmg` | 推荐，拖入 Applications 后使用 |
| Linux | GitHub Releases | 按发行版和架构选择，需要能访问 GitHub |

下载地址：

- 官方网站：`https://ccswitch.io`
- Windows 备用网盘：`https://wwarq.lanzn.com/ie5Si3ss9ckb`
- macOS 备用网盘：`https://wwarq.lanzn.com/iGvms3ss9fgf`
- Linux / GitHub Releases：`https://github.com/farion1231/cc-switch/releases`

![CCSwitch 视频说明](/images/subapi/13-ccswitch-video-guide.png)

## 导入并启用

::: warning 重要
导入成功不等于已经启用。最终要看 CCSwitch 里对应条目是否显示 **使用中**。
:::

`导入到 CCS` 依赖浏览器识别 `ccswitch://` 协议。建议用 **Google Chrome** 打开：

```text
https://subapi.loucer.cn/keys
```

找到要使用的 Key，点击右侧 **导入到 CCS**。

![Chrome 导入 CCSwitch](/images/subapi/10-chrome-import-to-ccswitch.png)

如果 Chrome 弹出「是否打开 CC Switch」提示，选择允许/打开。正常情况下，CCSwitch 会被拉起，并自动新增一条 `loucer_api` 配置。

## 确认正在使用

导入成功后，CCSwitch 里会出现 `loucer_api`，地址应显示为：

```text
https://subapi.loucer.cn
```

![CCSwitch 确认 loucer_api 正在使用](/images/subapi/ccswitch-active-config.png)

如果条目显示 **使用中**，说明已经启动成功。如果条目显示 **启用**，点击 **启用**，直到该配置显示 **使用中**。

启用后点 **刷新用量**，能看到余额/用量就说明配置正常。

## 手动导入

如果一键导入始终失败，就在 CCSwitch 里手动新增配置：

| 字段 | 填写内容 |
| --- | --- |
| 名称 | 自定义，例如 `subapi` 或 `loucer_api` |
| Base URL | `https://subapi.loucer.cn` |
| API Key | 粘贴站内生成的 `sk-...` |
| 类型 | OpenAI Compatible / OpenAI 兼容 |

保存后回到列表，点击 **启用**，直到该配置显示 **使用中**。
