# 支付FM 独立服务商

本模块随 SubAPI 部署，在支付设置的“启用的服务商”中单独启用“支付FM”。用户选择的仍是支付宝、微信或配置的其他支付方式。

## 配置

1. 添加 FM 实例，填写商户号、接入密钥、HTTPS 易支付兼容下单地址、HTTPS 原生接口根地址。
2. 下单地址取 FM 后台的易支付兼容接口地址。原生根地址取后台“接口根地址”，不要包含 `/queryOutOrder`。两者可能不同。
3. 选择已开通的支付宝/微信支付方式；其他方式用自定义方式映射。扫码模式使用兼容 JSON 下单，收银台模式使用兼容跳转下单。
4. 回调使用 `/api/v1/payment/webhook/zhifufm`；基础域名独立设置，建议使用主域名 HTTPS。同步跳转仍使用 `/payment/result`。
5. 开通 FM 查单权限后，在 FM 服务商配置内启用“支付后自动查单”开关。默认开启，与 Telegram 开关独立。关闭此设置只影响后来创建的订单，不能关闭已有订单的复核策略。
6. 在“支付宝 / 微信收款来源”选择支付FM，单独保存。启用 FM 实例不会自动改写现有收款来源。

接入密钥加密存储、管理员读取脱敏，输入留空保留原密钥。退款接口未验证，FM 自动退款及用户退款开关暂不提供。

## 查单行为

- 下单、验签使用 FM 易支付兼容协议；查询使用原生 `POST {queryBase}/queryOutOrder`。
- 按文档使用 URL Query 参数 `merchantNum`、`orderNo`、`sign`；签名为 `md5(merchantNum + orderNo + 接入密钥)` 的小写值。
- 仅已验签并通过订单、商户、金额校验的回调产生持久化任务，成功落库后应答 `success`，不在回调中等待上游查询。
- 只接受本站对应实例的订单。查询结果要求 `success=true`、`code=200`、`orderState=4`，订单号、商户号、平台交易号一致且订单金额精确到分一致。
- 比较 `amount`，不把浮动实际支付金额 `tradeMoney` 当订单应付金额。
- 页面确认、刷新、取消、到期以及历史订单恢复不主动查询 FM。结果页面只读取本站状态。
- 同一商户使用数据库共享限流；目前保守预留查询超时 10 秒再加 3 秒间隔，两个请求至少间隔 13 秒。多实例不能绕过限流。
- 网络故障、429、5xx 最多查询三次；两次失败尝试间隔至少 15 秒。权限失败、未支付、金额或身份不一致转人工复核，不自动放行。
- 自动查询限于订单到期后 5 分钟内，超过期限转人工；不批量扫描历史订单。有效回调较晚到达时仍会告警，不因重复回调重置期限或次数。
- 已查询确认的付款凭证持久化后，只重试本地入账，不再查 FM。余额与订阅沿用现有防重流程。
- 用户收到有效支付通知后看到独立的“正在查单”状态，隐藏付款二维码和取消付款按钮；查单通过后显示“正在入账”，仅充值完成后显示成功。异常显示待人工复核，不显示成功。查单很快时，页面可能直接显示最终成功，不刻意延迟。
- FM 充值成功的 Telegram 通知依据持久化付款证明显示“FM 已查单，付款核验通过，余额已入账”；关闭自动查单时显示“FM 未查单（自动查单已关闭）”，不会仅根据配置开关标记已查单。
- Telegram 只负责成功通知和异常告警。关闭 Telegram 不影响复核；外部通知可能在极端发送确认失败时重复，入账仍防重。

## 存量与升级

新模块 `provider_key=zhifufm`。历史 `easypay` 订单及 `/webhook/easypay` 路径保持原绑定；不会将它们批量改成 FM 或产生历史复核任务。

停用 FM 实例仅阻止新订单，已有订单仍能验签、复核。存在未完成订单或待处理/待人工复核任务时，禁止删除实例、替换关键商户凭据与接口地址。切换支付来源不改变历史订单。

升级包含迁移 `244_zhifufm_callback_reviews.sql`，新增复核任务及商户限流表。旧版回滚前先恢复原支付来源并停用新 FM 实例，等待 FM 存量订单与复核任务完成。尚有未完成订单时必须保留支持 FM 的新版处理回调；保留新增表和订单，不能删除付款凭证。真实商户接口及小额付款仍需单独验收，本地模拟测试不替代实付验收。

## 本地验证

```powershell
cd backend
go test -tags unit ./internal/payment/provider ./internal/service ./internal/handler ./internal/server/routes ./migrations -run 'TestZhifuFM|TestFM|TestTelegram|TestVerifyOrder|Test.*VisibleMethod|Test.*Provider.*|Test.*Payment|Test.*Migration' -count=1
```

设置 `FM_TEST_POSTGRES_DSN` 可在隔离 PostgreSQL 重跑 FM 测试。测试拒绝非 `127.0.0.1` 地址及非 `fm_fixture` 数据库，每次仅创建并清理本次随机测试 schema。不要填生产数据库连接串。

接口依据：[查单](https://docs.zhifux.com/read/zhifufm/querybyoutno)、[兼容回调](https://docs.zhifux.com/read/zhifufm/notifyv2)。查询频率遵循客服的“回调后验证、禁止持续轮询”要求。
