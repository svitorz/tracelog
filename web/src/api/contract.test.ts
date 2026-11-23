import { describe, expect, it } from 'vitest'
import applications from '@fixtures/applications.json'
import dataRange from '@fixtures/data_range.json'
import dataRangeEmpty from '@fixtures/data_range_empty.json'
import environments from '@fixtures/environments.json'
import errorFrequency from '@fixtures/error_frequency.json'
import errorInvalidParameter from '@fixtures/error_invalid_parameter.json'
import errorNotFound from '@fixtures/error_not_found.json'
import eventDetail from '@fixtures/event_detail.json'
import eventsLastPage from '@fixtures/events_last_page.json'
import eventsPage from '@fixtures/events_page.json'
import severityCounts from '@fixtures/severity_counts.json'
import sources from '@fixtures/sources.json'
import {
  SEVERITIES,
  type ApiErrorBody,
  type Application,
  type DataRange,
  type Environment,
  type ErrorFrequency,
  type ErrorFrequencyDay,
  type EventDetail,
  type EventList,
  type EventSummary,
  type SeverityCounts,
  type Source,
} from './types'

// Cada objeto lista todas as chaves do tipo: o `satisfies` faz o tsc falhar se faltar ou
// sobrar uma. Os testes comparam essas listas com as fixtures geradas pelo Go.
type Keys<T> = Record<keyof T, true>
const keys = <T,>(shape: Keys<T>) => Object.keys(shape).sort()

const summaryKeys = keys<EventSummary>({
  id: true, external_id: true, occurred_at: true, received_at: true, created_at: true, severity: true,
  category: true, message: true, normalization_status: true, ingestion_channel: true, trace_id: true,
  correlation_id: true, application_id: true, environment_id: true, source_id: true,
})
const detailKeys = keys<EventDetail>({
  id: true, external_id: true, occurred_at: true, received_at: true, created_at: true, severity: true,
  category: true, message: true, normalization_status: true, ingestion_channel: true, trace_id: true,
  correlation_id: true, application_id: true, environment_id: true, source_id: true,
  parse_error: true, raw_line: true, payload: true,
})

const sorted = (value: object) => Object.keys(value).sort()

describe('TypeScript types match the Go fixtures', () => {
  it('event list and detail', () => {
    for (const page of [eventsPage, eventsLastPage]) {
      expect(sorted(page)).toEqual(keys<EventList>({ items: true, next_cursor: true }))
      for (const item of page.items) expect(sorted(item)).toEqual(summaryKeys)
    }
    expect(sorted(eventDetail)).toEqual(detailKeys)
    for (const item of eventsPage.items) expect(SEVERITIES).toContain(item.severity)
  })

  it('metrics', () => {
    expect(sorted(errorFrequency)).toEqual(keys<ErrorFrequency>({ from: true, to: true, days: true }))
    for (const day of errorFrequency.days) {
      expect(sorted(day)).toEqual(keys<ErrorFrequencyDay>({ date: true, errors: true, fatal: true, total: true }))
    }
    expect(sorted(severityCounts)).toEqual(keys<SeverityCounts>({ counts: true, total: true }))
    expect(sorted(severityCounts.counts)).toEqual([...SEVERITIES].sort())
    const rangeKeys = keys<DataRange>({
      first_occurred_at: true, last_occurred_at: true, last_error_at: true, default_from: true, default_to: true,
    })
    expect(sorted(dataRange)).toEqual(rangeKeys)
    expect(sorted(dataRangeEmpty)).toEqual(rangeKeys)
  })

  it('reference data', () => {
    for (const item of applications.items) {
      expect(sorted(item)).toEqual(keys<Application>({ id: true, name: true, service_key: true, description: true, created_at: true }))
    }
    for (const item of environments.items) {
      expect(sorted(item)).toEqual(keys<Environment>({ id: true, code: true, name: true, description: true, created_at: true }))
    }
    for (const item of sources.items) {
      expect(sorted(item)).toEqual(keys<Source>({ id: true, name: true, source_type: true, description: true, created_at: true }))
    }
  })

  it('errors', () => {
    expect(sorted(errorInvalidParameter)).toEqual(keys<ApiErrorBody>({ error: true, code: true, field: true }))
    expect(sorted(errorNotFound)).toEqual(['code', 'error'])
  })
})
