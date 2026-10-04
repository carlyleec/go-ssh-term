import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { ApiError } from '~/api/apiClient'
import queries, { type SavedConnection } from '~/api/queries'
import { Button } from '~/components/button'
import { PageHeader } from '~/components/page-header'
import { useAuth } from '~/hooks/use-auth'
import { ConnectionDrawer } from './-components/connection-drawer'
import { ImportDrawer } from './-components/import-drawer'

export const Route = createFileRoute('/_authed/connections/')({
  component: ConnectionsPage,
})

function ConnectionsPage() {
  const { account, isSigningOut } = useAuth()
  const client = useQueryClient()
  const navigate = useNavigate()
  const accountID = account?.id ?? ''
  const connections = queries.connections.useQuery(accountID)
  const [drawer, setDrawer] = useState<
    'add' | 'import' | SavedConnection | null
  >(null)
  const [confirmation, setConfirmation] = useState<SavedConnection | null>(null)
  const [notice, setNotice] = useState('')
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
  if (!account) return null
  const busy = remove.isPending || isSigningOut
  return (
    <section className="mx-auto max-w-6xl px-6 py-12">
      <PageHeader
        title="Connections"
        description="Manage saved hosts and their connection settings."
      >
        <Button
          type="button"
          className="btn-outline"
          disabled={busy}
          onClick={() => setDrawer('add')}
        >
          Add connection
        </Button>
        <Button
          type="button"
          className="btn-outline"
          disabled={busy}
          onClick={() => setDrawer('import')}
        >
          Import SSH config
        </Button>
      </PageHeader>
      <p role="status" className="mt-4">
        {notice}
      </p>
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
              Open SSH Keys in the navbar and upload{' '}
              <code className="break-all">demo/keys/demo_ed25519</code> from the
              repository. This key is only for the local demo.
            </li>
            <li>
              Open Import SSH config and select <code>demo/ssh_config</code>.
              Choose your uploaded key for each host, check the selection, and
              confirm the import.
            </li>
            <li>
              Open Workspace and use Connect to choose your saved destination,
              verify its host fingerprint, and open a terminal. Try{' '}
              <code>cat /host-info.txt</code>.
            </li>
          </ol>
        </div>
      )}
      {!!connections.data?.length && (
        <div className="mt-6">
          <ul className="space-y-4">
            {connections.data.map((connection) => (
              <li
                key={connection.id}
                className="rounded-box border border-base-300 p-5"
              >
                <div className="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-4">
                  <div className="min-w-0">
                    <h2 className="font-semibold break-words">
                      {connection.name}
                    </h2>
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
                  <div className="flex flex-col gap-2 sm:flex-row">
                    <Button
                      type="button"
                      className="btn-outline btn-sm"
                      disabled={busy}
                      aria-label={`Edit ${connection.name}`}
                      onClick={() => {
                        setConfirmation(null)
                        setDrawer(connection)
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
                    <div className="mt-3 flex flex-wrap justify-end gap-2">
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
      {drawer === 'import' && (
        <ImportDrawer
          accountID={accountID}
          onClose={() => setDrawer(null)}
          onImported={() => {
            setDrawer(null)
            setNotice(
              'Connections imported. Open Workspace to start a terminal.',
            )
          }}
        />
      )}
      {(drawer === 'add' ||
        (typeof drawer === 'object' && drawer !== null)) && (
        <ConnectionDrawer
          accountID={accountID}
          connection={
            typeof drawer === 'object' ? (drawer ?? undefined) : undefined
          }
          onClose={() => setDrawer(null)}
          onManageKeys={() => void navigate({ to: '/keys' })}
        />
      )}
    </section>
  )
}
