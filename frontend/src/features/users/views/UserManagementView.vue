<script setup lang="ts">
import type { Component } from 'vue'
import { computed, h, onMounted, ref } from 'vue'
import {
  NAlert,
  NButton,
  NDataTable,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NModal,
  NPopconfirm,
  NSpace,
  NSwitch,
  NTag,
  useMessage,
  type DataTableColumns,
} from 'naive-ui'
import { CircleDollarSign, KeyRound, ShieldCheck, UserRound } from 'lucide-vue-next'

import {
  createUser,
  disableUser,
  enableUser,
  listUsers,
  updateUser,
  updateUserQuota,
} from '@/features/users/api/usersApi'
import type { UserSummary } from '@/shared/types/api'
import { formatCompact, formatDateTime, formatInteger, formatUsd } from '@/shared/utils/format'

const message = useMessage()
const isLoading = ref(false)
const isSavingUser = ref(false)
const users = ref<UserSummary[]>([])
const editorVisible = ref(false)
const editingUserId = ref<number | null>(null)
const userAccount = ref('')
const userPassword = ref('')
const isUserAdmin = ref(false)
const userNickname = ref('')
const quotaUnlimited = ref(true)
const quotaLifetimeUsd = ref(0)
const quotaMonthlyUsd = ref(0)
const isEditingFirstUser = computed(() => editingUserId.value === 1)

interface UserMetricCard {
  key: string
  label: string
  value: string
  footnote: string
  tone: 'teal' | 'blue' | 'purple' | 'green'
  icon: Component
}

const userMetrics = computed<UserMetricCard[]>(() => {
  const activeUsers = users.value.filter((user) => user.disabled_at === null).length
  const adminUsers = users.value.filter((user) => user.is_admin).length
  const boundKeys = users.value.reduce((total, user) => total + user.key_count, 0)
  const todayCost = users.value.reduce((total, user) => total + user.today_estimated_cost_usd, 0)
  return [
    {
      key: 'active',
      label: '启用用户',
      value: formatInteger(activeUsers),
      footnote: `共 ${formatInteger(users.value.length)} 个账号`,
      tone: 'teal',
      icon: UserRound,
    },
    {
      key: 'admins',
      label: '管理员',
      value: formatInteger(adminUsers),
      footnote: '拥有管理权限',
      tone: 'purple',
      icon: ShieldCheck,
    },
    {
      key: 'keys',
      label: '绑定 Key',
      value: formatInteger(boundKeys),
      footnote: '当前用户集合',
      tone: 'blue',
      icon: KeyRound,
    },
    {
      key: 'cost',
      label: '今日费用',
      value: formatUsd(todayCost),
      footnote: '按现价估算',
      tone: 'green',
      icon: CircleDollarSign,
    },
  ]
})

function userLabel(row: UserSummary): string {
  return row.nickname.trim() || row.username.trim() || '未知用户'
}

function quotaBalanceValue(row: UserSummary, bucket: 'monthly' | 'lifetime'): string {
  if (row.quota.unlimited) {
    return '无限制'
  }

  const value = bucket === 'monthly' ? row.quota.monthly_remaining_usd : row.quota.lifetime_remaining_usd
  return formatUsd(value)
}

function quotaBalanceClass(row: UserSummary): string {
  if (row.quota.paused || !row.quota.can_create_keys) {
    return 'is-error'
  }
  if (row.quota.unpriced_records > 0) {
    return 'is-warning'
  }
  return row.quota.unlimited ? 'is-unlimited' : 'is-normal'
}

function quotaDetail(row: UserSummary): string | null {
  if (row.quota.paused) {
    return 'Key 已因余额暂停'
  }
  if (row.quota.sync_error) {
    return '同步异常'
  }
  if (row.quota.unpriced_records > 0) {
    return `未定价 ${formatInteger(row.quota.unpriced_records)} 条`
  }
  return null
}

function todayRequestDetail(row: UserSummary): string {
  if (!row.today_records) {
    return `累计 ${formatInteger(row.records)}`
  }
  const failed = row.today_failed_records
    ? `失败 ${formatInteger(row.today_failed_records)}`
    : '无失败'
  const rate = Math.round((row.today_success_records / row.today_records) * 100)
  return `${rate}% · ${failed}`
}

function todayCostDetail(row: UserSummary): string {
  if (!row.today_records) {
    return '今日无请求'
  }
  if (row.today_unpriced_records > 0) {
    return `未计价 ${formatInteger(row.today_unpriced_records)}`
  }
  return '已计价'
}

function lastModelLabel(row: UserSummary): string {
  return row.last_model ?? '-'
}

function lastProviderLabel(row: UserSummary): string {
  return row.last_provider ?? '未知服务商'
}

function setQuotaLifetimeUsd(value: number | null) {
  quotaLifetimeUsd.value = value ?? 0
}

function setQuotaMonthlyUsd(value: number | null) {
  quotaMonthlyUsd.value = value ?? 0
}

function resetEditor() {
  editingUserId.value = null
  userAccount.value = ''
  userPassword.value = ''
  isUserAdmin.value = false
  userNickname.value = ''
  quotaUnlimited.value = true
  quotaLifetimeUsd.value = 0
  quotaMonthlyUsd.value = 0
}

function openCreateUser() {
  resetEditor()
  userPassword.value = 'password'
  editorVisible.value = true
}

function editUser(row: UserSummary) {
  editingUserId.value = row.id
  userAccount.value = row.username
  userPassword.value = ''
  isUserAdmin.value = row.id === 1 ? true : row.is_admin
  userNickname.value = row.nickname
  quotaUnlimited.value = row.quota.unlimited
  quotaLifetimeUsd.value = row.quota.lifetime_quota_usd ?? 0
  quotaMonthlyUsd.value = row.quota.monthly_quota_usd ?? 0
  editorVisible.value = true
}

async function refresh() {
  isLoading.value = true
  try {
    users.value = await listUsers()
  } catch (error) {
    message.error(error instanceof Error ? error.message : '加载用户列表失败')
  } finally {
    isLoading.value = false
  }
}

function isUserDisabled(row: UserSummary): boolean {
  return row.disabled_at !== null
}

async function disableUserRow(row: UserSummary) {
  try {
    await disableUser(row.id)
    message.success('用户已禁用')
    await refresh()
  } catch (error) {
    message.error(error instanceof Error ? error.message : '禁用用户失败')
  }
}

async function enableUserRow(row: UserSummary) {
  try {
    await enableUser(row.id)
    message.success('用户已启用')
    await refresh()
  } catch (error) {
    message.error(error instanceof Error ? error.message : '启用用户失败')
  }
}

async function saveUser() {
  const nickname = userNickname.value.trim()
  if (!nickname) {
    message.error('用户昵称不能为空')
    return
  }
  const username = userAccount.value.trim()
  if (!username) {
    message.error('账号不能为空')
    return
  }
  const isEditing = editingUserId.value !== null
  const password = userPassword.value.trim()
  if (!isEditing && !password) {
    message.error('密码不能为空')
    return
  }
  isSavingUser.value = true
  try {
    const payload = {
      username,
      password: password || undefined,
      is_admin: isEditingFirstUser.value ? true : isUserAdmin.value,
      nickname,
    }
    const saved =
      editingUserId.value !== null
        ? await updateUser(editingUserId.value, payload)
        : await createUser(payload)
    await updateUserQuota(saved.id, {
      lifetime_quota_usd: quotaUnlimited.value ? null : quotaLifetimeUsd.value,
      monthly_quota_usd: quotaUnlimited.value ? null : quotaMonthlyUsd.value,
    })
    message.success(isEditing ? '用户已保存' : '用户已创建')
    editorVisible.value = false
    resetEditor()
    await refresh()
  } catch (error) {
    message.error(error instanceof Error ? error.message : '保存用户失败')
  } finally {
    isSavingUser.value = false
  }
}

const columns: DataTableColumns<UserSummary> = [
  {
    title: '用户昵称',
    key: 'nickname',
    width: 120,
    render: (row) => userLabel(row),
  },
  {
    title: '账号',
    key: 'username',
    width: 130,
    render: (row) => row.username,
  },
  {
    title: '角色',
    key: 'is_admin',
    width: 90,
    render: (row) => (row.is_admin ? '管理员' : '普通用户'),
  },
  {
    title: '状态',
    key: 'disabled_at',
    width: 90,
    render: (row) =>
      h(
        NTag,
        {
          size: 'small',
          type: isUserDisabled(row) ? 'warning' : 'success',
          bordered: false,
        },
        { default: () => (isUserDisabled(row) ? '已禁用' : '启用中') },
      ),
  },
  {
    title: '余额',
    key: 'quota',
    width: 210,
    render: (row) => {
      const detail = quotaDetail(row)
      return h('div', { class: ['metric-stack', 'quota-balance-stack'] }, [
        h('div', { class: ['quota-balance-row', 'is-monthly', quotaBalanceClass(row)] }, [
          h('span', { class: 'quota-balance-label' }, '每月余额：'),
          h('strong', { class: 'quota-balance-value' }, quotaBalanceValue(row, 'monthly')),
        ]),
        h('div', { class: ['quota-balance-row', 'is-lifetime', quotaBalanceClass(row)] }, [
          h('span', { class: 'quota-balance-label' }, '不限时余额：'),
          h('strong', { class: 'quota-balance-value' }, quotaBalanceValue(row, 'lifetime')),
        ]),
        ...(detail
          ? [
              h(
                'span',
                { class: ['metric-muted', 'quota-balance-detail', { 'is-error': row.quota.sync_error || row.quota.paused }] },
                detail,
              ),
            ]
          : []),
      ])
    },
  },
  {
    title: 'API KEY',
    key: 'key_count',
    width: 95,
    render: (row) => `${formatInteger(row.key_count)} 个`,
  },
  {
    title: '今日请求',
    key: 'today_records',
    width: 140,
    render: (row) =>
      h('div', { class: 'metric-stack' }, [
        h('span', { class: 'metric-primary' }, formatInteger(row.today_records)),
        h('span', { class: 'metric-muted' }, todayRequestDetail(row)),
      ]),
  },
  {
    title: '今日输入',
    key: 'today_input_tokens',
    width: 120,
    render: (row) => formatCompact(row.today_input_tokens),
  },
  {
    title: '今日输出',
    key: 'today_output_tokens',
    width: 120,
    render: (row) => formatCompact(row.today_output_tokens),
  },
  {
    title: '今日缓存',
    key: 'today_cached_tokens',
    width: 120,
    render: (row) => formatCompact(row.today_cached_tokens),
  },
  {
    title: '今日总 Token',
    key: 'today_total_tokens',
    width: 145,
    render: (row) => formatCompact(row.today_total_tokens),
  },
  {
    title: '今日费用',
    key: 'today_estimated_cost_usd',
    width: 150,
    render: (row) =>
      h('div', { class: 'metric-stack' }, [
        h('span', { class: 'metric-primary' }, formatUsd(row.today_estimated_cost_usd)),
        h(
          'span',
          { class: ['metric-muted', { 'is-error': row.today_unpriced_records > 0 }] },
          todayCostDetail(row),
        ),
      ]),
  },
  {
    title: '最近模型',
    key: 'last_model',
    width: 160,
    render: (row) =>
      h('div', { class: 'metric-stack' }, [
        h('span', { class: 'model-value' }, lastModelLabel(row)),
        h('span', { class: 'metric-muted' }, lastProviderLabel(row)),
      ]),
  },
  {
    title: '最近使用',
    key: 'last_seen_at',
    width: 150,
    render: (row) => formatDateTime(row.last_seen_at),
  },
  {
    title: '',
    key: 'actions',
    width: 90,
    fixed: 'right',
    render: (row) =>
      h(
        NSpace,
        { size: 4 },
        {
          default: () => [
            h(
              NButton,
              { size: 'small', quaternary: true, onClick: () => editUser(row) },
              { default: () => '编辑' },
            ),
            row.id === 1
              ? null
              : isUserDisabled(row)
                ? h(
                    NPopconfirm,
                    { onPositiveClick: () => enableUserRow(row) },
                    {
                      trigger: () =>
                        h(
                          NButton,
                          { size: 'small', quaternary: true, type: 'primary' },
                          { default: () => '启用' },
                        ),
                      default: () => `启用用户 ${userLabel(row)} 并恢复其 API KEY？`,
                    },
                  )
                : h(
                    NPopconfirm,
                    { onPositiveClick: () => disableUserRow(row) },
                    {
                      trigger: () =>
                        h(
                          NButton,
                          { size: 'small', quaternary: true, type: 'warning' },
                          { default: () => '禁用' },
                        ),
                      default: () => `禁用用户 ${userLabel(row)} 并从 CPA 移除其 API KEY？`,
                    },
                  ),
          ],
        },
      ),
  },
]

onMounted(refresh)
</script>

<template>
  <section class="page">
    <div class="page-header">
      <div>
        <h1 class="page-title">用户管理</h1>
        <p class="page-subtitle">管理用户昵称、登录账号、密码和角色</p>
      </div>
      <NSpace>
        <NButton secondary :loading="isLoading" @click="refresh">刷新</NButton>
        <NButton type="primary" @click="openCreateUser">增加用户</NButton>
      </NSpace>
    </div>

    <div class="metric-grid user-metrics">
      <div v-for="metric in userMetrics" :key="metric.key" class="metric-card" :class="`is-${metric.tone}`">
        <div class="metric-icon" aria-hidden="true">
          <component :is="metric.icon" :size="20" :stroke-width="2.2" />
        </div>
        <div class="metric-label">{{ metric.label }}</div>
        <div class="metric-value">{{ metric.value }}</div>
        <div class="metric-footnote">{{ metric.footnote }}</div>
      </div>
    </div>

    <section class="panel table-panel">
      <NDataTable
        size="small"
        :loading="isLoading"
        :columns="columns"
        :data="users"
        :pagination="{ pageSize: 12 }"
        table-layout="fixed"
        :scroll-x="2000"
      />
    </section>

    <NModal
      v-model:show="editorVisible"
      preset="card"
      :mask-closable="false"
      :closable="false"
      :title="editingUserId ? '编辑用户' : '增加用户'"
      :style="{ width: 'min(520px, calc(100vw - 32px))' }"
    >
      <NAlert v-if="editingUserId === null" type="warning" :bordered="false" class="user-editor-warning">
        账号一旦创建，不允许删除，只允许禁用，请谨慎操作。
      </NAlert>

      <NForm label-placement="top">
        <NFormItem label="用户昵称" required>
          <NInput
            v-model:value="userNickname"
            placeholder="例如：研发用户"
            @keyup.enter="saveUser"
          />
        </NFormItem>
        <NFormItem label="账号" required>
          <NInput
            v-model:value="userAccount"
            autocomplete="username"
            :disabled="editingUserId !== null"
            placeholder="例如：user001"
            @keyup.enter="saveUser"
          />
        </NFormItem>
        <NFormItem label="密码" :required="editingUserId === null">
          <NInput
            v-model:value="userPassword"
            type="password"
            show-password-on="mousedown"
            autocomplete="new-password"
            :placeholder="editingUserId ? '留空不修改密码' : '请输入登录密码'"
            @keyup.enter="saveUser"
          />
        </NFormItem>
        <NFormItem label="是否设为管理员">
          <NSwitch v-model:value="isUserAdmin" :disabled="isEditingFirstUser" />
        </NFormItem>
        <NFormItem label="余额设置">
          <div class="quota-unlimited-row">
            <div>
              <div class="quota-unlimited-title">不限制余额</div>
              <div class="quota-unlimited-desc">开启后不扣余额，也不会因余额暂停 API Key。</div>
            </div>
            <NSwitch v-model:value="quotaUnlimited" />
          </div>
        </NFormItem>
        <div class="form-grid quota-editor-grid">
          <NFormItem label="不限时余额 USD">
            <NInputNumber
              :value="quotaLifetimeUsd"
              :disabled="quotaUnlimited"
              :min="0"
              :precision="8"
              placeholder="0"
              @update:value="setQuotaLifetimeUsd"
            />
          </NFormItem>
          <NFormItem label="每月余额 USD">
            <NInputNumber
              :value="quotaMonthlyUsd"
              :disabled="quotaUnlimited"
              :min="0"
              :precision="8"
              placeholder="0"
              @update:value="setQuotaMonthlyUsd"
            />
          </NFormItem>
        </div>
        <NAlert type="info" :bordered="false" class="quota-editor-hint">
          关闭不限制后，扣费顺序：先扣每月余额，不足部分再扣不限时余额；两者都无剩余时暂停该用户的 API Key。
        </NAlert>
        <div class="user-editor-actions">
          <NButton secondary @click="editorVisible = false">取消</NButton>
          <NButton type="primary" :loading="isSavingUser" @click="saveUser">
            {{ editingUserId ? '保存' : '创建' }}
          </NButton>
        </div>
      </NForm>
    </NModal>
  </section>
</template>

<style scoped>
.user-metrics {
  grid-template-columns: repeat(4, minmax(150px, 1fr));
}

.user-editor-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.user-editor-warning {
  margin-bottom: 12px;
}

.quota-editor-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px 12px;
}

.quota-editor-hint {
  margin: -2px 0 12px;
}

.quota-unlimited-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  gap: 16px;
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid var(--cpa-border);
  border-radius: var(--cpa-radius);
  background: var(--cpa-surface);
}

.quota-unlimited-title {
  color: var(--cpa-text-strong);
  font-size: 13px;
  font-weight: 760;
}

.quota-unlimited-desc {
  margin-top: 2px;
  color: var(--cpa-muted);
  font-size: 12px;
  line-height: 1.35;
}

:global(.metric-stack) {
  display: grid;
  gap: 2px;
  min-width: 0;
  line-height: 1.28;
}

:global(.quota-balance-stack) {
  gap: 3px;
}

:global(.quota-balance-row) {
  display: inline-flex;
  align-items: center;
  min-width: 0;
  width: fit-content;
  max-width: 100%;
  padding: 2px 7px;
  overflow: hidden;
  border-radius: var(--cpa-radius-sm);
  font-size: 12px;
  line-height: 1.35;
  white-space: nowrap;
}

:global(.quota-balance-row.is-monthly.is-normal) {
  background: var(--cpa-success-weak);
  color: var(--cpa-success);
}

:global(.quota-balance-row.is-lifetime.is-normal) {
  background: var(--cpa-primary-weak);
  color: var(--cpa-primary);
}

:global(.quota-balance-row.is-unlimited) {
  background: var(--cpa-primary-wash);
  color: var(--cpa-primary);
}

:global(.quota-balance-row.is-warning) {
  background: var(--cpa-warning-weak);
  color: var(--cpa-warning);
}

:global(.quota-balance-row.is-error) {
  background: var(--cpa-danger-weak);
  color: var(--cpa-danger);
}

:global(.quota-balance-label),
:global(.quota-balance-value) {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

:global(.quota-balance-label) {
  flex: 0 0 auto;
  font-weight: 600;
}

:global(.quota-balance-value) {
  font-weight: 760;
}

:global(.metric-primary) {
  font-weight: 600;
}

:global(.metric-muted) {
  min-width: 0;
  overflow: hidden;
  color: var(--cpa-muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

:global(.metric-muted.is-error) {
  color: var(--cpa-danger);
}

:global(.model-value) {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 900px) {
  .user-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 560px) {
  .user-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .quota-editor-grid {
    grid-template-columns: 1fr;
  }
}
</style>
