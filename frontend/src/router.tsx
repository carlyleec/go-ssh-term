import {
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
} from '@tanstack/react-router'
import { App } from './app'

const rootRoute = createRootRoute({
  component: Outlet,
})

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: App,
})

export const router = createRouter({
  routeTree: rootRoute.addChildren([indexRoute]),
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
