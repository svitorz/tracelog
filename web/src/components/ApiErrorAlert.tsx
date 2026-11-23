import type { ReactNode } from 'react'
import { ApiError } from '../api/client'

interface Props {
  error: unknown
  onRetry?: () => void
  /** Text for a 404, which each screen words differently. */
  notFound?: string
  /** Extra actions, e.g. a "clear filters" link for a 400. */
  children?: ReactNode
}

function describe(error: unknown, notFound: string | undefined): { message: string; retry: boolean } {
  if (!(error instanceof ApiError)) return { message: 'Something went wrong.', retry: true }
  if (error.isUnreachable) return { message: 'Could not reach the TraceLog API.', retry: true }
  if (error.status === 400) return { message: `Invalid request: ${error.message}`, retry: false }
  if (error.status === 404) return { message: notFound ?? 'Not found.', retry: false }
  if (error.status >= 500) return { message: 'The TraceLog API failed to answer (internal error).', retry: true }
  return { message: error.message, retry: false }
}

/** Error state shared by the screens (spec 0006, seção 4.6). */
export function ApiErrorAlert({ error, onRetry, notFound, children }: Props) {
  const { message, retry } = describe(error, notFound)
  return (
    <div role="alert" className="mt-4 rounded border border-red-300 bg-red-50 p-4 text-red-900">
      <p className="font-medium">{message}</p>
      <div className="mt-2 flex flex-wrap gap-3">
        {retry && onRetry && (
          <button
            type="button"
            onClick={onRetry}
            className="rounded border border-red-700 px-3 py-1 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-red-700"
          >
            Try again
          </button>
        )}
        {children}
      </div>
    </div>
  )
}
