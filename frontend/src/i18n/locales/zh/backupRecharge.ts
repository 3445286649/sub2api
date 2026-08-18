export default {
  nav: {
    backupRecharge: '备用充值',
  },
  backupRecharge: {
    title: '备用充值',
    description: '在站内打开备用充值渠道，购买完成后回到兑换码页面完成兑换。',
    refresh: '刷新当前渠道',
    openExternal: '外部打开',
    goRedeem: '去兑换码',
    loading: '正在加载充值渠道',
    unavailable: '备用充值暂未开放',
    unavailableDescription: '当前没有可用的充值渠道，请稍后再试或联系管理员。',
    loadFailed: '渠道页面加载失败',
    loadFailedDescription: '该卡网可能禁止站内嵌入，你可以使用外部打开继续购买。',
    channelReady: '渠道已加载',
    purchaseHint: '支付和发货由外部卡网完成；发货后请复制卡密，再回到兑换码页面使用。',
    frameLabel: '{name}充值页面',
  },
  admin: {
    settings: {
      site: {
        rechargeStorefront: {
          channelsTitle: '充值渠道',
          channelsHint: '最多配置 8 个 HTTPS 渠道；关闭渠道只会隐藏入口，不会删除历史配置。',
          addChannel: '添加渠道',
          sortOrder: '排序',
          moveUp: '上移渠道',
          moveDown: '下移渠道',
          channelName: '渠道名称',
          channelUrl: '渠道网址',
          enabled: '启用',
          noChannels: '暂未配置渠道',
          compatibilityHint: '保存时会同步旧版主地址和备用地址字段，便于版本回滚。',
        },
      },
    },
  },
}
