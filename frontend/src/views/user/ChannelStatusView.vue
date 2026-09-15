<template>
  <ChannelStatusV1View v-if="mode === 'v1'" />
  <ChannelStatusV2View v-else-if="mode === 'v2' || route.query.monitor_view === 'v2'" />
  <ChannelStatusV3View v-else />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { getChannelMonitorMode } from '@/utils/featureFlags'
import ChannelStatusV1View from './ChannelStatusV1View.vue'
import ChannelStatusV2View from './ChannelStatusV2View.vue'
import ChannelStatusV3View from './ChannelStatusV3View.vue'

const mode = computed(() => getChannelMonitorMode())
const route = useRoute()
</script>
