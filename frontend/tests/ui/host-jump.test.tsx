import { afterEach, beforeEach, expect, test } from 'bun:test'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import { HostModal } from '~/routes/_authed/connections/-components/host-modal'
import { mockFetch } from '../mock-fetch'

const originalFetch = globalThis.fetch
const target = {
  id: '22222222-2222-4222-8222-222222222222',
  jump_connection_id: '33333333-3333-4333-8333-333333333333',
  name: 'Private target',
  host: 'target-1',
  port: 22,
  username: 'demo',
  ssh_key_id: '11111111-1111-4111-8111-111111111111',
  created_at: '',
  updated_at: '',
}
let client: QueryClient
let bastionState: string
let targetState: string
let opened: number
let closed: number
let failure: number
let decisions: Array<Record<string, unknown>>
beforeEach(() => {
  client = new QueryClient()
  bastionState = 'unknown'
  targetState = 'unknown'
  opened = closed = failure = 0
  decisions = []
  globalThis.fetch = mockFetch(async (input, init) => {
    const url = String(input)
    expect(url.startsWith(`/api/connections/${target.id}/host-`)).toBe(true)
    const body = JSON.parse(String(init?.body))
    const deciding = !url.endsWith('/host-key')
    const bastion = deciding
      ? body.jump_connection_id === target.jump_connection_id
      : bastionState !== 'trusted'
    if (deciding) {
      decisions.push(body)
      if (failure)
        return Response.json(
          { error: 'jump connection changed; inspect again', hop: 'bastion' },
          { status: failure },
        )
      expect(body.host).toBe(bastion ? 'bastion' : 'target-1')
      expect(body.fingerprint).toBe(
        url.endsWith('/reset')
          ? 'SHA256:old-bastion'
          : bastion
            ? 'SHA256:bastion'
            : 'SHA256:target',
      )
      if (url.endsWith('/reset')) {
        bastionState = 'unknown'
        return new Response(null, { status: 204 })
      }
      if (bastion) bastionState = 'trusted'
      else targetState = 'trusted'
    }
    return Response.json({
      hop: bastion ? 'bastion' : 'target',
      ...(bastion ? { jump_connection_id: target.jump_connection_id } : {}),
      host: bastion ? 'bastion' : 'target-1',
      port: 22,
      state: bastion ? bastionState : targetState,
      algorithm: 'ssh-ed25519',
      fingerprint: bastion ? 'SHA256:bastion' : 'SHA256:target',
      trusted_fingerprint:
        bastion && bastionState === 'changed' ? 'SHA256:old-bastion' : '',
    })
  })
})
afterEach(() => {
  cleanup()
  client.clear()
  globalThis.fetch = originalFetch
})
function show() {
  render(
    <QueryClientProvider client={client}>
      <HostModal
        accountID="owner"
        connection={target}
        onClose={() => {
          closed++
        }}
        onConnect={() => {
          opened++
        }}
      />
    </QueryClientProvider>,
  )
}

test('approves bastion then independently verifies target before opening', async () => {
  show()
  await screen.findByText('Bastion fingerprint')
  expect(screen.queryByRole('button', { name: 'Open terminal' }) === null).toBe(
    true,
  )
  fireEvent.click(screen.getByRole('button', { name: 'Approve fingerprint' }))
  await screen.findByText('Target fingerprint')
  expect(decisions[0]?.jump_connection_id).toBe(target.jump_connection_id)
  expect(screen.queryByRole('button', { name: 'Open terminal' }) === null).toBe(
    true,
  )
  expect(opened).toBe(0)
  fireEvent.click(screen.getByRole('button', { name: 'Approve fingerprint' }))
  fireEvent.click(await screen.findByRole('button', { name: 'Open terminal' }))
  expect(decisions[1]?.jump_connection_id).toBeUndefined()
  expect(opened).toBe(1)
})

test('changed bastion requires reset and fresh approval before traversing', async () => {
  bastionState = 'changed'
  show()
  await screen.findByText('SHA256:old-bastion')
  expect(
    screen.queryByRole('button', { name: 'Approve fingerprint' }) === null,
  ).toBe(true)
  fireEvent.click(screen.getByRole('button', { name: 'Reset host trust' }))
  expect(decisions).toHaveLength(0)
  fireEvent.click(screen.getByRole('button', { name: 'Confirm trust reset' }))
  await screen.findByRole('button', { name: 'Approve fingerprint' })
  expect(decisions).toHaveLength(1)
  expect(screen.queryByText('Target fingerprint') === null).toBe(true)
  expect(opened).toBe(0)
  fireEvent.click(screen.getByRole('button', { name: 'Approve fingerprint' }))
  await screen.findByText('Target fingerprint')
})

test('stale bastion decision identifies hop and allows explicit reinspection', async () => {
  show()
  failure = 409
  fireEvent.click(
    await screen.findByRole('button', { name: 'Approve fingerprint' }),
  )
  await screen.findByText('Bastion: jump connection changed; inspect again')
  expect(opened).toBe(0)
  expect(screen.queryByRole('button', { name: 'Open terminal' }) === null).toBe(
    true,
  )
  failure = 0
  fireEvent.click(screen.getByRole('button', { name: 'Inspect again' }))
  await waitFor(() => expect(screen.queryByRole('alert') === null).toBe(true))
})

test('rejecting bastion closes without approving either host', async () => {
  show()
  fireEvent.click(
    await screen.findByRole('button', { name: 'Reject and close' }),
  )
  await waitFor(() => expect(closed).toBe(1))
  expect(decisions).toHaveLength(0)
  expect(opened).toBe(0)
})
