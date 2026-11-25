import { describe, expect, it } from 'vitest'
import { formatMessageTime } from './format-time'

describe('formatMessageTime', () => {
  const now = new Date(2026, 10, 25, 20, 15, 0) // 2026-11-25 20:15:00

  it('returns empty string for zero or missing timestamps', () => {
    expect(formatMessageTime(0, now)).toBe('')
    expect(formatMessageTime(undefined, now)).toBe('')
    expect(formatMessageTime(null, now)).toBe('')
  })

  it('shows only HH:mm for today', () => {
    const today = new Date(2026, 10, 25, 9, 5, 0).getTime()
    expect(formatMessageTime(today, now)).toBe('09:05')
  })

  it('shows MM-DD HH:mm for earlier days in the same year', () => {
    const yesterday = new Date(2026, 10, 24, 18, 30, 0).getTime()
    expect(formatMessageTime(yesterday, now)).toBe('11-24 18:30')
  })

  it('shows YYYY-MM-DD HH:mm for previous years', () => {
    const lastYear = new Date(2025, 11, 31, 23, 59, 0).getTime()
    expect(formatMessageTime(lastYear, now)).toBe('2025-12-31 23:59')
  })
})
