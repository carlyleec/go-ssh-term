import { useEffect, useRef, useState } from 'react'
import type { SavedConnection } from '~/api/queries'
import { Button } from '~/components/button'
import type { TerminalState } from './terminal-runtime'

export function TerminalPanel({
  connection,
  onClose,
  onReconnect,
  reconnectPending = false,
}: {
  connection: SavedConnection
  onClose: () => void
  onReconnect?: () => void
  reconnectPending?: boolean
}) {
  const container = useRef<HTMLDivElement>(null)
  const [status, setStatus] = useState<TerminalState>({
    state: 'connecting',
    message: 'Opening terminal…',
  })
  useEffect(() => {
    let canceled = false
    let dispose: (() => void) | undefined
    void import('./terminal-runtime')
      .then(({ mountTerminal }) => {
        if (canceled || !container.current) return
        dispose = mountTerminal(container.current, connection.id, setStatus)
      })
      .catch(() => {
        if (!canceled)
          setStatus({
            state: 'failed',
            message: 'Could not load the terminal. Close it and try again.',
          })
      })
    return () => {
      canceled = true
      dispose?.()
    }
  }, [connection.id])
  return (
    <section
      className="mt-6 overflow-hidden rounded-box border border-base-300"
      aria-label={`Terminal for ${connection.name}`}
    >
      <div className="flex flex-wrap items-center justify-between gap-3 bg-base-200 p-4">
        <div className="min-w-0">
          <h2 className="font-semibold break-words">
            {connection.name} terminal
          </h2>
          <p className="break-all text-sm">
            {connection.username}@{connection.host}:{connection.port}
          </p>
          <p
            role={status.state === 'failed' ? 'alert' : 'status'}
            className="mt-1 text-sm"
          >
            {status.state[0]?.toUpperCase()}
            {status.state.slice(1)}
            {status.message ? ` — ${status.message}` : ''}
          </p>
        </div>
        <div className="flex gap-2">
          {(status.state === 'failed' || status.state === 'disconnected') &&
            onReconnect && (
              <Button
                type="button"
                className="btn-primary btn-sm"
                disabled={reconnectPending}
                onClick={onReconnect}
              >
                {reconnectPending ? 'Preparing reconnect…' : 'Reconnect'}
              </Button>
            )}
          <Button
            type="button"
            className="btn-outline btn-sm"
            onClick={onClose}
          >
            Close terminal
          </Button>
        </div>
      </div>
      <div className="bg-base-100 p-3">
        <div ref={container} className="h-[55vh] min-h-64" />
      </div>
      <p className="px-4 py-3 text-sm text-base-content/75">
        Close this terminal before opening another. Leaving this page ends the
        shell. Reconnect starts a fresh shell without previous output, commands,
        or working directory.
      </p>
    </section>
  )
}
