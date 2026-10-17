# 简付支付服务商

协议依据：[简付开放接口文档](https://jian-pay.com/docs/)。

## 配置

1. 在管理员「设置 → 支付设置」的「启用的服务商」中启用「简付」。
2. 新建简付实例，填写 `clientNo`、商户 KEY 和网关。主网关为 `https://jpay.hzjianban.com`，备用为 `https://api.jian-pay.com`。
3. 支付方式默认仅选支付宝。微信渠道开通并验证后，才勾选微信。
4. 配置 HTTPS 回调域名。支付通知路径为 `/api/v1/payment/webhook/jianpay`；支付返回使用服务端生成的本站结果地址，保留订单号和签名恢复令牌。
5. 在「支付宝 / 微信收款来源」中选择「简付 支付宝」并保存。微信可继续选择其他服务商，也可保持关闭。
6. 「支付后自动查单」默认开启：成功回调验签后，使用简付原生接口复核，查询失败暂停入账。关闭后，新订单通过回调验签、金额、商户、订单及支付方式校验即可入账。每笔订单保存下单时的开关策略；切换开关不改变已有订单，缺少策略快照的旧订单仍需查单。此开关独立于 FM，也不关闭手动核验、定期补偿查单或下单超时恢复查单。

服务商密钥加密存储，管理员读取接口不返回原密钥，编辑时留空保留原值。存在历史订单后，商户号、密钥、网关不能直接替换，也不能删除实例；切换商户请新建实例并切换新订单来源。停用实例不影响原订单回调和查单。

## 已接入能力

- 原生 JSON 收银台下单：`POST /open/payment/pay/create`。
- 主动查单：`POST /open/payment/pay/info`，使用本站商户订单号 `merchantOrderNo` 查询。
- 支付方式显式映射：支付宝为 `alipay`，微信为 `wx`，不会交给收银台自动选择。
- 元转换为整数分；成功回调必须匹配商户、订单、支付方式、应付金额和上游订单号。
- 回调 MD5 验签后，按订单保存的「支付后自动查单」策略决定是否再次查单确认。所有回调均校验金额、商户及订单归属；成功处理返回纯文本 `success`。
- 接入现有定期查单、用户订单核验、幂等入账和订阅履约。
- 下单失败或超时时保留原待核验订单，后续查单和回调仍可恢复，不自动切换支付商或新建上游订单。
- 桌面端将返回的 `payUrl` 本地生成为二维码；手机端打开收银台。`payQrcodeUrl` 是图片地址，不作为二维码内容。

## 当前边界

- 微信协议已适配，但商户渠道是否可用需要开通后实测。官方目前不支持微信外浏览器 H5 支付。
- 未接入 API 下单（`api-create`）和商户自有 JSAPI。
- 首版退款关闭，后端拒绝启用退款。
- 文档未提供关单接口；本地取消/过期不代表简付已关单。过期恢复窗口以外的付款保留审计并通知管理员人工核对。
- 未经过真实商户支付验证前，本地测试通过只证明协议与本站订单流程正确，不代表真实资金已到账。

## 本地验证

后端（在 `backend` 下）：

```sh
go test -tags unit ./internal/payment/... ./internal/service ./internal/handler ./internal/server/routes -run 'JianPay|Payment|Provider|FMSource|FMCallback|ZhifuFM' -count=1
```

前端（在 `frontend` 下）：

```sh
pnpm exec vitest run src/components/payment/__tests__ src/components/admin/settings/__tests__/FMPaymentRouting.spec.ts src/api/__tests__/settings.paymentVisibleMethods.spec.ts
pnpm run build
```

实际商户联调应使用新订单核对：支付方式、金额、上游订单号、本站到账与审计；重复回调和再次查单不能重复发放。
