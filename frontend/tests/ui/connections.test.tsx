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
import {
  FakeSocket,
  FakeTerminal,
  installTerminals,
  restoreTerminals,
} from './terminal-fakes'

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
  installTerminals()
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
        hop: 'target',
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
      if (fields.jump_connection_id === null) delete fields.jump_connection_id
      const id = init.method === 'PUT' ? url.split('/').at(-1) : record.id
      const updated = { ...record, ...fields, id }
      records = [...records.filter((item) => item.id !== id), updated]
      return Response.json(
        { connection: updated },
        { status: init.method === 'POST' ? 201 : 200 },
      )
    }
    if (init?.method === 'DELETE') {
      deleted++
      await pending
      if (deleteStatus === 401) signedIn = false
      if (deleteStatus !== 204)
        return Response.json(
          {
            error:
              deleteStatus === 409
                ? 'Connection is used as a jump; update or delete dependent connections first.'
                : 'Could not delete connection.',
          },
          { status: deleteStatus },
        )
      records = records.filter((item) => item.id !== url.split('/').at(-1))
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
  restoreTerminals()
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
  expect(FakeSocket.instances).toHaveLength(0)
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
  expect(FakeSocket.instances).toHaveLength(0)
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

test('verified picker opens one terminal and session cleanup disposes it', async () => {
  hostState = 'trusted'
  await inspectHost()
  fireEvent.click(await screen.findByRole('button', { name: 'Open terminal' }))
  await screen.findByRole('region', { name: 'Terminal for Local bastion' })
  await waitFor(() => expect(FakeSocket.instances).toHaveLength(1))
  expect(
    (screen.getByRole('button', { name: 'Connect' }) as HTMLButtonElement)
      .disabled,
  ).toBe(true)
  signedIn = false
  await act(async () => client.setQueryData(['current-user'], null))
  await screen.findByRole('heading', { name: 'Account access' })
  expect(FakeSocket.instances[0]?.closed).toBe(1)
  expect(FakeTerminal.instances[0]?.disposed).toBe(true)
})

test('closing a terminal allows another explicit connection', async () => {
  hostState = 'trusted'
  await inspectHost()
  fireEvent.click(await screen.findByRole('button', { name: 'Open terminal' }))
  await waitFor(() => expect(FakeSocket.instances).toHaveLength(1))
  fireEvent.click(screen.getByRole('button', { name: 'Close terminal' }))
  expect(FakeSocket.instances[0]?.closed).toBe(1)
  expect(
    (screen.getByRole('button', { name: 'Connect' }) as HTMLButtonElement)
      .disabled,
  ).toBe(false)
})

async function endedTerminal() {
  hostState = 'trusted'
  await inspectHost()
  fireEvent.click(await screen.findByRole('button', { name: 'Open terminal' }))
  await waitFor(() => expect(FakeSocket.instances).toHaveLength(1))
  act(() => FakeSocket.instances[0]?.status('disconnected', 'Shell exited.'))
  await screen.findByRole('button', { name: 'Reconnect' })
}

test('manual reconnect rechecks current configuration and trust before replacing the shell', async () => {
  await endedTerminal()
  expect(FakeSocket.instances).toHaveLength(1)
  records = [{ ...record, name: 'Updated destination', host: 'new-host' }]
  hostState = 'changed'
  fireEvent.click(screen.getByRole('button', { name: 'Reconnect' }))
  await screen.findByText(/host key changed/i)
  expect(screen.queryByRole('button', { name: 'Open terminal' })).toBeNull()
  expect(FakeSocket.instances).toHaveLength(1)
  expect(FakeTerminal.instances[0]?.disposed).toBe(false)
  fireEvent.click(screen.getByRole('button', { name: 'Close' }))
  hostState = 'trusted'
  fireEvent.click(screen.getByRole('button', { name: 'Reconnect' }))
  fireEvent.click(await screen.findByRole('button', { name: 'Open terminal' }))
  await waitFor(() => expect(FakeSocket.instances).toHaveLength(2))
  expect(FakeTerminal.instances[0]?.disposed).toBe(true)
  await screen.findByRole('region', {
    name: 'Terminal for Updated destination',
  })
  expect(screen.queryByRole('button', { name: 'Reconnect' })).toBeNull()
  act(() => FakeSocket.instances[1]?.status('failed', 'Could not connect.'))
  await screen.findByRole('button', { name: 'Reconnect' })
  expect(FakeSocket.instances).toHaveLength(2)
})

test('manual reconnect handles deleted configurations, storage failures, and expiry', async () => {
  await endedTerminal()
  records = []
  fireEvent.click(screen.getByRole('button', { name: 'Reconnect' }))
  await screen.findByText(/This saved connection was deleted/)
  expect(FakeSocket.instances).toHaveLength(1)
  listStatus = 503
  fireEvent.click(screen.getByRole('button', { name: 'Reconnect' }))
  await screen.findByText('Could not prepare reconnect. Try again.')
  listStatus = 401
  fireEvent.click(screen.getByRole('button', { name: 'Reconnect' }))
  await screen.findByRole('heading', { name: 'Account access' })
  expect(FakeSocket.instances).toHaveLength(1)
  expect(FakeTerminal.instances[0]?.disposed).toBe(true)
})

test('closing during reconnect lookup ignores its late result and duplicate clicks', async () => {
  await endedTerminal()
  const fetchBefore = globalThis.fetch
  let release: (() => void) | undefined
  const blocked = new Promise<void>((resolve) => {
    release = resolve
  })
  let lookups = 0
  globalThis.fetch = mockFetch(async (input, init) => {
    if (String(input) === '/api/connections') {
      lookups++
      await blocked
    }
    return fetchBefore(input, init)
  })
  const reconnect = screen.getByRole('button', { name: 'Reconnect' })
  fireEvent.click(reconnect)
  fireEvent.click(reconnect)
  fireEvent.click(screen.getByRole('button', { name: 'Close terminal' }))
  await act(async () => {
    release?.()
    await blocked
  })
  expect(lookups).toBe(1)
  expect(screen.queryByRole('heading', { name: 'Verify SSH host' })).toBeNull()
  expect(FakeSocket.instances).toHaveLength(1)
})

test('repeated manual reconnects reset terminal state and ignore callbacks from older attempts', async () => {
  await endedTerminal()
  let previousMessage: FakeSocket['onmessage'] = null
  for (let cycle = 0; cycle < 3; cycle++) {
    const oldTerminal = FakeTerminal.instances.at(-1)
    fireEvent.click(screen.getByRole('button', { name: 'Reconnect' }))
    fireEvent.click(
      await screen.findByRole('button', { name: 'Open terminal' }),
    )
    await waitFor(() => expect(FakeSocket.instances).toHaveLength(cycle + 2))
    const socket = FakeSocket.instances.at(-1)
    const terminal = FakeTerminal.instances.at(-1)
    if (!socket || !terminal) throw new Error('Missing replacement terminal')
    expect(oldTerminal?.disposed).toBe(true)
    act(() => {
      previousMessage?.({
        data: '{"type":"status","state":"connected"}',
      } as MessageEvent)
      previousMessage?.({
        data: new TextEncoder().encode('stale output').buffer,
      } as MessageEvent)
    })
    expect(screen.queryByText('Connected')).toBeNull()
    expect(terminal.writes).toHaveLength(0)
    expect(terminal.options.disableStdin).toBe(true)
    const late = socket.onmessage
    act(() => {
      socket.open()
      socket.status('connected')
      socket.message(new TextEncoder().encode(`output ${cycle}`).buffer)
      socket.status(cycle % 2 ? 'failed' : 'disconnected', 'Attempt ended.')
    })
    await screen.findByRole('button', { name: 'Reconnect' })
    expect(terminal.writes).toHaveLength(1)
    expect(socket.closed).toBe(1)
    act(() =>
      late?.({ data: '{"type":"status","state":"connected"}' } as MessageEvent),
    )
    expect(terminal.options.disableStdin).toBe(true)
    expect(FakeSocket.instances).toHaveLength(cycle + 2)
    previousMessage = late
  }
})

test.each(['logout', 'expiry'])(
  '%s during replacement setup disposes the new terminal and ignores late readiness',
  async (reason) => {
    await endedTerminal()
    fireEvent.click(screen.getByRole('button', { name: 'Reconnect' }))
    fireEvent.click(
      await screen.findByRole('button', { name: 'Open terminal' }),
    )
    await waitFor(() => expect(FakeSocket.instances).toHaveLength(2))
    const socket = FakeSocket.instances[1]
    const late = socket?.onmessage
    signedIn = false
    if (reason === 'logout') {
      const previous = globalThis.fetch
      globalThis.fetch = mockFetch(async (input, init) => {
        if (String(input) === '/api/auth/logout')
          return new Response(null, { status: 204 })
        return previous(input, init)
      })
      fireEvent.click(screen.getByRole('button', { name: 'Cam' }))
      fireEvent.click(screen.getByRole('button', { name: 'Sign out' }))
    } else {
      await act(async () => {
        await client.invalidateQueries({ queryKey: ['current-user'] })
      })
    }
    await screen.findByRole('heading', { name: 'Account access' })
    act(() =>
      late?.({ data: '{"type":"status","state":"connected"}' } as MessageEvent),
    )
    expect(socket?.closed).toBe(1)
    expect(FakeTerminal.instances.every((terminal) => terminal.disposed)).toBe(
      true,
    )
    expect(FakeSocket.instances).toHaveLength(2)
    expect(screen.queryByRole('button', { name: 'Reconnect' })).toBeNull()
  },
)

test('editing connection details preserves an existing jump reference', async () => {
  const jumpID = '33333333-3333-4333-8333-333333333333'
  records = [
    { ...record, id: jumpID, name: 'Jump host' },
    { ...record, jump_connection_id: jumpID },
  ]
  await open()
  fireEvent.click(
    await screen.findByRole('button', { name: 'Edit Local bastion' }),
  )
  await screen.findByRole('dialog', { name: 'Edit connection' })
  fireEvent.change(screen.getByLabelText('Connection name'), {
    target: { value: 'Renamed target' },
  })
  save()
  await screen.findByRole('button', { name: 'Edit Renamed target' })
  expect(
    records.find((item) => item.id === record.id)?.jump_connection_id,
  ).toBe(jumpID)
})

const jumpRecord: SavedConnection = {
  ...record,
  id: '33333333-3333-4333-8333-333333333333',
  name: 'Jump host',
}
const chainedRecord: SavedConnection = {
  ...record,
  id: '44444444-4444-4444-8444-444444444444',
  name: 'Private target',
  jump_connection_id: jumpRecord.id,
}

test('jump selector creates a routed target and excludes multi-hop choices', async () => {
  records = [jumpRecord, chainedRecord]
  await open()
  await add()
  fill()
  const selector = screen.getByLabelText('Jump through')
  expect(
    within(selector).queryByRole('option', { name: /Private target/ }),
  ).toBeNull()
  fireEvent.change(selector, { target: { value: jumpRecord.id } })
  save()
  await screen.findByRole('button', { name: 'Edit Local bastion' })
  expect(
    records.find((item) => item.id === record.id)?.jump_connection_id,
  ).toBe(jumpRecord.id)
  expect(screen.getAllByText('Jump through: Jump host')).toHaveLength(2)
  fireEvent.click(screen.getByRole('button', { name: 'Edit Local bastion' }))
  await screen.findByRole('dialog', { name: 'Edit connection' })
  const edit = screen.getByLabelText('Jump through') as HTMLSelectElement
  expect(edit.value).toBe(jumpRecord.id)
  expect(
    within(edit).queryByRole('option', { name: /Local bastion/ }),
  ).toBeNull()
  fireEvent.change(edit, { target: { value: '' } })
  save()
  await waitFor(() => expect(screen.queryByRole('dialog') === null).toBe(true))
  expect(
    records.find((item) => item.id === record.id)?.jump_connection_id,
  ).toBeUndefined()
})

test('a referenced jump must stay direct and deletion conflicts preserve the configuration', async () => {
  records = [jumpRecord, chainedRecord, record]
  await open()
  fireEvent.click(await screen.findByRole('button', { name: 'Edit Jump host' }))
  await screen.findByRole('dialog', { name: 'Edit connection' })
  await screen.findByText(/must remain direct/)
  expect(
    within(screen.getByLabelText('Jump through')).getAllByRole('option'),
  ).toHaveLength(1)
  fireEvent.click(screen.getByRole('button', { name: 'Close' }))
  deleteStatus = 409
  fireEvent.click(screen.getByRole('button', { name: 'Delete Jump host' }))
  fireEvent.click(screen.getByRole('button', { name: 'Confirm deletion' }))
  await screen.findByText(/Connection is used as a jump/)
  expect(records).toHaveLength(3)
  fireEvent.click(screen.getByRole('button', { name: 'Cancel deletion' }))
  fireEvent.click(screen.getByRole('button', { name: 'Edit Private target' }))
  await screen.findByRole('dialog', { name: 'Edit connection' })
  fireEvent.change(screen.getByLabelText('Jump through'), {
    target: { value: '' },
  })
  save()
  await waitFor(() => expect(screen.queryByRole('dialog') === null).toBe(true))
  deleteStatus = 204
  fireEvent.click(screen.getByRole('button', { name: 'Delete Jump host' }))
  fireEvent.click(screen.getByRole('button', { name: 'Confirm deletion' }))
  await screen.findByText('Connection deleted.')
  expect(records.some((item) => item.id === jumpRecord.id)).toBe(false)
  expect(records.some((item) => item.id === chainedRecord.id)).toBe(true)
})

test('stale jump choices cannot silently become direct connections', async () => {
  records = [jumpRecord, { ...record, jump_connection_id: jumpRecord.id }]
  await open()
  fireEvent.click(
    await screen.findByRole('button', { name: 'Edit Local bastion' }),
  )
  await screen.findByRole('dialog', { name: 'Edit connection' })
  records = [{ ...record, jump_connection_id: jumpRecord.id }]
  await act(async () => {
    await client.invalidateQueries({ queryKey: ['connections', 'account-id'] })
  })
  await screen.findByText(/This jump is no longer available/)
  save()
  expect(saved).toBe(0)
  expect(
    (screen.getByLabelText('Jump through') as HTMLSelectElement).value,
  ).toBe(jumpRecord.id)
  fireEvent.change(screen.getByLabelText('Jump through'), {
    target: { value: '' },
  })
  save()
  await waitFor(() => expect(saved).toBe(1))
})

test('jump list errors retry without discarding inputs and pending saves freeze the choice', async () => {
  records = [jumpRecord]
  await open()
  listStatus = 503
  await add()
  fill()
  await screen.findByRole('button', { name: 'Retry jump connections' })
  expect(
    (
      screen.getByRole('button', {
        name: 'Save connection',
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true)
  listStatus = 200
  fireEvent.click(
    screen.getByRole('button', { name: 'Retry jump connections' }),
  )
  await waitFor(() =>
    expect(
      (screen.getByLabelText('Jump through') as HTMLSelectElement).disabled,
    ).toBe(false),
  )
  fireEvent.change(screen.getByLabelText('Jump through'), {
    target: { value: jumpRecord.id },
  })
  saveStatus = 400
  save()
  await screen.findByText('Could not save connection.')
  expect(
    (screen.getByLabelText('Jump through') as HTMLSelectElement).value,
  ).toBe(jumpRecord.id)
  saveStatus = 200
  let release: (() => void) | undefined
  pending = new Promise<void>((resolve) => {
    release = resolve
  })
  save()
  await screen.findByRole('button', { name: 'Saving…' })
  expect(
    (screen.getByLabelText('Jump through') as HTMLSelectElement).disabled,
  ).toBe(true)
  await act(async () => {
    release?.()
    await pending
  })
  await screen.findByRole('button', { name: 'Edit Local bastion' })
  expect(
    records.find((item) => item.id === record.id)?.jump_connection_id,
  ).toBe(jumpRecord.id)
})
