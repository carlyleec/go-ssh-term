import { createRouter } from '@tanstack/react-router'
import { routeTree } from './routetree.gen'

export const router = createRouter({ routeTree })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
