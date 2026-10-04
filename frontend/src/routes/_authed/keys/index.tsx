import { useMutation, useQueryClient } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { ApiError } from '~/api/apiClient'
import queries, { type SSHKey } from '~/api/queries'
import { Button } from '~/components/button'
import { PageHeader } from '~/components/page-header'
import { useAuth } from '~/hooks/use-auth'
import { KeyDrawer } from './-components/key-drawer'

export const Route = createFileRoute('/_authed/keys/')({ component: KeysPage })

function KeysPage() {
  const { account, isSigningOut } = useAuth()
  const accountID = account?.id ?? ''
  const client = useQueryClient()
  const keys = queries.keys.useQuery(accountID)
  const [uploadOpen, setUploadOpen] = useState(false)
  const [confirmation, setConfirmation] = useState<SSHKey | null>(null)
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
    if (keys.error instanceof ApiError && keys.error.status === 401)
      client.setQueryData(queries.auth.currentUser.queryKey, null)
  }, [keys.error, client])
  const remove = useMutation({
    ...queries.keys.remove,
    onSuccess: async () => {
      if (!mounted.current) return
      setConfirmation(null)
      setNotice('Key deleted.')
      await client.invalidateQueries({
        queryKey: queries.keys.options(accountID).queryKey,
      })
    },
    onError: (error) => {
      if (mounted.current && error instanceof ApiError && error.status === 401)
        client.setQueryData(queries.auth.currentUser.queryKey, null)
    },
  })
  const busy = remove.isPending || isSigningOut
  return (
    <section className="mx-auto max-w-6xl px-6 py-12">
      <PageHeader
        title="SSH Keys"
        description="Manage the credentials used by your saved connections."
      >
        <Button
          type="button"
          className="btn-outline"
          disabled={busy}
          onClick={() => setUploadOpen(true)}
        >
          Upload SSH key
        </Button>
      </PageHeader>
      <p role="status" className="mt-4">
        {notice}
      </p>
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
      <ul className="mt-6 space-y-4">
        {keys.data?.map((key) => (
          <li key={key.id} className="rounded-box border border-base-300 p-5">
            <div className="grid grid-cols-[minmax(0,1fr)_auto] items-start gap-4">
              <div className="min-w-0">
                <h2 className="font-semibold break-words">{key.name}</h2>
                <p className="mt-2 break-all font-mono text-sm">
                  {key.public_fingerprint}
                </p>
                <p className="mt-2 text-sm text-base-content/65">
                  Added {new Date(key.created_at).toLocaleString()}
                </p>
              </div>
              {confirmation?.id !== key.id && (
                <Button
                  type="button"
                  className="btn-outline btn-sm"
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
            </div>
            {confirmation?.id === key.id ? (
              <div className="mt-3">
                <p>
                  Delete “{key.name}”? You will need the original file to upload
                  it again.
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
            ) : null}
          </li>
        ))}
      </ul>

      {uploadOpen && (
        <KeyDrawer
          accountID={accountID}
          onClose={() => setUploadOpen(false)}
          onUploaded={() => {
            setUploadOpen(false)
            setNotice('Key uploaded.')
          }}
        />
      )}
    </section>
  )
}
