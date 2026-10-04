import type { QueryClient } from '@tanstack/react-query'
import {
  createRootRouteWithContext,
  Link,
  Outlet,
  useNavigate,
} from '@tanstack/react-router'
import { useRef, useState } from 'react'
import { useAuth } from '~/hooks/use-auth'
import { clearSessionData } from './_authed/route'

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()(
  {
    component: App,
    notFoundComponent: NotFoundPage,
  },
)

function App() {
  const { queryClient } = Route.useRouteContext()
  const { account, signOut, isSigningOut } = useAuth()
  const navigate = useNavigate()
  const [accountMenuOpen, setAccountMenuOpen] = useState(false)
  const accountButton = useRef<HTMLButtonElement>(null)
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
          {account && (
            <div className="order-last flex w-full flex-wrap items-center gap-1 sm:order-none sm:w-auto">
              {(
                [
                  ['/workspace', 'Workspace'],
                  ['/connections', 'Connections'],
                  ['/keys', 'SSH Keys'],
                ] as const
              ).map(([to, label]) => (
                <Link
                  key={to}
                  to={to}
                  className="btn btn-ghost btn-sm"
                  activeProps={{
                    className: 'bg-base-300 text-primary',
                    'aria-current': 'page',
                  }}
                >
                  {label}
                </Link>
              ))}
            </div>
          )}
          {account ? (
            <fieldset
              aria-label="Account"
              className="relative ml-auto"
              onBlur={(event) => {
                if (!event.currentTarget.contains(event.relatedTarget))
                  setAccountMenuOpen(false)
              }}
              onKeyDown={(event) => {
                if (event.key === 'Escape') {
                  setAccountMenuOpen(false)
                  accountButton.current?.focus()
                }
              }}
            >
              <button
                ref={accountButton}
                type="button"
                className="btn btn-ghost btn-sm max-w-full"
                aria-expanded={accountMenuOpen}
                aria-controls="account-dropdown"
                onClick={() => setAccountMenuOpen(!accountMenuOpen)}
              >
                <svg
                  aria-hidden="true"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  className="size-5 shrink-0"
                >
                  <circle cx="12" cy="12" r="10" />
                  <circle cx="12" cy="9" r="3" />
                  <path d="M5.5 19.5a6.5 6.5 0 0 1 13 0" />
                </svg>
                <span className="max-w-48 truncate">
                  {account.display_name}
                </span>
                <svg
                  aria-hidden="true"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  className={`size-4 shrink-0 transition-transform motion-reduce:transition-none ${accountMenuOpen ? 'rotate-180' : ''}`}
                >
                  <path d="m6 9 6 6 6-6" />
                </svg>
              </button>
              {accountMenuOpen && (
                <div
                  id="account-dropdown"
                  className="absolute right-0 z-50 mt-2 w-64 rounded-box border border-base-300 bg-base-200 p-2 shadow-lg"
                >
                  <button
                    type="button"
                    className="btn btn-ghost btn-sm w-full justify-start"
                    disabled={isSigningOut}
                    onClick={() =>
                      signOut.mutate(undefined, {
                        onSuccess: async () => {
                          setAccountMenuOpen(false)
                          await clearSessionData(queryClient)
                          await navigate({ to: '/login', replace: true })
                        },
                      })
                    }
                  >
                    <svg
                      aria-hidden="true"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="2"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      className="size-4 shrink-0"
                    >
                      <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4M9 12h12m-4-4 4 4-4 4" />
                    </svg>
                    {isSigningOut ? 'Signing out…' : 'Sign out'}
                  </button>
                  {signOut.error && (
                    <p role="alert" className="px-3 py-2 text-sm text-error">
                      {signOut.error.message}
                    </p>
                  )}
                </div>
              )}
            </fieldset>
          ) : (
            <Link to="/login" className="btn btn-primary btn-sm">
              Account access
            </Link>
          )}
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
