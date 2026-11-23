import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import applications from '@fixtures/applications.json'
import environments from '@fixtures/environments.json'
import eventDetail from '@fixtures/event_detail.json'
import eventsLastPage from '@fixtures/events_last_page.json'
import eventsPage from '@fixtures/events_page.json'
import { calledURL, mockFetchRoutes } from '../api/testing'
import { currentLocation, historyBack, renderApp } from '../testing'

const reference = {
  '/api/v1/applications': { body: applications },
  '/api/v1/environments': { body: environments },
}

/** Answers /api/v1/events with the first page, or the last page when a cursor is sent. */
function mockEvents(extra: Record<string, { status?: number; body: unknown }> = {}) {
  const fetch = mockFetchRoutes({ ...reference, ...extra })
  const base = fetch.getMockImplementation()!
  fetch.mockImplementation((input: RequestInfo | URL) => {
    const url = calledURL({ mock: { calls: [[input]] } })
    if (url.startsWith('/api/v1/events?') && !('/api/v1/events' in extra)) {
      const body = url.includes('cursor=') ? eventsLastPage : eventsPage
      return Promise.resolve(new Response(JSON.stringify(body), { status: 200 }))
    }
    return base(input)
  })
  return fetch
}

function eventCalls(fetch: ReturnType<typeof mockFetchRoutes>) {
  return fetch.mock.calls.map((_, i) => calledURL(fetch, i)).filter((u) => u.startsWith('/api/v1/events'))
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('Events screen', () => {
  it('lists events with UTC times, application names and links to the detail', async () => {
    const fetch = mockEvents()
    renderApp('/events')

    expect(screen.getByText('Loading events…')).toBeDefined()
    const table = await screen.findByRole('table', { name: 'Events, newest first' })
    const rows = within(table).getAllByRole('row').slice(1)
    expect(rows).toHaveLength(2)

    const first = within(rows[0]!)
    expect(first.getByText('2026-03-10 14:47:00 UTC')).toBeDefined()
    expect(first.getByText('error')).toBeDefined()
    expect(first.getByText('Local Application')).toBeDefined()
    const link = first.getByRole('link', { name: 'payment provider timeout' })
    expect(link.getAttribute('href')).toBe('/events/1a7b3c5d-2e4f-4a6b-8c9d-0e1f2a3b4c5d')
    expect(link.getAttribute('title')).toBe('payment provider timeout')

    expect(eventCalls(fetch)).toEqual(['/api/v1/events?limit=50'])
    expect(document.title).toBe('Events · TraceLog')
  })

  it('renders log text as text, never as HTML', async () => {
    const xss = { items: [eventDetail], next_cursor: null }
    mockFetchRoutes({ ...reference, '/api/v1/events': { body: xss } })
    renderApp('/events')

    const link = await screen.findByRole('link', { name: '<script>alert("xss")</script>' })
    expect(link.textContent).toBe('<script>alert("xss")</script>')
    expect(document.querySelector('main script')).toBeNull()
    expect(document.querySelector('main img')).toBeNull()
  })

  it('reads filters from the URL into the controls and the API call', async () => {
    const fetch = mockEvents()
    renderApp('/events?severity=error&from=2026-03-01&to=2026-03-14T23:59:59Z&application_id=550e8400-e29b-41d4-a716-446655440010')

    await screen.findByRole('table')
    expect(screen.getByLabelText<HTMLSelectElement>('Severity').value).toBe('error')
    expect(screen.getByLabelText<HTMLInputElement>('From (UTC)').value).toBe('2026-03-01')
    // RFC3339 vindo da URL fica num campo de texto, para não sumir no próximo Apply.
    const to = screen.getByLabelText<HTMLInputElement>('To (UTC)')
    expect(to.type).toBe('text')
    expect(to.value).toBe('2026-03-14T23:59:59Z')
    await waitFor(() => expect(screen.getByLabelText<HTMLSelectElement>('Application').value).toBe('550e8400-e29b-41d4-a716-446655440010'))

    expect(eventCalls(fetch)[0]).toBe(
      '/api/v1/events?severity=error&application_id=550e8400-e29b-41d4-a716-446655440010&from=2026-03-01&to=2026-03-14T23%3A59%3A59Z&limit=50',
    )
  })

  it('applies filters into the URL, writing only what is set and dropping the cursor', async () => {
    const fetch = mockEvents()
    const user = userEvent.setup()
    renderApp('/events?cursor=abc')
    await screen.findByRole('table')

    await user.selectOptions(screen.getByLabelText('Severity'), 'fatal')
    await user.type(screen.getByLabelText('From (UTC)'), '2026-03-10')
    await user.click(screen.getByRole('button', { name: 'Apply' }))

    await waitFor(() => expect(currentLocation()).toBe('/events?severity=fatal&from=2026-03-10'))
    await waitFor(() => expect(eventCalls(fetch).at(-1)).toBe('/api/v1/events?severity=fatal&from=2026-03-10&limit=50'))

    await user.click(screen.getByRole('button', { name: 'Clear filters' }))
    await waitFor(() => expect(currentLocation()).toBe('/events'))
    expect(screen.getByLabelText<HTMLSelectElement>('Severity').value).toBe('')
  })

  it('pages forward with the cursor, back with history, and to the first page', async () => {
    const fetch = mockEvents()
    const user = userEvent.setup()
    renderApp('/events?severity=error')
    await screen.findByRole('table')

    expect(screen.getByRole<HTMLButtonElement>('button', { name: 'First page' }).disabled).toBe(true)
    await user.click(screen.getByRole('button', { name: 'Next page' }))

    await waitFor(() => expect(currentLocation()).toBe(`/events?severity=error&cursor=${eventsPage.next_cursor}`))
    await screen.findByRole('link', { name: '<script>alert("xss")</script>' })
    expect(eventCalls(fetch).at(-1)).toContain(`cursor=${eventsPage.next_cursor}`)
    // Última página: sem next_cursor.
    expect(screen.getByRole<HTMLButtonElement>('button', { name: 'Next page' }).disabled).toBe(true)

    act(() => historyBack())
    await waitFor(() => expect(currentLocation()).toBe('/events?severity=error'))
    await screen.findByRole('link', { name: 'payment provider timeout' })

    await user.click(screen.getByRole('button', { name: 'Next page' }))
    await waitFor(() => expect(currentLocation()).toContain('cursor='))
    await user.click(await screen.findByRole('button', { name: 'First page' }))
    await waitFor(() => expect(currentLocation()).toBe('/events?severity=error'))
  })

  it('shows the empty state with a way out', async () => {
    mockFetchRoutes({ ...reference, '/api/v1/events': { body: { items: [], next_cursor: null } } })
    renderApp('/events?severity=debug')

    expect(await screen.findByText(/No events for these filters/)).toBeDefined()
    expect(screen.getAllByRole('link', { name: 'Clear filters' })[0]!.getAttribute('href')).toBe('/events')
  })

  it('marks the field a 400 points to and offers to clear the filters', async () => {
    const body = { error: 'severity must be one of debug, info, warn, error, fatal', code: 'invalid_parameter', field: 'severity' }
    mockFetchRoutes({ ...reference, '/api/v1/events': { status: 400, body } })
    renderApp('/events?severity=critical')

    const alert = await screen.findByRole('alert')
    expect(within(alert).getByText(`Invalid request: ${body.error}`)).toBeDefined()
    expect(within(alert).queryByRole('button', { name: 'Try again' })).toBeNull()
    expect(within(alert).getByRole('link', { name: 'Clear filters' }).getAttribute('href')).toBe('/events')
    const severity = screen.getByLabelText<HTMLSelectElement>('Severity')
    expect(severity.getAttribute('aria-invalid')).toBe('true')
    expect(severity.value).toBe('critical')
  })

  it('offers a retry when the API is unreachable or fails', async () => {
    const fetch = mockFetchRoutes({ ...reference })
    const user = userEvent.setup()
    renderApp('/events')

    const alert = await screen.findByRole('alert')
    expect(within(alert).getByText('Could not reach the TraceLog API.')).toBeDefined()
    const before = eventCalls(fetch).length
    await user.click(within(alert).getByRole('button', { name: 'Try again' }))
    await waitFor(() => expect(eventCalls(fetch).length).toBeGreaterThan(before))
  })

  it('shows a generic message for a 500', async () => {
    mockFetchRoutes({ ...reference, '/api/v1/events': { status: 500, body: { error: 'internal error', code: 'internal' } } })
    renderApp('/events')
    expect(await screen.findByText('The TraceLog API failed to answer (internal error).')).toBeDefined()
  })

  it('is reachable from the header and marks the current page', async () => {
    mockEvents()
    const user = userEvent.setup()
    mockFetchRoutes({ ...reference, '/ready': { body: { status: 'ready' } }, '/api/v1/metrics/data-range': { body: {} } })
    renderApp('/')
    await user.click(screen.getByRole('link', { name: 'Events' }))
    await waitFor(() => expect(currentLocation()).toBe('/events'))
    expect(screen.getByRole('link', { name: 'Events' }).getAttribute('aria-current')).toBe('page')
  })
})
