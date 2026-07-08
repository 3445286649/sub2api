# 新用户 3 分钟速通

目标：用最短路径完成 **充值/兑换到账 -> 创建 API Key -> 导入 CCSwitch -> 发起一次可验证请求**。

![仪表盘入口](/images/subapi/01-dashboard-overview.png)

## 适用对象

- 第一次使用 subapi 的用户。
- 已经拿到兑换码或准备站内充值的用户。
- 不想先看完整文档，只想快速跑通的人。

## 3 分钟流程

1. 确认账户有余额或有效订阅。
2. 进入 **API 密钥** 页面创建 Key。
3. 创建 Key 时选择可用分组。
4. 用 Google Chrome 打开 `https://subapi.loucer.cn/keys`。
5. 点击 **导入到 CCS**。
6. 在 CCSwitch 里确认 `loucer_api` 显示 **使用中**。
7. 点击 **刷新用量** 或发起一次测试请求。

## 成功标志

- 仪表盘余额或订阅状态正常。
- API Key 已绑定可用分组。
- CCSwitch 中 `loucer_api` 显示 **使用中**。
- 站内 **使用记录** 能看到调用记录。

## 下一步

- 不会创建 Key：看 [API Key 创建与安全](/api-key)。
- 不会导入 CCSwitch：看 [CCSwitch 接入](/ccswitch)。
- 请求失败：看 [常见报错排查](/troubleshooting)。
