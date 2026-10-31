<template>
  <div class="space-y-3 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
    <div class="flex items-center justify-between gap-2">
      <label class="input-label mb-0">{{ t('admin.accounts.videoCapabilities.title') }}</label>
      <button type="button" class="btn btn-secondary btn-sm" @click="add">{{ t('admin.accounts.videoCapabilities.add') }}</button>
    </div>
    <p class="input-hint">{{ t('admin.accounts.videoCapabilities.hint') }}</p>
    <div v-for="(item, index) in modelValue" :key="index" class="space-y-3 border-t border-gray-200 pt-3 dark:border-dark-600">
      <div class="flex gap-2">
        <input :value="item.model" class="input min-w-0 flex-1" :aria-label="t('admin.accounts.videoCapabilities.model')" :placeholder="t('admin.accounts.videoCapabilities.model')" @input="update(index, 'model', ($event.target as HTMLInputElement).value)" />
        <button type="button" class="btn btn-secondary btn-sm" @click="emit('update:modelValue', modelValue.filter((_, i) => i !== index))">{{ t('common.delete') }}</button>
      </div>
      <div class="grid grid-cols-2 gap-3 sm:grid-cols-3">
        <label v-for="field in countFields" :key="field.key" class="text-xs">
          {{ t(`admin.accounts.videoCapabilities.${field.key}`) }}
          <input type="number" min="0" :max="field.max" :value="item[field.key]" class="input mt-1" @input="update(index, field.key, Number(($event.target as HTMLInputElement).value))" />
        </label>
      </div>
      <label class="flex items-center gap-2 text-sm"><input type="checkbox" :checked="item.supports_frames" @change="update(index, 'supports_frames', ($event.target as HTMLInputElement).checked)" />{{ t('admin.accounts.videoCapabilities.frames') }}</label>
      <div class="flex flex-wrap gap-3 text-sm">
        <label v-for="resolution in ['480p', '720p', '1080p', '4k']" :key="resolution" class="flex items-center gap-1">
          <input type="checkbox" :checked="item.resolutions.includes(resolution)" @change="toggleResolution(index, resolution)" />{{ resolution }}
        </label>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { VideoModelCapability } from '@/types'
const props = defineProps<{ modelValue: VideoModelCapability[] }>()
const emit = defineEmits<{ 'update:modelValue': [VideoModelCapability[]] }>()
const { t } = useI18n()
const countFields = [
  { key: 'max_reference_images', max: 30 }, { key: 'max_reference_videos', max: 10 },
  { key: 'max_reference_audios', max: 10 }, { key: 'min_duration', max: 600 }, { key: 'max_duration', max: 600 },
] as const
function update<K extends keyof VideoModelCapability>(index: number, key: K, value: VideoModelCapability[K]) {
  emit('update:modelValue', props.modelValue.map((item, i) => i === index ? { ...item, [key]: value } : item))
}
function toggleResolution(index: number, resolution: string) {
  const current = props.modelValue[index].resolutions
  update(index, 'resolutions', current.includes(resolution) ? current.filter(value => value !== resolution) : [...current, resolution])
}
function add() {
  emit('update:modelValue', [...props.modelValue, { model: '', max_reference_images: 9, max_reference_videos: 10, max_reference_audios: 10, supports_frames: true, resolutions: ['480p', '720p', '1080p'], min_duration: 0, max_duration: 0 }])
}
</script>
