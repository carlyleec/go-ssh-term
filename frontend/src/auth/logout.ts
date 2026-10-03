import type { QueryClient } from '@tanstack/react-query'
import { currentUserOptions } from './current-user'

export async function clearSessionData(queryClient: QueryClient) {
  // Abort requests before clearing data so late responses cannot restore it.
  await queryClient.cancelQueries()
  queryClient.removeQueries({
    predicate: (query) => query.queryKey[0] !== currentUserOptions.queryKey[0],
  })
  queryClient.getMutationCache().clear()
  queryClient.setQueryData(currentUserOptions.queryKey, null)
}

export async function logout() {
  let response: Response
  try {
    response = await fetch('/api/auth/logout', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: '{}',
      cache: 'no-store',
    })
  } catch {
    throw new Error(
      'Could not confirm sign-out. Check your connection and try again.',
    )
  }
  if (response.status !== 204)
    throw new Error('Could not sign out. Please try again.')
}
