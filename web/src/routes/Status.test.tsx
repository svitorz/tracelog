import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import applications from '@fixtures/applications.json'
import dataRange from '@fixtures/data_range.json'
import { mockFetchRoutes } from '../api/testing'
import { renderApp } from '../testing'

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('Status page', () => {
  it('shows the data the API returns', async () => {
    mockFetchRoutes({
      '/ready': { body: { status: 'ready' } },
      '/api/v1/metrics/data-range': { body: dataRange },
      '/api/v1/applications': { body: applications },
    })

    renderApp()

    expect(screen.getByText('Checking the API…')).toBeDefined()
    expect(await screen.findByText('Connected, database ready')).toBeDefined()
    expect(screen.getByText('Local Application')).toBeDefined()
    expect(screen.getByText('2026-03-01 to 2026-03-14')).toBeDefined()
  })

  it('shows an alert with retry when the API is unreachable', async () => {
    const fetch = mockFetchRoutes({})
    renderApp()

    const alert = await screen.findByRole('alert')
    expect(within(alert).getByText('Could not reach the TraceLog API.')).toBeDefined()

    const calls = fetch.mock.calls.length
    await userEvent.click(within(alert).getByRole('button', { name: 'Try again' }))
    expect(fetch.mock.calls.length).toBeGreaterThan(calls)
  })

  it('renders the not-found page for unknown routes', async () => {
    mockFetchRoutes({})
    renderApp('/nope')
    expect(await screen.findByRole('heading', { name: 'Page not found' })).toBeDefined()
  })
})
