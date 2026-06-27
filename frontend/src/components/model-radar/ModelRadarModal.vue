<template>
  <teleport to="body">
    <transition name="radar-fade">
      <div
        v-if="open"
        class="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/65 px-3 py-4 backdrop-blur-sm sm:px-5"
        role="dialog"
        aria-modal="true"
        aria-labelledby="model-radar-title"
        @click.self="close"
      >
        <div class="flex max-h-[88vh] w-full max-w-5xl flex-col overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-2xl dark:border-dark-700 dark:bg-dark-900">
          <div class="flex items-start justify-between gap-4 border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:px-6">
            <div class="min-w-0">
              <p class="text-xs font-semibold text-cyan-600 dark:text-cyan-300">Daily Model Radar</p>
              <h2 id="model-radar-title" class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">
                模型雷达
              </h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
                每天用固定题集测试模型和推理档位，推荐当前更稳的 Codex 配置。
              </p>
            </div>
            <button type="button" class="rounded-lg p-2 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-800 dark:hover:text-gray-200" aria-label="关闭" @click="close">
              <Icon name="x" size="sm" />
            </button>
          </div>

          <main class="min-h-0 flex-1 overflow-y-auto px-5 py-5 sm:px-6">
            <div v-if="loading" class="flex h-64 items-center justify-center text-sm text-gray-500 dark:text-gray-400">
              正在读取今日雷达...
            </div>

            <div v-else-if="error" class="rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800 dark:border-amber-500/20 dark:bg-amber-500/10 dark:text-amber-200">
              {{ error }}
            </div>

            <div v-else-if="!current?.run" class="rounded-xl border border-gray-200 bg-gray-50 p-5 text-sm text-gray-600 dark:border-dark-700 dark:bg-dark-800/60 dark:text-gray-300">
              还没有已发布的模型雷达结果。管理员配置专用测试 Key 后，系统会每天自动生成推荐。
            </div>

            <div v-else class="space-y-5">
              <section class="space-y-3">
                <div class="flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
                  <div>
                    <div class="text-sm font-semibold text-gray-900 dark:text-white">今日 Top 3</div>
                    <p class="text-xs text-gray-500 dark:text-gray-400">
                      更新时间：{{ formatDateTime(current.updated_at || current.run.finished_at || current.run.updated_at) }}
                    </p>
                  </div>
                  <button type="button" class="inline-flex items-center gap-2 rounded-lg border border-gray-200 px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50 dark:border-dark-600 dark:text-gray-200 dark:hover:bg-dark-700" @click="copyConfig(current.recommendation)">
                    <Icon name="copy" size="sm" />
                    复制第 1 名
                  </button>
                </div>

                <div class="grid gap-3 lg:grid-cols-3">
                  <article
                    v-for="item in topThreeResults"
                    :key="`${item.model}-${item.reasoning_effort}`"
                    class="rounded-xl border p-4"
                    :class="item.rank === 1 ? 'border-cyan-500/30 bg-cyan-500/10' : 'border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800'"
                  >
                    <div class="flex items-center justify-between gap-3">
                      <span class="rounded-full px-2 py-0.5 text-xs font-semibold" :class="item.rank === 1 ? 'bg-cyan-500 text-white' : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'">#{{ item.rank }}</span>
                      <button type="button" class="rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-dark-700 dark:hover:text-gray-200" title="复制配置" @click="copyConfig(item)">
                        <Icon name="copy" size="sm" />
                      </button>
                    </div>
                    <div class="mt-4 font-mono text-xl font-semibold text-gray-900 dark:text-white">{{ item.model }}</div>
                    <div class="mt-2 inline-flex rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-cyan-700 dark:bg-dark-700 dark:text-cyan-300">
                      {{ item.reasoning_effort || 'default' }}
                    </div>
                    <div class="mt-4 grid grid-cols-2 gap-2 text-xs text-gray-500 dark:text-gray-400">
                      <div>
                        <div>雷达分</div>
                        <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ item.score }}</div>
                      </div>
                      <div>
                        <div>通过</div>
                        <div class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ item.pass_count }}/{{ item.total_count }}</div>
                      </div>
                    </div>
                  </article>
                </div>
              </section>

              <section class="overflow-hidden rounded-xl border border-gray-200 dark:border-dark-700">
                <div class="border-b border-gray-100 px-4 py-3 text-sm font-semibold text-gray-900 dark:border-dark-700 dark:text-white">
                  今日排名
                </div>
                <div class="overflow-x-auto">
                  <table class="min-w-full divide-y divide-gray-100 text-sm dark:divide-dark-700">
                    <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
                      <tr>
                        <th class="px-4 py-3 text-left">排名</th>
                        <th class="px-4 py-3 text-left">模型</th>
                        <th class="px-4 py-3 text-left">推理档位</th>
                        <th class="px-4 py-3 text-left">雷达分</th>
                        <th class="px-4 py-3 text-left">通过率</th>
                        <th class="px-4 py-3 text-left">平均耗时</th>
                      </tr>
                    </thead>
                    <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
                      <tr v-for="item in current.results" :key="`${item.model}-${item.reasoning_effort}`" class="text-gray-700 dark:text-gray-200" :class="item.rank <= 3 ? 'bg-cyan-50/40 dark:bg-cyan-500/5' : ''">
                        <td class="px-4 py-3 font-semibold">#{{ item.rank }}</td>
                        <td class="px-4 py-3 font-mono">{{ item.model }}</td>
                        <td class="px-4 py-3">{{ item.reasoning_effort || 'default' }}</td>
                        <td class="px-4 py-3">{{ item.score }}</td>
                        <td class="px-4 py-3">{{ item.pass_count }}/{{ item.total_count }}</td>
                        <td class="px-4 py-3">{{ item.avg_latency_ms ? `${item.avg_latency_ms}ms` : '-' }}</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </section>

              <section class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
                <div class="mb-3 text-sm font-semibold text-gray-900 dark:text-white">最近 7 次最佳推荐</div>
                <div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
                  <div v-for="item in current.history" :key="item.id" class="rounded-lg bg-gray-50 px-3 py-2 dark:bg-dark-800">
                    <div class="font-mono text-sm text-gray-900 dark:text-white">{{ item.model }}</div>
                    <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                      {{ item.reasoning_effort || 'default' }} · {{ item.score }} 分 · {{ item.pass_count }}/{{ item.total_count }}
                    </div>
                  </div>
                </div>
              </section>
            </div>
          </main>
        </div>
      </div>
    </transition>
  </teleport>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { getCurrent, type ModelRadarCurrentResponse, type ModelRadarResult } from '@/api/modelRadar'
import { useAppStore } from '@/stores'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ 'update:open': [value: boolean] }>()

const appStore = useAppStore()
const loading = ref(false)
const error = ref('')
const current = ref<ModelRadarCurrentResponse | null>(null)

const configSnippet = computed(() => {
  const rec = current.value?.recommendation
  if (!rec) return ''
  return formatConfigSnippet(rec)
})

const topThreeResults = computed(() => current.value?.results.slice(0, 3) || [])

watch(() => props.open, (open) => {
  if (open) void load()
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    current.value = await getCurrent()
  } catch (err) {
    error.value = (err as { message?: string }).message || '模型雷达暂时不可用'
  } finally {
    loading.value = false
  }
}

function close() {
  emit('update:open', false)
}

async function copyConfig(item?: ModelRadarResult | null) {
  const snippet = item ? formatConfigSnippet(item) : configSnippet.value
  if (!snippet) return
  await navigator.clipboard.writeText(snippet)
  appStore.showSuccess('已复制推荐配置')
}

function formatConfigSnippet(item: ModelRadarResult): string {
  return `model = "${item.model}"\nmodel_reasoning_effort = "${item.reasoning_effort || 'medium'}"`
}

function formatDateTime(value?: string | null): string {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', { hour12: false })
}
</script>

<style scoped>
.radar-fade-enter-active,
.radar-fade-leave-active {
  transition: opacity 0.18s ease;
}

.radar-fade-enter-from,
.radar-fade-leave-to {
  opacity: 0;
}
</style>
