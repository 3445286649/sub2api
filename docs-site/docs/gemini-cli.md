# Gemini CLI 接入

**不要手写或猜测 Gemini 的地址。** 不同分组的路径不一样，站内会根据 Key 的分组生成正确的配置。

::: info 需要 Gemini 或 Antigravity 分组
点击 **使用密钥** 能看到 **Gemini CLI** 标签，就说明分组支持，见 [模型与分组怎么选](/models-and-groups)。
:::

## 配置步骤

1. 打开 [API 密钥](https://codeplan.codes/keys)。
2. 找到绑定了 Gemini 或 Antigravity 分组的 Key，点击 **使用密钥**。
3. 选择 **Gemini CLI** 标签，再选你的系统。
4. 复制生成的 `GOOGLE_GEMINI_BASE_URL`、`GEMINI_API_KEY` 和 `GEMINI_MODEL`。
5. 完全退出终端，重新打开后再启动 Gemini CLI。

配置结构如下，**具体的值以弹窗生成为准**：

```bash
export GOOGLE_GEMINI_BASE_URL="以站内生成值为准"
export GEMINI_API_KEY="sk-你的密钥"
export GEMINI_MODEL="以站内生成模型为准"
```

::: warning 不要照搬其它分组的地址
Gemini 分组和 Antigravity 分组的地址不同（Antigravity 带 `/antigravity` 路径），必须复制当前这个 Key 生成的内容。
:::

## 验证

1. Gemini CLI 能正常返回测试结果。
2. [使用记录](https://codeplan.codes/usage) 出现这次调用。
3. 记录里的模型、Key 和费用符合预期。

## 常见问题

| 现象 | 处理方式 |
| --- | --- |
| `401` | Key 错误或被禁用，重新复制 |
| `404` · `model_not_found` | 模型名和分组不匹配，从弹窗重新复制 `GEMINI_MODEL` |
| 能回复但没有使用记录 | 仍在用 Google 官方登录或旧环境变量，见 [排查](/troubleshooting#能回复但没有使用记录) |
| 修改后不生效 | 完全关闭旧终端，再重新打开 |

更多报错见 [常见报错排查](/troubleshooting)。
