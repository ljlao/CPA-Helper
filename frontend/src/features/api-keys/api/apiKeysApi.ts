import { apiClient } from '@/shared/api/apiClient'
import type {
  ApiKeyCreatePayload,
  ApiKeyUpdatePayload,
  UserApiKeySummary,
} from '@/shared/types/api'

function toApiKeyList(value: unknown): UserApiKeySummary[] {
  return Array.isArray(value) ? (value as UserApiKeySummary[]) : []
}

export function listApiKeys(): Promise<UserApiKeySummary[]> {
  return apiClient.get<unknown>('/api-keys').then(toApiKeyList)
}

export function createApiKey(payload: ApiKeyCreatePayload): Promise<UserApiKeySummary> {
  return apiClient.post<UserApiKeySummary>('/api-keys', payload)
}

export function updateApiKey(
  apiKeyHash: string,
  payload: ApiKeyUpdatePayload,
): Promise<UserApiKeySummary> {
  return apiClient.put<UserApiKeySummary>(`/api-keys/${apiKeyHash}`, payload)
}

export function deleteApiKey(apiKeyHash: string): Promise<void> {
  return apiClient.delete(`/api-keys/${apiKeyHash}`)
}
