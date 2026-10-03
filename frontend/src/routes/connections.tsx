import { createFileRoute, Link } from '@tanstack/react-router'

import { AccessError, AccessPending } from '../auth/access-status'
import { requireAccount } from '../auth/current-user'

export const Route = createFileRoute('/connections')({
  beforeLoad: ({ context }) => requireAccount(context.queryClient),
  pendingComponent: AccessPending,
  errorComponent: AccessError,
  component: ConnectionsPage,
})

function ConnectionsPage() {
  const { account } = Route.useRouteContext()
  return (
    <section className="mx-auto max-w-2xl px-6 py-20">
      <span className="badge badge-outline mb-5">Coming soon</span>
      <h1 className="text-3xl font-bold">Connections</h1>
      <p className="mt-5 leading-relaxed text-base-content/75">
        Your saved SSH hosts and terminal tabs will live here. Key management,
        configuration import, and SSH connections aren’t available yet.
      </p>
      <p className="mt-4 leading-relaxed text-base-content/75">
        Signed in as {account.display_name}.
      </p>
      <Link to="/" className="btn btn-primary mt-8">
        Back to home
      </Link>
    </section>
  )
}
