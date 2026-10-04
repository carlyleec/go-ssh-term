import type { QueryClient } from '@tanstack/react-query'
import {
  createRootRouteWithContext,
  Link,
  Outlet,
} from '@tanstack/react-router'

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()(
  {
    component: App,
    notFoundComponent: NotFoundPage,
  },
)

function App() {
  return (
    <div className="flex min-h-screen flex-col bg-base-100 text-base-content">
      <a className="sr-only focus:not-sr-only focus:p-4" href="#main-content">
        Skip to content
      </a>
      <header className="border-b border-base-300">
        <nav
          aria-label="Main navigation"
          className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-3 px-6 py-5"
        >
          <Link to="/" className="text-lg font-bold tracking-tight">
            Browser SSH Gateway
          </Link>
          <Link to="/login" className="btn btn-primary btn-sm">
            Account access
          </Link>
        </nav>
      </header>
      <main id="main-content" className="flex-1" tabIndex={-1}>
        <Outlet />
      </main>
      <footer className="border-t border-base-300 px-6 py-5 text-center text-sm text-base-content/70">
        A local learning project built with Go and React.
      </footer>
    </div>
  )
}

function NotFoundPage() {
  return (
    <section className="mx-auto max-w-2xl px-6 py-20">
      <p className="mb-3 text-sm font-semibold text-base-content/60">404</p>
      <h1 className="text-3xl font-bold">Page not found</h1>
      <p className="mt-4 mb-8">That page doesn’t exist.</p>
      <Link to="/" className="btn btn-primary">
        Back to home
      </Link>
    </section>
  )
}
