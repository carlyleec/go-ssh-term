import { type QueryClient, queryOptions } from '@tanstack/react-query'
import { redirect } from '@tanstack/react-router'
import type { Account } from './passkeys'

export const currentUserOptions = queryOptions({
  queryKey: ['current-user'],
  queryFn: async ({ signal }): Promise<Account | null> => {
    const response = await fetch('/api/auth/me', {
      credentials: 'same-origin',
      cache: 'no-store',
      signal,
    })
    if (response.status === 401) return null
    if (!response.ok)
      throw new Error('Could not check your session. Please try again.')
    const data = await response.json()
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
})

export async function requireAccount(queryClient: QueryClient) {
  const account = await queryClient.fetchQuery(currentUserOptions)
  if (!account) throw redirect({ to: '/login', replace: true })
  return { account }
}

export async function redirectSignedIn(queryClient: QueryClient) {
  const account = await queryClient.fetchQuery(currentUserOptions)
  if (account) throw redirect({ to: '/connections', replace: true })
}
