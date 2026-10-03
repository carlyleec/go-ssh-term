import { useMutation, useQuery } from '@tanstack/react-query'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'

import { AccessError, AccessPending } from '../auth/access-status'
import { currentUserOptions, requireAccount } from '../auth/current-user'

import { clearSessionData, logout } from '../auth/logout'
import { KeyModal } from '../sshkeys/key-modal'

export const Route = createFileRoute('/connections')({
  beforeLoad: ({ context }) => requireAccount(context.queryClient),
  pendingComponent: AccessPending,
  errorComponent: AccessError,
  component: ConnectionsPage,
})

function ConnectionsPage() {
  const { queryClient } = Route.useRouteContext()
  const navigate = useNavigate()
  const leaving = useRef(false)
  const [keysOpen, setKeysOpen] = useState(false)
  const currentUser = useQuery({
    ...currentUserOptions,
    refetchOnMount: false,
    refetchInterval: 30_000,
    refetchOnWindowFocus: true,
    refetchOnReconnect: true,
  })
  const signOut = useMutation({
    mutationFn: logout,
    onSuccess: async () => {
      leaving.current = true
      await clearSessionData(queryClient)
      await navigate({ to: '/login', replace: true })
    },
    retry: false,
    networkMode: 'always',
  })
  useEffect(() => {
    if (currentUser.data === null && !leaving.current) {
      leaving.current = true
      void clearSessionData(queryClient).then(() =>
        navigate({ to: '/login', replace: true }),
      )
    }
  }, [currentUser.data, queryClient, navigate])

  if (currentUser.data === null)
    return (
      <p role="status" className="px-6 py-20">
        Your session has ended. Returning to sign in…
      </p>
    )
  if (currentUser.isError) return <AccessError />
  if (!currentUser.data) return <AccessPending />
  const account = currentUser.data
  return (
    <section className="mx-auto max-w-2xl px-6 py-20">
      <span className="badge badge-outline mb-5">Coming soon</span>
      <h1 className="text-3xl font-bold">Connections</h1>
      <p className="mt-5 leading-relaxed text-base-content/75">
        Upload your SSH keys to prepare your workspace. Saved hosts,
        configuration import, and SSH terminals are coming next.
      </p>
      <p className="mt-4 leading-relaxed text-base-content/75">
        Signed in as {account.display_name}.
      </p>
      <button
        type="button"
        className="btn btn-primary mt-6 mr-3"
        disabled={signOut.isPending}
        onClick={() => setKeysOpen(true)}
      >
        Manage SSH keys
      </button>
      {keysOpen && (
        <KeyModal accountID={account.id} onClose={() => setKeysOpen(false)} />
      )}
      <button
        type="button"
        className="btn btn-outline mt-6"
        disabled={signOut.isPending}
        onClick={() => signOut.mutate()}
      >
        {signOut.isPending ? 'Signing out…' : 'Sign out'}
      </button>
      {signOut.error && (
        <p role="alert" className="mt-3 text-error">
          {signOut.error.message}
        </p>
      )}
      <Link to="/" className="btn btn-primary mt-8">
        Back to home
      </Link>
    </section>
  )
}
