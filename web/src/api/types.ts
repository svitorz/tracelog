// Tipos escritos à mão a partir da spec 0005 (seção 5.1). Os testes comparam as chaves
// destes tipos com as fixtures de internal/adapters/http/testdata/api, que o Go também
// verifica: se um lado mudar um campo, um dos testes quebra.

export const SEVERITIES = ['debug', 'info', 'warn', 'error', 'fatal'] as const
export type Severity = (typeof SEVERITIES)[number]

export interface EventSummary {
  id: string
  external_id: string | null
  occurred_at: string
  received_at: string
  created_at: string
  severity: Severity
  category: string
  message: string
  normalization_status: string
  ingestion_channel: string
  trace_id: string | null
  correlation_id: string | null
  application_id: string
  environment_id: string
  source_id: string
}

export interface EventDetail extends EventSummary {
  parse_error: string | null
  raw_line: string | null
  payload: Record<string, unknown>
}

export interface EventList {
  items: EventSummary[]
  next_cursor: string | null
}

export interface ErrorFrequencyDay {
  date: string
  errors: number
  fatal: number
  total: number
}

export interface ErrorFrequency {
  from: string
  to: string
  days: ErrorFrequencyDay[]
}

export interface SeverityCounts {
  counts: Record<Severity, number>
  total: number
}

export interface DataRange {
  first_occurred_at: string | null
  last_occurred_at: string | null
  last_error_at: string | null
  default_from: string
  default_to: string
}

export interface Application {
  id: string
  name: string
  service_key: string
  description: string | null
  created_at: string
}

export interface Environment {
  id: string
  code: string
  name: string
  description: string | null
  created_at: string
}

export interface Source {
  id: string
  name: string
  source_type: string
  description: string | null
  created_at: string
}

export interface ItemList<T> {
  items: T[]
}

export type ErrorCode = 'invalid_parameter' | 'invalid_cursor' | 'not_found' | 'method_not_allowed' | 'internal'

export interface ApiErrorBody {
  error: string
  code: ErrorCode
  field?: string
}

/** Filters shared by the event list and the aggregates; absent means "no filter". */
export interface EventScope {
  application_id?: string
  environment_id?: string
  from?: string
  to?: string
}

export interface EventListFilters extends EventScope {
  severity?: string
  limit?: number
  cursor?: string
}
