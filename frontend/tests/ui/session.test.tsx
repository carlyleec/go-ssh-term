import { afterEach, beforeEach, expect, spyOn, test } from 'bun:test'
import { createClock } from '@sinonjs/fake-timers'
import {
  focusManager,
  onlineManager,
  QueryClient,
  QueryClientProvider,
  timeoutManager,
} from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { act, cleanup, fireEvent, render, screen } from '@testing-library/react'
import { StrictMode } from 'react'
import { currentUserOptions } from '../../src/auth/current-user'
import type { Account } from '../../src/auth/passkeys'
import { routeTree } from '../../src/routetree.gen'
import { mockFetch } from '../mock-fetch'

const originalFetch = globalThis.fetch
let client: QueryClient
let signedIn: boolean
let status: number
let logoutStatus: number
let reads: number
let logouts: number
let caughtErrors: unknown[]
const clock = createClock()

// Control Query timers; React, router, and DOM waits keep real timers.
timeoutManager.setTimeoutProvider<number>({
  setTimeout: (callback, delay) =>
    Number(clock.setTimeout(() => callback(), delay)),
  clearTimeout: (id) => clock.clearTimeout(id),
  setInterval: (callback, delay) =>
    Number(clock.setInterval(() => callback(), delay)),
  clearInterval: (id) => clock.clearInterval(id),
})
let router: ReturnType<typeof createTestRouter>

function createTestRouter(path = '/connections') {
  return createRouter({
    routeTree,
    history: createMemoryHistory({ initialEntries: [path] }),
    context: { queryClient: client },
    defaultPendingMs: 0,
    defaultPendingMinMs: 0,
  })
}

beforeEach(() => {
  client = new QueryClient()
  signedIn = true
  status = 200
  logoutStatus = 204
  reads = 0
  logouts = 0
  caughtErrors = []
  Object.defineProperty(document, 'visibilityState', {
    configurable: true,
    value: 'visible',
  })
  focusManager.setFocused(undefined)
  onlineManager.setOnline(true)
  globalThis.fetch = mockFetch(async (input) => {
    const url = String(input)
    if (url === '/api/auth/me') {
      reads++
      if (status !== 200)
        return Response.json({ error: 'unavailable' }, { status })
      return signedIn
        ? Response.json({ account: { id: 'account-id', display_name: 'Cam' } })
        : Response.json({ error: 'sign in' }, { status: 401 })
    }
    if (url === '/api/auth/logout') {
      logouts++
      if (logoutStatus === 0) throw new TypeError('network disconnected')
      if (logoutStatus !== 204)
        return Response.json({ error: 'unavailable' }, { status: logoutStatus })
      signedIn = false
      return new Response(null, { status: 204 })
    }
    throw new Error(`Unexpected request: ${url}`)
  })
})

afterEach(async () => {
  cleanup()
  await client.cancelQueries()
  client.clear()
  clock.reset()
  focusManager.setFocused(undefined)
  onlineManager.setOnline(true)
  globalThis.fetch = originalFetch
})

async function open(path = '/connections') {
  router = createTestRouter(path)
  await act(async () => {
    render(
      <StrictMode>
        <QueryClientProvider client={client}>
          <RouterProvider router={router} />
        </QueryClientProvider>
      </StrictMode>,
      {
        onCaughtError: (error) => {
          caughtErrors.push(error)
        },
      },
    )
    await router.load()
  })
}

async function workspace() {
  await open()
  expect(await screen.findByRole('button', { name: 'Sign out' })).not.toBeNull()
}

async function expectLogin() {
  await screen.findByRole('heading', { name: 'Account access' })
  expect(router.state.location.pathname).toBe('/login')
  expect(screen.queryByRole('button', { name: 'Sign out' })).toBeNull()
}

function visibility(value: 'visible' | 'hidden') {
  Object.defineProperty(document, 'visibilityState', {
    configurable: true,
    value,
  })
  document.dispatchEvent(new Event('visibilitychange', { bubbles: true }))
}

test('sign-out uses the real mutation success callback to clear data and replace navigation', async () => {
  await workspace()
  client.setQueryData(['connections'], [{ host: 'private' }])
  fireEvent.click(screen.getByRole('button', { name: 'Sign out' }))
  await expectLogin()
  expect(logouts).toBe(1)
  expect(client.getQueryData(['connections'])).toBeUndefined()
  expect(client.getQueryData(currentUserOptions.queryKey)).toBeNull()
  expect(router.history.length).toBe(1)
})

test('failed logout stays in the workspace and permits a manual retry', async () => {
  await workspace()
  logoutStatus = 503
  fireEvent.click(screen.getByRole('button', { name: 'Sign out' }))
  await screen.findByRole('alert')
  expect(router.state.location.pathname).toBe('/connections')
  expect(client.getQueryData(currentUserOptions.queryKey)).not.toBeNull()
  expect(logouts).toBe(1)
  logoutStatus = 204
  fireEvent.click(screen.getByRole('button', { name: 'Sign out' }))
  await expectLogin()
  expect(logouts).toBe(2)
})

test('returning to a visible tab revalidates an expired session', async () => {
  await workspace()
  visibility('hidden')
  signedIn = false
  const previousReads = reads
  visibility('visible')
  await expectLogin()
  expect(reads).toBeGreaterThan(previousReads)
})

test('reconnecting revalidates an expired session', async () => {
  await workspace()
  window.dispatchEvent(new Event('offline'))
  signedIn = false
  window.dispatchEvent(new Event('online'))
  await expectLogin()
})

test('route error retry renders the workspace after the service recovers', async () => {
  const originalWarn = console.warn
  const warning = spyOn(console, 'warn').mockImplementation((...args) => {
    if (String(args[0]).startsWith('Warning: Error in route match:')) return
    originalWarn(...args)
  })
  try {
    status = 503
    await open()
    await screen.findByRole('heading', { name: 'Could not check your session' })
    expect(caughtErrors).toHaveLength(1)
    expect(caughtErrors[0]).toEqual(
      new Error('Could not check your session. Please try again.'),
    )
    expect(router.state.location.pathname).toBe('/connections')
    status = 200
    fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(
      await screen.findByRole('button', { name: 'Sign out' }),
    ).not.toBeNull()
  } finally {
    warning.mockRestore()
  }
})

test('background revalidation failure hides private UI and retry restores it', async () => {
  await workspace()
  status = 503
  visibility('hidden')
  visibility('visible')
  await screen.findByRole('heading', { name: 'Could not check your session' })
  expect(screen.queryByText(/Signed in as/)).toBeNull()
  expect(router.state.location.pathname).toBe('/connections')
  status = 200
  fireEvent.click(screen.getByRole('button', { name: 'Try again' }))
  await screen.findByRole('button', { name: 'Sign out' })
})

test('visible workspace polling detects expiry without a navigation or focus event', async () => {
  await workspace()
  signedIn = false
  const previousReads = reads
  await act(async () => {
    await clock.tickAsync(29_999)
  })
  expect(reads).toBe(previousReads)
  await act(async () => {
    await clock.tickAsync(1)
  })
  await act(async () => {
    await clock.tickAsync(0)
  })
  await expectLogin()
  expect(client.getQueryData(currentUserOptions.queryKey)).toBeNull()
})

test('hidden workspace skips polling and catches expiry when it becomes visible', async () => {
  await workspace()
  visibility('hidden')
  signedIn = false
  const previousReads = reads
  await act(async () => {
    await clock.tickAsync(90_000)
  })
  expect(reads).toBe(previousReads)
  expect(router.state.location.pathname).toBe('/connections')
  await act(async () => {
    visibility('visible')
  })
  await act(async () => {
    await clock.tickAsync(0)
  })
  await expectLogin()
})

test('an anonymous initial visit redirects without displaying private workspace content', async () => {
  signedIn = false
  client.setQueryData<Account | null>(currentUserOptions.queryKey, () => ({
    id: 'stale-account',
    display_name: 'Stale identity',
  }))
  await open()
  await expectLogin()
  expect(screen.queryByText(/Stale identity/)).toBeNull()
})

test('an uncertain logout keeps the workspace available for retry', async () => {
  await workspace()
  logoutStatus = 0
  fireEvent.click(screen.getByRole('button', { name: 'Sign out' }))
  const alert = await screen.findByRole('alert')
  expect(alert.textContent).toContain('Could not confirm sign-out')
  expect(router.state.location.pathname).toBe('/connections')
  expect(logouts).toBe(1)
  logoutStatus = 204
  fireEvent.click(screen.getByRole('button', { name: 'Sign out' }))
  await expectLogin()
})

test('pending logout disables duplicate submission and waits for server success', async () => {
  await workspace()
  const api = globalThis.fetch
  let finish!: () => void
  const pending = new Promise<void>((resolve) => {
    finish = resolve
  })
  globalThis.fetch = mockFetch(async (input, init) => {
    if (String(input) === '/api/auth/logout') await pending
    return api(input, init)
  })
  fireEvent.click(screen.getByRole('button', { name: 'Sign out' }))
  const button = await screen.findByRole('button', { name: 'Signing out…' })
  expect((button as HTMLButtonElement).disabled).toBe(true)
  fireEvent.click(button)
  expect(router.state.location.pathname).toBe('/connections')
  expect(client.getQueryData(currentUserOptions.queryKey)).not.toBeNull()
  await act(async () => {
    finish()
  })
  await expectLogin()
  expect(logouts).toBe(1)
})
