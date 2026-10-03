import { afterEach, expect, mock, test } from 'bun:test'
import { mockFetch } from './mock-fetch'

const registration = mock(async () => ({ id: 'created-credential' }))
const authentication = mock(async () => ({ id: 'existing-credential' }))
mock.module('@simplewebauthn/browser', () => ({
  startRegistration: registration,
  startAuthentication: authentication,
}))
const { accessWithPasskey, accessErrorMessage, validateDisplayName } =
  await import('../src/auth/passkeys')
const originalFetch = globalThis.fetch

afterEach(() => {
  globalThis.fetch = originalFetch
  registration.mockClear()
  authentication.mockClear()
})

test('registration unwraps Go options and sends the credential before returning identity', async () => {
  const calls: { url: string; body: unknown }[] = []
  const options = { challenge: 'challenge' }
  const account = { id: 'account', display_name: 'Cam' }
  globalThis.fetch = mockFetch(async (url, init) => {
    calls.push({ url: String(url), body: JSON.parse(String(init?.body)) })
    return Response.json(
      calls.length === 1 ? { publicKey: options } : { account },
    )
  })
  expect(
    await accessWithPasskey({ kind: 'register', displayName: ' Cam ' }),
  ).toEqual(account)
  expect(registration).toHaveBeenCalledWith({ optionsJSON: options })
  expect(calls).toEqual([
    { url: '/api/auth/register/begin', body: { display_name: 'Cam' } },
    { url: '/api/auth/register/finish', body: { id: 'created-credential' } },
  ])
})

test('cancellation does not finish; a manual login retry begins again', async () => {
  const calls: string[] = []
  globalThis.fetch = mockFetch(async (url) => {
    calls.push(String(url))
    return Response.json(
      String(url).endsWith('begin')
        ? { publicKey: { challenge: 'fresh' } }
        : { account: { id: 'account', display_name: 'Cam' } },
    )
  })
  authentication.mockRejectedValueOnce(
    new DOMException('Cancelled', 'NotAllowedError'),
  )
  await expect(accessWithPasskey({ kind: 'login' })).rejects.toThrow(
    'Cancelled',
  )
  expect(calls).toEqual(['/api/auth/login/begin'])
  await accessWithPasskey({ kind: 'login' })
  expect(calls).toEqual([
    '/api/auth/login/begin',
    '/api/auth/login/begin',
    '/api/auth/login/finish',
  ])
  expect(
    accessErrorMessage(new DOMException('Cancelled', 'NotAllowedError')),
  ).toContain('cancelled')
})

test('registration preserves session-failure recovery guidance without retrying', async () => {
  let requests = 0
  const recovery =
    'account saved but session could not be started; sign in with your passkey'
  globalThis.fetch = mockFetch(async () => {
    requests++
    return requests === 1
      ? Response.json({ publicKey: {} })
      : Response.json({ error: recovery }, { status: 503 })
  })
  const error = await accessWithPasskey({
    kind: 'register',
    displayName: 'Cam',
  }).catch((error: Error) => error)
  expect(error).toBeInstanceOf(Error)
  expect(accessErrorMessage(error as Error)).toBe(recovery)
  expect(requests).toBe(2)
})

test('display names count Unicode code points and reject control characters', () => {
  expect(validateDisplayName('😀'.repeat(64))).toBeUndefined()
  expect(validateDisplayName('😀'.repeat(65))).toBeDefined()
  expect(validateDisplayName('   ')).toBeDefined()
  expect(validateDisplayName('Ca\u0000m')).toBeDefined()
})
