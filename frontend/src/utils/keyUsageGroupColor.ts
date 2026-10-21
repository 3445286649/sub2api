export type KeyUsageGroupTone = 'openai' | 'claude' | 'gemini' | 'other'

// Color follows the group platform only, never the group name.
export function keyUsageGroupTone(platform?: string | null): KeyUsageGroupTone {
  if (platform === 'openai') return 'openai'
  if (platform === 'anthropic') return 'claude'
  if (platform === 'gemini') return 'gemini'
  return 'other'
}
