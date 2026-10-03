export type SSHKey = {
  id: string
  name: string
  public_fingerprint: string
  created_at: string
}

export class SessionExpired extends Error {
  constructor() {
    super('Your session has ended. Please sign in again.')
  }
}

export async function keyRequest(path = '', init?: RequestInit) {
  let response: Response
  try {
    response = await fetch(`/api/keys${path}`, {
      credentials: 'same-origin',
      cache: 'no-store',
      ...init,
    })
  } catch (error) {
    if (init?.signal?.aborted) throw error
    throw new Error(
      'Could not reach the server. Check your connection and retry.',
    )
  }
  if (response.status === 401) throw new SessionExpired()
  if (!response.ok) {
    const data = await response.json().catch(() => null)
    throw new Error(
      typeof data?.error === 'string'
        ? data.error
        : 'Could not complete the key request. Please retry.',
    )
  }
  return response
}

export async function listKeys(signal: AbortSignal): Promise<SSHKey[]> {
  const response = await keyRequest('', { signal })
  const data = await response.json()
  if (!Array.isArray(data?.keys))
    throw new Error('The server returned an unexpected key list.')
  return data.keys
}

export function validateKeyName(value: string) {
  const name = value.trim()
  if (!name) return 'Enter a name for this key.'
  if ([...name].length > 64) return 'Use at most 64 characters.'
  if (/\p{Cc}/u.test(name)) return 'The name cannot contain control characters.'
  if (new TextEncoder().encode(value).length > 256)
    return 'The name is too long. Remove extra whitespace.'
  return undefined
}

export function validateKeyFile(file: File | null) {
  if (!file || file.size === 0) return 'Choose a private-key file.'
  if (file.size > 16_384) return 'The private-key file must be at most 16 KiB.'
  return undefined
}
