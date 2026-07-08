# VS Code 插件接入

VS Code 里不同 AI 插件的界面不一样，但核心配置都差不多：选择 OpenAI 兼容供应商，填写 subapi 的 Base URL 和 API Key。

## 推荐流程

1. 先在站内创建 API Key，并确认 Key 绑定了可用分组。
2. 用 Chrome 打开 `https://subapi.loucer.cn/keys`，点击 **导入到 CCS**。
3. 在 CCSwitch 里点击 **启用**，直到 loucer_api 显示 **使用中**。
4. 在 CCSwitch 里点击 **刷新用量**，确认连通。
5. 打开或重启 VS Code。
6. 在 Codex / Claude Code / 其它 AI 插件里选择 OpenAI 兼容供应商。
7. 填写 Base URL 和 API Key。
8. 发起一次测试请求，并回站内 **使用记录** 验证。

![CCSwitch 确认 loucer_api 正在使用](/images/subapi/ccswitch-active-config.png)

## 两种接入方式怎么选

| 方式 | 适合场景 | 注意点 |
| --- | --- | --- |
| 先导入 CCSwitch，再打开 VS Code | 推荐给大多数用户 | VS Code 最好在 CCSwitch 启用后重新打开 |
| 插件里手动填 Base URL + API Key | 插件支持 OpenAI 兼容配置时 | 以站内使用记录验证是否真的走 subapi |

如果用户不熟悉配置，优先走 CCSwitch。它更像“一键切换当前 AI 接入地址”。

## 通用填写规则

| 字段 | 填写 |
| --- | --- |
| Provider | OpenAI Compatible / OpenAI 兼容 / Custom OpenAI |
| API Key | 站内创建的 sk-... |
| Base URL | https://subapi.loucer.cn/v1 |
| Model | 选择 Key 分组支持的模型 |

下图是通用 OpenAI 兼容配置示意。不同插件界面不一样，但要填的关键字段基本一致。

![VS Code 通用 OpenAI 兼容配置示意](/images/subapi/vscode-openai-compatible-settings.png)

如果插件会自动追加 /v1，则 Base URL 填：

~~~text
https://subapi.loucer.cn
~~~

::: warning 不要混用官方登录态
有些插件同时支持“官方账号登录”和“自定义 API”。如果 VS Code 能回复但 subapi 没有使用记录，通常说明插件还在使用官方账号或旧配置。
:::

## VS Code 里的 Codex 插件

配置时优先找这些字段：

| 字段 | 建议 |
| --- | --- |
| Provider | OpenAI Compatible / Custom OpenAI |
| API Key | sk-你的密钥 |
| Base URL | https://subapi.loucer.cn/v1 |
| Model | 选择你 Key 分组里支持的模型 |

![VS Code Codex 插件配置示意](/images/subapi/vscode-codex-settings.png)

配置完成后，重新加载 VS Code 窗口，然后让 Codex 回复一句：

~~~text
用一句话回复：Codex 连接成功。
~~~

测试成功后，回到 subapi 的 **使用记录** 看是否有新记录。

## VS Code 里的 Claude Code 插件

如果插件支持 OpenAI 兼容供应商，填写方式和 Codex 类似：

| 字段 | 建议 |
| --- | --- |
| Provider | OpenAI Compatible / Custom OpenAI / OpenAI 兼容 |
| API Key | sk-你的密钥 |
| Base URL | https://subapi.loucer.cn/v1 |
| Model | 选择你 Key 分组里支持的模型 |

![VS Code Claude Code 插件配置示意](/images/subapi/vscode-claude-code-settings.png)

如果插件只支持 Anthropic 官方账号或官方 Key，不要硬填 OpenAI 地址。优先使用 CCSwitch 当前配置，或按插件支持的兼容模式配置。

配置完成后，重新加载 VS Code 窗口，然后让 Claude Code 回复一句：

~~~text
用一句话回复：Claude Code 连接成功。
~~~

## 测试连通

配置后做三步检查：

1. VS Code 插件能正常回复。
2. 站内 **使用记录** 出现记录。
3. 记录里的模型、Key、费用符合预期。

如果 VS Code 能回复但站内没有使用记录，说明请求没有走 subapi，优先检查：

- 插件是否仍在使用官方登录模式。
- Provider 是否选成 OpenAI 兼容。
- Base URL 是否填错。
- 是否需要重启 VS Code 或重新加载窗口。

## 常见问题

| 现象 | 处理方式 |
| --- | --- |
| 插件找不到 OpenAI Compatible | 看插件是否支持自定义 Provider；不支持就只能走它支持的官方渠道 |
| Base URL 填 `/v1` 后报路径错误 | 尝试改成 `https://subapi.loucer.cn`，该插件可能会自动拼接 `/v1` |
| 能回复但站内没记录 | 切换 Provider，退出官方账号模式，重新加载 VS Code |
| 报余额不足 | 检查余额、订阅和 Key 绑定分组 |
