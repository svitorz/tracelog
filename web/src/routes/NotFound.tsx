import { Link } from 'react-router'

export function NotFound() {
  return (
    <section>
      <h1 className="text-xl font-semibold">Page not found</h1>
      <p className="mt-2">
        <Link to="/" className="text-blue-700 underline">
          Back to TraceLog
        </Link>
      </p>
    </section>
  )
}
