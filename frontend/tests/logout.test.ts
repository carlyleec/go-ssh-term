import { afterEach, expect, mock, test } from 'bun:test'
import { QueryClient } from '@tanstack/react-query'
import { currentUserOptions } from '../src/auth/current-user'
import { clearSessionData, logout } from '../src/auth/logout'

const originalFetch = globalThis.fetch
let client = new QueryClient()
afterEach(() => {
  globalThis.fetch = originalFetch
  client.clear()
  client = new QueryClient()
})

test('successful logout clears private cached data and marks current user anonymous', async () => {
  client.setQueryData(currentUserOptions.queryKey, {
    id: 'id',
    display_name: 'Cam',
  })
  client.setQueryData(['connections'], [{ host: 'private-host' }])
  const fetcher = mock(async () => new Response(null, { status: 204 }))
  globalThis.fetch = fetcher as typeof fetch
  await logout()
  await clearSessionData(client)
  expect(fetcher.mock.calls[0]).toEqual([
    '/api/auth/logout',
    {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: '{}',
      cache: 'no-store',
    },
  ])
  expect(client.getQueryData(['connections'])).toBeUndefined()
  expect(client.getQueryData(currentUserOptions.queryKey)).toBeNull()
})

test('failed or uncertain logout preserves identity and allows manual retry', async () => {
  const account = { id: 'id', display_name: 'Cam' }
  client.setQueryData(currentUserOptions.queryKey, account)
  globalThis.fetch = mock(
    async () => new Response(null, { status: 503 }),
  ) as typeof fetch
  await expect(logout()).rejects.toThrow('Could not sign out')
  expect(client.getQueryData(currentUserOptions.queryKey)).toEqual(account)
  globalThis.fetch = mock(async () => {
    throw new TypeError('offline')
  }) as typeof fetch
  await expect(logout()).rejects.toThrow('Could not confirm sign-out')
  expect(client.getQueryData(currentUserOptions.queryKey)).toEqual(account)
  globalThis.fetch = mock(
    async () => new Response(null, { status: 204 }),
  ) as typeof fetch
  await logout()
  await clearSessionData(client)
  expect(client.getQueryData(currentUserOptions.queryKey)).toBeNull()
})

test('session cleanup cancels pending reads so late results cannot repopulate private data', async () => {
  let resolve!: (value: string) => void
  const pending = client
    .fetchQuery({
      queryKey: ['connections'],
      queryFn: () =>
        new Promise<string>((done) => {
          resolve = done
        }),
    })
    .catch(() => undefined)
  await clearSessionData(client)
  resolve('private data')
  await pending
  expect(client.getQueryData(['connections'])).toBeUndefined()
  expect(client.getQueryData(currentUserOptions.queryKey)).toBeNull()
})
