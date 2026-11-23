import { afterEach, describe, expect, it, vi } from 'vitest'
import eventDetail from '@fixtures/event_detail.json'
import eventsPage from '@fixtures/events_page.json'
import errorInternal from '@fixtures/error_internal.json'
import errorInvalidParameter from '@fixtures/error_invalid_parameter.json'
import errorNotFound from '@fixtures/error_not_found.json'
import { api, ApiError, buildQuery } from './client'
import { calledURL, mockFetchJSON } from './testing'

afterEach(() => {
  vi.unstubAllGlobals()
})

async function catchApiError(promise: Promise<unknown>): Promise<ApiError> {
  try {
    await promise
  } catch (err) {
    if (err instanceof ApiError) return err
    throw err
  }
  throw new Error('expected the call to fail')
}

describe('buildQuery', () => {
  it('drops absent and blank values and encodes the rest', () => {
    expect(buildQuery({ severity: 'error', from: '', to: '  ', cursor: undefined, limit: 50 })).toBe(
      '?severity=error&limit=50',
    )
    expect(buildQuery({ q: 'a&b=c' })).toBe('?q=a%26b%3Dc')
    expect(buildQuery({})).toBe('')
  })
})

describe('api client', () => {
  it('lists events with the filters in the query string', async () => {
    const fetch = mockFetchJSON(200, eventsPage)

    const page = await api.listEvents({ severity: 'fatal', application_id: '', cursor: 'abc', limit: 50 })

    expect(calledURL(fetch)).toBe('/api/v1/events?severity=fatal&cursor=abc&limit=50')
    expect(page.items).toHaveLength(2)
    expect(page.next_cursor).toBe(eventsPage.next_cursor)
  })

  it('encodes the event id in the path', async () => {
    const fetch = mockFetchJSON(200, eventDetail)
    await api.getEvent('a/b?c')
    expect(calledURL(fetch)).toBe('/api/v1/events/a%2Fb%3Fc')
  })

  it('keeps the field of a 400 from the API', async () => {
    mockFetchJSON(400, errorInvalidParameter)
    const err = await catchApiError(api.listEvents({ limit: 201 }))
    expect(err.status).toBe(400)
    expect(err.code).toBe('invalid_parameter')
    expect(err.field).toBe('limit')
    expect(err.message).toBe('limit must be an integer between 1 and 200')
    expect(err.isUnreachable).toBe(false)
  })

  it('reports 404 and 500 with the API code', async () => {
    mockFetchJSON(404, errorNotFound)
    expect((await catchApiError(api.getEvent('x'))).code).toBe('not_found')

    mockFetchJSON(500, errorInternal)
    const err = await catchApiError(api.dataRange())
    expect(err.status).toBe(500)
    expect(err.message).toBe('internal error')
  })

  it('treats a network failure as unreachable', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.reject(new TypeError('Failed to fetch'))))
    const err = await catchApiError(api.applications())
    expect(err.isUnreachable).toBe(true)
  })

  it('treats a non-JSON response (a proxy error page) as unreachable', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() => Promise.resolve(new Response('<html>502 Bad Gateway</html>', { status: 502 }))),
    )
    const err = await catchApiError(api.sources())
    expect(err.isUnreachable).toBe(true)
  })

  it('reports readiness without throwing', async () => {
    mockFetchJSON(503, { status: 'unavailable' })
    expect(await api.ready()).toBe(false)
    vi.stubGlobal('fetch', vi.fn(() => Promise.reject(new TypeError('Failed to fetch'))))
    expect(await api.ready()).toBe(false)
    mockFetchJSON(200, { status: 'ready' })
    expect(await api.ready()).toBe(true)
  })
})
