import { formatDateLocalInput } from './format'

export type KeyUsagePeriod = '24h' | '7d' | '14d' | '30d' | 'thisMonth' | 'lastMonth' | 'custom'

export function keyUsageRange(period: KeyUsagePeriod, now = new Date(), startDate = '', endDate = '') {
  let end = new Date(now)
  let start = new Date(now)
  if (period === '24h') {
    start = new Date(now.getTime() - 86400000)
  } else {
    start.setHours(0, 0, 0, 0)
    end.setHours(0, 0, 0, 0)
    end.setDate(end.getDate() + 1)
    if (period === 'custom') {
      start = new Date(`${startDate}T00:00:00`)
      end = new Date(`${endDate}T00:00:00`)
      end.setDate(end.getDate() + 1)
    } else if (period === 'thisMonth') {
      start.setDate(1)
    } else if (period === 'lastMonth') {
      start.setDate(1)
      end = new Date(start)
      start.setMonth(start.getMonth() - 1)
    } else {
      start.setDate(start.getDate() - (Number.parseInt(period) - 1))
    }
  }
  if (!Number.isFinite(start.getTime()) || !Number.isFinite(end.getTime()) || start >= end || end.getTime() - start.getTime() > 366 * 86400000) {
    throw new Error('Invalid time range')
  }
  return { start_time: start.toISOString(), end_time: end.toISOString() }
}

export function keyUsageDisplayDates(range: { start_time: string; end_time: string }) {
  return { startDate: formatDateLocalInput(new Date(range.start_time)), endDate: formatDateLocalInput(new Date(new Date(range.end_time).getTime() - 1)) }
}
