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
  within,
} from '@testing-library/react'
import { StrictMode } from 'react'
import type { SavedConnection } from '~/api/queries'
import { routeTree } from '~/routetree.gen'
import { mockFetch } from '../mock-fetch'

const originalFetch = globalThis.fetch
const keyID = '11111111-1111-4111-8111-111111111111'
const record: SavedConnection = {
  id: '22222222-2222-4222-8222-222222222222',
  name: 'Local bastion',
  host: 'bastion',
  port: 22,
  username: 'demo',
  ssh_key_id: keyID,
  created_at: '2026-10-04T12:00:00Z',
  updated_at: '2026-10-04T12:00:00Z',
}
let client: QueryClient
let records: SavedConnection[]
let keys: boolean
let signedIn: boolean
let saveStatus: number
let listStatus: number
let deleteStatus: number
let saved: number
let deleted: number
let pending: Promise<void> | undefined
let hostState: string
let approvals: number
let resets: number
let hostStatus: number
beforeEach(() => {
  client = new QueryClient()
  records = []
  keys = true
  signedIn = true
  saveStatus = 200
  listStatus = 200
  deleteStatus = 204
  saved = 0
  deleted = 0
  pending = undefined
  hostState = 'unknown'
  approvals = 0
  resets = 0
  hostStatus = 200
  globalThis.fetch = mockFetch(async (input, init) => {
    const url = String(input)
    if (url === '/api/auth/me')
      return signedIn
        ? Response.json({ account: { id: 'account-id', display_name: 'Cam' } })
        : Response.json({ error: 'sign in' }, { status: 401 })
    if (url === '/api/keys')
      return Response.json({
        keys: keys
          ? [
              {
                id: keyID,
                name: 'Demo key',
                public_fingerprint: 'SHA256:demo',
                created_at: record.created_at,
              },
            ]
          : [],
      })
    if (url === '/api/connections' && !init?.method) {
      if (listStatus === 401) signedIn = false
      return listStatus === 200
        ? Response.json({ connections: records })
        : Response.json(
            { error: 'Could not list connections.' },
            { status: listStatus },
          )
    }
    if (url.includes('/host-')) {
      if (hostStatus === 401) signedIn = false
      if (hostStatus !== 200)
        return Response.json(
          { error: 'Host inspection failed.' },
          { status: hostStatus },
        )
      if (url.endsWith('/reset')) {
        resets++
        hostState = 'unknown'
        return new Response(null, { status: 204 })
      }
      if (url.endsWith('/host-trust')) {
        approvals++
        hostState = 'trusted'
      }
      return Response.json({
        host: 'bastion',
        port: 22,
        state: hostState,
        algorithm: 'ssh-ed25519',
        fingerprint: 'SHA256:presented',
        trusted_fingerprint: hostState === 'unknown' ? '' : 'SHA256:previous',
      })
    }
    if (init?.method === 'POST' || init?.method === 'PUT') {
      saved++
      await pending
      if (saveStatus === 401) signedIn = false
      if (saveStatus !== 200)
        return Response.json(
          { error: 'Could not save connection.' },
          { status: saveStatus },
        )
      expect(new Headers(init.headers).get('Content-Type')).toBe(
        'application/json',
      )
      const fields = JSON.parse(String(init.body))
      expect(fields.ssh_key_id).toBe(keyID)
      expect(typeof fields.port).toBe('number')
      records = [{ ...record, ...fields }]
      return Response.json(
        { connection: records[0] },
        { status: init.method === 'POST' ? 201 : 200 },
      )
    }
    if (init?.method === 'DELETE') {
      deleted++
      await pending
      if (deleteStatus === 401) signedIn = false
      if (deleteStatus !== 204)
        return Response.json(
          { error: 'Could not delete connection.' },
          { status: deleteStatus },
        )
      records = []
      return new Response(null, { status: 204 })
    }
    throw new Error(`Unexpected request ${url}`)
  })
})
afterEach(async () => {
  cleanup()
  await client.cancelQueries()
  client.clear()
  globalThis.fetch = originalFetch
})
async function open() {
  const router = createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: ['/connections'] }),
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
  await screen.findByRole('heading', { name: 'Connections' })
}
async function add() {
  fireEvent.click(screen.getByRole('button', { name: 'Add connection' }))
  await screen.findByRole('dialog', { name: 'Add connection' })
  await screen.findByRole('option', { name: /Demo key/ })
}
function fill() {
  for (const [label, value] of [
    ['Connection name', 'Local bastion'],
    ['Hostname or IP address', 'bastion'],
    ['Port', '22'],
    ['SSH username', 'demo'],
    ['SSH key', keyID],
  ])
    fireEvent.change(screen.getByLabelText(label), { target: { value } })
}
function save() {
  fireEvent.click(screen.getByRole('button', { name: 'Save connection' }))
}

test('empty demo instructions, create, edit, picker, and confirmed deletion', async () => {
  await open()
  await screen.findByRole('heading', { name: 'Start with the local demo' })
  expect(screen.getByText('demo/keys/demo_ed25519')).toBeTruthy()
  await add()
  fill()
  save()
  await screen.findByRole('button', { name: 'Edit Local bastion' })
  expect(saved).toBe(1)
  fireEvent.click(screen.getByRole('button', { name: 'Edit Local bastion' }))
  await screen.findByRole('dialog', { name: 'Edit connection' })
  expect((screen.getByLabelText('SSH key') as HTMLSelectElement).value).toBe(
    keyID,
  )
  fireEvent.change(screen.getByLabelText('Connection name'), {
    target: { value: 'Renamed' },
  })
  save()
  await screen.findByRole('button', { name: 'Edit Renamed' })
  expect(saved).toBe(2)
  fireEvent.click(screen.getByRole('button', { name: 'Connect' }))
  const picker = await screen.findByRole('dialog', {
    name: 'Choose a connection',
  })
  fireEvent.click(within(picker).getByRole('button', { name: /Renamed/ }))
  await screen.findByRole('dialog', { name: 'Verify SSH host' })
  fireEvent.click(
    await screen.findByRole('button', { name: 'Reject and close' }),
  )
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  fireEvent.click(screen.getByRole('button', { name: 'Delete Renamed' }))
  expect(deleted).toBe(0)
  fireEvent.click(screen.getByRole('button', { name: 'Cancel deletion' }))
  expect(deleted).toBe(0)
  fireEvent.click(screen.getByRole('button', { name: 'Delete Renamed' }))
  fireEvent.click(screen.getByRole('button', { name: 'Confirm deletion' }))
  await screen.findByRole('heading', { name: 'Start with the local demo' })
  expect(deleted).toBe(1)
})

test('validation blocks submission and closing discards form values', async () => {
  await open()
  await add()
  save()
  await screen.findByText('Choose an SSH key.')
  expect(saved).toBe(0)
  fill()
  fireEvent.change(screen.getByLabelText('Port'), {
    target: { value: '65536' },
  })
  save()
  await screen.findByText('Enter a whole-number port from 1 to 65535.')
  expect(saved).toBe(0)
  fireEvent.click(screen.getByRole('button', { name: 'Close' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  await add()
  expect(
    (screen.getByLabelText('Connection name') as HTMLInputElement).value,
  ).toBe('')
})

test('no keys directs to key management and disables saving', async () => {
  keys = false
  await open()
  fireEvent.click(screen.getByRole('button', { name: 'Add connection' }))
  await screen.findByText('Upload an SSH key before saving a connection.')
  expect(
    (
      screen.getByRole('button', {
        name: 'Save connection',
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true)
  fireEvent.click(
    within(screen.getByRole('dialog')).getByRole('button', {
      name: 'Manage SSH keys',
    }),
  )
  await screen.findByRole('dialog', { name: 'SSH keys' })
})

test('list and save failures allow manual retry without losing form values', async () => {
  listStatus = 503
  await open()
  await screen.findByText('Could not list connections.')
  listStatus = 200
  fireEvent.click(screen.getByRole('button', { name: 'Retry connection list' }))
  await screen.findByRole('heading', { name: 'Start with the local demo' })
  saveStatus = 400
  await add()
  fill()
  save()
  await screen.findByText('Could not save connection.')
  expect(
    (screen.getByLabelText('Hostname or IP address') as HTMLInputElement).value,
  ).toBe('bastion')
  saveStatus = 200
  save()
  await screen.findByRole('button', { name: 'Edit Local bastion' })
  expect(saved).toBe(2)
})

test('pending saves prevent duplicate submissions and dismissal', async () => {
  let finish!: () => void
  pending = new Promise((resolve) => {
    finish = resolve
  })
  await open()
  await add()
  fill()
  save()
  await screen.findByRole('button', { name: 'Saving…' })
  expect(
    (screen.getByRole('button', { name: 'Close' }) as HTMLButtonElement)
      .disabled,
  ).toBe(true)
  fireEvent.submit(
    screen.getByLabelText('Connection name').closest('form') as HTMLFormElement,
  )
  const cancel = new Event('cancel', { cancelable: true })
  screen.getByRole('dialog').dispatchEvent(cancel)
  expect(cancel.defaultPrevented).toBe(true)
  expect(saved).toBe(1)
  await act(async () => finish())
  await screen.findByRole('button', { name: 'Edit Local bastion' })
})

test.each(['list', 'save', 'delete'])(
  '401 from %s clears connection cache and returns to login',
  async (operation) => {
    records = [record]
    if (operation === 'list') listStatus = 401
    await open()
    if (operation === 'save') {
      saveStatus = 401
      fireEvent.click(
        await screen.findByRole('button', { name: 'Edit Local bastion' }),
      )
      await screen.findByRole('dialog')
      save()
    }
    if (operation === 'delete') {
      deleteStatus = 401
      fireEvent.click(
        await screen.findByRole('button', { name: 'Delete Local bastion' }),
      )
      fireEvent.click(screen.getByRole('button', { name: 'Confirm deletion' }))
    }
    await screen.findByRole('heading', { name: 'Account access' })
    expect(client.getQueryData(['connections', 'account-id'])).toBeUndefined()
  },
)

test('deletion failures retry and pending deletion blocks competing actions', async () => {
  records = [record]
  deleteStatus = 503
  await open()
  fireEvent.click(
    await screen.findByRole('button', { name: 'Delete Local bastion' }),
  )
  fireEvent.click(screen.getByRole('button', { name: 'Confirm deletion' }))
  await screen.findByText('Could not delete connection.')
  let finish!: () => void
  pending = new Promise((resolve) => {
    finish = resolve
  })
  deleteStatus = 204
  fireEvent.click(screen.getByRole('button', { name: 'Confirm deletion' }))
  const button = (await screen.findByRole('button', {
    name: 'Deleting…',
  })) as HTMLButtonElement
  expect(button.disabled).toBe(true)
  expect(
    (
      screen.getByRole('button', {
        name: 'Add connection',
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true)
  fireEvent.click(button)
  expect(deleted).toBe(2)
  await act(async () => finish())
  await screen.findByRole('heading', { name: 'Start with the local demo' })
})

test('a save finishing after session cleanup cannot restore private query data', async () => {
  let finish!: () => void
  pending = new Promise((resolve) => {
    finish = resolve
  })
  await open()
  await add()
  fill()
  save()
  await screen.findByRole('button', { name: 'Saving…' })
  signedIn = false
  await act(async () => {
    client.setQueryData(['current-user'], null)
  })
  await screen.findByRole('heading', { name: 'Account access' })
  await act(async () => finish())
  expect(client.getQueryData(['connections', 'account-id'])).toBeUndefined()
  expect(client.getQueryData(['ssh-keys', 'account-id'])).toBeUndefined()
})

async function inspectHost() {
  records = [record]
  await open()
  await screen.findByRole('button', { name: 'Edit Local bastion' })
  fireEvent.click(screen.getByRole('button', { name: 'Connect' }))
  const picker = await screen.findByRole('dialog', {
    name: 'Choose a connection',
  })
  fireEvent.click(within(picker).getByRole('button', { name: /Local bastion/ }))
  await screen.findByRole('dialog', { name: 'Verify SSH host' })
}

test('host approval shows the fingerprint and rejection persists nothing', async () => {
  await inspectHost()
  await screen.findByText('SHA256:presented')
  fireEvent.click(screen.getByRole('button', { name: 'Reject and close' }))
  await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull())
  expect(approvals).toBe(0)
  fireEvent.click(screen.getByRole('button', { name: 'Connect' }))
  fireEvent.click(
    within(await screen.findByRole('dialog')).getByRole('button', {
      name: /Local bastion/,
    }),
  )
  fireEvent.click(
    await screen.findByRole('button', { name: 'Approve fingerprint' }),
  )
  await screen.findByText(/Host fingerprint verified/)
  expect(approvals).toBe(1)
})

test('changed host requires confirmed reset and a separate fresh approval', async () => {
  hostState = 'changed'
  await inspectHost()
  await screen.findByText('SHA256:previous')
  expect(
    screen.queryByRole('button', { name: 'Approve fingerprint' }),
  ).toBeNull()
  fireEvent.click(screen.getByRole('button', { name: 'Reset host trust' }))
  expect(resets).toBe(0)
  fireEvent.click(screen.getByRole('button', { name: 'Cancel reset' }))
  expect(resets).toBe(0)
  fireEvent.click(screen.getByRole('button', { name: 'Reset host trust' }))
  fireEvent.click(screen.getByRole('button', { name: 'Confirm trust reset' }))
  await screen.findByRole('button', { name: 'Approve fingerprint' })
  expect(resets).toBe(1)
  expect(approvals).toBe(0)
})

test('host inspection can retry and 401 approval clears private state', async () => {
  hostStatus = 502
  await inspectHost()
  await screen.findByText('Host inspection failed.')
  hostStatus = 200
  fireEvent.click(screen.getByRole('button', { name: 'Inspect again' }))
  await screen.findByRole('button', { name: 'Approve fingerprint' })
  hostStatus = 401
  fireEvent.click(screen.getByRole('button', { name: 'Approve fingerprint' }))
  await screen.findByRole('heading', { name: 'Account access' })
  expect(
    client.getQueryData(['host-inspection', 'account-id', record.id]),
  ).toBeUndefined()
})
