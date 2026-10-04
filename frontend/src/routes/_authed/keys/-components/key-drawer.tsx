import { useForm } from '@tanstack/react-form'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'
import { z } from 'zod'
import { ApiError } from '~/api/apiClient'
import queries from '~/api/queries'
import { Button } from '~/components/button'
import { Input } from '~/components/input'

export function KeyDrawer({
  accountID,
  onClose,
  onUploaded,
}: {
  accountID: string
  onClose: () => void
  onUploaded: () => void
}) {
  const client = useQueryClient()
  const dialog = useRef<HTMLDialogElement>(null)
  const fileInput = useRef<HTMLInputElement>(null)
  const active = useRef(false)
  const mounted = useRef(false)
  const queryKey = queries.keys.options(accountID).queryKey

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

  const form = useForm({
    defaultValues: { name: '', file: null as File | null },
    validators: { onChange: keyUploadSchema },
    onSubmit: async () => {
      if (active.current) return
      active.current = true
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
      // A session check can unmount the drawer while a mutation is in flight.
      if (!mounted.current) return
      form.reset()
      if (fileInput.current) fileInput.current.value = ''
      await client.invalidateQueries({ queryKey })
      if (mounted.current) onUploaded()
    },
    onError: expire,
  })
  useEffect(() => () => form.reset(), [form])

  const busy = upload.isPending

  return (
    <dialog
      ref={dialog}
      className="side-drawer"
      aria-labelledby="keys-title"
      onCancel={(event) => {
        if (active.current || busy) event.preventDefault()
      }}
      onClose={onClose}
    >
      <div className="drawer-panel">
        <div className="drawer-heading">
          <h2 id="keys-title" className="text-2xl font-bold">
            Upload SSH key
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
          className="mt-6 flex flex-1 flex-col gap-4"
          onSubmit={(event) => {
            event.preventDefault()
            if (!busy) void form.handleSubmit()
          }}
        >
          <form.Field name="name">
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
                  {[
                    ...new Set(
                      field.state.meta.errors.map((error) => error?.message),
                    ),
                  ].join(' ')}
                </p>
              </div>
            )}
          </form.Field>
          <form.Field name="file">
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
                  {[
                    ...new Set(
                      field.state.meta.errors.map((error) => error?.message),
                    ),
                  ].join(' ')}
                </p>
              </div>
            )}
          </form.Field>
          {upload.error && (
            <p role="alert" className="text-error">
              {upload.error.message}
            </p>
          )}
          <div className="drawer-actions">
            <Button type="submit" className="btn-primary" disabled={busy}>
              {upload.isPending ? 'Uploading…' : 'Upload key'}
            </Button>
          </div>
        </form>
      </div>
    </dialog>
  )
}

export const keyUploadSchema = z.object({
  name: z
    .string()
    .refine((value) => value.trim().length > 0, {
      error: 'Enter a name for this key.',
      abort: true,
    })
    .refine((value) => [...value.trim()].length <= 64, {
      error: 'Use at most 64 characters.',
      abort: true,
    })
    .refine((value) => !/\p{Cc}/u.test(value.trim()), {
      error: 'The name cannot contain control characters.',
      abort: true,
    })
    .refine((value) => new TextEncoder().encode(value).length <= 256, {
      error: 'The name is too long. Remove extra whitespace.',
    }),
  file: z
    .file()
    .nullable()
    .refine((file) => file !== null && file.size > 0, {
      error: 'Choose a private-key file.',
      abort: true,
    })
    .refine((file) => file === null || file.size <= 16_384, {
      error: 'The private-key file must be at most 16 KiB.',
    }),
})
