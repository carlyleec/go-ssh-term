import { useForm } from '@tanstack/react-form'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { ApiError } from '../../../../api/apiClient'
import queries, { type SSHKey } from '../../../../api/queries'
import { Button } from '../../../../components/button'
import { Input } from '../../../../components/input'

export function KeyModal({
  accountID,
  onClose,
}: {
  accountID: string
  onClose: () => void
}) {
  const client = useQueryClient()
  const dialog = useRef<HTMLDialogElement>(null)
  const fileInput = useRef<HTMLInputElement>(null)
  const active = useRef(false)
  const mounted = useRef(false)
  const [confirmation, setConfirmation] = useState<SSHKey | null>(null)
  const [notice, setNotice] = useState('')
  const queryKey = queries.keys.options(accountID).queryKey
  const keys = queries.keys.useQuery(accountID)

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
    if (keys.error instanceof ApiError && keys.error.status === 401)
      client.setQueryData(queries.auth.currentUser.queryKey, null)
  }, [keys.error, client])

  const form = useForm({
    defaultValues: { name: '', file: null as File | null },
    onSubmit: async () => {
      if (active.current) return
      active.current = true
      setNotice('')
      try {
        await upload.mutateAsync(form.state.values)
      } catch {
        // The mutation owns the visible error and leaves the form available for retry.
      } finally {
        active.current = false
      }
    },
  })
  const upload = useMutation({
    ...queries.keys.upload,
    onSuccess: async () => {
      // A session check can unmount the modal while a mutation is in flight.
      if (!mounted.current) return
      form.reset()
      if (fileInput.current) fileInput.current.value = ''
      setNotice('Key uploaded.')
      await client.invalidateQueries({ queryKey })
    },
    onError: expire,
  })
  const remove = useMutation({
    ...queries.keys.remove,
    onSuccess: async () => {
      if (!mounted.current) return
      setConfirmation(null)
      setNotice('Key deleted.')
      await client.invalidateQueries({ queryKey })
    },
    onError: expire,
  })
  useEffect(() => () => form.reset(), [form])

  const busy = upload.isPending || remove.isPending

  return (
    <dialog
      ref={dialog}
      className="modal"
      aria-labelledby="keys-title"
      onCancel={(event) => {
        if (active.current || busy) event.preventDefault()
      }}
      onClose={onClose}
    >
      <div className="modal-box max-w-2xl">
        <div className="flex items-center justify-between gap-4">
          <h2 id="keys-title" className="text-2xl font-bold">
            SSH keys
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
        <p id="key-help" className="mt-4 text-sm text-base-content/75">
          Upload one unencrypted Ed25519 private key in OpenSSH format, at most
          16 KiB. Passphrase-protected keys are not supported. Stored private
          keys cannot be downloaded.
        </p>
        <form
          className="mt-6 space-y-4"
          onSubmit={(event) => {
            event.preventDefault()
            if (!busy) void form.handleSubmit()
          }}
        >
          <form.Field
            name="name"
            validators={{
              onBlur: ({ value }) => validateKeyName(value),
              onSubmit: ({ value }) => validateKeyName(value),
            }}
          >
            {(field) => (
              <div>
                <label htmlFor="key-name" className="mb-2 block font-medium">
                  Key name
                </label>
                <Input
                  id="key-name"
                  className="w-full"
                  value={field.state.value}
                  disabled={busy}
                  onBlur={field.handleBlur}
                  onChange={(event) => field.handleChange(event.target.value)}
                  aria-invalid={field.state.meta.errors.length > 0}
                  aria-describedby="key-name-error"
                />
                <p
                  id="key-name-error"
                  role="alert"
                  className="mt-2 text-sm text-error"
                >
                  {[...new Set(field.state.meta.errors)].join(' ')}
                </p>
              </div>
            )}
          </form.Field>
          <form.Field
            name="file"
            validators={{
              onChange: ({ value }) => validateKeyFile(value),
              onSubmit: ({ value }) => validateKeyFile(value),
            }}
          >
            {(field) => (
              <div>
                <label htmlFor="key-file" className="mb-2 block font-medium">
                  Private-key file
                </label>
                <Input
                  ref={fileInput}
                  id="key-file"
                  type="file"
                  className="w-full"
                  disabled={busy}
                  onChange={(event) =>
                    field.handleChange(event.target.files?.[0] ?? null)
                  }
                  aria-invalid={field.state.meta.errors.length > 0}
                  aria-describedby="key-help key-file-error"
                />
                <p
                  id="key-file-error"
                  role="alert"
                  className="mt-2 text-sm text-error"
                >
                  {[...new Set(field.state.meta.errors)].join(' ')}
                </p>
              </div>
            )}
          </form.Field>
          {upload.error && (
            <p role="alert" className="text-error">
              {upload.error.message}
            </p>
          )}
          <Button type="submit" className="btn-primary" disabled={busy}>
            {upload.isPending ? 'Uploading…' : 'Upload key'}
          </Button>
        </form>
        <p role="status" className="mt-4">
          {notice}
        </p>
        <h3 className="mt-6 text-lg font-semibold">Saved keys</h3>
        {keys.isPending && (
          <p role="status" className="mt-3">
            Loading keys…
          </p>
        )}
        {keys.isError && (
          <div className="mt-3">
            <p role="alert" className="text-error">
              {keys.error.message}
            </p>
            <Button
              type="button"
              className="btn-outline mt-2"
              disabled={keys.isFetching || busy}
              onClick={() => void keys.refetch()}
            >
              Retry key list
            </Button>
          </div>
        )}
        {keys.data?.length === 0 && (
          <p className="mt-3">No SSH keys yet. Upload a key to get started.</p>
        )}
        <ul className="mt-3 space-y-4">
          {keys.data?.map((key) => (
            <li key={key.id} className="rounded-box border border-base-300 p-4">
              <p className="font-semibold break-words">{key.name}</p>
              <p className="mt-2 break-all font-mono text-sm">
                {key.public_fingerprint}
              </p>
              <p className="mt-2 text-sm text-base-content/65">
                Added {new Date(key.created_at).toLocaleString()}
              </p>
              {confirmation?.id === key.id ? (
                <div className="mt-3">
                  <p>
                    Delete “{key.name}”? You will need the original file to
                    upload it again.
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
                        setNotice('')
                        remove.mutate(key.id, {
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
              ) : (
                <Button
                  type="button"
                  className="btn-outline btn-sm mt-3"
                  disabled={busy}
                  aria-label={`Delete ${key.name}`}
                  onClick={() => {
                    remove.reset()
                    setConfirmation(key)
                  }}
                >
                  Delete
                </Button>
              )}
            </li>
          ))}
        </ul>
      </div>
    </dialog>
  )
}

function validateKeyName(value: string) {
  const name = value.trim()
  if (!name) return 'Enter a name for this key.'
  if ([...name].length > 64) return 'Use at most 64 characters.'
  if (/\p{Cc}/u.test(name)) return 'The name cannot contain control characters.'
  if (new TextEncoder().encode(value).length > 256)
    return 'The name is too long. Remove extra whitespace.'
  return undefined
}

function validateKeyFile(file: File | null) {
  if (!file || file.size === 0) return 'Choose a private-key file.'
  if (file.size > 16_384) return 'The private-key file must be at most 16 KiB.'
  return undefined
}
