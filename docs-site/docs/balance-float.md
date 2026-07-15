# Sub2API 余额悬浮窗使用教程

Sub2API Balance Float 是一款桌面余额悬浮窗。连接时只需输入自己的 Sub2API API Key，无需填写接口地址。

它常驻桌面，以悬浮球显示可用余额；单击后可查看今日消费与请求次数、本月消费、近 7 日消费，并可快速打开 Sub2API 主站。

::: tip 适用版本
本文适用于 **v0.1.0**，支持 Windows 10/11、Intel Mac 与 Apple Silicon。
:::

## 下载 v0.1.0

| 平台 | 适用场景 | 下载 |
| --- | --- | --- |
| Windows NSIS | 普通用户推荐 | [下载安装版 EXE](/downloads/sub2api-balance-float/v0.1.0/Sub2API-Balance-Float-Setup-v0.1.0.exe) |
| Windows MSI | 企业或批量部署 | <a href="/downloads/sub2api-balance-float/v0.1.0/Sub2API-Balance-Float-Setup-v0.1.0.msi" download>下载安装版 MSI</a> |
| Windows 便携版 | 无需安装，直接运行 | [下载便携版 EXE](/downloads/sub2api-balance-float/v0.1.0/Sub2API-Balance-Float-Portable-v0.1.0.exe) |
| macOS Universal | Intel 与 Apple Silicon | <a href="/downloads/sub2api-balance-float/v0.1.0/Sub2API-Balance-Float-macOS-Universal-v0.1.0.dmg" download>下载 DMG</a> |

[下载 SHA-256 校验清单](/downloads/sub2api-balance-float/v0.1.0/SHA256SUMS.csv)

::: warning 安装包签名说明
当前 v0.1.0 的 Windows 与 macOS 安装包尚未做代码签名。请只从本页下载并先核对 SHA-256；确认一致后，再处理 Windows SmartScreen 或 macOS 安全提示。
:::

## 软件显示什么

- **可用余额（USD）**
- **今日消费与请求次数**
- **本月消费**
- **近 7 日消费**
- 当前 API Key Profile 与同步状态
- **主站直达**：点击面板右下角的 **主站** 按钮，在默认浏览器打开 Sub2API

本工具展示的是 Sub2API 余额与消费统计，**不是 5 小时限额或周限额工具**。

## 使用前准备

1. 已有可登录的 Sub2API 账户。
2. 账户中有一个已启用的 API Key。
3. 没有 Key 时，先打开 [API 密钥页面](https://subapi.loucer.cn/keys) 创建。

::: danger 保护 API Key
不要把完整 API Key 发到聊天、工单或截图中。
:::

## 一、安装与首次启动

### Windows

- 普通用户运行 `Sub2API-Balance-Float-Setup-v0.1.0.exe`，按向导安装。
- 企业或批量部署可使用 MSI。
- 便携版无需安装，请放在固定目录，不要放进会自动清理的临时目录。

如果 SmartScreen 拦截，请先核对下载来源和 SHA-256。确认一致后，选择 **更多信息 → 仍要运行**。

### macOS

1. 打开 `Sub2API-Balance-Float-macOS-Universal-v0.1.0.dmg`。
2. 将应用拖入 **Applications（应用程序）**。
3. 首次打开时，在 Finder 的「应用程序」中右键应用并选择 **打开**。
4. 如果系统仍拦截，到 **系统设置 → 隐私与安全性** 核对应用来源后选择打开。

![安装与首次启动](/images/balance-float/01-install-and-first-launch.png)

## 二、连接第一个 API Key

1. 启动应用。首次启动会进入 **API Keys** 页面。
2. 点击右上角 **+**。
3. 输入便于识别的 Profile 名称，例如「主账号」或「工作 Key」。
4. 粘贴完整 API Key，确认前后没有空格、换行或多余字符。
5. 点击 **验证并保存**，等待验证完成。
6. 验证成功后会自动进入余额面板，并将该 Profile 设为当前账号。

API Key 只发送到预置的 Sub2API 余额查询端点。完整 Key 保存到 Windows Credential Manager 或 macOS Keychain，不写入普通配置文件或日志，界面也不会回显完整 Key。

![连接第一个 API Key](/images/balance-float/02-connect-api-key.png)

## 三、日常操作

1. **单击展开**：鼠标经过悬浮球只恢复清晰度，不会展开；单击后才打开余额面板。
2. **移开收起**：鼠标移出面板后，面板自动淡出并恢复为悬浮球。
3. **拖动位置**：按住悬浮球拖动；靠近屏幕边缘会自动吸附。
4. **刷新数据**：查看顶端同步状态，必要时点击刷新按钮。
5. **保持展开或置顶**：使用右上角控制按钮切换对应状态。
6. **打开主站**：点击右下角 **主站** 按钮。

余额面板显示当前选中 Profile 的数据。切换 Profile 后，请等待验证和刷新完成再核对余额。

![日常操作](/images/balance-float/03-daily-use.png)

## 四、管理多个 Key

从余额面板左上角的当前 Profile 名称进入 **API Keys**：

- **添加**：点击右上角 **+**，输入名称和新的 API Key。
- **切换**：点击其他 Profile；验证通过后，该 Profile 成为当前账号。
- **重绑**：Key 失效、轮换或更换时，只更新当前 Profile 的凭据。
- **重命名或删除**：从 Profile 的更多菜单操作。

每次操作只影响选中的 Profile。遇到身份错误时优先使用 **重绑**，不要反复创建重复 Profile。

![多 Key 管理](/images/balance-float/04-multi-key.png)

## 五、异常状态与处理

| 界面提示 | 常见原因 | 处理方式 |
| --- | --- | --- |
| API Key 无效 | Key 输入错误、已撤销或格式不完整 | 进入 API Keys，选择对应 Profile 后重新绑定 |
| 访问被拒绝 | 账号权限、IP 白名单或接口策略限制 | 登录主站检查账号权限与 IP 设置 |
| Key 已禁用 | 当前 Key 已在服务端停用 | 启用原 Key，或创建新 Key 后重绑 |
| 数据已过期 | 网络暂时不可用，应用保留最近一次可信快照 | 检查网络并手动刷新 |
| 服务暂不可用 | 接口超时、响应变化或服务异常 | 稍后重试，并确认主站可以正常访问 |

应用不会在接口异常时伪造余额。无法确认的数据会明确显示为过期、不可用或需要重新连接。

![异常状态与处理方式](/images/balance-float/05-troubleshooting.png)

## 六、校验安装包

### Windows PowerShell

```powershell
Get-FileHash -Algorithm SHA256 "下载的安装包路径"
```

### macOS Terminal

```bash
shasum -a 256 "/下载的安装包路径"
```

v0.1.0 官方文件：

| 文件 | SHA-256 |
| --- | --- |
| Windows NSIS | `0346CBC7963C3AB545BD7D7086F95E7848E8EE37B217A556110BE97100866980` |
| Windows MSI | `3D68F2CFCD87553239EE64A5E8E8E619B77D3B6182E44C8AAFF1FE4A26CD4B` |
| Windows 便携版 | `4DA7A70ACF66DB92FFD2F055E994FAAF3931DB0B6CC899CF81C1EAABC3A6C523` |
| macOS Universal | `821FB098BF418C19C028D7B7B8C5E5765965AF3EE7356974339178880E5E88D0` |

## 常见问题

### 鼠标悬停为什么不展开？

这是正常设计。悬停只恢复悬浮球清晰度，**单击**才展开面板，减少桌面误触。

### 为什么没有 5 小时或周限额？

本工具适配 Sub2API 余额查询，只展示可用余额和消费统计。

### 更换 Key 后需要新建 Profile 吗？

不需要。对原 Profile 使用 **重绑**，可以保留名称并替换凭据。

### 卸载前如何清除已保存的 Key？

先在 API Keys 页面删除不再需要的 Profile，再卸载应用或删除便携版文件。需要确认凭据已完全清除时，同时检查 Windows Credential Manager 或 macOS Keychain。

## 相关入口

- [Sub2API 主站](https://subapi.loucer.cn)
- [API 密钥页面](https://subapi.loucer.cn/keys)
- [API Key 创建与安全](/api-key)
- [计费、余额与使用记录](/billing)
- [常见报错排查](/troubleshooting)
