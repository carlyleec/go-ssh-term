import { afterEach, beforeEach, expect, test } from 'bun:test'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import { StrictMode } from 'react'
import type { SchemaImportRequest } from '~/api/generated/schema.gen'
import { ImportModal } from '~/routes/_authed/connections/-components/import-modal'
import { mockFetch } from '../mock-fetch'

const originalFetch = globalThis.fetch
const keyID = '11111111-1111-4111-8111-111111111111'
let client: QueryClient
let saved: number
let closed: number
let request: SchemaImportRequest | undefined
let confirmStatus: number
let diagnostics: { line: number; message: string }[]
let dataFailure: boolean
let hold: Promise<void> | undefined
const entries = [
  {
    name: 'bastion',
    host: 'bastion',
    username: 'demo',
    port: 22,
    identity: 'demo/key',
    jump: '',
    line: 1,
  },
  {
    name: 'target-1',
    host: 'target-1',
    username: 'demo',
    port: 22,
    identity: 'demo/key',
    jump: 'bastion',
    line: 6,
  },
]
beforeEach(() => {
  client = new QueryClient()
  saved = 0
  closed = 0
  confirmStatus = 201
  diagnostics = []
  dataFailure = false
  request = undefined
  hold = undefined
  globalThis.fetch = mockFetch(async (input, init) => {
    const url = String(input)
    if (url === '/api/keys')
      return dataFailure
        ? Response.json({ error: 'failed' }, { status: 503 })
        : Response.json({
            keys: [
              {
                id: keyID,
                name: 'Demo',
                public_fingerprint: 'SHA256:demo',
                created_at: '2026-10-04T12:00:00Z',
              },
            ],
          })
    if (url === '/api/connections') return Response.json({ connections: [] })
    if (url.endsWith('/import/preview')) {
      request = JSON.parse(String(init?.body)) as SchemaImportRequest
      const issues = (request.selections ?? []).flatMap((s) => [
        ...(!s.ssh_key_id
          ? [{ name: s.name, message: 'select an uploaded SSH key' }]
          : []),
        ...(s.name === 'target-1' &&
        !request?.selections?.some((item) => item.name === 'bastion')
          ? [{ name: s.name, message: 'select the jump entry' }]
          : []),
      ])
      return Response.json({
        entries,
        diagnostics,
        issues,
        can_confirm:
          !!request.selections?.length &&
          issues.length === 0 &&
          diagnostics.length === 0,
      })
    }
    if (url.endsWith('/import/confirm')) {
      saved++
      if (hold) await hold
      if (confirmStatus !== 201)
        return Response.json(
          { error: 'import is no longer valid; preview the selection again' },
          { status: confirmStatus },
        )
      return Response.json({ connections: [] }, { status: 201 })
    }
    throw new Error(`unexpected request ${url}`)
  })
})
afterEach(() => {
  cleanup()
  client.clear()
  globalThis.fetch = originalFetch
})
function mount() {
  render(
    <StrictMode>
      <QueryClientProvider client={client}>
        <ImportModal
          accountID="owner"
          onClose={() => closed++}
          onImported={() => closed++}
        />
      </QueryClientProvider>
    </StrictMode>,
  )
}
async function load() {
  const file = new File(['sample'], 'ssh_config')
  fireEvent.change(screen.getByLabelText('SSH config file'), {
    target: { files: [file] },
  })
  await screen.findByLabelText('Key for bastion')
}
async function ready() {
  await load()
  await waitFor(() =>
    expect(screen.getAllByRole('option', { name: /Demo/ }).length).toBe(2),
  )
  for (const name of ['bastion', 'target-1'])
    fireEvent.change(screen.getByLabelText(`Key for ${name}`), {
      target: { value: keyID },
    })
  fireEvent.click(screen.getByRole('button', { name: 'Check selection' }))
  await screen.findByText(/Ready to import 2/)
}
test('file preview and cancellation write nothing', async () => {
  mount()
  await load()
  expect(
    (
      screen.getByRole('button', {
        name: 'Confirm import',
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true)
  fireEvent.click(screen.getByRole('button', { name: 'Cancel' }))
  expect(saved).toBe(0)
  expect(closed).toBe(1)
})
test('explicit mappings and review are required before confirmation', async () => {
  mount()
  await ready()
  expect(saved).toBe(0)
  expect(request?.selections?.every((s) => s.ssh_key_id === keyID)).toBe(true)
  fireEvent.click(screen.getByRole('button', { name: 'Confirm import' }))
  await waitFor(() => expect(closed).toBe(1))
  expect(saved).toBe(1)
})
test('changing selection invalidates review and reports deselected jumps', async () => {
  mount()
  await ready()
  fireEvent.click(screen.getByRole('checkbox', { name: 'bastion' }))
  expect(
    (
      screen.getByRole('button', {
        name: 'Confirm import',
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true)
  fireEvent.click(screen.getByRole('button', { name: 'Check selection' }))
  await screen.findByText('target-1: select the jump entry')
  expect(saved).toBe(0)
})
test('missing mappings and unsupported syntax block confirmation', async () => {
  mount()
  await load()
  fireEvent.click(screen.getByRole('button', { name: 'Check selection' }))
  await screen.findByText('bastion: select an uploaded SSH key')
  diagnostics = [{ line: 4, message: 'unsupported directive: ProxyCommand' }]
  fireEvent.change(screen.getByLabelText('SSH config file'), {
    target: { files: [new File(['invalid'], 'bad')] },
  })
  await screen.findByText('Line 4: unsupported directive: ProxyCommand')
  expect(
    (
      screen.getByRole('button', {
        name: 'Check selection',
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true)
})
test('oversized files are rejected before upload', async () => {
  mount()
  fireEvent.change(screen.getByLabelText('SSH config file'), {
    target: { files: [new File(['x'.repeat(65537)], 'big')] },
  })
  await screen.findByText('Config must be at most 64 KiB.')
  expect(request).toBeUndefined()
})
test('stale confirmation requires another review', async () => {
  mount()
  await ready()
  confirmStatus = 409
  fireEvent.click(screen.getByRole('button', { name: 'Confirm import' }))
  await screen.findByText(
    'import is no longer valid; preview the selection again',
  )
  expect(
    (
      screen.getByRole('button', {
        name: 'Confirm import',
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true)
  expect(closed).toBe(0)
})
test('pending confirmation prevents double submits and cancellation', async () => {
  mount()
  await ready()
  let release = () => {}
  hold = new Promise<void>((resolve) => {
    release = resolve
  })
  fireEvent.click(screen.getByRole('button', { name: 'Confirm import' }))
  fireEvent.click(screen.getByRole('button', { name: 'Confirm import' }))
  await waitFor(() => expect(saved).toBe(1))
  expect(
    (screen.getByRole('button', { name: 'Cancel' }) as HTMLButtonElement)
      .disabled,
  ).toBe(true)
  release()
  await waitFor(() => expect(closed).toBe(1))
})
test('metadata load failure blocks review and can be retried', async () => {
  dataFailure = true
  mount()
  await load()
  await screen.findByText('Could not load keys or saved connections.')
  expect(
    (
      screen.getByRole('button', {
        name: 'Check selection',
      }) as HTMLButtonElement
    ).disabled,
  ).toBe(true)
  dataFailure = false
  fireEvent.click(screen.getByRole('button', { name: 'Retry import data' }))
  await waitFor(() =>
    expect(
      screen.queryByText('Could not load keys or saved connections.'),
    ).toBeNull(),
  )
})
