import { afterEach, expect, test } from 'bun:test'
import { QueryClient } from '@tanstack/react-query'
import type { Account } from '~/api/queries'
import queries from '~/api/queries'
import { clearSessionData } from '~/routes/_authed/route'
import { mockFetch } from './mock-fetch'

const currentUserOptions = queries.auth.currentUser
const logout = queries.auth.logout.mutationFn

const originalFetch = globalThis.fetch
let client = new QueryClient()
afterEach(() => {
  globalThis.fetch = originalFetch
  client.clear()
  client = new QueryClient()
})

test('successful logout clears private cached data and marks current user anonymous', async () => {
  client.setQueryData<Account | null>(currentUserOptions.queryKey, () => ({
    id: 'id',
    display_name: 'Cam',
  }))
  client.setQueryData(['connections'], [{ host: 'private-host' }])
  const fetcher = mockFetch(async () => new Response(null, { status: 204 }))
  globalThis.fetch = fetcher
  await logout()
  await clearSessionData(client)
  expect(fetcher.mock.calls[0]).toEqual([
    '/api/auth/logout',
    {
      method: 'POST',
      credentials: 'same-origin',
      headers: new Headers({
        'Content-Type': 'application/json',
        Accept: 'application/json',
      }),
      body: '{}',
      cache: 'no-store',
    },
  ])
  expect(client.getQueryData(['connections'])).toBeUndefined()
  expect(
    client.getQueryData<Account | null>(currentUserOptions.queryKey),
  ).toBeNull()
})

test('failed or uncertain logout preserves identity and allows manual retry', async () => {
  const account = { id: 'id', display_name: 'Cam' }
  client.setQueryData<Account | null>(currentUserOptions.queryKey, account)
  globalThis.fetch = mockFetch(async () => new Response(null, { status: 503 }))
  await expect(logout()).rejects.toThrow('Could not sign out')
  expect(
    client.getQueryData<Account | null>(currentUserOptions.queryKey),
  ).toEqual(account)
  globalThis.fetch = mockFetch(async () => {
    throw new TypeError('offline')
  })
  await expect(logout()).rejects.toThrow('Could not confirm sign-out')
  expect(
    client.getQueryData<Account | null>(currentUserOptions.queryKey),
  ).toEqual(account)
  globalThis.fetch = mockFetch(async () => new Response(null, { status: 204 }))
  await logout()
  await clearSessionData(client)
  expect(
    client.getQueryData<Account | null>(currentUserOptions.queryKey),
  ).toBeNull()
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
  expect(
    client.getQueryData<Account | null>(currentUserOptions.queryKey),
  ).toBeNull()
})
