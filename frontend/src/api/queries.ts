import {
  type PublicKeyCredentialCreationOptionsJSON,
  type PublicKeyCredentialRequestOptionsJSON,
  startAuthentication,
  startRegistration,
} from '@simplewebauthn/browser'
import { queryOptions, useQuery } from '@tanstack/react-query'
import apiClient, { ApiError } from './apiClient'
import { ENDPOINTS } from './endpoints'

export type Account = { id: string; display_name: string }
export type AccessAttempt =
  | { kind: 'login' }
  | { kind: 'register'; displayName: string }
export type SSHKey = {
  id: string
  name: string
  public_fingerprint: string
  created_at: string
}

const auth = {
  currentUser: queryOptions({
    queryKey: ['current-user'],
    queryFn: async ({ signal }): Promise<Account | null> => {
      let data: { account: Account }
      try {
        data = await apiClient.get<{ account: Account }>(
          ENDPOINTS.currentUser,
          {
            signal,
          },
        )
      } catch (error) {
        if (error instanceof ApiError && error.status === 401) return null
        throw error
      }
      if (
        typeof data?.account?.id !== 'string' ||
        typeof data?.account?.display_name !== 'string'
      ) {
        throw new Error('The server returned an unexpected account response.')
      }
      return data.account
    },
    // Recheck server authority on each navigation, including after session expiry.
    staleTime: 0,
    retry: false,
    networkMode: 'always',
  }),
  access: {
    mutationFn: async (attempt: AccessAttempt): Promise<Account> => {
      if (attempt.kind === 'register') {
        const options = await apiClient.post<{
          publicKey: PublicKeyCredentialCreationOptionsJSON
        }>(ENDPOINTS.registerBegin, {
          display_name: attempt.displayName.trim(),
        })
        const credential = await startRegistration({
          optionsJSON: options.publicKey,
        })
        const result = await apiClient.post<{ account: Account }>(
          ENDPOINTS.registerFinish,
          credential,
        )
        return result.account
      }
      const options = await apiClient.post<{
        publicKey: PublicKeyCredentialRequestOptionsJSON
      }>(ENDPOINTS.loginBegin, {})
      const credential = await startAuthentication({
        optionsJSON: options.publicKey,
      })
      const result = await apiClient.post<{ account: Account }>(
        ENDPOINTS.loginFinish,
        credential,
      )
      return result.account
    },
    retry: false,
    // Interactive prompts must not be queued for a later network reconnect.
    networkMode: 'always' as const,
  },
  logout: {
    mutationFn: async () => {
      try {
        await apiClient.post<void>(
          ENDPOINTS.logout,
          {},
          { expectedStatus: 204 },
        )
      } catch (error) {
        if (error instanceof ApiError && error.status === 0)
          throw new Error(
            'Could not confirm sign-out. Check your connection and try again.',
          )
        throw new Error('Could not sign out. Please try again.')
      }
    },
    retry: false,
    networkMode: 'always' as const,
  },
}

const keyOptions = (accountID: string) =>
  queryOptions({
    queryKey: ['ssh-keys', accountID],
    queryFn: async ({ signal }): Promise<SSHKey[]> => {
      const data = await apiClient.get<{ keys: SSHKey[] }>(ENDPOINTS.keys, {
        signal,
      })
      if (!Array.isArray(data?.keys))
        throw new Error('The server returned an unexpected key list.')
      return data.keys
    },
    retry: false,
    networkMode: 'always',
  })

const keys = {
  options: keyOptions,
  useQuery: (accountID: string) => useQuery(keyOptions(accountID)),
  upload: {
    mutationFn: async ({ name, file }: { name: string; file: File | null }) => {
      const body = new FormData()
      body.append('name', name.trim())
      if (file) body.append('private_key', file)
      await apiClient.post(ENDPOINTS.keys, body)
    },
    retry: false,
    networkMode: 'always' as const,
  },
  remove: {
    mutationFn: (id: string) =>
      apiClient.delete<void>(`${ENDPOINTS.keys}/${encodeURIComponent(id)}`),
    retry: false,
    networkMode: 'always' as const,
  },
}

const queries = { auth, keys }

export default queries
