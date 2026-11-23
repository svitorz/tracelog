import { describe, expect, it } from 'vitest'
import { formatUTC, isDateOnly } from './time'

describe('formatUTC', () => {
  it('shows UTC regardless of the offset in the input', () => {
    expect(formatUTC('2026-03-10T14:47:00Z')).toBe('2026-03-10 14:47:00 UTC')
    expect(formatUTC('2026-03-10T21:30:00-03:00')).toBe('2026-03-11 00:30:00 UTC')
    expect(formatUTC('2026-11-23T20:38:40.512345Z')).toBe('2026-11-23 20:38:40 UTC')
  })

  it('returns invalid input unchanged', () => {
    expect(formatUTC('yesterday')).toBe('yesterday')
  })

  it('recognizes YYYY-MM-DD', () => {
    expect(isDateOnly('2026-03-01')).toBe(true)
    expect(isDateOnly('2026-03-01T00:00:00Z')).toBe(false)
    expect(isDateOnly(undefined)).toBe(false)
  })
})
