import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { ApiError } from '~/api/apiClient'
import queries, { type SavedConnection } from '~/api/queries'
import { Button } from '~/components/button'

export function HostModal({
  accountID,
  connection,
  onClose,
  onConnect,
}: {
  accountID: string
  connection: SavedConnection
  onClose: () => void
  onConnect: () => void
}) {
  const dialog = useRef<HTMLDialogElement>(null)
  const mounted = useRef(false)
  const active = useRef(false)
  const [confirmReset, setConfirmReset] = useState(false)
  const client = useQueryClient()
  const options = queries.connections.hostOptions(accountID, connection.id)
  const host = useQuery(options)
  useEffect(() => {
    mounted.current = true
    dialog.current?.showModal()
    return () => {
      mounted.current = false
    }
  }, [])
  function expire(error: Error) {
    if (mounted.current && error instanceof ApiError && error.status === 401)
      client.setQueryData(queries.auth.currentUser.queryKey, null)
  }
  useEffect(() => {
    if (host.error instanceof ApiError && host.error.status === 401)
      client.setQueryData(queries.auth.currentUser.queryKey, null)
  }, [host.error, client])
  const approve = useMutation({
    ...queries.connections.approveHost,
    onError: expire,
    onSuccess: (data) => {
      if (mounted.current) client.setQueryData(options.queryKey, data)
    },
  })
  const reset = useMutation({
    ...queries.connections.resetTrust,
    onError: expire,
    onSuccess: async () => {
      if (!mounted.current) return
      setConfirmReset(false)
      await client.invalidateQueries({ queryKey: options.queryKey })
    },
  })
  const busy = approve.isPending || reset.isPending
  const view = host.isSuccess && !host.isFetching ? host.data : undefined
  async function decide(kind: 'approve' | 'reset') {
    if (active.current || !view) return
    active.current = true
    try {
      const decision = {
        host: view.host,
        port: view.port,
        fingerprint:
          kind === 'approve' ? view.fingerprint : view.trusted_fingerprint,
      }
      if (kind === 'approve')
        await approve.mutateAsync({ id: connection.id, decision })
      else await reset.mutateAsync({ id: connection.id, decision })
    } catch {
      /* Preserve the decision error for review and explicit retry. */
    } finally {
      active.current = false
    }
  }
  return (
    <dialog
      ref={dialog}
      className="modal"
      aria-labelledby="host-title"
      onClose={onClose}
      onCancel={(event) => {
        if (busy || active.current) event.preventDefault()
      }}
    >
      <div className="modal-box max-w-2xl">
        <div className="flex items-center justify-between gap-4">
          <h2 id="host-title" className="text-2xl font-bold">
            Verify SSH host
          </h2>
          <Button
            type="button"
            className="btn-ghost"
            disabled={busy}
            onClick={() => dialog.current?.close()}
          >
            Close
          </Button>
        </div>
        <p className="mt-4 break-words">{connection.name}</p>
        {host.isFetching && (
          <p role="status" className="mt-4">
            Inspecting host fingerprint…
          </p>
        )}
        {host.isError && (
          <p role="alert" className="mt-4 text-error">
            {host.error.message}
          </p>
        )}
        {view && (
          <div className="mt-4 space-y-4">
            <p className="break-all font-mono">
              {view.host}:{view.port}
            </p>
            <p>Presented key ({view.algorithm})</p>
            <p className="break-all font-mono">{view.fingerprint}</p>
            {view.state === 'unknown' && (
              <>
                <p>
                  This host is not trusted yet. Compare this fingerprint with
                  the host’s trusted configuration before approving. No SSH
                  login has been attempted.
                </p>
                <div className="flex flex-wrap gap-3">
                  <Button
                    type="button"
                    className="btn-primary"
                    disabled={busy}
                    onClick={() => void decide('approve')}
                  >
                    {approve.isPending ? 'Approving…' : 'Approve fingerprint'}
                  </Button>
                  <Button
                    type="button"
                    className="btn-outline"
                    disabled={busy}
                    onClick={() => dialog.current?.close()}
                  >
                    Reject and close
                  </Button>
                </div>
              </>
            )}
            {view.state === 'trusted' && (
              <div>
                <p role="status">
                  Host fingerprint verified. Ready to open a shell.
                </p>
                <Button
                  type="button"
                  className="btn-primary mt-3"
                  disabled={busy}
                  onClick={onConnect}
                >
                  Open terminal
                </Button>
              </div>
            )}
            {view.state === 'changed' && (
              <>
                <p role="alert" className="text-error">
                  The SSH host key changed. Connection blocked. Verify why the
                  host identity changed before removing its old trust.
                </p>
                <p>Previously trusted fingerprint</p>
                <p className="break-all font-mono">
                  {view.trusted_fingerprint}
                </p>
                {confirmReset ? (
                  <div>
                    <p>
                      Forget this trusted key for your account and this
                      endpoint? Every saved connection to this endpoint will
                      require fresh approval. This does not approve the new key.
                    </p>
                    <div className="mt-3 flex flex-wrap gap-3">
                      <Button
                        type="button"
                        className="btn-error"
                        disabled={busy}
                        onClick={() => void decide('reset')}
                      >
                        {reset.isPending ? 'Resetting…' : 'Confirm trust reset'}
                      </Button>
                      <Button
                        type="button"
                        disabled={busy}
                        onClick={() => setConfirmReset(false)}
                      >
                        Cancel reset
                      </Button>
                    </div>
                  </div>
                ) : (
                  <Button
                    type="button"
                    className="btn-outline"
                    disabled={busy}
                    onClick={() => {
                      reset.reset()
                      setConfirmReset(true)
                    }}
                  >
                    Reset host trust
                  </Button>
                )}
              </>
            )}
          </div>
        )}
        {(approve.error || reset.error) && (
          <p role="alert" className="mt-4 text-error">
            {approve.error?.message ?? reset.error?.message}
          </p>
        )}
        <Button
          type="button"
          className="btn-outline mt-5"
          disabled={busy || host.isFetching}
          onClick={() => {
            approve.reset()
            reset.reset()
            setConfirmReset(false)
            void host.refetch()
          }}
        >
          Inspect again
        </Button>
      </div>
    </dialog>
  )
}
