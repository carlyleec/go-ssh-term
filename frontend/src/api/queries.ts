import {
  type PublicKeyCredentialCreationOptionsJSON,
  type PublicKeyCredentialRequestOptionsJSON,
  startAuthentication,
  startRegistration,
} from '@simplewebauthn/browser'
import { queryOptions, useQuery } from '@tanstack/react-query'
import apiClient, { ApiError } from './apiClient'
import { ENDPOINTS } from './endpoints'

import type {
  SchemaAccount,
  SchemaConnection,
  SchemaConnectionFields,
  SchemaImportRequest,
  SchemaKeyMetadata,
} from './generated/schema.gen'
import {
  ConnectionBodySchema,
  ConnectionsBodySchema,
  CurrentUserOutputBodySchema,
  HostInspectionSchema,
  ImportPreviewSchema,
  KeysBodySchema,
} from './generated/zod.gen'
export type Account = SchemaAccount
export type AccessAttempt =
  | { kind: 'login' }
  | { kind: 'register'; displayName: string }
export type SSHKey = SchemaKeyMetadata

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
      if (!CurrentUserOutputBodySchema.safeParse(data).success) {
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
    mutationKey: ['sign-out'],
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
      if (!KeysBodySchema.safeParse(data).success)
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

export type SavedConnection = SchemaConnection
export type ConnectionFields = SchemaConnectionFields
const connectionOptions = (accountID: string) =>
  queryOptions({
    queryKey: ['connections', accountID],
    queryFn: async ({ signal }): Promise<SavedConnection[]> => {
      const data = await apiClient.get(ENDPOINTS.connections, { signal })
      return ConnectionsBodySchema.parse(data).connections
    },
    retry: false,
    networkMode: 'always',
  })
const connections = {
  previewImport: {
    mutationFn: async (body: SchemaImportRequest) =>
      ImportPreviewSchema.parse(
        await apiClient.post(`${ENDPOINTS.connections}/import/preview`, body, {
          expectedStatus: 200,
        }),
      ),
    retry: false,
    networkMode: 'always' as const,
  },
  confirmImport: {
    mutationFn: async (body: SchemaImportRequest) =>
      ConnectionsBodySchema.parse(
        await apiClient.post(`${ENDPOINTS.connections}/import/confirm`, body, {
          expectedStatus: 201,
        }),
      ),
    retry: false,
    networkMode: 'always' as const,
  },
  hostOptions: (accountID: string, id: string) =>
    queryOptions({
      queryKey: ['host-inspection', accountID, id],
      queryFn: async ({ signal }) =>
        HostInspectionSchema.parse(
          await apiClient.post(
            `${ENDPOINTS.connections}/${encodeURIComponent(id)}/host-key`,
            {},
            { signal, expectedStatus: 200 },
          ),
        ),
      retry: false,
      networkMode: 'always',
      refetchOnMount: 'always',
      refetchOnWindowFocus: false,
      refetchOnReconnect: false,
    }),
  approveHost: {
    mutationFn: async ({
      id,
      decision,
    }: {
      id: string
      decision: {
        host: string
        port: number
        fingerprint: string
        jump_connection_id?: string
      }
    }) =>
      HostInspectionSchema.parse(
        await apiClient.post(
          `${ENDPOINTS.connections}/${encodeURIComponent(id)}/host-trust`,
          decision,
          { expectedStatus: 200 },
        ),
      ),
    retry: false,
    networkMode: 'always' as const,
  },
  resetTrust: {
    mutationFn: ({
      id,
      decision,
    }: {
      id: string
      decision: {
        host: string
        port: number
        fingerprint: string
        jump_connection_id?: string
      }
    }) =>
      apiClient.post<void>(
        `${ENDPOINTS.connections}/${encodeURIComponent(id)}/host-trust/reset`,
        decision,
        { expectedStatus: 204 },
      ),
    retry: false,
    networkMode: 'always' as const,
  },
  options: connectionOptions,
  useQuery: (accountID: string) => useQuery(connectionOptions(accountID)),
  save: {
    mutationFn: async ({
      id,
      fields,
    }: {
      id?: string
      fields: ConnectionFields
    }): Promise<SavedConnection> => {
      const data = id
        ? await apiClient.put(
            `${ENDPOINTS.connections}/${encodeURIComponent(id)}`,
            fields,
            { expectedStatus: 200 },
          )
        : await apiClient.post(ENDPOINTS.connections, fields, {
            expectedStatus: 201,
          })
      return ConnectionBodySchema.parse(data).connection
    },
    retry: false,
    networkMode: 'always' as const,
  },
  remove: {
    mutationFn: (id: string) =>
      apiClient.delete<void>(
        `${ENDPOINTS.connections}/${encodeURIComponent(id)}`,
        { expectedStatus: 204 },
      ),
    retry: false,
    networkMode: 'always' as const,
  },
}

const queries = { auth, keys, connections }

export default queries
