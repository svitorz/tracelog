import type {
  ApiErrorBody,
  Application,
  DataRange,
  Environment,
  ErrorFrequency,
  EventDetail,
  EventList,
  EventListFilters,
  EventScope,
  ItemList,
  SeverityCounts,
  Source,
} from './types'

/**
 * Failure talking to the API. `status` is 0 when no HTTP response arrived (network
 * error) or the response was not the API's JSON (for example a proxy error page).
 */
export class ApiError extends Error {
  readonly status: number
  readonly code: string | undefined
  readonly field: string | undefined

  constructor(message: string, status: number, code?: string, field?: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.field = field
  }

  get isUnreachable(): boolean {
    return this.status === 0
  }
}

type QueryValue = string | number | undefined

/** Builds a query string, leaving out absent and empty values. */
export function buildQuery(params: Record<string, QueryValue>): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined) continue
    const text = String(value).trim()
    if (text !== '') search.set(key, text)
  }
  const query = search.toString()
  return query === '' ? '' : `?${query}`
}

function isErrorBody(value: unknown): value is ApiErrorBody {
  return typeof value === 'object' && value !== null && typeof (value as ApiErrorBody).error === 'string'
}

async function getJSON<T>(path: string, signal?: AbortSignal): Promise<T> {
  let response: Response
  try {
    response = await fetch(path, { headers: { Accept: 'application/json' }, signal: signal ?? null })
  } catch (err) {
    if (err instanceof DOMException && err.name === 'AbortError') throw err
    throw new ApiError('Could not reach the TraceLog API.', 0)
  }

  let body: unknown
  try {
    body = await response.json()
  } catch {
    throw new ApiError('The TraceLog API returned an unexpected response.', 0)
  }

  if (!response.ok) {
    if (isErrorBody(body)) throw new ApiError(body.error, response.status, body.code, body.field)
    throw new ApiError(`Request failed with HTTP ${response.status}.`, response.status)
  }
  return body as T
}

const base = '/api/v1'

export const api = {
  listEvents: (filters: EventListFilters, signal?: AbortSignal) =>
    getJSON<EventList>(`${base}/events${buildQuery({ ...filters })}`, signal),
  getEvent: (id: string, signal?: AbortSignal) =>
    getJSON<EventDetail>(`${base}/events/${encodeURIComponent(id)}`, signal),
  errorFrequency: (scope: EventScope, signal?: AbortSignal) =>
    getJSON<ErrorFrequency>(`${base}/metrics/error-frequency${buildQuery({ ...scope })}`, signal),
  severityCounts: (scope: EventScope, signal?: AbortSignal) =>
    getJSON<SeverityCounts>(`${base}/metrics/severity-counts${buildQuery({ ...scope })}`, signal),
  dataRange: (signal?: AbortSignal) => getJSON<DataRange>(`${base}/metrics/data-range`, signal),
  applications: (signal?: AbortSignal) => getJSON<ItemList<Application>>(`${base}/applications`, signal),
  environments: (signal?: AbortSignal) => getJSON<ItemList<Environment>>(`${base}/environments`, signal),
  sources: (signal?: AbortSignal) => getJSON<ItemList<Source>>(`${base}/sources`, signal),
  /** GET /ready, outside /api: 200 {"status":"ready"} or 503 {"status":"unavailable"}. */
  ready: async (signal?: AbortSignal): Promise<boolean> => {
    try {
      const response = await fetch('/ready', { signal: signal ?? null })
      return response.ok
    } catch (err) {
      if (err instanceof DOMException && err.name === 'AbortError') throw err
      return false
    }
  },
}
