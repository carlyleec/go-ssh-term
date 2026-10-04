import { afterEach, expect, test } from 'bun:test'
import apiClient, { ApiError } from '../src/api/apiClient'
import { mockFetch } from './mock-fetch'

const originalFetch = globalThis.fetch
afterEach(() => {
  globalThis.fetch = originalFetch
})

test('returns the API envelope and leaves multipart boundaries to the browser', async () => {
  const body = new FormData()
  body.append('name', 'demo')
  globalThis.fetch = mockFetch(async (_url, options) => {
    expect(options?.body).toBe(body)
    expect(new Headers(options?.headers).has('Content-Type')).toBe(false)
    expect(options?.credentials).toBe('same-origin')
    return Response.json({ key: { id: 'key-id' } })
  })
  expect(
    await apiClient.post<{ key: { id: string } }>('/api/keys', body),
  ).toEqual({
    key: { id: 'key-id' },
  })
})

test('HTTP errors retain status and body, including 401 and non-JSON failures', async () => {
  for (const status of [401, 409, 500]) {
    globalThis.fetch = mockFetch(async () =>
      Response.json({ error: 'Request failed' }, { status }),
    )
    const error = await apiClient.get('/api/keys').catch((error) => error)
    expect(error).toBeInstanceOf(ApiError)
    if (!(error instanceof ApiError)) throw new Error('Expected ApiError')
    expect(error.status).toBe(status)
    expect(error.body).toEqual({ error: 'Request failed' })
    expect(error.message).toBe('Request failed')
  }
  globalThis.fetch = mockFetch(
    async () => new Response('Unavailable', { status: 502 }),
  )
  const error = await apiClient.get('/api/keys').catch((error) => error)
  if (!(error instanceof ApiError)) throw new Error('Expected ApiError')
  expect(error.status).toBe(502)
  expect(error.body).toBeNull()
})

test('distinguishes network failures from cancellations', async () => {
  const failure = new TypeError('offline')
  globalThis.fetch = mockFetch(async () => {
    throw failure
  })
  const error = await apiClient.get('/api/keys').catch((error) => error)
  expect(error).toBeInstanceOf(ApiError)
  if (!(error instanceof ApiError)) throw new Error('Expected ApiError')
  expect(error.status).toBe(0)
  const controller = new AbortController()
  controller.abort()
  expect(
    await apiClient
      .get('/api/keys', { signal: controller.signal })
      .catch((error) => error),
  ).toBe(failure)
})

test('accepts empty 204 responses and rejects malformed JSON and unexpected success statuses', async () => {
  globalThis.fetch = mockFetch(async () => new Response(null, { status: 204 }))
  expect(await apiClient.delete('/api/keys/key-id')).toBeUndefined()
  globalThis.fetch = mockFetch(async () => new Response('not JSON'))
  await expect(apiClient.get('/api/keys')).rejects.toThrow(
    'unexpected response',
  )
  globalThis.fetch = mockFetch(async () => Response.json({}))
  await expect(
    apiClient.post('/api/auth/logout', {}, { expectedStatus: 204 }),
  ).rejects.toBeInstanceOf(ApiError)
})
