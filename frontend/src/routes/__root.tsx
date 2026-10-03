import type { QueryClient } from '@tanstack/react-query'
import { createRootRouteWithContext } from '@tanstack/react-router'
import { App, NotFoundPage } from '../app'

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()(
  {
    component: App,
    notFoundComponent: NotFoundPage,
  },
)
