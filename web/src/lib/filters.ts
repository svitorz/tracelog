import type { EventListFilters } from '../api/types'

// Filtros da lista de eventos <-> query string da UI (spec 0006, seção 4.1). Os nomes são
// os mesmos da API; a UI não valida valores: quem valida é a API, que responde 400 com o
// campo (spec 0005).

export const EVENT_FILTER_KEYS = ['severity', 'application_id', 'environment_id', 'from', 'to'] as const
export type EventFilterKey = (typeof EVENT_FILTER_KEYS)[number]

export type EventFilters = Partial<Record<EventFilterKey, string>>

/** Reads the known filters from the URL, trimmed; blank and unknown keys are dropped. */
export function readEventFilters(search: URLSearchParams): EventFilters {
  const filters: EventFilters = {}
  for (const key of EVENT_FILTER_KEYS) {
    const value = search.get(key)?.trim()
    if (value) filters[key] = value
  }
  return filters
}

export function readCursor(search: URLSearchParams): string | undefined {
  return search.get('cursor')?.trim() || undefined
}

/** Builds the UI query string; absent values are not written. */
export function writeEventSearch(filters: EventFilters, cursor?: string): URLSearchParams {
  const search = new URLSearchParams()
  for (const key of EVENT_FILTER_KEYS) {
    const value = filters[key]?.trim()
    if (value) search.set(key, value)
  }
  if (cursor) search.set('cursor', cursor)
  return search
}

/** Reads a submitted filter form; fields with other names are ignored. */
export function filtersFromForm(form: FormData): EventFilters {
  const filters: EventFilters = {}
  for (const key of EVENT_FILTER_KEYS) {
    const value = form.get(key)
    if (typeof value === 'string' && value.trim() !== '') filters[key] = value.trim()
  }
  return filters
}

export function hasFilters(filters: EventFilters): boolean {
  return EVENT_FILTER_KEYS.some((key) => filters[key] !== undefined)
}

export function toApiFilters(filters: EventFilters, cursor: string | undefined, limit: number): EventListFilters {
  return { ...filters, cursor, limit }
}
