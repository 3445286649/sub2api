export default {
  nav: {
    backupRecharge: 'Backup Recharge',
  },
  backupRecharge: {
    title: 'Backup Recharge',
    description: 'Open a backup recharge channel in the site, then return to redeem your delivered code.',
    refresh: 'Refresh channel',
    openExternal: 'Open externally',
    goRedeem: 'Go to redeem',
    loading: 'Loading recharge channel',
    unavailable: 'Backup recharge is unavailable',
    unavailableDescription: 'There are no available recharge channels right now. Try again later or contact an administrator.',
    loadFailed: 'Channel failed to load',
    loadFailedDescription: 'This store may block embedded pages. Open it externally to continue purchasing.',
    channelReady: 'Channel loaded',
    purchaseHint: 'Payment and fulfillment happen on the external store. Copy the delivered code, then redeem it here.',
    frameLabel: '{name} recharge page',
  },
  admin: {
    settings: {
      site: {
        rechargeStorefront: {
          channelsTitle: 'Recharge channels',
          channelsHint: 'Configure up to 8 HTTPS channels. Disabling a channel hides it without deleting its configuration.',
          addChannel: 'Add channel',
          sortOrder: 'Sort order',
          moveUp: 'Move channel up',
          moveDown: 'Move channel down',
          channelName: 'Channel name',
          channelUrl: 'Channel URL',
          enabled: 'Enabled',
          noChannels: 'No channels configured',
          compatibilityHint: 'Saving also syncs the legacy primary and backup URL fields for rollback compatibility.',
        },
      },
    },
  },
}
