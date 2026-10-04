import { useQueryClient } from '@tanstack/react-query'
import { createFileRoute, Link, useNavigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { ApiError } from '~/api/apiClient'
import queries from '~/api/queries'
import { Button } from '~/components/button'
import { PageHeader } from '~/components/page-header'
import { useAuth } from '~/hooks/use-auth'
import { useTerminalWorkspace } from '../-components/terminal-workspace'
import { ConnectDrawer } from './-components/connect-drawer'

export const Route = createFileRoute('/_authed/workspace/')({
  component: WorkspacePage,
})

function WorkspacePage() {
  const { account, isSigningOut } = useAuth()
  const client = useQueryClient()
  const navigate = useNavigate()
  const connections = queries.connections.useQuery(account?.id ?? '')
  const { hasTerminal, inspect } = useTerminalWorkspace()
  const [pickerOpen, setPickerOpen] = useState(false)
  useEffect(() => {
    if (
      connections.error instanceof ApiError &&
      connections.error.status === 401
    )
      client.setQueryData(queries.auth.currentUser.queryKey, null)
  }, [connections.error, client])
  return (
    <section className="mx-auto max-w-6xl px-6 pt-12 pb-6">
      <PageHeader
        title="Workspace"
        description="Open a shell using one of your saved connections."
      >
        <Button
          type="button"
          className="btn-outline"
          disabled={isSigningOut || !connections.isSuccess || hasTerminal}
          onClick={() => setPickerOpen(true)}
        >
          Connect
        </Button>
      </PageHeader>
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
            disabled={connections.isFetching || isSigningOut}
            onClick={() => void connections.refetch()}
          >
            Retry connection list
          </Button>
        </div>
      )}
      {!hasTerminal && (
        <div className="mt-8 rounded-box border border-dashed border-base-300 bg-base-200 p-10 text-center">
          <h2 className="text-xl font-semibold">No active terminal</h2>
          <p className="mt-3 text-base-content/75">
            Choose Connect to select a destination and review its host
            fingerprint.
          </p>
          {connections.isSuccess && connections.data.length === 0 && (
            <p className="mt-4">
              Start by{' '}
              <Link to="/keys" className="link">
                uploading an SSH key
              </Link>{' '}
              and{' '}
              <Link to="/connections" className="link">
                adding or importing a connection
              </Link>
              .
            </p>
          )}
        </div>
      )}
      {pickerOpen && (
        <ConnectDrawer
          connections={connections.data ?? []}
          onClose={() => setPickerOpen(false)}
          onAdd={() => void navigate({ to: '/connections' })}
          onSelect={(connection) => {
            setPickerOpen(false)
            inspect(connection)
          }}
        />
      )}
    </section>
  )
}
