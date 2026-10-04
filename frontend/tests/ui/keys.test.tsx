import { afterEach, beforeEach, expect, test } from 'bun:test'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import { StrictMode } from 'react'
import type { SSHKey } from '~/api/queries'
import { routeTree } from '~/routetree.gen'
import { mockFetch } from '../mock-fetch'

const originalFetch = globalThis.fetch
let client: QueryClient
let records: SSHKey[]
let signedIn: boolean
let listWait: Promise<void> | undefined
let deleteWait: Promise<void> | undefined
let uploads: number
let deletes: number
let listStatus: number
let uploadStatus: number
let deleteStatus: number
let uploadWait: Promise<void> | undefined
const metadata = {
  id: 'key-id',
  name: 'Lab',
  public_fingerprint: 'SHA256:public-fingerprint',
  created_at: '2026-10-03T12:00:00Z',
}

beforeEach(() => {
  client = new QueryClient()
  records = []
  signedIn = true
  listWait = undefined
  deleteWait = undefined
  uploads = 0
  deletes = 0
  listStatus = 200
  uploadStatus = 201
  deleteStatus = 204
  uploadWait = undefined
  globalThis.fetch = mockFetch(async (input, init) => {
    const url = String(input)
    if (url === '/api/connections') return Response.json({ connections: [] })
    if (url === '/api/auth/me' && !signedIn)
      return Response.json({ error: 'sign in' }, { status: 401 })
    if (url === '/api/auth/me')
      return Response.json({
        account: { id: 'account-id', display_name: 'Cam' },
      })
    if (url === '/api/keys' && init?.method === 'POST') {
      uploads++
      if (uploadStatus === 401) signedIn = false
      await uploadWait
      expect(new Headers(init.headers).has('Content-Type')).toBe(false)
      const body = init.body as FormData
      expect(body.get('name')).toBe('Lab')
      expect(body.get('private_key')).toBeInstanceOf(File)
      if (uploadStatus !== 201)
        return Response.json(
          { error: 'Passphrase-protected keys are not supported.' },
          { status: uploadStatus },
        )
      records = [metadata]
      return Response.json({ key: metadata }, { status: 201 })
    }
    if (url === '/api/keys' && !init?.method) {
      await listWait
      if (listStatus === 401) signedIn = false
      return listStatus === 200
        ? Response.json({ keys: records })
        : Response.json(
            { error: 'Could not list keys.' },
            { status: listStatus },
          )
    }
    if (url === '/api/keys/key-id' && init?.method === 'DELETE') {
      deletes++
      await deleteWait
      if (deleteStatus === 401) signedIn = false
      if (deleteStatus !== 204)
        return Response.json(
          { error: 'Could not delete key.' },
          { status: deleteStatus },
        )
      records = []
      return new Response(null, { status: 204 })
    }
    throw new Error(`Unexpected request: ${url}`)
  })
})

afterEach(async () => {
  cleanup()
  await client.cancelQueries()
  client.clear()
  globalThis.fetch = originalFetch
})

async function openKeys(upload = true) {
  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: ['/keys'] }),
    context: { queryClient: client },
  })
  await act(async () => {
    render(
      <StrictMode>
        <QueryClientProvider client={client}>
          <RouterProvider router={router} />
        </QueryClientProvider>
      </StrictMode>,
    )
    await router.load()
  })
  if (upload && listStatus !== 401) {
    fireEvent.click(
      await screen.findByRole('button', { name: 'Upload SSH key' }),
    )
    await screen.findByRole('dialog', { name: 'Upload SSH key' })
  }
}

function fill(file = new File(['private material'], 'lab')) {
  fireEvent.change(screen.getByLabelText('Key name'), {
    target: { value: 'Lab' },
  })
  fireEvent.change(screen.getByLabelText('Private-key file'), {
    target: { files: [file] },
  })
}

async function submit() {
  fireEvent.click(screen.getByRole('button', { name: 'Upload key' }))
}

test('uploads multipart data, clears the file and name, lists metadata, and confirms deletion', async () => {
  await openKeys()
  await screen.findByText(/No SSH keys yet/)
  fill()
  await submit()
  await screen.findByText(metadata.public_fingerprint)
  expect(uploads).toBe(1)
  expect(screen.queryByRole('dialog')).toBeNull()
  expect(document.body.textContent).not.toContain('private material')
  fireEvent.click(screen.getByRole('button', { name: 'Delete Lab' }))
  expect(deletes).toBe(0)
  fireEvent.click(screen.getByRole('button', { name: 'Cancel deletion' }))
  expect(deletes).toBe(0)
  fireEvent.click(screen.getByRole('button', { name: 'Delete Lab' }))
  fireEvent.click(screen.getByRole('button', { name: 'Confirm deletion' }))
  await screen.findByText(/No SSH keys yet/)
  expect(deletes).toBe(1)
})

test('validates required fields and oversize files before upload', async () => {
  await openKeys()
  await submit()
  await screen.findByText('Enter a name for this key.')
  await screen.findByText('Choose a private-key file.')
  fill(new File(['x'.repeat(16_385)], 'large'))
  await submit()
  await screen.findByText('The private-key file must be at most 16 KiB.')
  expect(uploads).toBe(0)
})

test('preserves server validation errors and permits an explicit retry', async () => {
  uploadStatus = 400
  await openKeys()
  fill()
  await submit()
  await screen.findByText('Passphrase-protected keys are not supported.')
  expect(uploads).toBe(1)
  uploadStatus = 201
  await submit()
  await screen.findByText(metadata.public_fingerprint)
  expect(uploads).toBe(2)
})

test('pending uploads disable competing actions and duplicate submission', async () => {
  let finish!: () => void
  uploadWait = new Promise<void>((resolve) => {
    finish = resolve
  })
  await openKeys()
  fill()
  await submit()
  const pending = (await screen.findByRole('button', {
    name: 'Uploading…',
  })) as HTMLButtonElement
  expect(pending.disabled).toBe(true)
  expect(
    (screen.getByRole('button', { name: 'Close' }) as HTMLButtonElement)
      .disabled,
  ).toBe(true)
  fireEvent.submit(
    screen.getByLabelText('Key name').closest('form') as HTMLFormElement,
  )
  expect(uploads).toBe(1)
  const cancel = new Event('cancel', { cancelable: true })
  screen.getByRole('dialog').dispatchEvent(cancel)
  expect(cancel.defaultPrevented).toBe(true)
  await act(async () => finish())
  await screen.findByText(metadata.public_fingerprint)
})

test('list and deletion failures remain retryable', async () => {
  listStatus = 503
  records = [metadata]
  await openKeys(false)
  await screen.findByText('Could not list keys.')
  listStatus = 200
  fireEvent.click(screen.getByRole('button', { name: 'Retry key list' }))
  await screen.findByText(metadata.public_fingerprint)
  deleteStatus = 503
  fireEvent.click(screen.getByRole('button', { name: 'Delete Lab' }))
  fireEvent.click(screen.getByRole('button', { name: 'Confirm deletion' }))
  await screen.findByText('Could not delete key.')
  expect(deletes).toBe(1)
  deleteStatus = 204
  fireEvent.click(screen.getByRole('button', { name: 'Confirm deletion' }))
  await screen.findByText(/No SSH keys yet/)
  expect(deletes).toBe(2)
})

test.each(['list', 'upload', 'delete'])(
  'a 401 from %s clears key data and returns to login',
  async (operation) => {
    records = [metadata]
    if (operation === 'list') listStatus = 401
    await openKeys(operation === 'upload')
    if (operation === 'upload') {
      uploadStatus = 401
      fill()
      await submit()
    }
    if (operation === 'delete') {
      deleteStatus = 401
      fireEvent.click(await screen.findByRole('button', { name: 'Delete Lab' }))
      fireEvent.click(screen.getByRole('button', { name: 'Confirm deletion' }))
    }
    await screen.findByRole('heading', { name: 'Account access' })
    expect(client.getQueryData(['ssh-keys', 'account-id'])).toBeUndefined()
  },
)

test('closing and reopening discards the upload form', async () => {
  await openKeys()
  fill()
  fireEvent.click(screen.getByRole('button', { name: 'Close' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  fireEvent.click(screen.getByRole('button', { name: 'Upload SSH key' }))
  await screen.findByRole('dialog')
  expect((screen.getByLabelText('Key name') as HTMLInputElement).value).toBe('')
})

test('shows list loading and prevents duplicate deletion while pending', async () => {
  let finishList!: () => void
  let finishDelete!: () => void
  listWait = new Promise<void>((resolve) => {
    finishList = resolve
  })
  deleteWait = new Promise<void>((resolve) => {
    finishDelete = resolve
  })
  records = [metadata]
  await openKeys(false)
  await screen.findByText('Loading keys…')
  await act(async () => finishList())
  fireEvent.click(await screen.findByRole('button', { name: 'Delete Lab' }))
  fireEvent.click(screen.getByRole('button', { name: 'Confirm deletion' }))
  const button = (await screen.findByRole('button', {
    name: 'Deleting…',
  })) as HTMLButtonElement
  expect(button.disabled).toBe(true)
  expect(
    (
      screen.getByRole('button', {
        name: 'Upload SSH key',
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true)
  fireEvent.click(button)
  expect(deletes).toBe(1)
  await act(async () => finishDelete())
  await screen.findByText(/No SSH keys yet/)
})
