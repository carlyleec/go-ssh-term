import { afterEach, expect, mock, test } from 'bun:test'
import { QueryClient } from '@tanstack/react-query'
import { isRedirect } from '@tanstack/react-router'
import {
  currentUserOptions,
  redirectSignedIn,
  requireAccount,
} from '../src/auth/current-user'

const originalFetch = globalThis.fetch
const clients: QueryClient[] = []
function client() {
  const value = new QueryClient()
  clients.push(value)
  return value
}
afterEach(() => {
  globalThis.fetch = originalFetch
  for (const value of clients) value.clear()
  clients.length = 0
})

test('anonymous workspace access redirects to login', async () => {
  globalThis.fetch = mock(async () =>
    Response.json({ error: 'sign in' }, { status: 401 }),
  ) as typeof fetch
  const error = await requireAccount(client()).catch((error) => error)
  expect(isRedirect(error)).toBe(true)
  expect(error.options.to).toBe('/login')
})

test('cached identity cannot authorize a navigation after session expiry', async () => {
  const queryClient = client()
  queryClient.setQueryData(currentUserOptions.queryKey, {
    id: 'old',
    display_name: 'Old',
  })
  globalThis.fetch = mock(async () =>
    Response.json({}, { status: 401 }),
  ) as typeof fetch
  const error = await requireAccount(queryClient).catch((error) => error)
  expect(isRedirect(error)).toBe(true)
  expect(queryClient.getQueryData(currentUserOptions.queryKey)).toBeNull()
})

test('valid identity enters workspace and redirects away from login', async () => {
  const account = { id: 'account-id', display_name: 'Cam' }
  globalThis.fetch = mock(async () =>
    Response.json({ account }),
  ) as typeof fetch
  expect(await requireAccount(client())).toEqual({ account })
  const error = await redirectSignedIn(client()).catch((error) => error)
  expect(isRedirect(error)).toBe(true)
  expect(error.options.to).toBe('/connections')
})

test('service failures are errors rather than anonymous redirects and can be retried', async () => {
  const queryClient = client()
  const fetcher = mock(async () => Response.json({}, { status: 503 }))
  globalThis.fetch = fetcher as typeof fetch
  const error = await requireAccount(queryClient).catch((error) => error)
  expect(error).toBeInstanceOf(Error)
  expect(isRedirect(error)).toBe(false)
  expect(fetcher).toHaveBeenCalledTimes(1)
  globalThis.fetch = mock(async () =>
    Response.json({ account: { id: 'id', display_name: 'Cam' } }),
  ) as typeof fetch
  expect((await requireAccount(queryClient)).account.id).toBe('id')
})

test('network failure and malformed identity never grant access', async () => {
  globalThis.fetch = mock(async () => {
    throw new TypeError('offline')
  }) as typeof fetch
  await expect(requireAccount(client())).rejects.toThrow('offline')
  globalThis.fetch = mock(async () =>
    Response.json({ account: {} }),
  ) as typeof fetch
  await expect(requireAccount(client())).rejects.toThrow('unexpected account')
})
