import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { ApiError } from '~/api/apiClient'
import type {
  SchemaImportPreview,
  SchemaImportSelection,
} from '~/api/generated/schema.gen'
import queries from '~/api/queries'
import { Button } from '~/components/button'

export function ImportDrawer({
  accountID,
  onClose,
  onImported,
}: {
  accountID: string
  onClose: () => void
  onImported: () => void
}) {
  const client = useQueryClient()
  const keys = queries.keys.useQuery(accountID)
  const connections = queries.connections.useQuery(accountID)
  const dialog = useRef<HTMLDialogElement>(null)
  const mounted = useRef(false)
  const active = useRef(false)
  const [config, setConfig] = useState('')
  const [preview, setPreview] = useState<SchemaImportPreview | null>(null)
  const [selections, setSelections] = useState<SchemaImportSelection[]>([])
  const [reviewed, setReviewed] = useState(false)
  const [error, setError] = useState('')
  const [reading, setReading] = useState(false)
  const parse = useMutation(queries.connections.previewImport)
  const save = useMutation(queries.connections.confirmImport)
  const busy = reading || parse.isPending || save.isPending
  useEffect(() => {
    mounted.current = true
    dialog.current?.showModal()
    return () => {
      mounted.current = false
    }
  }, [])
  useEffect(() => {
    for (const problem of [keys.error, connections.error]) {
      if (problem instanceof ApiError && problem.status === 401)
        client.setQueryData(queries.auth.currentUser.queryKey, null)
    }
  }, [keys.error, connections.error, client])
  function report(problem: unknown) {
    if (!mounted.current) return
    if (problem instanceof ApiError && problem.status === 401)
      client.setQueryData(queries.auth.currentUser.queryKey, null)
    else
      setError(
        problem instanceof Error
          ? problem.message
          : 'Could not import config. Try again.',
      )
  }
  function change(next: SchemaImportSelection[]) {
    setSelections(next)
    setReviewed(false)
    setError('')
  }
  function update(name: string, fields: Partial<SchemaImportSelection>) {
    change(
      selections.map((item) =>
        item.name === name ? { ...item, ...fields } : item,
      ),
    )
  }
  async function load(file: File | undefined) {
    if (active.current) return
    setReviewed(false)
    setPreview(null)
    setSelections([])
    setConfig('')
    setError('')
    if (!file) return
    if (file.size > 65536) {
      setError('Config must be at most 64 KiB.')
      return
    }
    active.current = true
    setReading(true)
    try {
      const text = new TextDecoder('utf-8', { fatal: true }).decode(
        await file.arrayBuffer(),
      )
      const result = await parse.mutateAsync({ config: text })
      if (!mounted.current) return
      setConfig(text)
      setPreview(result)
      setSelections(
        result.entries.map((entry) => ({
          name: entry.name,
          ssh_key_id: '',
          jump_connection_id: '',
          jump_updated_at: '',
        })),
      )
    } catch (problem) {
      report(problem)
    } finally {
      active.current = false
      if (mounted.current) setReading(false)
    }
  }
  async function review() {
    if (active.current) return
    active.current = true
    setReviewed(false)
    setError('')
    try {
      const result = await parse.mutateAsync({ config, selections })
      if (mounted.current) {
        setPreview(result)
        setReviewed(result.can_confirm)
      }
    } catch (problem) {
      report(problem)
    } finally {
      active.current = false
    }
  }
  async function confirm() {
    if (active.current || !reviewed || !preview?.can_confirm) return
    active.current = true
    setError('')
    try {
      await save.mutateAsync({ config, selections })
      await client.invalidateQueries({
        queryKey: queries.connections.options(accountID).queryKey,
      })
      if (mounted.current) onImported()
    } catch (problem) {
      setReviewed(false)
      report(problem)
    } finally {
      active.current = false
    }
  }
  return (
    <dialog
      ref={dialog}
      className="side-drawer"
      aria-labelledby="import-title"
      onClose={onClose}
      onCancel={(event) => {
        if (active.current) event.preventDefault()
      }}
    >
      <div className="drawer-panel drawer-panel-wide">
        <div className="drawer-heading">
          <h2 id="import-title" className="text-xl font-semibold">
            Import SSH config
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
        <p className="mt-3">
          Choose demo/ssh_config or a supported config file (64 KiB maximum).
          Nothing is saved until you confirm.
        </p>
        <p className="mt-2 text-sm">
          Use one literal Host alias per block with HostName, User,
          IdentityFile, optional Port, and one ProxyJump alias. Patterns,
          inheritance, commands, quoting, and other directives are unsupported.
          IdentityFile is a hint only; choose an uploaded key for each host.
        </p>
        <label className="form-control mt-4 block">
          <span>SSH config file</span>
          <input
            type="file"
            className="file-input file-input-bordered mt-2 w-full"
            disabled={busy}
            onChange={(event) => void load(event.target.files?.[0])}
          />
        </label>
        {busy && (
          <p role="status" className="mt-3">
            {save.isPending ? 'Saving import…' : 'Checking config…'}
          </p>
        )}
        {error && (
          <p role="alert" className="mt-3 text-error">
            {error}
          </p>
        )}
        {(keys.isPending || connections.isPending) && (
          <p role="status">Loading keys and saved connections…</p>
        )}
        {(keys.isError || connections.isError) && (
          <div role="alert" className="mt-3 text-error">
            Could not load keys or saved connections.
            <Button
              type="button"
              disabled={busy}
              onClick={() => {
                setReviewed(false)
                void keys.refetch()
                void connections.refetch()
              }}
            >
              Retry import data
            </Button>
          </div>
        )}
        {keys.isSuccess && keys.data.length === 0 && (
          <p role="alert" className="mt-3">
            Upload a key in Manage SSH keys before importing.
          </p>
        )}
        {preview && (
          <>
            {preview.diagnostics.map((item) => (
              <p
                role="alert"
                className="mt-2 text-error"
                key={`${item.line}-${item.message}`}
              >
                Line {item.line}: {item.message}
              </p>
            ))}
            {preview.diagnostics.length === 0 && (
              <div className="mt-5 space-y-4">
                {preview.entries.map((entry) => {
                  const selection = selections.find(
                    (item) => item.name === entry.name,
                  )
                  const selectedJump = selections.some(
                    (item) => item.name === entry.jump,
                  )
                  const jumpOptions =
                    connections.data?.filter(
                      (item) =>
                        item.name === entry.jump && !item.jump_connection_id,
                    ) ?? []
                  const conflict = connections.data?.some(
                    (item) => item.name === entry.name,
                  )
                  return (
                    <fieldset
                      className="rounded-box border border-base-300 p-4"
                      key={entry.name}
                      disabled={busy}
                    >
                      <label className="flex items-center gap-3 font-semibold">
                        <input
                          type="checkbox"
                          className="checkbox"
                          checked={!!selection}
                          onChange={(event) => {
                            const next = event.target.checked
                              ? [
                                  ...selections,
                                  {
                                    name: entry.name,
                                    ssh_key_id: '',
                                    jump_connection_id: '',
                                    jump_updated_at: '',
                                  },
                                ]
                              : selections.filter(
                                  (item) => item.name !== entry.name,
                                )
                            change(
                              next.map((item) =>
                                preview.entries.find(
                                  (candidate) => candidate.name === item.name,
                                )?.jump === entry.name
                                  ? {
                                      ...item,
                                      jump_connection_id: '',
                                      jump_updated_at: '',
                                    }
                                  : item,
                              ),
                            )
                          }}
                        />
                        {entry.name}
                      </label>
                      <p className="mt-2 break-all font-mono text-sm">
                        {entry.username}@{entry.host}:{entry.port}
                      </p>
                      <p className="mt-2 break-all text-sm">
                        Identity hint: {entry.identity}
                      </p>
                      {conflict && (
                        <p className="mt-2 text-error">
                          Name already exists. Deselect this host or change its
                          alias in the file.
                        </p>
                      )}
                      {selection && (
                        <label className="mt-3 block">
                          Key for {entry.name}
                          <select
                            className="select select-bordered mt-1 w-full"
                            value={selection.ssh_key_id}
                            onChange={(event) =>
                              update(entry.name, {
                                ssh_key_id: event.target.value,
                              })
                            }
                          >
                            <option value="">Choose uploaded key</option>
                            {keys.data?.map((key) => (
                              <option key={key.id} value={key.id}>
                                {key.name} — {key.public_fingerprint} ({key.id})
                              </option>
                            ))}
                          </select>
                        </label>
                      )}
                      {entry.jump && (
                        <p className="mt-3">
                          Jump through: {entry.jump}
                          {selectedJump ? ' (selected entry)' : ''}
                        </p>
                      )}
                      {selection && entry.jump && !selectedJump && (
                        <label className="mt-2 block">
                          Existing jump for {entry.name}
                          <select
                            className="select select-bordered mt-1 w-full"
                            value={selection.jump_connection_id}
                            onChange={(event) => {
                              const jump = jumpOptions.find(
                                (item) => item.id === event.target.value,
                              )
                              update(entry.name, {
                                jump_connection_id: jump?.id ?? '',
                                jump_updated_at: jump?.updated_at ?? '',
                              })
                            }}
                          >
                            <option value="">
                              Choose existing {entry.jump}, or select its entry
                            </option>
                            {jumpOptions.map((jump) => (
                              <option key={jump.id} value={jump.id}>
                                {jump.name} — {jump.username}@{jump.host}:
                                {jump.port} ({jump.id})
                              </option>
                            ))}
                          </select>
                        </label>
                      )}
                    </fieldset>
                  )
                })}
              </div>
            )}
            {preview.issues.map((item) => (
              <p
                role="alert"
                className="mt-2 text-error"
                key={`${item.name}-${item.message}`}
              >
                {item.name}: {item.message}
              </p>
            ))}
            {reviewed && (
              <p role="status" className="mt-4">
                Ready to import {selections.length} connections. Review the
                settings above, then confirm.
              </p>
            )}
          </>
        )}
        <div className="drawer-actions">
          <Button
            type="button"
            className="btn-outline"
            disabled={
              busy ||
              !preview ||
              preview.diagnostics.length > 0 ||
              selections.length === 0 ||
              !keys.isSuccess ||
              !connections.isSuccess
            }
            onClick={() => void review()}
          >
            Check selection
          </Button>
          <Button
            type="button"
            className="btn-primary"
            disabled={
              busy ||
              !reviewed ||
              !preview?.can_confirm ||
              !keys.isSuccess ||
              !connections.isSuccess
            }
            onClick={() => void confirm()}
          >
            Confirm import
          </Button>
          <Button
            type="button"
            className="btn-ghost"
            disabled={busy}
            onClick={() => dialog.current?.close()}
          >
            Cancel
          </Button>
        </div>
      </div>
    </dialog>
  )
}
