# Codex 接入

这一页讲的是让 Codex 通过 subapi 调用模型。推荐先用 CCSwitch 导入并启用 loucer_api，确认连通后再打开 Codex。

## 最短路径

按这个顺序做，通常 3 分钟内可以跑通：

1. 在 subapi 创建 API Key，并绑定可用分组。
2. 用 Chrome 打开 API Key 页面，点击 **导入到 CCS**。
3. 在 CCSwitch 里点击 loucer_api 的 **启用**，确认显示 **使用中**。
4. 点击 CCSwitch 的 **刷新用量** 测试连通。
5. 重新打开 Codex，发送一句测试消息。
6. 回到 subapi 的 **使用记录**，确认有新请求。

## 先完成前置步骤

在配置 Codex 前，先确认这三件事：

1. 账户有余额或有效订阅。
2. 已经创建 API Key，并且 Key 绑定了可用分组。
3. CCSwitch 里 loucer_api 已经显示 **使用中**。

如果还没有 Key，先看 [API Key 创建与安全](/api-key)。

![API 密钥页面](/images/subapi/04-api-keys-page.png)

## 1. 导入到 CCSwitch

用 Google Chrome 打开：

~~~text
https://subapi.loucer.cn/keys
~~~

找到要使用的 Key，点击 **导入到 CCS**。

![Chrome 导入 CCSwitch](/images/subapi/10-chrome-import-to-ccswitch.png)

Chrome 弹出打开应用时选择允许。正常情况下，CCSwitch 会自动新增一条 loucer_api 配置。

## 2. 启用 loucer_api

回到 CCSwitch，确认 loucer_api 的地址是：

~~~text
https://subapi.loucer.cn
~~~

然后点击 **启用**，直到这条配置显示 **使用中**。

![CCSwitch 确认 loucer_api 正在使用](/images/subapi/ccswitch-active-config.png)

::: warning 重要
导入成功不等于已经启用。只有显示 **使用中** 的配置才是当前生效配置。
:::

## 3. 测试 CCSwitch 连通

在 CCSwitch 里点击 **刷新用量**。

如果能看到余额或用量，说明 Key、Base URL、CCSwitch 配置基本正常。

如果刷新失败，先检查：

- loucer_api 是否显示 **使用中**。
- API Key 有没有复制完整。
- Key 是否绑定了可用分组。
- 账户余额或订阅是否有效。

## 4. 打开 Codex

CCSwitch 配置确认正常后，再打开 Codex。

建议顺序：

1. 先关闭已经打开的 Codex 终端或 VS Code 窗口。
2. 确认 CCSwitch 中 loucer_api 是 **使用中**。
3. 重新打开终端或 VS Code。
4. 启动 Codex。
5. 发送一句简单测试，例如：

~~~text
用一句话回复：你已经连接成功。
~~~

如果能正常对话，并且站内 **使用记录** 出现调用记录，说明 Codex 已经走 subapi。

## 5. 如果 Codex 没有读取 CCSwitch

优先确认 Codex 是在 CCSwitch 启用后重新打开的。很多桌面客户端和 VS Code 扩展只会在启动时读取一次环境或代理配置。

如果仍然不生效，再使用手动配置。

## 6. 手动配置方式

如果你的 Codex 不读取 CCSwitch 当前配置，可以改用手动环境变量。

macOS / Linux：

~~~bash
export OPENAI_API_KEY="sk-你的密钥"
export OPENAI_BASE_URL="https://subapi.loucer.cn/v1"
~~~

Windows PowerShell 建议在系统环境变量里设置：

~~~powershell
setx OPENAI_API_KEY "sk-你的密钥"
setx OPENAI_BASE_URL "https://subapi.loucer.cn/v1"
~~~

设置后重新打开终端，再启动 Codex。

::: tip 什么时候填 /v1
如果配置项叫 **Base URL** 或 **OpenAI Base URL**，通常填 `https://subapi.loucer.cn/v1`。

如果插件说明会自动拼接 `/v1`，就填 `https://subapi.loucer.cn`。
:::

## 7. VS Code 里的 Codex 插件

如果你是在 VS Code 里用 Codex 插件，按这个规则填：

| 字段 | 填写 |
| --- | --- |
| Provider | OpenAI Compatible / Custom OpenAI / OpenAI 兼容 |
| API Key | 站内创建的 sk-... |
| Base URL | https://subapi.loucer.cn/v1 |
| Model | 选择 Key 绑定分组支持的模型 |

下图是 VS Code Codex 插件的配置示意。不同插件字段名称可能略有差异，核心就是 Provider、API Key、Base URL、Model 这四项。

![VS Code Codex 插件配置示意](/images/subapi/vscode-codex-settings.png)

如果插件会自动追加 /v1，Base URL 改填：

~~~text
https://subapi.loucer.cn
~~~

配置完成后重启 VS Code 或重新加载窗口，再发起一次测试请求。

## 8. 怎么确认真的走了 subapi

最终以站内 **使用记录** 为准：

- 有记录：说明请求打到了 subapi。
- 没记录：说明 Codex 没走 subapi，优先检查 Base URL、Provider、Key 和当前是否仍在官方账号登录模式。

## 常见现象

| 现象 | 优先检查 |
| --- | --- |
| Codex 能回复，但站内没有使用记录 | Codex 仍在走官方账号或其它供应商配置 |
| CCSwitch 刷新用量失败 | loucer_api 是否启用、Key 是否完整、Key 是否绑定分组 |
| 报 401 / invalid api key | API Key 是否复制完整，是否多了空格 |
| 报 model not found | 当前 Key 绑定分组是否支持你选择的模型 |

常见错误可以看 [常见报错排查](/troubleshooting)。
