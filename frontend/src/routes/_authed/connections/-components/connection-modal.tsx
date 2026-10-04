import { useForm } from '@tanstack/react-form'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'
import { z } from 'zod'
import { ApiError } from '~/api/apiClient'
import queries, { type SavedConnection } from '~/api/queries'
import { Button } from '~/components/button'
import { Input } from '~/components/input'

export const connectionSchema = z.object({
  name: z
    .string()
    .refine(
      (v) =>
        v.trim().length > 0 &&
        [...v.trim()].length <= 64 &&
        !/\p{Cc}/u.test(v.trim()),
      'Enter a name with 1–64 characters and no control characters.',
    ),
  host: z.string().refine((value) => {
    const v = value.trim()
    if (
      z.ipv4().safeParse(v).success ||
      (z.ipv6().safeParse(v).success && !v.includes('%'))
    )
      return true
    const dns = v.replace(/\.$/, '')
    return (
      dns.length > 0 &&
      dns.length <= 253 &&
      !/^[\d.]+$/.test(dns) &&
      dns
        .split('.')
        .every((label) => /^[a-z\d](?:[a-z\d-]{0,61}[a-z\d])?$/i.test(label))
    )
  }, 'Enter a hostname or IP address without a URL, port, brackets, or zone.'),
  port: z
    .string()
    .refine(
      (v) => /^\d+$/.test(v) && Number(v) >= 1 && Number(v) <= 65535,
      'Enter a whole-number port from 1 to 65535.',
    ),
  username: z
    .string()
    .refine(
      (v) => /^[a-z\d_][a-z\d_.-]{0,63}$/i.test(v.trim()),
      'Use 1–64 letters, digits, underscores, dots, or hyphens; start with a letter, digit, or underscore.',
    ),
  ssh_key_id: z.string().min(1, 'Choose an SSH key.'),
  jump_connection_id: z.string(),
})

export function ConnectionModal({
  accountID,
  connection,
  onClose,
  onManageKeys,
}: {
  accountID: string
  connection?: SavedConnection
  onClose: () => void
  onManageKeys: () => void
}) {
  const client = useQueryClient()
  const dialog = useRef<HTMLDialogElement>(null)
  const mounted = useRef(false)
  const active = useRef(false)
  const keys = queries.keys.useQuery(accountID)
  const connections = queries.connections.useQuery(accountID)
  const isReferenced =
    connections.data?.some(
      (item) =>
        item.jump_connection_id === connection?.id && connection !== undefined,
    ) ?? false
  const jumps =
    connections.isSuccess && !isReferenced
      ? connections.data.filter(
          (item) => item.id !== connection?.id && !item.jump_connection_id,
        )
      : []
  const validJump = (id: string) => !id || jumps.some((item) => item.id === id)
  useEffect(() => {
    mounted.current = true
    dialog.current?.showModal()
    return () => {
      mounted.current = false
    }
  }, [])
  useEffect(() => {
    if (keys.error instanceof ApiError && keys.error.status === 401)
      client.setQueryData(queries.auth.currentUser.queryKey, null)
  }, [keys.error, client])
  useEffect(() => {
    if (
      connections.error instanceof ApiError &&
      connections.error.status === 401
    )
      client.setQueryData(queries.auth.currentUser.queryKey, null)
  }, [connections.error, client])
  const save = useMutation({
    ...queries.connections.save,
    onSuccess: async () => {
      if (!mounted.current) return
      await client.invalidateQueries({
        queryKey: queries.connections.options(accountID).queryKey,
      })
      if (mounted.current) dialog.current?.close()
    },
    onError: (error) => {
      if (mounted.current && error instanceof ApiError && error.status === 401)
        client.setQueryData(queries.auth.currentUser.queryKey, null)
    },
  })
  const form = useForm({
    defaultValues: {
      name: connection?.name ?? '',
      host: connection?.host ?? '',
      port: String(connection?.port ?? 22),
      username: connection?.username ?? '',
      ssh_key_id: connection?.ssh_key_id ?? '',
      jump_connection_id: connection?.jump_connection_id ?? '',
    },
    validators: { onChange: connectionSchema },
    onSubmit: async ({ value }) => {
      if (
        active.current ||
        !keys.isSuccess ||
        !connections.isSuccess ||
        !validJump(value.jump_connection_id) ||
        !keys.data.some((key) => key.id === value.ssh_key_id)
      )
        return
      active.current = true
      try {
        await save.mutateAsync({
          id: connection?.id,
          fields: {
            ...value,
            jump_connection_id: value.jump_connection_id || null,
            name: value.name.trim(),
            host: value.host.trim(),
            username: value.username.trim(),
            port: Number(value.port),
          },
        })
      } catch {
        /* The mutation retains errors and form values for a manual retry. */
      } finally {
        active.current = false
      }
    },
  })
  const busy = save.isPending
  return (
    <dialog
      ref={dialog}
      className="modal"
      aria-labelledby="connection-title"
      onClose={onClose}
      onCancel={(event) => {
        if (active.current || busy) event.preventDefault()
      }}
    >
      <div className="modal-box max-w-2xl">
        <div className="flex items-center justify-between gap-4">
          <h2 id="connection-title" className="text-2xl font-bold">
            {connection ? 'Edit connection' : 'Add connection'}
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
        <p className="mt-3 text-sm text-base-content/75">
          Save a destination and the SSH key used to sign in. Changes apply to
          future connections.
        </p>
        <form
          className="mt-6 space-y-4"
          onSubmit={(event) => {
            event.preventDefault()
            if (!busy) void form.handleSubmit()
          }}
        >
          {(
            [
              { name: 'name', label: 'Connection name' },
              { name: 'host', label: 'Hostname or IP address' },
              { name: 'port', label: 'Port' },
              { name: 'username', label: 'SSH username' },
            ] as const
          ).map(({ name, label }) => (
            <form.Field key={name} name={name}>
              {(field) => (
                <div>
                  <label
                    htmlFor={`connection-${name}`}
                    className="mb-2 block font-medium"
                  >
                    {label}
                  </label>
                  <Input
                    id={`connection-${name}`}
                    className="w-full"
                    inputMode={name === 'port' ? 'numeric' : undefined}
                    value={field.state.value}
                    disabled={busy}
                    onBlur={field.handleBlur}
                    onChange={(e) => field.handleChange(e.target.value)}
                    aria-invalid={field.state.meta.errors.length > 0}
                    aria-describedby={`connection-${name}-error`}
                  />
                  <p
                    id={`connection-${name}-error`}
                    role="alert"
                    className="mt-1 text-sm text-error"
                  >
                    {[
                      ...new Set(
                        field.state.meta.errors.map((e) => e?.message),
                      ),
                    ].join(' ')}
                  </p>
                </div>
              )}
            </form.Field>
          ))}
          <form.Field name="ssh_key_id">
            {(field) => (
              <div>
                <label
                  htmlFor="connection-key"
                  className="mb-2 block font-medium"
                >
                  SSH key
                </label>
                <select
                  id="connection-key"
                  className="select w-full"
                  value={field.state.value}
                  disabled={busy || !keys.isSuccess}
                  onBlur={field.handleBlur}
                  onChange={(e) => field.handleChange(e.target.value)}
                  aria-invalid={field.state.meta.errors.length > 0}
                  aria-describedby="connection-key-error"
                >
                  <option value="">Choose a key</option>
                  {keys.data?.map((key) => (
                    <option key={key.id} value={key.id}>
                      {key.name} — {key.public_fingerprint}
                    </option>
                  ))}
                </select>
                <p
                  id="connection-key-error"
                  role="alert"
                  className="mt-1 text-sm text-error"
                >
                  {[
                    ...new Set(field.state.meta.errors.map((e) => e?.message)),
                  ].join(' ')}
                  {keys.isSuccess &&
                  field.state.value &&
                  !keys.data.some((key) => key.id === field.state.value)
                    ? ' This key is no longer available. Choose another key.'
                    : ''}
                </p>
              </div>
            )}
          </form.Field>
          <form.Field name="jump_connection_id">
            {(field) => (
              <div>
                <label
                  htmlFor="connection-jump"
                  className="mb-2 block font-medium"
                >
                  Jump through
                </label>
                <select
                  id="connection-jump"
                  className="select w-full"
                  value={field.state.value}
                  disabled={busy || !connections.isSuccess}
                  onBlur={field.handleBlur}
                  onChange={(event) => field.handleChange(event.target.value)}
                  aria-invalid={
                    connections.isSuccess && !validJump(field.state.value)
                  }
                  aria-describedby="connection-jump-help connection-jump-error"
                >
                  <option value="">None — connect directly</option>
                  {field.state.value &&
                    !jumps.some((item) => item.id === field.state.value) && (
                      <option value={field.state.value} disabled>
                        Unavailable jump connection
                      </option>
                    )}
                  {jumps.map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.name} — {item.username}@{item.host}:{item.port}
                    </option>
                  ))}
                </select>
                <p
                  id="connection-jump-help"
                  className="mt-1 text-sm text-base-content/75"
                >
                  {isReferenced
                    ? 'This connection is used as a jump by another saved connection and must remain direct.'
                    : 'Choose a saved direct connection to reach this host through one bastion, or connect directly.'}
                </p>
                <p
                  id="connection-jump-error"
                  role="alert"
                  className="mt-1 text-sm text-error"
                >
                  {connections.isSuccess && !validJump(field.state.value)
                    ? 'This jump is no longer available. Choose another connection or connect directly.'
                    : ''}
                </p>
              </div>
            )}
          </form.Field>
          {connections.isPending && (
            <p role="status">Loading jump connections…</p>
          )}
          {connections.isError && (
            <div>
              <p role="alert" className="text-error">
                {connections.error.message}
              </p>
              <Button
                type="button"
                disabled={connections.isFetching || busy}
                onClick={() => void connections.refetch()}
              >
                Retry jump connections
              </Button>
            </div>
          )}
          {keys.isPending && <p role="status">Loading keys…</p>}
          {keys.isError && (
            <div>
              <p role="alert" className="text-error">
                {keys.error.message}
              </p>
              <Button
                type="button"
                disabled={keys.isFetching || busy}
                onClick={() => void keys.refetch()}
              >
                Retry key list
              </Button>
            </div>
          )}
          {keys.isSuccess && keys.data.length === 0 && (
            <div>
              <p>Upload an SSH key before saving a connection.</p>
              <Button type="button" onClick={onManageKeys}>
                Manage SSH keys
              </Button>
            </div>
          )}
          {save.error && (
            <p role="alert" className="text-error">
              {save.error.message}
            </p>
          )}
          <Button
            type="submit"
            className="btn-primary"
            disabled={
              busy ||
              !keys.isSuccess ||
              keys.data.length === 0 ||
              !connections.isSuccess
            }
          >
            {busy ? 'Saving…' : 'Save connection'}
          </Button>
        </form>
      </div>
    </dialog>
  )
}
