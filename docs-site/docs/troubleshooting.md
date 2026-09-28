# 常见报错排查

先看报错里的**状态码**或关键词，在下表找到对应行。表格里的原因都来自 CodePlan 后端的实际返回。

## 按报错查

| 报错 | 意思 | 处理方式 |
| --- | --- | --- |
| `401` · `Invalid API key` | Key 错误、不完整、被删除或被禁用 | 重新复制 Key，检查前后有没有空格；必要时新建一个 |
| `401` · `Authorization required` | 请求里没带 Key | 检查 Key 是否填进了正确的字段，见各客户端教程 |
| `403` · 余额不足 | 账户余额用完了 | 到 [充值/订阅](https://codeplan.codes/purchase) 充值，到账后立即恢复 |
| `403` · 订阅已到期 | 订阅过期 | 续费，或把 Key 换到余额分组 |
| `403` · 本周期额度已用完 | 订阅的周期额度用完了 | 等周期重置，或把 Key 换到余额分组 |
| `403` · `restricted to Claude Code clients` | 这个分组只允许 Claude Code 使用 | 用 Claude Code 访问，或换一个 Key 分组 |
| `404` · `model_not_found` | 当前分组里没有这个模型 | 换成分组支持的模型名，或调整 Key 分组，见 [模型与分组](/models-and-groups) |
| `404` · 路径错误 | 地址末尾的 `/v1` 加多了或加少了 | 对照 [接入地址怎么填](/api-key#接入地址怎么填) 改正 |
| `429` · `Concurrency limit` / `Too many pending requests` | 同时发出的请求太多 | 稍等几秒重试；多数客户端会自动退避 |
| `429` · `rate_limit_exceeded` | 触发了日 / 周 / 月用量上限 | 等窗口重置，报错里通常带有重置时间 |
| `502` / `524` / 超时 | 上游暂时繁忙或响应慢 | 稍后重试；长时间持续请联系 [售后](/support) |
| `503` · `No available accounts` | 分组里可用账号暂时耗尽 | 稍后重试；持续出现请联系 [售后](/support) |

::: tip 为什么余额不足是 403
CodePlan 用 `403` + `billing_error` 表示计费相关的拒绝（余额、订阅、额度），不是 `402`。看到 `403` 时，先读报错里的中文提示。
:::

## 能回复但没有使用记录

**这是最常见、也最容易被忽略的情况。** 客户端能正常对话，但站内 [使用记录](https://codeplan.codes/usage) 里没有这次调用，说明请求**根本没有到达 CodePlan**，而是走了别的地方。

按顺序检查：

1. **是不是还在用官方账号登录？** 很多客户端同时支持官方登录和自定义接入，官方登录优先。
   - Claude Code：运行 `claude auth status` 查看，需要时 `claude auth logout`。
   - Codex / VS Code 插件：退出官方账号登录。
2. **CCSwitch 里生效的是不是 CodePlan？** 只有显示 **使用中** 的那条才生效，见 [CCSwitch 接入](/ccswitch#_3-启用并确认「使用中」)。
3. **变量名对不对？** Claude Code 读的是 `ANTHROPIC_BASE_URL`，不是 `OPENAI_BASE_URL`。
4. **重启了吗？** 见下一节。

## 改了配置不生效

多数客户端只在**启动时**读一次配置。改完配置或在 CCSwitch 切换之后：

- 终端客户端：关掉这个终端窗口，新开一个再启动。
- VS Code 插件：执行 **重新加载窗口**（<kbd>Ctrl</kbd>/<kbd>Cmd</kbd> + <kbd>Shift</kbd> + <kbd>P</kbd> → `Reload Window`）。
- 桌面端（如 Codex 桌面版）：完全退出（包括托盘图标）再重新打开。

## CCSwitch 导入失败

| 现象 | 处理方式 |
| --- | --- |
| 点 **导入到 CCS** 没反应 | 用 Google Chrome 打开站点；确认 CCSwitch 已安装并打开过一次 |
| 提示「未安装 CC Switch」 | Windows 用 `.msi` 安装包重装，它会注册 `ccswitch://` 协议 |
| 导入成功但不生效 | CCSwitch 里点击 **启用**，直到 **CodePlan** 显示 **使用中** |
| 一直导入失败 | 在 CCSwitch 里 [手动添加](/ccswitch#一键导入失败时-手动添加) |

## 建议的排查顺序

1. 站内余额或订阅是否正常。
2. Key 是否存在、已启用、绑定了正确的分组。
3. 客户端里的地址和 Key 是否正确（注意 `/v1`）。
4. 客户端是否已经重启。
5. 发一次测试请求，看 [使用记录](https://codeplan.codes/usage) 有没有出现。

还是解决不了，带上报错截图和 Key 名称联系 [售后](/support)。
