import { type QueryClient, useQuery } from '@tanstack/react-query'
import {
  createFileRoute,
  Outlet,
  redirect,
  useNavigate,
} from '@tanstack/react-router'
import { useEffect, useRef } from 'react'
import queries from '~/api/queries'
import { AccessError, AccessPending } from '~/components/access-status'

export const Route = createFileRoute('/_authed')({
  beforeLoad: ({ context }) => requireAccount(context.queryClient),
  pendingComponent: AccessPending,
  errorComponent: AccessError,
  component: AuthedLayout,
})

function AuthedLayout() {
  const { queryClient } = Route.useRouteContext()
  const navigate = useNavigate()
  const leaving = useRef(false)
  const currentUser = useQuery({
    ...queries.auth.currentUser,
    refetchOnMount: false,
    refetchInterval: 30_000,
    refetchOnWindowFocus: true,
    refetchOnReconnect: true,
  })

  useEffect(() => {
    if (currentUser.data === null && !leaving.current) {
      leaving.current = true
      void clearSessionData(queryClient).then(() =>
        navigate({ to: '/login', replace: true }),
      )
    }
  }, [currentUser.data, queryClient, navigate])

  if (currentUser.data === null)
    return (
      <p role="status" className="px-6 py-20">
        Your session has ended. Returning to sign in…
      </p>
    )
  if (currentUser.isError) return <AccessError />
  if (!currentUser.data) return <AccessPending />
  return <Outlet />
}

export async function requireAccount(queryClient: QueryClient) {
  const account = await queryClient.fetchQuery(queries.auth.currentUser)
  if (!account) throw redirect({ to: '/login', replace: true })
  return { account }
}

export async function clearSessionData(queryClient: QueryClient) {
  // Abort requests before clearing data so late responses cannot restore it.
  await queryClient.cancelQueries()
  queryClient.removeQueries({
    predicate: (query) =>
      query.queryKey[0] !== queries.auth.currentUser.queryKey[0],
  })
  queryClient.getMutationCache().clear()
  queryClient.setQueryData(queries.auth.currentUser.queryKey, null)
}
