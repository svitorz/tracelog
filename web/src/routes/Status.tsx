import { useQuery } from '@tanstack/react-query'
import { api, ApiError } from '../api/client'

// Prova de vida da fase 4: mostra que a UI alcança a API (via proxy do Vite em dev).
export function Status() {
  const ready = useQuery({ queryKey: ['ready'], queryFn: ({ signal }) => api.ready(signal) })
  const range = useQuery({ queryKey: ['data-range'], queryFn: ({ signal }) => api.dataRange(signal) })
  const apps = useQuery({ queryKey: ['applications'], queryFn: ({ signal }) => api.applications(signal) })

  const loading = ready.isPending || range.isPending || apps.isPending
  const failure = range.error ?? apps.error

  return (
    <section aria-labelledby="status-title" aria-busy={loading}>
      <h1 id="status-title" className="text-xl font-semibold">
        API connection
      </h1>

      {loading && <p className="mt-4 text-slate-700">Checking the API…</p>}

      {!loading && failure && (
        <div role="alert" className="mt-4 rounded border border-red-300 bg-red-50 p-4 text-red-900">
          <p className="font-medium">
            {failure instanceof ApiError && failure.isUnreachable
              ? 'Could not reach the TraceLog API.'
              : failure.message}
          </p>
          <button
            type="button"
            className="mt-2 rounded border border-red-700 px-3 py-1 text-red-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-red-700"
            onClick={() => {
              void range.refetch()
              void apps.refetch()
              void ready.refetch()
            }}
          >
            Try again
          </button>
        </div>
      )}

      {!loading && !failure && range.data && apps.data && (
        <dl className="mt-4 grid grid-cols-1 gap-x-6 gap-y-2 sm:grid-cols-[max-content_1fr]">
          <dt className="font-medium">Status</dt>
          <dd>{ready.data ? 'Connected, database ready' : 'Connected, database not ready'}</dd>
          <dt className="font-medium">Applications</dt>
          <dd>{apps.data.items.map((a) => a.name).join(', ') || 'none'}</dd>
          <dt className="font-medium">First event</dt>
          <dd>{range.data.first_occurred_at ?? 'no events yet'}</dd>
          <dt className="font-medium">Latest error</dt>
          <dd>{range.data.last_error_at ?? 'no errors'}</dd>
          <dt className="font-medium">Suggested period (UTC)</dt>
          <dd>
            {range.data.default_from} to {range.data.default_to}
          </dd>
        </dl>
      )}
    </section>
  )
}
