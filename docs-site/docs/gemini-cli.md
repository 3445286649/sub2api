# Gemini CLI 接入

不要手写或猜测 Gemini 路径。站内会根据 Key 所属平台生成对应配置。

## 推荐方式

1. 打开 [API 密钥](https://subapi.loucer.cn/keys)。
2. 找到绑定 Gemini 分组的 Key，点击 **使用密钥**。
3. 选择 **Gemini CLI** 标签。
4. 按系统复制 `GOOGLE_GEMINI_BASE_URL`、`GEMINI_API_KEY` 和 `GEMINI_MODEL`。
5. 完全退出并重新打开终端或 Gemini CLI。

## 配置结构

```bash
export GOOGLE_GEMINI_BASE_URL="以站内生成值为准"
export GEMINI_API_KEY="sk-你的密钥"
export GEMINI_MODEL="以站内生成模型为准"
```

::: warning 不要照搬其它分组的地址
不同 Gemini / Antigravity 分组的 Base URL 可能不同，必须复制当前 Key 的生成内容。
:::

## 成功标志

1. Gemini CLI 能正常返回测试结果。
2. [使用记录](https://subapi.loucer.cn/usage) 出现新请求。
3. 记录里的模型、Key 和费用符合预期。

## 常见问题

- `401`：Key 错误或被禁用。
- `model not found`：模型名和分组不匹配。
- 能回复但没有记录：仍在使用官方登录态或旧环境变量。
- 修改后不生效：完全关闭旧终端，再重新打开。
