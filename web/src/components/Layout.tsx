import { Link, NavLink, Outlet } from 'react-router'

export function Layout() {
  return (
    <div className="min-h-screen">
      <header className="border-b border-slate-200 bg-white">
        <nav aria-label="Main" className="mx-auto flex max-w-6xl items-center gap-6 px-4 py-3">
          <Link to="/" className="text-lg font-semibold text-slate-900 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700">
            TraceLog
          </Link>
          <NavLink
            to="/events"
            className="rounded px-1 text-slate-800 underline-offset-4 hover:underline aria-[current=page]:font-semibold aria-[current=page]:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700"
          >
            Events
          </NavLink>
        </nav>
      </header>
      <main className="mx-auto max-w-6xl px-4 py-6">
        <Outlet />
      </main>
    </div>
  )
}
