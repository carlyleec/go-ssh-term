import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, Link } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { ApiError } from '~/api/apiClient'
import queries, { type SavedConnection } from '~/api/queries'
import { Button } from '~/components/button'
import { useAuth } from '~/hooks/use-auth'
import { ConnectModal } from './-components/connect-modal'
import { ConnectionModal } from './-components/connection-modal'
import { HostModal } from './-components/host-modal'
import { ImportModal } from './-components/import-modal'
import { KeyModal } from './-components/key-modal'
import { TerminalPanel } from './-components/terminal-panel'

export const Route = createFileRoute('/_authed/connections/')({
  component: ConnectionsPage,
})

function ConnectionsPage() {
  const { account, signOut } = useAuth()
  const client = useQueryClient()
  const accountID = account?.id ?? ''
  const connections = queries.connections.useQuery(accountID)
  const [modal, setModal] = useState<
    'keys' | 'add' | 'connect' | 'import' | SavedConnection | null
  >(null)
  const [confirmation, setConfirmation] = useState<SavedConnection | null>(null)
  const [hostTarget, setHostTarget] = useState<SavedConnection | null>(null)
  const [terminal, setTerminal] = useState<SavedConnection | null>(null)
  const [notice, setNotice] = useState('')
  const [terminalAttempt, setTerminalAttempt] = useState(0)
  const [reconnectPending, setReconnectPending] = useState(false)
  const reconnectRequest = useRef(0)
  const reconnectActive = useRef(false)
  const mounted = useRef(false)
  const active = useRef(false)
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])
  useEffect(() => {
    if (
      connections.error instanceof ApiError &&
      connections.error.status === 401
    )
      client.setQueryData(queries.auth.currentUser.queryKey, null)
  }, [connections.error, client])
  const remove = useMutation({
    ...queries.connections.remove,
    onSuccess: async () => {
      if (!mounted.current) return
      setConfirmation(null)
      setNotice('Connection deleted.')
      await client.invalidateQueries({
        queryKey: queries.connections.options(accountID).queryKey,
      })
    },
    onError: (error) => {
      if (mounted.current && error instanceof ApiError && error.status === 401)
        client.setQueryData(queries.auth.currentUser.queryKey, null)
    },
  })
  async function reconnect() {
    if (!terminal || reconnectActive.current || signOut.isPending) return
    reconnectActive.current = true
    const request = ++reconnectRequest.current
    setReconnectPending(true)
    setNotice('')
    try {
      const latest = await client.fetchQuery({
        ...queries.connections.options(accountID),
        staleTime: 0,
      })
      if (!mounted.current || request !== reconnectRequest.current) return
      const target = latest.find((item) => item.id === terminal.id)
      if (!target) {
        setNotice(
          'This saved connection was deleted. Close the terminal and choose another connection.',
        )
        return
      }
      setHostTarget(target)
    } catch (error) {
      if (!mounted.current || request !== reconnectRequest.current) return
      if (error instanceof ApiError && error.status === 401)
        client.setQueryData(queries.auth.currentUser.queryKey, null)
      else setNotice('Could not prepare reconnect. Try again.')
    } finally {
      if (mounted.current && request === reconnectRequest.current) {
        reconnectActive.current = false
        setReconnectPending(false)
      }
    }
  }
  if (!account) return null
  const busy = remove.isPending || signOut.isPending
  return (
    <section className="mx-auto max-w-5xl px-6 py-12">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-3xl font-bold">Connections</h1>
          <p className="mt-2 text-base-content/75">
            Signed in as {account.display_name}.
          </p>
        </div>
        <Button
          type="button"
          className="btn-ghost"
          disabled={busy}
          onClick={() => signOut.mutate()}
        >
          {signOut.isPending ? 'Signing out…' : 'Sign out'}
        </Button>
      </div>
      {signOut.error && (
        <p role="alert" className="mt-3 text-error">
          {signOut.error.message}
        </p>
      )}
      <div className="mt-6 flex flex-wrap gap-3">
        <Button
          type="button"
          className="btn-primary"
          disabled={busy || !connections.isSuccess || terminal !== null}
          onClick={() => {
            setNotice('')
            setModal('connect')
          }}
        >
          Connect
        </Button>
        <Button
          type="button"
          className="btn-outline"
          disabled={busy}
          onClick={() => setModal('add')}
        >
          Add connection
        </Button>
        <Button
          type="button"
          className="btn-outline"
          disabled={busy}
          onClick={() => setModal('keys')}
        >
          Manage SSH keys
        </Button>
        <Button
          type="button"
          className="btn-outline"
          disabled={busy}
          onClick={() => setModal('import')}
        >
          Import SSH config
        </Button>
      </div>
      <p role="status" className="mt-4">
        {notice}
      </p>
      {terminal && (
        <TerminalPanel
          key={terminalAttempt}
          connection={terminal}
          onReconnect={() => void reconnect()}
          reconnectPending={reconnectPending || busy || hostTarget !== null}
          onClose={() => {
            reconnectRequest.current++
            reconnectActive.current = false
            setReconnectPending(false)
            setHostTarget(null)
            setTerminal(null)
          }}
        />
      )}
      {connections.isPending && (
        <p role="status" className="mt-6">
          Loading connections…
        </p>
      )}
      {connections.isError && (
        <div className="mt-6">
          <p role="alert" className="text-error">
            {connections.error.message}
          </p>
          <Button
            type="button"
            className="btn-outline mt-3"
            disabled={busy || connections.isFetching}
            onClick={() => void connections.refetch()}
          >
            Retry connection list
          </Button>
        </div>
      )}
      {connections.isSuccess && connections.data.length === 0 && (
        <div className="rounded-box mt-6 border border-base-300 bg-base-200 p-6">
          <h2 className="text-xl font-semibold">Start with the local demo</h2>
          <ol className="mt-4 list-inside list-decimal space-y-3">
            <li>
              Start the lab with{' '}
              <code className="break-all">
                docker compose up --build -d --wait bastion target-1 target-2
              </code>
              .
            </li>
            <li>
              Open Manage SSH keys and upload{' '}
              <code className="break-all">demo/keys/demo_ed25519</code> from the
              repository. This key is only for the local demo.
            </li>
            <li>
              Open Import SSH config and select <code>demo/ssh_config</code>.
              Choose your uploaded key for each host, check the selection, and
              confirm the import.
            </li>
            <li>
              Use Connect to choose your saved destination, verify its host
              fingerprint, and open a terminal. Try{' '}
              <code>cat /host-info.txt</code>.
            </li>
          </ol>
        </div>
      )}
      {!!connections.data?.length && (
        <div className="mt-6">
          <h2 className="text-xl font-semibold">Saved connections</h2>
          <ul className="mt-4 space-y-4">
            {connections.data.map((connection) => (
              <li
                key={connection.id}
                className="rounded-box border border-base-300 p-5"
              >
                <div className="flex flex-wrap items-start justify-between gap-4">
                  <div className="min-w-0">
                    <h3 className="font-semibold break-words">
                      {connection.name}
                    </h3>
                    <p className="mt-1 break-all font-mono text-sm text-base-content/75">
                      {connection.username}@{connection.host}:{connection.port}
                    </p>
                    {connection.jump_connection_id && (
                      <p className="mt-1 text-sm">
                        Jump through:{' '}
                        {connections.data.find(
                          (item) => item.id === connection.jump_connection_id,
                        )?.name ?? 'Unavailable connection'}
                      </p>
                    )}
                  </div>
                  <div className="flex gap-2">
                    <Button
                      type="button"
                      className="btn-outline btn-sm"
                      disabled={busy}
                      aria-label={`Edit ${connection.name}`}
                      onClick={() => {
                        setConfirmation(null)
                        setModal(connection)
                      }}
                    >
                      Edit
                    </Button>
                    <Button
                      type="button"
                      className="btn-outline btn-sm"
                      disabled={busy}
                      aria-label={`Delete ${connection.name}`}
                      onClick={() => {
                        remove.reset()
                        setConfirmation(connection)
                        setNotice('')
                      }}
                    >
                      Delete
                    </Button>
                  </div>
                </div>
                {confirmation?.id === connection.id && (
                  <div className="mt-4">
                    <p>
                      Delete “{connection.name}”? This removes its saved
                      configuration and keeps the SSH key.
                    </p>
                    {remove.error && (
                      <p role="alert" className="mt-2 text-error">
                        {remove.error.message}
                      </p>
                    )}
                    <div className="mt-3 flex gap-2">
                      <Button
                        type="button"
                        className="btn-error"
                        disabled={busy}
                        onClick={() => {
                          if (active.current) return
                          active.current = true
                          remove.mutate(connection.id, {
                            onSettled: () => {
                              active.current = false
                            },
                          })
                        }}
                      >
                        {remove.isPending ? 'Deleting…' : 'Confirm deletion'}
                      </Button>
                      <Button
                        type="button"
                        className="btn-ghost"
                        disabled={busy}
                        onClick={() => setConfirmation(null)}
                      >
                        Cancel deletion
                      </Button>
                    </div>
                  </div>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}
      {modal === 'import' && (
        <ImportModal
          accountID={accountID}
          onClose={() => setModal(null)}
          onImported={() => {
            setModal(null)
            setNotice('Connections imported. Use Connect to open a terminal.')
          }}
        />
      )}
      {modal === 'keys' && (
        <KeyModal accountID={accountID} onClose={() => setModal(null)} />
      )}
      {(modal === 'add' || (typeof modal === 'object' && modal !== null)) && (
        <ConnectionModal
          accountID={accountID}
          connection={
            typeof modal === 'object' ? (modal ?? undefined) : undefined
          }
          onClose={() => setModal(null)}
          onManageKeys={() => setModal('keys')}
        />
      )}
      {modal === 'connect' && (
        <ConnectModal
          connections={connections.isSuccess ? connections.data : []}
          onClose={() => setModal(null)}
          onAdd={() => setModal('add')}
          onSelect={(connection) => {
            setHostTarget(connection)
            setModal(null)
          }}
        />
      )}
      {hostTarget && (
        <HostModal
          accountID={accountID}
          connection={hostTarget}
          onClose={() => setHostTarget(null)}
          onConnect={() => {
            if (!mounted.current || signOut.isPending) return
            setTerminalAttempt((attempt) => attempt + 1)
            setTerminal(hostTarget)
            setHostTarget(null)
          }}
        />
      )}
      <Link to="/" className="btn btn-ghost mt-8">
        Back to home
      </Link>
    </section>
  )
}
