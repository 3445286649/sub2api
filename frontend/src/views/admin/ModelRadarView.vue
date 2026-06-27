<template>
  <AppLayout>
    <div class="space-y-5">
      <header class="card overflow-hidden">
        <div class="flex flex-col gap-5 px-5 py-5 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <p class="text-xs font-semibold uppercase tracking-wide text-primary-600 dark:text-primary-400">Daily Model Radar</p>
            <h1 class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">模型雷达</h1>
            <p class="mt-2 max-w-2xl text-sm text-gray-500 dark:text-dark-400">
              固定题集每日测试模型和推理档位，把当天更稳的配置推荐给用户。
            </p>
          </div>
          <div class="flex flex-wrap items-center gap-2">
            <button type="button" class="btn btn-secondary btn-sm" @click="loadAll">
              <Icon name="refresh" size="sm" />
            </button>
            <button type="button" class="btn btn-primary btn-sm" :disabled="running" @click="runNow">
              <Icon name="play" size="sm" />
              {{ running ? '启动中...' : '立即运行' }}
            </button>
          </div>
        </div>

        <div class="grid border-t border-gray-100 bg-gray-50/70 dark:border-dark-700 dark:bg-dark-800/60 sm:grid-cols-2 xl:grid-cols-5">
          <div v-for="item in summaryCards" :key="item.label" class="border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:border-r xl:border-b-0">
            <div class="text-xs text-gray-500 dark:text-dark-400">{{ item.label }}</div>
            <div class="mt-1 truncate text-sm font-semibold text-gray-900 dark:text-white">{{ item.value }}</div>
            <div v-if="item.hint" class="mt-1 truncate text-xs text-gray-500 dark:text-dark-400">{{ item.hint }}</div>
          </div>
        </div>
      </header>

      <section class="grid gap-5 xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
        <div class="space-y-5">
          <div class="card p-5">
            <div class="flex items-center justify-between gap-4">
              <div>
                <h2 class="text-base font-semibold text-gray-900 dark:text-white">运行入口</h2>
                <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">开关和自动运行时间按 Asia/Shanghai 生效。</p>
              </div>
              <Toggle v-model="form.enabled" />
            </div>

            <div class="mt-5 grid gap-4 sm:grid-cols-2">
              <label class="block">
                <span class="input-label">每日运行时间</span>
                <input v-model="runTime" class="input font-mono text-sm" type="time" />
                <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">每天 {{ runTime || '--:--' }} 自动运行，按 Asia/Shanghai 时间。</span>
              </label>
              <label class="block">
                <span class="input-label">单题超时</span>
                <div class="flex items-center gap-2">
                  <input v-model.number="form.timeout_seconds" class="input" type="number" min="10" max="300" />
                  <span class="text-sm text-gray-500 dark:text-dark-400">秒</span>
                </div>
              </label>
              <label class="block">
                <span class="input-label">并发</span>
                <input v-model.number="form.concurrency" class="input" type="number" min="1" max="6" />
              </label>
              <label class="block">
                <span class="input-label">每日预算</span>
                <div class="flex items-center gap-2">
                  <input v-model.number="form.daily_budget_usd_cents" class="input" type="number" min="0" />
                  <span class="text-sm text-gray-500 dark:text-dark-400">美分</span>
                </div>
              </label>
            </div>
          </div>

          <div class="card p-5">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">测试身份</h2>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">可选择本站已有 Key，也可手动填写外部 Key。</p>
            </div>

            <div class="mt-4 inline-flex rounded-lg border border-gray-200 bg-gray-50 p-1 dark:border-dark-700 dark:bg-dark-800">
              <button
                v-for="source in keySourceOptions"
                :key="source.value"
                type="button"
                class="rounded-md px-3 py-1.5 text-sm transition"
                :class="form.api_key_source === source.value ? 'bg-primary-600 text-white shadow-sm' : 'text-gray-500 hover:text-gray-900 dark:text-dark-400 dark:hover:text-white'"
                @click="setKeySource(source.value)"
              >
                {{ source.label }}
              </button>
            </div>

            <div class="mt-5 space-y-4">
              <label class="block">
                <span class="input-label">API Base URL</span>
                <div class="flex flex-col gap-2 sm:flex-row">
                  <input
                    v-model="form.api_base_url"
                    class="input font-mono text-sm"
                    name="model-radar-api-base-url"
                    autocomplete="off"
                    autocapitalize="none"
                    spellcheck="false"
                    placeholder="https://your-domain.com 或 https://your-domain.com/v1"
                  />
                  <button type="button" class="btn btn-secondary whitespace-nowrap" @click="useCurrentSiteURL">
                    使用本站
                  </button>
                </div>
              </label>

              <label v-if="form.api_key_source === 'existing'" class="block">
                <span class="input-label">选择 API Key</span>
                <select
                  v-model.number="selectedAPIKeyID"
                  class="input text-sm"
                  name="model-radar-existing-key"
                  autocomplete="off"
                  :disabled="keysLoading"
                >
                  <option :value="0">{{ keysLoading ? '正在读取...' : '请选择当前账号的 API Key' }}</option>
                  <option v-for="key in apiKeys" :key="key.id" :value="key.id">
                    {{ key.name || `Key #${key.id}` }} · {{ maskApiKey(key.key) }} · {{ key.group?.name || '无分组' }} · {{ key.status }}
                  </option>
                </select>
                <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">只保存 Key ID，运行时由后端读取，前端不暴露明文。</span>
              </label>

              <label v-else class="block">
                <span class="input-label">专用测试 API Key</span>
                <input
                  v-model="form.api_key"
                  class="input font-mono text-sm"
                  type="password"
                  name="model-radar-custom-token"
                  autocomplete="new-password"
                  autocapitalize="none"
                  spellcheck="false"
                  :placeholder="form.api_key_configured ? `已配置：${form.api_key_masked}` : 'sk-...'"
                />
                <span class="mt-1 block text-xs text-gray-500 dark:text-dark-400">留空保存时保留已有自定义 Key。</span>
              </label>
            </div>
          </div>
        </div>

        <div class="card p-5">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-white">模型矩阵</h2>
              <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">可添加任意模型和推理档位组合。</p>
            </div>
            <div class="flex flex-wrap gap-2">
              <button type="button" class="btn btn-secondary btn-sm" @click="addMatrixRow">
                <Icon name="plus" size="sm" />
                添加组合
              </button>
              <button type="button" class="btn btn-secondary btn-sm" @click="resetDefaultMatrix">
                恢复默认
              </button>
            </div>
          </div>

          <div class="mt-4 overflow-x-auto rounded-lg border border-gray-200 dark:border-dark-700">
            <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
              <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800 dark:text-dark-400">
                <tr>
                  <th class="px-3 py-3 text-left font-medium">模型</th>
                  <th class="px-3 py-3 text-left font-medium">推理档位</th>
                  <th class="px-3 py-3 text-left font-medium">状态</th>
                  <th class="px-3 py-3 text-right font-medium">操作</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
                <tr v-for="(item, index) in form.matrix" :key="index">
                  <td class="px-3 py-2">
                    <input v-model="item.model" class="input font-mono text-sm" placeholder="gpt-5.5" />
                  </td>
                  <td class="px-3 py-2">
                    <input v-model="item.reasoning_effort" class="input text-sm" placeholder="medium / high / xhigh" />
                  </td>
                  <td class="px-3 py-2">
                    <label class="inline-flex items-center gap-2 text-sm text-gray-700 dark:text-dark-300">
                      <input v-model="item.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500 dark:border-dark-600 dark:bg-dark-800" />
                      启用
                    </label>
                  </td>
                  <td class="px-3 py-2 text-right">
                    <button type="button" class="btn btn-secondary btn-sm text-red-600 hover:text-red-700 dark:text-red-400" @click="removeMatrixRow(index)">
                      删除
                    </button>
                  </td>
                </tr>
                <tr v-if="form.matrix.length === 0">
                  <td colspan="4" class="px-3 py-8 text-center text-sm text-gray-500 dark:text-dark-400">还没有模型组合，请添加至少一项。</td>
                </tr>
              </tbody>
            </table>
          </div>

          <div class="mt-5 flex justify-end">
            <button type="button" class="btn btn-primary" :disabled="saving" @click="saveConfig">
              {{ saving ? '保存中...' : '保存配置' }}
            </button>
          </div>
        </div>
      </section>

      <section class="card overflow-hidden">
        <div class="flex items-center justify-between border-b border-gray-100 px-5 py-4 dark:border-dark-700">
          <h2 class="text-base font-semibold text-gray-900 dark:text-white">运行历史</h2>
          <span class="text-xs text-gray-500 dark:text-dark-400">最近 {{ runs.length }} 条</span>
        </div>
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
            <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800 dark:text-dark-400">
              <tr>
                <th class="px-4 py-3 text-left font-medium">ID</th>
                <th class="px-4 py-3 text-left font-medium">日期</th>
                <th class="px-4 py-3 text-left font-medium">触发</th>
                <th class="px-4 py-3 text-left font-medium">状态</th>
                <th class="px-4 py-3 text-left font-medium">发布</th>
                <th class="px-4 py-3 text-left font-medium">成功组合</th>
                <th class="px-4 py-3 text-right font-medium">操作</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
              <tr v-for="run in runs" :key="run.id" class="text-gray-700 dark:text-dark-300">
                <td class="px-4 py-3">#{{ run.id }}</td>
                <td class="px-4 py-3">{{ formatDate(run.run_date) }}</td>
                <td class="px-4 py-3">{{ run.trigger_type }}</td>
                <td class="px-4 py-3">
                  <span class="rounded-full px-2 py-0.5 text-xs" :class="statusClass(run.status)">{{ run.status }}</span>
                </td>
                <td class="px-4 py-3">{{ run.published ? '是' : '否' }}</td>
                <td class="px-4 py-3">{{ run.success_combinations }}/{{ run.total_combinations }}</td>
                <td class="px-4 py-3 text-right">
                  <button type="button" class="text-sm font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300" @click="loadRunDetail(run.id)">
                    查看详情
                  </button>
                </td>
              </tr>
              <tr v-if="runs.length === 0">
                <td colspan="7" class="px-4 py-8 text-center text-sm text-gray-500 dark:text-dark-400">暂无运行记录</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section v-if="selectedDetail" class="card overflow-hidden">
        <div class="flex flex-col gap-2 border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">运行详情 #{{ selectedDetail.run.id }}</h2>
            <p class="mt-1 text-xs text-gray-500 dark:text-dark-400">
              {{ formatDate(selectedDetail.run.run_date) }} · {{ selectedDetail.run.status }} · {{ selectedDetail.run.published ? '已发布' : '未发布' }}
            </p>
          </div>
          <button type="button" class="btn btn-secondary btn-sm" @click="selectedDetail = null">
            关闭
          </button>
        </div>

        <div class="space-y-4 p-5">
          <div v-for="result in selectedDetail.results" :key="result.id" class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
            <div class="flex flex-wrap items-center gap-2">
              <span class="font-mono text-sm font-semibold text-gray-900 dark:text-white">{{ result.model }}</span>
              <span class="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600 dark:bg-dark-800 dark:text-dark-300">{{ result.reasoning_effort || 'default' }}</span>
              <span class="rounded-full px-2 py-0.5 text-xs" :class="statusClass(result.status)">{{ result.status }}</span>
            </div>
            <div class="mt-2 text-xs text-gray-500 dark:text-dark-400">
              排名 #{{ result.rank }} · 雷达分 {{ result.score }} · 通过 {{ result.pass_count }}/{{ result.total_count }} · 错误 {{ result.error_count }} · 平均耗时 {{ result.avg_latency_ms ? `${result.avg_latency_ms}ms` : '-' }}
            </div>
            <p v-if="result.error_message" class="mt-2 text-xs text-red-600 dark:text-red-400">{{ result.error_message }}</p>

            <div class="mt-4 overflow-x-auto">
              <table class="min-w-full divide-y divide-gray-200 text-xs dark:divide-dark-700">
                <thead class="text-gray-500 dark:text-dark-400">
                  <tr>
                    <th class="py-2 pr-3 text-left">题目</th>
                    <th class="px-3 py-2 text-left">结果</th>
                    <th class="px-3 py-2 text-left">期望</th>
                    <th class="px-3 py-2 text-left">输出摘要</th>
                    <th class="px-3 py-2 text-left">错误</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-100 dark:divide-dark-800">
                  <tr v-for="task in tasksForResult(result.id)" :key="task.id">
                    <td class="py-2 pr-3 font-mono text-gray-700 dark:text-dark-300">{{ task.task_id }}</td>
                    <td class="px-3 py-2" :class="task.passed ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'">{{ task.passed ? '通过' : '失败' }}</td>
                    <td class="px-3 py-2 font-mono text-gray-500 dark:text-dark-400">{{ task.expected_answer }}</td>
                    <td class="max-w-md px-3 py-2 text-gray-500 dark:text-dark-400">{{ task.actual_answer || '-' }}</td>
                    <td class="max-w-md px-3 py-2 text-red-600 dark:text-red-400">{{ task.error_message || '-' }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, reactive, ref, onMounted } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import { getConfig, updateConfig, runNow as runNowAPI, listRuns, getRun } from '@/api/admin/modelRadar'
import { keysAPI } from '@/api/keys'
import { maskApiKey } from '@/utils/maskApiKey'
import type { ApiKey } from '@/types'
import type { ModelRadarConfig, ModelRadarRun, ModelRadarRunDetail, ModelRadarTaskResult, ModelRadarStatus } from '@/api/modelRadar'

const defaultMatrix = [
  { model: 'gpt-5.5', reasoning_effort: 'xhigh', enabled: true },
  { model: 'gpt-5.5', reasoning_effort: 'high', enabled: true },
  { model: 'gpt-5.5', reasoning_effort: 'medium', enabled: true },
  { model: 'gpt-5.4', reasoning_effort: 'xhigh', enabled: true },
  { model: 'gpt-5.4', reasoning_effort: 'high', enabled: true },
  { model: 'gpt-5.4', reasoning_effort: 'medium', enabled: true },
]

const appStore = useAppStore()
const saving = ref(false)
const running = ref(false)
const keysLoading = ref(false)
const apiKeys = ref<ApiKey[]>([])
const runs = ref<ModelRadarRun[]>([])
const selectedDetail = ref<ModelRadarRunDetail | null>(null)
const form = reactive<ModelRadarConfig>({
  enabled: false,
  api_base_url: '',
  api_key_source: 'custom',
  api_key_id: null,
  api_key_name: '',
  api_key_group_name: '',
  api_key_status: '',
  api_key: '',
  api_key_configured: false,
  api_key_masked: '',
  run_hour: 4,
  run_minute: 30,
  timeout_seconds: 90,
  concurrency: 2,
  daily_budget_usd_cents: 100,
  matrix: [],
})

const keySourceOptions = [
  { label: '选择已有 Key', value: 'existing' as const },
  { label: '手动填写 Key', value: 'custom' as const },
]

const runTime = computed({
  get: () => `${String(form.run_hour ?? 4).padStart(2, '0')}:${String(form.run_minute ?? 30).padStart(2, '0')}`,
  set: (value: string) => {
    const [hour, minute] = value.split(':').map(part => Number(part))
    form.run_hour = Number.isFinite(hour) ? hour : 4
    form.run_minute = Number.isFinite(minute) ? minute : 30
  },
})

const selectedAPIKeyID = computed({
  get: () => form.api_key_id || 0,
  set: (value: number) => {
    form.api_key_id = value > 0 ? value : null
  },
})

const latestRun = computed(() => runs.value[0] || null)
const latestSuccess = computed(() => runs.value.find(item => item.status === 'succeeded' && item.published) || null)
const summaryCards = computed(() => [
  { label: '功能状态', value: form.enabled ? '已开启' : '已关闭', hint: form.enabled ? '用户侧入口可见' : '用户侧入口隐藏' },
  { label: 'Key 来源', value: form.api_key_source === 'existing' ? '选择已有 Key' : '手动填写 Key', hint: keySummary.value },
  { label: 'Base URL', value: form.api_base_url || '未配置', hint: '调用 /v1/chat/completions' },
  { label: '下次运行', value: runTime.value, hint: 'Asia/Shanghai' },
  { label: '最近成功', value: latestSuccess.value ? `#${latestSuccess.value.id}` : '暂无', hint: latestRun.value ? `最近 #${latestRun.value.id} · ${latestRun.value.status}` : '暂无运行记录' },
])

const keySummary = computed(() => {
  if (form.api_key_source === 'existing') {
    return form.api_key_name || (form.api_key_id ? `Key #${form.api_key_id}` : '未选择')
  }
  return form.api_key_configured ? form.api_key_masked : '未配置'
})

onMounted(loadAll)

async function loadAll() {
  await Promise.all([loadConfig(), loadRuns(), loadKeys()])
}

async function loadConfig() {
  const cfg = await getConfig()
  Object.assign(form, cfg, {
    api_key_source: cfg.api_key_source || 'custom',
    api_key_id: cfg.api_key_id || null,
    api_key: '',
    matrix: cfg.matrix?.length ? cfg.matrix.map(item => ({ ...item })) : [],
  })
}

async function loadKeys() {
  keysLoading.value = true
  try {
    const res = await keysAPI.list(1, 100, { status: 'active' })
    apiKeys.value = res.items
  } catch (err) {
    appStore.showError((err as { message?: string }).message || '读取 API Key 失败')
  } finally {
    keysLoading.value = false
  }
}

async function loadRuns() {
  const res = await listRuns()
  runs.value = res.items
}

async function loadRunDetail(id: number) {
  try {
    selectedDetail.value = await getRun(id)
  } catch (err) {
    appStore.showError((err as { message?: string }).message || '读取运行详情失败')
  }
}

async function saveConfig() {
  if (looksLikeEmail(form.api_base_url) || !looksLikeModelRadarURL(form.api_base_url)) {
    appStore.showError('API Base URL 需要填写站点地址，例如 https://subapi.loucer.cn/v1')
    return
  }
  if (form.api_key_source === 'existing' && !form.api_key_id) {
    appStore.showError('请选择一个 API Key')
    return
  }
  if (form.matrix.length === 0 || !form.matrix.some(item => item.enabled && item.model.trim())) {
    appStore.showError('请至少启用一个模型组合')
    return
  }
  saving.value = true
  try {
    const payload = {
      ...form,
      api_key: form.api_key_source === 'existing' ? '' : form.api_key,
      matrix: form.matrix.map(item => ({
        model: item.model.trim(),
        reasoning_effort: item.reasoning_effort.trim(),
        enabled: item.enabled,
      })),
    }
    const saved = await updateConfig(payload)
    Object.assign(form, saved, { api_key: '', matrix: saved.matrix.map(item => ({ ...item })) })
    appStore.showSuccess('模型雷达配置已保存')
  } catch (err) {
    appStore.showError((err as { message?: string }).message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function runNow() {
  running.value = true
  try {
    const detail = await runNowAPI()
    await loadRuns()
    if (detail.run) {
      selectedDetail.value = {
        ...detail,
        results: detail.results || [],
        task_results: detail.task_results || {},
      }
    }
    appStore.showSuccess('模型雷达已开始运行，请稍后刷新运行历史查看结果')
  } catch (err) {
    appStore.showError((err as { message?: string }).message || '运行失败')
  } finally {
    running.value = false
  }
}

function setKeySource(source: 'custom' | 'existing') {
  form.api_key_source = source
  if (source === 'existing') {
    form.api_key = ''
    if (!apiKeys.value.length) void loadKeys()
  } else {
    form.api_key_id = null
  }
}

function useCurrentSiteURL() {
  form.api_base_url = appStore.apiBaseUrl || `${window.location.origin}/v1`
}

function looksLikeEmail(value: string) {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value.trim())
}

function looksLikeModelRadarURL(value: string) {
  const trimmed = value.trim()
  if (!trimmed) return false
  try {
    const url = new URL(trimmed)
    return url.protocol === 'http:' || url.protocol === 'https:'
  } catch {
    return false
  }
}

function addMatrixRow() {
  form.matrix.push({ model: '', reasoning_effort: 'medium', enabled: true })
}

function removeMatrixRow(index: number) {
  form.matrix.splice(index, 1)
}

function resetDefaultMatrix() {
  form.matrix = defaultMatrix.map(item => ({ ...item }))
}

function statusClass(status: ModelRadarStatus | string) {
  if (status === 'succeeded') return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300'
  if (status === 'running') return 'bg-primary-100 text-primary-700 dark:bg-primary-500/10 dark:text-primary-300'
  if (status === 'failed') return 'bg-red-100 text-red-700 dark:bg-red-500/10 dark:text-red-300'
  return 'bg-gray-100 text-gray-600 dark:bg-dark-800 dark:text-dark-300'
}

function formatDate(value: string) {
  return new Date(value).toLocaleDateString('zh-CN')
}

function tasksForResult(resultId: number): ModelRadarTaskResult[] {
  return selectedDetail.value?.task_results[String(resultId)] || []
}
</script>
