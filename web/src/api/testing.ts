import { vi } from 'vitest'

/** Replaces fetch with one that answers every call with the given status and JSON body. */
export function mockFetchJSON(status: number, body: unknown) {
  const fn = vi.fn((input: RequestInfo | URL) => {
    void input
    return Promise.resolve(
      new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } }),
    )
  })
  vi.stubGlobal('fetch', fn)
  return fn
}

/** Replaces fetch with one that routes by path prefix; unknown paths fail like a network error. */
export function mockFetchRoutes(routes: Record<string, { status?: number; body: unknown }>) {
  const fn = vi.fn((input: RequestInfo | URL) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    const path = url.split('?')[0] ?? url
    const route = routes[path]
    if (!route) return Promise.reject(new TypeError(`no mock for ${path}`))
    return Promise.resolve(
      new Response(JSON.stringify(route.body), {
        status: route.status ?? 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
  })
  vi.stubGlobal('fetch', fn)
  return fn
}

export function calledURL(fn: { mock: { calls: unknown[][] } }, call = 0): string {
  const input = fn.mock.calls[call]?.[0]
  return typeof input === 'string' ? input : String(input)
}
