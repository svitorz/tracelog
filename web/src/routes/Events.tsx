import { useQuery } from '@tanstack/react-query'
import { useId, type ReactNode } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router'
import { api, ApiError } from '../api/client'
import { SEVERITIES, type Application, type Environment, type EventSummary } from '../api/types'
import { ApiErrorAlert } from '../components/ApiErrorAlert'
import { SeverityBadge } from '../components/SeverityBadge'
import {
  filtersFromForm,
  hasFilters,
  readCursor,
  readEventFilters,
  toApiFilters,
  writeEventSearch,
  type EventFilterKey,
  type EventFilters,
} from '../lib/filters'
import { formatUTC, isDateOnly } from '../lib/time'
import { useDocumentTitle } from '../lib/useDocumentTitle'

/** The API default page size (spec 0005); the MVP has no page-size selector. */
const PAGE_SIZE = 50

const focusRing = 'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700'
const control = `mt-1 block w-full rounded border border-slate-400 bg-white px-2 py-1.5 aria-[invalid=true]:border-red-700 ${focusRing}`
const button = `whitespace-nowrap rounded border px-3 py-1.5 font-medium disabled:cursor-not-allowed disabled:opacity-50 ${focusRing}`

export function Events() {
  useDocumentTitle('Events')
  const [search] = useSearchParams()
  const navigate = useNavigate()

  const filters = readEventFilters(search)
  const cursor = readCursor(search)
  const query = toApiFilters(filters, cursor, PAGE_SIZE)

  const events = useQuery({ queryKey: ['events', query], queryFn: ({ signal }) => api.listEvents(query, signal) })
  const apps = useQuery({ queryKey: ['applications'], queryFn: ({ signal }) => api.applications(signal) })
  const envs = useQuery({ queryKey: ['environments'], queryFn: ({ signal }) => api.environments(signal) })

  // Each page is a history entry, so the browser's Back button is "previous page".
  const show = (next: EventFilters, nextCursor?: string) =>
    void navigate({ search: writeEventSearch(next, nextCursor).toString() })

  const invalidField =
    events.error instanceof ApiError && events.error.status === 400 ? (events.error.field ?? 'cursor') : undefined
  const appNames = new Map((apps.data?.items ?? []).map((a) => [a.id, a.name]))

  return (
    <section aria-labelledby="events-title" aria-busy={events.isFetching}>
      <h1 id="events-title" className="text-xl font-semibold">
        Events
      </h1>

      <FilterForm
        key={writeEventSearch(filters).toString()}
        filters={filters}
        invalidField={invalidField}
        applications={apps.data?.items}
        environments={envs.data?.items}
        onApply={(next) => show(next)}
        onClear={() => show({})}
      />

      {events.isPending && <p className="mt-4 text-slate-700">Loading events…</p>}

      {events.isError && (
        <ApiErrorAlert error={events.error} onRetry={() => void events.refetch()}>
          {invalidField !== undefined && (
            <Link to="/events" className="self-center text-red-900 underline">
              Clear filters
            </Link>
          )}
        </ApiErrorAlert>
      )}

      {events.isSuccess && events.data.items.length === 0 && (
        <p className="mt-4 rounded border border-slate-300 bg-white p-4">
          No events for these filters.{' '}
          {(hasFilters(filters) || cursor) && (
            <Link to="/events" className="text-blue-700 underline">
              Clear filters
            </Link>
          )}
        </p>
      )}

      {events.isSuccess && events.data.items.length > 0 && (
        <>
          <EventsTable items={events.data.items} appNames={appNames} />
          <nav aria-label="Pagination" className="mt-4 flex items-center gap-3">
            <button
              type="button"
              className={`${button} border-slate-400 bg-white`}
              disabled={!cursor}
              onClick={() => show(filters)}
            >
              First page
            </button>
            <button
              type="button"
              className={`${button} border-blue-700 bg-blue-700 text-white`}
              disabled={!events.data.next_cursor}
              onClick={() => events.data.next_cursor && show(filters, events.data.next_cursor)}
            >
              Next page
            </button>
            <span className="text-sm text-slate-700">
              {events.data.items.length} events on this page{cursor ? ' (continued)' : ''}. Use the browser&apos;s
              Back button for the previous page.
            </span>
          </nav>
        </>
      )}
    </section>
  )
}

interface FilterFormProps {
  filters: EventFilters
  invalidField: string | undefined
  applications: Application[] | undefined
  environments: Environment[] | undefined
  onApply: (filters: EventFilters) => void
  onClear: () => void
}

function FilterForm({ filters, invalidField, applications, environments, onApply, onClear }: FilterFormProps) {
  const invalid = (key: EventFilterKey) => (invalidField === key ? true : undefined)

  return (
    <form
      aria-label="Filters"
      className="mt-4 grid grid-cols-1 gap-3 rounded border border-slate-300 bg-white p-4 sm:grid-cols-2 lg:grid-cols-6"
      onSubmit={(event) => {
        event.preventDefault()
        onApply(filtersFromForm(new FormData(event.currentTarget)))
      }}
    >
      <Field label="Severity">
        {(id) => (
          <select id={id} name="severity" defaultValue={filters.severity ?? ''} aria-invalid={invalid('severity')} className={control}>
            <option value="">All</option>
            {SEVERITIES.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
            {filters.severity && !(SEVERITIES as readonly string[]).includes(filters.severity) && (
              <option value={filters.severity}>{filters.severity}</option>
            )}
          </select>
        )}
      </Field>

      <ReferenceSelect
        label="Application"
        name="application_id"
        value={filters.application_id}
        items={applications?.map((a) => ({ id: a.id, label: a.name }))}
        invalid={invalid('application_id')}
      />
      <ReferenceSelect
        label="Environment"
        name="environment_id"
        value={filters.environment_id}
        items={environments?.map((e) => ({ id: e.id, label: e.name }))}
        invalid={invalid('environment_id')}
      />
      <DateField label="From (UTC)" name="from" value={filters.from} invalid={invalid('from')} />
      <DateField label="To (UTC)" name="to" value={filters.to} invalid={invalid('to')} />

      <div className="flex items-end gap-2">
        <button type="submit" className={`${button} border-blue-700 bg-blue-700 text-white`}>
          Apply
        </button>
        <button type="button" className={`${button} border-slate-400 bg-white`} onClick={onClear}>
          Clear filters
        </button>
      </div>
    </form>
  )
}

interface ReferenceSelectProps {
  label: string
  name: EventFilterKey
  value: string | undefined
  items: { id: string; label: string }[] | undefined
  invalid: true | undefined
}

function ReferenceSelect({ label, name, value, items, invalid }: ReferenceSelectProps) {
  // An id in the URL that is not in the list stays visible instead of silently vanishing.
  const unknown = value !== undefined && items !== undefined && !items.some((i) => i.id === value)
  return (
    <Field label={label}>
      {(id) => (
        // key: defaultValue só vale na montagem; remonta quando as opções chegam, senão o
        // valor vindo da URL não seria selecionado.
        <select
          key={items === undefined ? 'loading' : 'loaded'}
          id={id}
          name={name}
          defaultValue={value ?? ''}
          disabled={items === undefined}
          aria-invalid={invalid}
          className={control}
        >
          <option value="">All</option>
          {items?.map((i) => (
            <option key={i.id} value={i.id}>
              {i.label}
            </option>
          ))}
          {unknown && <option value={value}>Unknown ({value})</option>}
        </select>
      )}
    </Field>
  )
}

function DateField({ label, name, value, invalid }: { label: string; name: EventFilterKey; value: string | undefined; invalid: true | undefined }) {
  // A value from the URL that is not YYYY-MM-DD (e.g. RFC3339) is kept in a text field,
  // because a date input would show it empty and drop it on the next Apply.
  const type = value === undefined || isDateOnly(value) ? 'date' : 'text'
  return (
    <Field label={label}>
      {(id) => <input id={id} name={name} type={type} defaultValue={value ?? ''} aria-invalid={invalid} className={control} />}
    </Field>
  )
}

/** Label tied to its control by id/htmlFor: more robust for assistive tech than nesting. */
function Field({ label, children }: { label: string; children: (id: string) => ReactNode }) {
  const id = useId()
  return (
    <div className="text-sm">
      <label htmlFor={id} className="font-medium">
        {label}
      </label>
      {children(id)}
    </div>
  )
}

function EventsTable({ items, appNames }: { items: EventSummary[]; appNames: Map<string, string> }) {
  return (
    <div className="mt-4 overflow-x-auto rounded border border-slate-300 bg-white">
      <table className="w-full min-w-[48rem] text-left text-sm">
        <caption className="sr-only">Events, newest first</caption>
        <thead className="bg-slate-100 text-slate-800">
          <tr>
            <th scope="col" className="px-3 py-2">Occurred at</th>
            <th scope="col" className="px-3 py-2">Severity</th>
            <th scope="col" className="px-3 py-2">Category</th>
            <th scope="col" className="px-3 py-2">Message</th>
            <th scope="col" className="px-3 py-2">Application</th>
            <th scope="col" className="px-3 py-2">Channel</th>
          </tr>
        </thead>
        <tbody>
          {items.map((e) => (
            <tr key={e.id} className="border-t border-slate-200 align-top">
              <td className="whitespace-nowrap px-3 py-2 font-mono text-xs">{formatUTC(e.occurred_at)}</td>
              <td className="px-3 py-2">
                <SeverityBadge severity={e.severity} />
              </td>
              <td className="px-3 py-2">{e.category}</td>
              <td className="max-w-[28rem] px-3 py-2">
                <Link to={`/events/${encodeURIComponent(e.id)}`} title={e.message} className={`block truncate text-blue-800 underline ${focusRing}`}>
                  {e.message}
                </Link>
              </td>
              <td className="px-3 py-2">{appNames.get(e.application_id) ?? e.application_id}</td>
              <td className="px-3 py-2">{e.ingestion_channel}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
