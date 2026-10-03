import {
  type PublicKeyCredentialCreationOptionsJSON,
  type PublicKeyCredentialRequestOptionsJSON,
  startAuthentication,
  startRegistration,
} from '@simplewebauthn/browser'

export type Account = { id: string; display_name: string }
export type AccessAttempt =
  | { kind: 'login' }
  | { kind: 'register'; displayName: string }
class RequestError extends Error {}

async function post<T>(path: string, body: unknown): Promise<T> {
  let response: Response
  try {
    response = await fetch(`/api/auth/${path}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'same-origin',
      cache: 'no-store',
      body: JSON.stringify(body),
    })
  } catch {
    throw new RequestError(
      'Could not reach the server. Check your connection and try again. If you just created a passkey, try signing in first.',
    )
  }
  const data = await response.json().catch(() => null)
  if (!response.ok) {
    throw new RequestError(
      typeof data?.error === 'string'
        ? data.error
        : 'The server could not complete this request. Please try again.',
    )
  }
  if (!data)
    throw new RequestError(
      'The server returned an unexpected response. If you just created a passkey, try signing in first.',
    )
  return data as T
}

export async function accessWithPasskey(
  attempt: AccessAttempt,
): Promise<Account> {
  if (attempt.kind === 'register') {
    const options = await post<{
      publicKey: PublicKeyCredentialCreationOptionsJSON
    }>('register/begin', { display_name: attempt.displayName.trim() })
    const credential = await startRegistration({
      optionsJSON: options.publicKey,
    })
    const result = await post<{ account: Account }>(
      'register/finish',
      credential,
    )
    return result.account
  }
  const options = await post<{
    publicKey: PublicKeyCredentialRequestOptionsJSON
  }>('login/begin', {})
  const credential = await startAuthentication({
    optionsJSON: options.publicKey,
  })
  const result = await post<{ account: Account }>('login/finish', credential)
  return result.account
}

export function accessErrorMessage(error: Error): string {
  if (error instanceof RequestError) return error.message
  const cause = error.cause instanceof Error ? error.cause : error
  if (cause.name === 'NotAllowedError' || cause.name === 'AbortError') {
    return 'The passkey prompt was cancelled, timed out, or was not allowed. Try again when you’re ready.'
  }
  if (cause.name === 'InvalidStateError')
    return 'This passkey may already be registered. Try signing in instead.'
  if (cause.name === 'SecurityError')
    return 'Passkeys are unavailable at this address. Use the documented localhost URL or HTTPS.'
  return 'Could not use your passkey. Check that your browser and device support passkeys, then try again.'
}

export function validateDisplayName(value: string): string | undefined {
  const name = value.trim()
  if (!name || [...name].length > 64 || /\p{Cc}/u.test(name)) {
    return 'Enter 1–64 characters without control characters.'
  }
}
