import type { CurrentUserResponse, User } from '@/types'

export type AuthUserPayload = User & { run_mode?: 'standard' | 'simple' }

export function normalizeUser<T extends AuthUserPayload | CurrentUserResponse>(user: T): T {
  return {
    ...user,
    balance_notify_extra_emails: Array.isArray(user.balance_notify_extra_emails)
      ? user.balance_notify_extra_emails
      : [],
    subscriptions: Array.isArray(user.subscriptions) ? user.subscriptions : [],
  }
}
