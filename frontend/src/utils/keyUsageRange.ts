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

export type KeyUsageRange = { start_time: string; end_time: string }

// Previous comparison window: month periods compare to the same days of the prior month,
// everything else to the adjacent window of equal length.
export function previousKeyUsageRange(period: KeyUsagePeriod, range: KeyUsageRange): { prev_start_time: string; prev_end_time: string } {
  const start = new Date(range.start_time)
  const end = new Date(range.end_time)
  if (period === 'thisMonth' || period === 'lastMonth') {
    const prevStart = new Date(start.getFullYear(), start.getMonth() - 1, 1)
    let prevEnd: Date
    if (period === 'lastMonth') {
      prevEnd = new Date(start)
    } else {
      // end is local midnight after the last included day; clamp that day to the prior month's length.
      const lastDay = new Date(end.getTime() - 1).getDate()
      const prevMonthDays = new Date(start.getFullYear(), start.getMonth(), 0).getDate()
      prevEnd = new Date(prevStart.getFullYear(), prevStart.getMonth(), Math.min(lastDay, prevMonthDays) + 1)
    }
    return { prev_start_time: prevStart.toISOString(), prev_end_time: prevEnd.toISOString() }
  }
  return { prev_start_time: new Date(start.getTime() - (end.getTime() - start.getTime())).toISOString(), prev_end_time: start.toISOString() }
}

export function keyUsageGranularity(range: KeyUsageRange): 'hour' | 'day' {
  return new Date(range.end_time).getTime() - new Date(range.start_time).getTime() <= 2 * 86400000 ? 'hour' : 'day'
}

export function keyUsageTimezone() {
  try { return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC' } catch { return 'UTC' }
}

export function keyUsageCompareLabel(period: KeyUsagePeriod): 'vsLastMonth' | 'vsPrevious' {
  return period === 'thisMonth' || period === 'lastMonth' ? 'vsLastMonth' : 'vsPrevious'
}

const pad = (n: number) => String(n).padStart(2, '0')
// 2026/10/15 00:00 – 10/21 23:59; the end shows the last included minute.
export function formatKeyUsageRange(range: KeyUsageRange) {
  const start = new Date(range.start_time)
  const end = new Date(new Date(range.end_time).getTime() - 1)
  const time = (d: Date) => `${pad(d.getHours())}:${pad(d.getMinutes())}`
  const md = (d: Date) => `${pad(d.getMonth() + 1)}/${pad(d.getDate())}`
  const endText = start.getFullYear() === end.getFullYear() ? md(end) : `${end.getFullYear()}/${md(end)}`
  return `${start.getFullYear()}/${md(start)} ${time(start)} – ${endText} ${time(end)}`
}
