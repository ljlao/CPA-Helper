<script setup lang="ts">
import type { Component } from 'vue'
import { computed, h, onMounted, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NDataTable,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NModal,
  NRadioButton,
  NRadioGroup,
  NSelect,
  NSpace,
  useDialog,
  useMessage,
  type DataTableColumns,
} from 'naive-ui'
import {
  Activity,
  CircleDollarSign,
  Copy,
  Eye,
  EyeOff,
  KeyRound,
  Layers3,
  Send,
} from 'lucide-vue-next'

import {
  createApiKey,
  deleteApiKey,
  getModelRequestGuide,
  listApiKeys,
  testModelRequest,
  updateApiKey,
} from '@/features/api-keys/api/apiKeysApi'
import { listAvailableModels } from '@/features/models/api/availableModelsApi'
import { getCurrentUserQuota } from '@/features/users/api/usersApi'
import { getUsageOverview } from '@/features/usage/api/usageApi'
import type {
  AvailableModel,
  AvailableModelsResponse,
  ModelRequestEndpoint,
  ModelRequestGuide,
  ModelRequestTestResponse,
  UsageSummary,
  UserApiKeySummary,
  UserQuotaStatus,
} from '@/shared/types/api'
import { copyToClipboard } from '@/shared/utils/clipboard'
import { formatCompact, formatDateTime, formatInteger, formatUsd } from '@/shared/utils/format'

const message = useMessage()
const dialog = useDialog()
const isLoading = ref(false)
const isSaving = ref(false)
const apiKeys = ref<UserApiKeySummary[]>([])
const usageSummary = ref<UsageSummary | null>(null)
const quotaStatus = ref<UserQuotaStatus | null>(null)
const modelRequestGuide = ref<ModelRequestGuide | null>(null)
const availableModels = ref<AvailableModelsResponse | null>(null)
const editorVisible = ref(false)
const requestTestVisible = ref(false)
const requestTestApiKey = ref<UserApiKeySummary | null>(null)
const requestEndpoint = ref<ModelRequestEndpoint>('chat_completions')
const requestTestModel = ref<string | null>(null)
const requestTestMessage = ref('请用一句中文回复：连接测试成功。')
const requestTestResult = ref<ModelRequestTestResponse | null>(null)
const requestTestError = ref<string | null>(null)
const isAvailableModelsLoading = ref(false)
const isRequestTesting = ref(false)
const editingApiKeyHash = ref<string | null>(null)
const apiKeyDescription = ref('VSCode')
const generatedApiKey = ref<string | null>(null)
const generatedApiKeyHash = ref<string | null>(null)
const visibleApiKeyHashes = ref<Set<string>>(new Set())

const requestLoadingText = '加载中'

interface RequestEndpointOption {
  label: string
  value: ModelRequestEndpoint
  path: string
  urlLabel: string
}

const chatCompletionsEndpointOption: RequestEndpointOption = {
  label: 'Chat Completions',
  value: 'chat_completions',
  path: '/chat/completions',
  urlLabel: 'Chat Completions URL',
}

const requestEndpointOptions: RequestEndpointOption[] = [
  chatCompletionsEndpointOption,
  {
    label: 'Responses',
    value: 'responses',
    path: '/responses',
    urlLabel: 'Responses URL',
  },
  {
    label: 'Claude Messages',
    value: 'claude_messages',
    path: '/messages',
    urlLabel: 'Claude Messages URL',
  },
]

interface ApiKeyMetricCard {
  key: string
  label: string
  value: string
  footnote: string
  tone: 'teal' | 'blue' | 'purple' | 'green'
  icon: Component
}

const apiKeyMetrics = computed<ApiKeyMetricCard[]>(() => {
  const summary = usageSummary.value
  const todayRequests = summary?.total_records ?? 0
  const failedToday = summary?.failed_records ?? 0
  const todayCost = summary?.estimated_cost_usd ?? 0
  const todayTokens = summary?.total_tokens ?? 0
  const quota = quotaStatus.value
  return [
    {
      key: 'keys',
      label: 'API 密钥',
      value: formatInteger(apiKeys.value.length),
      footnote: '当前账号',
      tone: 'teal',
      icon: KeyRound,
    },
    {
      key: 'requests',
      label: '今日请求',
      value: formatInteger(todayRequests),
      footnote: `失败 ${formatInteger(failedToday)}`,
      tone: 'blue',
      icon: Activity,
    },
    {
      key: 'tokens',
      label: '今日 Token',
      value: formatCompact(todayTokens),
      footnote: '当前账号用量',
      tone: 'purple',
      icon: Layers3,
    },
    {
      key: 'cost',
      label: '今日费用',
      value: formatUsd(todayCost),
      footnote: '按现价估算',
      tone: 'green',
      icon: CircleDollarSign,
    },
    {
      key: 'quota',
      label: '可用余额',
      value: quotaValueText(quota),
      footnote: quotaFootnote(quota),
      tone: quota?.paused ? 'purple' : 'green',
      icon: CircleDollarSign,
    },
  ]
})

const canCreateApiKey = computed(() => quotaStatus.value?.can_create_keys ?? true)

const requestBaseURL = computed(() => modelRequestGuide.value?.openai_base_url ?? requestLoadingText)
const requestEndpointMeta = computed(
  () =>
    requestEndpointOptions.find((option) => option.value === requestEndpoint.value) ??
    chatCompletionsEndpointOption,
)
const requestEndpointURL = computed(() => {
  const baseURL = modelRequestGuide.value?.openai_base_url
  if (!baseURL) {
    return requestLoadingText
  }
  return `${baseURL.replace(/\/$/, '')}${requestEndpointMeta.value.path}`
})
const requestEndpointURLLabel = computed(() => requestEndpointMeta.value.urlLabel)
const requestTestApiKeyText = computed(() => requestTestApiKey.value?.api_key || '<你的 API KEY>')
const requestHeaderLines = computed(() => {
  if (requestEndpoint.value === 'claude_messages') {
    return [`x-api-key: ${requestTestApiKeyText.value}`, 'anthropic-version: 2023-06-01']
  }
  return [`Authorization: Bearer ${requestTestApiKeyText.value}`]
})
const requestHeadersText = computed(() => requestHeaderLines.value.join('\n'))
const sampleRequest = computed(() => {
  const targetURL =
    requestEndpointURL.value === requestLoadingText
      ? `<${requestEndpointURLLabel.value}>`
      : requestEndpointURL.value
  const model = requestTestModel.value || '<模型名>'
  const content = requestTestMessage.value.trim() || '你好'
  const body = requestBodyForEndpoint(requestEndpoint.value, model, content)
  return [
    `curl ${targetURL} \\`,
    ...requestHeaderLines.value.map((header) => `  -H "${header}" \\`),
    '  -H "Content-Type: application/json" \\',
    `  -d ${quoteForCurl(JSON.stringify(body))}`,
  ].join('\n')
})
const requestTestModelOptions = computed(() => {
  const selectedHash = requestTestApiKey.value?.api_key_hash
  const models = availableModels.value?.models ?? []
  const filtered = selectedHash
    ? models.filter((model) => model.sources.some((source) => source.api_key_hash === selectedHash))
    : models
  return filtered.map((model) => ({
    label: modelOptionLabel(model),
    value: model.id,
  }))
})
const requestTestReplyText = computed(() => {
  const reply = requestTestResult.value?.reply?.trim()
  return reply || '模型返回成功，但没有可展示文本。'
})
const requestTestUsageText = computed(() => {
  const usage = requestTestResult.value?.usage
  if (!usage) {
    return ''
  }
  const input = numberFromUsage(usage.prompt_tokens ?? usage.input_tokens)
  const output = numberFromUsage(usage.completion_tokens ?? usage.output_tokens)
  const total = numberFromUsage(usage.total_tokens)
  const parts: string[] = []
  if (input !== null) {
    parts.push(`输入 ${formatInteger(input)}`)
  }
  if (output !== null) {
    parts.push(`输出 ${formatInteger(output)}`)
  }
  if (total !== null) {
    parts.push(`总计 ${formatInteger(total)}`)
  }
  return parts.join(' / ')
})

watch(requestEndpoint, () => {
  requestTestResult.value = null
  requestTestError.value = null
})

function requestBodyForEndpoint(
  endpoint: ModelRequestEndpoint,
  model: string,
  content: string,
): Record<string, unknown> {
  if (endpoint === 'responses') {
    return {
      model,
      input: content,
      stream: false,
    }
  }
  if (endpoint === 'claude_messages') {
    return {
      model,
      max_tokens: 1024,
      messages: [{ role: 'user', content }],
    }
  }
  return {
    model,
    messages: [{ role: 'user', content }],
    stream: false,
  }
}

function quoteForCurl(value: string): string {
  return "'" + value.replace(/'/g, "'\"'\"'") + "'"
}

function quotaValueText(quota: UserQuotaStatus | null): string {
  if (!quota) {
    return '加载中'
  }
  if (quota.unlimited) {
    return '每月余额 无限制'
  }
  return `每月余额 ${formatUsd(quota.monthly_remaining_usd ?? 0)}`
}

function quotaFootnote(quota: UserQuotaStatus | null): string {
  if (!quota) {
    return '额度加载中'
  }
  if (quota.unlimited) {
    return '不限时余额 无限制'
  }
  const lifetimeText = `不限时余额 ${formatUsd(quota.lifetime_remaining_usd ?? 0)}`
  const notes: string[] = []
  if (quota.sync_error) {
    notes.push('Key 同步异常')
  }
  if (quota.unpriced_records > 0) {
    notes.push(`未定价 ${formatInteger(quota.unpriced_records)} 条`)
  }
  if (quota.paused) {
    notes.push('Key 已因余额暂停')
  }
  return notes.length > 0 ? `${lifetimeText} · ${notes.join(' · ')}` : lifetimeText
}

function modelOptionLabel(model: AvailableModel): string {
  return model.id
}

function numberFromUsage(value: unknown): number | null {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return null
  }
  return value
}

function ensureRequestTestModel() {
  const options = requestTestModelOptions.value
  if (options.length === 0) {
    requestTestModel.value = null
    return
  }
  if (!requestTestModel.value || !options.some((option) => option.value === requestTestModel.value)) {
    const firstOption = options[0]
    requestTestModel.value = firstOption ? firstOption.value : null
  }
}

function displayedApiKey(row: UserApiKeySummary): string {
  if (row.api_key && isApiKeyVisible(row)) {
    return row.api_key
  }
  return maskDisplayedApiKey(row.api_key)
}

function maskDisplayedApiKey(apiKey: string | null | undefined): string {
  if (!apiKey) {
    return '未知'
  }
  if (apiKey.length <= 12) {
    return `${apiKey.slice(0, 3)}${'*'.repeat(Math.max(apiKey.length - 3, 0))}`
  }
  const visiblePrefix = apiKey.startsWith('sk-') ? 4 : 6
  const visibleSuffix = 4
  const maskedLength = Math.max(apiKey.length - visiblePrefix - visibleSuffix, 8)
  return `${apiKey.slice(0, visiblePrefix)}${'*'.repeat(maskedLength)}${apiKey.slice(-visibleSuffix)}`
}

function renderMaskedKeyTitle() {
  return h('span', { class: 'api-key-title' }, [
    h(NIcon, { class: 'api-key-mask-icon', component: EyeOff }),
    h('span', '密钥（点击复制）'),
  ])
}

function isApiKeyVisible(row: UserApiKeySummary): boolean {
  return visibleApiKeyHashes.value.has(row.api_key_hash)
}

function toggleApiKeyVisibility(row: UserApiKeySummary) {
  if (!row.api_key) {
    message.info('当前没有完整密钥可显示')
    return
  }
  const nextVisible = new Set(visibleApiKeyHashes.value)
  if (nextVisible.has(row.api_key_hash)) {
    nextVisible.delete(row.api_key_hash)
  } else {
    nextVisible.add(row.api_key_hash)
  }
  visibleApiKeyHashes.value = nextVisible
}

async function copyApiKey(row: UserApiKeySummary) {
  try {
    if (!row.api_key) {
      message.info('当前没有完整密钥可复制')
      return
    }
    await copyToClipboard(row.api_key)
    message.success('API 密钥已复制')
  } catch (error) {
    message.error(error instanceof Error ? error.message : '复制失败')
  }
}

async function copyGeneratedApiKey() {
  if (!generatedApiKey.value) {
    return
  }
  try {
    await copyToClipboard(generatedApiKey.value)
    message.success('API 密钥已复制')
  } catch (error) {
    message.error(error instanceof Error ? error.message : '复制失败')
  }
}

async function loadModelRequestGuide() {
  try {
    modelRequestGuide.value = await getModelRequestGuide()
  } catch (error) {
    message.error(error instanceof Error ? error.message : '加载请求地址失败')
  }
}

async function loadAvailableModelsForTest() {
  isAvailableModelsLoading.value = true
  try {
    availableModels.value = await listAvailableModels()
    ensureRequestTestModel()
  } catch (error) {
    message.error(error instanceof Error ? error.message : '加载可用模型失败')
  } finally {
    isAvailableModelsLoading.value = false
  }
}

function openRequestTest(row: UserApiKeySummary) {
  requestTestApiKey.value = row
  requestTestModel.value = row.last_model ?? row.models[0] ?? null
  requestTestResult.value = null
  requestTestError.value = null
  requestTestVisible.value = true
  if (!modelRequestGuide.value) {
    void loadModelRequestGuide()
  }
  if (!availableModels.value) {
    void loadAvailableModelsForTest()
  } else {
    ensureRequestTestModel()
  }
}

async function copyRequestValue(label: string, value: string) {
  if (!value || value === requestLoadingText) {
    return
  }
  try {
    await copyToClipboard(value)
    message.success(`${label} 已复制`)
  } catch (error) {
    message.error(error instanceof Error ? error.message : '复制失败')
  }
}

async function runRequestTest() {
  if (isRequestTesting.value) {
    return
  }
  const currentKey = requestTestApiKey.value
  const model = requestTestModel.value?.trim() ?? ''
  if (!currentKey) {
    message.error('请选择要测试的 API KEY')
    return
  }
  if (!model) {
    message.error('请选择测试模型')
    return
  }
  isRequestTesting.value = true
  requestTestResult.value = null
  requestTestError.value = null
  try {
    requestTestResult.value = await testModelRequest({
      api_key_hash: currentKey.api_key_hash,
      endpoint: requestEndpoint.value,
      model,
      message: requestTestMessage.value,
    })
    message.success('请求测试完成')
  } catch (error) {
    requestTestError.value = error instanceof Error ? error.message : '请求测试失败'
  } finally {
    isRequestTesting.value = false
  }
}

function openCreateDialog() {
  if (!canCreateApiKey.value) {
    message.error('当前账号额度已用尽，API KEY 已暂停')
    return
  }
  editingApiKeyHash.value = null
  apiKeyDescription.value = 'VSCode'
  generatedApiKey.value = null
  generatedApiKeyHash.value = null
  editorVisible.value = true
}

function closeGeneratedApiKey() {
  generatedApiKey.value = null
  generatedApiKeyHash.value = null
}

function editApiKey(row: UserApiKeySummary) {
  editingApiKeyHash.value = row.api_key_hash
  apiKeyDescription.value = row.description || 'VSCode'
  generatedApiKey.value = null
  generatedApiKeyHash.value = null
  editorVisible.value = true
}

async function refresh() {
  isLoading.value = true
  try {
    const [nextApiKeys, overview, quota, guide] = await Promise.all([
      listApiKeys(),
      getUsageOverview({ scope: 'account' }),
      getCurrentUserQuota(),
      getModelRequestGuide(),
    ])
    apiKeys.value = nextApiKeys
    usageSummary.value = overview.summary
    quotaStatus.value = quota
    modelRequestGuide.value = guide
    if (editingApiKeyHash.value) {
      const current = apiKeys.value.find((item) => item.api_key_hash === editingApiKeyHash.value)
      if (!current) {
        editorVisible.value = false
        editingApiKeyHash.value = null
      }
    }
  } catch (error) {
    message.error(error instanceof Error ? error.message : '加载 API 密钥失败')
  } finally {
    isLoading.value = false
  }
}

async function saveApiKey() {
  if (isSaving.value) {
    return
  }
  const description = apiKeyDescription.value.trim()
  if (!description) {
    message.error('API KEY 描述不能为空')
    return
  }
  isSaving.value = true
  try {
    if (editingApiKeyHash.value) {
      await updateApiKey(editingApiKeyHash.value, { description })
      message.success('API 密钥已更新')
    } else {
      if (!canCreateApiKey.value) {
        message.error('当前账号额度已用尽，API KEY 已暂停')
        return
      }
      const created = await createApiKey({ description })
      generatedApiKey.value = created.api_key ?? null
      generatedApiKeyHash.value = created.api_key_hash
      message.success('API 密钥已创建并同步到 CPA')
    }
    editorVisible.value = false
    editingApiKeyHash.value = null
    await refresh()
  } catch (error) {
    message.error(error instanceof Error ? error.message : '保存 API 密钥失败')
  } finally {
    isSaving.value = false
  }
}

function confirmDelete(row: UserApiKeySummary) {
  dialog.warning({
    title: '删除 API 密钥',
    content: `将删除 ${row.description || '未命名'} 对应的密钥，并从 CPA 中移除。`,
    positiveText: '删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      await deleteApiKey(row.api_key_hash)
      message.success('API 密钥已删除')
      if (editingApiKeyHash.value === row.api_key_hash) {
        editorVisible.value = false
        editingApiKeyHash.value = null
      }
      if (generatedApiKeyHash.value === row.api_key_hash) {
        generatedApiKey.value = null
        generatedApiKeyHash.value = null
      }
      await refresh()
    },
  })
}

const columns: DataTableColumns<UserApiKeySummary> = [
  {
    title: renderMaskedKeyTitle,
    key: 'api_key',
    width: 430,
    render: (row) =>
      h(
        'div',
        { class: 'api-key-cell' },
        [
          h(
            'button',
            {
              class: 'api-key-visibility-button',
              disabled: !row.api_key,
              title: isApiKeyVisible(row) ? '隐藏完整密钥' : '显示完整密钥',
              type: 'button',
              onClick: () => toggleApiKeyVisibility(row),
            },
            [
              h(NIcon, {
                class: 'api-key-mask-icon',
                component: isApiKeyVisible(row) ? Eye : EyeOff,
              }),
            ],
          ),
          h(
            'button',
            {
              class: 'api-key-copy-button',
              type: 'button',
              title: row.api_key ? '点击复制完整密钥' : '无完整密钥可复制',
              onClick: () => copyApiKey(row),
            },
            h('span', { class: 'api-key-mask-text' }, displayedApiKey(row)),
          ),
        ],
      ),
  },
  {
    title: '描述',
    key: 'description',
    width: 240,
    render: (row) => row.description || '-',
  },
  {
    title: '创建时间',
    key: 'created_at',
    width: 180,
    render: (row) => formatDateTime(row.created_at),
  },
  {
    title: '',
    key: 'actions',
    width: 230,
    fixed: 'right',
    render: (row) =>
      h(NSpace, { size: 4 }, {
        default: () => [
          h(
            NButton,
            { size: 'small', quaternary: true, onClick: () => openRequestTest(row) },
            {
              icon: () => h(NIcon, { component: Send }),
              default: () => '请求测试',
            },
          ),
          h(
            NButton,
            { size: 'small', quaternary: true, onClick: () => editApiKey(row) },
            { default: () => '编辑' },
          ),
          h(
            NButton,
            { size: 'small', quaternary: true, type: 'error', onClick: () => confirmDelete(row) },
            { default: () => '删除' },
          ),
        ],
      }),
  },
]

onMounted(refresh)
</script>

<template>
  <section class="page">
    <div class="page-header">
      <div>
        <h1 class="page-title">API 密钥</h1>
        <p class="page-subtitle">仅管理当前账号自己的密钥</p>
      </div>
      <NSpace>
        <NButton secondary :loading="isLoading" @click="refresh">刷新</NButton>
        <NButton type="primary" :disabled="!canCreateApiKey" @click="openCreateDialog">
          新建 API 密钥
        </NButton>
      </NSpace>
    </div>

    <div class="metric-grid api-key-metrics">
      <div v-for="metric in apiKeyMetrics" :key="metric.key" class="metric-card" :class="`is-${metric.tone}`">
        <div class="metric-icon" aria-hidden="true">
          <component :is="metric.icon" :size="20" :stroke-width="2.2" />
        </div>
        <div class="metric-label">{{ metric.label }}</div>
        <div class="metric-value">{{ metric.value }}</div>
        <div class="metric-footnote">{{ metric.footnote }}</div>
      </div>
    </div>

    <section class="panel api-key-panel-shell">
      <div class="panel-inner api-key-panel">
        <NAlert type="warning" :bordered="false" title="请求链路说明">
          Agent 发起的模型请求仍需 Agent 直接发送到 CPA，CPA-Helper 不代理或中转这些请求；仅调用 CPA
          的 usage 队列、API KEY 创建与删除、凭证管理等接口，用于用量查看、密钥创建和凭证维护。API
          密钥拥有当前账号的完整权限，请妥善保管。
        </NAlert>

        <NAlert v-if="quotaStatus?.paused" type="error" :bordered="false" title="额度已用尽">
          当前账号 API KEY 已从 CPA 暂停。补充额度或进入新月份恢复月额度后，系统会自动恢复可用 Key。
        </NAlert>
        <NAlert v-else-if="quotaStatus?.unpriced_records" type="warning" :bordered="false">
          当前账号存在 {{ formatInteger(quotaStatus.unpriced_records) }} 条未定价用量，未计入额度扣减。
        </NAlert>

        <div v-if="generatedApiKey" class="generated-key-box">
          <div class="generated-key-main">
            <div class="generated-key-title">新创建的密钥</div>
            <div class="generated-key-value">{{ generatedApiKey }}</div>
          </div>
          <NSpace>
            <NButton secondary @click="copyGeneratedApiKey">复制</NButton>
            <NButton tertiary @click="closeGeneratedApiKey">关闭</NButton>
          </NSpace>
        </div>

        <NDataTable
          class="api-key-table"
          size="small"
          :loading="isLoading"
          :columns="columns"
          :data="apiKeys"
          :pagination="{ pageSize: 12 }"
          table-layout="fixed"
          :scroll-x="1080"
        />
      </div>
    </section>

    <NModal
      v-model:show="editorVisible"
      preset="card"
      :mask-closable="false"
      :closable="false"
      :title="editingApiKeyHash ? '编辑 API 密钥' : '新建 API 密钥'"
      :style="{ width: 'min(520px, calc(100vw - 32px))' }"
    >
      <NForm label-placement="top">
        <NFormItem label="API KEY 描述">
          <NInput
            v-model:value="apiKeyDescription"
            :disabled="isSaving"
            placeholder="例如：VSCode"
            @keyup.enter="saveApiKey"
          />
        </NFormItem>
        <div class="modal-actions">
          <NButton secondary :disabled="isSaving" @click="editorVisible = false">取消</NButton>
          <NButton
            type="primary"
            :loading="isSaving"
            :disabled="isSaving || (!editingApiKeyHash && !canCreateApiKey)"
            @click="saveApiKey"
          >
            {{ editingApiKeyHash ? '保存' : '创建' }}
          </NButton>
        </div>
      </NForm>
    </NModal>

    <NModal
      v-model:show="requestTestVisible"
      preset="card"
      title="请求测试"
      :style="{ width: 'min(760px, calc(100vw - 32px))' }"
    >
      <div class="request-test">
        <NAlert type="info" :bordered="false">
          这里提供当前 API KEY 的请求说明，也可以直接选择模型发起一次真实测试。
        </NAlert>

        <div class="request-endpoint-switch">
          <span class="request-endpoint-label">请求格式</span>
          <NRadioGroup v-model:value="requestEndpoint" size="small">
            <NRadioButton
              v-for="option in requestEndpointOptions"
              :key="option.value"
              :value="option.value"
            >
              {{ option.label }}
            </NRadioButton>
          </NRadioGroup>
        </div>

        <div class="request-guide-list">
          <div class="request-guide-row">
            <div>
              <div class="request-guide-label">Base URL</div>
              <code class="request-guide-value">{{ requestBaseURL }}</code>
            </div>
            <NButton size="small" secondary @click="copyRequestValue('Base URL', requestBaseURL)">
              <template #icon>
                <NIcon :component="Copy" />
              </template>
              复制
            </NButton>
          </div>
          <div class="request-guide-row">
            <div>
              <div class="request-guide-label">{{ requestEndpointURLLabel }}</div>
              <code class="request-guide-value">{{ requestEndpointURL }}</code>
            </div>
            <NButton size="small" secondary @click="copyRequestValue('请求 URL', requestEndpointURL)">
              <template #icon>
                <NIcon :component="Copy" />
              </template>
              复制
            </NButton>
          </div>
          <div class="request-guide-row">
            <div>
              <div class="request-guide-label">Header</div>
              <code class="request-guide-value request-guide-value-multiline">{{ requestHeadersText }}</code>
            </div>
            <NButton size="small" secondary @click="copyRequestValue('Header', requestHeadersText)">
              <template #icon>
                <NIcon :component="Copy" />
              </template>
              复制
            </NButton>
          </div>
        </div>

        <div class="request-example">
          <div class="request-example-head">
            <span>curl 示例</span>
            <NButton size="small" secondary @click="copyRequestValue('curl 示例', sampleRequest)">
              <template #icon>
                <NIcon :component="Copy" />
              </template>
              复制示例
            </NButton>
          </div>
          <pre>{{ sampleRequest }}</pre>
        </div>

        <div class="request-test-section-title">请求测试</div>

        <NForm label-placement="top" class="request-test-form">
          <NFormItem label="测试模型">
            <NSelect
              v-model:value="requestTestModel"
              filterable
              clearable
              :loading="isAvailableModelsLoading"
              :options="requestTestModelOptions"
              placeholder="选择当前 Key 可用的模型"
            />
          </NFormItem>
          <NFormItem label="测试消息">
            <NInput
              v-model:value="requestTestMessage"
              type="textarea"
              :autosize="{ minRows: 3, maxRows: 5 }"
              placeholder="输入要发送给模型的测试消息"
            />
          </NFormItem>
        </NForm>

        <NAlert
          v-if="!isAvailableModelsLoading && requestTestModelOptions.length === 0"
          type="warning"
          :bordered="false"
        >
          当前 Key 暂未查询到可选模型，可以先刷新模型列表，或到「可用模型」页面检查 Key 是否可用。
        </NAlert>

        <div class="modal-actions request-test-actions">
          <NButton secondary :loading="isAvailableModelsLoading" @click="loadAvailableModelsForTest">
            刷新模型
          </NButton>
          <NButton
            type="primary"
            :loading="isRequestTesting"
            :disabled="!requestTestModel || isAvailableModelsLoading"
            @click="runRequestTest"
          >
            <template #icon>
              <NIcon :component="Send" />
            </template>
            发送测试
          </NButton>
        </div>

        <NAlert v-if="requestTestError" type="error" :bordered="false">
          {{ requestTestError }}
        </NAlert>

        <div v-if="requestTestResult" class="request-test-result">
          <div class="request-test-result-head">
            <span>模型回复</span>
            <span>
              HTTP {{ requestTestResult.status_code }} · {{ requestTestResult.duration_ms }}ms
              <template v-if="requestTestUsageText"> · {{ requestTestUsageText }}</template>
            </span>
          </div>
          <pre>{{ requestTestReplyText }}</pre>
        </div>
      </div>
    </NModal>
  </section>
</template>

<style scoped>
.api-key-panel {
  display: grid;
  gap: 14px;
  min-width: 0;
}

.api-key-metrics {
  grid-template-columns: repeat(5, minmax(0, 1fr));
}

.api-key-panel-shell,
.api-key-table {
  min-width: 0;
  min-height: 0;
}

.api-key-table :deep(.n-data-table-wrapper) {
  overflow: hidden;
}

.generated-key-box {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  min-width: 0;
  padding: 16px;
  border: 1px solid var(--cpa-border);
  border-radius: var(--cpa-radius);
  background:
    linear-gradient(135deg, rgb(0 154 168 / 10%), rgb(29 141 255 / 7%)),
    var(--cpa-primary-wash);
  box-shadow: var(--cpa-shadow-hairline);
}

.generated-key-main {
  min-width: 0;
}

.generated-key-title {
  margin-bottom: 4px;
  font-weight: 700;
}

.generated-key-value {
  overflow-wrap: anywhere;
  font-family: Consolas, 'SFMono-Regular', 'Microsoft YaHei UI', monospace;
  font-size: 13px;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.request-test {
  display: grid;
  gap: 14px;
}

.request-endpoint-switch {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 10px 12px;
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid var(--cpa-border);
  border-radius: var(--cpa-radius);
  background: var(--cpa-surface-muted);
}

.request-endpoint-label {
  color: var(--cpa-text-muted);
  font-size: 12px;
  font-weight: 700;
}

.request-guide-list {
  display: grid;
  overflow: hidden;
  border: 1px solid var(--cpa-border);
  border-radius: var(--cpa-radius);
  background: var(--cpa-surface);
}

.request-guide-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 12px;
  min-width: 0;
  padding: 12px 14px;
  border-bottom: 1px solid var(--cpa-border);
}

.request-guide-row:last-child {
  border-bottom: 0;
}

.request-guide-label {
  margin-bottom: 4px;
  color: var(--cpa-text-muted);
  font-size: 12px;
  font-weight: 700;
}

.request-guide-value {
  display: block;
  min-width: 0;
  overflow-wrap: anywhere;
  color: var(--cpa-text);
  font-family: Consolas, 'SFMono-Regular', 'Microsoft YaHei UI', monospace;
  font-size: 13px;
  line-height: 1.45;
}

.request-guide-value-multiline {
  white-space: pre-wrap;
}

.request-example,
.request-test-form {
  display: grid;
}

.request-example {
  overflow: hidden;
  border: 1px solid var(--cpa-border);
  border-radius: var(--cpa-radius);
  background: var(--cpa-surface);
}

.request-example-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--cpa-border);
  font-weight: 700;
}

.request-example pre {
  overflow: auto;
  margin: 0;
  padding: 14px;
  background: var(--cpa-surface-muted);
  color: var(--cpa-text);
  font-family: Consolas, 'SFMono-Regular', 'Microsoft YaHei UI', monospace;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
}

.request-test-section-title {
  color: var(--cpa-text);
  font-size: 14px;
  font-weight: 700;
}

.request-test-form {
  gap: 2px;
}

.request-test-actions {
  align-items: center;
}

.request-test-result {
  overflow: hidden;
  border: 1px solid var(--cpa-border);
  border-radius: var(--cpa-radius);
  background: var(--cpa-surface);
}

.request-test-result-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 14px;
  border-bottom: 1px solid var(--cpa-border);
  font-weight: 700;
}

.request-test-result-head span:last-child {
  color: var(--cpa-text-muted);
  font-size: 12px;
  font-weight: 500;
}

.request-test-result pre {
  overflow: auto;
  margin: 0;
  padding: 14px;
  background: var(--cpa-surface-muted);
  color: var(--cpa-text);
  font-family: Consolas, 'SFMono-Regular', 'Microsoft YaHei UI', monospace;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre-wrap;
}

:global(.api-key-cell) {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  min-width: 0;
}

:global(.api-key-visibility-button),
:global(.api-key-copy-button) {
  border: 0;
  background: transparent;
  color: var(--cpa-text);
  font: inherit;
  cursor: pointer;
}

:global(.api-key-visibility-button) {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  padding: 0;
  border-radius: 6px;
}

:global(.api-key-copy-button) {
  min-width: 0;
  flex: 1 1 auto;
  overflow: hidden;
  padding: 0;
  line-height: 1.35;
  text-align: left;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:global(.api-key-title) {
  display: inline-flex;
  align-items: center;
  gap: 12px;
}

:global(.api-key-mask-icon) {
  flex: 0 0 auto;
  color: var(--cpa-text-muted);
}

:global(.api-key-mask-text) {
  display: block;
  min-width: 0;
  overflow: hidden;
  font-family: Consolas, 'SFMono-Regular', 'Microsoft YaHei UI', monospace;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:global(.api-key-visibility-button:hover),
:global(.api-key-visibility-button:focus-visible),
:global(.api-key-copy-button:hover),
:global(.api-key-copy-button:focus-visible) {
  color: var(--cpa-primary);
}

:global(.api-key-visibility-button:disabled) {
  color: var(--cpa-text-muted);
  cursor: not-allowed;
  opacity: 0.56;
}

:global(.api-key-visibility-button:focus-visible),
:global(.api-key-copy-button:focus-visible) {
  outline: 2px solid color-mix(in srgb, var(--cpa-primary) 32%, transparent);
  outline-offset: 2px;
}

@media (max-width: 900px) {
  .api-key-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .generated-key-box {
    flex-direction: column;
  }

  .request-guide-row {
    grid-template-columns: 1fr;
  }

  .request-example-head,
  .request-test-result-head {
    align-items: flex-start;
    flex-direction: column;
  }
}

@media (max-width: 720px) {
  .api-key-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 430px) {
  .api-key-metrics {
    grid-template-columns: 1fr;
  }
}
</style>
