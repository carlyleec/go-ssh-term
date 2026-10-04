import { createFileRoute, Link } from '@tanstack/react-router'
import { useState } from 'react'
import { Button } from '../../../components/button'
import { useAuth } from '../../../hooks/use-auth'
import { KeyModal } from './-components/key-modal'

export const Route = createFileRoute('/_authed/connections/')({
  component: ConnectionsPage,
})

function ConnectionsPage() {
  const { account, signOut } = useAuth()
  const [keysOpen, setKeysOpen] = useState(false)
  if (!account) return null
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
      <Button
        type="button"
        className="btn-primary mt-6 mr-3"
        disabled={signOut.isPending}
        onClick={() => setKeysOpen(true)}
      >
        Manage SSH keys
      </Button>
      {keysOpen && (
        <KeyModal accountID={account.id} onClose={() => setKeysOpen(false)} />
      )}
      <Button
        type="button"
        className="btn-outline mt-6"
        disabled={signOut.isPending}
        onClick={() => signOut.mutate()}
      >
        {signOut.isPending ? 'Signing out…' : 'Sign out'}
      </Button>
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
