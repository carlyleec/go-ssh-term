import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import queries from '../api/queries'

export function useAuth() {
  const client = useQueryClient()
  const currentUser = useQuery({
    ...queries.auth.currentUser,
    refetchOnMount: false,
    refetchOnWindowFocus: false,
    refetchOnReconnect: false,
  })
  const signOut = useMutation({
    ...queries.auth.logout,
    onSuccess: async () => {
      // A late session check must not restore identity after confirmed logout.
      await client.cancelQueries({
        queryKey: queries.auth.currentUser.queryKey,
      })
      client.setQueryData(queries.auth.currentUser.queryKey, null)
    },
  })
  return { account: currentUser.data, signOut }
}
