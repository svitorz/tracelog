import { describe, expect, it } from 'vitest'
import { filtersFromForm, hasFilters, readCursor, readEventFilters, writeEventSearch } from './filters'

describe('event filters <-> query string', () => {
  it('reads known filters, trims them and drops blanks and unknown keys', () => {
    const search = new URLSearchParams('severity=%20error%20&from=&application_id=abc&foo=bar&cursor=c1')
    expect(readEventFilters(search)).toEqual({ severity: 'error', application_id: 'abc' })
    expect(readCursor(search)).toBe('c1')
    expect(readCursor(new URLSearchParams('cursor=%20'))).toBeUndefined()
  })

  it('writes only what is present, in a stable order, with the cursor last', () => {
    expect(writeEventSearch({ to: '2026-03-14', severity: 'fatal', from: ' ' }, 'c2').toString()).toBe(
      'severity=fatal&to=2026-03-14&cursor=c2',
    )
    expect(writeEventSearch({}).toString()).toBe('')
  })

  it('round-trips', () => {
    const filters = { severity: 'warn', application_id: 'a', environment_id: 'e', from: '2026-03-01', to: '2026-03-02' }
    expect(readEventFilters(writeEventSearch(filters))).toEqual(filters)
  })

  it('reads a submitted form and ignores other fields', () => {
    const form = new FormData()
    form.set('severity', '')
    form.set('from', '2026-03-01')
    form.set('cursor', 'should-not-pass')
    expect(filtersFromForm(form)).toEqual({ from: '2026-03-01' })
    expect(hasFilters({})).toBe(false)
    expect(hasFilters({ to: 'x' })).toBe(true)
  })
})
