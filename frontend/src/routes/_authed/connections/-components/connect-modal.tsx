import { useEffect, useRef } from 'react'
import type { SavedConnection } from '~/api/queries'
import { Button } from '~/components/button'

export function ConnectModal({
  connections,
  onClose,
  onSelect,
  onAdd,
}: {
  connections: SavedConnection[]
  onClose: () => void
  onSelect: (connection: SavedConnection) => void
  onAdd: () => void
}) {
  const dialog = useRef<HTMLDialogElement>(null)
  useEffect(() => {
    dialog.current?.showModal()
  }, [])
  return (
    <dialog
      ref={dialog}
      className="modal"
      aria-labelledby="connect-title"
      onClose={onClose}
    >
      <div className="modal-box">
        <div className="flex items-center justify-between gap-4">
          <h2 id="connect-title" className="text-2xl font-bold">
            Choose a connection
          </h2>
          <Button
            type="button"
            className="btn-ghost"
            onClick={() => dialog.current?.close()}
          >
            Close
          </Button>
        </div>
        <p className="mt-3 text-base-content/75">
          Choose a saved destination and verify its host fingerprint to open a
          shell.
        </p>
        {connections.length === 0 ? (
          <div className="mt-5">
            <p>No saved connections yet.</p>
            <Button type="button" className="btn-primary mt-3" onClick={onAdd}>
              Add connection
            </Button>
          </div>
        ) : (
          <ul className="mt-5 space-y-3">
            {connections.map((connection) => (
              <li key={connection.id}>
                <Button
                  type="button"
                  className="btn-outline h-auto w-full justify-start py-3 text-left"
                  onClick={() => onSelect(connection)}
                >
                  <span className="min-w-0">
                    <span className="block break-words">{connection.name}</span>
                    <span className="block break-all text-xs font-normal">
                      {connection.username}@{connection.host}:{connection.port}
                    </span>
                  </span>
                </Button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </dialog>
  )
}
